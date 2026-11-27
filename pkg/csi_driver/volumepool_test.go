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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/api/googleapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
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
		expectCode         codes.Code // checked when expectErr is set and this is not codes.OK
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
			expectCode:     codes.ResourceExhausted,
		},
		{
			name: "pool contention (HTTP 409 ABORTED) is retryable",
			req: &csi.CreateVolumeRequest{
				Name: "test-volumepool-contended",
				Parameters: map[string]string{
					paramKeyVolumePool: "projects/test-project/locations/us-central1/volumePools/" + file.FakeVolumePoolContended + "-pool",
				},
				VolumeCapabilities: volumeCapabilities,
			},
			featureEnabled: true,
			expectErr:      true,
			expectCode:     codes.Aborted,
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
				if tc.expectCode != codes.OK {
					if got := status.Code(err); got != tc.expectCode {
						t.Errorf("got error code %v, want %v (error: %v)", got, tc.expectCode, err)
					}
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

			// Capacity validation is intentionally skipped as we bypass K8s standard PVC capacity tests for backend pool agent operations.
		})
	}
}

// volumePoolTestService wraps the fake file service. Tests can replace single
// Volume Pool calls, and the wrapper counts how often each call is made.
type volumePoolTestService struct {
	file.Service
	getVolume    func(ctx context.Context, name string) (*file.PoolVolume, error)
	createVolume func(ctx context.Context, parentPool, volumeID string) (*file.PoolVolume, error)
	deleteVolume func(ctx context.Context, name string) error
	mu           sync.Mutex
	calls        map[string]int
	createErr    error // the error of the last CreateVolumePoolVolume call
}

func (s *volumePoolTestService) count(call string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls[call]++
}

func (s *volumePoolTestService) GetVolumePoolVolume(ctx context.Context, name string) (*file.PoolVolume, error) {
	s.count("GetVolumePoolVolume")
	if s.getVolume != nil {
		return s.getVolume(ctx, name)
	}
	return s.Service.GetVolumePoolVolume(ctx, name)
}

func (s *volumePoolTestService) CreateVolumePoolVolume(ctx context.Context, parentPool, volumeID string) (*file.PoolVolume, error) {
	s.count("CreateVolumePoolVolume")
	create := s.Service.CreateVolumePoolVolume
	if s.createVolume != nil {
		create = s.createVolume
	}
	vol, err := create(ctx, parentPool, volumeID)
	s.mu.Lock()
	s.createErr = err
	s.mu.Unlock()
	return vol, err
}

func (s *volumePoolTestService) DeleteVolumePoolVolume(ctx context.Context, name string) error {
	s.count("DeleteVolumePoolVolume")
	if s.deleteVolume != nil {
		return s.deleteVolume(ctx, name)
	}
	return s.Service.DeleteVolumePoolVolume(ctx, name)
}

// newVolumePoolTestController returns a controller with the Volume Pools feature
// enabled and a volumePoolTestService, wrapping the fake, as its file service.
func newVolumePoolTestController(t *testing.T) (*controllerServer, *volumePoolTestService) {
	t.Helper()
	cs := initTestController(t).(*controllerServer)
	cs.config.features.FeatureVolumePools = &FeatureVolumePools{Enabled: true}
	fs := &volumePoolTestService{Service: cs.config.fileService, calls: map[string]int{}}
	cs.config.fileService = fs
	return cs, fs
}

// newVolumePoolCreateRequest returns a CreateVolume request for the volume name
// in the Volume Pool pool, with one single-node-writer mount capability.
func newVolumePoolCreateRequest(name, pool string) *csi.CreateVolumeRequest {
	return &csi.CreateVolumeRequest{
		Name:       name,
		Parameters: map[string]string{paramKeyVolumePool: pool},
		VolumeCapabilities: []*csi.VolumeCapability{{
			AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
			AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
		}},
	}
}

func TestCreateVolume_VolumePoolIdempotent(t *testing.T) {
	ctx := context.Background()
	const poolPath = "projects/test-project/locations/us-central1/volumePools/my-pool"
	const volName = "test-volumepool-idempotent"
	volumePath := poolPath + "/volumes/" + volName
	newReq := func(pool string) *csi.CreateVolumeRequest {
		req := newVolumePoolCreateRequest(volName, pool)
		req.Parameters[paramFileProtocol] = v4_1FileProtocol
		req.Parameters[paramMountOptions] = "nconnect=8"
		req.CapacityRange = &csi.CapacityRange{RequiredBytes: 1 * util.Gb}
		return req
	}
	wantContext := map[string]string{attrIP: "10.1.1.1", attrVolume: "vol_" + volName, attrFileProtocol: v4_1FileProtocol, attrMountOptions: "nconnect=8"}

	const fullPool = "projects/test-project/locations/us-central1/volumePools/" + file.FakeVolumePoolExhausted + "-pool"
	// The create's 429 has its own message, so that the tests can tell it from
	// the GET's errors.
	const fullPoolMsg = "no free share in the pool"
	tooManyRequests := func(context.Context, string, string) (*file.PoolVolume, error) {
		return nil, file.FakeVolumePoolError(http.StatusTooManyRequests, "RESOURCE_EXHAUSTED", fullPoolMsg)
	}
	// getFailsAfter429 makes the create fail with 429 and the GET with getErr.
	getFailsAfter429 := func(getErr error) func(*testing.T, *controllerServer, *volumePoolTestService) {
		return func(_ *testing.T, _ *controllerServer, fs *volumePoolTestService) {
			fs.createVolume = tooManyRequests
			fs.getVolume = func(context.Context, string) (*file.PoolVolume, error) { return nil, getErr }
		}
	}
	alreadyExists := func(context.Context, string, string) (*file.PoolVolume, error) {
		return nil, file.FakeVolumePoolError(http.StatusConflict, "ALREADY_EXISTS", "volume already exists")
	}
	createFirst := func(t *testing.T, cs *controllerServer, _ *volumePoolTestService) {
		if _, err := cs.CreateVolume(ctx, newReq(poolPath)); err != nil {
			t.Fatalf("first CreateVolume: %v", err)
		}
	}

	tests := []struct {
		name string
		// setup runs before the CreateVolume under test.
		setup      func(t *testing.T, cs *controllerServer, fs *volumePoolTestService)
		pool       string
		wantCode   codes.Code
		wantMsg    string // substring of the error message, if set
		wantCreate int    // CreateVolumePoolVolume calls made by the CreateVolume under test
		wantGet    int    // GetVolumePoolVolume calls made by the CreateVolume under test
		// wantCreateHTTP, if set, is the HTTP code of the create's error, to
		// show which path the fake took.
		wantCreateHTTP int
	}{
		{
			name:       "new volume is created without a lookup",
			wantCreate: 1,
		},
		{
			name:           "existing volume is looked up after ALREADY_EXISTS",
			setup:          createFirst,
			wantCreate:     1,
			wantGet:        1,
			wantCreateHTTP: http.StatusConflict,
		},
		{
			// Filestore looks for a free share before it checks the name, so in
			// a full pool a create for an existing name gets HTTP 429
			// RESOURCE_EXHAUSTED instead of ALREADY_EXISTS.
			name: "existing volume in a full pool is looked up after 429",
			setup: func(t *testing.T, cs *controllerServer, fs *volumePoolTestService) {
				createFirst(t, cs, fs)
				fs.createVolume = tooManyRequests
			},
			wantCreate: 1,
			wantGet:    1,
		},
		{
			// The same through the fake, which, like Filestore, fails a create
			// in a full pool with 429 even for an existing name.
			name: "existing volume in a pool the fake reports as full",
			setup: func(t *testing.T, _ *controllerServer, fs *volumePoolTestService) {
				if err := file.FakeAddVolumePoolVolume(fs.Service, &file.PoolVolume{
					Name:      fullPool + "/volumes/" + volName,
					IpAddress: "10.1.1.1",
					MountName: "vol_" + volName,
					State:     file.VolumePoolVolumeStateHot,
				}); err != nil {
					t.Fatalf("failed to add volume: %v", err)
				}
			},
			pool:           fullPool,
			wantCreate:     1,
			wantGet:        1,
			wantCreateHTTP: http.StatusTooManyRequests,
		},
		{
			// The 429 is returned, not the lookup's 404.
			name:           "new volume in a full pool keeps the 429 mapping",
			pool:           fullPool,
			wantCode:       codes.ResourceExhausted,
			wantMsg:        "no available volumes in the pool",
			wantCreate:     1,
			wantGet:        1,
			wantCreateHTTP: http.StatusTooManyRequests,
		},
		{
			name:       "GET 403 after 429 returns the 429",
			setup:      getFailsAfter429(file.FakeVolumePoolError(http.StatusForbidden, "PERMISSION_DENIED", "permission denied")),
			wantCode:   codes.ResourceExhausted,
			wantMsg:    fullPoolMsg,
			wantCreate: 1,
			wantGet:    1,
		},
		{
			name:       "GET 500 after 429 returns the 429",
			setup:      getFailsAfter429(file.FakeVolumePoolError(http.StatusInternalServerError, "INTERNAL", "internal error")),
			wantCode:   codes.ResourceExhausted,
			wantMsg:    fullPoolMsg,
			wantCreate: 1,
			wantGet:    1,
		},
		{
			name:       "GET 503 after 429 returns the 429",
			setup:      getFailsAfter429(file.FakeVolumePoolError(http.StatusServiceUnavailable, "UNAVAILABLE", "service unavailable")),
			wantCode:   codes.ResourceExhausted,
			wantMsg:    fullPoolMsg,
			wantCreate: 1,
			wantGet:    1,
		},
		{
			name:       "GET 429 after 429 returns the 429",
			setup:      getFailsAfter429(file.FakeVolumePoolError(http.StatusTooManyRequests, "RESOURCE_EXHAUSTED", "rate limit exceeded")),
			wantCode:   codes.ResourceExhausted,
			wantMsg:    fullPoolMsg,
			wantCreate: 1,
			wantGet:    1,
		},
		{
			name:       "GET timeout after 429 returns the 429",
			setup:      getFailsAfter429(fmt.Errorf("get volume: %w", context.DeadlineExceeded)),
			wantCode:   codes.ResourceExhausted,
			wantMsg:    fullPoolMsg,
			wantCreate: 1,
			wantGet:    1,
		},
		{
			name:       "GET canceled after 429 returns the 429",
			setup:      getFailsAfter429(context.Canceled),
			wantCode:   codes.ResourceExhausted,
			wantMsg:    fullPoolMsg,
			wantCreate: 1,
			wantGet:    1,
		},
		{
			name:       "GET non-HTTP error after 429 returns the 429",
			setup:      getFailsAfter429(errors.New("connection reset by peer")),
			wantCode:   codes.ResourceExhausted,
			wantMsg:    fullPoolMsg,
			wantCreate: 1,
			wantGet:    1,
		},
		{
			// The same through the fake: a full pool, and a GET that fails
			// with 403.
			name:           "GET 403 after 429 from the fake returns the 429",
			pool:           "projects/test-project/locations/us-central1/volumePools/" + file.FakeVolumePoolExhausted + "-" + file.FakeVolumePoolGetForbidden + "-pool",
			wantCode:       codes.ResourceExhausted,
			wantMsg:        "no available volumes in the pool",
			wantCreate:     1,
			wantGet:        1,
			wantCreateHTTP: http.StatusTooManyRequests,
		},
		{
			name: "ALREADY_EXISTS for a volume that is not found is retryable",
			setup: func(_ *testing.T, _ *controllerServer, fs *volumePoolTestService) {
				fs.createVolume = alreadyExists
			},
			wantCode:   codes.Aborted,
			wantMsg:    "reported as existing but is not found",
			wantCreate: 1,
			wantGet:    1,
		},
		{
			name: "lookup error after ALREADY_EXISTS keeps its mapping",
			setup: func(t *testing.T, _ *controllerServer, fs *volumePoolTestService) {
				forbiddenPool := "projects/test-project/locations/us-central1/volumePools/" + file.FakeVolumePoolGetForbidden + "-pool"
				if _, err := fs.Service.CreateVolumePoolVolume(ctx, forbiddenPool, volName); err != nil {
					t.Fatalf("failed to create volume: %v", err)
				}
			},
			pool:       "projects/test-project/locations/us-central1/volumePools/" + file.FakeVolumePoolGetForbidden + "-pool",
			wantCode:   codes.PermissionDenied,
			wantCreate: 1,
			wantGet:    1,
		},
		{
			name: "lookup error 429 after ALREADY_EXISTS keeps its mapping",
			setup: func(_ *testing.T, _ *controllerServer, fs *volumePoolTestService) {
				fs.createVolume = alreadyExists
				fs.getVolume = func(context.Context, string) (*file.PoolVolume, error) {
					return nil, file.FakeVolumePoolError(http.StatusTooManyRequests, "RESOURCE_EXHAUSTED", "rate limit exceeded")
				}
			},
			wantCode:   codes.ResourceExhausted,
			wantCreate: 1,
			wantGet:    1,
		},
		{
			// Only an HTTP 404 means "missing"; an error that merely mentions
			// NOT_FOUND must not be reported as a volume that went away.
			name: "lookup error mentioning NOT_FOUND is not a missing volume",
			setup: func(_ *testing.T, _ *controllerServer, fs *volumePoolTestService) {
				fs.createVolume = alreadyExists
				fs.getVolume = func(context.Context, string) (*file.PoolVolume, error) {
					return nil, errors.New("transport failure: NOT_FOUND in proxy response")
				}
			},
			wantCode:   codes.NotFound,
			wantCreate: 1,
			wantGet:    1,
		},
		{
			name:       "409 ABORTED is retryable without a lookup",
			pool:       "projects/test-project/locations/us-central1/volumePools/" + file.FakeVolumePoolContended + "-pool",
			wantCode:   codes.Aborted,
			wantCreate: 1,
		},
		{
			name: "other create errors keep their mapping without a lookup",
			setup: func(_ *testing.T, _ *controllerServer, fs *volumePoolTestService) {
				fs.createVolume = func(context.Context, string, string) (*file.PoolVolume, error) {
					return nil, file.FakeVolumePoolError(http.StatusForbidden, "PERMISSION_DENIED", "permission denied")
				}
			},
			wantCode:   codes.PermissionDenied,
			wantCreate: 1,
		},
		{
			name: "another operation on the volume is in progress",
			setup: func(_ *testing.T, cs *controllerServer, _ *volumePoolTestService) {
				cs.config.volumeLocks.TryAcquire(volumePath)
			},
			wantCode: codes.Aborted,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cs, fs := newVolumePoolTestController(t)
			if tc.setup != nil {
				tc.setup(t, cs, fs)
			}
			pool := tc.pool
			if pool == "" {
				pool = poolPath
			}
			fs.calls = map[string]int{}

			resp, err := cs.CreateVolume(ctx, newReq(pool))
			if got := status.Code(err); got != tc.wantCode {
				t.Fatalf("got error code %v, want %v (error: %v)", got, tc.wantCode, err)
			}
			if tc.wantMsg != "" && !strings.Contains(status.Convert(err).Message(), tc.wantMsg) {
				t.Errorf("error message %q does not contain %q", status.Convert(err).Message(), tc.wantMsg)
			}
			if got := fs.calls["CreateVolumePoolVolume"]; got != tc.wantCreate {
				t.Errorf("CreateVolumePoolVolume called %d times, want %d", got, tc.wantCreate)
			}
			if got := fs.calls["GetVolumePoolVolume"]; got != tc.wantGet {
				t.Errorf("GetVolumePoolVolume called %d times, want %d", got, tc.wantGet)
			}
			if tc.wantCreateHTTP != 0 {
				var googleErr *googleapi.Error
				if !errors.As(fs.createErr, &googleErr) || googleErr.Code != tc.wantCreateHTTP {
					t.Errorf("create error = %v, want HTTP %d", fs.createErr, tc.wantCreateHTTP)
				}
			}
			if tc.wantCode != codes.OK {
				return
			}
			if got, want := resp.GetVolume().GetVolumeId(), buildVolumePoolVolumeID(pool, volName, "10.1.1.1"); got != want {
				t.Errorf("got volume ID %q, want %q", got, want)
			}
			if got, want := resp.GetVolume().GetCapacityBytes(), int64(1*util.Gb); got != want {
				t.Errorf("got capacity %d, want %d", got, want)
			}
			if diff := cmp.Diff(wantContext, resp.GetVolume().GetVolumeContext()); diff != "" {
				t.Errorf("unexpected volume context (-want +got):\n%s", diff)
			}
		})
	}

	t.Run("repeated CreateVolume returns the same response", func(t *testing.T) {
		cs, _ := newVolumePoolTestController(t)
		first, err := cs.CreateVolume(ctx, newReq(poolPath))
		if err != nil {
			t.Fatalf("first CreateVolume: %v", err)
		}
		second, err := cs.CreateVolume(ctx, newReq(poolPath))
		if err != nil {
			t.Fatalf("repeated CreateVolume: %v", err)
		}
		if !proto.Equal(first.GetVolume(), second.GetVolume()) {
			t.Errorf("repeated CreateVolume returned %v, want %v", second.GetVolume(), first.GetVolume())
		}
	})
}

// TestCreateVolume_VolumePoolVolumeState checks that CreateVolume returns only a
// volume that can be mounted, whether the volume already existed or was just
// created.
func TestCreateVolume_VolumePoolVolumeState(t *testing.T) {
	ctx := context.Background()
	const poolPath = "projects/test-project/locations/us-central1/volumePools/my-pool"
	states := []struct {
		name          string
		state         string
		mountPoint    bool
		ipOnly        bool // the mount point has an IP but no mount name
		mountNameOnly bool // the mount point has a mount name but no IP
		wantCode      codes.Code
		wantMsg       string // substring of the error message, if set
	}{
		{name: "HOT", state: file.VolumePoolVolumeStateHot, mountPoint: true},
		{name: "unspecified", state: file.VolumePoolVolumeStateUnspecified, mountPoint: true},
		{name: "no state reported", state: "", mountPoint: true},
		{name: "unspecified without a mount point", state: file.VolumePoolVolumeStateUnspecified, wantCode: codes.Aborted},
		{name: "HOT without a mount point", state: file.VolumePoolVolumeStateHot, wantCode: codes.Aborted, wantMsg: "has no mount point yet"},
		{name: "HOT with an IP but no mount name", state: file.VolumePoolVolumeStateHot, ipOnly: true, wantCode: codes.Aborted},
		{name: "HOT with a mount name but no IP", state: file.VolumePoolVolumeStateHot, mountNameOnly: true, wantCode: codes.Aborted},
		{name: "unknown state with a mount point", state: "DELETING", mountPoint: true, wantCode: codes.Aborted, wantMsg: `in state "DELETING", which can't be mounted`},
		{name: "CREATING", state: file.VolumePoolVolumeStateCreating, wantCode: codes.Aborted},
		{name: "WARM", state: file.VolumePoolVolumeStateWarm, wantCode: codes.FailedPrecondition},
		{name: "COLD", state: file.VolumePoolVolumeStateCold, wantCode: codes.FailedPrecondition},
	}
	// createErr is the create's error for an existing volume, which is then
	// looked up; nil: the create returns a new volume.
	for _, mode := range []struct {
		name      string
		createErr error
	}{
		{name: "new"},
		{name: "existing after 409 ALREADY_EXISTS", createErr: file.FakeVolumePoolError(http.StatusConflict, "ALREADY_EXISTS", "volume already exists")},
		{name: "existing after 429 RESOURCE_EXHAUSTED", createErr: file.FakeVolumePoolError(http.StatusTooManyRequests, "RESOURCE_EXHAUSTED", "rate limit exceeded")},
	} {
		for _, st := range states {
			t.Run(mode.name+"/"+st.name, func(t *testing.T) {
				cs, fs := newVolumePoolTestController(t)
				vol := func(name string) *file.PoolVolume {
					v := &file.PoolVolume{Name: name, State: st.state}
					if st.mountPoint {
						v.IpAddress, v.MountName = "10.1.1.9", "vol_state"
					}
					if st.ipOnly {
						v.IpAddress = "10.1.1.9"
					}
					if st.mountNameOnly {
						v.MountName = "vol_state"
					}
					return v
				}
				if mode.createErr != nil {
					fs.createVolume = func(context.Context, string, string) (*file.PoolVolume, error) { return nil, mode.createErr }
					fs.getVolume = func(_ context.Context, name string) (*file.PoolVolume, error) { return vol(name), nil }
				} else {
					fs.createVolume = func(_ context.Context, parent, id string) (*file.PoolVolume, error) {
						return vol(parent + "/volumes/" + id), nil
					}
				}
				_, err := cs.CreateVolume(ctx, newVolumePoolCreateRequest("state-vol", poolPath))
				if got := status.Code(err); got != st.wantCode {
					t.Errorf("got error code %v, want %v (error: %v)", got, st.wantCode, err)
				}
				if st.wantMsg != "" && !strings.Contains(status.Convert(err).Message(), st.wantMsg) {
					t.Errorf("error message %q does not contain %q", status.Convert(err).Message(), st.wantMsg)
				}
				if got, want := fs.calls["GetVolumePoolVolume"], map[bool]int{true: 1}[mode.createErr != nil]; got != want {
					t.Errorf("GetVolumePoolVolume called %d times, want %d", got, want)
				}
			})
		}
	}
}

// TestCreateVolume_VolumePoolHTTP runs CreateVolume against the real Filestore
// client and an HTTP server that returns Filestore-style responses.
func TestCreateVolume_VolumePoolHTTP(t *testing.T) {
	ctx := context.Background()
	const (
		poolPath = "projects/test-project/locations/us-central1/volumePools/my-pool"
		volName  = "pvc-123"
	)
	volumePath := poolPath + "/volumes/" + volName
	hot := map[string]any{
		"name":       volumePath,
		"state":      "HOT",
		"quotaMib":   "1024",
		"mountPoint": map[string]any{"ipAddress": "10.0.0.5", "mountName": "share_abc"},
	}
	errorBody := func(code int, canonicalStatus string) json.RawMessage {
		return json.RawMessage(file.FakeVolumePoolError(code, canonicalStatus, "test error").Body)
	}
	type response struct {
		code int
		body any // encoded as JSON; nil: no body
	}
	notFound := response{http.StatusNotFound, errorBody(http.StatusNotFound, "NOT_FOUND")}
	alreadyExists := response{http.StatusConflict, errorBody(http.StatusConflict, "ALREADY_EXISTS")}
	// The create's 429 has its own message, so that the tests can tell it from
	// the GET's errors.
	const fullPoolMsg = "no free share in the pool"
	exhausted := response{http.StatusTooManyRequests, json.RawMessage(file.FakeVolumePoolError(http.StatusTooManyRequests, "RESOURCE_EXHAUSTED", fullPoolMsg).Body)}

	tests := []struct {
		name    string
		create  response   // response to the create, which comes first
		gets    []response // responses to the GET requests that follow, in order
		want    codes.Code
		wantMsg string // substring of the error message, if set
	}{
		{
			name:   "new volume is created without a lookup",
			create: response{http.StatusOK, hot},
		},
		{
			name:   "409 ALREADY_EXISTS then the volume is found",
			create: alreadyExists,
			gets:   []response{{http.StatusOK, hot}},
		},
		{
			// A full pool fails a create for an existing name with 429.
			name:   "429 RESOURCE_EXHAUSTED then the volume is found",
			create: exhausted,
			gets:   []response{{http.StatusOK, hot}},
		},
		{
			name:    "429 RESOURCE_EXHAUSTED for a new volume keeps its mapping",
			create:  exhausted,
			gets:    []response{notFound},
			want:    codes.ResourceExhausted,
			wantMsg: fullPoolMsg,
		},
		{
			// A failed GET after a 429 returns the 429.
			name:    "429 RESOURCE_EXHAUSTED then 403 on get",
			create:  exhausted,
			gets:    []response{{http.StatusForbidden, errorBody(http.StatusForbidden, "PERMISSION_DENIED")}},
			want:    codes.ResourceExhausted,
			wantMsg: fullPoolMsg,
		},
		{
			name:    "429 RESOURCE_EXHAUSTED then 500 on get",
			create:  exhausted,
			gets:    []response{{http.StatusInternalServerError, errorBody(http.StatusInternalServerError, "INTERNAL")}},
			want:    codes.ResourceExhausted,
			wantMsg: fullPoolMsg,
		},
		{
			name:    "429 RESOURCE_EXHAUSTED then 429 on get",
			create:  exhausted,
			gets:    []response{{http.StatusTooManyRequests, errorBody(http.StatusTooManyRequests, "RESOURCE_EXHAUSTED")}},
			want:    codes.ResourceExhausted,
			wantMsg: fullPoolMsg,
		},
		{
			name:   "429 RESOURCE_EXHAUSTED then a WARM volume",
			create: exhausted,
			gets:   []response{{http.StatusOK, map[string]any{"name": volumePath, "state": "WARM"}}},
			want:   codes.FailedPrecondition,
		},
		{
			name:   "429 RESOURCE_EXHAUSTED then a CREATING volume",
			create: exhausted,
			gets:   []response{{http.StatusOK, map[string]any{"name": volumePath, "state": "CREATING"}}},
			want:   codes.Aborted,
		},
		{
			name:   "409 ABORTED is retryable without a lookup",
			create: response{http.StatusConflict, errorBody(http.StatusConflict, "ABORTED")},
			want:   codes.Aborted,
		},
		{
			name:   "409 ALREADY_EXISTS then 404 is retryable",
			create: alreadyExists,
			gets:   []response{notFound},
			want:   codes.Aborted,
		},
		{
			name:   "409 ALREADY_EXISTS then 429 on get",
			create: alreadyExists,
			gets:   []response{exhausted},
			want:   codes.ResourceExhausted,
		},
		{
			name:   "409 ALREADY_EXISTS then 403 on get",
			create: alreadyExists,
			gets:   []response{{http.StatusForbidden, errorBody(http.StatusForbidden, "PERMISSION_DENIED")}},
			want:   codes.PermissionDenied,
		},
		{
			name:   "409 ALREADY_EXISTS then a WARM volume",
			create: alreadyExists,
			gets:   []response{{http.StatusOK, map[string]any{"name": volumePath, "state": "WARM"}}},
			want:   codes.FailedPrecondition,
		},
		{
			name:   "409 ALREADY_EXISTS then a CREATING volume",
			create: alreadyExists,
			gets:   []response{{http.StatusOK, map[string]any{"name": volumePath, "state": "CREATING"}}},
			want:   codes.Aborted,
		},
		{
			// An unexpected 2xx must not look like "not found".
			name:   "409 ALREADY_EXISTS then an unexpected 204 on get",
			create: alreadyExists,
			gets:   []response{{http.StatusNoContent, nil}},
			want:   codes.Internal,
		},
		{
			name:   "new volume created WARM can't be mounted",
			create: response{http.StatusOK, map[string]any{"name": volumePath, "state": "WARM"}},
			want:   codes.FailedPrecondition,
		},
		{
			name:   "409 without status keeps the default mapping without a lookup",
			create: response{http.StatusConflict, map[string]any{"error": map[string]any{"code": http.StatusConflict, "message": "conflict"}}},
			want:   codes.Internal,
		},
		{
			name:   "403 on create keeps its mapping without a lookup",
			create: response{http.StatusForbidden, errorBody(http.StatusForbidden, "PERMISSION_DENIED")},
			want:   codes.PermissionDenied,
		},
		{
			// An unexpected 2xx must not look like success.
			name:   "create returns an unexpected 202",
			create: response{http.StatusAccepted, hot},
			want:   codes.Internal,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gets, creates int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				resp := response{http.StatusBadRequest, errorBody(http.StatusBadRequest, "INVALID_ARGUMENT")}
				switch {
				case r.Method == http.MethodPost && r.URL.Path == "/v1beta1/"+poolPath+"/volumes" && r.URL.Query().Get("volumeId") == volName && creates == 0:
					creates++
					resp = tc.create
				// A GET is expected only after the create.
				case r.Method == http.MethodGet && r.URL.Path == "/v1beta1/"+volumePath && creates == 1 && gets < len(tc.gets):
					resp = tc.gets[gets]
					gets++
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(resp.code)
				if resp.body == nil {
					return
				}
				if err := json.NewEncoder(w).Encode(resp.body); err != nil {
					t.Errorf("failed to write response: %v", err)
				}
			}))
			defer server.Close()

			fileService, err := file.NewGCFSService("test", server.Client(), server.URL+"/", "")
			if err != nil {
				t.Fatalf("failed to create Filestore service: %v", err)
			}
			cs := initTestController(t).(*controllerServer)
			cs.config.features.FeatureVolumePools = &FeatureVolumePools{Enabled: true}
			cs.config.fileService = fileService

			resp, err := cs.CreateVolume(ctx, newVolumePoolCreateRequest(volName, poolPath))
			if got := status.Code(err); got != tc.want {
				t.Fatalf("got error code %v, want %v (error: %v)", got, tc.want, err)
			}
			if tc.wantMsg != "" && !strings.Contains(status.Convert(err).Message(), tc.wantMsg) {
				t.Errorf("error message %q does not contain %q", status.Convert(err).Message(), tc.wantMsg)
			}
			if gets != len(tc.gets) {
				t.Errorf("got %d GET requests, want %d", gets, len(tc.gets))
			}
			if creates != 1 {
				t.Errorf("got %d create requests, want 1", creates)
			}
			if tc.want != codes.OK {
				return
			}
			if got, want := resp.GetVolume().GetVolumeId(), buildVolumePoolVolumeID(poolPath, volName, "10.0.0.5"); got != want {
				t.Errorf("got volume ID %q, want %q", got, want)
			}
			wantContext := map[string]string{attrIP: "10.0.0.5", attrVolume: "share_abc", attrFileProtocol: v3FileProtocol}
			if diff := cmp.Diff(wantContext, resp.GetVolume().GetVolumeContext()); diff != "" {
				t.Errorf("unexpected volume context (-want +got):\n%s", diff)
			}
		})
	}
}

// TestVolumePoolLockReleased checks that CreateVolume and DeleteVolume release
// the per-volume lock, so that the next operation on the volume can run.
func TestVolumePoolLockReleased(t *testing.T) {
	ctx := context.Background()
	const poolPath = "projects/test-project/locations/us-central1/volumePools/my-pool"
	cs, fs := newVolumePoolTestController(t)
	req := newVolumePoolCreateRequest("lock-vol", poolPath)
	resp, err := cs.CreateVolume(ctx, req)
	if err != nil {
		t.Fatalf("CreateVolume: %v", err)
	}
	if _, err := cs.CreateVolume(ctx, req); err != nil {
		t.Fatalf("second CreateVolume: %v (lock not released?)", err)
	}
	del := &csi.DeleteVolumeRequest{VolumeId: resp.GetVolume().GetVolumeId()}
	for i := 0; i < 2; i++ {
		if _, err := cs.DeleteVolume(ctx, del); err != nil {
			t.Fatalf("DeleteVolume #%d: %v (lock not released?)", i+1, err)
		}
	}
	if _, err := cs.CreateVolume(ctx, req); err != nil {
		t.Fatalf("CreateVolume after DeleteVolume: %v (lock not released?)", err)
	}
	if got := fs.calls["DeleteVolumePoolVolume"]; got != 2 {
		t.Errorf("DeleteVolumePoolVolume called %d times, want 2", got)
	}
}

func TestDeleteVolume_VolumePoolLocked(t *testing.T) {
	cs, fs := newVolumePoolTestController(t)
	cs.config.volumeLocks.TryAcquire("projects/test-project/locations/us-central1/volumePools/my-pool/volumes/test-vol")
	_, err := cs.DeleteVolume(context.Background(), &csi.DeleteVolumeRequest{
		VolumeId: buildVolumePoolVolumeID("projects/test-project/locations/us-central1/volumePools/my-pool", "test-vol", "10.1.1.1"),
	})
	if got := status.Code(err); got != codes.Aborted {
		t.Errorf("got error code %v, want %v (error: %v)", got, codes.Aborted, err)
	}
	if got := fs.calls["DeleteVolumePoolVolume"]; got != 0 {
		t.Errorf("DeleteVolumePoolVolume called %d times, want 0", got)
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
		expectCode     codes.Code // checked when expectErr is set and this is not codes.OK
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
		{
			name:           "contention (HTTP 409 ABORTED) is retryable",
			volumeID:       "volumepool://test-project/us-central1/" + file.FakeVolumePoolContended + "-pool/volumes/test-vol?ip=10.1.1.1",
			featureEnabled: true,
			expectErr:      true,
			expectCode:     codes.Aborted,
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
				if tc.expectCode != codes.OK {
					if got := status.Code(err); got != tc.expectCode {
						t.Errorf("got error code %v, want %v (error: %v)", got, tc.expectCode, err)
					}
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

func TestCheckVolumePoolVolumeName(t *testing.T) {
	valid := []string{
		"pvc-00000000-0000-4000-8000-000000000001",
		"substrate-00000000-0000-4000-8000-000000000002-fs",
		"a",
		"a" + strings.Repeat("b", 62),
	}
	invalid := []string{"", ".", "..", "a/b", "?x", "#x", "x?y", "x#y", "a%2Fb", "Upper", "a_b", "-a", "a-", "1abc", "a b", "a" + strings.Repeat("b", 63)}
	for _, name := range valid {
		if err := checkVolumePoolVolumeName(name); err != nil {
			t.Errorf("checkVolumePoolVolumeName(%q) = %v, want nil", name, err)
		}
	}
	for _, name := range invalid {
		if got := status.Code(checkVolumePoolVolumeName(name)); got != codes.InvalidArgument {
			t.Errorf("checkVolumePoolVolumeName(%q) code = %v, want InvalidArgument", name, got)
		}
	}

	// CreateVolume rejects an invalid name before any API call.
	cs, fs := newVolumePoolTestController(t)
	for _, name := range []string{"..", "?x", "a/b"} {
		_, err := cs.CreateVolume(context.Background(), newVolumePoolCreateRequest(name, "projects/test-project/locations/us-central1/volumePools/my-pool"))
		if got := status.Code(err); got != codes.InvalidArgument {
			t.Errorf("CreateVolume(name %q) code = %v, want InvalidArgument", name, got)
		}
	}
	if len(fs.calls) != 0 {
		t.Errorf("CreateVolume with invalid names made API calls: %v", fs.calls)
	}
}

func TestCanonicalVolumePoolName(t *testing.T) {
	const canonical = "projects/test-project/locations/us-central1/volumePools/my-pool"
	for _, pool := range []string{
		canonical,
		canonical + "/",
		"/" + canonical,
		" " + canonical + " ",
	} {
		got, err := canonicalVolumePoolName(pool)
		if err != nil || got != canonical {
			t.Errorf("canonicalVolumePoolName(%q) = %q, %v; want %q, nil", pool, got, err, canonical)
		}
	}
	if got, err := canonicalVolumePoolName("projects/123456789/locations/us-central1-c/volumePools/p1"); err != nil || got != "projects/123456789/locations/us-central1-c/volumePools/p1" {
		t.Errorf("canonicalVolumePoolName(project number) = %q, %v", got, err)
	}
	for _, pool := range []string{
		"",
		"my-pool",
		"projects/test-project/locations/us-central1",
		"projects/test-project/locations/us-central1/volumePools/",
		"projects/test-project/locations/us-central1/volumePools/my-pool/volumes/x",
		"projects/test-project/locations/us-central1//volumePools/my-pool",
		"projects/example.com:test-project/locations/us-central1/volumePools/my-pool",
		"projects/Test-Project/locations/us-central1/volumePools/my-pool",
		"projects/test-project/locations/us-central1/volumePools/my-pool?x=y",
	} {
		if _, err := canonicalVolumePoolName(pool); status.Code(err) != codes.InvalidArgument {
			t.Errorf("canonicalVolumePoolName(%q) error = %v, want InvalidArgument", pool, err)
		}
	}
}

// TestVolumePoolLockReleasedAfterFailure checks that a failing CreateVolume or
// DeleteVolume releases the per-volume lock.
func TestVolumePoolLockReleasedAfterFailure(t *testing.T) {
	ctx := context.Background()
	const poolPath = "projects/test-project/locations/us-central1/volumePools/my-pool"
	req := newVolumePoolCreateRequest("fail-vol", poolPath)
	failures := map[string]func(fs *volumePoolTestService){
		"409 ABORTED": func(fs *volumePoolTestService) {
			fs.createVolume = func(context.Context, string, string) (*file.PoolVolume, error) {
				return nil, file.FakeVolumePoolError(http.StatusConflict, "ABORTED", "concurrency contention, please retry")
			}
		},
		"WARM volume": func(fs *volumePoolTestService) {
			fs.createVolume = func(_ context.Context, parent, id string) (*file.PoolVolume, error) {
				return &file.PoolVolume{Name: parent + "/volumes/" + id, State: file.VolumePoolVolumeStateWarm}, nil
			}
		},
	}
	for name, fail := range failures {
		t.Run("CreateVolume "+name, func(t *testing.T) {
			cs, fs := newVolumePoolTestController(t)
			fail(fs)
			if _, err := cs.CreateVolume(ctx, req); err == nil {
				t.Fatal("first CreateVolume succeeded, want an error")
			}
			fs.createVolume = nil
			if _, err := cs.CreateVolume(ctx, req); err != nil {
				t.Fatalf("CreateVolume after a failure: %v (lock not released?)", err)
			}
		})
	}
	t.Run("DeleteVolume 409 ABORTED", func(t *testing.T) {
		cs, fs := newVolumePoolTestController(t)
		resp, err := cs.CreateVolume(ctx, req)
		if err != nil {
			t.Fatalf("CreateVolume: %v", err)
		}
		fs.deleteVolume = func(context.Context, string) error {
			return file.FakeVolumePoolError(http.StatusConflict, "ABORTED", "concurrency contention, please retry")
		}
		del := &csi.DeleteVolumeRequest{VolumeId: resp.GetVolume().GetVolumeId()}
		if _, err := cs.DeleteVolume(ctx, del); status.Code(err) != codes.Aborted {
			t.Fatalf("first DeleteVolume error = %v, want Aborted", err)
		}
		fs.deleteVolume = nil
		if _, err := cs.DeleteVolume(ctx, del); err != nil {
			t.Fatalf("DeleteVolume after a failure: %v (lock not released?)", err)
		}
	})
}

// TestVolumePoolLockExclusion checks that CreateVolume and DeleteVolume on the
// same volume exclude each other, even when the volume-pool parameter isn't in
// canonical form, and that another volume isn't blocked.
func TestVolumePoolLockExclusion(t *testing.T) {
	ctx := context.Background()
	const canonicalPool = "projects/test-project/locations/us-central1/volumePools/my-pool"
	newReq := func(name string) *csi.CreateVolumeRequest {
		return newVolumePoolCreateRequest(name, canonicalPool+"/")
	}
	cs, fs := newVolumePoolTestController(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	// Block only the first create of held-vol, so that a missing lock makes the
	// concurrent create below succeed (and fail the test) instead of hanging.
	var heldCreates atomic.Int32
	fs.createVolume = func(ctx context.Context, parent, id string) (*file.PoolVolume, error) {
		if id == "held-vol" && heldCreates.Add(1) == 1 {
			close(entered)
			<-release
		}
		return fs.Service.CreateVolumePoolVolume(ctx, parent, id)
	}

	held := make(chan error, 1)
	go func() {
		_, err := cs.CreateVolume(ctx, newReq("held-vol"))
		held <- err
	}()
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("CreateVolume did not reach the create call")
	}

	if _, err := cs.CreateVolume(ctx, newReq("held-vol")); status.Code(err) != codes.Aborted {
		t.Errorf("concurrent CreateVolume of the same volume: error = %v, want Aborted", err)
	}
	del := &csi.DeleteVolumeRequest{VolumeId: buildVolumePoolVolumeID(canonicalPool, "held-vol", "10.1.1.1")}
	if _, err := cs.DeleteVolume(ctx, del); status.Code(err) != codes.Aborted {
		t.Errorf("concurrent DeleteVolume of the same volume: error = %v, want Aborted", err)
	}
	if _, err := cs.CreateVolume(ctx, newReq("other-vol")); err != nil {
		t.Errorf("CreateVolume of another volume: %v, want success", err)
	}

	close(release)
	if err := <-held; err != nil {
		t.Errorf("held CreateVolume: %v", err)
	}
	if _, err := cs.DeleteVolume(ctx, del); err != nil {
		t.Errorf("DeleteVolume after CreateVolume finished: %v", err)
	}
}

// TestDeleteVolume_VolumePoolHTTP runs DeleteVolume against the real Filestore
// client and an HTTP server that returns Filestore-style responses.
func TestDeleteVolume_VolumePoolHTTP(t *testing.T) {
	const poolPath = "projects/test-project/locations/us-central1/volumePools/my-pool"
	errorBody := func(code int, canonicalStatus string) string {
		return file.FakeVolumePoolError(code, canonicalStatus, "test error").Body
	}
	tests := []struct {
		name string
		code int
		body string
		want codes.Code
	}{
		{name: "200", code: http.StatusOK, body: "{}"},
		{name: "204", code: http.StatusNoContent},
		{name: "404 is success", code: http.StatusNotFound, body: errorBody(http.StatusNotFound, "NOT_FOUND")},
		{name: "409 ABORTED is retryable", code: http.StatusConflict, body: errorBody(http.StatusConflict, "ABORTED"), want: codes.Aborted},
		{name: "other 409 keeps the default mapping", code: http.StatusConflict, body: errorBody(http.StatusConflict, "FAILED_PRECONDITION"), want: codes.Internal},
		{name: "unexpected 202", code: http.StatusAccepted, body: "{}", want: codes.Internal},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var deletes int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete || r.URL.Path != "/v1beta1/"+poolPath+"/volumes/pvc-1" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL)
				}
				deletes++
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.code)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			fileService, err := file.NewGCFSService("test", server.Client(), server.URL+"/", "")
			if err != nil {
				t.Fatalf("failed to create Filestore service: %v", err)
			}
			cs := initTestController(t).(*controllerServer)
			cs.config.features.FeatureVolumePools = &FeatureVolumePools{Enabled: true}
			cs.config.fileService = fileService
			_, err = cs.DeleteVolume(context.Background(), &csi.DeleteVolumeRequest{VolumeId: buildVolumePoolVolumeID(poolPath, "pvc-1", "10.0.0.5")})
			if got := status.Code(err); got != tc.want {
				t.Errorf("got error code %v, want %v (error: %v)", got, tc.want, err)
			}
			if deletes != 1 {
				t.Errorf("got %d DELETE requests, want 1", deletes)
			}
		})
	}
}

// TestDeleteVolume_VolumePoolInvalidID checks that DeleteVolume rejects a
// volume ID whose name or pool part isn't valid, before any API call.
func TestDeleteVolume_VolumePoolInvalidID(t *testing.T) {
	for _, volumeID := range []string{
		"volumepool://test-project/us-central1/my-pool/volumes/Pvc-A?ip=10.1.1.1",
		"volumepool://test-project/us-central1/my-pool/volumes/..?ip=10.1.1.1",
		"volumepool://My-Project/us-central1/my-pool/volumes/pvc-a?ip=10.1.1.1",
		"volumepool://test-project/us-central1/My_Pool/volumes/pvc-a?ip=10.1.1.1",
		// An escaped slash, extra path segments, a missing host or location.
		"volumepool://test-project/us-central1/my-pool/volumes/a%2Fb?ip=10.1.1.1",
		"volumepool://test-project/us-central1/my-pool/volumes/pvc-a/extra?ip=10.1.1.1",
		"volumepool://test-project/us-central1/my-pool/volumes/pvc-a/?ip=10.1.1.1",
		"volumepool:///us-central1/my-pool/volumes/pvc-a?ip=10.1.1.1",
		"volumepool://test-project/us-central1/my-pool/volumes/pvc-a?ip=10.1.1.1#frag",
		"volumepool://test-project//my-pool/volumes/pvc-a?ip=10.1.1.1",
		// An escaped character that would alias another name.
		"volumepool://test-project/us-central1/my-pool/volumes/pvc%2Da?ip=10.1.1.1",
		// Escapes that Go keeps escaped, and an empty fragment.
		"volumepool://test-project/us-central1/my-pool/volumes/a%3Fb?ip=10.1.1.1",
		"volumepool://test-project/us-central1/my-pool/volumes/pvc%20a?ip=10.1.1.1",
		"volumepool://test-project/us-central1/my-pool/volumes/pvc-a#",
	} {
		cs, fs := newVolumePoolTestController(t)
		_, err := cs.DeleteVolume(context.Background(), &csi.DeleteVolumeRequest{VolumeId: volumeID})
		if got := status.Code(err); got != codes.InvalidArgument {
			t.Errorf("DeleteVolume(%q) code = %v, want InvalidArgument (error: %v)", volumeID, got, err)
		}
		if len(fs.calls) != 0 {
			t.Errorf("DeleteVolume(%q) made API calls: %v", volumeID, fs.calls)
		}
	}
}

// TestDeleteVolume_VolumePoolIDWithoutIP checks that a volume ID without the ip
// query parameter still deletes the volume.
func TestDeleteVolume_VolumePoolIDWithoutIP(t *testing.T) {
	for _, volumeID := range []string{
		"volumepool://test-project/us-central1/my-pool/volumes/pvc-a",
		"volumepool://test-project/us-central1/my-pool/volumes/pvc-a?ip=",
	} {
		cs, fs := newVolumePoolTestController(t)
		if _, err := cs.DeleteVolume(context.Background(), &csi.DeleteVolumeRequest{VolumeId: volumeID}); err != nil {
			t.Errorf("DeleteVolume(%q) = %v, want success", volumeID, err)
		}
		if got := fs.calls["DeleteVolumePoolVolume"]; got != 1 {
			t.Errorf("DeleteVolume(%q) made %d delete calls, want 1", volumeID, got)
		}
	}
}

// TestParseVolumePoolVolumeIDStrict checks the parser itself, not only the
// checks that DeleteVolume applies to its result.
func TestParseVolumePoolVolumeIDStrict(t *testing.T) {
	for _, id := range []string{
		"volumepool://test-project/us-central1/my-pool/volumes/pvc-a?ip=10.1.1.1",
		"volumepool://test-project/us-central1/my-pool/volumes/pvc-a",
	} {
		parent, volID, _, err := parseVolumePoolVolumeID(id)
		if err != nil || parent != "projects/test-project/locations/us-central1/volumePools/my-pool" || volID != "pvc-a" {
			t.Errorf("parseVolumePoolVolumeID(%q) = %q, %q, %v", id, parent, volID, err)
		}
	}
	for _, id := range []string{
		"volumepool:///us-central1/my-pool/volumes/pvc-a?ip=10.1.1.1",
		"volumepool://test-project//my-pool/volumes/pvc-a?ip=10.1.1.1",
		"volumepool://test-project/us-central1//volumes/pvc-a?ip=10.1.1.1",
		"volumepool://test-project/us-central1/my-pool/volumes/?ip=10.1.1.1",
		"volumepool://test-project/us-central1/my-pool/disks/pvc-a?ip=10.1.1.1",
		"volumepool://test-project/us-central1/my-pool/volumes/a%2Fb?ip=10.1.1.1",
		"volumepool://test-project/us-central1/my-pool/volumes/pvc%2Da?ip=10.1.1.1",
		"volumepool://test-project/us-central1/my-pool/volumes/pvc-a/extra?ip=10.1.1.1",
		"volumepool://test-project/us-central1/my-pool/volumes/pvc-a?ip=10.1.1.1#frag",
		"volumepool://user@test-project/us-central1/my-pool/volumes/pvc-a?ip=10.1.1.1",
		"volumepool://test-project/us-central1/my-pool/volumes/a%3Fb?ip=10.1.1.1",
		"volumepool://test-project/us-central1/my-pool/volumes/pvc%20a?ip=10.1.1.1",
		"volumepool://test-project/us-central1/my-pool/volumes/a%25b?ip=10.1.1.1",
		"volumepool://test-project/us-central1/my-pool/volumes/pvc-a#",
		"volumepool://test-project/us-central1/my-pool/volumes/pvc-a?ip=10.1.1.1#",
		"modeInstance/test-project/us-central1/my-pool/volumes/pvc-a",
	} {
		if parent, volID, _, err := parseVolumePoolVolumeID(id); err == nil {
			t.Errorf("parseVolumePoolVolumeID(%q) = %q, %q, nil; want an error", id, parent, volID)
		}
	}
}

// TestCreateVolume_VolumePoolInvalidPool checks that CreateVolume rejects an
// invalid volume-pool parameter before any API call.
func TestCreateVolume_VolumePoolInvalidPool(t *testing.T) {
	for _, pool := range []string{
		"projects/test-project/locations/us-central1",
		"projects/example.com:test-project/locations/us-central1/volumePools/my-pool",
		"projects/test-project/locations/us-central1/volumePools/my-pool?x=y",
	} {
		cs, fs := newVolumePoolTestController(t)
		_, err := cs.CreateVolume(context.Background(), newVolumePoolCreateRequest("pvc-a", pool))
		if got := status.Code(err); got != codes.InvalidArgument {
			t.Errorf("CreateVolume(volume-pool %q) code = %v, want InvalidArgument (error: %v)", pool, got, err)
		}
		if len(fs.calls) != 0 {
			t.Errorf("CreateVolume(volume-pool %q) made API calls: %v", pool, fs.calls)
		}
	}
}

// TestDeleteVolume_VolumePoolHandlerNotFound checks the handler's own 404
// branch: an HTTP 404 with an AIP-193 body (no error reason) from the delete
// call means the volume is already gone.
func TestDeleteVolume_VolumePoolHandlerNotFound(t *testing.T) {
	cs, fs := newVolumePoolTestController(t)
	fs.deleteVolume = func(context.Context, string) error {
		return file.FakeVolumePoolError(http.StatusNotFound, "NOT_FOUND", "volume not found")
	}
	_, err := cs.DeleteVolume(context.Background(), &csi.DeleteVolumeRequest{
		VolumeId: buildVolumePoolVolumeID("projects/test-project/locations/us-central1/volumePools/my-pool", "gone-vol", "10.1.1.1"),
	})
	if err != nil {
		t.Errorf("DeleteVolume of a volume whose delete returns HTTP 404 = %v, want success", err)
	}
	if got := fs.calls["DeleteVolumePoolVolume"]; got != 1 {
		t.Errorf("DeleteVolumePoolVolume called %d times, want 1", got)
	}
}
