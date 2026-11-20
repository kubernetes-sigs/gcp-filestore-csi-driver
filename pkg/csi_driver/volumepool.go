/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package driver

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"strings"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/file"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/util"
)

const (
	paramKeyVolumePool  = "volume-pool"
	volumePoolURLScheme = "volumepool"
	volumePoolURLPrefix = volumePoolURLScheme + "://"
)

func buildVolumePoolVolumeID(parentPool, volumeID, ipAddress string) string {
	// Scheme format: volumepool://{project}/{location}/{volume_pool}/volumes/{uuid}?ip={ip_address}
	// parentPool format: projects/{project}/locations/{location}/volumePools/{volume_pool}

	hostPath := strings.Replace(parentPool, "projects/", volumePoolURLPrefix, 1)
	hostPath = strings.Replace(hostPath, "/locations/", "/", 1)
	hostPath = strings.Replace(hostPath, "/volumePools/", "/", 1)

	return fmt.Sprintf("%s/volumes/%s?ip=%s", hostPath, volumeID, ipAddress)
}

func parseVolumePoolVolumeID(volumeID string) (parent, volID, ipAddress string, err error) {
	u, err := url.Parse(volumeID)
	if err != nil {
		return "", "", "", err
	}

	if u.Scheme != volumePoolURLScheme {
		return "", "", "", fmt.Errorf("invalid volume ID scheme %q: expected %q", u.Scheme, volumePoolURLScheme)
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 4 || parts[2] != "volumes" {
		return "", "", "", fmt.Errorf("invalid composite volume ID path: %s", volumeID)
	}

	project := u.Host
	location := parts[0]
	poolName := parts[1]
	volID = parts[3]

	parent = fmt.Sprintf("projects/%s/locations/%s/volumePools/%s", project, location, poolName)
	ipAddress = u.Query().Get("ip")

	return
}

func isVolumePoolVolumeID(volumeID string) bool {
	return strings.HasPrefix(volumeID, volumePoolURLPrefix)
}

func (s *controllerServer) handleCreateVolumePoolVolume(ctx context.Context, req *csi.CreateVolumeRequest, poolPath string) (*csi.CreateVolumeResponse, error) {
	name := req.GetName()
	if len(name) == 0 {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume name must be provided")
	}
	quotaMib, err := volumePoolVolumeQuotaMib(req.GetCapacityRange())
	if err != nil {
		return nil, err
	}

	klog.V(4).Infof("Creating Volume Pool volume: pool %q, requestID %q, quota %d MiB (0 for the volume pool's default)", poolPath, name, quotaMib)
	poolVol, err := s.config.fileService.CreateVolumePoolVolume(ctx, poolPath, name, quotaMib)
	if err != nil {
		klog.Errorf("Failed to create Volume Pool volume %q in pool %s, quota %d MiB (0 for the volume pool's default): %v", name, poolPath, quotaMib, err)
		return nil, file.StatusError(err)
	}

	compositeVolumeID := buildVolumePoolVolumeID(poolPath, name, poolVol.IpAddress)
	capacityBytes := volumePoolVolumeCapacityBytes(poolVol, quotaMib)

	klog.Infof("Successfully created Volume Pool volume: %q, Composite ID: %q, IP: %q, MountName: %q, Capacity: %d bytes", poolVol.Name, compositeVolumeID, poolVol.IpAddress, poolVol.MountName, capacityBytes)

	fileProtocol := v3FileProtocol
	if protocol, ok := req.GetParameters()[paramFileProtocol]; ok {
		if strings.ToUpper(protocol) == v4_1FileProtocol {
			fileProtocol = v4_1FileProtocol
		}
	}

	resp := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      compositeVolumeID,
			CapacityBytes: capacityBytes,
			VolumeContext: map[string]string{
				attrIP:           poolVol.IpAddress,
				attrVolume:       poolVol.MountName,
				attrFileProtocol: fileProtocol,
			},
		},
	}

	if mountOpts, ok := req.GetParameters()[paramMountOptions]; ok {
		resp.Volume.VolumeContext[attrMountOptions] = mountOpts
	}

	return resp, nil
}

// volumePoolVolumeQuotaMib returns the quota, in MiB, that a new volume pool
// volume gets for capRange, the capacity range of a CreateVolume request. It
// returns 0 if capRange asks for no capacity: the volume then gets its volume
// pool's default volume quota.
//
// The quota is the required capacity rounded up to a whole MiB, so that the
// volume is at least as large as required. With only a limit, it is the limit
// rounded down to a whole MiB, and at most the largest quota, so that the
// volume is not larger than the limit.
//
// A malformed capacity range gives InvalidArgument. A capacity range that no
// quota the API accepts, from file.MinVolumePoolVolumeQuotaMib to
// file.MaxVolumePoolVolumeQuotaMib, satisfies gives OutOfRange, as the CSI spec
// requires for an unsupported capacity range. So does a required capacity of
// 99 MiB or less, which is still below the smallest quota after rounding up:
// it is refused rather than raised to the smallest quota.
func volumePoolVolumeQuotaMib(capRange *csi.CapacityRange) (int64, error) {
	required := capRange.GetRequiredBytes()
	limit := capRange.GetLimitBytes()
	switch {
	case required < 0 || limit < 0:
		return 0, status.Errorf(codes.InvalidArgument, "invalid capacity range: required bytes (%d) and limit bytes (%d) must not be negative", required, limit)
	case limit > 0 && required > limit:
		return 0, status.Errorf(codes.InvalidArgument, "invalid capacity range: required bytes (%d) exceed limit bytes (%d)", required, limit)
	case required == 0 && limit == 0:
		return 0, nil
	}

	if required == 0 {
		quotaMib := min(limit/util.Mb, file.MaxVolumePoolVolumeQuotaMib)
		if quotaMib < file.MinVolumePoolVolumeQuotaMib {
			return 0, status.Errorf(codes.OutOfRange, "capacity limit of %d bytes is less than the minimum volume pool volume size of %d MiB", limit, file.MinVolumePoolVolumeQuotaMib)
		}
		return quotaMib, nil
	}

	if required > file.MaxVolumePoolVolumeQuotaMib*util.Mb {
		return 0, status.Errorf(codes.OutOfRange, "required capacity of %d bytes exceeds the maximum volume pool volume size of %d MiB", required, file.MaxVolumePoolVolumeQuotaMib)
	}
	quotaMib := (required + util.Mb - 1) / util.Mb
	if quotaMib < file.MinVolumePoolVolumeQuotaMib {
		return 0, status.Errorf(codes.OutOfRange, "required capacity of %d bytes is less than the minimum volume pool volume size of %d MiB", required, file.MinVolumePoolVolumeQuotaMib)
	}
	if limit > 0 && quotaMib*util.Mb > limit {
		return 0, status.Errorf(codes.OutOfRange, "capacity range from %d to %d bytes contains no whole number of MiB", required, limit)
	}
	return quotaMib, nil
}

// volumePoolVolumeCapacityBytes returns the capacity, in bytes, of vol, a
// volume that CreateVolume created with a quota of requestedQuotaMib MiB: its
// quota as the API reports it, if that is positive and its size in bytes fits
// in an int64, or else, with a warning, the requested quota. 0, when neither
// is known, means that the capacity is unknown.
func volumePoolVolumeCapacityBytes(vol *file.PoolVolume, requestedQuotaMib int64) int64 {
	if vol.QuotaMib > 0 && vol.QuotaMib <= math.MaxInt64/util.Mb {
		return vol.QuotaMib * util.Mb
	}
	klog.Warningf("Volume Pool volume %q: the API reported a quota of %d MiB (0 for none), which can't be used; reporting the requested quota of %d MiB (0 for none) as its capacity instead", vol.Name, vol.QuotaMib, requestedQuotaMib)
	return requestedQuotaMib * util.Mb
}

func (s *controllerServer) handleDeleteVolumePoolVolume(ctx context.Context, req *csi.DeleteVolumeRequest, volumeID string) (*csi.DeleteVolumeResponse, error) {
	klog.V(4).Infof("Deleting Volume Pool volume from composite ID: %q", volumeID)

	parent, volID, ipAddress, err := parseVolumePoolVolumeID(volumeID)
	if err != nil {
		klog.Errorf("Failed to parse composite volume ID %q: %v", volumeID, err)
		return nil, status.Errorf(codes.InvalidArgument, "failed to parse composite volume ID: %v", err)
	}

	klog.V(4).Infof("Parsed release parameters: parent=%q, volID=%q, ipAddress=%q", parent, volID, ipAddress)

	name := fmt.Sprintf("%s/volumes/%s", parent, volID)
	err = s.config.fileService.DeleteVolumePoolVolume(ctx, name)
	if err != nil {
		if file.IsNotFoundErr(err) {
			klog.Warningf("Volume Pool volume %q not found, returning success", volumeID)
			return &csi.DeleteVolumeResponse{}, nil
		}
		klog.Errorf("Failed to delete Volume Pool volume %q: %v", volumeID, err)
		return nil, file.StatusError(err)
	}

	klog.Infof("Successfully deleted Volume Pool volume: %q", volumeID)
	return &csi.DeleteVolumeResponse{}, nil
}
