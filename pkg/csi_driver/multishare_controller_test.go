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
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/google/go-cmp/cmp"
	filev1beta1multishare "google.golang.org/api/file/v1beta1"
	"google.golang.org/protobuf/testing/protocmp"
	"k8s.io/apimachinery/pkg/util/uuid"
	cloud "sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/file"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/util"
)

const (
	testDriverName           = "test.filestore"
	testDrivernameLabelValue = "test_filestore"
	testClusterName          = "test-cluster"
	testPVCName              = "testPVC"
	testPVCNamespace         = "testNamespace"
	testPVName               = "testPV"
	instanceUriFmt           = "projects/%s/locations/%s/instances/%s"
	shareUriFmt              = "projects/%s/locations/%s/instances/%s/shares/%s"
	backupURIFmt             = "projects/%s/locations/%s/backups/%s"
	multishareVolIdFmt       = modeMultishare + "/%s/%s/%s/%s/%s"
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
	config := &controllerServerConfig{
		driver:          initTestDriver(t),
		fileService:     fileService,
		cloud:           cloudProvider,
		volumeLocks:     util.NewVolumeLocks(),
		ecfsDescription: "",
		isRegional:      true,
		clusterName:     testClusterName,
		tagManager:      cloud.NewFakeTagManager(),
	}
	return NewMultishareController(config)
}

func initTestMultishareControllerWithFeatureOpts(t *testing.T, features *GCFSDriverFeatureOptions) *MultishareController {
	fileService, err := file.NewFakeService()
	if err != nil {
		t.Fatalf("failed to initialize GCFS service: %v", err)
	}

	cloudProvider, err := cloud.NewFakeCloud()
	if err != nil {
		t.Fatalf("Failed to get cloud provider: %v", err)
	}
	cloudProvider.File = fileService
	config := &controllerServerConfig{
		driver:          initTestDriver(t),
		fileService:     fileService,
		cloud:           cloudProvider,
		volumeLocks:     util.NewVolumeLocks(),
		ecfsDescription: "",
		isRegional:      true,
		clusterName:     testClusterName,
		features:        features,
		tagManager:      cloud.NewFakeTagManager(),
	}
	return NewMultishareController(config)
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
		minSizeBytes     int64
		maxSizeBytes     int64
		expectErr        bool
		expectedCapacity int64
	}{
		{
			name:         "req and limit not set",
			cap:          &csi.CapacityRange{},
			expectErr:    true,
			minSizeBytes: util.MinShareSizeBytes,
			maxSizeBytes: util.MaxShareSizeBytes,
		},
		{
			name:             "empty cap range, test 1",
			expectedCapacity: 100 * util.Gb,
			minSizeBytes:     util.MinShareSizeBytes,
			maxSizeBytes:     util.MaxShareSizeBytes,
		},
		{
			name:             "empty cap range, test 2",
			expectedCapacity: util.ConfigurablePackMinShareSizeBytes,
			minSizeBytes:     util.ConfigurablePackMinShareSizeBytes,
			maxSizeBytes:     util.MaxShareSizeBytes,
		},
		{
			name:      "cap range limit less than minimum supported size, test 1",
			expectErr: true,
			cap: &csi.CapacityRange{
				LimitBytes: 99 * util.Gb,
			},
			minSizeBytes: util.MinShareSizeBytes,
			maxSizeBytes: util.MaxShareSizeBytes,
		},
		{
			name:      "cap range limit less than minimum supported size, test 2",
			expectErr: true,
			cap: &csi.CapacityRange{
				LimitBytes: 9 * util.Gb,
			},
			minSizeBytes: util.ConfigurablePackMinShareSizeBytes,
			maxSizeBytes: util.MaxShareSizeBytes,
		},
		{
			name:      "cap range limit more than maximum supported size",
			expectErr: true,
			cap: &csi.CapacityRange{
				LimitBytes: 2 * util.Tb,
			},
			minSizeBytes: util.MinShareSizeBytes,
			maxSizeBytes: util.MaxShareSizeBytes,
		},
		{
			name:      "cap range req less than minimum supported size, test 1",
			expectErr: true,
			cap: &csi.CapacityRange{
				RequiredBytes: 99 * util.Gb,
			},
			minSizeBytes: util.MinShareSizeBytes,
			maxSizeBytes: util.MaxShareSizeBytes,
		},
		{
			name:      "cap range req less than minimum supported size, test 2",
			expectErr: true,
			cap: &csi.CapacityRange{
				RequiredBytes: 9 * util.Gb,
			},
			minSizeBytes: util.ConfigurablePackMinShareSizeBytes,
			maxSizeBytes: util.MaxShareSizeBytes,
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
			minSizeBytes:     util.MinShareSizeBytes,
			maxSizeBytes:     util.MaxShareSizeBytes,
		},
		{
			name: "cap range req and limit set, limit exceeds min range",
			cap: &csi.CapacityRange{
				RequiredBytes: 1 * util.Gb,
				LimitBytes:    9 * util.Gb,
			},
			expectErr:    true,
			minSizeBytes: util.ConfigurablePackMinShareSizeBytes,
			maxSizeBytes: util.MaxShareSizeBytes,
		},
		{
			name: "cap range req and limit set, limit exceeds min range",
			cap: &csi.CapacityRange{
				RequiredBytes: 1 * util.Gb,
				LimitBytes:    99 * util.Gb,
			},
			expectErr:    true,
			minSizeBytes: util.MinShareSizeBytes,
			maxSizeBytes: util.MaxShareSizeBytes,
		},
		{
			name: "cap range req and limit set within range",
			cap: &csi.CapacityRange{
				RequiredBytes: 100 * util.Gb,
				LimitBytes:    100 * util.Gb,
			},
			minSizeBytes:     util.ConfigurablePackMinShareSizeBytes,
			maxSizeBytes:     128 * util.Gb,
			expectedCapacity: 100 * util.Gb,
		},
		{
			name: "cap range req and limit set, limit exceed range",
			cap: &csi.CapacityRange{
				RequiredBytes: 100 * util.Gb,
				LimitBytes:    130 * util.Gb,
			},
			minSizeBytes: util.ConfigurablePackMinShareSizeBytes,
			maxSizeBytes: 128 * util.Gb,
			expectErr:    true,
		},
		{
			name: "cap range req set, req exceed range",
			cap: &csi.CapacityRange{
				RequiredBytes: 130 * util.Gb,
			},
			minSizeBytes: util.ConfigurablePackMinShareSizeBytes,
			maxSizeBytes: 128 * util.Gb,
			expectErr:    true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			capacity, err := getShareRequestCapacity(tc.cap, tc.minSizeBytes, tc.maxSizeBytes)
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
	var (
		parameterLabels = "key1=value1,key2=value2"
	)

	tests := []struct {
		name          string
		params        map[string]string
		driver        string
		cliLabels     map[string]string
		expectedLabel map[string]string
		expectErr     bool
	}{
		{
			name:   "empty params",
			driver: testDriverName,
			expectedLabel: map[string]string{
				tagKeyCreatedBy:       testDrivernameLabelValue,
				TagKeyClusterName:     testClusterName,
				TagKeyClusterLocation: testLocation,
			},
		},
		{
			name:   "user labels",
			driver: testDriverName,
			params: map[string]string{
				ParameterKeyLabels:             "a=b,c=d",
				ParamMultishareInstanceScLabel: "testsc",
			},
			expectedLabel: map[string]string{
				tagKeyCreatedBy:                        testDrivernameLabelValue,
				util.ParamMultishareInstanceScLabelKey: "testsc",
				"a":                                    "b",
				"c":                                    "d",
				TagKeyClusterName:                      testClusterName,
				TagKeyClusterLocation:                  testLocation,
			},
		},
		{
			name:   "Parsing labels in storageClass fails(invalid KV separator(:) used)",
			driver: testDriverName,
			params: map[string]string{
				ParamMultishareInstanceScLabel: "testsc",
				ParameterKeyLabels:             "key1:value1,key2:value2",
			},
			cliLabels: map[string]string{
				"key3": "value3",
				"key4": "value4",
			},
			expectedLabel: nil,
			expectErr:     true,
		},
		{
			name:   "storageClass labels contain reserved metadata label(storage_gke_io_created-by)",
			driver: testDriverName,
			params: map[string]string{
				ParamMultishareInstanceScLabel: "testsc",
				ParameterKeyLabels:             "key1=value1,key2=value2,storage_gke_io_created-by=test_filestore",
			},
			cliLabels: map[string]string{
				"key3": "value3",
				"key4": "value4",
			},
			expectedLabel: nil,
			expectErr:     true,
		},
		{
			name:   "storageClass labels parameter not present, only the CLI labels are defined",
			driver: testDriverName,
			params: map[string]string{
				ParamMultishareInstanceScLabel: "testsc",
			},
			cliLabels: map[string]string{
				"key3": "value3",
				"key4": "value4",
			},
			expectedLabel: map[string]string{
				"key3":                                 "value3",
				"key4":                                 "value4",
				tagKeyCreatedBy:                        testDrivernameLabelValue,
				util.ParamMultishareInstanceScLabelKey: "testsc",
				TagKeyClusterName:                      testClusterName,
				TagKeyClusterLocation:                  testLocation,
			},
		},
		{
			name:   "CLI labels not defined, labels are defined only in storageClass object",
			driver: testDriverName,
			params: map[string]string{
				ParamMultishareInstanceScLabel: "testsc",
				ParameterKeyLabels:             parameterLabels,
			},
			cliLabels: nil,
			expectedLabel: map[string]string{
				"key1":                                 "value1",
				"key2":                                 "value2",
				tagKeyCreatedBy:                        testDrivernameLabelValue,
				util.ParamMultishareInstanceScLabelKey: "testsc",
				TagKeyClusterName:                      testClusterName,
				TagKeyClusterLocation:                  testLocation,
			},
		},
		{
			name:   "CLI labels and storageClass labels parameter not defined",
			driver: testDriverName,
			params: map[string]string{
				ParamMultishareInstanceScLabel: "testsc",
			},
			cliLabels: nil,
			expectedLabel: map[string]string{
				tagKeyCreatedBy:                        testDrivernameLabelValue,
				util.ParamMultishareInstanceScLabelKey: "testsc",
				TagKeyClusterName:                      testClusterName,
				TagKeyClusterLocation:                  testLocation,
			},
		},
		{
			name:   "CLI labels and storageClass labels has duplicates",
			driver: testDriverName,
			params: map[string]string{
				ParamMultishareInstanceScLabel: "testsc",
				ParameterKeyLabels:             parameterLabels,
			},
			cliLabels: map[string]string{
				"key1": "value1",
				"key2": "value202",
			},
			expectedLabel: map[string]string{
				"key1":                                 "value1",
				"key2":                                 "value2",
				tagKeyCreatedBy:                        testDrivernameLabelValue,
				util.ParamMultishareInstanceScLabelKey: "testsc",
				TagKeyClusterName:                      testClusterName,
				TagKeyClusterLocation:                  testLocation,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			label, err := extractInstanceLabels(tc.params, tc.cliLabels, tc.driver, testClusterName, testLocation)
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
				ParamMultishareInstanceScLabel: "testsc",
			},
		},
		{
			name: "driver labels",
			params: map[string]string{
				ParameterKeyLabels:             "a=b,c=d",
				ParamMultishareInstanceScLabel: "testsc",
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
					ParamConnectMode: "blah",
				},
			},
			expectErr: true,
		},
		{
			name:         "valid params",
			instanceName: testInstanceName,
			req: &csi.CreateVolumeRequest{
				Parameters: map[string]string{
					ParamConnectMode:               directPeering,
					ParameterKeyLabels:             "a=b,c=d",
					ParamMultishareInstanceScLabel: testInstanceScPrefix,
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
					TagKeyClusterLocation:                  testRegion,
					TagKeyClusterName:                      testClusterName,
					util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
				},
				Protocol: v3FileProtocol,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := initTestMultishareController(t)
			filer, err := m.generateNewMultishareInstance(tc.instanceName, tc.req, 10)
			if tc.expectErr && err == nil {
				t.Error("expected error, got none")
			}
			if !tc.expectErr && err != nil {
				t.Errorf("unexpected error: %q", err)
			}
			if !reflect.DeepEqual(filer, tc.expectedInstance) {
				t.Errorf("got filer %v, want %+v", filer, tc.expectedInstance)
			}
		})
	}
}

func TestGenerateCSICreateVolumeResponse(t *testing.T) {
	tests := []struct {
		name              string
		prefix            string
		share             *file.Share
		expectError       bool
		features          *GCFSDriverFeatureOptions
		expectedResp      *csi.CreateVolumeResponse
		maxShareSizeBytes int64
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
					Protocol: v3FileProtocol,
				},
				CapacityBytes: 1 * util.Tb,
			},
			maxShareSizeBytes: 1 * util.Tb,
			expectedResp: &csi.CreateVolumeResponse{
				Volume: &csi.Volume{
					VolumeId:      modeMultishare + "/" + testInstanceScPrefix + "/" + testProject + "/" + testLocation + "/" + testInstanceName + "/" + testShareName,
					CapacityBytes: 1 * util.Tb,
					VolumeContext: map[string]string{
						attrIP:           "1.1.1.1",
						attrFileProtocol: v3FileProtocol,
					},
				},
			},
		},
		{
			name:   "valid share object, with configurable share feature enabled",
			prefix: testInstanceScPrefix,
			features: &GCFSDriverFeatureOptions{
				FeatureMaxSharesPerInstance: &FeatureMaxSharesPerInstance{
					Enabled: true,
				},
			},
			share: &file.Share{
				Name: testShareName,
				Parent: &file.MultishareInstance{
					Name:     testInstanceName,
					Project:  testProject,
					Location: testLocation,
					Network: file.Network{
						Ip: "1.1.1.1",
					},
					Protocol: v4_1FileProtocol,
				},
				CapacityBytes: 1 * util.Tb,
			},
			maxShareSizeBytes: 1 * util.Tb,
			expectedResp: &csi.CreateVolumeResponse{
				Volume: &csi.Volume{
					VolumeId:      modeMultishare + "/" + testInstanceScPrefix + "/" + testProject + "/" + testLocation + "/" + testInstanceName + "/" + testShareName,
					CapacityBytes: 1 * util.Tb,
					VolumeContext: map[string]string{
						attrIP:           "1.1.1.1",
						attrMaxShareSize: strconv.Itoa(util.Tb),
						attrFileProtocol: v4_1FileProtocol,
					},
				},
			},
		},
		{
			name:   "valid share object, with configurable share feature enabled, test 2",
			prefix: testInstanceScPrefix,
			features: &GCFSDriverFeatureOptions{
				FeatureMaxSharesPerInstance: &FeatureMaxSharesPerInstance{
					Enabled: true,
				},
			},
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
			maxShareSizeBytes: 100 * util.Gb,
			expectedResp: &csi.CreateVolumeResponse{
				Volume: &csi.Volume{
					VolumeId:      modeMultishare + "/" + testInstanceScPrefix + "/" + testProject + "/" + testLocation + "/" + testInstanceName + "/" + testShareName,
					CapacityBytes: 1 * util.Tb,
					VolumeContext: map[string]string{
						attrIP:           "1.1.1.1",
						attrMaxShareSize: strconv.Itoa(100 * util.Gb),
						attrFileProtocol: v3FileProtocol,
					},
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			//m := initTestMultishareController(t)
			m := initTestMultishareControllerWithFeatureOpts(t, tc.features)
			resp, err := m.generateCSICreateVolumeResponse(tc.prefix, tc.share, tc.maxShareSizeBytes)
			if tc.expectError && err == nil {
				t.Error("expected error, got none")
			}
			if !tc.expectError && err != nil {
				t.Error("unexpected error")
			}
			if !cmp.Equal(resp, tc.expectedResp, protocmp.Transform()) {
				t.Errorf("test %q failed: got resp %+v, expected %+v, diff: %s", tc.name, resp, tc.expectedResp, cmp.Diff(resp, tc.expectedResp, protocmp.Transform()))
			}
		})
	}
}

func TestGenerateInstanceDescFromEcfsDesc(t *testing.T) {
	tests := []struct {
		name       string
		inputdesc  string
		outputdesc string
	}{
		{
			name: "empty",
		},
		{
			name:      "invalid key value pair, unknown key",
			inputdesc: "k1=v1",
		},
		{
			name:      "invalid key value pair, unknown key1",
			inputdesc: "k1=v1,ecfs-version=test",
		},
		{
			name:      "invalid key value pair, unknown key2",
			inputdesc: "k1=v1,image-project-id=test",
		},
		{
			name:      "invalid key value pair",
			inputdesc: "k1=v1,image-project-id",
		},
		{
			name:       "case1: valid key value pair",
			inputdesc:  "ecfs-version=ems-filestore-scaleout-3-6-0-1-70bd79ed0a91,image-project-id=elastifile-ci",
			outputdesc: fmt.Sprintf(ecfsDataPlaneVersionFormat, "elastifile-ci", "ems-filestore-scaleout-3-6-0-1-70bd79ed0a91"),
		},
		{
			name:       "case2: valid key value pair",
			inputdesc:  "image-project-id=elastifile-ci,ecfs-version=ems-filestore-scaleout-3-6-0-1-70bd79ed0a91",
			outputdesc: fmt.Sprintf(ecfsDataPlaneVersionFormat, "elastifile-ci", "ems-filestore-scaleout-3-6-0-1-70bd79ed0a91"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			op := generateInstanceDescFromEcfsDesc(tc.inputdesc)
			if op != tc.outputdesc {
				t.Errorf("got %s, want %s", op, tc.outputdesc)
			}
		})
	}
}

func TestMultishareCreateVolume(t *testing.T) {
	testVolName := "pvc-" + string(uuid.NewUUID())
	testShareName := util.ConvertVolToShareName(testVolName)
	testInstanceName1 := "fs-" + string(uuid.NewUUID())
	testInstanceName2 := "fs-" + string(uuid.NewUUID())
	features := &GCFSDriverFeatureOptions{
		FeatureMultishareBackups: &FeatureMultishareBackups{
			Enabled: true,
		},
		FeatureNFSExportOptionsOnCreate: &FeatureNFSExportOptionsOnCreate{
			Enabled: true,
		},
	}
	type OpItem struct {
		id     string
		target string
		verb   string
		done   bool
	}
	tests := []struct {
		name              string
		prefix            string
		ops               []OpItem
		initInstances     []*file.MultishareInstance
		initShares        []*file.Share
		req               *csi.CreateVolumeRequest
		resp              *csi.CreateVolumeResponse
		expectedOptions   []*file.NfsExportOptions
		errorExpected     bool
		checkOnlyVolidFmt bool // for auto generated instance, the instance name is not known
		features          *GCFSDriverFeatureOptions
	}{
		{
			name: "create volume called with volume size < 100G in required bytes",
			req: &csi.CreateVolumeRequest{
				Name: testVolName,
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 99 * util.Gb,
				},
				Parameters: map[string]string{
					ParamMultishareInstanceScLabel: testInstanceScPrefix,
				},
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
				},
			},
			errorExpected: true,
		},
		{
			name: "create volume called with volume size < 100G in limit bytes",
			req: &csi.CreateVolumeRequest{
				Name: testVolName,
				CapacityRange: &csi.CapacityRange{
					LimitBytes: 99 * util.Gb,
				},
				Parameters: map[string]string{
					ParamMultishareInstanceScLabel: testInstanceScPrefix,
				},
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
				},
			},
			errorExpected: true,
		},
		{
			name: "create volume called with volume size > 1T in required bytes",
			req: &csi.CreateVolumeRequest{
				Name: testVolName,
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 2 * util.Tb,
				},
				Parameters: map[string]string{
					ParamMultishareInstanceScLabel: testInstanceScPrefix,
				},
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
				},
			},
			errorExpected: true,
		},
		{
			name: "create volume called with volume size > 1T in limit bytes",
			req: &csi.CreateVolumeRequest{
				Name: testVolName,
				CapacityRange: &csi.CapacityRange{
					LimitBytes: 2 * util.Tb,
				},
				Parameters: map[string]string{
					ParamMultishareInstanceScLabel: testInstanceScPrefix,
				},
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
				},
			},
			errorExpected: true,
		},
		{
			name: "no initial instances, create instance and share, success response",
			req: &csi.CreateVolumeRequest{
				Name: testVolName,
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 100 * util.Gb,
				},
				Parameters: map[string]string{
					ParamMultishareInstanceScLabel: testInstanceScPrefix,
				},
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
				},
			},
			checkOnlyVolidFmt: true,
		},
		{
			name: "no initial instances, create instance and share, success response",
			req: &csi.CreateVolumeRequest{
				Name: testVolName,
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 100 * util.Gb,
				},
				Parameters: map[string]string{
					ParamMultishareInstanceScLabel: testInstanceScPrefix,
				},
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
				},
			},
			checkOnlyVolidFmt: true,
		},
		{
			name: "nfs-export-options feature is disabled",
			req: &csi.CreateVolumeRequest{
				Name: testVolName,
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 100 * util.Gb,
				},
				Parameters: map[string]string{
					ParamMultishareInstanceScLabel: testInstanceScPrefix,
					ParamNfsExportOptions: `[
						{
							"accessMode": "READ_WRITE",
							"ipRanges": [
								"10.0.0.0/24"
						],
							"squashMode": "ROOT_SQUASH",
							"anonUid": "1003",
							"anonGid": "1003"
						},
						{
							"accessMode": "READ_ONLY",
							"ipRanges": [
								"10.0.0.0/28"
							],
							"squashMode": "NO_ROOT_SQUASH"
							}
					]`,
				},
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
				},
			},
			errorExpected: true,
		},
		{
			name: "add nfs-export-options",
			req: &csi.CreateVolumeRequest{
				Name: testVolName,
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 100 * util.Gb,
				},
				Parameters: map[string]string{
					ParamMultishareInstanceScLabel: testInstanceScPrefix,
					ParamNfsExportOptions: `[
						{
							"accessMode": "READ_WRITE",
							"ipRanges": [
								"10.0.0.0/24"
						],
							"squashMode": "ROOT_SQUASH",
							"anonUid": "1003",
							"anonGid": "1003"
						},
						{
							"accessMode": "READ_ONLY",
							"ipRanges": [
								"10.0.0.0/28"
							],
							"squashMode": "NO_ROOT_SQUASH"
							}
					]`,
				},
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
				},
			},
			features:          features,
			checkOnlyVolidFmt: true,
			expectedOptions: []*file.NfsExportOptions{
				{
					AccessMode: "READ_WRITE",
					IpRanges:   []string{"10.0.0.0/24"},
					SquashMode: "ROOT_SQUASH",
					AnonGid:    1003,
					AnonUid:    1003,
				},
				{
					AccessMode: "READ_ONLY",
					IpRanges:   []string{"10.0.0.0/28"},
					SquashMode: "NO_ROOT_SQUASH",
				},
			},
		},
		{
			name: "1 initial ready 1Tib instances with 0 shares having NFSv3 Protocol, 1 initial ready 1Tib instances with 0 shares having NFSv4 Protocol, create 100Gib share in a NFSv4 instance, success response",
			initInstances: []*file.MultishareInstance{
				{
					Name:     testInstanceName1,
					Location: "us-central1",
					Project:  "test-project",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
						TagKeyClusterLocation:                  testLocation,
						TagKeyClusterName:                      "",
					},
					CapacityBytes: 1 * util.Tb,
					Tier:          "enterprise",
					Network: file.Network{
						Ip:          testIP,
						Name:        defaultNetwork,
						ConnectMode: directPeering,
					},
					State:    "READY",
					Protocol: v3FileProtocol,
				},
				{
					Name:     testInstanceName2,
					Location: "us-central1",
					Project:  "test-project",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
						TagKeyClusterLocation:                  testLocation,
						TagKeyClusterName:                      "",
					},
					CapacityBytes: 1 * util.Tb,
					Tier:          "enterprise",
					Network: file.Network{
						Ip:          testIP,
						Name:        defaultNetwork,
						ConnectMode: directPeering,
					},
					State:    "READY",
					Protocol: v4_1FileProtocol,
				},
			},
			ops: []OpItem{},
			req: &csi.CreateVolumeRequest{
				Name: testVolName,
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 100 * util.Gb,
				},
				Parameters: map[string]string{
					ParamMultishareInstanceScLabel: testInstanceScPrefix,
					paramTier:                      "enterprise",
					paramFileProtocol:              v4_1FileProtocol,
				},
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
				},
			},
			resp: &csi.CreateVolumeResponse{
				Volume: &csi.Volume{
					CapacityBytes: 100 * util.Gb,
					VolumeId:      fmt.Sprintf(multishareVolIdFmt, testInstanceScPrefix, testProject, testRegion, testInstanceName2, testShareName),
					VolumeContext: map[string]string{
						attrIP:           testIP,
						attrFileProtocol: v4_1FileProtocol,
					},
				},
			},
		},
		{
			name: "1 initial ready 1Tib instances with 0 shares, 1 busy instance, create 100Gib share and use the ready instance, success response",
			initInstances: []*file.MultishareInstance{
				{
					Name:     testInstanceName1,
					Location: "us-central1",
					Project:  "test-project",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
						TagKeyClusterLocation:                  testLocation,
						TagKeyClusterName:                      "",
					},
					CapacityBytes: 1 * util.Tb,
					Tier:          "enterprise",
					Network: file.Network{
						Ip:          testIP,
						Name:        defaultNetwork,
						ConnectMode: directPeering,
					},
					State:    "READY",
					Protocol: v4_1FileProtocol,
				},
				{
					Name:     testInstanceName2,
					Location: "us-central1",
					Project:  "test-project",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
						TagKeyClusterLocation:                  testLocation,
						TagKeyClusterName:                      "",
					},
					CapacityBytes: 1 * util.Tb,
					Tier:          "enterprise",
					Network: file.Network{
						Ip:          testIP,
						Name:        defaultNetwork,
						ConnectMode: directPeering,
					},
					State:    "CREATING",
					Protocol: v4_1FileProtocol,
				},
			},
			ops: []OpItem{
				{
					id:     "op1",
					target: fmt.Sprintf(instanceUriFmt, testProject, testRegion, testInstanceName2),
					verb:   "create",
				},
			},
			req: &csi.CreateVolumeRequest{
				Name: testVolName,
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 100 * util.Gb,
				},
				Parameters: map[string]string{
					ParamMultishareInstanceScLabel: testInstanceScPrefix,
					paramTier:                      "enterprise",
					paramFileProtocol:              v4_1FileProtocol,
				},
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
				},
			},
			resp: &csi.CreateVolumeResponse{
				Volume: &csi.Volume{
					CapacityBytes: 100 * util.Gb,
					VolumeId:      fmt.Sprintf(multishareVolIdFmt, testInstanceScPrefix, testProject, testRegion, testInstanceName1, testShareName),
					VolumeContext: map[string]string{
						attrIP:           testIP,
						attrFileProtocol: v4_1FileProtocol,
					},
				},
			},
		},
		{
			name: "share op in progress found, return retry error to client",
			initInstances: []*file.MultishareInstance{
				{
					Name:     testInstanceName1,
					Location: "us-central1",
					Project:  "test-project",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
						TagKeyClusterLocation:                  testLocation,
						TagKeyClusterName:                      "",
					},
					CapacityBytes: 1 * util.Tb,
					Tier:          "Enterprise",
					Network: file.Network{
						Ip: testIP,
					},
				},
			},
			ops: []OpItem{
				{
					id:     "op1",
					target: fmt.Sprintf(shareUriFmt, testProject, testRegion, testInstanceName1, testShareName),
					verb:   "create",
				},
			},
			req: &csi.CreateVolumeRequest{
				Name: testVolName,
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 100 * util.Gb,
				},
				Parameters: map[string]string{
					ParamMultishareInstanceScLabel: testInstanceScPrefix,
				},
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
				},
			},
			errorExpected: true,
		},
		{
			name: "share already exists, return success",
			initInstances: []*file.MultishareInstance{
				{
					Name:     testInstanceName1,
					Location: "us-central1",
					Project:  "test-project",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
						TagKeyClusterLocation:                  testLocation,
						TagKeyClusterName:                      "",
					},
					CapacityBytes: 1 * util.Tb,
					Tier:          "Enterprise",
					Network: file.Network{
						Ip: testIP,
					},
					State:    "READY",
					Protocol: v4_1FileProtocol,
				},
			},
			initShares: []*file.Share{
				{
					Name: testShareName,
					Parent: &file.MultishareInstance{
						Name:     testInstanceName1,
						Location: "us-central1",
						Project:  "test-project",
						Labels: map[string]string{
							util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
						},
						CapacityBytes: 1 * util.Tb,
						Tier:          "Enterprise",
						Network: file.Network{
							Ip: testIP,
						},
						State:    "READY",
						Protocol: v4_1FileProtocol,
					},
					CapacityBytes:  100 * util.Gb,
					MountPointName: testShareName,
					State:          "READY",
				},
			},
			req: &csi.CreateVolumeRequest{
				Name: testVolName,
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 100 * util.Gb,
				},
				Parameters: map[string]string{
					ParamMultishareInstanceScLabel: testInstanceScPrefix,
					paramFileProtocol:              v4_1FileProtocol,
				},
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
				},
			},
			resp: &csi.CreateVolumeResponse{
				Volume: &csi.Volume{
					CapacityBytes: 100 * util.Gb,
					VolumeId:      fmt.Sprintf(multishareVolIdFmt, testInstanceScPrefix, testProject, testRegion, testInstanceName1, testShareName),
					VolumeContext: map[string]string{
						attrIP:           testIP,
						attrFileProtocol: v4_1FileProtocol,
					},
				},
			},
		},
		// TODO: Add test cases for instance resize
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var v1beta1ops []*filev1beta1multishare.Operation
			for _, item := range tc.ops {
				var meta filev1beta1multishare.OperationMetadata
				meta.Target = item.target
				meta.Verb = item.verb
				bytes, _ := json.Marshal(meta)
				v1beta1ops = append(v1beta1ops, &filev1beta1multishare.Operation{
					Name:     item.id,
					Done:     item.done,
					Metadata: bytes,
				})
			}

			s, err := file.NewFakeServiceForMultishare(tc.initInstances, tc.initShares, v1beta1ops)
			if err != nil {
				t.Fatalf("failed to fake service: %v", err)
			}
			cloudProvider, _ := cloud.NewFakeCloud()
			cloudProvider.File = s
			config := &controllerServerConfig{
				driver:          initTestDriver(t),
				fileService:     s,
				cloud:           cloudProvider,
				volumeLocks:     util.NewVolumeLocks(),
				ecfsDescription: "",
				features:        tc.features,
			}
			mcs := NewMultishareController(config)
			resp, err := mcs.CreateVolume(context.Background(), tc.req)
			if tc.errorExpected && err == nil {
				t.Errorf("expected error not found")
			}
			if !tc.errorExpected && err != nil {
				t.Errorf("unexpected error %s", err)
			}
			if !tc.errorExpected && tc.req.Parameters[ParamNfsExportOptions] != "" {
				instance, err := s.GetShare(context.TODO(), &file.Share{Name: util.ConvertVolToShareName(tc.req.Name)})
				if err != nil {
					t.Errorf("test %q failed: couldn't get instance %v: %v", tc.name, tc.req.Name, err)
					return
				}
				for i := range tc.expectedOptions {
					if !reflect.DeepEqual(instance.NfsExportOptions[i], tc.expectedOptions[i]) {
						t.Errorf("tc %q failed; nfs export options not equal at index %d: got %+v, expected %+v", tc.name, i, instance.NfsExportOptions[i], tc.expectedOptions[i])
					}
				}
			}
			if tc.checkOnlyVolidFmt {
				if !strings.Contains(resp.Volume.VolumeId, modeMultishare) || !strings.Contains(resp.Volume.VolumeId, testShareName) {
					t.Errorf("unexpected vol id %s", resp.Volume.VolumeId)
				}
			} else {
				if tc.resp != nil && resp == nil {
					t.Errorf("mismatch in response")
				}
				if tc.resp == nil && resp != nil {
					t.Errorf("mismatch in response")
				}
				if !cmp.Equal(resp, tc.resp, protocmp.Transform()) {
					t.Errorf("test %q failed: got resp %+v, expected %+v, diff: %s", tc.name, resp, tc.resp, cmp.Diff(resp, tc.resp, protocmp.Transform()))
				}
			}
		})
	}
}

func TestMultishareCreateVolumeFromBackup(t *testing.T) {
	type BackupTestInfo struct {
		backup *file.BackupInfo
		state  string
	}
	testVolName := "pvc-" + string(uuid.NewUUID())
	testShareName := util.ConvertVolToShareName(testVolName)
	testInstanceName1 := "fs-" + string(uuid.NewUUID())
	testInstanceName2 := "fs-" + string(uuid.NewUUID())
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
	features := &GCFSDriverFeatureOptions{
		FeatureMultishareBackups: &FeatureMultishareBackups{
			Enabled: true,
		},
		FeatureNFSExportOptionsOnCreate: &FeatureNFSExportOptionsOnCreate{
			Enabled: true,
		},
	}

	defaultBackup := &BackupTestInfo{
		backup: &file.BackupInfo{
			Project:            testProject,
			Location:           testRegion,
			SourceInstanceName: testInstanceName1,
			SourceShare:        testShareName,
			Name:               "mybackup",
			BackupURI:          "projects/test-project/locations/us-central1/backups/mybackup",
			SourceVolumeId:     modeMultishare + "/" + testRegion + "/" + testInstanceName1 + "/" + testShareName,
		},
	}
	type OpItem struct {
		id     string
		target string
		verb   string
		done   bool
	}
	tests := []struct {
		name              string
		prefix            string
		ops               []OpItem
		initInstances     []*file.MultishareInstance
		initShares        []*file.Share
		req               *csi.CreateVolumeRequest
		resp              *csi.CreateVolumeResponse
		checkOnlyVolidFmt bool
		expectedOptions   []*file.NfsExportOptions
		initialBackup     *BackupTestInfo
		features          *GCFSDriverFeatureOptions
		errorExpected     bool
	}{
		{
			name: "create volume called with volume content source, but multishare backup feature is disabled",
			req: &csi.CreateVolumeRequest{
				Name: testVolName,
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 100 * util.Gb,
				},
				Parameters: map[string]string{
					ParamMultishareInstanceScLabel: testInstanceScPrefix,
				},
				VolumeCapabilities: volumeCapabilities,
				VolumeContentSource: &csi.VolumeContentSource{
					Type: &csi.VolumeContentSource_Snapshot{
						Snapshot: &csi.VolumeContentSource_SnapshotSource{
							SnapshotId: "projects/test-project/locations/us-central1/backups/mybackup",
						},
					},
				},
			},
			features: &GCFSDriverFeatureOptions{
				FeatureMultishareBackups: &FeatureMultishareBackups{
					Enabled: false,
				},
			},
			initialBackup:     defaultBackup,
			checkOnlyVolidFmt: true,
			errorExpected:     true,
		},
		{
			name: "create volume called with volume content source and nfsExportOptions set",
			req: &csi.CreateVolumeRequest{
				Name: testVolName,
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 100 * util.Gb,
				},
				Parameters: map[string]string{
					ParamMultishareInstanceScLabel: testInstanceScPrefix,
					ParamNfsExportOptions: `[
						{
							"accessMode": "READ_WRITE",
							"ipRanges": [
								"10.0.0.0/24",
								"10.124.124.0/28"
						],
							"squashMode": "ROOT_SQUASH",
							"anonUid": "1003",
							"anonGid": "1003"
						},
						{
							"accessMode": "READ_ONLY",
							"ipRanges": [
								"10.0.0.0/28"
							],
							"squashMode": "NO_ROOT_SQUASH"
							}
					]`,
					paramFileProtocol: v4_1FileProtocol,
				},
				VolumeCapabilities: volumeCapabilities,
				VolumeContentSource: &csi.VolumeContentSource{
					Type: &csi.VolumeContentSource_Snapshot{
						Snapshot: &csi.VolumeContentSource_SnapshotSource{
							SnapshotId: "projects/test-project/locations/us-central1/backups/mybackup",
						},
					},
				},
			},
			features: features,
			expectedOptions: []*file.NfsExportOptions{
				{
					AccessMode: "READ_WRITE",
					IpRanges:   []string{"10.0.0.0/24", "10.124.124.0/28"},
					SquashMode: "ROOT_SQUASH",
					AnonGid:    1003,
					AnonUid:    1003,
				},
				{
					AccessMode: "READ_ONLY",
					IpRanges:   []string{"10.0.0.0/28"},
					SquashMode: "NO_ROOT_SQUASH",
				},
			},
			resp: &csi.CreateVolumeResponse{
				Volume: &csi.Volume{
					CapacityBytes: 100 * util.Gb,
					VolumeId:      fmt.Sprintf(multishareVolIdFmt, testInstanceScPrefix, testProject, testRegion, testInstanceName1, testShareName),
					VolumeContext: map[string]string{
						attrIP:           testIP,
						attrFileProtocol: v4_1FileProtocol,
					},
					ContentSource: &csi.VolumeContentSource{
						Type: &csi.VolumeContentSource_Snapshot{
							Snapshot: &csi.VolumeContentSource_SnapshotSource{
								SnapshotId: "projects/test-project/locations/us-central1/backups/mybackup",
							},
						},
					},
				},
			},
			initialBackup:     defaultBackup,
			checkOnlyVolidFmt: true,
		},
		{
			name: "create volume called with volume content source, no existing instance or share",
			req: &csi.CreateVolumeRequest{
				Name: testVolName,
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 100 * util.Gb,
				},
				Parameters: map[string]string{
					ParamMultishareInstanceScLabel: testInstanceScPrefix,
				},
				VolumeCapabilities: volumeCapabilities,
				VolumeContentSource: &csi.VolumeContentSource{
					Type: &csi.VolumeContentSource_Snapshot{
						Snapshot: &csi.VolumeContentSource_SnapshotSource{
							SnapshotId: "projects/test-project/locations/us-central1/backups/mybackup",
						},
					},
				},
			},
			features: features,
			resp: &csi.CreateVolumeResponse{
				Volume: &csi.Volume{
					CapacityBytes: 100 * util.Gb,
					VolumeId:      fmt.Sprintf(multishareVolIdFmt, testInstanceScPrefix, testProject, testRegion, testInstanceName1, testShareName),
					VolumeContext: map[string]string{
						attrIP:           testIP,
						attrFileProtocol: v3FileProtocol,
					},
					ContentSource: &csi.VolumeContentSource{
						Type: &csi.VolumeContentSource_Snapshot{
							Snapshot: &csi.VolumeContentSource_SnapshotSource{
								SnapshotId: "projects/test-project/locations/us-central1/backups/mybackup",
							},
						},
					},
				},
			},
			initialBackup:     defaultBackup,
			checkOnlyVolidFmt: true,
		},
		{
			name: "1 initial ready 1Tib instance with 0 shares, 1 busy instance,  create 100Gib share with content source in free instance, success response",
			initInstances: []*file.MultishareInstance{
				{
					Name:     testInstanceName1,
					Location: "us-central1",
					Project:  "test-project",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
						TagKeyClusterLocation:                  testLocation,
						TagKeyClusterName:                      "",
					},
					CapacityBytes: 1 * util.Tb,
					Tier:          "enterprise",
					Network: file.Network{
						Ip:          testIP,
						Name:        defaultNetwork,
						ConnectMode: directPeering,
					},
					State:    "READY",
					Protocol: v4_1FileProtocol,
				},
				{
					Name:     testInstanceName2,
					Location: "us-central1",
					Project:  "test-project",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
						TagKeyClusterLocation:                  testLocation,
						TagKeyClusterName:                      "",
					},
					CapacityBytes: 1 * util.Tb,
					Tier:          "enterprise",
					Network: file.Network{
						Ip:          testIP,
						Name:        defaultNetwork,
						ConnectMode: directPeering,
					},
					State:    "CREATING",
					Protocol: v4_1FileProtocol,
				},
			},
			ops: []OpItem{
				{
					id:     "op1",
					target: fmt.Sprintf(instanceUriFmt, testProject, testRegion, testInstanceName2),
					verb:   "create",
				},
			},
			req: &csi.CreateVolumeRequest{
				Name: testVolName,
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 100 * util.Gb,
				},
				Parameters: map[string]string{
					ParamMultishareInstanceScLabel: testInstanceScPrefix,
					paramTier:                      "enterprise",
					paramFileProtocol:              v4_1FileProtocol,
				},
				VolumeCapabilities: volumeCapabilities,
				VolumeContentSource: &csi.VolumeContentSource{
					Type: &csi.VolumeContentSource_Snapshot{
						Snapshot: &csi.VolumeContentSource_SnapshotSource{
							SnapshotId: "projects/test-project/locations/us-central1/backups/mybackup",
						},
					},
				},
			},
			features: features,
			resp: &csi.CreateVolumeResponse{
				Volume: &csi.Volume{
					CapacityBytes: 100 * util.Gb,
					VolumeId:      fmt.Sprintf(multishareVolIdFmt, testInstanceScPrefix, testProject, testRegion, testInstanceName1, testShareName),
					VolumeContext: map[string]string{
						attrIP:           testIP,
						attrFileProtocol: v4_1FileProtocol,
					},
					ContentSource: &csi.VolumeContentSource{
						Type: &csi.VolumeContentSource_Snapshot{
							Snapshot: &csi.VolumeContentSource_SnapshotSource{
								SnapshotId: "projects/test-project/locations/us-central1/backups/mybackup",
							},
						},
					},
				},
			},
			initialBackup: defaultBackup,
		},
		{
			name: "1 initial ready 1Tib instance with 0 shares, create 100Gib share with content source in same instance, success response",
			initInstances: []*file.MultishareInstance{
				{
					Name:     testInstanceName1,
					Location: "us-central1",
					Project:  "test-project",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
						TagKeyClusterLocation:                  testLocation,
						TagKeyClusterName:                      "",
					},
					CapacityBytes: 1 * util.Tb,
					Tier:          "enterprise",
					Network: file.Network{
						Ip:          testIP,
						Name:        defaultNetwork,
						ConnectMode: directPeering,
					},
					State:    "READY",
					Protocol: v4_1FileProtocol,
				},
			},
			req: &csi.CreateVolumeRequest{
				Name: testVolName,
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 100 * util.Gb,
				},
				Parameters: map[string]string{
					ParamMultishareInstanceScLabel: testInstanceScPrefix,
					paramTier:                      "enterprise",
					paramFileProtocol:              v4_1FileProtocol,
				},
				VolumeCapabilities: volumeCapabilities,
				VolumeContentSource: &csi.VolumeContentSource{
					Type: &csi.VolumeContentSource_Snapshot{
						Snapshot: &csi.VolumeContentSource_SnapshotSource{
							SnapshotId: "projects/test-project/locations/us-central1/backups/mybackup",
						},
					},
				},
			},
			features: features,
			resp: &csi.CreateVolumeResponse{
				Volume: &csi.Volume{
					CapacityBytes: 100 * util.Gb,
					VolumeId:      fmt.Sprintf(multishareVolIdFmt, testInstanceScPrefix, testProject, testRegion, testInstanceName1, testShareName),
					VolumeContext: map[string]string{
						attrIP:           testIP,
						attrFileProtocol: v4_1FileProtocol,
					},
					ContentSource: &csi.VolumeContentSource{
						Type: &csi.VolumeContentSource_Snapshot{
							Snapshot: &csi.VolumeContentSource_SnapshotSource{
								SnapshotId: "projects/test-project/locations/us-central1/backups/mybackup",
							},
						},
					},
				},
			},
			initialBackup: defaultBackup,
		},
		{
			name: "share already exists, return success",
			initInstances: []*file.MultishareInstance{
				{
					Name:     testInstanceName1,
					Location: "us-central1",
					Project:  "test-project",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
						TagKeyClusterLocation:                  testLocation,
						TagKeyClusterName:                      "",
					},
					CapacityBytes: 1 * util.Tb,
					Tier:          "Enterprise",
					Network: file.Network{
						Ip: testIP,
					},
					State:    "READY",
					Protocol: v4_1FileProtocol,
				},
			},
			initShares: []*file.Share{
				{
					Name: testShareName,
					Parent: &file.MultishareInstance{
						Name:     testInstanceName1,
						Location: "us-central1",
						Project:  "test-project",
						Labels: map[string]string{
							util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
						},
						CapacityBytes: 1 * util.Tb,
						Tier:          "Enterprise",
						Network: file.Network{
							Ip: testIP,
						},
						State:    "READY",
						Protocol: v4_1FileProtocol,
					},
					CapacityBytes:  100 * util.Gb,
					MountPointName: testShareName,
					State:          "READY",
					BackupId:       "projects/test-project/locations/us-central1/backups/mybackup",
				},
			},
			req: &csi.CreateVolumeRequest{
				Name: testVolName,
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 100 * util.Gb,
				},
				Parameters: map[string]string{
					ParamMultishareInstanceScLabel: testInstanceScPrefix,
					paramFileProtocol:              v4_1FileProtocol,
				},
				VolumeCapabilities: volumeCapabilities,
				VolumeContentSource: &csi.VolumeContentSource{
					Type: &csi.VolumeContentSource_Snapshot{
						Snapshot: &csi.VolumeContentSource_SnapshotSource{
							SnapshotId: "projects/test-project/locations/us-central1/backups/mybackup",
						},
					},
				},
			},
			features: features,
			resp: &csi.CreateVolumeResponse{
				Volume: &csi.Volume{
					CapacityBytes: 100 * util.Gb,
					VolumeId:      fmt.Sprintf(multishareVolIdFmt, testInstanceScPrefix, testProject, testRegion, testInstanceName1, testShareName),
					VolumeContext: map[string]string{
						attrIP:           testIP,
						attrFileProtocol: v4_1FileProtocol,
					},
					ContentSource: &csi.VolumeContentSource{
						Type: &csi.VolumeContentSource_Snapshot{
							Snapshot: &csi.VolumeContentSource_SnapshotSource{
								SnapshotId: "projects/test-project/locations/us-central1/backups/mybackup",
							},
						},
					},
				},
			},
			initialBackup: defaultBackup,
		},
		{
			name: "share op in progress found, return retry error to client",
			initInstances: []*file.MultishareInstance{
				{
					Name:     testInstanceName1,
					Location: "us-central1",
					Project:  "test-project",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
						TagKeyClusterLocation:                  testLocation,
						TagKeyClusterName:                      "",
					},
					CapacityBytes: 1 * util.Tb,
					Tier:          "Enterprise",
					Network: file.Network{
						Ip: testIP,
					},
				},
			},
			features: features,
			ops: []OpItem{
				{
					id:     "op1",
					target: fmt.Sprintf(shareUriFmt, testProject, testRegion, testInstanceName1, testShareName),
					verb:   "create",
				},
			},
			req: &csi.CreateVolumeRequest{
				Name: testVolName,
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 100 * util.Gb,
				},
				Parameters: map[string]string{
					ParamMultishareInstanceScLabel: testInstanceScPrefix,
				},
				VolumeCapabilities: volumeCapabilities,
				VolumeContentSource: &csi.VolumeContentSource{
					Type: &csi.VolumeContentSource_Snapshot{
						Snapshot: &csi.VolumeContentSource_SnapshotSource{
							SnapshotId: "projects/test-project/locations/us-central1/backups/mybackup",
						},
					},
				},
			},
			initialBackup: defaultBackup,
			errorExpected: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var v1beta1ops []*filev1beta1multishare.Operation
			for _, item := range tc.ops {
				var meta filev1beta1multishare.OperationMetadata
				meta.Target = item.target
				meta.Verb = item.verb
				bytes, _ := json.Marshal(meta)
				v1beta1ops = append(v1beta1ops, &filev1beta1multishare.Operation{
					Name:     item.id,
					Done:     item.done,
					Metadata: bytes,
				})
			}

			s, err := file.NewFakeServiceForMultishare(tc.initInstances, tc.initShares, v1beta1ops)
			if err != nil {
				t.Fatalf("failed to fake service: %v", err)
			}
			cloudProvider, _ := cloud.NewFakeCloud()
			cloudProvider.File = s
			config := &controllerServerConfig{
				driver:          initTestDriver(t),
				fileService:     s,
				cloud:           cloudProvider,
				volumeLocks:     util.NewVolumeLocks(),
				ecfsDescription: "",
				features:        tc.features,
			}
			mcs := NewMultishareController(config)

			if tc.initialBackup != nil {
				existingBackup, _ := s.CreateBackup(context.TODO(), tc.initialBackup.backup)
				if tc.initialBackup.state != "" {
					existingBackup.State = tc.initialBackup.state
				}
			}
			resp, err := mcs.CreateVolume(context.Background(), tc.req)
			if tc.errorExpected && err == nil {
				t.Errorf("expected error not found")
			}
			if !tc.errorExpected && err != nil {
				t.Errorf("unexpected error")
			}
			if !tc.errorExpected && tc.req.Parameters[ParamNfsExportOptions] != "" {
				instance, err := s.GetShare(context.TODO(), &file.Share{Name: util.ConvertVolToShareName(tc.req.Name)})
				if err != nil {
					t.Errorf("test %q failed: couldn't get instance %v: %v", tc.name, tc.req.Name, err)
					return
				}
				for i := range tc.expectedOptions {
					if !reflect.DeepEqual(instance.NfsExportOptions[i], tc.expectedOptions[i]) {
						t.Errorf("tc %q failed; nfs export options not equal at index %d: got %+v, expected %+v", tc.name, i, instance.NfsExportOptions[i], tc.expectedOptions[i])
					}
				}
			}
			if tc.checkOnlyVolidFmt && !tc.errorExpected {
				if !strings.Contains(resp.Volume.VolumeId, modeMultishare) || !strings.Contains(resp.Volume.VolumeId, testShareName) {
					t.Errorf("unexpected vol id %s", resp.Volume.VolumeId)
				}
			} else {
				if tc.resp != nil && resp == nil {
					t.Errorf("mismatch in response")
				}
				if tc.resp == nil && resp != nil {
					t.Errorf("mismatch in response")
				}
				if !cmp.Equal(resp, tc.resp, protocmp.Transform()) {
					t.Errorf("test %q failed: got resp %+v, expected %+v, diff: %s", tc.name, resp, tc.resp, cmp.Diff(resp, tc.resp, protocmp.Transform()))
				}
			}
		})
	}
}

func TestMultishareDeleteVolume(t *testing.T) {
	testVolName := "pvc-" + string(uuid.NewUUID())
	testShareName := util.ConvertVolToShareName(testVolName)
	testInstanceName1 := "fs-" + string(uuid.NewUUID())
	testVolId := fmt.Sprintf("%s/%s/%s/%s/%s/%s", modeMultishare, testInstanceScPrefix, testProject, testRegion, testInstanceName1, testShareName)
	type OpItem struct {
		id     string
		target string
		verb   string
		done   bool
	}
	tests := []struct {
		name          string
		ops           []OpItem
		initInstance  []*file.MultishareInstance
		initShares    []*file.Share
		req           *csi.DeleteVolumeRequest
		resp          *csi.DeleteVolumeResponse
		errorExpected bool
	}{
		{
			name: "share not found, instance not found, success response",
			req: &csi.DeleteVolumeRequest{
				VolumeId: testVolId,
			},
			resp: &csi.DeleteVolumeResponse{},
		},
		{
			name: "share not found, instance not ready (instance op in progress), error response",
			req: &csi.DeleteVolumeRequest{
				VolumeId: testVolId,
			},
			initInstance: []*file.MultishareInstance{
				{
					Name:     testInstanceName1,
					Location: testRegion,
					Project:  testProject,
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
					},
					CapacityBytes: 1 * util.Tb,
					Tier:          "Enterprise",
					Network: file.Network{
						Ip: testIP,
					},
				},
			},
			ops: []OpItem{
				{
					id:     "op1",
					target: fmt.Sprintf(instanceUriFmt, testProject, testRegion, testInstanceName1),
					verb:   "update",
				},
			},
			errorExpected: true,
		},
		{
			name: "share not found, instance not ready (share op in progress for the instance), error response",
			req: &csi.DeleteVolumeRequest{
				VolumeId: testVolId,
			},
			initInstance: []*file.MultishareInstance{
				{
					Name:     testInstanceName1,
					Location: testRegion,
					Project:  testProject,
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
					},
					CapacityBytes: 1 * util.Tb,
					Tier:          "Enterprise",
					Network: file.Network{
						Ip: testIP,
					},
				},
			},
			ops: []OpItem{
				{
					id:     "op1",
					target: fmt.Sprintf(shareUriFmt, testProject, testRegion, testInstanceName1, testShareName),
					verb:   "update",
				},
			},
			errorExpected: true,
		},
		{
			name: "share not found, instance ready with 0 shares, instance deleted, success response",
			req: &csi.DeleteVolumeRequest{
				VolumeId: testVolId,
			},
			initInstance: []*file.MultishareInstance{
				{
					Name:     testInstanceName1,
					Location: testRegion,
					Project:  testProject,
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
					},
					CapacityBytes: 1 * util.Tb,
					Tier:          "Enterprise",
					Network: file.Network{
						Ip: testIP,
					},
				},
			},
			resp: &csi.DeleteVolumeResponse{},
		},
		{
			name: "share found, share deleted, instance ready with 0 shares, instance deleted, success response",
			req: &csi.DeleteVolumeRequest{
				VolumeId: testVolId,
			},
			initInstance: []*file.MultishareInstance{
				{
					Name:     testInstanceName1,
					Location: testRegion,
					Project:  testProject,
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
					},
					CapacityBytes: 1 * util.Tb,
					Tier:          "Enterprise",
					Network: file.Network{
						Ip: testIP,
					},
				},
			},
			initShares: []*file.Share{
				{
					Name: testShareName,
					Parent: &file.MultishareInstance{
						Project:  testProject,
						Location: testRegion,
						Name:     testInstanceName1,
						Labels: map[string]string{
							util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
						},
						CapacityBytes: 1 * util.Tb,
						Tier:          "Enterprise",
						Network: file.Network{
							Ip: testIP,
						},
					},
					MountPointName: testShareName,
				},
			},
			resp: &csi.DeleteVolumeResponse{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var v1beta1ops []*filev1beta1multishare.Operation
			for _, item := range tc.ops {
				var meta filev1beta1multishare.OperationMetadata
				meta.Target = item.target
				meta.Verb = item.verb
				bytes, _ := json.Marshal(meta)
				v1beta1ops = append(v1beta1ops, &filev1beta1multishare.Operation{
					Name:     item.id,
					Done:     item.done,
					Metadata: bytes,
				})
			}

			s, err := file.NewFakeServiceForMultishare(tc.initInstance, tc.initShares, v1beta1ops)
			if err != nil {
				t.Fatalf("failed to fake service: %v", err)
			}
			cloudProvider, _ := cloud.NewFakeCloud()
			cloudProvider.File = s
			config := &controllerServerConfig{
				driver:          initTestDriver(t),
				fileService:     s,
				cloud:           cloudProvider,
				volumeLocks:     util.NewVolumeLocks(),
				ecfsDescription: "",
			}
			mcs := NewMultishareController(config)
			resp, err := mcs.DeleteVolume(context.Background(), tc.req)
			if tc.errorExpected && err == nil {
				t.Errorf("expected error not found")
			}
			if !tc.errorExpected && err != nil {
				t.Errorf("unexpected error")
			}
			if tc.resp != nil && resp == nil {
				t.Errorf("mismatch in response")
			}
			if tc.resp == nil && resp != nil {
				t.Errorf("mismatch in response")
			}
			if !reflect.DeepEqual(resp, tc.resp) {
				t.Errorf("got resp %+v, expected %+v", resp, tc.resp)
			}
		})
	}

}

func TestMultishareControllerExpandVolume(t *testing.T) {
	testVolName := "pvc-" + string(uuid.NewUUID())
	testShareName := util.ConvertVolToShareName(testVolName)
	testInstanceName1 := "fs-" + string(uuid.NewUUID())
	testVolId := fmt.Sprintf("%s/%s/%s/%s/%s/%s", modeMultishare, testInstanceScPrefix, testProject, testRegion, testInstanceName1, testShareName)
	baseCap := 100 * util.Gb
	mediumCap := 200 * util.Gb
	largeCap := 500 * util.Gb
	type OpItem struct {
		id     string
		target string
		verb   string
		done   bool
	}
	tests := []struct {
		name          string
		ops           []OpItem
		initInstance  []*file.MultishareInstance
		initShares    []*file.Share
		req           *csi.ControllerExpandVolumeRequest
		resp          *csi.ControllerExpandVolumeResponse
		errorExpected bool
	}{
		{
			name: "Target expansion < 100G in required bytes",
			req: &csi.ControllerExpandVolumeRequest{
				VolumeId:      testVolId,
				CapacityRange: &csi.CapacityRange{RequiredBytes: 99 * util.Gb},
			},
			errorExpected: true,
		},
		{
			name: "Target expansion > 1T in required bytes",
			req: &csi.ControllerExpandVolumeRequest{
				VolumeId:      testVolId,
				CapacityRange: &csi.CapacityRange{RequiredBytes: 2 * util.Tb},
			},
			errorExpected: true,
		},
		{
			name: "share not found, error respond",
			req: &csi.ControllerExpandVolumeRequest{
				VolumeId:      testVolId,
				CapacityRange: &csi.CapacityRange{RequiredBytes: int64(mediumCap)},
			},
			initInstance: []*file.MultishareInstance{
				{
					Name:     testInstanceName1,
					Location: testRegion,
					Project:  testProject,
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
					},
					CapacityBytes: 1 * util.Tb,
					Tier:          "Enterprise",
					Network: file.Network{
						Ip: testIP,
					},
				},
			},
			errorExpected: true,
		},
		{
			name: "share already larger than request, succeed",
			req: &csi.ControllerExpandVolumeRequest{
				VolumeId:      testVolId,
				CapacityRange: &csi.CapacityRange{RequiredBytes: int64(mediumCap)},
			},
			initInstance: []*file.MultishareInstance{
				{
					Name:     testInstanceName1,
					Location: testRegion,
					Project:  testProject,
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
					},
					CapacityBytes: 1 * util.Tb,
					Tier:          "Enterprise",
					Network: file.Network{
						Ip: testIP,
					},
				},
			},
			initShares: []*file.Share{
				{
					Name: testShareName,
					Parent: &file.MultishareInstance{
						Project:  testProject,
						Location: testRegion,
						Name:     testInstanceName1,
						Labels: map[string]string{
							util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
						},
						CapacityBytes: 1 * util.Tb,
						Tier:          "Enterprise",
						Network: file.Network{
							Ip: testIP,
						},
					},
					MountPointName: testShareName,
					CapacityBytes:  int64(largeCap),
				},
			},
			resp: &csi.ControllerExpandVolumeResponse{
				CapacityBytes:         int64(largeCap),
				NodeExpansionRequired: false,
			},
		},
		{
			name: "share found, instance op ongoing, error",
			req: &csi.ControllerExpandVolumeRequest{
				VolumeId:      testVolId,
				CapacityRange: &csi.CapacityRange{RequiredBytes: int64(mediumCap)},
			},
			initInstance: []*file.MultishareInstance{
				{
					Name:     testInstanceName1,
					Location: testRegion,
					Project:  testProject,
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
					},
					CapacityBytes: 1 * util.Tb,
					Tier:          "Enterprise",
					Network: file.Network{
						Ip: testIP,
					},
				},
			},
			initShares: []*file.Share{
				{
					Name: testShareName,
					Parent: &file.MultishareInstance{
						Project:  testProject,
						Location: testRegion,
						Name:     testInstanceName1,
						Labels: map[string]string{
							util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
						},
						CapacityBytes: 1 * util.Tb,
						Tier:          "Enterprise",
						Network: file.Network{
							Ip: testIP,
						},
					},
					MountPointName: testShareName,
					CapacityBytes:  int64(baseCap),
				},
			},
			ops: []OpItem{
				{
					id:     "op1",
					target: fmt.Sprintf(instanceUriFmt, testProject, testRegion, testInstanceName1),
					verb:   "update",
				},
			},
			errorExpected: true,
		},
		{
			name: "share found, instance needs expansion",
			req: &csi.ControllerExpandVolumeRequest{
				VolumeId:      testVolId,
				CapacityRange: &csi.CapacityRange{RequiredBytes: int64(largeCap)},
			},
			initInstance: []*file.MultishareInstance{
				{
					Name:     testInstanceName1,
					Location: testRegion,
					Project:  testProject,
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
					},
					CapacityBytes: 1 * util.Tb,
					Tier:          "Enterprise",
					Network: file.Network{
						Ip: testIP,
					},
				},
			},
			initShares: []*file.Share{
				{
					Name: testShareName,
					Parent: &file.MultishareInstance{
						Project:  testProject,
						Location: testRegion,
						Name:     testInstanceName1,
						Labels: map[string]string{
							util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
						},
						CapacityBytes: 1 * util.Tb,
						Tier:          "Enterprise",
						Network: file.Network{
							Ip: testIP,
						},
					},
					MountPointName: testShareName,
					CapacityBytes:  int64(baseCap),
				},
				{
					Name: testShareName + "1",
					Parent: &file.MultishareInstance{
						Project:  testProject,
						Location: testRegion,
						Name:     testInstanceName1,
						Labels: map[string]string{
							util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
						},
						CapacityBytes: 1 * util.Tb,
						Tier:          "Enterprise",
						Network: file.Network{
							Ip: testIP,
						},
					},
					MountPointName: testShareName + "1",
					CapacityBytes:  int64(mediumCap),
				},
				{
					Name: testShareName + "2",
					Parent: &file.MultishareInstance{
						Project:  testProject,
						Location: testRegion,
						Name:     testInstanceName1,
						Labels: map[string]string{
							util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
						},
						CapacityBytes: 1 * util.Tb,
						Tier:          "Enterprise",
						Network: file.Network{
							Ip: testIP,
						},
					},
					MountPointName: testShareName + "2",
					CapacityBytes:  int64(largeCap),
				},
			},
			resp: &csi.ControllerExpandVolumeResponse{
				CapacityBytes:         int64(largeCap),
				NodeExpansionRequired: false,
			},
		},
		{
			name: "share found, instance does not need expansion",
			req: &csi.ControllerExpandVolumeRequest{
				VolumeId:      testVolId,
				CapacityRange: &csi.CapacityRange{RequiredBytes: int64(largeCap)},
			},
			initInstance: []*file.MultishareInstance{
				{
					Name:     testInstanceName1,
					Location: testRegion,
					Project:  testProject,
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
					},
					CapacityBytes: 1 * util.Tb,
					Tier:          "Enterprise",
					Network: file.Network{
						Ip: testIP,
					},
				},
			},
			initShares: []*file.Share{
				{
					Name: testShareName,
					Parent: &file.MultishareInstance{
						Project:  testProject,
						Location: testRegion,
						Name:     testInstanceName1,
						Labels: map[string]string{
							util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
						},
						CapacityBytes: 1 * util.Tb,
						Tier:          "Enterprise",
						Network: file.Network{
							Ip: testIP,
						},
					},
					MountPointName: testShareName,
					CapacityBytes:  int64(baseCap),
				},
			},
			resp: &csi.ControllerExpandVolumeResponse{
				CapacityBytes:         int64(largeCap),
				NodeExpansionRequired: false,
			},
		},
		{
			name: "shareOp on other shares ongoing for instance",
			req: &csi.ControllerExpandVolumeRequest{
				VolumeId:      testVolId,
				CapacityRange: &csi.CapacityRange{RequiredBytes: int64(mediumCap)},
			},
			initInstance: []*file.MultishareInstance{
				{
					Name:     testInstanceName1,
					Location: testRegion,
					Project:  testProject,
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
					},
					CapacityBytes: 1 * util.Tb,
					Tier:          "Enterprise",
					Network: file.Network{
						Ip: testIP,
					},
				},
			},
			initShares: []*file.Share{
				{
					Name: testShareName,
					Parent: &file.MultishareInstance{
						Project:  testProject,
						Location: testRegion,
						Name:     testInstanceName1,
						Labels: map[string]string{
							util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
						},
						CapacityBytes: 1 * util.Tb,
						Tier:          "Enterprise",
						Network: file.Network{
							Ip: testIP,
						},
					},
					MountPointName: testShareName,
					CapacityBytes:  int64(baseCap),
				},
				{
					Name: testShareName + "1",
					Parent: &file.MultishareInstance{
						Project:  testProject,
						Location: testRegion,
						Name:     testInstanceName1,
						Labels: map[string]string{
							util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
						},
						CapacityBytes: 1 * util.Tb,
						Tier:          "Enterprise",
						Network: file.Network{
							Ip: testIP,
						},
					},
					MountPointName: testShareName + "1",
					CapacityBytes:  int64(mediumCap),
				},
			},
			ops: []OpItem{
				{
					id:     "op1",
					target: fmt.Sprintf(shareUriFmt, testProject, testRegion, testInstanceName1, testShareName+"1"),
					verb:   "update",
				},
			},
			errorExpected: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var v1beta1ops []*filev1beta1multishare.Operation
			for _, item := range tc.ops {
				var meta filev1beta1multishare.OperationMetadata
				meta.Target = item.target
				meta.Verb = item.verb
				bytes, _ := json.Marshal(meta)
				v1beta1ops = append(v1beta1ops, &filev1beta1multishare.Operation{
					Name:     item.id,
					Done:     item.done,
					Metadata: bytes,
				})
			}

			s, err := file.NewFakeServiceForMultishare(tc.initInstance, tc.initShares, v1beta1ops)
			if err != nil {
				t.Fatalf("failed to fake service: %v", err)
			}
			cloudProvider, _ := cloud.NewFakeCloud()
			cloudProvider.File = s
			config := &controllerServerConfig{
				driver:          initTestDriver(t),
				fileService:     s,
				cloud:           cloudProvider,
				volumeLocks:     util.NewVolumeLocks(),
				ecfsDescription: "",
			}
			mcs := NewMultishareController(config)
			resp, err := mcs.ControllerExpandVolume(context.Background(), tc.req)
			if tc.errorExpected && err == nil {
				t.Errorf("expected error not found")
			}
			if !tc.errorExpected && err != nil {
				t.Errorf("unexpected error")
			}
			if tc.resp != nil && resp == nil {
				t.Errorf("mismatch in response")
			}
			if tc.resp == nil && resp != nil {
				t.Errorf("mismatch in response")
			}
			if !reflect.DeepEqual(resp, tc.resp) {
				t.Errorf("got resp %+v, expected %+v", resp, tc.resp)
			}
		})
	}
}

func TestIsValidMaxVolSize(t *testing.T) {
	tests := []struct {
		val         int64
		expectedret bool
	}{
		{
			val: 0,
		},
		{
			val:         128 * util.Gb,
			expectedret: true,
		},
		{
			val:         256 * util.Gb,
			expectedret: true,
		},
		{
			val:         512 * util.Gb,
			expectedret: true,
		},
		{
			val:         1 * util.Tb,
			expectedret: true,
		},
		{
			val:         1024 * util.Gb,
			expectedret: true,
		},
		{
			val:         131072 * util.Mb,
			expectedret: true,
		},
		// Negative test cases
		{
			val:         131073 * util.Mb,
			expectedret: false,
		},
		{
			val:         131073,
			expectedret: false,
		},
		{
			val:         1023 * util.Tb,
			expectedret: false,
		},
		{
			val:         127 * util.Gb,
			expectedret: false,
		},
		{
			val:         257 * util.Gb,
			expectedret: false,
		},
		{
			val:         -1024 * util.Gb,
			expectedret: false,
		},
		{
			val:         0,
			expectedret: false,
		},
	}
	for _, tc := range tests {
		if isValidMaxVolSize(tc.val) != tc.expectedret {
			t.Errorf("failed for val %d, expected ret %v", tc.val, tc.expectedret)
		}
	}
}

func TestParseMaxVolumeSizeParam(t *testing.T) {
	tests := []struct {
		name                          string
		req                           *csi.CreateVolumeRequest
		features                      *GCFSDriverFeatureOptions
		expectedSharesPerInstance     int
		expectedMaxShareCapacityBytes int64
		expectError                   bool
	}{
		{
			name: "feature not enabled, param set",
			req: &csi.CreateVolumeRequest{
				Parameters: map[string]string{
					paramMaxVolumeSize: "128Gi",
				},
			},
			expectError: true,
		},
		{
			name: "feature enabled, param set, value not set",
			features: &GCFSDriverFeatureOptions{
				FeatureMaxSharesPerInstance: &FeatureMaxSharesPerInstance{
					Enabled: true,
				},
			},
			req: &csi.CreateVolumeRequest{
				Parameters: map[string]string{
					paramMaxVolumeSize: "",
				},
			},
			expectError: true,
		},
		{
			name: "feature enabled, param set, invalid value test1",
			features: &GCFSDriverFeatureOptions{
				FeatureMaxSharesPerInstance: &FeatureMaxSharesPerInstance{
					Enabled: true,
				},
			},
			req: &csi.CreateVolumeRequest{
				Parameters: map[string]string{
					paramMaxVolumeSize: "-128Gi",
				},
			},
			expectError: true,
		},
		{
			name: "feature enabled, param set, invalid value",
			features: &GCFSDriverFeatureOptions{
				FeatureMaxSharesPerInstance: &FeatureMaxSharesPerInstance{
					Enabled: true,
				},
			},
			req: &csi.CreateVolumeRequest{
				Parameters: map[string]string{
					paramMaxVolumeSize: "12i",
				},
			},
			expectError: true,
		},
		{
			name: "feature enabled, param set, unexpected value test1",
			features: &GCFSDriverFeatureOptions{
				FeatureMaxSharesPerInstance: &FeatureMaxSharesPerInstance{
					Enabled: true,
				},
			},
			req: &csi.CreateVolumeRequest{
				Parameters: map[string]string{
					paramMaxVolumeSize: "100Gi",
				},
			},
			expectError: true,
		},
		{
			name: "feature enabled, param set, unexpected value test2",
			features: &GCFSDriverFeatureOptions{
				FeatureMaxSharesPerInstance: &FeatureMaxSharesPerInstance{
					Enabled: true,
				},
			},
			req: &csi.CreateVolumeRequest{
				Parameters: map[string]string{
					paramMaxVolumeSize: "100Gi",
				},
			},
			expectError: true,
		},
		{
			name: "feature enabled, param not set, defaults returned",
			features: &GCFSDriverFeatureOptions{
				FeatureMaxSharesPerInstance: &FeatureMaxSharesPerInstance{
					Enabled: true,
				},
			},
			req: &csi.CreateVolumeRequest{
				Parameters: map[string]string{},
			},
			expectedSharesPerInstance:     util.MaxSharesPerInstance,
			expectedMaxShareCapacityBytes: util.MaxShareSizeBytes,
		},
		{
			name: "feature enabled, param set success test1",
			features: &GCFSDriverFeatureOptions{
				FeatureMaxSharesPerInstance: &FeatureMaxSharesPerInstance{
					Enabled: true,
				},
			},
			req: &csi.CreateVolumeRequest{
				Parameters: map[string]string{
					paramMaxVolumeSize: "128Gi",
				},
			},
			expectedSharesPerInstance:     80,
			expectedMaxShareCapacityBytes: 128 * util.Gb,
		},
		{
			name: "feature enabled, param set success test2",
			features: &GCFSDriverFeatureOptions{
				FeatureMaxSharesPerInstance: &FeatureMaxSharesPerInstance{
					Enabled: true,
				},
			},
			req: &csi.CreateVolumeRequest{
				Parameters: map[string]string{
					paramMaxVolumeSize: "256Gi",
				},
			},
			expectedSharesPerInstance:     40,
			expectedMaxShareCapacityBytes: 256 * util.Gb,
		},
		{
			name: "feature enabled, param set success test3",
			features: &GCFSDriverFeatureOptions{
				FeatureMaxSharesPerInstance: &FeatureMaxSharesPerInstance{
					Enabled: true,
				},
			},
			req: &csi.CreateVolumeRequest{
				Parameters: map[string]string{
					paramMaxVolumeSize: "512Gi",
				},
			},
			expectedSharesPerInstance:     20,
			expectedMaxShareCapacityBytes: 512 * util.Gb,
		},
		{
			name: "feature enabled, param set success test4",
			features: &GCFSDriverFeatureOptions{
				FeatureMaxSharesPerInstance: &FeatureMaxSharesPerInstance{
					Enabled: true,
				},
			},
			req: &csi.CreateVolumeRequest{
				Parameters: map[string]string{
					paramMaxVolumeSize: "1024Gi",
				},
			},
			expectedSharesPerInstance:     10,
			expectedMaxShareCapacityBytes: 1 * util.Tb,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := initTestMultishareControllerWithFeatureOpts(t, tc.features)
			sharePerInstance, maxShareCapacity, err := m.parseMaxVolumeSizeParam(tc.req.GetParameters())
			if tc.expectError && err == nil {
				t.Errorf("failed")
			}
			if !tc.expectError && err != nil {
				t.Errorf("failed")
			}
			if sharePerInstance != tc.expectedSharesPerInstance || maxShareCapacity != tc.expectedMaxShareCapacityBytes {
				t.Errorf("failed")
			}
		})
	}
}

func TestCreateMultishareSnapshot(t *testing.T) {

	type BackupTestInfo struct {
		backup *file.BackupInfo
		state  string
	}
	backupName := "mybackup"
	backupName2 := "mybackup2"
	testInstanceName1 := "fs-" + string(uuid.NewUUID())
	defaultSourceVolumeID := modeMultishare + "/" + testRegion + "/" + testInstanceName1 + "/" + testShareName
	defaultBackupUri := fmt.Sprintf("projects/%s/locations/%s/backups/%s", testProject, testRegion, backupName)

	features := &GCFSDriverFeatureOptions{
		FeatureMultishareBackups: &FeatureMultishareBackups{
			Enabled: true,
		},
	}
	cases := []struct {
		name          string
		req           *csi.CreateSnapshotRequest
		resp          *csi.CreateSnapshotResponse
		initialBackup *BackupTestInfo
		features      *GCFSDriverFeatureOptions
		expectErr     bool
	}{
		//Failure test cases/
		{
			name: "Feature multisharebackups is not enabled",
			req: &csi.CreateSnapshotRequest{
				SourceVolumeId: modeMultishare + "/" + testRegion + "/" + "differnetInstanceName" + "/" + testShareName,
				Name:           backupName,
				Parameters: map[string]string{
					util.VolumeSnapshotTypeKey: "backup",
				},
			},
			expectErr: true,
		},
		{
			name: "Existing backup found, with different instance ID, error expected",
			req: &csi.CreateSnapshotRequest{
				SourceVolumeId: modeMultishare + "/" + testRegion + "/" + "differnetInstanceName" + "/" + testShareName,
				Name:           backupName,
				Parameters: map[string]string{
					util.VolumeSnapshotTypeKey: "backup",
				},
			},
			features: features,
			initialBackup: &BackupTestInfo{
				backup: &file.BackupInfo{
					Project:            testProject,
					Location:           testRegion,
					SourceInstanceName: testInstanceName1,
					SourceShare:        testShareName,
					Name:               backupName,
					BackupURI:          defaultBackupUri,
					SourceVolumeId:     modeMultishare + "/" + testRegion + "/" + testInstanceName1 + "/" + testShareName,
				},
			},
			expectErr: true,
		},
		{
			name: "Existing backup found, with different share name, error expected",
			req: &csi.CreateSnapshotRequest{
				SourceVolumeId: modeMultishare + "/" + testRegion + "/" + testInstanceName1 + "/" + "differentsharename",
				Name:           backupName,
				Parameters: map[string]string{
					util.VolumeSnapshotTypeKey: "backup",
				},
			},
			features: features,
			initialBackup: &BackupTestInfo{
				backup: &file.BackupInfo{
					Project:            testProject,
					Location:           testRegion,
					SourceInstanceName: testInstanceName1,
					SourceShare:        testShareName,
					Name:               backupName,
					BackupURI:          defaultBackupUri,
					SourceVolumeId:     modeMultishare + "/" + testRegion + "/" + testInstanceName1 + "/" + testShareName,
				},
			},
			expectErr: true,
		},
		{
			name: "Existing backup found in state CREATING, error expected",
			req: &csi.CreateSnapshotRequest{
				SourceVolumeId: defaultSourceVolumeID,
				Name:           backupName,
				Parameters: map[string]string{
					util.VolumeSnapshotTypeKey: "backup",
				},
			},
			features: features,
			initialBackup: &BackupTestInfo{
				backup: &file.BackupInfo{
					Project:            testProject,
					Location:           testRegion,
					SourceInstanceName: testInstanceName1,
					SourceShare:        testShareName,
					Name:               backupName,
					BackupURI:          defaultBackupUri,
					SourceVolumeId:     modeMultishare + "/" + testRegion + "/" + testInstanceName1 + "/" + testShareName,
				},
				state: "CREATING",
			},
			expectErr: true,
		},
		{
			name: "Existing backup found in state FINALIZING, error expected",
			req: &csi.CreateSnapshotRequest{
				SourceVolumeId: defaultSourceVolumeID,
				Name:           backupName,
				Parameters: map[string]string{
					util.VolumeSnapshotTypeKey: "backup",
				},
			},
			features: features,
			initialBackup: &BackupTestInfo{
				backup: &file.BackupInfo{
					Project:            testProject,
					Location:           testRegion,
					SourceInstanceName: testInstanceName1,
					SourceShare:        testShareName,
					Name:               backupName,
					BackupURI:          defaultBackupUri,
					SourceVolumeId:     modeMultishare + "/" + testRegion + "/" + testInstanceName1 + "/" + testShareName,
				},
				state: "FINALIZING",
			},
			expectErr: true,
		},
		{
			name: "Parameters contain misconfigured labels(invalid KV separator(:) used)",
			req: &csi.CreateSnapshotRequest{
				SourceVolumeId: "modeInstance/us-central1/myinstance/myshare",
				Name:           backupName,
				Parameters: map[string]string{
					util.VolumeSnapshotTypeKey: "backup",
					"labels":                   "key1:value1",
				},
			},
			initialBackup: &BackupTestInfo{
				backup: &file.BackupInfo{
					Project:            testProject,
					Location:           testRegion,
					SourceInstanceName: testInstanceName1,
					SourceShare:        testShareName,
					Name:               backupName,
					BackupURI:          defaultBackupUri,
					SourceVolumeId:     modeMultishare + "/" + testRegion + "/" + testInstanceName1 + "/" + testShareName,
				},
				state: "CREATING",
			},
			expectErr: true,
		},
		{
			name: "adding tags to multishare snapshot fails(failure scenario mocked)",
			req: &csi.CreateSnapshotRequest{
				SourceVolumeId: defaultSourceVolumeID,
				Name:           backupName2,
				Parameters: map[string]string{
					util.VolumeSnapshotTypeKey:     "backup",
					cloud.ParameterKeyResourceTags: "kubernetes/test1/test1",
				},
			},
			features:      features,
			initialBackup: nil,
			expectErr:     true,
		},
		//Success test cases
		{
			name: "No existing backup",
			req: &csi.CreateSnapshotRequest{
				SourceVolumeId: defaultSourceVolumeID,
				Name:           backupName,
				Parameters: map[string]string{
					util.VolumeSnapshotTypeKey: "backup",
				},
			},
			features: features,
			resp: &csi.CreateSnapshotResponse{
				Snapshot: &csi.Snapshot{
					SizeBytes:      1 * util.Tb,
					SnapshotId:     defaultBackupUri,
					SourceVolumeId: defaultSourceVolumeID,
					ReadyToUse:     true,
				},
			},
			initialBackup: nil,
		},
		{
			name: "Existing backup found, with same volume Id, in state READY",
			req: &csi.CreateSnapshotRequest{
				SourceVolumeId: defaultSourceVolumeID,
				Name:           backupName,
				Parameters: map[string]string{
					util.VolumeSnapshotTypeKey: "backup",
				},
			},
			features: features,
			initialBackup: &BackupTestInfo{
				backup: &file.BackupInfo{
					Project:            testProject,
					Location:           testRegion,
					SourceInstanceName: testInstanceName1,
					SourceShare:        testShareName,
					Name:               backupName,
					BackupURI:          defaultBackupUri,
					SourceVolumeId:     modeMultishare + "/" + testRegion + "/" + testInstanceName1 + "/" + testShareName,
				},
				state: "READY",
			},
		},
		{
			// If the incorrect labels were added, labels processing will not happen for already
			// existing backup resources.
			name: "Existing backup found, in state READY. Labels will not be processed.",
			req: &csi.CreateSnapshotRequest{
				SourceVolumeId: defaultSourceVolumeID,
				Name:           backupName,
				Parameters: map[string]string{
					util.VolumeSnapshotTypeKey: "backup",
					"labels":                   "key1:value1",
				},
			},
			features: features,
			initialBackup: &BackupTestInfo{
				backup: &file.BackupInfo{
					Project:            testProject,
					Location:           testRegion,
					SourceInstanceName: testInstanceName1,
					SourceShare:        testShareName,
					Name:               backupName,
					BackupURI:          defaultBackupUri,
					SourceVolumeId:     modeMultishare + "/" + testRegion + "/" + testInstanceName1 + "/" + testShareName,
				},
				state: "READY",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := initTestMultishareControllerWithFeatureOpts(t, tc.features)
			fileService := m.fileService

			m.tagManager.(*cloud.FakeTagServiceManager).
				On("AttachResourceTags", context.TODO(), cloud.FilestoreBackUp, backupName, testRegion, tc.req.GetName(), tc.req.GetParameters()).
				Return(nil)
			m.tagManager.(*cloud.FakeTagServiceManager).
				On("AttachResourceTags", context.TODO(), cloud.FilestoreBackUp, backupName2, testRegion, tc.req.GetName(), tc.req.GetParameters()).
				Return(fmt.Errorf("mock failure: error while adding tags to multishare snapshot"))

			if tc.initialBackup != nil {
				existingBackup, _ := fileService.CreateBackup(context.TODO(), tc.initialBackup.backup)
				if tc.initialBackup.state != "" {
					existingBackup.State = tc.initialBackup.state
				}
			}
			resp, err := m.CreateSnapshot(context.TODO(), tc.req)
			if !tc.expectErr && err != nil {
				t.Errorf("test %q failed: %v", tc.name, err)
			}
			if tc.expectErr && err == nil {
				t.Errorf("test %q failed; got success", tc.name)
			}

			if tc.resp != nil {
				if resp.Snapshot.SizeBytes != tc.resp.Snapshot.SizeBytes {
					t.Errorf("test %q failed, %v, mismatch, got %v, want %v", tc.name, "SizeBytes", resp.Snapshot.SizeBytes, tc.resp.Snapshot.SizeBytes)
				}
				if resp.Snapshot.SnapshotId != tc.resp.Snapshot.SnapshotId {
					t.Errorf("test %q failed, %v, mismatch, got %v, want %v", tc.name, "SnapshotId", resp.Snapshot.SnapshotId, tc.resp.Snapshot.SnapshotId)
				}
				if resp.Snapshot.SourceVolumeId != tc.resp.Snapshot.SourceVolumeId {
					t.Errorf("test %q failed, %v, mismatch, got %v, want %v", tc.name, "SourceVolumeId", resp.Snapshot.SourceVolumeId, tc.resp.Snapshot.SourceVolumeId)
				}
				if resp.Snapshot.ReadyToUse != tc.resp.Snapshot.ReadyToUse {
					t.Errorf("test %q failed, %v, mismatch, got %v, want %v", tc.name, "ReadyToUse", resp.Snapshot.ReadyToUse, tc.resp.Snapshot.ReadyToUse)
				}
			}
			if !tc.expectErr && tc.initialBackup == nil {
				backup, _ := fileService.GetBackup(context.TODO(), defaultBackupUri)
				if backup.Backup.Labels[tagKeyCreatedBy] != "test-driver" {
					t.Errorf("labels check for %v failed on test %q, got %v, want %v", tagKeyCreatedBy, tc.name, backup.Backup.Labels[tagKeyCreatedBy], "test-driver")
				}
				if backup.Backup.Labels[tagKeySnapshotName] != tc.req.Name {
					t.Errorf("labels check for %v failed on test %q, got %v, want %v", tagKeySnapshotName, tc.name, backup.Backup.Labels[tagKeySnapshotName], tc.req.Name)
				}
			}
		})

	}
}
