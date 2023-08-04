package file

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

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
		project, location, instance, err := getInstanceNameFromURI(test.uri)
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
	}

	for _, test := range cases {
		errCode := CodeForError(test.err)
		if (test.expectedErrCode == nil) != (errCode == nil) {
			t.Errorf("test %v failed: got %v, expected %v", test.name, errCode, test.expectedErrCode)
		}
		if test.expectedErrCode != nil && *errCode != *test.expectedErrCode {
			t.Errorf("test %v failed: got %v, expected %v", test.name, errCode, test.expectedErrCode)
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
			name:            "404 http error",
			err:             &googleapi.Error{Code: http.StatusNotFound},
			expectedErrCode: util.ErrCodePtr(codes.NotFound),
		},
		{
			name:            "wrapped 404 http error",
			err:             fmt.Errorf("got error: %w", &googleapi.Error{Code: http.StatusNotFound}),
			expectedErrCode: util.ErrCodePtr(codes.NotFound),
		},
		{
			name:            "wrapped 429 http error",
			err:             fmt.Errorf("got error: %w", &googleapi.Error{Code: http.StatusTooManyRequests}),
			expectedErrCode: util.ErrCodePtr(codes.ResourceExhausted),
		},
		{
			name:            "wrapped 400 http error",
			err:             fmt.Errorf("got error: %w", &googleapi.Error{Code: http.StatusBadRequest}),
			expectedErrCode: util.ErrCodePtr(codes.InvalidArgument),
		},
		{
			name:            "wrapped 403 http error",
			err:             fmt.Errorf("got error: %w", &googleapi.Error{Code: http.StatusForbidden}),
			expectedErrCode: util.ErrCodePtr(codes.PermissionDenied),
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
		errCode := isUserError(test.err)
		if (test.expectedErrCode == nil) != (errCode == nil) {
			t.Errorf("test %v failed: got %v, expected %v", test.name, errCode, test.expectedErrCode)
		}
		if test.expectedErrCode != nil && *errCode != *test.expectedErrCode {
			t.Errorf("test %v failed: got %v, expected %v", test.name, errCode, test.expectedErrCode)
		}
	}
}
