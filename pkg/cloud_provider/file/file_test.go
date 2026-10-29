package file

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	filev1beta1 "google.golang.org/api/file/v1beta1"
	"google.golang.org/api/option"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/googleapis/gax-go/v2/apierror"
	"google.golang.org/api/googleapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/util"
)

func TestCompareInstances(t *testing.T) {
	cases := []struct {
		name               string
		a                  *ServiceInstance
		b                  *ServiceInstance
		expectedMismatches []string
	}{
		{
			name: "matches equal",
			a: &ServiceInstance{
				Tier: "tier",
				Volume: Volume{
					Name:      "volName",
					SizeBytes: 10 * util.Gb,
				},
				Network: Network{
					Name: "networkName",
				},
			},
			b: &ServiceInstance{
				Tier: "tier",
				Volume: Volume{
					Name:      "volName",
					SizeBytes: 10 * util.Gb,
				},
				Network: Network{
					Name: "networkName",
				},
			},
		},
		{
			name: "matches equal rounded capacity",
			a: &ServiceInstance{
				Tier: "tier",
				Volume: Volume{
					Name:      "volName",
					SizeBytes: 10 * util.Gb,
				},
				Network: Network{
					Name: "networkName",
				},
			},
			b: &ServiceInstance{
				Tier: "tier",
				Volume: Volume{
					Name:      "volName",
					SizeBytes: 10*util.Gb - 1,
				},
				Network: Network{
					Name: "networkName",
				},
			},
		},
		{
			name: "matches equal tier lowercase",
			a: &ServiceInstance{
				Tier: "tier",
				Volume: Volume{
					Name:      "volName",
					SizeBytes: 10 * util.Gb,
				},
				Network: Network{
					Name: "networkName",
				},
			},
			b: &ServiceInstance{
				Tier: "TIER",
				Volume: Volume{
					Name:      "volName",
					SizeBytes: 10 * util.Gb,
				},
				Network: Network{
					Name: "networkName",
				},
			},
		},
		{
			name: "nothing matches",
			a: &ServiceInstance{
				Tier: "tier",
				Volume: Volume{
					Name:      "volName",
					SizeBytes: 10 * util.Gb,
				},
				Network: Network{
					Name: "networkName",
				},
			},
			b: &ServiceInstance{
				Tier: "tier2",
				Volume: Volume{
					Name:      "volName2",
					SizeBytes: 10*util.Gb + 1,
				},
				Network: Network{
					Name: "networkName2",
				},
			},
			expectedMismatches: []string{
				"tier",
				"volume name",
				"volume size",
				"network name",
			},
		},
	}

	for _, test := range cases {
		err := CompareInstances(test.a, test.b)
		if len(test.expectedMismatches) == 0 {
			if err != nil {
				t.Errorf("test %v failed: expected match, got %v", test.name, err)
			}
		} else {
			if err == nil {
				t.Errorf("test %v failed: expected mismatches, got success", test.name)
			} else {
				for _, mismatch := range test.expectedMismatches {
					if !strings.Contains(err.Error(), mismatch) {
						t.Errorf("test %v failed: didn't get expected mismatch %v", test.name, mismatch)
					}
				}
			}
		}
	}
}

func TestUpdateInstancePerformance(t *testing.T) {
	ctx := context.Background()
	mgr := &gcfsServiceManager{}

	// nil perfConfig -> error
	if err := mgr.UpdateInstancePerformance(ctx, &ServiceInstance{}, nil); err == nil {
		t.Fatalf("expected error for nil perfConfig")
	}

	// zero values -> error
	if err := mgr.UpdateInstancePerformance(ctx, &ServiceInstance{}, &PerformanceConfig{}); err == nil {
		t.Fatalf("expected error for empty perfConfig")
	}

	// success path using httptest server to mock PATCH and operations GET
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// return an operation for PATCH and return done=true for GET on operations
		if r.Method == "PATCH" && strings.Contains(r.URL.Path, "/instances/") {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"name":"projects/proj/locations/loc/operations/op1","done":false}`)
			return
		}
		if r.Method == "GET" && strings.Contains(r.URL.Path, "/operations/") {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"name":"projects/proj/locations/loc/operations/op1","done":true}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	// create file service pointing to test server
	svc, err := filev1beta1.NewService(ctx, option.WithEndpoint(ts.URL+"/"), option.WithHTTPClient(ts.Client()))
	if err != nil {
		t.Fatalf("failed to create file service: %v", err)
	}

	mgr.instancesService = filev1beta1.NewProjectsLocationsInstancesService(svc)
	mgr.operationsService = filev1beta1.NewProjectsLocationsOperationsService(svc)

	si := &ServiceInstance{Project: "proj", Location: "loc", Name: "name"}
	perf := &PerformanceConfig{FixedIOPS: 100}

	if err := mgr.UpdateInstancePerformance(ctx, si, perf); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
}

func TestCreateInstance_PerformanceConfig(t *testing.T) {
	ctx := context.Background()
	mgr := &gcfsServiceManager{}

	// Mock server to capture Create request and return operation and instance
	var captured filev1beta1.Instance
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && strings.Contains(r.URL.Path, "/instances") {
			if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"name":"projects/proj/locations/loc/operations/op1","done":false}`)
			return
		}
		if r.Method == "GET" && strings.Contains(r.URL.Path, "/operations/") {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"name":"projects/proj/locations/loc/operations/op1","done":true}`)
			return
		}
		if r.Method == "GET" && strings.Contains(r.URL.Path, "/instances/") {
			// return the instance including performance config (only FixedIops)
			inst := filev1beta1.Instance{
				Name:       "projects/proj/locations/loc/instances/name",
				Tier:       "tier",
				FileShares: []*filev1beta1.FileShareConfig{{Name: "vol", CapacityGb: 10}},
				Networks:   []*filev1beta1.NetworkConfig{{Network: "net"}},
				PerformanceConfig: &filev1beta1.PerformanceConfig{
					FixedIops: &filev1beta1.FixedIOPS{MaxIops: 42},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(&inst)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	svc, err := filev1beta1.NewService(ctx, option.WithEndpoint(ts.URL+"/"), option.WithHTTPClient(ts.Client()))
	if err != nil {
		t.Fatalf("failed to create file service: %v", err)
	}

	mgr.instancesService = filev1beta1.NewProjectsLocationsInstancesService(svc)
	mgr.operationsService = filev1beta1.NewProjectsLocationsOperationsService(svc)

	si := &ServiceInstance{Project: "proj", Location: "loc", Name: "name", PerformanceConfig: &PerformanceConfig{FixedIOPS: 1000}, Volume: Volume{Name: "vol"}, Network: Network{Name: "net"}}

	_, err = mgr.CreateInstance(ctx, si)
	if err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}

	if captured.PerformanceConfig == nil {
		t.Fatalf("expected PerformanceConfig to be sent in Create request")
	}
	if captured.PerformanceConfig.FixedIops == nil || captured.PerformanceConfig.FixedIops.MaxIops != 1000 {
		t.Fatalf("unexpected FixedIops in request: %+v", captured.PerformanceConfig.FixedIops)
	}
	if captured.PerformanceConfig.IopsPerTb != nil {
		t.Fatalf("did not expect IopsPerTb in request: %+v", captured.PerformanceConfig.IopsPerTb)
	}

	siBoth := &ServiceInstance{Project: "proj", Location: "loc", Name: "name", PerformanceConfig: &PerformanceConfig{FixedIOPS: 1, IOPSPerTB: 2}, Volume: Volume{Name: "vol"}, Network: Network{Name: "net"}}
	_, err = mgr.CreateInstance(ctx, siBoth)
	if err == nil {
		t.Fatalf("expected CreateInstance to fail when both performance params set")
	}
}

func TestCloudInstanceToServiceInstance_PerformanceConfig(t *testing.T) {
	inst := &filev1beta1.Instance{
		Name:       fmt.Sprintf("projects/%s/locations/%s/instances/%s", "proj", "loc", "name"),
		Tier:       "tier",
		FileShares: []*filev1beta1.FileShareConfig{{Name: "vol", CapacityGb: 10}},
		Networks:   []*filev1beta1.NetworkConfig{{Network: "net", IpAddresses: []string{"1.2.3.4"}}},
		PerformanceConfig: &filev1beta1.PerformanceConfig{
			FixedIops: &filev1beta1.FixedIOPS{MaxIops: 1234},
			IopsPerTb: &filev1beta1.IOPSPerTB{MaxIopsPerTb: 5678},
		},
	}

	si, err := cloudInstanceToServiceInstance(inst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if si.PerformanceConfig == nil {
		t.Fatalf("expected PerformanceConfig to be populated")
	}
	if si.PerformanceConfig.FixedIOPS != 1234 {
		t.Fatalf("expected FixedIOPS 1234, got %d", si.PerformanceConfig.FixedIOPS)
	}
	if si.PerformanceConfig.IOPSPerTB != 5678 {
		t.Fatalf("expected IOPSPerTB 5678, got %d", si.PerformanceConfig.IOPSPerTB)
	}
}

func TestGetInstanceNameFromURI(t *testing.T) {
	cases := []struct {
		name      string
		uri       string
		project   string
		location  string
		instance  string
		expectErr bool
	}{
		{
			name:     "good",
			uri:      "projects/test-project1/locations/test-$location/instances/test-^instance",
			project:  "test-project1",
			location: "test-$location",
			instance: "test-^instance",
		},
		{
			name:      "bad prefix",
			uri:       "badprojects/test-project/locations/test-location/instances/test-instance",
			expectErr: true,
		},
		{
			name:      "bad suffix",
			uri:       "projects/test-project/locations/test-location/instances/test-instance/bad",
			expectErr: true,
		},
		{
			name:      "missing instance",
			uri:       "projects/test-project/locations/test-location/instances/",
			expectErr: true,
		},
		{
			name:      "missing location",
			uri:       "projects/test-project/locations//instances/test-instance",
			expectErr: true,
		},
		{
			name:      "missing project",
			uri:       "projects//locations/test-location/instances/test-instance",
			expectErr: true,
		},
	}

	for _, test := range cases {
		project, location, instance, err := GetInstanceNameFromURI(test.uri)
		if err == nil && test.expectErr {
			t.Errorf("test %v failed: got success", test.name)
		}
		if err != nil && !test.expectErr {
			t.Errorf("test %v failed: got error: %v", test.name, err)
		}

		if project != test.project {
			t.Errorf("test %v failed: got project %q, expected %q", test.name, project, test.project)
		}
		if location != test.location {
			t.Errorf("test %v failed: got location %q, expected %q", test.name, location, test.location)
		}
		if instance != test.instance {
			t.Errorf("test %v failed: got instance %q, expected %q", test.name, instance, test.instance)
		}
	}
}

func TestIsMultishareInstanceTarget(t *testing.T) {
	tests := []struct {
		name     string
		inputuri string
		match    bool
	}{
		{
			name:     "empty",
			inputuri: "",
		},
		{
			name:     "invalid case 1",
			inputuri: "projects/test-project/locations/us-central1/instances/test-instance/shares/test-share",
		},
		{
			name:     "invalid case 2",
			inputuri: "projectstest-project/locations/us-central1/instances/test-instance/shares/test-share",
		},
		{
			name:     "invalid case 3",
			inputuri: "projects/test-project/locations/us-central1/instances/test-instance/",
		},
		{
			name:     "valid case 1",
			inputuri: "projects/test-project/locations/us-central1/instances/test-instance",
			match:    true,
		},
		{
			name:     "valid case 2",
			inputuri: "projects/test-project/locations/us-central1-c/instances/test-instance",
			match:    true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if IsInstanceTarget(tc.inputuri) != tc.match {
				t.Errorf("unecpected error")
			}
		})
	}
}

func TestIsMultishareShareTarget(t *testing.T) {
	tests := []struct {
		name     string
		inputuri string
		match    bool
	}{
		{
			name:     "empty",
			inputuri: "",
		},
		{
			name:     "invalid case 1",
			inputuri: "projects/test-project/locations/us-central1/instances/test-instance",
		},
		{
			name:     "invalid case 2",
			inputuri: "projectstest-project/locations/us-central1/instances/test-instance",
		},
		{
			name:     "invalid case 3",
			inputuri: "projects/test-project/locations/us-central1/instances/test-instance/shares/test-share/",
		},
		{
			name:     "valid case 1",
			inputuri: "projects/test-project/locations/us-central1/instances/test-instance/shares/test-share",
			match:    true,
		},
		{
			name:     "valid case 2",
			inputuri: "projects/test-project/locations/us-central1-c/instances/test-instance/shares/test-share",
			match:    true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if IsShareTarget(tc.inputuri) != tc.match {
				t.Errorf("unecpected error")
			}
		})
	}
}

func TestGenerateMultishareInstanceURI(t *testing.T) {
	tests := []struct {
		name          string
		input         *MultishareInstance
		expectedUri   string
		errorExpected bool
	}{
		{
			name:          "nil instance",
			errorExpected: true,
		},
		{
			name: "empty project",
			input: &MultishareInstance{
				Location: "us-central1",
				Name:     "test",
			},
			errorExpected: true,
		},
		{
			name: "empty location",
			input: &MultishareInstance{
				Project: "test-project",
				Name:    "test",
			},
			errorExpected: true,
		},
		{
			name: "empty name",
			input: &MultishareInstance{
				Location: "us-central1",
				Project:  "test-project",
			},
			errorExpected: true,
		},
		{
			name: "valid input",
			input: &MultishareInstance{
				Location: "us-central1",
				Project:  "test-project",
				Name:     "test-instance",
			},
			expectedUri: "projects/test-project/locations/us-central1/instances/test-instance",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			uri, err := GenerateMultishareInstanceURI(tc.input)
			if err != nil && !tc.errorExpected {
				t.Errorf("unexpected error")
			}
			if err == nil && tc.errorExpected {
				t.Errorf("expected error got nil")
			}
			if tc.expectedUri != uri {
				t.Errorf("got %s, want %s", uri, tc.expectedUri)
			}
		})
	}
}

func TestGenerateShareURI(t *testing.T) {
	tests := []struct {
		name          string
		input         *Share
		expectedUri   string
		errorExpected bool
	}{
		{
			name:          "nil share",
			errorExpected: true,
		},
		{
			name:          "nil share parent",
			errorExpected: true,
			input: &Share{
				Name: "test-share",
			},
		},
		{
			name: "empty project",
			input: &Share{
				Parent: &MultishareInstance{
					Location: "us-central1",
				},
				Name: "test-share",
			},
			errorExpected: true,
		},
		{
			name: "empty location",
			input: &Share{
				Parent: &MultishareInstance{
					Project: "test-project",
				},
				Name: "test-share",
			},
			errorExpected: true,
		},
		{
			name: "empty instance name",
			input: &Share{
				Parent: &MultishareInstance{
					Project:  "test-project",
					Location: "us-central1",
				},
				Name: "test-share",
			},
			errorExpected: true,
		},
		{
			name: "valid input",
			input: &Share{
				Parent: &MultishareInstance{
					Location: "us-central1",
					Project:  "test-project",
					Name:     "test-instance",
				},
				Name: "test-share",
			},
			expectedUri: "projects/test-project/locations/us-central1/instances/test-instance/shares/test-share",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			uri, err := GenerateShareURI(tc.input)
			if err != nil && !tc.errorExpected {
				t.Errorf("unexpected error")
			}
			if err == nil && tc.errorExpected {
				t.Errorf("expected error got nil")
			}
			if tc.expectedUri != uri {
				t.Errorf("got %s, want %s", uri, tc.expectedUri)
			}
		})
	}
}

func TestCreateFilestoreEndpointUrlBasePath(t *testing.T) {
	var (
		testBasePath    = "https://" + testEndpoint + "/"
		stagingBasePath = "https://" + stagingEndpoint + "/"
	)
	_ = testBasePath
	tests := []struct {
		name          string
		inputurl      string
		opurl         string
		errorExpected bool
	}{
		{
			name:  "tc1 - empty endpoint, prod base path picked",
			opurl: prodBasePath,
		},
		{
			name:     "tc1 - test endpoint",
			inputurl: testEndpoint,
			opurl:    testBasePath,
		},
		{
			name:     "tc2 - staging endpoint",
			inputurl: stagingEndpoint,
			opurl:    stagingBasePath,
		},
		{
			name:     "tc3 - prod endpoint",
			inputurl: prodEndpoint,
			opurl:    prodBasePath,
		},
		{
			name:          "tc4 - invalid endpoint",
			inputurl:      "Test-file.googleapis.com",
			errorExpected: true,
		},
		{
			name:          "tc5 - invalid endpoint",
			inputurl:      "random.com",
			errorExpected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			url, err := createFilestoreEndpointUrlBasePath(tc.inputurl)
			if err != nil && !tc.errorExpected {
				t.Errorf("unexpected error %v", err)
			}
			if err == nil && tc.errorExpected {
				t.Errorf("expected error got nil")
			}
			if tc.opurl != url {
				t.Errorf("got %s, want %s", url, tc.opurl)
			}
		})
	}
}

func TestIsContextError(t *testing.T) {
	cases := []struct {
		name            string
		err             error
		expectedErrCode *codes.Code
	}{
		{
			name:            "deadline exceeded error",
			err:             context.DeadlineExceeded,
			expectedErrCode: util.ErrCodePtr(codes.DeadlineExceeded),
		},
		{
			name:            "contains 'context deadline exceeded'",
			err:             fmt.Errorf("got error: %w", context.DeadlineExceeded),
			expectedErrCode: util.ErrCodePtr(codes.DeadlineExceeded),
		},
		{
			name:            "context canceled error",
			err:             context.Canceled,
			expectedErrCode: util.ErrCodePtr(codes.Canceled),
		},
		{
			name:            "contains 'context canceled'",
			err:             fmt.Errorf("got error: %w", context.Canceled),
			expectedErrCode: util.ErrCodePtr(codes.Canceled),
		},
		{
			name:            "does not contain 'context canceled' or 'context deadline exceeded'",
			err:             fmt.Errorf("unknown error"),
			expectedErrCode: nil,
		},
		{
			name:            "nil error",
			err:             nil,
			expectedErrCode: nil,
		},
	}

	for _, test := range cases {
		errCode := isContextError(test.err)
		if (test.expectedErrCode == nil) != (errCode == nil) {
			t.Errorf("test %v failed: got %v, expected %v", test.name, errCode, test.expectedErrCode)
		}
		if test.expectedErrCode != nil && *errCode != *test.expectedErrCode {
			t.Errorf("test %v failed: got %v, expected %v", test.name, errCode, test.expectedErrCode)
		}
	}
}

func TestCodeForError(t *testing.T) {
	getGoogleAPIWrappedError := func(err error) *googleapi.Error {
		apierr, _ := apierror.ParseError(err, false)
		wrappedError := &googleapi.Error{}
		wrappedError.Wrap(apierr)

		return wrappedError
	}
	getAPIError := func(err error) *apierror.APIError {
		apierror, _ := apierror.ParseError(err, true)
		return apierror
	}
	cases := []struct {
		name            string
		err             error
		expectedErrCode *codes.Code
	}{
		{
			name: "googleapi.Error that wraps apierror.APIError of http kind",
			err: getGoogleAPIWrappedError(&googleapi.Error{
				Code:    404,
				Message: "data requested not found error",
			}),
			expectedErrCode: util.ErrCodePtr(codes.NotFound),
		},
		{
			name: "googleapi.Error that wraps apierror.APIError of status kind",
			err: getGoogleAPIWrappedError(status.New(
				codes.Internal, "Internal status error",
			).Err()),
			expectedErrCode: util.ErrCodePtr(codes.Internal),
		},
		{
			name: "apierror.APIError of status kind",
			err: getAPIError(status.New(
				codes.Canceled, "Internal status error",
			).Err()),
			expectedErrCode: util.ErrCodePtr(codes.Canceled),
		},
		{
			name: "apierror.APIError of http kind",
			err: getAPIError(&googleapi.Error{
				Code:    404,
				Message: "data requested not found error",
			}),
			expectedErrCode: util.ErrCodePtr(codes.NotFound),
		},
		{
			name:            "apierror.APIError wrapped 429 http error",
			err:             fmt.Errorf("got error: %w", &googleapi.Error{Code: http.StatusTooManyRequests}),
			expectedErrCode: util.ErrCodePtr(codes.ResourceExhausted),
		},
		{
			name:            "apierror.APIError  wrapped 400 http error",
			err:             fmt.Errorf("got error: %w", &googleapi.Error{Code: http.StatusBadRequest}),
			expectedErrCode: util.ErrCodePtr(codes.InvalidArgument),
		},
		{
			name:            "apierror.APIError  wrapped 403 http error",
			err:             fmt.Errorf("got error: %w", &googleapi.Error{Code: http.StatusForbidden}),
			expectedErrCode: util.ErrCodePtr(codes.PermissionDenied),
		},
		{
			name:            "RESOURCE_EXHAUSTED error",
			err:             fmt.Errorf("got error: RESOURCE_EXHAUSTED: Operation rate exceeded"),
			expectedErrCode: util.ErrCodePtr(codes.ResourceExhausted),
		},
		{
			name:            "deadline exceeded error",
			err:             context.DeadlineExceeded,
			expectedErrCode: util.ErrCodePtr(codes.DeadlineExceeded),
		},
		{
			name:            "contains 'context deadline exceeded'",
			err:             fmt.Errorf("got error: %w", context.DeadlineExceeded),
			expectedErrCode: util.ErrCodePtr(codes.DeadlineExceeded),
		},
		{
			name:            "context canceled error",
			err:             context.Canceled,
			expectedErrCode: util.ErrCodePtr(codes.Canceled),
		},
		{
			name:            "contains 'context canceled'",
			err:             fmt.Errorf("got error: %w", context.Canceled),
			expectedErrCode: util.ErrCodePtr(codes.Canceled),
		},
		{
			name:            "does not contain 'context canceled' or 'context deadline exceeded'",
			err:             fmt.Errorf("unknown error"),
			expectedErrCode: util.ErrCodePtr(codes.Internal),
		},
		{
			name:            "404 googleapi error",
			err:             &googleapi.Error{Code: http.StatusNotFound},
			expectedErrCode: util.ErrCodePtr(codes.NotFound),
		},
		{
			name:            "wrapped 404 googleapi error",
			err:             fmt.Errorf("got error: %w", &googleapi.Error{Code: http.StatusNotFound}),
			expectedErrCode: util.ErrCodePtr(codes.NotFound),
		},
		{
			name:            "status error with Aborted error code",
			err:             status.Error(codes.Aborted, "aborted error"),
			expectedErrCode: util.ErrCodePtr(codes.Aborted),
		},
		{
			name:            "nil error",
			err:             nil,
			expectedErrCode: nil,
		},
		{
			name:            "Filestore system limit error",
			err:             fmt.Errorf("got error: System limit for internal resources has been reached"),
			expectedErrCode: util.ErrCodePtr(codes.ResourceExhausted),
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			errCode := codeForError(test.err)
			if (test.expectedErrCode == nil) != (errCode == nil) {
				t.Errorf("test %v failed: got %v, expected %v", test.name, errCode, test.expectedErrCode)
			}
			if test.expectedErrCode != nil && *errCode != *test.expectedErrCode {
				t.Errorf("test %v failed: got %v, expected %v", test.name, errCode, test.expectedErrCode)
			}
		})
	}
}

func TestStatusError(t *testing.T) {
	cases := []struct {
		name        string
		err         error
		expectedErr error
	}{
		{
			name:        "404 googleapi error",
			err:         &googleapi.Error{Code: http.StatusNotFound},
			expectedErr: status.Error(codes.NotFound, ""),
		},
		{
			name:        "nil error",
			err:         nil,
			expectedErr: nil,
		},
	}

	for _, test := range cases {
		err := StatusError(test.err)
		if (test.expectedErr == nil) != (err == nil) {
			t.Errorf("test %v failed: got %v, expected %v", test.name, err, test.expectedErr)
		}
	}
}

func TestIsUserError(t *testing.T) {
	cases := []struct {
		name            string
		err             error
		expectedErrCode *codes.Code
	}{
		{
			name:            "nil error",
			err:             nil,
			expectedErrCode: nil,
		},
		{
			name:            "RESOURCE_EXHAUSTED error",
			err:             fmt.Errorf("got error: RESOURCE_EXHAUSTED: Operation rate exceeded"),
			expectedErrCode: util.ErrCodePtr(codes.ResourceExhausted),
		},
		{
			name:            "INVALID_ARGUMENT error",
			err:             fmt.Errorf("got error: INVALID_ARGUMENT"),
			expectedErrCode: util.ErrCodePtr(codes.InvalidArgument),
		},
		{
			name:            "PERMISSION_DENIED error",
			err:             fmt.Errorf("got error: PERMISSION_DENIED"),
			expectedErrCode: util.ErrCodePtr(codes.PermissionDenied),
		},
		{
			name:            "NOT_FOUND error",
			err:             fmt.Errorf("got error: NOT_FOUND"),
			expectedErrCode: util.ErrCodePtr(codes.NotFound),
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			errCode := isUserOperationError(test.err)
			if (test.expectedErrCode == nil) != (errCode == nil) {
				t.Errorf("test %v failed: got %v, expected %v", test.name, errCode, test.expectedErrCode)
			}
			if test.expectedErrCode != nil && *errCode != *test.expectedErrCode {
				t.Errorf("test %v failed: got %v, expected %v", test.name, errCode, test.expectedErrCode)
			}
		})
	}
}

func TestCreateVolumePoolVolume(t *testing.T) {
	parentPool := "projects/test-project/locations/us-central1/volumePools/test-pool"
	volID := "vol-123"

	tests := []struct {
		name         string
		responseCode int
		responseBody *VolumePoolVolume
		expectErr    bool
		expected     *PoolVolume
	}{
		{
			name:         "successful create VolumePool volume",
			responseCode: http.StatusOK,
			responseBody: &VolumePoolVolume{
				Name: fmt.Sprintf("%s/volumes/%s", parentPool, volID),
				MountPoint: &VolumePoolMountPoint{
					IpAddress: "10.0.0.1",
					MountName: "vol_123",
				},
			},
			expected: &PoolVolume{
				Name:      fmt.Sprintf("%s/volumes/%s", parentPool, volID),
				IpAddress: "10.0.0.1",
				MountName: "vol_123",
			},
		},
		{
			name:         "api error response",
			responseCode: http.StatusBadRequest,
			expectErr:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedPath := fmt.Sprintf("/v1beta1/%s/volumes", parentPool)
				if r.URL.Path != expectedPath {
					t.Errorf("expected URL path %q, got %q", expectedPath, r.URL.Path)
				}
				if r.Method != http.MethodPost {
					t.Errorf("expected method %s, got %s", http.MethodPost, r.Method)
				}
				if r.URL.Query().Get("volumeId") != volID {
					t.Errorf("expected volumeId %q, got %q", volID, r.URL.Query().Get("volumeId"))
				}

				if tc.responseBody != nil {
					var reqBody VolumePoolVolume
					if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
						t.Errorf("failed to decode request body: %v", err)
					}
				}

				w.WriteHeader(tc.responseCode)
				json.NewEncoder(w).Encode(tc.responseBody)
			}))
			defer server.Close()

			ctx := context.Background()
			fileService, err := filev1beta1.NewService(ctx, option.WithHTTPClient(server.Client()))
			if err != nil {
				t.Fatalf("failed to create service client: %v", err)
			}
			fileService.BasePath = server.URL + "/"

			manager := &gcfsServiceManager{
				fileService: fileService,
				httpClient:  server.Client(),
			}

			resp, err := manager.CreateVolumePoolVolume(ctx, parentPool, volID)
			if tc.expectErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(resp, tc.expected); diff != "" {
				t.Errorf("unexpected volume pool: %s", diff)
			}
		})
	}
}

func TestDeleteVolumePoolVolume(t *testing.T) {
	name := "projects/test-project/locations/us-central1/volumePools/test-pool/volumes/vol-123"

	tests := []struct {
		testName     string
		responseCode int
		responseBody string
		expectErr    bool
		wantCode     codes.Code
	}{
		{
			testName:     "successful release",
			responseCode: http.StatusOK,
		},
		{
			testName:     "volume does not exist",
			responseCode: http.StatusNotFound,
		},
		{
			testName:     "api error response",
			responseCode: http.StatusBadRequest,
			expectErr:    true,
			wantCode:     codes.InvalidArgument,
		},
		{
			testName:     "canonical code in the error body",
			responseCode: http.StatusBadRequest,
			responseBody: apiErrorBody(http.StatusBadRequest, "FAILED_PRECONDITION", "volume is busy"),
			expectErr:    true,
			wantCode:     codes.FailedPrecondition,
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedPath := fmt.Sprintf("/v1beta1/%s", name)
				if r.URL.Path != expectedPath {
					t.Errorf("expected URL path %q, got %q", expectedPath, r.URL.Path)
				}
				if r.Method != http.MethodDelete {
					t.Errorf("expected method %s, got %s", http.MethodDelete, r.Method)
				}

				w.WriteHeader(tc.responseCode)
				fmt.Fprint(w, tc.responseBody)
			}))
			defer server.Close()

			ctx := context.Background()
			fileService, err := filev1beta1.NewService(ctx, option.WithHTTPClient(server.Client()))
			if err != nil {
				t.Fatalf("failed to create service client: %v", err)
			}
			fileService.BasePath = server.URL + "/"

			manager := &gcfsServiceManager{
				fileService: fileService,
				httpClient:  server.Client(),
			}

			err = manager.DeleteVolumePoolVolume(ctx, name)
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if code := status.Code(VolumePoolStatusError(err)); code != tc.wantCode {
					t.Errorf("DeleteVolumePoolVolume() = %v (code %v), want code %v", err, code, tc.wantCode)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestExtractNfsShareExportOptionsSetsSourceNetwork(t *testing.T) {
	const network = "projects/test-project/global/networks/test-network"

	got := extractNfsShareExportOptions(
		[]*NfsExportOptions{
			{
				IpRanges: []string{"10.0.0.0/24"},
			},
		},
		network,
	)

	if len(got) != 1 {
		t.Fatalf("got %d options, want 1", len(got))
	}
	if got[0].Network != network {
		t.Errorf("got network %q, want %q", got[0].Network, network)
	}
}

// newVolumePoolTestManager returns a gcfsServiceManager that sends its requests,
// including polls of long-running operations, to server. It also shortens the
// polling of volume pool operations for the duration of the test.
func newVolumePoolTestManager(t *testing.T, server *httptest.Server) *gcfsServiceManager {
	t.Helper()
	fileService, err := filev1beta1.NewService(context.Background(), option.WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatalf("failed to create service client: %v", err)
	}
	fileService.BasePath = server.URL + "/"

	interval, timeout := volumePoolOpPollInterval, volumePoolOpTimeout
	volumePoolOpPollInterval, volumePoolOpTimeout = 5*time.Millisecond, 500*time.Millisecond
	t.Cleanup(func() { volumePoolOpPollInterval, volumePoolOpTimeout = interval, timeout })

	return &gcfsServiceManager{
		fileService:       fileService,
		httpClient:        server.Client(),
		operationsService: filev1beta1.NewProjectsLocationsOperationsService(fileService),
		backupService:     filev1beta1.NewProjectsLocationsBackupsService(fileService),
	}
}

func apiErrorBody(code int, status, message string) string {
	return fmt.Sprintf(`{"error": {"code": %d, "message": %q, "status": %q}}`, code, message, status)
}

func TestGetVolumePoolVolume(t *testing.T) {
	name := "projects/test-project/locations/us-central1/volumePools/test-pool/volumes/vol-123"

	tests := []struct {
		name         string
		responseCode int
		responseBody string
		want         *PoolVolume
		wantCode     codes.Code
	}{
		{
			name:         "HOT volume",
			responseCode: http.StatusOK,
			responseBody: fmt.Sprintf(`{"name": %q, "mountPoint": {"ipAddress": "10.0.0.1", "mountName": "vol_123"}, "quotaMib": "2048", "state": "HOT"}`, name),
			want:         &PoolVolume{Name: name, IpAddress: "10.0.0.1", MountName: "vol_123", State: VolumePoolVolumeStateHot},
		},
		{
			name:         "COLD volume has no mount point",
			responseCode: http.StatusOK,
			responseBody: fmt.Sprintf(`{"name": %q, "quotaMib": "1024", "state": "COLD"}`, name),
			want:         &PoolVolume{Name: name, State: VolumePoolVolumeStateCold},
		},
		{
			name:         "volume does not exist",
			responseCode: http.StatusNotFound,
			responseBody: apiErrorBody(http.StatusNotFound, "NOT_FOUND", "volume vol-123 not found"),
			wantCode:     codes.NotFound,
		},
		{
			name:         "not found without an error body",
			responseCode: http.StatusNotFound,
			responseBody: "not found",
			wantCode:     codes.NotFound,
		},
		{
			name:         "failed precondition keeps its code",
			responseCode: http.StatusBadRequest,
			responseBody: apiErrorBody(http.StatusBadRequest, "FAILED_PRECONDITION", "Volume Pools are not supported"),
			wantCode:     codes.FailedPrecondition,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/v1beta1/"+name {
					t.Errorf("got %s %s, want GET /v1beta1/%s", r.Method, r.URL.Path, name)
				}
				w.WriteHeader(tc.responseCode)
				fmt.Fprint(w, tc.responseBody)
			}))
			defer server.Close()

			got, err := newVolumePoolTestManager(t, server).GetVolumePoolVolume(context.Background(), name)
			if code := status.Code(err); code != tc.wantCode {
				t.Fatalf("GetVolumePoolVolume() error = %v (code %v), want code %v", err, code, tc.wantCode)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("GetVolumePoolVolume() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCreateVolumePoolVolumeFromSource(t *testing.T) {
	parentPool := "projects/test-project/locations/us-central1/volumePools/test-pool"
	volumeName := parentPool + "/volumes/vol-123"
	backupName := "projects/test-project/locations/us-central1/backups/snap-1"
	opName := "projects/test-project/locations/us-central1/operations/op-1"
	runningOp := fmt.Sprintf(`{"name": %q, "done": false}`, opName)

	tests := []struct {
		name         string
		createCode   int
		createBody   string
		opBodies     []string // returned by successive operation polls; the last one repeats
		want         *PoolVolume
		wantCode     codes.Code
		wantGetCalls int
	}{
		{
			name:       "restores the volume",
			createCode: http.StatusOK,
			createBody: runningOp,
			opBodies: []string{
				runningOp,
				fmt.Sprintf(`{"name": %q, "done": true, "response": {"volume": {"name": %q}}}`, opName, volumeName),
			},
			want:         &PoolVolume{Name: volumeName, IpAddress: "10.0.0.2", MountName: "vol_123", State: VolumePoolVolumeStateHot},
			wantGetCalls: 1,
		},
		{
			name:       "operation fails with a canonical code",
			createCode: http.StatusOK,
			createBody: runningOp,
			opBodies:   []string{fmt.Sprintf(`{"name": %q, "done": true, "error": {"code": 9, "message": "backup is not in READY state"}}`, opName)},
			wantCode:   codes.FailedPrecondition,
		},
		{
			name:       "operation is still running when the wait ends",
			createCode: http.StatusOK,
			createBody: runningOp,
			opBodies:   []string{runningOp},
			wantCode:   codes.DeadlineExceeded,
		},
		{
			name:       "volume already exists",
			createCode: http.StatusConflict,
			createBody: apiErrorBody(http.StatusConflict, "ALREADY_EXISTS", "volume vol-123 already exists"),
			wantCode:   codes.AlreadyExists,
		},
		{
			name:       "invalid backup",
			createCode: http.StatusBadRequest,
			createBody: apiErrorBody(http.StatusBadRequest, "INVALID_ARGUMENT", "backup snap-1 was created from volume pool other-pool"),
			wantCode:   codes.InvalidArgument,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var requests []map[string]interface{}
			opPolls, getCalls := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodPost && r.URL.Path == "/v1beta1/"+parentPool+"/volumes:createFromSource":
					var body map[string]interface{}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Errorf("failed to decode request body: %v", err)
					}
					requests = append(requests, body)
					w.WriteHeader(tc.createCode)
					fmt.Fprint(w, tc.createBody)
				case r.Method == http.MethodGet && r.URL.Path == "/v1beta1/"+opName:
					fmt.Fprint(w, tc.opBodies[min(opPolls, len(tc.opBodies)-1)])
					opPolls++
				case r.Method == http.MethodGet && r.URL.Path == "/v1beta1/"+volumeName:
					getCalls++
					fmt.Fprintf(w, `{"name": %q, "mountPoint": {"ipAddress": "10.0.0.2", "mountName": "vol_123"}, "quotaMib": "1024", "state": "HOT"}`, volumeName)
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			manager := newVolumePoolTestManager(t, server)

			got, err := manager.CreateVolumePoolVolumeFromSource(context.Background(), parentPool, "vol-123", backupName)
			if code := status.Code(err); code != tc.wantCode {
				t.Fatalf("CreateVolumePoolVolumeFromSource() error = %v (code %v), want code %v", err, code, tc.wantCode)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("CreateVolumePoolVolumeFromSource() mismatch (-want +got):\n%s", diff)
			}
			if getCalls != tc.wantGetCalls {
				t.Errorf("volume was read %d times, want %d", getCalls, tc.wantGetCalls)
			}

			if len(requests) != 1 {
				t.Fatalf("got %d createFromSource requests, want 1", len(requests))
			}
			body := requests[0]
			if body["volumeId"] != "vol-123" || body["backup"] != backupName {
				t.Errorf("request body = %v, want volumeId vol-123 and backup %s", body, backupName)
			}
			if _, ok := body["quota"]; ok {
				t.Errorf("request body = %v, want no quota (the API rejects it)", body)
			}
			if id, _ := body["requestId"].(string); uuid.Validate(id) != nil {
				t.Errorf("requestId = %q, want a UUID", id)
			}
		})
	}
}

func TestCreateVolumePoolVolumeFromSource_NewRequestIDPerCall(t *testing.T) {
	parentPool := "projects/test-project/locations/us-central1/volumePools/test-pool"
	var requestIDs []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			RequestID string `json:"requestId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		requestIDs = append(requestIDs, body.RequestID)
		w.WriteHeader(http.StatusConflict)
		fmt.Fprint(w, apiErrorBody(http.StatusConflict, "ALREADY_EXISTS", "volume vol-123 already exists"))
	}))
	defer server.Close()
	manager := newVolumePoolTestManager(t, server)

	for i := 0; i < 2; i++ {
		if _, err := manager.CreateVolumePoolVolumeFromSource(context.Background(), parentPool, "vol-123", "projects/test-project/locations/us-central1/backups/snap-1"); status.Code(err) != codes.AlreadyExists {
			t.Fatalf("CreateVolumePoolVolumeFromSource() error = %v, want code AlreadyExists", err)
		}
	}
	if len(requestIDs) != 2 || requestIDs[0] == "" || requestIDs[0] == requestIDs[1] {
		t.Errorf("request IDs = %q, want two different IDs", requestIDs)
	}
}

func TestCreateBackup_VolumePool(t *testing.T) {
	sourceVolume := "projects/test-project/locations/us-central1/volumePools/test-pool/volumes/vol-123"
	backupURI := "projects/test-project/locations/us-central1/backups/snap-1"
	opName := "projects/test-project/locations/us-central1/operations/op-1"
	backupInfo := &BackupInfo{
		Name:           "snap-1",
		SourceVolumeId: "volumepool://test-project/us-central1/test-pool/volumes/vol-123?ip=10.0.0.1",
		SourceVolume:   sourceVolume,
		BackupURI:      backupURI,
		Project:        "test-project",
		Location:       "us-central1",
		Labels:         map[string]string{"storage_gke_io_created-by": "test-driver"},
	}

	tests := []struct {
		name        string
		createCode  int
		createBody  string
		opBody      string // returned by every operation poll; done without an error if empty
		backupState string // state of the finished backup; READY if empty
		wantCode    codes.Code
	}{
		{
			name:       "creates a backup of the volume",
			createCode: http.StatusOK,
			createBody: fmt.Sprintf(`{"name": %q, "done": false}`, opName),
		},
		{
			name:       "volume is busy",
			createCode: http.StatusBadRequest,
			createBody: apiErrorBody(http.StatusBadRequest, "FAILED_PRECONDITION", "Volume vol-123 is currently in use by another operation"),
			wantCode:   codes.FailedPrecondition,
		},
		{
			name:       "operation fails with a canonical code",
			createCode: http.StatusOK,
			createBody: fmt.Sprintf(`{"name": %q, "done": false}`, opName),
			opBody:     fmt.Sprintf(`{"name": %q, "done": true, "error": {"code": 9, "message": "volume is not HOT"}}`, opName),
			wantCode:   codes.FailedPrecondition,
		},
		{
			name:       "operation is still running when the wait ends",
			createCode: http.StatusOK,
			createBody: fmt.Sprintf(`{"name": %q, "done": false}`, opName),
			opBody:     fmt.Sprintf(`{"name": %q, "done": false}`, opName),
			wantCode:   codes.DeadlineExceeded,
		},
		{
			name:        "finished backup is not READY",
			createCode:  http.StatusOK,
			createBody:  fmt.Sprintf(`{"name": %q, "done": false}`, opName),
			backupState: "INVALID",
			wantCode:    codes.Internal,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var created map[string]interface{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodPost && r.URL.Path == "/v1beta1/projects/test-project/locations/us-central1/backups":
					if got := r.URL.Query().Get("backupId"); got != "snap-1" {
						t.Errorf("backupId = %q, want snap-1", got)
					}
					if err := json.NewDecoder(r.Body).Decode(&created); err != nil {
						t.Errorf("failed to decode request body: %v", err)
					}
					w.WriteHeader(tc.createCode)
					fmt.Fprint(w, tc.createBody)
				case r.Method == http.MethodGet && r.URL.Path == "/v1beta1/"+opName:
					if tc.opBody != "" {
						fmt.Fprint(w, tc.opBody)
						return
					}
					fmt.Fprintf(w, `{"name": %q, "done": true}`, opName)
				case r.Method == http.MethodGet && r.URL.Path == "/v1beta1/"+backupURI:
					state := tc.backupState
					if state == "" {
						state = "READY"
					}
					fmt.Fprintf(w, `{"name": %q, "state": %q, "sourceVolume": %q, "createTime": "2026-10-19T10:00:00Z"}`, backupURI, state, sourceVolume)
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			manager := newVolumePoolTestManager(t, server)

			backup, err := manager.CreateBackup(context.Background(), backupInfo)
			if code := status.Code(err); code != tc.wantCode {
				t.Fatalf("CreateBackup() error = %v (code %v), want code %v", err, code, tc.wantCode)
			}

			if created["sourceVolume"] != sourceVolume {
				t.Errorf("request body = %v, want sourceVolume %s", created, sourceVolume)
			}
			for _, field := range []string{"sourceInstance", "sourceFileShare"} {
				if _, ok := created[field]; ok {
					t.Errorf("request body = %v, want no %s", created, field)
				}
			}
			if tc.wantCode != codes.OK {
				return
			}
			if backup.Name != backupURI {
				t.Errorf("backup name = %q, want %q", backup.Name, backupURI)
			}

			got, err := manager.GetBackup(context.Background(), backupURI)
			if err != nil {
				t.Fatalf("GetBackup() error = %v", err)
			}
			if got.SourceVolume != sourceVolume {
				t.Errorf("GetBackup() SourceVolume = %q, want %q", got.SourceVolume, sourceVolume)
			}
		})
	}
}

func TestVolumePoolAPIError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode codes.Code
	}{
		{
			name:     "canonical status in the body",
			err:      &googleapi.Error{Code: http.StatusBadRequest, Message: "busy", Body: apiErrorBody(http.StatusBadRequest, "FAILED_PRECONDITION", "busy")},
			wantCode: codes.FailedPrecondition,
		},
		{
			name:     "conflict with ALREADY_EXISTS",
			err:      &googleapi.Error{Code: http.StatusConflict, Body: apiErrorBody(http.StatusConflict, "ALREADY_EXISTS", "exists")},
			wantCode: codes.AlreadyExists,
		},
		{
			name:     "wrapped API error",
			err:      fmt.Errorf("call failed: %w", &googleapi.Error{Code: http.StatusBadRequest, Body: apiErrorBody(http.StatusBadRequest, "FAILED_PRECONDITION", "busy")}),
			wantCode: codes.FailedPrecondition,
		},
		{
			name:     "no body: code from the HTTP status",
			err:      &googleapi.Error{Code: http.StatusNotFound},
			wantCode: codes.NotFound,
		},
		{
			name:     "unknown HTTP status",
			err:      &googleapi.Error{Code: http.StatusBadGateway, Body: "bad gateway"},
			wantCode: codes.Internal,
		},
		{
			name:     "not an API error",
			err:      context.DeadlineExceeded,
			wantCode: codes.DeadlineExceeded,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := volumePoolAPIError(tc.err, "get volume")
			if code := status.Code(StatusError(err)); code != tc.wantCode {
				t.Errorf("volumePoolAPIError() = %v (code %v), want code %v", err, code, tc.wantCode)
			}
			if msg := status.Convert(err).Message(); !strings.HasPrefix(msg, "get volume") {
				t.Errorf("volumePoolAPIError() message = %q, want it to start with the call", msg)
			}
		})
	}
}

func TestVolumePoolStatusError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode codes.Code
	}{
		{name: "nil", err: nil, wantCode: codes.OK},
		{
			// Errors from the generated Go client implement GRPCStatus with a
			// code derived from the HTTP status (InvalidArgument here).
			name:     "Go client error with a canonical status in the body",
			err:      &googleapi.Error{Code: http.StatusBadRequest, Body: apiErrorBody(http.StatusBadRequest, "FAILED_PRECONDITION", "busy")},
			wantCode: codes.FailedPrecondition,
		},
		{
			name:     "wrapped API error",
			err:      fmt.Errorf("get backup: %w", &googleapi.Error{Code: http.StatusConflict, Body: apiErrorBody(http.StatusConflict, "ALREADY_EXISTS", "exists")}),
			wantCode: codes.AlreadyExists,
		},
		{name: "API error without a body", err: &googleapi.Error{Code: http.StatusNotFound}, wantCode: codes.NotFound},
		{name: "status error", err: status.Error(codes.Aborted, "busy"), wantCode: codes.Aborted},
		{name: "context error", err: fmt.Errorf("get volume: %w", context.DeadlineExceeded), wantCode: codes.DeadlineExceeded},
		{name: "other error", err: errors.New("boom"), wantCode: codes.Internal},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if code := status.Code(VolumePoolStatusError(tc.err)); code != tc.wantCode {
				t.Errorf("VolumePoolStatusError(%v) code = %v, want %v", tc.err, code, tc.wantCode)
			}
		})
	}
}

// The caller's deadline ends the wait before volumePoolOpTimeout does. In
// production, the CO's timeout usually ends it first.
func TestWaitForVolumePoolOp_ContextDeadline(t *testing.T) {
	opName := "projects/test-project/locations/us-central1/operations/op-1"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"name": %q, "done": false}`, opName)
	}))
	defer server.Close()
	manager := newVolumePoolTestManager(t, server)
	volumePoolOpTimeout = time.Minute

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := manager.waitForVolumePoolOp(ctx, &filev1beta1.Operation{Name: opName})
	if code := status.Code(err); code != codes.DeadlineExceeded {
		t.Fatalf("waitForVolumePoolOp() error = %v (code %v), want code %v", err, code, codes.DeadlineExceeded)
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Errorf("waitForVolumePoolOp() returned after %v, want it to stop at the context deadline", elapsed)
	}
}
