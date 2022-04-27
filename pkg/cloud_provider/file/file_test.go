package file

import (
	"strings"
	"testing"

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
