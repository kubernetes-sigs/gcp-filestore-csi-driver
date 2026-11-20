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
	"net/url"
	"strings"
	"time"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/file"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/util"
)

const (
	paramKeyVolumePool       = "volume-pool"
	volumePoolURLScheme      = "volumepool"
	volumePoolURLPrefix      = volumePoolURLScheme + "://"
	volumePoolCleanupTimeout = 15 * time.Second
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

func (s *controllerServer) volumePoolQuotaEnabled() bool {
	return s.config.features != nil &&
		s.config.features.FeatureVolumePools != nil &&
		s.config.features.FeatureVolumePools.QuotaEnabled
}

func (s *controllerServer) handleCreateVolumePoolVolume(ctx context.Context, req *csi.CreateVolumeRequest, poolPath string) (*csi.CreateVolumeResponse, error) {
	name := req.GetName()
	if len(name) == 0 {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume name must be provided")
	}
	quotaEnabled := s.volumePoolQuotaEnabled()
	var quotaMib int64
	if quotaEnabled {
		var err error
		quotaMib, err = volumePoolVolumeQuotaMib(req.GetCapacityRange())
		if err != nil {
			return nil, err
		}
	}

	klog.V(4).Infof("Creating Volume Pool volume: pool %q, requestID %q, quota %d MiB (0 for the volume pool's default)", poolPath, name, quotaMib)
	poolVol, err := s.config.fileService.CreateVolumePoolVolume(ctx, poolPath, name, quotaMib)
	if err != nil {
		klog.Errorf("Failed to create Volume Pool volume %q in pool %s, quota %d MiB (0 for the volume pool's default): %v", name, poolPath, quotaMib, err)
		return nil, file.StatusError(err)
	}

	var capacityBytes int64
	if quotaEnabled {
		capacityBytes, err = volumePoolVolumeCapacityBytes(poolVol, req.GetCapacityRange(), quotaMib)
		if err != nil {
			klog.Errorf("Volume Pool volume %q in pool %s has unusable capacity: %v; deleting volume", name, poolPath, err)
			if cleanupErr := s.cleanupVolumePoolVolume(ctx, poolPath, name); cleanupErr != nil {
				klog.Errorf("Failed to delete Volume Pool volume %q in pool %s after capacity mismatch: %v", name, poolPath, cleanupErr)
				return nil, status.Errorf(codes.Internal, "%v (cleanup failed; volume may be orphaned: %v)", err, cleanupErr)
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
	} else {
		capacityBytes = req.GetCapacityRange().GetRequiredBytes()
		if poolVol.QuotaMib > 0 && poolVol.QuotaMib <= file.MaxVolumePoolVolumeQuotaMib && capacityBytes > 0 && poolVol.QuotaMib*util.Mb < capacityBytes {
			klog.Warningf("Volume Pool volume %q created with pool default quota %d MiB, smaller than requested %d bytes (per-volume quota sizing is disabled)", poolVol.Name, poolVol.QuotaMib, capacityBytes)
		}
	}

	compositeVolumeID := buildVolumePoolVolumeID(poolPath, name, poolVol.IpAddress)

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

// cleanupVolumePoolVolume deletes a volume pool volume on a bounded context
// detached from ctx's cancellation so post-create cleanup still runs if the
// caller's deadline has nearly elapsed.
func (s *controllerServer) cleanupVolumePoolVolume(ctx context.Context, poolPath, name string) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), volumePoolCleanupTimeout)
	defer cancel()
	volName := fmt.Sprintf("%s/volumes/%s", poolPath, name)
	if err := s.config.fileService.DeleteVolumePoolVolume(cleanupCtx, volName); err != nil && !file.IsNotFoundErr(err) {
		return err
	}
	return nil
}

// volumePoolVolumeQuotaMib returns the quota, in MiB, that a new volume pool
// volume gets for capRange, the capacity range of a CreateVolume request. It
// returns 0 if capRange asks for no capacity: the volume then gets its volume
// pool's default volume quota.
//
// The quota is the required capacity rounded up to a whole MiB and raised to at
// least file.MinVolumePoolVolumeQuotaMib, so that the volume is at least as
// large as required. With only a limit, it is the limit rounded down to a whole
// MiB, and at most file.MaxVolumePoolVolumeQuotaMib, so that the volume is not
// larger than the limit.
//
// A malformed capacity range gives InvalidArgument. A capacity range that no
// quota the API accepts, from file.MinVolumePoolVolumeQuotaMib to
// file.MaxVolumePoolVolumeQuotaMib, satisfies gives OutOfRange, as the CSI spec
// requires for an unsupported capacity range.
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

	if limit > 0 && limit < file.MinVolumePoolVolumeQuotaMib*util.Mb {
		return 0, status.Errorf(codes.OutOfRange, "capacity limit of %d bytes is less than the minimum volume pool volume size of %d MiB", limit, file.MinVolumePoolVolumeQuotaMib)
	}

	if required == 0 {
		return min(limit/util.Mb, file.MaxVolumePoolVolumeQuotaMib), nil
	}

	if required > file.MaxVolumePoolVolumeQuotaMib*util.Mb {
		return 0, status.Errorf(codes.OutOfRange, "required capacity of %d bytes exceeds the maximum volume pool volume size of %d MiB", required, file.MaxVolumePoolVolumeQuotaMib)
	}
	quotaMib := (required + util.Mb - 1) / util.Mb
	if quotaMib < file.MinVolumePoolVolumeQuotaMib {
		klog.Warningf("Required capacity of %d bytes is less than the minimum volume pool volume size of %d MiB; rounding up to %d MiB", required, file.MinVolumePoolVolumeQuotaMib, file.MinVolumePoolVolumeQuotaMib)
		return file.MinVolumePoolVolumeQuotaMib, nil
	}
	if limit > 0 && quotaMib*util.Mb > limit {
		return 0, status.Errorf(codes.OutOfRange, "capacity range from %d to %d bytes contains no whole number of MiB", required, limit)
	}
	return quotaMib, nil
}

// volumePoolVolumeCapacityBytes returns the capacity, in bytes, of vol, a
// volume that CreateVolume created for capRange with a quota of
// requestedQuotaMib MiB.
//
// If the API omits the quota (0), it logs a warning and reports
// requestedQuotaMib instead (or 0, meaning unknown, if none was requested). If
// the API reports a quota that is negative, exceeds
// file.MaxVolumePoolVolumeQuotaMib, or does not satisfy capRange, it returns an
// error so CreateVolume can delete the volume and fail rather than returning a
// volume outside the requested capacity range.
func volumePoolVolumeCapacityBytes(vol *file.PoolVolume, capRange *csi.CapacityRange, requestedQuotaMib int64) (int64, error) {
	if vol.QuotaMib == 0 {
		klog.Warningf("Volume Pool volume %q: the API reported no quota; reporting the requested quota of %d MiB (0 for none) as its capacity instead", vol.Name, requestedQuotaMib)
		return requestedQuotaMib * util.Mb, nil
	}
	if vol.QuotaMib < 0 || vol.QuotaMib > file.MaxVolumePoolVolumeQuotaMib {
		return 0, fmt.Errorf("volume pool volume %q has invalid quota %d MiB (supported range: 1 to %d MiB)", vol.Name, vol.QuotaMib, file.MaxVolumePoolVolumeQuotaMib)
	}
	capBytes := vol.QuotaMib * util.Mb
	if required := capRange.GetRequiredBytes(); required > 0 && capBytes < required {
		return 0, fmt.Errorf("volume pool volume %q quota %d MiB (%d bytes) is less than required capacity of %d bytes", vol.Name, vol.QuotaMib, capBytes, required)
	}
	if limit := capRange.GetLimitBytes(); limit > 0 && capBytes > limit {
		return 0, fmt.Errorf("volume pool volume %q quota %d MiB (%d bytes) exceeds capacity limit of %d bytes", vol.Name, vol.QuotaMib, capBytes, limit)
	}
	return capBytes, nil
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
