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
	"strings"
	"testing"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	filev1beta1multishare "google.golang.org/api/file/v1beta1"
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
				tagKeyCreatedBy:       testDrivernameLabelValue,
				tagKeyClusterName:     testClusterName,
				tagKeyClusterLocation: testLocation,
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
				tagKeyClusterName:                      testClusterName,
				tagKeyClusterLocation:                  testLocation,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			label, err := extractInstanceLabels(tc.params, tc.driver, testClusterName, testLocation)
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
					tagKeyClusterLocation:                  testRegion,
					tagKeyClusterName:                      testClusterName,
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
				t.Errorf("unexpected error: %q", err)
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
			m := initTestMultishareController(t)
			resp, err := m.generateCSICreateVolumeResponse(tc.prefix, tc.share)
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
		errorExpected     bool
		checkOnlyVolidFmt bool // for auto generated instance, the instance name is not known
	}{
		{
			name: "create volume called with volume content source",
			req: &csi.CreateVolumeRequest{
				Name: testVolName,
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 100 * util.Gb,
				},
				Parameters: map[string]string{
					paramMultishareInstanceScLabel: testInstanceScPrefix,
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
				VolumeContentSource: &csi.VolumeContentSource{},
			},
			errorExpected: true,
		},
		{
			name: "create volume called with volume size < 100G in required bytes",
			req: &csi.CreateVolumeRequest{
				Name: testVolName,
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 99 * util.Gb,
				},
				Parameters: map[string]string{
					paramMultishareInstanceScLabel: testInstanceScPrefix,
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
					paramMultishareInstanceScLabel: testInstanceScPrefix,
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
					paramMultishareInstanceScLabel: testInstanceScPrefix,
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
					paramMultishareInstanceScLabel: testInstanceScPrefix,
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
					paramMultishareInstanceScLabel: testInstanceScPrefix,
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
			name: "1 initial ready 1Tib instances with 0 shares, 1 busy instance, create 100Gib share and use the ready instance, success response",
			initInstances: []*file.MultishareInstance{
				{
					Name:     testInstanceName1,
					Location: "us-central1",
					Project:  "test-project",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
						tagKeyClusterLocation:                  testLocation,
						tagKeyClusterName:                      "",
					},
					CapacityBytes: 1 * util.Tb,
					Tier:          "enterprise",
					Network: file.Network{
						Ip:          testIP,
						Name:        defaultNetwork,
						ConnectMode: directPeering,
					},
					State: "READY",
				},
				{
					Name:     testInstanceName2,
					Location: "us-central1",
					Project:  "test-project",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
						tagKeyClusterLocation:                  testLocation,
						tagKeyClusterName:                      "",
					},
					CapacityBytes: 1 * util.Tb,
					Tier:          "enterprise",
					Network: file.Network{
						Ip:          testIP,
						Name:        defaultNetwork,
						ConnectMode: directPeering,
					},
					State: "READY",
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
					paramMultishareInstanceScLabel: testInstanceScPrefix,
					paramTier:                      "enterprise",
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
						attrIP: testIP,
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
						tagKeyClusterLocation:                  testLocation,
						tagKeyClusterName:                      "",
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
					paramMultishareInstanceScLabel: testInstanceScPrefix,
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
						tagKeyClusterLocation:                  testLocation,
						tagKeyClusterName:                      "",
					},
					CapacityBytes: 1 * util.Tb,
					Tier:          "Enterprise",
					Network: file.Network{
						Ip: testIP,
					},
					State: "READY",
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
						State: "READY",
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
					paramMultishareInstanceScLabel: testInstanceScPrefix,
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
						attrIP: testIP,
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
			}
			mcs := NewMultishareController(config)
			resp, err := mcs.CreateVolume(context.Background(), tc.req)
			if tc.errorExpected && err == nil {
				t.Errorf("expected error not found")
			}
			if !tc.errorExpected && err != nil {
				t.Errorf("unexpected error")
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
				if !reflect.DeepEqual(resp, tc.resp) {
					t.Errorf("got resp %+v, expected %+v", resp, tc.resp)
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
			name: "share not found, instance not found, succes response",
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
