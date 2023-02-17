/*
Copyright 2018 The Kubernetes Authors.

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
	"testing"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	cloud "sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider"
)

func initTestDriver(t *testing.T) *GCFSDriver {
	c, err := cloud.NewFakeCloud()
	if err != nil {
		t.Fatalf("Failed to init cloud")
	}

	config := &GCFSDriverConfig{
		Name:     "test-driver",
		NodeName: "test-node",
		Version:  "test-version",
		RunNode:  true,
		Cloud:    c,
	}
	driver, err := NewGCFSDriver(config)
	if err != nil {
		t.Fatalf("failed to init driver: %v", err)
	}
	if driver == nil {
		t.Fatalf("driver is nil")
	}
	return driver
}

func TestDriverValidateVolumeCapability(t *testing.T) {
	driver := initTestDriver(t)

	cases := []struct {
		name       string
		capability *csi.VolumeCapability
		expectErr  bool
	}{
		{
			name:       "nil caps",
			capability: nil,
			expectErr:  true,
		},
		{
			name: "missing access type",
			capability: &csi.VolumeCapability{
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
				},
			},
			expectErr: true,
		},
		{
			name: "missing access mode",
			capability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{},
				},
			},
			expectErr: true,
		},
		{
			name: "mount, snw ",
			capability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{},
				},
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
				},
			},
		},
		{
			name: "mount, snr ",
			capability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{},
				},
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY,
				},
			},
		},
		{
			name: "mount, mnr ",
			capability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{},
				},
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY,
				},
			},
		},
		{
			name: "mount, mnsw ",
			capability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{},
				},
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_SINGLE_WRITER,
				},
			},
		},
		{
			name: "mount, mnmw ",
			capability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{},
				},
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
				},
			},
		},
		{
			name: "mount, invalid fstype",
			capability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{FsType: "abc"},
				},
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
				},
			},
			// TODO: uncomment after https://github.com/kubernetes-csi/external-provisioner/issues/328
			// expectErr: true,
		},
		{
			name: "mount, unknown accessmode",
			capability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{},
				},
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_UNKNOWN,
				},
			},
			expectErr: true,
		},
		{
			name: "block, mnmw ",
			capability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Block{
					Block: &csi.VolumeCapability_BlockVolume{},
				},
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
				},
			},
			expectErr: true,
		},
	}

	for _, test := range cases {
		err := driver.validateVolumeCapability(test.capability)
		if err != nil && !test.expectErr {
			t.Errorf("test %q failed: %v", test.name, err)
		}
		if err == nil && test.expectErr {
			t.Errorf("test %q failed: got success", test.name)
		}
	}
}

func TestDriverValidateVolumeCapabilities(t *testing.T) {
	driver := initTestDriver(t)

	cases := []struct {
		name         string
		capabilities []*csi.VolumeCapability
		expectErr    bool
	}{
		{
			name:         "nil caps",
			capabilities: nil,
			expectErr:    true,
		},
		{
			name: "multiple good capabilities",
			capabilities: []*csi.VolumeCapability{
				{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
					},
				},
				{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY,
					},
				},
			},
		},
		{
			name:      "multiple bad capabilities",
			expectErr: true,
			capabilities: []*csi.VolumeCapability{
				{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
					},
				},
				{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_UNKNOWN,
					},
				},
			},
		},
	}

	for _, test := range cases {
		err := driver.validateVolumeCapabilities(test.capabilities)
		if err != nil && !test.expectErr {
			t.Errorf("test %q failed: %v", test.name, err)
		}
		if err == nil && test.expectErr {
			t.Errorf("test %q failed: got success", test.name)
		}
	}
}
