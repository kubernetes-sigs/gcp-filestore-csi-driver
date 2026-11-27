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
	"regexp"
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

	// Require exactly {project} as the host and
	// /{location}/{pool}/volumes/{volume} as the path, with no percent-escapes
	// in the path (such as an escaped slash or space) and no fragment, not even
	// an empty one. The ip query parameter is optional here: DeleteVolume
	// identifies the volume without it.
	if u.Host == "" || u.EscapedPath() != u.Path || u.Opaque != "" || u.User != nil ||
		strings.Contains(volumeID, "#") {
		return "", "", "", fmt.Errorf("invalid composite volume ID: %s", volumeID)
	}
	parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	if len(parts) != 4 || parts[0] == "" || parts[1] == "" || parts[2] != "volumes" || parts[3] == "" {
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

var (
	// volumePoolVolumeIDRE is the format Filestore accepts for a Volume Pool
	// volume ID: 1 to 63 lowercase letters, digits and hyphens, starting with
	// a letter and not ending with a hyphen. CO-generated names such as
	// pvc-<uuid> match it.
	volumePoolVolumeIDRE = regexp.MustCompile(`^[a-z](?:[-a-z0-9]{0,61}[a-z0-9])?$`)
	// volumePoolNameRE matches a Volume Pool resource name,
	// projects/{project}/locations/{location}/volumePools/{pool}.
	volumePoolNameRE = regexp.MustCompile(`^projects/([a-z0-9][-a-z0-9]*)/locations/([a-z][-a-z0-9]*)/volumePools/([a-z](?:[-a-z0-9]{0,61}[a-z0-9])?)$`)
)

// checkVolumePoolVolumeName returns InvalidArgument unless name is a valid
// Volume Pool volume ID. The ID becomes part of the API request path, so a
// name such as "..", or one containing "/", "?" or "#", would otherwise reach
// a different API method.
func checkVolumePoolVolumeName(name string) error {
	if !volumePoolVolumeIDRE.MatchString(name) {
		return status.Errorf(codes.InvalidArgument, "invalid volume name %q for a volume pool volume: want 1 to 63 lowercase letters, digits and hyphens, starting with a letter and not ending with a hyphen", name)
	}
	return nil
}

// canonicalVolumePoolName returns the canonical form of pool, the value of the
// volume-pool parameter, or InvalidArgument if it isn't a Volume Pool resource
// name, projects/{project}/locations/{location}/volumePools/{pool}. Surrounding
// whitespace and slashes are removed. The name must also survive the round
// trip through a volume ID, so that DeleteVolume sees the pool, and takes the
// lock, that CreateVolume used.
func canonicalVolumePoolName(pool string) (string, error) {
	canonical := strings.Trim(strings.TrimSpace(pool), "/")
	if !volumePoolNameRE.MatchString(canonical) {
		return "", status.Errorf(codes.InvalidArgument, "invalid %s parameter %q: want projects/{project}/locations/{location}/volumePools/{pool}", paramKeyVolumePool, pool)
	}
	if parent, _, _, err := parseVolumePoolVolumeID(buildVolumePoolVolumeID(canonical, "volume", "")); err != nil || parent != canonical {
		return "", status.Errorf(codes.InvalidArgument, "unsupported %s parameter %q: it can't be encoded in a volume ID", paramKeyVolumePool, pool)
	}
	return canonical, nil
}

// volumePoolVolumeName returns the resource name of the volume volumeID in the
// pool poolName. CreateVolume and DeleteVolume use it both for the API calls
// and as the key of the per-volume lock, so that both lock the same key.
func volumePoolVolumeName(poolName, volumeID string) string {
	return fmt.Sprintf("%s/volumes/%s", poolName, volumeID)
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
	if err := checkVolumePoolVolumeName(name); err != nil {
		return nil, err
	}
	poolPath, err := canonicalVolumePoolName(poolPath)
	if err != nil {
		return nil, err
	}

	klog.V(4).Infof("Creating Volume Pool volume: pool %q, requestID %q", poolPath, name)
	volumeName := volumePoolVolumeName(poolPath, name)
	// Serialize operations on the same volume, as the other CreateVolume and
	// DeleteVolume paths do. The lock is per controller pod: across replicas,
	// safety relies on Filestore keeping volume names unique. If another pod
	// creates the same volume concurrently, this pod's create gets HTTP 409
	// ALREADY_EXISTS (or 429 if that create took the last free share) and this
	// pod looks the volume up; on HTTP 409 ABORTED the caller retries.
	if acquired := s.config.volumeLocks.TryAcquire(volumeName); !acquired {
		return nil, status.Errorf(codes.Aborted, util.VolumeOperationAlreadyExistsFmt, volumeName)
	}
	defer s.config.volumeLocks.Release(volumeName)

	poolVol, created, err := s.createVolumePoolVolume(ctx, poolPath, name)
	if err != nil {
		return nil, err
	}
	// Check the volume however it was found: new, or existing.
	if err := checkVolumePoolVolumeMountable(poolVol); err != nil {
		return nil, err
	}

	compositeVolumeID := buildVolumePoolVolumeID(poolPath, name, poolVol.IpAddress)

	if created {
		klog.Infof("Successfully created Volume Pool volume: %q, Composite ID: %q, IP: %q, MountName: %q", poolVol.Name, compositeVolumeID, poolVol.IpAddress, poolVol.MountName)
	} else {
		klog.Infof("Successfully found existing Volume Pool volume: %q, Composite ID: %q, IP: %q, MountName: %q", poolVol.Name, compositeVolumeID, poolVol.IpAddress, poolVol.MountName)
	}

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
// Two CreateVolume requests with the same name for the same pool can't ask for
// different backend volumes: the driver sends no quota or ip_acls, so Filestore
// applies the pool's defaults, and the response fields (capacity, protocol,
// mount options) come from the request. So an existing volume is always
// compatible, and codes.AlreadyExists is never returned. Once the driver sends
// a quota or ip_acls, compare them here and return codes.AlreadyExists on a
// mismatch.
//
// Only an HTTP 404 from the API means that the volume doesn't exist. Any other
// error is returned as is; the caller logs and maps it.
func (s *controllerServer) lookupVolumePoolVolume(ctx context.Context, volumeName string) (*file.PoolVolume, error) {
	vol, err := s.config.fileService.GetVolumePoolVolume(ctx, volumeName)
	if err == nil {
		return vol, nil
	}
	if file.IsHTTPNotFoundErr(err) {
		return nil, nil
	}
	return nil, err
}

// createVolumePoolVolume creates the volume name in poolPath, or finds the
// existing volume of that name, and returns it whatever its state. The bool is
// true only if this call's create succeeded.
//
// The create comes first, so a new volume costs one API call. Only two create
// errors lead to a lookup (GET): HTTP 409 ALREADY_EXISTS, and any HTTP 429,
// because Filestore looks for a free share before it checks the name, so a full
// pool returns 429 for an existing name. Every other error, including HTTP 409
// ABORTED (contention), keeps its mapping without a lookup.
func (s *controllerServer) createVolumePoolVolume(ctx context.Context, poolPath, name string) (*file.PoolVolume, bool, error) {
	volumeName := volumePoolVolumeName(poolPath, name)
	poolVol, createErr := s.config.fileService.CreateVolumePoolVolume(ctx, poolPath, name)
	if createErr == nil {
		return poolVol, true, nil
	}
	switch {
	case file.IsAlreadyExistsErr(createErr):
		// The volume exists, created by an earlier attempt or a concurrent
		// request, so a failed lookup returns its own error.
		klog.Infof("Volume Pool volume %q already exists (HTTP 409 ALREADY_EXISTS), looking it up", volumeName)
		poolVol, err := s.lookupVolumePoolVolume(ctx, volumeName)
		if err != nil {
			klog.Errorf("Failed to look up Volume Pool volume %q after HTTP 409 ALREADY_EXISTS: %v", volumeName, err)
			return nil, false, volumePoolStatusError(err)
		}
		if poolVol == nil {
			// The volume was deleted between the two calls. Another
			// CreateVolume can succeed, so ask the caller to retry.
			klog.Warningf("Volume Pool volume %q was reported as existing but is not found", volumeName)
			return nil, false, status.Errorf(codes.Aborted, "volume %q was reported as existing but is not found, retry", volumeName)
		}
		return poolVol, false, nil
	case file.IsHTTPTooManyRequestsErr(createErr):
		// A full pool, or rate limiting. This also covers two pods racing for
		// the last free share: the loser's lookup usually finds the volume the
		// other created. If the lookup doesn't return the volume, for whatever
		// reason (404, 403, 429, 5xx, a timeout), the original 429 is returned,
		// as before the lookup was added, and the lookup error is logged.
		klog.Infof("Creating Volume Pool volume %q failed with HTTP 429, looking it up in case it already exists: %v", volumeName, createErr)
		poolVol, err := s.lookupVolumePoolVolume(ctx, volumeName)
		if err != nil {
			klog.Warningf("Failed to look up Volume Pool volume %q after HTTP 429 from its create, returning the 429: %v", volumeName, err)
		}
		if poolVol == nil {
			klog.Errorf("Failed to create Volume Pool volume %q in pool %s: %v", name, poolPath, createErr)
			return nil, false, volumePoolStatusError(createErr)
		}
		klog.Infof("Found existing Volume Pool volume %q after HTTP 429; the pool may be full", volumeName)
		return poolVol, false, nil
	default:
		klog.Errorf("Failed to create Volume Pool volume %q in pool %s: %v", name, poolPath, createErr)
		return nil, false, volumePoolStatusError(createErr)
	}
}

// hasVolumePoolMountPoint reports whether vol has a complete mount point.
func hasVolumePoolMountPoint(vol *file.PoolVolume) bool {
	return vol.IpAddress != "" && vol.MountName != ""
}

// checkVolumePoolVolumeMountable returns nil if vol can be mounted, and
// otherwise the error to return for it. The API reports a mount point only for
// HOT and unspecified-state volumes.
//
// A COLD or WARM volume gives FailedPrecondition, because it stays so until it
// is unfrozen or made HOT. The CO then gets no volume ID that it could delete,
// so the volume must be unfrozen, made HOT or deleted out of band. That
// happens if a volume with the requested name was frozen after an earlier
// attempt created it, or if the create returned the volume WARM, which it does
// when the share it gets allows no clients.
//
// The create returns a new volume HOT or WARM, never CREATING. A lookup reports
// CREATING only for a volume that is being restored from a backup, which this
// driver doesn't do, so CREATING gives Aborted only as a defensive case. A
// volume without a mount point gives Aborted too, so that the caller retries
// until it has one: for example, the API reports a volume whose instance is
// being recovered as unspecified-state, without a mount point. A state this
// driver doesn't know also gives Aborted, with a warning.
func checkVolumePoolVolumeMountable(vol *file.PoolVolume) error {
	if vol == nil {
		return status.Error(codes.Internal, "the Volume Pool API returned no volume")
	}
	switch vol.State {
	case "", file.VolumePoolVolumeStateUnspecified, file.VolumePoolVolumeStateHot, file.VolumePoolVolumeStateCreating,
		file.VolumePoolVolumeStateWarm, file.VolumePoolVolumeStateCold:
	default:
		klog.Warningf("Volume Pool volume %q is in unknown state %q", vol.Name, vol.State)
		return status.Errorf(codes.Aborted, "volume %q is in state %q, which can't be mounted", vol.Name, vol.State)
	}
	switch {
	case vol.State == file.VolumePoolVolumeStateCold:
		return status.Errorf(codes.FailedPrecondition, "volume %q is COLD; unfreeze it first", vol.Name)
	case vol.State == file.VolumePoolVolumeStateWarm:
		return status.Errorf(codes.FailedPrecondition, "volume %q is WARM, which cannot be mounted; make it HOT first", vol.Name)
	case vol.State == file.VolumePoolVolumeStateCreating:
		return status.Errorf(codes.Aborted, "volume %q is still being created", vol.Name)
	case !hasVolumePoolMountPoint(vol):
		// HOT, unspecified or no state reported, without a mount point.
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

	// The ID came from CreateVolume, which checked both parts. Check them
	// again, as both become part of the API request path.
	if err := checkVolumePoolVolumeName(volID); err != nil {
		return nil, err
	}
	if parent, err = canonicalVolumePoolName(parent); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid volume pool in volume ID %q: want volumepool://{project}/{location}/{pool}/volumes/{volume}", volumeID)
	}
	name := volumePoolVolumeName(parent, volID)
	if acquired := s.config.volumeLocks.TryAcquire(name); !acquired {
		return nil, status.Errorf(codes.Aborted, util.VolumeOperationAlreadyExistsFmt, name)
	}
	defer s.config.volumeLocks.Release(name)

	err = s.config.fileService.DeleteVolumePoolVolume(ctx, name)
	if err != nil {
		if file.IsNotFoundErr(err) || file.IsHTTPNotFoundErr(err) {
			klog.Warningf("Volume Pool volume %q not found, returning success", volumeID)
			return &csi.DeleteVolumeResponse{}, nil
		}
		klog.Errorf("Failed to delete Volume Pool volume %q: %v", volumeID, err)
		return nil, volumePoolStatusError(err)
	}

	klog.Infof("Successfully deleted Volume Pool volume: %q", volumeID)
	return &csi.DeleteVolumeResponse{}, nil
}
