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

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	filev1beta1 "google.golang.org/api/file/v1beta1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
	cloud "sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider"
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
	if len(parts) != 4 || parts[2] != "volumes" || u.Host == "" || parts[0] == "" || parts[1] == "" || parts[3] == "" {
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

// volumePoolsEnabled reports whether the Volume Pools feature is enabled.
func (s *controllerServer) volumePoolsEnabled() bool {
	f := s.config.features
	return f != nil && f.FeatureVolumePools != nil && f.FeatureVolumePools.Enabled
}

func (s *controllerServer) handleCreateVolumePoolVolume(ctx context.Context, req *csi.CreateVolumeRequest, poolPath string) (*csi.CreateVolumeResponse, error) {
	name := req.GetName()
	if len(name) == 0 {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume name must be provided")
	}

	klog.V(4).Infof("Creating Volume Pool volume: pool %q, requestID %q", poolPath, name)
	backupName, err := volumePoolSourceBackup(req.GetVolumeContentSource())
	if err != nil {
		return nil, err
	}

	volumeName := fmt.Sprintf("%s/volumes/%s", poolPath, name)
	if acquired := s.config.volumeLocks.TryAcquire(volumeName); !acquired {
		return nil, status.Errorf(codes.Aborted, util.VolumeOperationAlreadyExistsFmt, volumeName)
	}
	defer s.config.volumeLocks.Release(volumeName)

	// Check the source snapshot first, as the instance and multishare paths
	// do, so that a missing snapshot, or one taken in another volume pool, is
	// reported even if a volume with the requested name already exists.
	if backupName != "" {
		if err := s.checkVolumePoolSourceBackup(ctx, poolPath, backupName); err != nil {
			return nil, err
		}
	}

	// Look the volume up first, so that a retried request returns the volume
	// that an earlier attempt created.
	poolVol, err := s.lookupVolumePoolVolume(ctx, volumeName)
	if err != nil {
		return nil, err
	}
	if poolVol == nil {
		if poolVol, err = s.createVolumePoolVolume(ctx, poolPath, name, backupName); err != nil {
			return nil, err
		}
	}

	compositeVolumeID := buildVolumePoolVolumeID(poolPath, name, poolVol.IpAddress)

	klog.Infof("Volume Pool volume is ready: %q, Composite ID: %q, IP: %q, MountName: %q", poolVol.Name, compositeVolumeID, poolVol.IpAddress, poolVol.MountName)

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
			ContentSource: req.GetVolumeContentSource(),
		},
	}

	if mountOpts, ok := req.GetParameters()[paramMountOptions]; ok {
		resp.Volume.VolumeContext[attrMountOptions] = mountOpts
	}

	return resp, nil
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
		if isVolumePoolNotFound(err) {
			klog.Warningf("Volume Pool volume %q not found, returning success", volumeID)
			return &csi.DeleteVolumeResponse{}, nil
		}
		klog.Errorf("Failed to delete Volume Pool volume %q: %v", volumeID, err)
		return nil, volumePoolError(err)
	}

	klog.Infof("Successfully deleted Volume Pool volume: %q", volumeID)
	return &csi.DeleteVolumeResponse{}, nil
}

// volumePoolSourceBackup returns the backup that a new volume pool volume is
// restored from, or "" for an empty volume. Volume pool volumes can only be
// restored from backups; cloning another volume is not supported.
func volumePoolSourceBackup(src *csi.VolumeContentSource) (string, error) {
	if src == nil {
		return "", nil
	}
	snapshot := src.GetSnapshot()
	if snapshot == nil {
		return "", status.Error(codes.InvalidArgument, "volume pool volumes can only be created from a snapshot; cloning a volume is not supported")
	}
	id := snapshot.GetSnapshotId()
	if isBackup, err := util.IsBackupHandle(id); err != nil || !isBackup {
		return "", status.Errorf(codes.InvalidArgument, "unsupported volume content source %q: volume pool volumes can only be restored from backups", id)
	}
	return id, nil
}

// lookupVolumePoolVolume returns the volume named volumeName, or nil if it does
// not exist. If the volume exists but cannot be mounted, it returns Aborted
// while the volume is being created, so that the caller retries, and
// FailedPrecondition in any other state.
//
// A WARM or COLD volume keeps failing with FailedPrecondition, and the CO never
// gets a volume ID that it could delete: the volume must be unfrozen or deleted
// out of band. This can only happen if a volume with the requested name was
// frozen after an earlier attempt created it.
//
// An existing HOT volume is returned without checking which snapshot, if any,
// it was restored from, because the API does not record it. The risk is low:
// the CO uses a unique name for each volume, such as the PVC's UID.
func (s *controllerServer) lookupVolumePoolVolume(ctx context.Context, volumeName string) (*file.PoolVolume, error) {
	vol, err := s.config.fileService.GetVolumePoolVolume(ctx, volumeName)
	if err != nil {
		err = volumePoolError(err)
		if status.Code(err) == codes.NotFound {
			return nil, nil
		}
		return nil, err
	}
	if vol.State == file.VolumePoolVolumeStateCreating {
		return nil, status.Errorf(codes.Aborted, "volume %q is still being created", volumeName)
	}
	if !isVolumePoolVolumeHot(vol) {
		return nil, status.Errorf(codes.FailedPrecondition, "volume %q already exists but cannot be mounted: its state is %q", volumeName, vol.State)
	}
	return vol, nil
}

// createVolumePoolVolume creates the volume name in poolPath, restoring it from
// backupName if that is set. If another request created the volume first, it
// returns that volume instead.
func (s *controllerServer) createVolumePoolVolume(ctx context.Context, poolPath, name, backupName string) (*file.PoolVolume, error) {
	var poolVol *file.PoolVolume
	var err error
	if backupName != "" {
		poolVol, err = s.restoreVolumePoolVolume(ctx, poolPath, name, backupName)
	} else {
		poolVol, err = s.config.fileService.CreateVolumePoolVolume(ctx, poolPath, name)
		err = volumePoolError(err)
	}
	if err == nil {
		return poolVol, nil
	}
	if status.Code(err) != codes.AlreadyExists {
		klog.Errorf("Failed to create Volume Pool volume %q in pool %s: %v", name, poolPath, err)
		return nil, err
	}

	volumeName := fmt.Sprintf("%s/volumes/%s", poolPath, name)
	klog.Infof("Volume Pool volume %q was created by another request, looking it up", volumeName)
	poolVol, err = s.lookupVolumePoolVolume(ctx, volumeName)
	if err != nil {
		return nil, err
	}
	if poolVol == nil {
		return nil, status.Errorf(codes.Aborted, "volume %q was reported to exist but was not found", volumeName)
	}
	return poolVol, nil
}

// checkVolumePoolSourceBackup checks that backupName exists and is a backup of
// a volume in the volume pool poolPath. The API only restores a backup into the
// volume pool of the backup's source volume; checking here gives a clearer
// error than the API does.
func (s *controllerServer) checkVolumePoolSourceBackup(ctx context.Context, poolPath, backupName string) error {
	backup, err := s.config.fileService.GetBackup(ctx, backupName)
	if err != nil {
		if isVolumePoolNotFound(err) {
			return status.Errorf(codes.NotFound, "source snapshot %q not found", backupName)
		}
		return volumePoolError(err)
	}
	if sourcePool, _, err := splitVolumePoolVolumeName(backup.SourceVolume); err != nil || !sameVolumePoolResource(sourcePool, poolPath) {
		return status.Errorf(codes.InvalidArgument, "snapshot %q was not taken from a volume in volume pool %q (source volume %q)", backupName, poolPath, backup.SourceVolume)
	}
	return nil
}

// restoreVolumePoolVolume creates the volume name in poolPath from the backup
// backupName, which checkVolumePoolSourceBackup has checked.
func (s *controllerServer) restoreVolumePoolVolume(ctx context.Context, poolPath, name, backupName string) (*file.PoolVolume, error) {
	poolVol, err := s.config.fileService.CreateVolumePoolVolumeFromSource(ctx, poolPath, name, backupName)
	if err != nil {
		return nil, volumePoolError(err)
	}
	// The restore operation has finished, so the volume should be HOT. If it
	// is not, return Aborted: a retry finds the volume and reports its state.
	if !isVolumePoolVolumeHot(poolVol) {
		return nil, status.Errorf(codes.Aborted, "restored volume %q cannot be mounted yet: its state is %q", poolVol.Name, poolVol.State)
	}
	return poolVol, nil
}

// handleCreateVolumePoolSnapshot creates a snapshot of a volume pool volume as a
// Filestore backup of that volume.
func (s *controllerServer) handleCreateVolumePoolSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	volumeID := req.GetSourceVolumeId()
	parent, volID, _, err := parseVolumePoolVolumeID(volumeID)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to parse composite volume ID %q: %v", volumeID, err)
	}
	project, location, err := splitVolumePoolName(parent)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to parse composite volume ID %q: %v", volumeID, err)
	}
	sourceVolume := fmt.Sprintf("%s/volumes/%s", parent, volID)

	params := req.GetParameters()
	if params != nil {
		if _, err := util.IsSnapshotTypeSupported(params); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	}
	backupURI, region, err := file.CreateBackupURI(location, project, req.GetName(), "")
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	// Backups of volume pool volumes are kept in the volume pool's region.
	if backupLocation := util.GetBackupLocation(params); backupLocation != "" && backupLocation != region {
		return nil, status.Errorf(codes.InvalidArgument, "snapshot location %q must be the volume pool's region %q", backupLocation, region)
	}
	// Check the labels before any API call, as the other snapshot paths do.
	labels, err := extractBackupLabels(params, s.config.extraVolumeLabels, s.config.driver.config.Name, req.GetName())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid labels for snapshot %q: %v", req.GetName(), err)
	}

	// Lock the volume's resource name, as CreateVolume and DeleteVolume do, so
	// that operations on the same volume are serialized.
	if acquired := s.config.volumeLocks.TryAcquire(sourceVolume); !acquired {
		return nil, status.Errorf(codes.Aborted, util.VolumeOperationAlreadyExistsFmt, sourceVolume)
	}
	defer s.config.volumeLocks.Release(sourceVolume)

	existing, err := s.config.fileService.GetBackup(ctx, backupURI)
	if err == nil {
		return existingVolumePoolSnapshot(existing, volumeID, sourceVolume)
	}
	if !isVolumePoolNotFound(err) {
		return nil, volumePoolError(err)
	}

	vol, err := s.config.fileService.GetVolumePoolVolume(ctx, sourceVolume)
	if err != nil {
		return nil, volumePoolError(err)
	}
	if !isVolumePoolVolumeHot(vol) {
		return nil, status.Errorf(codes.FailedPrecondition, "volume %q is in state %q; only HOT volumes can be snapshotted", sourceVolume, vol.State)
	}

	backup, err := s.config.fileService.CreateBackup(ctx, &file.BackupInfo{
		Name:           req.GetName(),
		SourceVolumeId: volumeID,
		SourceVolume:   sourceVolume,
		BackupURI:      backupURI,
		Project:        project,
		Location:       region,
		Labels:         labels,
	})
	if err != nil {
		err = volumePoolError(err)
		if status.Code(err) == codes.AlreadyExists {
			// The backup was created after the lookup above, for example by an
			// earlier attempt whose request the API processed after the client
			// gave up. Handle it like a backup that already existed.
			klog.Infof("Snapshot %q of Volume Pool volume %q was created by another request, looking it up", req.GetName(), volumeID)
			existing, getErr := s.config.fileService.GetBackup(ctx, backupURI)
			if getErr != nil {
				return nil, volumePoolError(getErr)
			}
			return existingVolumePoolSnapshot(existing, volumeID, sourceVolume)
		}
		klog.Errorf("Failed to create snapshot %q of Volume Pool volume %q: %v", req.GetName(), volumeID, err)
		return nil, err
	}
	klog.Infof("Created snapshot %q of Volume Pool volume %q", backup.Name, volumeID)

	// Resource manager tags are not attached, as they are not attached to
	// volume pool volumes either. The backup is created in the volume pool's
	// project, which can differ from the project that the tag manager works
	// in. Log requested tags so that they are not dropped silently.
	if tags, ok := params[cloud.ParameterKeyResourceTags]; ok {
		klog.Warningf("Resource tags %q of snapshot %q are not attached: backups of Volume Pool volumes do not support resource tags", tags, req.GetName())
	}
	return volumePoolSnapshotResponse(backup, volumeID)
}

// existingVolumePoolSnapshot returns the snapshot for backup, which already
// exists with the requested name, if it was taken from sourceVolume.
func existingVolumePoolSnapshot(backup *file.Backup, volumeID, sourceVolume string) (*csi.CreateSnapshotResponse, error) {
	if !sameVolumePoolResource(backup.SourceVolume, sourceVolume) {
		return nil, status.Errorf(codes.AlreadyExists, "backup %q already exists with a different source (volume %q, instance %q); requested source volume %q", backup.Backup.Name, backup.SourceVolume, backup.SourceInstance, sourceVolume)
	}
	switch backup.Backup.State {
	case "READY":
	case "CREATING", "FINALIZING":
		return nil, status.Errorf(codes.DeadlineExceeded, "backup %q is not ready yet, current state %s", backup.Backup.Name, backup.Backup.State)
	default:
		return nil, status.Errorf(codes.Internal, "backup %q is not ready, current state %s", backup.Backup.Name, backup.Backup.State)
	}
	return volumePoolSnapshotResponse(backup.Backup, volumeID)
}

func volumePoolSnapshotResponse(backup *filev1beta1.Backup, volumeID string) (*csi.CreateSnapshotResponse, error) {
	creationTime, err := util.ParseTimestamp(backup.CreateTime)
	if err != nil {
		return nil, err
	}
	return &csi.CreateSnapshotResponse{
		Snapshot: &csi.Snapshot{
			// SizeBytes is left unset, which means unknown: a volume backup's
			// capacity_gb does not describe the volume's size.
			SnapshotId:     backup.Name,
			SourceVolumeId: volumeID,
			CreationTime:   creationTime,
			ReadyToUse:     true,
		},
	}, nil
}

// isVolumePoolVolumeHot reports whether vol is HOT, the only state in which a
// volume can be mounted or backed up. A volume from an API version that does
// not report states counts as HOT if it has a mount point.
func isVolumePoolVolumeHot(vol *file.PoolVolume) bool {
	switch vol.State {
	case "", file.VolumePoolVolumeStateUnspecified, file.VolumePoolVolumeStateHot:
		return vol.IpAddress != "" && vol.MountName != ""
	default:
		return false
	}
}

// volumePoolError returns err, which a file service call returned, as a gRPC
// status error with the API's canonical code. See file.VolumePoolStatusError.
func volumePoolError(err error) error {
	return file.VolumePoolStatusError(err)
}

// isVolumePoolNotFound reports whether err, which a file service call
// returned, means that the resource does not exist: either its code is
// NotFound, or it is an API error with reason notFound, which the instance code
// also checks for.
func isVolumePoolNotFound(err error) bool {
	if err == nil {
		return false
	}
	return file.IsNotFoundErr(err) || status.Code(volumePoolError(err)) == codes.NotFound
}

// splitVolumePoolName splits a volume pool name,
// projects/{project}/locations/{location}/volumePools/{pool}, into its project
// and location.
func splitVolumePoolName(pool string) (project, location string, err error) {
	parts := strings.Split(pool, "/")
	if len(parts) != 6 || parts[0] != "projects" || parts[2] != "locations" || parts[4] != "volumePools" || parts[1] == "" || parts[3] == "" || parts[5] == "" {
		return "", "", fmt.Errorf("invalid volume pool name %q", pool)
	}
	return parts[1], parts[3], nil
}

// splitVolumePoolVolumeName splits a volume name,
// projects/{project}/locations/{location}/volumePools/{pool}/volumes/{volume},
// into the volume pool name and the volume ID.
func splitVolumePoolVolumeName(name string) (pool, volumeID string, err error) {
	pool, volumeID, found := strings.Cut(name, "/volumes/")
	if !found || volumeID == "" || strings.Contains(volumeID, "/") {
		return "", "", fmt.Errorf("invalid volume name %q", name)
	}
	if _, _, err := splitVolumePoolName(pool); err != nil {
		return "", "", fmt.Errorf("invalid volume name %q", name)
	}
	return pool, volumeID, nil
}

// sameVolumePoolResource reports whether a and b, resource names such as
// projects/{project}/locations/{location}/volumePools/{pool}/volumes/{volume},
// name the same resource. The project is ignored, because the API can report a
// project number where the driver uses the project ID.
func sameVolumePoolResource(a, b string) bool {
	return withoutProject(a) == withoutProject(b)
}

// withoutProject returns name without its leading "projects/{project}/".
func withoutProject(name string) string {
	rest, ok := strings.CutPrefix(name, "projects/")
	if !ok {
		return name
	}
	if _, after, ok := strings.Cut(rest, "/"); ok {
		return after
	}
	return name
}
