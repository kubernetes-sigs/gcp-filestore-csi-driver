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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/api/googleapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
			name: "HTTP 409 ALREADY_EXISTS for a volume that is then not found is retryable",
			req: &csi.CreateVolumeRequest{
				Name: "test-volumepool-already-exists",
				Parameters: map[string]string{
					paramKeyVolumePool: "projects/test-project/locations/us-central1/volumePools/" + file.FakeVolumePoolAlreadyExists + "-pool",
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
	calls        map[string]int
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

func (s *volumePoolTestService) DeleteVolumePoolVolume(ctx context.Context, name string) error {
	s.calls["DeleteVolumePoolVolume"]++
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
		if diff := cmp.Diff(first.GetVolume(), second.GetVolume(), cmp.Comparer(func(a, b *csi.Volume) bool {
			return a.GetVolumeId() == b.GetVolumeId() && a.GetCapacityBytes() == b.GetCapacityBytes() && cmp.Equal(a.GetVolumeContext(), b.GetVolumeContext())
		})); diff != "" {
			t.Errorf("repeated CreateVolume returned a different volume (-first +second):\n%s", diff)
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
		name       string
		state      string
		mountPoint bool
		ipOnly     bool // the mount point has an IP but no mount name
		wantCode   codes.Code
	}{
		{name: "HOT", state: file.VolumePoolVolumeStateHot, mountPoint: true},
		{name: "unspecified", state: file.VolumePoolVolumeStateUnspecified, mountPoint: true},
		{name: "no state reported", state: "", mountPoint: true},
		{name: "unspecified without a mount point", state: file.VolumePoolVolumeStateUnspecified, wantCode: codes.Aborted},
		{name: "HOT without a mount point", state: file.VolumePoolVolumeStateHot, wantCode: codes.Aborted},
		{name: "HOT with an IP but no mount name", state: file.VolumePoolVolumeStateHot, ipOnly: true, wantCode: codes.Aborted},
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
			name:   "429 RESOURCE_EXHAUSTED on create",
			gets:   []response{notFound},
			create: &response{http.StatusTooManyRequests, errorBody(http.StatusTooManyRequests, "RESOURCE_EXHAUSTED")},
			want:   codes.ResourceExhausted,
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
