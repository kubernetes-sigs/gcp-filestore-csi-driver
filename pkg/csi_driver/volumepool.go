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
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/api/googleapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/file"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/util"
)

const (
	paramKeyVolumePool                = "volume-pool"
	defaultVolumePoolVolumeCapacityGb = 1
	volumePoolURLScheme               = "volumepool"
	volumePoolURLPrefix               = volumePoolURLScheme + "://"
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

// volumePoolStatusError converts an error from a Volume Pool API call to a gRPC
// status error. An HTTP 409 ABORTED (concurrent claims on the same pool volume)
// becomes codes.Aborted, which tells the caller that an immediate retry can
// succeed. Everything else, including HTTP 409 ALREADY_EXISTS, keeps the
// mapping of file.StatusError, which the non-Volume Pool paths also use.
func volumePoolStatusError(err error) error {
	if file.IsAbortedErr(err) {
		return status.Error(codes.Aborted, err.Error())
	}
	// googleapi.Error.Error() doesn't print the canonical status, so log the
	// body of any other HTTP 409 to show why it wasn't treated as ABORTED.
	var googleErr *googleapi.Error
	if errors.As(err, &googleErr) && googleErr.Code == http.StatusConflict {
		klog.Warningf("Volume Pool call failed with HTTP 409 that is not ABORTED, response body: %s", googleErr.Body)
	}
	return file.StatusError(err)
}

func (s *controllerServer) handleCreateVolumePoolVolume(ctx context.Context, req *csi.CreateVolumeRequest, poolPath string) (*csi.CreateVolumeResponse, error) {
	name := req.GetName()
	if len(name) == 0 {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume name must be provided")
	}

	klog.V(4).Infof("Creating Volume Pool volume: pool %q, requestID %q", poolPath, name)
	volumeName := fmt.Sprintf("%s/volumes/%s", poolPath, name)
	// Serialize operations on the same volume, as the other CreateVolume and
	// DeleteVolume paths do.
	if acquired := s.config.volumeLocks.TryAcquire(volumeName); !acquired {
		return nil, status.Errorf(codes.Aborted, util.VolumeOperationAlreadyExistsFmt, volumeName)
	}
	defer s.config.volumeLocks.Release(volumeName)

	// CreateVolume must be idempotent: a retried request returns the volume
	// that an earlier attempt created (for example after its response was
	// lost). Look the volume up first. Creating first, and looking up only on
	// HTTP 409 ALREADY_EXISTS, would save a call for every new volume, but the
	// API's CreateVolume looks for a free share before it checks the name: in
	// a full pool, a retry for an existing volume gets HTTP 429
	// RESOURCE_EXHAUSTED instead of ALREADY_EXISTS, and never its volume.
	result := "found existing"
	poolVol, err := s.lookupVolumePoolVolume(ctx, volumeName)
	if err != nil {
		return nil, err
	}
	if poolVol == nil {
		var createdByOther bool
		if poolVol, createdByOther, err = s.createVolumePoolVolume(ctx, poolPath, name); err != nil {
			return nil, err
		}
		result = "created"
		if createdByOther {
			result = "found (created by another request)"
		}
	}
	// Check the volume however it was found: existing, new, or created by
	// another request.
	if err := checkVolumePoolVolumeMountable(poolVol); err != nil {
		return nil, err
	}

	compositeVolumeID := buildVolumePoolVolumeID(poolPath, name, poolVol.IpAddress)

	klog.Infof("Successfully %s Volume Pool volume: %q, Composite ID: %q, IP: %q, MountName: %q", result, poolVol.Name, compositeVolumeID, poolVol.IpAddress, poolVol.MountName)

	fileProtocol := v3FileProtocol
	if protocol, ok := req.GetParameters()[paramFileProtocol]; ok {
		if strings.ToUpper(protocol) == v4_1FileProtocol {
			fileProtocol = v4_1FileProtocol
		}
	}

	resp := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      compositeVolumeID,
			CapacityBytes: req.GetCapacityRange().GetRequiredBytes(),
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

// lookupVolumePoolVolume returns the volume named volumeName, whatever its
// state, or nil if it does not exist.
//
// Volume Pool volumes created by the driver have no attributes that can differ
// between two CreateVolume requests with the same name: the request sends no
// quota or ip_acls, and the CreateVolumeResponse capacity, protocol and mount
// options come from the request. So an existing volume is always compatible,
// and codes.AlreadyExists is never returned. Once the driver sends a quota,
// compare it here and return codes.AlreadyExists on a mismatch.
func (s *controllerServer) lookupVolumePoolVolume(ctx context.Context, volumeName string) (*file.PoolVolume, error) {
	vol, err := s.config.fileService.GetVolumePoolVolume(ctx, volumeName)
	if err == nil {
		return vol, nil
	}
	statusErr := volumePoolStatusError(err)
	if status.Code(statusErr) == codes.NotFound {
		return nil, nil
	}
	klog.Errorf("Failed to look up Volume Pool volume %q: %v", volumeName, err)
	return nil, statusErr
}

// createVolumePoolVolume creates the volume name in poolPath. If another
// request created the volume after the lookup (HTTP 409 ALREADY_EXISTS), it
// returns that volume instead, with createdByOther set. The volume is returned
// whatever its state.
func (s *controllerServer) createVolumePoolVolume(ctx context.Context, poolPath, name string) (poolVol *file.PoolVolume, createdByOther bool, err error) {
	poolVol, err = s.config.fileService.CreateVolumePoolVolume(ctx, poolPath, name)
	if err == nil {
		return poolVol, false, nil
	}
	if !file.IsAlreadyExistsErr(err) {
		klog.Errorf("Failed to create Volume Pool volume %q in pool %s: %v", name, poolPath, err)
		return nil, false, volumePoolStatusError(err)
	}

	volumeName := fmt.Sprintf("%s/volumes/%s", poolPath, name)
	klog.Infof("Volume Pool volume %q was created by another request, looking it up", volumeName)
	poolVol, err = s.lookupVolumePoolVolume(ctx, volumeName)
	if err != nil {
		return nil, false, err
	}
	if poolVol == nil {
		// The volume was deleted between the two calls. Another CreateVolume
		// can succeed, so ask the caller to retry.
		klog.Warningf("Volume Pool volume %q was reported as existing but is not found", volumeName)
		return nil, false, status.Errorf(codes.Aborted, "volume %q was reported as existing but is not found, retry", volumeName)
	}
	return poolVol, true, nil
}

// isVolumePoolVolumeHot reports whether vol is HOT and has a mount point, so
// that it can be mounted. A volume whose state is unspecified, or from an API
// version that does not report states, counts as HOT if it has a mount point.
func isVolumePoolVolumeHot(vol *file.PoolVolume) bool {
	switch vol.State {
	case "", file.VolumePoolVolumeStateUnspecified, file.VolumePoolVolumeStateHot:
		return vol.IpAddress != "" && vol.MountName != ""
	default:
		return false
	}
}

// checkVolumePoolVolumeMountable returns nil if vol can be mounted, and
// otherwise the error to return for it. The API reports a mount point only for
// HOT and unspecified-state volumes.
//
// A COLD or WARM volume gives FailedPrecondition, because it stays so until it
// is unfrozen or made HOT. The CO then gets no volume ID that it could delete,
// so the volume must be unfrozen, made HOT or deleted out of band. That
// happens if a volume with the requested name was frozen after an earlier
// attempt created it, or if the API created the volume WARM, which it does
// when the volume's share denies all clients.
//
// A volume that is still being created, or that has no mount point for another
// reason, gives Aborted, so that the caller retries until it has one. For
// example, the API reports a volume whose instance is being recovered as
// unspecified-state, without a mount point.
func checkVolumePoolVolumeMountable(vol *file.PoolVolume) error {
	switch {
	case vol.State == file.VolumePoolVolumeStateCold:
		return status.Errorf(codes.FailedPrecondition, "volume %q is COLD; unfreeze it first", vol.Name)
	case vol.State == file.VolumePoolVolumeStateWarm:
		return status.Errorf(codes.FailedPrecondition, "volume %q is WARM, which cannot be mounted; make it HOT first", vol.Name)
	case vol.State == file.VolumePoolVolumeStateCreating:
		return status.Errorf(codes.Aborted, "volume %q is still being created", vol.Name)
	case !isVolumePoolVolumeHot(vol):
		return status.Errorf(codes.Aborted, "volume %q has no mount point yet: its state is %q", vol.Name, vol.State)
	}
	return nil
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
	if acquired := s.config.volumeLocks.TryAcquire(name); !acquired {
		return nil, status.Errorf(codes.Aborted, util.VolumeOperationAlreadyExistsFmt, name)
	}
	defer s.config.volumeLocks.Release(name)

	err = s.config.fileService.DeleteVolumePoolVolume(ctx, name)
	if err != nil {
		if file.IsNotFoundErr(err) {
			klog.Warningf("Volume Pool volume %q not found, returning success", volumeID)
			return &csi.DeleteVolumeResponse{}, nil
		}
		klog.Errorf("Failed to delete Volume Pool volume %q: %v", volumeID, err)
		return nil, volumePoolStatusError(err)
	}

	klog.Infof("Successfully deleted Volume Pool volume: %q", volumeID)
	return &csi.DeleteVolumeResponse{}, nil
}
