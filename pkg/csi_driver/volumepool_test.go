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
	"maps"
	"net/http"
	"strings"
	"testing"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	filev1beta1 "google.golang.org/api/file/v1beta1"
	"google.golang.org/api/googleapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	cloud "sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/file"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/util"
)

func TestCreateVolume_VolumePool(t *testing.T) {
	ctx := context.Background()
	volumePoolPath := "projects/test-project/locations/us-central1/volumePools/my-pool"
	volumeCapabilities := []*csi.VolumeCapability{
		{
			AccessType: &csi.VolumeCapability_Mount{
				Mount: &csi.VolumeCapability_MountVolume{},
			},
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			},
		},
	}

	tests := []struct {
		name               string
		req                *csi.CreateVolumeRequest
		featureEnabled     bool
		expectErr          bool
		wantCode           codes.Code // checked when set and expectErr is true
		expectDefaultBytes bool
	}{
		{
			name: "successful allocation with volume-pool",
			req: &csi.CreateVolumeRequest{
				Name: "test-volumepool-volume",
				Parameters: map[string]string{
					paramKeyVolumePool: volumePoolPath,
				},
				VolumeCapabilities: volumeCapabilities,
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 1 * util.Gb,
				},
			},
			featureEnabled: true,
		},
		{
			name: "successful allocation with user-requested NFSv4.1",
			req: &csi.CreateVolumeRequest{
				Name: "test-volumepool-nfsv41",
				Parameters: map[string]string{
					paramKeyVolumePool: volumePoolPath,
					paramFileProtocol:  v4_1FileProtocol,
				},
				VolumeCapabilities: volumeCapabilities,
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 1 * util.Gb,
				},
			},
			featureEnabled: true,
		},
		{
			name: "omitted capacity defaulting",
			req: &csi.CreateVolumeRequest{
				Name: "test-volumepool-default-cap",
				Parameters: map[string]string{
					paramKeyVolumePool: volumePoolPath,
				},
				VolumeCapabilities: volumeCapabilities,
				CapacityRange:      nil,
			},
			featureEnabled:     true,
			expectDefaultBytes: true,
		},
		{
			name: "missing request name",
			req: &csi.CreateVolumeRequest{
				Name: "",
				Parameters: map[string]string{
					paramKeyVolumePool: volumePoolPath,
				},
				VolumeCapabilities: volumeCapabilities,
			},
			featureEnabled: true,
			expectErr:      true,
		},
		{
			name: "feature disabled",
			req: &csi.CreateVolumeRequest{
				Name: "test-volumepool-volume",
				Parameters: map[string]string{
					paramKeyVolumePool: volumePoolPath,
				},
				VolumeCapabilities: volumeCapabilities,
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 1 * util.Gb,
				},
			},
			featureEnabled: false,
			expectErr:      true,
		},
		{
			name: "pool exhausted",
			req: &csi.CreateVolumeRequest{
				Name: "test-volumepool-exhausted",
				Parameters: map[string]string{
					paramKeyVolumePool: "projects/test-project/locations/us-central1/volumePools/exhausted-pool",
				},
				VolumeCapabilities: volumeCapabilities,
			},
			featureEnabled: true,
			expectErr:      true,
			wantCode:       codes.ResourceExhausted,
		},
		{
			name: "with mount options",
			req: &csi.CreateVolumeRequest{
				Name: "test-volumepool-mount-opts",
				Parameters: map[string]string{
					paramKeyVolumePool: volumePoolPath,
					paramMountOptions:  "nconnect=8,rsize=1048576,wsize=1048576",
				},
				VolumeCapabilities: volumeCapabilities,
			},
			featureEnabled: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cs := initTestController(t).(*controllerServer)

			cs.config.features.FeatureVolumePools = &FeatureVolumePools{Enabled: tc.featureEnabled}

			resp, err := cs.CreateVolume(ctx, tc.req)
			if tc.expectErr {
				if err == nil {
					t.Errorf("expected error, got nil response: %v", resp)
				}
				if tc.wantCode != codes.OK {
					expectCode(t, err, tc.wantCode)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resp == nil || resp.Volume == nil {
				t.Fatalf("expected non-nil Volume in response")
			}

			if !isVolumePoolVolumeID(resp.Volume.VolumeId) {
				t.Errorf("expected volumepool:// composite volume ID, got %q", resp.Volume.VolumeId)
			}

			if resp.Volume.VolumeContext[attrIP] == "" {
				t.Errorf("expected ip attribute in VolumeContext")
			}

			if resp.Volume.VolumeContext[attrVolume] == "" {
				t.Errorf("expected volume attribute in VolumeContext")
			}

			expectedProtocol := v3FileProtocol
			if protocolParam, ok := tc.req.GetParameters()[paramFileProtocol]; ok && strings.ToUpper(protocolParam) == v4_1FileProtocol {
				expectedProtocol = v4_1FileProtocol
			}

			if resp.Volume.VolumeContext[attrFileProtocol] != expectedProtocol {
				t.Errorf("expected fileProtocol %q, got %q", expectedProtocol, resp.Volume.VolumeContext[attrFileProtocol])
			}

			if expectedMountOpts, ok := tc.req.GetParameters()[paramMountOptions]; ok {
				if resp.Volume.VolumeContext[attrMountOptions] != expectedMountOpts {
					t.Errorf("expected mountOptions %q, got %q", expectedMountOpts, resp.Volume.VolumeContext[attrMountOptions])
				}
			}

			// Every volume in a pool has the pool's share size, so the response
			// reports the requested size, or 0 (unknown) if none was requested.
			if got, want := resp.Volume.CapacityBytes, tc.req.GetCapacityRange().GetRequiredBytes(); got != want {
				t.Errorf("CapacityBytes = %d, want %d", got, want)
			}
			if tc.expectDefaultBytes && resp.Volume.CapacityBytes != 0 {
				t.Errorf("CapacityBytes = %d for a request without a size, want 0", resp.Volume.CapacityBytes)
			}
		})
	}
}

func TestDeleteVolume_VolumePool(t *testing.T) {
	ctx := context.Background()
	validVolumePoolID := "volumepool://test-project/us-central1/my-pool/volumes/test-vol?ip=10.1.1.1"

	tests := []struct {
		name           string
		volumeID       string
		preAllocate    bool
		featureEnabled bool
		expectErr      bool
	}{
		{
			name:           "successful deletion of volume pool volume",
			volumeID:       validVolumePoolID,
			preAllocate:    true,
			featureEnabled: true,
		},
		{
			name:           "idempotent deletion of non-existing volume",
			volumeID:       validVolumePoolID,
			preAllocate:    false,
			featureEnabled: true,
		},
		{
			name:           "feature disabled",
			volumeID:       validVolumePoolID,
			preAllocate:    true,
			featureEnabled: false,
			expectErr:      true,
		},
		{
			name:           "invalid composite volume id",
			volumeID:       "volumepool://invalid-id-format",
			featureEnabled: true,
			expectErr:      true,
		},
		{
			name:           "empty volume id",
			volumeID:       "",
			featureEnabled: true,
			expectErr:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cs := initTestController(t).(*controllerServer)

			cs.config.features.FeatureVolumePools = &FeatureVolumePools{Enabled: tc.featureEnabled}

			if tc.preAllocate {
				_, err := cs.config.fileService.CreateVolumePoolVolume(ctx, "projects/test-project/locations/us-central1/volumePools/my-pool", "test-vol")
				if err != nil {
					t.Fatalf("failed to pre-allocate volume: %v", err)
				}
			}

			req := &csi.DeleteVolumeRequest{
				VolumeId: tc.volumeID,
			}

			resp, err := cs.DeleteVolume(ctx, req)
			if tc.expectErr {
				if err == nil {
					t.Errorf("expected error, got nil response: %v", resp)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resp == nil {
				t.Fatalf("expected non-nil response")
			}
		})
	}
}

func TestVolumePoolID_ParseAndBuild(t *testing.T) {
	parentPool := "projects/my-project/locations/us-central1/volumePools/fifa-pool"
	volID := "pvc-12345-6789"
	ip := "10.10.10.10"

	builtID := buildVolumePoolVolumeID(parentPool, volID, ip)
	expectedID := "volumepool://my-project/us-central1/fifa-pool/volumes/pvc-12345-6789?ip=10.10.10.10"

	if builtID != expectedID {
		t.Errorf("buildVolumePoolVolumeID mismatch: expected %q, got %q", expectedID, builtID)
	}

	parsedParent, parsedVolID, parsedIP, err := parseVolumePoolVolumeID(builtID)
	if err != nil {
		t.Fatalf("unexpected error parsing volume ID: %v", err)
	}

	if parsedParent != parentPool {
		t.Errorf("parent mismatch: expected %q, got %q", parentPool, parsedParent)
	}
	if parsedVolID != volID {
		t.Errorf("volID mismatch: expected %q, got %q", volID, parsedVolID)
	}
	if parsedIP != ip {
		t.Errorf("IP mismatch: expected %q, got %q", ip, parsedIP)
	}
}

const (
	testVolumePool       = "projects/test-project/locations/us-central1/volumePools/my-pool"
	testSourceVolumeName = testVolumePool + "/volumes/src-vol"
	testSourceVolumeID   = "volumepool://test-project/us-central1/my-pool/volumes/src-vol?ip=10.1.1.1"
	testSnapshotName     = "snap-1"
	testBackupName       = "projects/test-project/locations/us-central1/backups/snap-1"
	testRestoredVolume   = "restored-vol"
)

// volumePoolTestService wraps the fake file service. Tests can replace single
// volume pool calls, and the wrapper counts how often each call is made.
type volumePoolTestService struct {
	file.Service
	getVolume        func(ctx context.Context, name string) (*file.PoolVolume, error)
	getBackup        func(ctx context.Context, name string) (*file.Backup, error)
	deleteVolume     func(ctx context.Context, name string) error
	createVolume     func(ctx context.Context, parentPool, volumeID string) (*file.PoolVolume, error)
	createFromSource func(ctx context.Context, parentPool, volumeID, backupName string) (*file.PoolVolume, error)
	createBackup     func(ctx context.Context, info *file.BackupInfo) (*filev1beta1.Backup, error)
	calls            map[string]int
	lastBackupInfo   *file.BackupInfo
}

func (s *volumePoolTestService) GetBackup(ctx context.Context, name string) (*file.Backup, error) {
	s.calls["GetBackup"]++
	if s.getBackup != nil {
		return s.getBackup(ctx, name)
	}
	return s.Service.GetBackup(ctx, name)
}

func (s *volumePoolTestService) DeleteVolumePoolVolume(ctx context.Context, name string) error {
	s.calls["DeleteVolumePoolVolume"]++
	if s.deleteVolume != nil {
		return s.deleteVolume(ctx, name)
	}
	return s.Service.DeleteVolumePoolVolume(ctx, name)
}

// googleAPIError returns an error like the ones that the generated Go client returns
// for an HTTP error response with canonical status st in its body.
func googleAPIError(httpCode int, st string) *googleapi.Error {
	return &googleapi.Error{
		Code:    httpCode,
		Message: strings.ToLower(st),
		Body:    fmt.Sprintf(`{"error": {"code": %d, "message": %q, "status": %q}}`, httpCode, strings.ToLower(st), st),
	}
}

func (s *volumePoolTestService) GetVolumePoolVolume(ctx context.Context, name string) (*file.PoolVolume, error) {
	s.calls["GetVolumePoolVolume"]++
	if s.getVolume != nil {
		return s.getVolume(ctx, name)
	}
	return s.Service.GetVolumePoolVolume(ctx, name)
}

func (s *volumePoolTestService) CreateVolumePoolVolume(ctx context.Context, parentPool, volumeID string) (*file.PoolVolume, error) {
	s.calls["CreateVolumePoolVolume"]++
	if s.createVolume != nil {
		return s.createVolume(ctx, parentPool, volumeID)
	}
	return s.Service.CreateVolumePoolVolume(ctx, parentPool, volumeID)
}

func (s *volumePoolTestService) CreateVolumePoolVolumeFromSource(ctx context.Context, parentPool, volumeID, backupName string) (*file.PoolVolume, error) {
	s.calls["CreateVolumePoolVolumeFromSource"]++
	if s.createFromSource != nil {
		return s.createFromSource(ctx, parentPool, volumeID, backupName)
	}
	return s.Service.CreateVolumePoolVolumeFromSource(ctx, parentPool, volumeID, backupName)
}

func (s *volumePoolTestService) CreateBackup(ctx context.Context, info *file.BackupInfo) (*filev1beta1.Backup, error) {
	s.calls["CreateBackup"]++
	s.lastBackupInfo = info
	if s.createBackup != nil {
		return s.createBackup(ctx, info)
	}
	return s.Service.CreateBackup(ctx, info)
}

// newVolumePoolTestController returns a controller with the Volume Pools feature
// enabled and a volumePoolTestService, wrapping the fake, as its file service.
// The volume pool already has the HOT volume testSourceVolumeName.
func newVolumePoolTestController(t *testing.T) (*controllerServer, *volumePoolTestService) {
	t.Helper()
	fake, err := file.NewFakeService()
	if err != nil {
		t.Fatalf("failed to create fake file service: %v", err)
	}
	if _, err := fake.CreateVolumePoolVolume(context.Background(), testVolumePool, "src-vol"); err != nil {
		t.Fatalf("failed to create source volume: %v", err)
	}
	fs := &volumePoolTestService{Service: fake, calls: map[string]int{}}
	cs := initTestController(t).(*controllerServer)
	cs.config.fileService = fs
	cs.config.features.FeatureVolumePools = &FeatureVolumePools{Enabled: true}
	return cs, fs
}

// volumeInState returns a getVolume override that reports every volume in state.
// Only HOT volumes have a mount point.
func volumeInState(state string) func(context.Context, string) (*file.PoolVolume, error) {
	return func(_ context.Context, name string) (*file.PoolVolume, error) {
		vol := &file.PoolVolume{Name: name, State: state}
		if state == file.VolumePoolVolumeStateHot {
			vol.IpAddress = "10.1.1.9"
			vol.MountName = "vol_existing"
		}
		return vol, nil
	}
}

// notFoundOnce returns a getVolume override that reports the volume missing on
// the first call and then calls next.
func notFoundOnce(next func(context.Context, string) (*file.PoolVolume, error)) func(context.Context, string) (*file.PoolVolume, error) {
	calls := 0
	return func(ctx context.Context, name string) (*file.PoolVolume, error) {
		calls++
		if calls == 1 {
			return nil, status.Errorf(codes.NotFound, "volume %q not found", name)
		}
		return next(ctx, name)
	}
}

// backupFromOtherProject returns a getBackup override that reports each backup
// of the fake as taken from the volume with the same location, volume pool and
// volume ID in the project other-project.
func backupFromOtherProject(fs *volumePoolTestService) func(context.Context, string) (*file.Backup, error) {
	return func(ctx context.Context, name string) (*file.Backup, error) {
		backup, err := fs.Service.GetBackup(ctx, name)
		if err != nil {
			return nil, err
		}
		other := *backup
		other.SourceVolume = strings.Replace(backup.SourceVolume, "projects/test-project/", "projects/other-project/", 1)
		return &other, nil
	}
}

// backupInState returns a getBackup override that reports each backup of the
// fake in state, such as CREATING. The fake's own backup is left unchanged.
func backupInState(fs *volumePoolTestService, state string) func(context.Context, string) (*file.Backup, error) {
	return func(ctx context.Context, name string) (*file.Backup, error) {
		backup, err := fs.Service.GetBackup(ctx, name)
		if err != nil {
			return nil, err
		}
		inner := *backup.Backup
		inner.State = state
		other := *backup
		other.Backup = &inner
		return &other, nil
	}
}

func expectCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if got := status.Code(err); got != want {
		t.Fatalf("got error %v (code %v), want code %v", err, got, want)
	}
}

func snapshotRequest(name, volumeID string, params map[string]string) *csi.CreateSnapshotRequest {
	return &csi.CreateSnapshotRequest{Name: name, SourceVolumeId: volumeID, Parameters: params}
}

func restoreRequest(name, snapshotID string) *csi.CreateVolumeRequest {
	return &csi.CreateVolumeRequest{
		Name:          name,
		Parameters:    map[string]string{paramKeyVolumePool: testVolumePool},
		CapacityRange: &csi.CapacityRange{RequiredBytes: 10 * util.Gb},
		VolumeContentSource: &csi.VolumeContentSource{
			Type: &csi.VolumeContentSource_Snapshot{
				Snapshot: &csi.VolumeContentSource_SnapshotSource{SnapshotId: snapshotID},
			},
		},
	}
}

func TestCreateSnapshot_VolumePool(t *testing.T) {
	ctx := context.Background()
	backupParams := map[string]string{util.VolumeSnapshotTypeKey: util.VolumeSnapshotTypeBackup}

	tests := []struct {
		name     string
		setup    func(cs *controllerServer, fs *volumePoolTestService)
		req      *csi.CreateSnapshotRequest
		wantCode codes.Code
		// wantNoAPICall is set when the driver must refuse the request
		// without asking the API for a backup.
		wantNoAPICall bool
	}{
		{
			name: "creates a backup of the volume",
			req:  snapshotRequest(testSnapshotName, testSourceVolumeID, backupParams),
		},
		{
			name: "no parameters",
			req:  snapshotRequest(testSnapshotName, testSourceVolumeID, nil),
		},
		{
			name: "location parameter is the volume pool's region",
			req: snapshotRequest(testSnapshotName, testSourceVolumeID, map[string]string{
				util.VolumeSnapshotTypeKey:     util.VolumeSnapshotTypeBackup,
				util.VolumeSnapshotLocationKey: "us-central1",
			}),
		},
		{
			name: "volume from an API version that reports no state",
			setup: func(_ *controllerServer, fs *volumePoolTestService) {
				fs.getVolume = func(_ context.Context, name string) (*file.PoolVolume, error) {
					return &file.PoolVolume{Name: name, IpAddress: "10.1.1.1", MountName: "vol_src-vol"}, nil
				}
			},
			req: snapshotRequest(testSnapshotName, testSourceVolumeID, backupParams),
		},
		{
			name: "location parameter is another region",
			req: snapshotRequest(testSnapshotName, testSourceVolumeID, map[string]string{
				util.VolumeSnapshotTypeKey:     util.VolumeSnapshotTypeBackup,
				util.VolumeSnapshotLocationKey: "us-east1",
			}),
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "unsupported snapshot type",
			req:      snapshotRequest(testSnapshotName, testSourceVolumeID, map[string]string{util.VolumeSnapshotTypeKey: "snapshot"}),
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "malformed volume ID",
			req:      snapshotRequest(testSnapshotName, "volumepool://test-project/us-central1", backupParams),
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "source volume does not exist",
			req:      snapshotRequest(testSnapshotName, "volumepool://test-project/us-central1/my-pool/volumes/missing?ip=10.1.1.1", backupParams),
			wantCode: codes.NotFound,
		},
		{
			// The API answers a COLD volume, which has no share, with a
			// misleading INTERNAL, so the driver doesn't call it.
			name: "source volume is COLD",
			setup: func(_ *controllerServer, fs *volumePoolTestService) {
				fs.getVolume = volumeInState(file.VolumePoolVolumeStateCold)
			},
			req:           snapshotRequest(testSnapshotName, testSourceVolumeID, backupParams),
			wantCode:      codes.FailedPrecondition,
			wantNoAPICall: true,
		},
		{
			// The volume becomes HOT soon, so the caller retries.
			name: "source volume is still being created",
			setup: func(_ *controllerServer, fs *volumePoolTestService) {
				fs.getVolume = volumeInState(file.VolumePoolVolumeStateCreating)
			},
			req:           snapshotRequest(testSnapshotName, testSourceVolumeID, backupParams),
			wantCode:      codes.Aborted,
			wantNoAPICall: true,
		},
		{
			// The API backs up WARM volumes.
			name: "source volume is WARM",
			setup: func(_ *controllerServer, fs *volumePoolTestService) {
				fs.getVolume = volumeInState(file.VolumePoolVolumeStateWarm)
			},
			req: snapshotRequest(testSnapshotName, testSourceVolumeID, backupParams),
		},
		{
			// A backup needs no mount point, so the API decides.
			name: "source volume in an unspecified state without a mount point",
			setup: func(_ *controllerServer, fs *volumePoolTestService) {
				fs.getVolume = volumeInState(file.VolumePoolVolumeStateUnspecified)
			},
			req: snapshotRequest(testSnapshotName, testSourceVolumeID, backupParams),
		},
		{
			name: "the API reports that the volume is busy",
			setup: func(_ *controllerServer, fs *volumePoolTestService) {
				fs.createBackup = func(context.Context, *file.BackupInfo) (*filev1beta1.Backup, error) {
					return nil, status.Error(codes.FailedPrecondition, "volume is currently in use by another operation")
				}
			},
			req:      snapshotRequest(testSnapshotName, testSourceVolumeID, backupParams),
			wantCode: codes.FailedPrecondition,
		},
		{
			name: "another operation on the volume is in progress",
			setup: func(cs *controllerServer, _ *volumePoolTestService) {
				cs.config.volumeLocks.TryAcquire(testSourceVolumeName)
			},
			req:      snapshotRequest(testSnapshotName, testSourceVolumeID, backupParams),
			wantCode: codes.Aborted,
		},
		{
			name: "Volume Pools feature disabled",
			setup: func(cs *controllerServer, _ *volumePoolTestService) {
				cs.config.features.FeatureVolumePools.Enabled = false
			},
			req:      snapshotRequest(testSnapshotName, testSourceVolumeID, backupParams),
			wantCode: codes.FailedPrecondition,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cs, fs := newVolumePoolTestController(t)
			if tc.setup != nil {
				tc.setup(cs, fs)
			}

			resp, err := cs.CreateSnapshot(ctx, tc.req)
			expectCode(t, err, tc.wantCode)
			if tc.wantNoAPICall && fs.calls["CreateBackup"] != 0 {
				t.Errorf("CreateBackup called %d times, want 0", fs.calls["CreateBackup"])
			}
			if tc.wantCode != codes.OK {
				return
			}

			snap := resp.GetSnapshot()
			if snap.GetSnapshotId() != testBackupName {
				t.Errorf("SnapshotId = %q, want %q", snap.GetSnapshotId(), testBackupName)
			}
			if snap.GetSourceVolumeId() != testSourceVolumeID {
				t.Errorf("SourceVolumeId = %q, want %q", snap.GetSourceVolumeId(), testSourceVolumeID)
			}
			if snap.GetSizeBytes() != 0 {
				t.Errorf("SizeBytes = %d, want 0 (unknown)", snap.GetSizeBytes())
			}
			if !snap.GetReadyToUse() || snap.GetCreationTime() == nil {
				t.Errorf("ReadyToUse = %v, CreationTime = %v; want a ready snapshot with a creation time", snap.GetReadyToUse(), snap.GetCreationTime())
			}

			info := fs.lastBackupInfo
			if info == nil {
				t.Fatalf("CreateBackup was not called")
			}
			if info.SourceVolume != testSourceVolumeName || info.SourceInstanceName != "" || info.SourceShare != "" {
				t.Errorf("CreateBackup got source volume %q, instance %q, share %q; want only source volume %q", info.SourceVolume, info.SourceInstanceName, info.SourceShare, testSourceVolumeName)
			}
			if info.Project != "test-project" || info.Location != "us-central1" || info.BackupURI != testBackupName {
				t.Errorf("CreateBackup got project %q, location %q, backup %q", info.Project, info.Location, info.BackupURI)
			}
			if info.Labels[tagKeySnapshotName] != testSnapshotName || info.Labels[tagKeyCreatedBy] != "test-driver" {
				t.Errorf("CreateBackup got labels %v, want %s=%s and %s=test-driver", info.Labels, tagKeySnapshotName, testSnapshotName, tagKeyCreatedBy)
			}
		})
	}
}

func TestCreateSnapshot_VolumePoolExistingBackup(t *testing.T) {
	ctx := context.Background()
	req := snapshotRequest(testSnapshotName, testSourceVolumeID, nil)

	t.Run("retry returns the existing snapshot", func(t *testing.T) {
		cs, fs := newVolumePoolTestController(t)
		first, err := cs.CreateSnapshot(ctx, req)
		expectCode(t, err, codes.OK)
		getVolumeCalls := fs.calls["GetVolumePoolVolume"]
		second, err := cs.CreateSnapshot(ctx, req)
		expectCode(t, err, codes.OK)

		if second.GetSnapshot().GetSnapshotId() != first.GetSnapshot().GetSnapshotId() {
			t.Errorf("retry returned snapshot %q, want %q", second.GetSnapshot().GetSnapshotId(), first.GetSnapshot().GetSnapshotId())
		}
		if got := fs.calls["GetVolumePoolVolume"] - getVolumeCalls; got != 0 {
			t.Errorf("retry called GetVolumePoolVolume %d times, want 0", got)
		}
		if got := fs.calls["CreateBackup"]; got != 1 {
			t.Errorf("CreateBackup called %d times, want 1", got)
		}
	})

	t.Run("existing snapshot is returned after the volume was frozen", func(t *testing.T) {
		cs, fs := newVolumePoolTestController(t)
		_, err := cs.CreateSnapshot(ctx, req)
		expectCode(t, err, codes.OK)
		fs.getVolume = volumeInState(file.VolumePoolVolumeStateCold)

		_, err = cs.CreateSnapshot(ctx, req)
		expectCode(t, err, codes.OK)
	})

	t.Run("existing snapshot is returned after the volume was deleted", func(t *testing.T) {
		cs, _ := newVolumePoolTestController(t)
		_, err := cs.CreateSnapshot(ctx, req)
		expectCode(t, err, codes.OK)
		_, err = cs.DeleteVolume(ctx, &csi.DeleteVolumeRequest{VolumeId: testSourceVolumeID})
		expectCode(t, err, codes.OK)

		_, err = cs.CreateSnapshot(ctx, req)
		expectCode(t, err, codes.OK)
	})

	t.Run("same name for a different volume", func(t *testing.T) {
		cs, fs := newVolumePoolTestController(t)
		if _, err := fs.Service.CreateVolumePoolVolume(ctx, testVolumePool, "other-vol"); err != nil {
			t.Fatalf("failed to create volume: %v", err)
		}
		_, err := cs.CreateSnapshot(ctx, req)
		expectCode(t, err, codes.OK)

		_, err = cs.CreateSnapshot(ctx, snapshotRequest(testSnapshotName, "volumepool://test-project/us-central1/my-pool/volumes/other-vol?ip=10.1.1.1", nil))
		expectCode(t, err, codes.AlreadyExists)
	})

	t.Run("same name as a backup of an instance", func(t *testing.T) {
		cs, fs := newVolumePoolTestController(t)
		_, err := fs.Service.CreateBackup(ctx, &file.BackupInfo{
			Name:               testSnapshotName,
			SourceVolumeId:     "modeInstance/us-central1-c/myinstance/myshare",
			BackupURI:          testBackupName,
			SourceInstanceName: "myinstance",
			SourceShare:        "myshare",
			Project:            "test-project",
			Location:           "us-central1",
		})
		if err != nil {
			t.Fatalf("failed to create instance backup: %v", err)
		}

		_, err = cs.CreateSnapshot(ctx, req)
		expectCode(t, err, codes.AlreadyExists)
	})

	for _, tc := range []struct {
		state    string
		wantCode codes.Code
	}{
		{state: "CREATING", wantCode: codes.DeadlineExceeded},
		{state: "FINALIZING", wantCode: codes.DeadlineExceeded},
		{state: "INVALID", wantCode: codes.Internal},
	} {
		t.Run("existing backup in state "+tc.state, func(t *testing.T) {
			cs, fs := newVolumePoolTestController(t)
			_, err := cs.CreateSnapshot(ctx, req)
			expectCode(t, err, codes.OK)
			backup, err := fs.Service.GetBackup(ctx, testBackupName)
			if err != nil {
				t.Fatalf("failed to get backup: %v", err)
			}
			backup.Backup.State = tc.state

			_, err = cs.CreateSnapshot(ctx, req)
			expectCode(t, err, tc.wantCode)
		})
	}
}

func TestDeleteSnapshot_VolumePool(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		snapshotID  string
		backupState string
		wantCode    codes.Code
	}{
		{
			name:       "deletes the backup of a volume",
			snapshotID: testBackupName,
		},
		{
			name:       "backup does not exist",
			snapshotID: "projects/test-project/locations/us-central1/backups/missing",
		},
		{
			name:        "backup is already being deleted",
			snapshotID:  testBackupName,
			backupState: "DELETING",
			wantCode:    codes.DeadlineExceeded,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cs, fs := newVolumePoolTestController(t)
			_, err := cs.CreateSnapshot(ctx, snapshotRequest(testSnapshotName, testSourceVolumeID, nil))
			expectCode(t, err, codes.OK)
			if tc.backupState != "" {
				backup, err := fs.Service.GetBackup(ctx, testBackupName)
				if err != nil {
					t.Fatalf("failed to get backup: %v", err)
				}
				backup.Backup.State = tc.backupState
			}

			_, err = cs.DeleteSnapshot(ctx, &csi.DeleteSnapshotRequest{SnapshotId: tc.snapshotID})
			expectCode(t, err, tc.wantCode)
			if tc.wantCode != codes.OK {
				return
			}
			if _, err := fs.Service.GetBackup(ctx, tc.snapshotID); !file.IsNotFoundErr(err) {
				t.Errorf("backup %q still exists after DeleteSnapshot (GetBackup error: %v)", tc.snapshotID, err)
			}
		})
	}
}

func TestCreateVolumeFromSnapshot_VolumePool(t *testing.T) {
	ctx := context.Background()
	restoredVolumeName := testVolumePool + "/volumes/" + testRestoredVolume

	tests := []struct {
		name     string
		setup    func(t *testing.T, cs *controllerServer, fs *volumePoolTestService)
		req      *csi.CreateVolumeRequest
		wantCode codes.Code
		// For successful requests: the IP the volume ID should carry, and how many
		// restores should have been started in total, including any in setup.
		wantIP       string
		wantRestores int
		// File service calls that CreateVolume must not make.
		wantNoCalls []string
	}{
		{
			name:         "restores the snapshot into a new volume",
			req:          restoreRequest(testRestoredVolume, testBackupName),
			wantIP:       "10.1.1.2",
			wantRestores: 1,
		},
		{
			name: "cloning a volume is not supported",
			req: &csi.CreateVolumeRequest{
				Name:       testRestoredVolume,
				Parameters: map[string]string{paramKeyVolumePool: testVolumePool},
				VolumeContentSource: &csi.VolumeContentSource{
					Type: &csi.VolumeContentSource_Volume{
						Volume: &csi.VolumeContentSource_VolumeSource{VolumeId: testSourceVolumeID},
					},
				},
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "snapshot ID that is not a backup",
			req:      restoreRequest(testRestoredVolume, "projects/test-project/locations/us-central1/snapshots/snap-1"),
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "malformed snapshot ID",
			req:      restoreRequest(testRestoredVolume, "snap-1"),
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "snapshot does not exist",
			req:      restoreRequest(testRestoredVolume, "projects/test-project/locations/us-central1/backups/missing"),
			wantCode: codes.NotFound,
		},
		{
			name: "snapshot of a volume in another volume pool",
			setup: func(t *testing.T, cs *controllerServer, fs *volumePoolTestService) {
				otherPool := "projects/test-project/locations/us-central1/volumePools/other-pool"
				if _, err := fs.Service.CreateVolumePoolVolume(ctx, otherPool, "other-vol"); err != nil {
					t.Fatalf("failed to create volume: %v", err)
				}
				_, err := cs.CreateSnapshot(ctx, snapshotRequest("snap-other", "volumepool://test-project/us-central1/other-pool/volumes/other-vol?ip=10.1.1.1", nil))
				expectCode(t, err, codes.OK)
			},
			req:      restoreRequest(testRestoredVolume, "projects/test-project/locations/us-central1/backups/snap-other"),
			wantCode: codes.InvalidArgument,
		},
		{
			name: "snapshot of an instance",
			setup: func(t *testing.T, _ *controllerServer, fs *volumePoolTestService) {
				_, err := fs.Service.CreateBackup(ctx, &file.BackupInfo{
					Name:               "instance-backup",
					SourceVolumeId:     "modeInstance/us-central1-c/myinstance/myshare",
					BackupURI:          "projects/test-project/locations/us-central1/backups/instance-backup",
					SourceInstanceName: "myinstance",
					SourceShare:        "myshare",
					Project:            "test-project",
					Location:           "us-central1",
				})
				if err != nil {
					t.Fatalf("failed to create instance backup: %v", err)
				}
			},
			req:      restoreRequest(testRestoredVolume, "projects/test-project/locations/us-central1/backups/instance-backup"),
			wantCode: codes.InvalidArgument,
		},
		{
			name: "retry after the volume was restored",
			setup: func(t *testing.T, cs *controllerServer, _ *volumePoolTestService) {
				_, err := cs.CreateVolume(ctx, restoreRequest(testRestoredVolume, testBackupName))
				expectCode(t, err, codes.OK)
			},
			req:          restoreRequest(testRestoredVolume, testBackupName),
			wantIP:       "10.1.1.2",
			wantRestores: 1,
		},
		{
			name: "restore is still running",
			setup: func(_ *testing.T, _ *controllerServer, fs *volumePoolTestService) {
				fs.getVolume = volumeInState(file.VolumePoolVolumeStateCreating)
			},
			req:      restoreRequest(testRestoredVolume, testBackupName),
			wantCode: codes.Aborted,
		},
		{
			name: "a volume with the same name is frozen",
			setup: func(_ *testing.T, _ *controllerServer, fs *volumePoolTestService) {
				fs.getVolume = volumeInState(file.VolumePoolVolumeStateCold)
			},
			req:      restoreRequest(testRestoredVolume, testBackupName),
			wantCode: codes.FailedPrecondition,
		},
		{
			name: "another request restored the volume first",
			setup: func(_ *testing.T, _ *controllerServer, fs *volumePoolTestService) {
				fs.getVolume = notFoundOnce(volumeInState(file.VolumePoolVolumeStateHot))
				fs.createFromSource = func(_ context.Context, _, volumeID, _ string) (*file.PoolVolume, error) {
					return nil, status.Errorf(codes.AlreadyExists, "volume %s already exists", volumeID)
				}
			},
			req:          restoreRequest(testRestoredVolume, testBackupName),
			wantIP:       "10.1.1.9",
			wantRestores: 1,
		},
		{
			name: "another request is still restoring the volume",
			setup: func(_ *testing.T, _ *controllerServer, fs *volumePoolTestService) {
				fs.getVolume = notFoundOnce(volumeInState(file.VolumePoolVolumeStateCreating))
				fs.createFromSource = func(_ context.Context, _, volumeID, _ string) (*file.PoolVolume, error) {
					return nil, status.Errorf(codes.AlreadyExists, "volume %s already exists", volumeID)
				}
			},
			req:      restoreRequest(testRestoredVolume, testBackupName),
			wantCode: codes.Aborted,
		},
		{
			name: "failed restore operation keeps its error code",
			setup: func(_ *testing.T, _ *controllerServer, fs *volumePoolTestService) {
				fs.createFromSource = func(context.Context, string, string, string) (*file.PoolVolume, error) {
					return nil, status.Error(codes.FailedPrecondition, "operation failed: backup is not in READY state")
				}
			},
			req:      restoreRequest(testRestoredVolume, testBackupName),
			wantCode: codes.FailedPrecondition,
		},
		{
			name: "restore still running after the wait returns DeadlineExceeded",
			setup: func(_ *testing.T, _ *controllerServer, fs *volumePoolTestService) {
				fs.createFromSource = func(context.Context, string, string, string) (*file.PoolVolume, error) {
					return nil, status.Error(codes.DeadlineExceeded, "operation is still running")
				}
			},
			req:      restoreRequest(testRestoredVolume, testBackupName),
			wantCode: codes.DeadlineExceeded,
		},
		{
			name: "retry after a failed restore starts a new restore",
			setup: func(t *testing.T, cs *controllerServer, fs *volumePoolTestService) {
				fs.createFromSource = func(context.Context, string, string, string) (*file.PoolVolume, error) {
					return nil, status.Error(codes.Internal, "operation failed: no available shares")
				}
				_, err := cs.CreateVolume(ctx, restoreRequest(testRestoredVolume, testBackupName))
				expectCode(t, err, codes.Internal)
				fs.createFromSource = nil
			},
			req:          restoreRequest(testRestoredVolume, testBackupName),
			wantIP:       "10.1.1.2",
			wantRestores: 2,
		},
		{
			name: "snapshot does not exist, with a canonical NOT_FOUND",
			setup: func(_ *testing.T, _ *controllerServer, fs *volumePoolTestService) {
				fs.getBackup = func(context.Context, string) (*file.Backup, error) {
					return nil, googleAPIError(http.StatusNotFound, "NOT_FOUND")
				}
			},
			req:      restoreRequest(testRestoredVolume, testBackupName),
			wantCode: codes.NotFound,
		},
		{
			name: "reading the snapshot fails with a canonical code",
			setup: func(_ *testing.T, _ *controllerServer, fs *volumePoolTestService) {
				fs.getBackup = func(context.Context, string) (*file.Backup, error) {
					return nil, googleAPIError(http.StatusBadRequest, "FAILED_PRECONDITION")
				}
			},
			req:      restoreRequest(testRestoredVolume, testBackupName),
			wantCode: codes.FailedPrecondition,
		},
		{
			name: "snapshot reports its source volume with a project number",
			setup: func(_ *testing.T, _ *controllerServer, fs *volumePoolTestService) {
				fs.getBackup = func(ctx context.Context, name string) (*file.Backup, error) {
					backup, err := fs.Service.GetBackup(ctx, name)
					if err != nil {
						return nil, err
					}
					withNumber := *backup
					withNumber.SourceVolume = "projects/123456789/locations/us-central1/volumePools/my-pool/volumes/src-vol"
					return &withNumber, nil
				}
			},
			req:          restoreRequest(testRestoredVolume, testBackupName),
			wantIP:       "10.1.1.2",
			wantRestores: 1,
		},
		{
			name: "snapshot of a volume pool with the same name in another project",
			setup: func(_ *testing.T, _ *controllerServer, fs *volumePoolTestService) {
				fs.getBackup = backupFromOtherProject(fs)
			},
			req:      restoreRequest(testRestoredVolume, testBackupName),
			wantCode: codes.InvalidArgument,
		},
		{
			name: "a volume with the requested name exists, but the snapshot is from a volume pool with the same name in another project",
			setup: func(t *testing.T, cs *controllerServer, fs *volumePoolTestService) {
				_, err := cs.CreateVolume(ctx, restoreRequest(testRestoredVolume, testBackupName))
				expectCode(t, err, codes.OK)
				fs.getBackup = backupFromOtherProject(fs)
			},
			req:      restoreRequest(testRestoredVolume, testBackupName),
			wantCode: codes.InvalidArgument,
		},
		{
			name: "snapshot is still being created",
			setup: func(_ *testing.T, _ *controllerServer, fs *volumePoolTestService) {
				fs.getBackup = backupInState(fs, "CREATING")
			},
			req:         restoreRequest(testRestoredVolume, testBackupName),
			wantCode:    codes.Unavailable,
			wantNoCalls: []string{"GetVolumePoolVolume", "CreateVolumePoolVolumeFromSource"},
		},
		{
			name: "snapshot is being finalized",
			setup: func(_ *testing.T, _ *controllerServer, fs *volumePoolTestService) {
				fs.getBackup = backupInState(fs, "FINALIZING")
			},
			req:         restoreRequest(testRestoredVolume, testBackupName),
			wantCode:    codes.Unavailable,
			wantNoCalls: []string{"GetVolumePoolVolume", "CreateVolumePoolVolumeFromSource"},
		},
		{
			name: "snapshot of a volume in another volume pool is still being created",
			setup: func(t *testing.T, cs *controllerServer, fs *volumePoolTestService) {
				otherPool := "projects/test-project/locations/us-central1/volumePools/other-pool"
				if _, err := fs.Service.CreateVolumePoolVolume(ctx, otherPool, "other-vol"); err != nil {
					t.Fatalf("failed to create volume: %v", err)
				}
				_, err := cs.CreateSnapshot(ctx, snapshotRequest("snap-other", "volumepool://test-project/us-central1/other-pool/volumes/other-vol?ip=10.1.1.1", nil))
				expectCode(t, err, codes.OK)
				fs.getBackup = backupInState(fs, "CREATING")
			},
			req:      restoreRequest(testRestoredVolume, "projects/test-project/locations/us-central1/backups/snap-other"),
			wantCode: codes.InvalidArgument,
		},
		{
			// A backup that is being deleted can't be restored, but a retry
			// must still return the volume that was restored from it.
			name: "a volume with the requested name exists, and the snapshot is being deleted",
			setup: func(t *testing.T, cs *controllerServer, fs *volumePoolTestService) {
				_, err := cs.CreateVolume(ctx, restoreRequest(testRestoredVolume, testBackupName))
				expectCode(t, err, codes.OK)
				fs.getBackup = backupInState(fs, "DELETING")
			},
			req:          restoreRequest(testRestoredVolume, testBackupName),
			wantIP:       "10.1.1.2",
			wantRestores: 1,
		},
		{
			name: "restored volume is not HOT",
			setup: func(_ *testing.T, _ *controllerServer, fs *volumePoolTestService) {
				fs.createFromSource = func(_ context.Context, parentPool, volumeID, _ string) (*file.PoolVolume, error) {
					return &file.PoolVolume{Name: parentPool + "/volumes/" + volumeID, State: file.VolumePoolVolumeStateCreating}, nil
				}
			},
			req:      restoreRequest(testRestoredVolume, testBackupName),
			wantCode: codes.Aborted,
		},
		{
			name: "restored volume has no mount point",
			setup: func(_ *testing.T, _ *controllerServer, fs *volumePoolTestService) {
				fs.createFromSource = func(_ context.Context, parentPool, volumeID, _ string) (*file.PoolVolume, error) {
					return &file.PoolVolume{Name: parentPool + "/volumes/" + volumeID, State: file.VolumePoolVolumeStateHot}, nil
				}
			},
			req:      restoreRequest(testRestoredVolume, testBackupName),
			wantCode: codes.Aborted,
		},
		{
			name: "a volume with the requested name exists, but the snapshot does not",
			setup: func(t *testing.T, cs *controllerServer, _ *volumePoolTestService) {
				_, err := cs.CreateVolume(ctx, restoreRequest(testRestoredVolume, testBackupName))
				expectCode(t, err, codes.OK)
			},
			req:      restoreRequest(testRestoredVolume, "projects/test-project/locations/us-central1/backups/missing"),
			wantCode: codes.NotFound,
		},
		{
			name: "a volume with the requested name exists, but the snapshot is from another volume pool",
			setup: func(t *testing.T, cs *controllerServer, fs *volumePoolTestService) {
				_, err := cs.CreateVolume(ctx, restoreRequest(testRestoredVolume, testBackupName))
				expectCode(t, err, codes.OK)
				otherPool := "projects/test-project/locations/us-central1/volumePools/other-pool"
				if _, err := fs.Service.CreateVolumePoolVolume(ctx, otherPool, "other-vol"); err != nil {
					t.Fatalf("failed to create volume: %v", err)
				}
				_, err = cs.CreateSnapshot(ctx, snapshotRequest("snap-other", "volumepool://test-project/us-central1/other-pool/volumes/other-vol?ip=10.1.1.1", nil))
				expectCode(t, err, codes.OK)
			},
			req:      restoreRequest(testRestoredVolume, "projects/test-project/locations/us-central1/backups/snap-other"),
			wantCode: codes.InvalidArgument,
		},
		{
			name: "another operation on the volume is in progress",
			setup: func(_ *testing.T, cs *controllerServer, _ *volumePoolTestService) {
				cs.config.volumeLocks.TryAcquire(restoredVolumeName)
			},
			req:      restoreRequest(testRestoredVolume, testBackupName),
			wantCode: codes.Aborted,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cs, fs := newVolumePoolTestController(t)
			_, err := cs.CreateSnapshot(ctx, snapshotRequest(testSnapshotName, testSourceVolumeID, nil))
			expectCode(t, err, codes.OK)
			if tc.setup != nil {
				tc.setup(t, cs, fs)
			}

			before := maps.Clone(fs.calls)
			resp, err := cs.CreateVolume(ctx, tc.req)
			expectCode(t, err, tc.wantCode)
			for _, call := range tc.wantNoCalls {
				if got := fs.calls[call] - before[call]; got != 0 {
					t.Errorf("CreateVolume called %s %d times, want 0", call, got)
				}
			}
			if tc.wantCode != codes.OK {
				return
			}

			vol := resp.GetVolume()
			if want := buildVolumePoolVolumeID(testVolumePool, testRestoredVolume, tc.wantIP); vol.GetVolumeId() != want {
				t.Errorf("VolumeId = %q, want %q", vol.GetVolumeId(), want)
			}
			if vol.GetVolumeContext()[attrIP] != tc.wantIP || vol.GetVolumeContext()[attrVolume] == "" {
				t.Errorf("VolumeContext = %v, want ip %q and a volume name", vol.GetVolumeContext(), tc.wantIP)
			}
			if got := vol.GetContentSource().GetSnapshot().GetSnapshotId(); got != testBackupName {
				t.Errorf("ContentSource snapshot = %q, want %q", got, testBackupName)
			}
			if want := int64(10 * util.Gb); vol.GetCapacityBytes() != want {
				t.Errorf("CapacityBytes = %d, want the requested %d", vol.GetCapacityBytes(), want)
			}
			if got := fs.calls["CreateVolumePoolVolumeFromSource"]; got != tc.wantRestores {
				t.Errorf("CreateVolumePoolVolumeFromSource called %d times, want %d", got, tc.wantRestores)
			}
			if got := fs.calls["CreateVolumePoolVolume"]; got != 0 {
				t.Errorf("CreateVolumePoolVolume called %d times for a restore, want 0", got)
			}
		})
	}
}

func TestCreateVolume_VolumePoolExistingVolume(t *testing.T) {
	ctx := context.Background()
	req := &csi.CreateVolumeRequest{
		Name:          "new-vol",
		Parameters:    map[string]string{paramKeyVolumePool: testVolumePool},
		CapacityRange: &csi.CapacityRange{RequiredBytes: 1 * util.Gb},
	}
	newVolumeName := testVolumePool + "/volumes/new-vol"

	tests := []struct {
		name     string
		setup    func(t *testing.T, cs *controllerServer, fs *volumePoolTestService)
		wantCode codes.Code
		wantIP   string
	}{
		{
			name: "retry after the volume was created",
			setup: func(t *testing.T, cs *controllerServer, _ *volumePoolTestService) {
				_, err := cs.CreateVolume(ctx, req)
				expectCode(t, err, codes.OK)
			},
			wantIP: "10.1.1.1",
		},
		{
			name: "another request created the volume first",
			setup: func(t *testing.T, _ *controllerServer, fs *volumePoolTestService) {
				if _, err := fs.Service.CreateVolumePoolVolume(ctx, testVolumePool, "new-vol"); err != nil {
					t.Fatalf("failed to create volume: %v", err)
				}
				fs.getVolume = notFoundOnce(fs.Service.GetVolumePoolVolume)
			},
			wantIP: "10.1.1.1",
		},
		{
			name: "volume is still being created",
			setup: func(_ *testing.T, _ *controllerServer, fs *volumePoolTestService) {
				fs.getVolume = volumeInState(file.VolumePoolVolumeStateCreating)
			},
			wantCode: codes.Aborted,
		},
		{
			name: "volume with the same name is frozen",
			setup: func(_ *testing.T, _ *controllerServer, fs *volumePoolTestService) {
				fs.getVolume = volumeInState(file.VolumePoolVolumeStateCold)
			},
			wantCode: codes.FailedPrecondition,
		},
		{
			name: "volume lookup fails",
			setup: func(_ *testing.T, _ *controllerServer, fs *volumePoolTestService) {
				fs.getVolume = func(context.Context, string) (*file.PoolVolume, error) {
					return nil, status.Error(codes.Unavailable, "service unavailable")
				}
			},
			wantCode: codes.Unavailable,
		},
		{
			name: "another operation on the volume is in progress",
			setup: func(_ *testing.T, cs *controllerServer, _ *volumePoolTestService) {
				cs.config.volumeLocks.TryAcquire(newVolumeName)
			},
			wantCode: codes.Aborted,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cs, fs := newVolumePoolTestController(t)
			tc.setup(t, cs, fs)

			resp, err := cs.CreateVolume(ctx, req)
			expectCode(t, err, tc.wantCode)
			if tc.wantCode != codes.OK {
				return
			}
			if want := buildVolumePoolVolumeID(testVolumePool, "new-vol", tc.wantIP); resp.GetVolume().GetVolumeId() != want {
				t.Errorf("VolumeId = %q, want %q", resp.GetVolume().GetVolumeId(), want)
			}
			if got := resp.GetVolume().GetCapacityBytes(); got != req.GetCapacityRange().GetRequiredBytes() {
				t.Errorf("CapacityBytes = %d, want the requested %d", got, req.GetCapacityRange().GetRequiredBytes())
			}
			if got := fs.calls["CreateVolumePoolVolume"]; got != 1 {
				t.Errorf("CreateVolumePoolVolume called %d times, want 1", got)
			}
		})
	}
}

// TestCreateVolume_VolumePoolVolumeState checks that CreateVolume returns only
// a volume that can be mounted. The same check applies however the volume was
// found: it already existed, it was created empty or restored from a snapshot,
// or another request created it first.
func TestCreateVolume_VolumePoolVolumeState(t *testing.T) {
	ctx := context.Background()
	const (
		newVol  = "new-vol"
		mountIP = "10.1.1.9"
	)

	states := []struct {
		name       string
		state      string
		mountPoint bool
		wantCode   codes.Code
	}{
		{name: "HOT", state: file.VolumePoolVolumeStateHot, mountPoint: true},
		{name: "unspecified", state: file.VolumePoolVolumeStateUnspecified, mountPoint: true},
		{name: "no state reported", state: "", mountPoint: true},
		// For example, the volume's instance is being recovered.
		{name: "unspecified without a mount point", state: file.VolumePoolVolumeStateUnspecified, wantCode: codes.Aborted},
		{name: "HOT without a mount point", state: file.VolumePoolVolumeStateHot, wantCode: codes.Aborted},
		{name: "CREATING", state: file.VolumePoolVolumeStateCreating, wantCode: codes.Aborted},
		{name: "WARM", state: file.VolumePoolVolumeStateWarm, wantCode: codes.FailedPrecondition},
		{name: "COLD", state: file.VolumePoolVolumeStateCold, wantCode: codes.FailedPrecondition},
	}
	paths := []struct {
		name    string
		restore bool
		// setup makes the file service return vol(name) for the requested
		// volume on this path.
		setup func(fs *volumePoolTestService, vol func(name string) *file.PoolVolume)
	}{
		{
			name: "existing volume",
			setup: func(fs *volumePoolTestService, vol func(string) *file.PoolVolume) {
				fs.getVolume = func(_ context.Context, name string) (*file.PoolVolume, error) {
					return vol(name), nil
				}
			},
		},
		{
			name: "new empty volume",
			setup: func(fs *volumePoolTestService, vol func(string) *file.PoolVolume) {
				fs.createVolume = func(_ context.Context, parentPool, volumeID string) (*file.PoolVolume, error) {
					return vol(parentPool + "/volumes/" + volumeID), nil
				}
			},
		},
		{
			name:    "restored volume",
			restore: true,
			setup: func(fs *volumePoolTestService, vol func(string) *file.PoolVolume) {
				fs.createFromSource = func(_ context.Context, parentPool, volumeID, _ string) (*file.PoolVolume, error) {
					return vol(parentPool + "/volumes/" + volumeID), nil
				}
			},
		},
		{
			name: "volume created by another request first",
			setup: func(fs *volumePoolTestService, vol func(string) *file.PoolVolume) {
				fs.getVolume = notFoundOnce(func(_ context.Context, name string) (*file.PoolVolume, error) {
					return vol(name), nil
				})
				fs.createVolume = func(_ context.Context, _, volumeID string) (*file.PoolVolume, error) {
					return nil, status.Errorf(codes.AlreadyExists, "volume %s already exists", volumeID)
				}
			},
		},
	}

	for _, p := range paths {
		for _, st := range states {
			t.Run(p.name+"/"+st.name, func(t *testing.T) {
				cs, fs := newVolumePoolTestController(t)
				req := &csi.CreateVolumeRequest{
					Name:       newVol,
					Parameters: map[string]string{paramKeyVolumePool: testVolumePool},
				}
				if p.restore {
					_, err := cs.CreateSnapshot(ctx, snapshotRequest(testSnapshotName, testSourceVolumeID, nil))
					expectCode(t, err, codes.OK)
					req = restoreRequest(newVol, testBackupName)
				}
				p.setup(fs, func(name string) *file.PoolVolume {
					vol := &file.PoolVolume{Name: name, State: st.state}
					if st.mountPoint {
						vol.IpAddress = mountIP
						vol.MountName = "vol_" + newVol
					}
					return vol
				})

				resp, err := cs.CreateVolume(ctx, req)
				expectCode(t, err, st.wantCode)
				if st.wantCode != codes.OK {
					return
				}
				if want := buildVolumePoolVolumeID(testVolumePool, newVol, mountIP); resp.GetVolume().GetVolumeId() != want {
					t.Errorf("VolumeId = %q, want %q", resp.GetVolume().GetVolumeId(), want)
				}
			})
		}
	}
}

// TestCreateVolume_VolumePoolParameter checks that CreateVolume refuses a
// malformed volume-pool parameter before calling the API, and that a valid one
// gives a volume ID that parses back to the same pool.
func TestCreateVolume_VolumePoolParameter(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		pool     string
		wantCode codes.Code
	}{
		{name: "project number", pool: "projects/123456789/locations/us-central1/volumePools/my-pool"},
		{name: "zone", pool: "projects/test-project/locations/us-central1-c/volumePools/my-pool"},
		{name: "trailing slash", pool: testVolumePool + "/", wantCode: codes.InvalidArgument},
		{name: "leading slash", pool: "/" + testVolumePool, wantCode: codes.InvalidArgument},
		{name: "pool ID only", pool: "my-pool", wantCode: codes.InvalidArgument},
		{name: "wrong collection name", pool: "projects/test-project/locations/us-central1/volumepools/my-pool", wantCode: codes.InvalidArgument},
		{name: "no location", pool: "projects/test-project/volumePools/my-pool", wantCode: codes.InvalidArgument},
		{name: "extra segment", pool: testVolumePool + "/volumes", wantCode: codes.InvalidArgument},
		{name: "empty project", pool: "projects//locations/us-central1/volumePools/my-pool", wantCode: codes.InvalidArgument},
		{name: "API URL", pool: "https://file.googleapis.com/v1beta1/" + testVolumePool, wantCode: codes.InvalidArgument},
		// These have the right shape, but a volume ID can't hold them.
		{name: "domain-scoped project", pool: "projects/google.com:test-project/locations/us-central1/volumePools/my-pool", wantCode: codes.InvalidArgument},
		{name: "escaped character", pool: "projects/test-project/locations/us-central1/volumePools/my%2Dpool", wantCode: codes.InvalidArgument},
		{name: "query character", pool: "projects/test-project/locations/us-central1/volumePools/my?pool", wantCode: codes.InvalidArgument},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cs, fs := newVolumePoolTestController(t)
			req := &csi.CreateVolumeRequest{
				Name:       "new-vol",
				Parameters: map[string]string{paramKeyVolumePool: tc.pool},
			}

			resp, err := cs.CreateVolume(ctx, req)
			expectCode(t, err, tc.wantCode)
			if tc.wantCode != codes.OK {
				if len(fs.calls) != 0 {
					t.Errorf("file service calls = %v, want none", fs.calls)
				}
				return
			}
			id := resp.GetVolume().GetVolumeId()
			if parent, volID, _, err := parseVolumePoolVolumeID(id); err != nil || parent != tc.pool || volID != "new-vol" {
				t.Errorf("volume ID %q parses to pool %q, volume %q, error %v; want pool %q, volume %q", id, parent, volID, err, tc.pool, "new-vol")
			}
		})
	}
}

func TestSplitVolumePoolVolumeName(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantPool   string
		wantVolume string
		wantErr    bool
	}{
		{
			name:       "valid name",
			input:      testSourceVolumeName,
			wantPool:   testVolumePool,
			wantVolume: "src-vol",
		},
		{name: "empty", input: "", wantErr: true},
		{name: "instance", input: "projects/test-project/locations/us-central1/instances/myinstance", wantErr: true},
		{name: "missing volume ID", input: testVolumePool + "/volumes/", wantErr: true},
		{name: "extra segment", input: testSourceVolumeName + "/extra", wantErr: true},
		{name: "not a volume pool", input: "projects/test-project/locations/us-central1/other/my-pool/volumes/v", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pool, volume, err := splitVolumePoolVolumeName(tc.input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("splitVolumePoolVolumeName(%q) error = %v, wantErr %v", tc.input, err, tc.wantErr)
			}
			if pool != tc.wantPool || volume != tc.wantVolume {
				t.Errorf("splitVolumePoolVolumeName(%q) = %q, %q; want %q, %q", tc.input, pool, volume, tc.wantPool, tc.wantVolume)
			}
		})
	}
}

// TestSameVolumePoolResource checks that projects are compared unless one name
// has a project number and the other a project ID.
func TestSameVolumePoolResource(t *testing.T) {
	const volume = "locations/us-central1/volumePools/agents/volumes/v1"
	tests := []struct {
		name string
		a, b string
		want bool
	}{
		{name: "same project ID", a: "projects/team-prod/" + volume, b: "projects/team-prod/" + volume, want: true},
		{name: "different project IDs", a: "projects/team-prod/" + volume, b: "projects/team-staging/" + volume},
		{name: "project IDs with digits", a: "projects/team-2/" + volume, b: "projects/team-3/" + volume},
		{name: "project number and project ID", a: "projects/123456789/" + volume, b: "projects/team-prod/" + volume, want: true},
		{name: "project ID and project number", a: "projects/team-prod/" + volume, b: "projects/123456789/" + volume, want: true},
		{name: "same project number", a: "projects/123456789/" + volume, b: "projects/123456789/" + volume, want: true},
		{name: "different project numbers", a: "projects/123456789/" + volume, b: "projects/987654321/" + volume},
		{name: "different volume", a: "projects/team-prod/" + volume, b: "projects/team-prod/locations/us-central1/volumePools/agents/volumes/v2"},
		{name: "different volume pool, project number", a: "projects/123456789/" + volume, b: "projects/team-prod/locations/us-central1/volumePools/other/volumes/v1"},
		{name: "different location", a: "projects/team-prod/" + volume, b: "projects/team-prod/locations/us-east1/volumePools/agents/volumes/v1"},
		{name: "volume pools, project number and project ID", a: "projects/123456789/locations/us-central1/volumePools/agents", b: "projects/team-prod/locations/us-central1/volumePools/agents", want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := sameVolumePoolResource(tc.a, tc.b); got != tc.want {
				t.Errorf("sameVolumePoolResource(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestVolumePoolsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		features *GCFSDriverFeatureOptions
		want     bool
	}{
		{name: "no feature options", features: nil, want: false},
		{name: "no Volume Pools option", features: &GCFSDriverFeatureOptions{}, want: false},
		{name: "disabled", features: &GCFSDriverFeatureOptions{FeatureVolumePools: &FeatureVolumePools{Enabled: false}}, want: false},
		{name: "enabled", features: &GCFSDriverFeatureOptions{FeatureVolumePools: &FeatureVolumePools{Enabled: true}}, want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cs := &controllerServer{config: &controllerServerConfig{features: tc.features}}
			if got := cs.volumePoolsEnabled(); got != tc.want {
				t.Errorf("volumePoolsEnabled() = %v, want %v", got, tc.want)
			}
		})
	}
}

// Without feature options, every Volume Pool RPC returns FailedPrecondition
// instead of panicking.
func TestVolumePoolRPCs_NoFeatureOptions(t *testing.T) {
	ctx := context.Background()
	cs, _ := newVolumePoolTestController(t)
	cs.config.features = nil

	_, err := cs.CreateVolume(ctx, &csi.CreateVolumeRequest{
		Name:       "new-vol",
		Parameters: map[string]string{paramKeyVolumePool: testVolumePool},
	})
	expectCode(t, err, codes.FailedPrecondition)

	_, err = cs.CreateVolume(ctx, restoreRequest("restored-vol", testBackupName))
	expectCode(t, err, codes.FailedPrecondition)

	_, err = cs.DeleteVolume(ctx, &csi.DeleteVolumeRequest{VolumeId: testSourceVolumeID})
	expectCode(t, err, codes.FailedPrecondition)

	_, err = cs.CreateSnapshot(ctx, snapshotRequest(testSnapshotName, testSourceVolumeID, nil))
	expectCode(t, err, codes.FailedPrecondition)
}

// Resource tags are not attached to backups of volume pool volumes, as they are
// not attached to volume pool volumes. The snapshot still succeeds.
func TestCreateSnapshot_VolumePoolResourceTags(t *testing.T) {
	cs, _ := newVolumePoolTestController(t)
	tagManager := cloud.NewFakeTagManager()
	cs.config.tagManager = tagManager

	_, err := cs.CreateSnapshot(context.Background(), snapshotRequest(testSnapshotName, testSourceVolumeID, map[string]string{
		util.VolumeSnapshotTypeKey:     util.VolumeSnapshotTypeBackup,
		cloud.ParameterKeyResourceTags: "test-project/env/prod",
	}))
	expectCode(t, err, codes.OK)
	if len(tagManager.Calls) != 0 {
		t.Errorf("tag manager was called %d times, want 0: %v", len(tagManager.Calls), tagManager.Calls)
	}
}

func TestDeleteVolume_VolumePoolErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode codes.Code
	}{
		{name: "volume is busy", err: status.Error(codes.FailedPrecondition, "delete volume: volume is busy"), wantCode: codes.FailedPrecondition},
		{name: "API error with a canonical code", err: googleAPIError(http.StatusBadRequest, "FAILED_PRECONDITION"), wantCode: codes.FailedPrecondition},
		{name: "volume does not exist", err: status.Error(codes.NotFound, "delete volume: not found"), wantCode: codes.OK},
		{name: "API error NOT_FOUND", err: googleAPIError(http.StatusNotFound, "NOT_FOUND"), wantCode: codes.OK},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cs, fs := newVolumePoolTestController(t)
			fs.deleteVolume = func(context.Context, string) error { return tc.err }

			_, err := cs.DeleteVolume(context.Background(), &csi.DeleteVolumeRequest{VolumeId: testSourceVolumeID})
			expectCode(t, err, tc.wantCode)
		})
	}
}

// CreateVolume, DeleteVolume and CreateSnapshot lock the same key, the
// volume's resource name, so they never run at the same time on one volume.
func TestVolumePoolRPCs_ShareVolumeLock(t *testing.T) {
	ctx := context.Background()
	cs, fs := newVolumePoolTestController(t)
	cs.config.volumeLocks.TryAcquire(testSourceVolumeName)

	_, err := cs.DeleteVolume(ctx, &csi.DeleteVolumeRequest{VolumeId: testSourceVolumeID})
	expectCode(t, err, codes.Aborted)
	_, err = cs.CreateSnapshot(ctx, snapshotRequest(testSnapshotName, testSourceVolumeID, nil))
	expectCode(t, err, codes.Aborted)
	_, err = cs.CreateVolume(ctx, &csi.CreateVolumeRequest{
		Name:       "src-vol",
		Parameters: map[string]string{paramKeyVolumePool: testVolumePool},
	})
	expectCode(t, err, codes.Aborted)
	if got := fs.calls["DeleteVolumePoolVolume"] + fs.calls["CreateBackup"] + fs.calls["GetVolumePoolVolume"]; got != 0 {
		t.Errorf("file service called %d times while the volume was locked, want 0", got)
	}

	cs.config.volumeLocks.Release(testSourceVolumeName)
	_, err = cs.DeleteVolume(ctx, &csi.DeleteVolumeRequest{VolumeId: testSourceVolumeID})
	expectCode(t, err, codes.OK)
	if !cs.config.volumeLocks.TryAcquire(testSourceVolumeName) {
		t.Errorf("DeleteVolume did not release the volume lock")
	}
}

func TestCreateSnapshot_VolumePoolChecks(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid labels are rejected before any API call", func(t *testing.T) {
		cs, fs := newVolumePoolTestController(t)
		_, err := cs.CreateSnapshot(ctx, snapshotRequest(testSnapshotName, testSourceVolumeID, map[string]string{
			util.VolumeSnapshotTypeKey: util.VolumeSnapshotTypeBackup,
			ParameterKeyLabels:         "not-a-label-list",
		}))
		expectCode(t, err, codes.InvalidArgument)
		if got := fs.calls["GetBackup"] + fs.calls["GetVolumePoolVolume"] + fs.calls["CreateBackup"]; got != 0 {
			t.Errorf("file service called %d times, want 0", got)
		}
	})

	t.Run("source volume ID with extra path segments", func(t *testing.T) {
		cs, _ := newVolumePoolTestController(t)
		_, err := cs.CreateSnapshot(ctx, snapshotRequest(testSnapshotName, "volumepool://test-project/us-central1/my-pool/volumes/src-vol/extra?ip=10.1.1.1", nil))
		expectCode(t, err, codes.InvalidArgument)
	})

	t.Run("existing backup reports the source volume with a project number", func(t *testing.T) {
		cs, fs := newVolumePoolTestController(t)
		fs.getBackup = func(context.Context, string) (*file.Backup, error) {
			return &file.Backup{
				Backup:       &filev1beta1.Backup{Name: testBackupName, State: "READY", CreateTime: "2020-10-02T15:01:23Z"},
				SourceVolume: "projects/123456789/locations/us-central1/volumePools/my-pool/volumes/src-vol",
			}, nil
		}
		resp, err := cs.CreateSnapshot(ctx, snapshotRequest(testSnapshotName, testSourceVolumeID, nil))
		expectCode(t, err, codes.OK)
		if resp.GetSnapshot().GetSnapshotId() != testBackupName {
			t.Errorf("SnapshotId = %q, want %q", resp.GetSnapshot().GetSnapshotId(), testBackupName)
		}
	})

	t.Run("existing backup of a volume with the same name in another project", func(t *testing.T) {
		cs, fs := newVolumePoolTestController(t)
		fs.getBackup = func(context.Context, string) (*file.Backup, error) {
			return &file.Backup{
				Backup:       &filev1beta1.Backup{Name: testBackupName, State: "READY", CreateTime: "2020-10-02T15:01:23Z"},
				SourceVolume: "projects/other-project/locations/us-central1/volumePools/my-pool/volumes/src-vol",
			}, nil
		}
		_, err := cs.CreateSnapshot(ctx, snapshotRequest(testSnapshotName, testSourceVolumeID, nil))
		expectCode(t, err, codes.AlreadyExists)
	})

	for _, tc := range []struct {
		name         string
		sourceVolume string
		wantCode     codes.Code
	}{
		{name: "backup created by another request after the lookup", sourceVolume: testSourceVolumeName, wantCode: codes.OK},
		{name: "backup of another volume created after the lookup", sourceVolume: testVolumePool + "/volumes/other-vol", wantCode: codes.AlreadyExists},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cs, fs := newVolumePoolTestController(t)
			if _, err := fs.Service.CreateVolumePoolVolume(ctx, testVolumePool, "other-vol"); err != nil {
				t.Fatalf("failed to create volume: %v", err)
			}
			// The backup does not exist when the handler first looks it up, and
			// appears before CreateBackup runs.
			lookups := 0
			fs.getBackup = func(ctx context.Context, name string) (*file.Backup, error) {
				lookups++
				if lookups == 1 {
					if _, err := fs.Service.CreateBackup(ctx, &file.BackupInfo{
						Name:         testSnapshotName,
						SourceVolume: tc.sourceVolume,
						BackupURI:    testBackupName,
						Project:      "test-project",
						Location:     "us-central1",
					}); err != nil {
						t.Fatalf("failed to create backup: %v", err)
					}
					return nil, status.Errorf(codes.NotFound, "backup %q not found", name)
				}
				return fs.Service.GetBackup(ctx, name)
			}

			_, err := cs.CreateSnapshot(ctx, snapshotRequest(testSnapshotName, testSourceVolumeID, nil))
			expectCode(t, err, tc.wantCode)
			if got := fs.calls["CreateBackup"]; got != 1 {
				t.Errorf("CreateBackup called %d times, want 1", got)
			}
		})
	}
}

func TestParseVolumePoolVolumeID_Invalid(t *testing.T) {
	for _, id := range []string{
		"volumepool://test-project/us-central1/my-pool/volumes/vol/extra?ip=10.1.1.1",
		"volumepool://test-project/us-central1/my-pool/volumes/?ip=10.1.1.1",
		"volumepool://test-project/us-central1/my-pool/other/vol?ip=10.1.1.1",
		"volumepool:///us-central1/my-pool/volumes/vol?ip=10.1.1.1",
		"volumepool://test-project/us-central1/volumes/vol?ip=10.1.1.1",
		"volumepool://invalid-id-format",
	} {
		if _, _, _, err := parseVolumePoolVolumeID(id); err == nil {
			t.Errorf("parseVolumePoolVolumeID(%q) succeeded, want an error", id)
		}
	}
}
