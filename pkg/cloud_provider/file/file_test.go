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
