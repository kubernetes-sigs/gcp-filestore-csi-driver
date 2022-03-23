/*
Copyright 2022 The Kubernetes Authors.

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
	"reflect"
	"testing"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	cloud "sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/file"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/util"
)

const (
	testDriverName           = "test.filestore"
	testDrivernameLabelValue = "test_filestore"
	testPVCName              = "testPVC"
	testPVCNamespace         = "testNamespace"
	testPVName               = "testPV"
)

func initTestMultishareController(t *testing.T) *MultishareController {
	fileService, err := file.NewFakeService()
	if err != nil {
		t.Fatalf("failed to initialize GCFS service: %v", err)
	}

	cloudProvider, err := cloud.NewFakeCloud()
	if err != nil {
		t.Fatalf("Failed to get cloud provider: %v", err)
	}
	return NewMultishareController(initTestDriver(t), fileService, cloudProvider, util.NewVolumeLocks())
}

func TestPickRegion(t *testing.T) {
	tests := []struct {
		name           string
		toporeq        *csi.TopologyRequirement
		expectErr      bool
		expectedRegion string
	}{
		{
			name:           "empty toppo",
			expectedRegion: "us-central1",
		},
		{
			name: "empty toppo req and preferred segments",
			toporeq: &csi.TopologyRequirement{
				Requisite: []*csi.Topology{
					{
						Segments: map[string]string{},
					},
				},
				Preferred: []*csi.Topology{
					{
						Segments: map[string]string{},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "empty toppo req segments",
			toporeq: &csi.TopologyRequirement{
				Requisite: []*csi.Topology{
					{
						Segments: map[string]string{},
					},
				},
				Preferred: []*csi.Topology{
					{
						Segments: map[string]string{
							TopologyKeyZone: "us-central1-c",
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "empty toppo preferred segments",
			toporeq: &csi.TopologyRequirement{
				Requisite: []*csi.Topology{
					{
						Segments: map[string]string{
							TopologyKeyZone: "us-central1-c",
						},
					},
				},
				Preferred: []*csi.Topology{
					{
						Segments: map[string]string{},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "empty preferred list",
			toporeq: &csi.TopologyRequirement{
				Requisite: []*csi.Topology{
					{
						Segments: map[string]string{
							TopologyKeyZone: "us-central1-c",
						},
					},
				},
			},
			expectedRegion: "us-central1",
		},
		{
			name: "empty req list",
			toporeq: &csi.TopologyRequirement{
				Preferred: []*csi.Topology{
					{
						Segments: map[string]string{
							TopologyKeyZone: "us-central1-c",
						},
					},
				},
			},
			expectedRegion: "us-central1",
		},
		{
			name: "non-empty preferred and req list",
			toporeq: &csi.TopologyRequirement{
				Requisite: []*csi.Topology{
					{
						Segments: map[string]string{
							TopologyKeyZone: "us-central1-a",
						},
					},
				},
				Preferred: []*csi.Topology{
					{
						Segments: map[string]string{
							TopologyKeyZone: "us-east1-a",
						},
					},
				},
			},
			expectedRegion: "us-east1",
		},
		{
			name: "malformed zone name in preferred list",
			toporeq: &csi.TopologyRequirement{
				Requisite: []*csi.Topology{
					{
						Segments: map[string]string{
							TopologyKeyZone: "us-central1-a",
						},
					},
				},
				Preferred: []*csi.Topology{
					{
						Segments: map[string]string{
							TopologyKeyZone: "us-east1a",
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "malformed zone name in req list",
			toporeq: &csi.TopologyRequirement{
				Requisite: []*csi.Topology{
					{
						Segments: map[string]string{
							TopologyKeyZone: "us-central1a",
						},
					},
				},
			},
			expectErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := initTestMultishareController(t)
			region, err := m.pickRegion(tc.toporeq)
			if tc.expectErr && err == nil {
				t.Error("expected error, got none")
			}
			if !tc.expectErr && err != nil {
				t.Error("unexpected error")
			}

			if tc.expectedRegion != region {
				t.Errorf("got %v, want %v", region, tc.expectedRegion)
			}
		})
	}
}

func TestGetShareRequestCapacity(t *testing.T) {
	tests := []struct {
		name             string
		cap              *csi.CapacityRange
		expectErr        bool
		expectedCapacity int64
	}{
		{
			name:      "req and limit not set",
			cap:       &csi.CapacityRange{},
			expectErr: true,
		},
		{
			name:             "empty cap range",
			expectedCapacity: 100 * util.Gb,
		},
		{
			name:      "cap range limit less than minimum supported size",
			expectErr: true,
			cap: &csi.CapacityRange{
				LimitBytes: 99 * util.Gb,
			},
		},
		{
			name:      "cap range limit more than maximum supported size",
			expectErr: true,
			cap: &csi.CapacityRange{
				LimitBytes: 2 * util.Tb,
			},
		},
		{
			name:      "cap range req less than minimum supported size",
			expectErr: true,
			cap: &csi.CapacityRange{
				RequiredBytes: 99 * util.Gb,
			},
		},
		{
			name:      "cap range req more than maximum supported size",
			expectErr: true,
			cap: &csi.CapacityRange{
				RequiredBytes: 2 * util.Tb,
			},
		},
		{
			name: "cap range req and limit set, limit less than req",
			cap: &csi.CapacityRange{
				RequiredBytes: 1 * util.Tb,
				LimitBytes:    100 * util.Gb,
			},
			expectErr: true,
		},
		{
			name: "cap range req and limit set",
			cap: &csi.CapacityRange{
				RequiredBytes: 100 * util.Gb,
				LimitBytes:    1 * util.Tb,
			},
			expectedCapacity: 1 * util.Tb,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			capacity, err := getShareRequestCapacity(tc.cap)
			if tc.expectErr && err == nil {
				t.Error("expected error, got none")
			}
			if !tc.expectErr && err != nil {
				t.Error("unexpected error")
			}

			if tc.expectedCapacity != capacity {
				t.Errorf("got %v, want %v", capacity, tc.expectedCapacity)
			}
		})
	}
}

func TestExtractInstanceLabels(t *testing.T) {
	tests := []struct {
		name          string
		params        map[string]string
		driver        string
		expectedLabel map[string]string
		expectErr     bool
	}{
		{
			name:   "empty params",
			driver: testDriverName,
			expectedLabel: map[string]string{
				tagKeyCreatedBy: testDrivernameLabelValue,
			},
		},
		{
			name:   "user labels",
			driver: testDriverName,
			params: map[string]string{
				ParameterKeyLabels:             "a=b,c=d",
				paramMultishareInstanceScLabel: "testsc",
			},
			expectedLabel: map[string]string{
				tagKeyCreatedBy:                        testDrivernameLabelValue,
				util.ParamMultishareInstanceScLabelKey: "testsc",
				"a":                                    "b",
				"c":                                    "d",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			label, err := extractInstanceLabels(tc.params, tc.driver)
			if tc.expectErr && err == nil {
				t.Error("expected error, got none")
			}
			if !tc.expectErr && err != nil {
				t.Error("unexpected error")
			}
			if len(label) != len(tc.expectedLabel) {
				t.Errorf("got len %v, want %v", len(label), len(tc.expectedLabel))
			}
			for k, v := range tc.expectedLabel {
				vgot, ok := label[k]
				if !ok {
					t.Errorf("key %v missing", k)
				}
				if vgot != v {
					t.Errorf("got %v, want %v", vgot, v)
				}
			}
		})
	}
}

func TestExtractShareLabels(t *testing.T) {
	tests := []struct {
		name          string
		params        map[string]string
		expectedLabel map[string]string
	}{
		{
			name: "empty params",
		},
		{
			name: "user labels ignored",
			params: map[string]string{
				ParameterKeyLabels:             "a=b,c=d",
				paramMultishareInstanceScLabel: "testsc",
			},
		},
		{
			name: "driver labels",
			params: map[string]string{
				ParameterKeyLabels:             "a=b,c=d",
				paramMultishareInstanceScLabel: "testsc",
				ParameterKeyPVCName:            testPVCName,
				ParameterKeyPVCNamespace:       testPVCNamespace,
				ParameterKeyPVName:             testPVName,
			},
			expectedLabel: map[string]string{
				tagKeyCreatedForClaimName:      testPVCName,
				tagKeyCreatedForClaimNamespace: testPVCNamespace,
				tagKeyCreatedForVolumeName:     testPVName,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			label := extractShareLabels(tc.params)
			if len(label) != len(tc.expectedLabel) {
				t.Errorf("got len %v, want %v", len(label), len(tc.expectedLabel))
			}
			for k, v := range tc.expectedLabel {
				vgot, ok := label[k]
				if !ok {
					t.Errorf("key %v missing", k)
				}
				if vgot != v {
					t.Errorf("got %v, want %v", vgot, v)
				}
			}
		})
	}
}

func TestGenerateNewMultishareInstance(t *testing.T) {
	tests := []struct {
		name             string
		instanceName     string
		req              *csi.CreateVolumeRequest
		expectedInstance *file.MultishareInstance
		expectErr        bool
	}{
		{
			name:         "non enterprise tier",
			instanceName: testInstanceName,
			req: &csi.CreateVolumeRequest{
				Parameters: map[string]string{
					paramTier: "standard",
				},
			},
			expectErr: true,
		},
		{
			name:         "invalid connect mode",
			instanceName: testInstanceName,
			req: &csi.CreateVolumeRequest{
				Parameters: map[string]string{
					paramConnectMode: "blah",
				},
			},
			expectErr: true,
		},
		{
			name:         "valid params",
			instanceName: testInstanceName,
			req: &csi.CreateVolumeRequest{
				Parameters: map[string]string{
					paramConnectMode:               directPeering,
					ParameterKeyLabels:             "a=b,c=d",
					paramMultishareInstanceScLabel: testInstanceScPrefix,
				},
			},
			expectedInstance: &file.MultishareInstance{
				Project:       "test-project",
				Location:      "us-central1",
				Name:          testInstanceName,
				CapacityBytes: util.MinMultishareInstanceSizeBytes,
				Network: file.Network{
					Name:        "default",
					ConnectMode: directPeering,
				},
				Tier:       enterpriseTier,
				KmsKeyName: "",
				Labels: map[string]string{
					"a":                                    "b",
					"c":                                    "d",
					tagKeyCreatedBy:                        "test-driver",
					util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := initTestMultishareController(t)
			filer, err := m.generateNewMultishareInstance(tc.instanceName, tc.req)
			if tc.expectErr && err == nil {
				t.Error("expected error, got none")
			}
			if !tc.expectErr && err != nil {
				t.Error("unexpected error")
			}
			if !reflect.DeepEqual(filer, tc.expectedInstance) {
				t.Errorf("got filer %+v, want %+v", filer, tc.expectedInstance)
			}
		})
	}
}

func TestGenerateCSICreateVolumeResponse(t *testing.T) {
	tests := []struct {
		name         string
		prefix       string
		share        *file.Share
		expectedResp *csi.CreateVolumeResponse
		expectError  bool
	}{
		{
			name:        "empty prefix",
			expectError: true,
		},
		{
			name:        "empty share object",
			prefix:      testInstanceScPrefix,
			expectError: true,
		},
		{
			name:   "invalid share object - missing parent",
			prefix: testInstanceScPrefix,
			share: &file.Share{
				Name: testShareName,
			},
			expectError: true,
		},
		{
			name:   "valid share object",
			prefix: testInstanceScPrefix,
			share: &file.Share{
				Name: testShareName,
				Parent: &file.MultishareInstance{
					Name:     testInstanceName,
					Project:  testProject,
					Location: testLocation,
					Network: file.Network{
						Ip: "1.1.1.1",
					},
				},
				CapacityBytes: 1 * util.Tb,
			},
			expectedResp: &csi.CreateVolumeResponse{
				Volume: &csi.Volume{
					VolumeId:      modeMultishare + "/" + testInstanceScPrefix + "/" + testProject + "/" + testLocation + "/" + testInstanceName + "/" + testShareName,
					CapacityBytes: 1 * util.Tb,
					VolumeContext: map[string]string{
						attrIP: "1.1.1.1",
					},
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := generateCSICreateVolumeResponse(tc.prefix, tc.share)
			if tc.expectError && err == nil {
				t.Error("expected error, got none")
			}
			if !tc.expectError && err != nil {
				t.Error("unexpected error")
			}
			if !reflect.DeepEqual(resp, tc.expectedResp) {
				t.Errorf("got %v, want %v", resp, tc.expectedResp)
			}
		})
	}
}
