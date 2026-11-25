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
	if s.createVolume != nil {
		return s.createVolume(ctx, parentPool, volumeID)
	}
	return s.Service.CreateVolumePoolVolume(ctx, parentPool, volumeID)
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

// googleAPIError returns an error like the one the Filestore client returns for
// an HTTP error response with canonical status st in its body.
func googleAPIError(httpCode int, st string) *googleapi.Error {
	return &googleapi.Error{
		Code:    httpCode,
		Message: strings.ToLower(st),
		Body:    fmt.Sprintf(`{"error": {"code": %d, "message": %q, "status": %q}}`, httpCode, strings.ToLower(st), st),
	}
}

// notFoundOnce returns a getVolume override that reports the volume missing on
// the first call and then calls next.
func notFoundOnce(next func(context.Context, string) (*file.PoolVolume, error)) func(context.Context, string) (*file.PoolVolume, error) {
	first := true
	return func(ctx context.Context, name string) (*file.PoolVolume, error) {
		if first {
			first = false
			return nil, googleAPIError(http.StatusNotFound, "NOT_FOUND")
		}
		return next(ctx, name)
	}
}

func TestCreateVolume_VolumePoolIdempotent(t *testing.T) {
	ctx := context.Background()
	const poolPath = "projects/test-project/locations/us-central1/volumePools/my-pool"
	const volName = "test-volumepool-idempotent"
	volumePath := poolPath + "/volumes/" + volName
	newReq := func(pool string) *csi.CreateVolumeRequest {
		return &csi.CreateVolumeRequest{
			Name: volName,
			Parameters: map[string]string{
				paramKeyVolumePool: pool,
				paramFileProtocol:  v4_1FileProtocol,
				paramMountOptions:  "nconnect=8",
			},
			VolumeCapabilities: []*csi.VolumeCapability{{
				AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
				AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
			}},
			CapacityRange: &csi.CapacityRange{RequiredBytes: 1 * util.Gb},
		}
	}
	wantContext := map[string]string{attrIP: "10.1.1.1", attrVolume: "vol_" + volName, attrFileProtocol: v4_1FileProtocol, attrMountOptions: "nconnect=8"}

	tests := []struct {
		name string
		// setup runs before the CreateVolume under test.
		setup      func(t *testing.T, cs *controllerServer, fs *volumePoolTestService)
		pool       string
		wantCode   codes.Code
		wantCreate int // CreateVolumePoolVolume calls made by the CreateVolume under test
		wantGet    int // GetVolumePoolVolume calls made by the CreateVolume under test
	}{
		{
			name:       "new volume is looked up, then created",
			wantCreate: 1,
			wantGet:    1,
		},
		{
			name: "existing volume is returned without a create",
			setup: func(t *testing.T, cs *controllerServer, _ *volumePoolTestService) {
				if _, err := cs.CreateVolume(ctx, newReq(poolPath)); err != nil {
					t.Fatalf("first CreateVolume: %v", err)
				}
			},
			wantGet: 1,
		},
		{
			// The API looks for a free share before it checks the name, so in a
			// full pool a create for an existing name gets HTTP 429
			// RESOURCE_EXHAUSTED instead of ALREADY_EXISTS.
			name: "existing volume in a full pool is returned",
			setup: func(t *testing.T, cs *controllerServer, fs *volumePoolTestService) {
				if _, err := cs.CreateVolume(ctx, newReq(poolPath)); err != nil {
					t.Fatalf("first CreateVolume: %v", err)
				}
				fs.createVolume = func(context.Context, string, string) (*file.PoolVolume, error) {
					return nil, googleAPIError(http.StatusTooManyRequests, "RESOURCE_EXHAUSTED")
				}
			},
			wantGet: 1,
		},
		{
			name: "another request created the volume after the lookup",
			setup: func(t *testing.T, _ *controllerServer, fs *volumePoolTestService) {
				if _, err := fs.Service.CreateVolumePoolVolume(ctx, poolPath, volName); err != nil {
					t.Fatalf("failed to create volume: %v", err)
				}
				fs.getVolume = notFoundOnce(fs.Service.GetVolumePoolVolume)
			},
			wantCreate: 1,
			wantGet:    2,
		},
		{
			name: "ALREADY_EXISTS for a volume that is still not found is retryable",
			setup: func(_ *testing.T, _ *controllerServer, fs *volumePoolTestService) {
				fs.createVolume = func(context.Context, string, string) (*file.PoolVolume, error) {
					return nil, googleAPIError(http.StatusConflict, "ALREADY_EXISTS")
				}
			},
			wantCode:   codes.Aborted,
			wantCreate: 1,
			wantGet:    2,
		},
		{
			name:     "lookup error 403 keeps its mapping",
			pool:     "projects/test-project/locations/us-central1/volumePools/" + file.FakeVolumePoolGetForbidden + "-pool",
			wantCode: codes.PermissionDenied,
			wantGet:  1,
		},
		{
			name: "lookup error 429 keeps its mapping",
			setup: func(_ *testing.T, _ *controllerServer, fs *volumePoolTestService) {
				fs.getVolume = func(context.Context, string) (*file.PoolVolume, error) {
					return nil, googleAPIError(http.StatusTooManyRequests, "RESOURCE_EXHAUSTED")
				}
			},
			wantCode: codes.ResourceExhausted,
			wantGet:  1,
		},
		{
			name: "get error after ALREADY_EXISTS keeps its mapping",
			setup: func(_ *testing.T, _ *controllerServer, fs *volumePoolTestService) {
				fs.getVolume = notFoundOnce(func(context.Context, string) (*file.PoolVolume, error) {
					return nil, googleAPIError(http.StatusForbidden, "PERMISSION_DENIED")
				})
				fs.createVolume = func(context.Context, string, string) (*file.PoolVolume, error) {
					return nil, googleAPIError(http.StatusConflict, "ALREADY_EXISTS")
				}
			},
			wantCode:   codes.PermissionDenied,
			wantCreate: 1,
			wantGet:    2,
		},
		{
			// Another request, for example on another controller pod, created
			// the volume after the lookup and took the last free share.
			name: "429 from create, then the volume is found",
			setup: func(t *testing.T, _ *controllerServer, fs *volumePoolTestService) {
				if _, err := fs.Service.CreateVolumePoolVolume(ctx, poolPath, volName); err != nil {
					t.Fatalf("failed to create volume: %v", err)
				}
				fs.getVolume = notFoundOnce(fs.Service.GetVolumePoolVolume)
				fs.createVolume = func(context.Context, string, string) (*file.PoolVolume, error) {
					return nil, googleAPIError(http.StatusTooManyRequests, "RESOURCE_EXHAUSTED")
				}
			},
			wantCreate: 1,
			wantGet:    2,
		},
		{
			name: "429 from create, then the volume is still not found",
			setup: func(_ *testing.T, _ *controllerServer, fs *volumePoolTestService) {
				fs.createVolume = func(context.Context, string, string) (*file.PoolVolume, error) {
					return nil, googleAPIError(http.StatusTooManyRequests, "RESOURCE_EXHAUSTED")
				}
			},
			wantCode:   codes.ResourceExhausted,
			wantCreate: 1,
			wantGet:    2,
		},
		{
			name: "429 from create, then the lookup fails with 403",
			setup: func(_ *testing.T, _ *controllerServer, fs *volumePoolTestService) {
				fs.getVolume = notFoundOnce(func(context.Context, string) (*file.PoolVolume, error) {
					return nil, googleAPIError(http.StatusForbidden, "PERMISSION_DENIED")
				})
				fs.createVolume = func(context.Context, string, string) (*file.PoolVolume, error) {
					return nil, googleAPIError(http.StatusTooManyRequests, "RESOURCE_EXHAUSTED")
				}
			},
			wantCode:   codes.PermissionDenied,
			wantCreate: 1,
			wantGet:    2,
		},
		{
			// Only an HTTP 404 means "missing"; an error that merely mentions
			// NOT_FOUND must not lead to a create.
			name: "lookup error mentioning NOT_FOUND is not a missing volume",
			setup: func(_ *testing.T, _ *controllerServer, fs *volumePoolTestService) {
				fs.getVolume = func(context.Context, string) (*file.PoolVolume, error) {
					return nil, errors.New("transport failure: NOT_FOUND in proxy response")
				}
			},
			wantCode: codes.NotFound,
			wantGet:  1,
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
			if got := fs.calls["CreateVolumePoolVolume"]; got != tc.wantCreate {
				t.Errorf("CreateVolumePoolVolume called %d times, want %d", got, tc.wantCreate)
			}
			if got := fs.calls["GetVolumePoolVolume"]; got != tc.wantGet {
				t.Errorf("GetVolumePoolVolume called %d times, want %d", got, tc.wantGet)
			}
			if tc.wantCode != codes.OK {
				return
			}
			if got, want := resp.GetVolume().GetVolumeId(), buildVolumePoolVolumeID(poolPath, volName, "10.1.1.1"); got != want {
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
		{name: "HOT without a mount point", state: file.VolumePoolVolumeStateHot, wantCode: codes.Aborted},
		{name: "HOT with an IP but no mount name", state: file.VolumePoolVolumeStateHot, ipOnly: true, wantCode: codes.Aborted},
		{name: "HOT with a mount name but no IP", state: file.VolumePoolVolumeStateHot, mountNameOnly: true, wantCode: codes.Aborted},
		{name: "unknown state with a mount point", state: "DELETING", mountPoint: true, wantCode: codes.Aborted, wantMsg: `in state "DELETING", which can't be mounted`},
		{name: "HOT without a mount point message", state: file.VolumePoolVolumeStateHot, wantCode: codes.Aborted, wantMsg: "has no mount point yet"},
		{name: "CREATING", state: file.VolumePoolVolumeStateCreating, wantCode: codes.Aborted},
		{name: "WARM", state: file.VolumePoolVolumeStateWarm, wantCode: codes.FailedPrecondition},
		{name: "COLD", state: file.VolumePoolVolumeStateCold, wantCode: codes.FailedPrecondition},
	}
	for _, existing := range []bool{true, false} {
		for _, st := range states {
			t.Run(fmt.Sprintf("existing=%t/%s", existing, st.name), func(t *testing.T) {
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
				if existing {
					fs.getVolume = func(_ context.Context, name string) (*file.PoolVolume, error) { return vol(name), nil }
				} else {
					fs.createVolume = func(_ context.Context, parent, id string) (*file.PoolVolume, error) {
						return vol(parent + "/volumes/" + id), nil
					}
				}
				_, err := cs.CreateVolume(ctx, &csi.CreateVolumeRequest{
					Name:       "state-vol",
					Parameters: map[string]string{paramKeyVolumePool: poolPath},
					VolumeCapabilities: []*csi.VolumeCapability{{
						AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
						AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
					}},
				})
				if got := status.Code(err); got != st.wantCode {
					t.Errorf("got error code %v, want %v (error: %v)", got, st.wantCode, err)
				}
				if st.wantMsg != "" && !strings.Contains(status.Convert(err).Message(), st.wantMsg) {
					t.Errorf("error message %q does not contain %q", status.Convert(err).Message(), st.wantMsg)
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
	errorBody := func(code int, canonicalStatus string) map[string]any {
		return map[string]any{"error": map[string]any{"code": code, "message": "test error", "status": canonicalStatus}}
	}
	type response struct {
		code int
		body map[string]any
	}
	notFound := response{http.StatusNotFound, errorBody(http.StatusNotFound, "NOT_FOUND")}

	tests := []struct {
		name   string
		gets   []response // responses to the GET requests, in order
		create *response  // response to the create; nil: the test expects no create
		want   codes.Code
	}{
		{
			name:   "new volume is created",
			gets:   []response{notFound},
			create: &response{http.StatusOK, hot},
		},
		{
			name: "existing volume is returned without a create",
			gets: []response{{http.StatusOK, hot}},
		},
		{
			name:   "409 ABORTED is retryable",
			gets:   []response{notFound},
			create: &response{http.StatusConflict, errorBody(http.StatusConflict, "ABORTED")},
			want:   codes.Aborted,
		},
		{
			name:   "409 ALREADY_EXISTS after the lookup returns the volume",
			gets:   []response{notFound, {http.StatusOK, hot}},
			create: &response{http.StatusConflict, errorBody(http.StatusConflict, "ALREADY_EXISTS")},
		},
		{
			name:   "409 ALREADY_EXISTS then 404 is retryable",
			gets:   []response{notFound, notFound},
			create: &response{http.StatusConflict, errorBody(http.StatusConflict, "ALREADY_EXISTS")},
			want:   codes.Aborted,
		},
		{
			name:   "409 ALREADY_EXISTS then 429 on get",
			gets:   []response{notFound, {http.StatusTooManyRequests, errorBody(http.StatusTooManyRequests, "RESOURCE_EXHAUSTED")}},
			create: &response{http.StatusConflict, errorBody(http.StatusConflict, "ALREADY_EXISTS")},
			want:   codes.ResourceExhausted,
		},
		{
			name: "lookup 403 keeps its mapping",
			gets: []response{{http.StatusForbidden, errorBody(http.StatusForbidden, "PERMISSION_DENIED")}},
			want: codes.PermissionDenied,
		},
		{
			name: "existing WARM volume can't be mounted",
			gets: []response{{http.StatusOK, map[string]any{"name": volumePath, "state": "WARM"}}},
			want: codes.FailedPrecondition,
		},
		{
			name:   "new volume created WARM can't be mounted",
			gets:   []response{notFound},
			create: &response{http.StatusOK, map[string]any{"name": volumePath, "state": "WARM"}},
			want:   codes.FailedPrecondition,
		},
		{
			name:   "409 without status keeps the default mapping",
			gets:   []response{notFound},
			create: &response{http.StatusConflict, map[string]any{"error": map[string]any{"code": http.StatusConflict, "message": "conflict"}}},
			want:   codes.Internal,
		},
		{
			name:   "429 RESOURCE_EXHAUSTED on create, volume still missing",
			gets:   []response{notFound, notFound},
			create: &response{http.StatusTooManyRequests, errorBody(http.StatusTooManyRequests, "RESOURCE_EXHAUSTED")},
			want:   codes.ResourceExhausted,
		},
		{
			name:   "429 RESOURCE_EXHAUSTED on create, volume found",
			gets:   []response{notFound, {http.StatusOK, hot}},
			create: &response{http.StatusTooManyRequests, errorBody(http.StatusTooManyRequests, "RESOURCE_EXHAUSTED")},
		},
		{
			name:   "409 ALREADY_EXISTS then a WARM volume",
			gets:   []response{notFound, {http.StatusOK, map[string]any{"name": volumePath, "state": "WARM"}}},
			create: &response{http.StatusConflict, errorBody(http.StatusConflict, "ALREADY_EXISTS")},
			want:   codes.FailedPrecondition,
		},
		{
			name:   "409 ALREADY_EXISTS then a CREATING volume",
			gets:   []response{notFound, {http.StatusOK, map[string]any{"name": volumePath, "state": "CREATING"}}},
			create: &response{http.StatusConflict, errorBody(http.StatusConflict, "ALREADY_EXISTS")},
			want:   codes.Aborted,
		},
		{
			// Only a 429 from create leads to a second lookup.
			name:   "403 on create keeps its mapping without a second lookup",
			gets:   []response{notFound},
			create: &response{http.StatusForbidden, errorBody(http.StatusForbidden, "PERMISSION_DENIED")},
			want:   codes.PermissionDenied,
		},
		{
			// An unexpected 2xx must not look like "not found".
			name: "lookup returns an unexpected 204",
			gets: []response{{http.StatusNoContent, nil}},
			want: codes.Internal,
		},
		{
			// An unexpected 2xx must not look like success.
			name:   "create returns an unexpected 202",
			gets:   []response{notFound},
			create: &response{http.StatusAccepted, hot},
			want:   codes.Internal,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gets, creates int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				resp := response{http.StatusBadRequest, errorBody(http.StatusBadRequest, "INVALID_ARGUMENT")}
				switch {
				case r.Method == http.MethodPost && r.URL.Path == "/v1beta1/"+poolPath+"/volumes" && r.URL.Query().Get("volumeId") == volName && tc.create != nil:
					creates++
					resp = *tc.create
				case r.Method == http.MethodGet && r.URL.Path == "/v1beta1/"+volumePath && gets < len(tc.gets):
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

			resp, err := cs.CreateVolume(ctx, &csi.CreateVolumeRequest{
				Name:       volName,
				Parameters: map[string]string{paramKeyVolumePool: poolPath},
				VolumeCapabilities: []*csi.VolumeCapability{{
					AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
					AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
				}},
			})
			if got := status.Code(err); got != tc.want {
				t.Fatalf("got error code %v, want %v (error: %v)", got, tc.want, err)
			}
			if gets != len(tc.gets) {
				t.Errorf("got %d GET requests, want %d", gets, len(tc.gets))
			}
			if wantCreates := map[bool]int{true: 1}[tc.create != nil]; creates != wantCreates {
				t.Errorf("got %d create requests, want %d", creates, wantCreates)
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
	req := &csi.CreateVolumeRequest{
		Name:       "lock-vol",
		Parameters: map[string]string{paramKeyVolumePool: poolPath},
		VolumeCapabilities: []*csi.VolumeCapability{{
			AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
			AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
		}},
	}
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
		"pvc-205cf9ee-3739-4112-8356-ff3b12763078",
		"substrate-08fe8bdc-aa8a-4542-9456-2faa9b182678-fs",
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
		_, err := cs.CreateVolume(context.Background(), &csi.CreateVolumeRequest{
			Name:       name,
			Parameters: map[string]string{paramKeyVolumePool: "projects/test-project/locations/us-central1/volumePools/my-pool"},
			VolumeCapabilities: []*csi.VolumeCapability{{
				AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
				AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
			}},
		})
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
	req := &csi.CreateVolumeRequest{
		Name:       "fail-vol",
		Parameters: map[string]string{paramKeyVolumePool: poolPath},
		VolumeCapabilities: []*csi.VolumeCapability{{
			AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
			AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
		}},
	}
	failures := map[string]func(fs *volumePoolTestService){
		"409 ABORTED": func(fs *volumePoolTestService) {
			fs.createVolume = func(context.Context, string, string) (*file.PoolVolume, error) {
				return nil, googleAPIError(http.StatusConflict, "ABORTED")
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
			return googleAPIError(http.StatusConflict, "ABORTED")
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
		return &csi.CreateVolumeRequest{
			Name:       name,
			Parameters: map[string]string{paramKeyVolumePool: canonicalPool + "/"},
			VolumeCapabilities: []*csi.VolumeCapability{{
				AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
				AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
			}},
		}
	}
	cs, fs := newVolumePoolTestController(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	fs.createVolume = func(ctx context.Context, parent, id string) (*file.PoolVolume, error) {
		if id == "held-vol" {
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
		return fmt.Sprintf(`{"error": {"code": %d, "message": "test error", "status": %q}}`, code, canonicalStatus)
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
