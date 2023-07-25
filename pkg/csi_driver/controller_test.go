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
	"fmt"
	"reflect"
	"testing"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	cloud "sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/file"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/util"
)

const (
	testProject            = "test-project"
	testZone               = "us-central1-c"
	testLocation           = "us-central1-c"
	testRegion             = "us-central1"
	testIP                 = "1.1.1.1"
	testCSIVolume          = "test-csi"
	testCSIVolume2         = "test-csi-2"
	testVolumeID           = "modeInstance/us-central1-c/test-csi/vol1"
	testMultishareVolumeID = modeMultishare + "/us-central1-c/test-csi/share1"
	testReservedIPV4CIDR   = "192.168.92.0/26"
	testBytes              = 1 * util.Tb
)

func initTestController(t *testing.T) csi.ControllerServer {
	fileService, err := file.NewFakeService()
	if err != nil {
		t.Fatalf("failed to initialize GCFS service: %v", err)
	}

	cloudProvider, err := cloud.NewFakeCloud()
	if err != nil {
		t.Fatalf("Failed to get cloud provider: %v", err)
	}
	return newControllerServer(&controllerServerConfig{
		driver:      initTestDriver(t),
		fileService: fileService,
		cloud:       cloudProvider,
		volumeLocks: util.NewVolumeLocks(),
		features:    &GCFSDriverFeatureOptions{FeatureLockRelease: &FeatureLockRelease{}},
	})
}

func initBlockingTestController(t *testing.T, operationUnblocker chan chan struct{}) csi.ControllerServer {
	fileService, err := file.NewFakeBlockingService(operationUnblocker)
	if err != nil {
		t.Fatalf("failed to initialize blocking GCFS service: %v", err)
	}

	cloudProvider, err := cloud.NewFakeCloud()
	if err != nil {
		t.Fatalf("Failed to get cloud provider: %v", err)
	}
	return newControllerServer(&controllerServerConfig{
		driver:      initTestDriver(t),
		fileService: fileService,
		cloud:       cloudProvider,
		volumeLocks: util.NewVolumeLocks(),
		features:    &GCFSDriverFeatureOptions{FeatureLockRelease: &FeatureLockRelease{}},
	})
}

func TestCreateVolumeFromSnapshot(t *testing.T) {
	type BackupInfo struct {
		s              *file.ServiceInstance
		backupName     string
		backupLocation string
		SourceVolumeId string
	}
	backupName := "mybackup"
	instanceName := "myinstance"
	shareName := "myshare"
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

	cases := []struct {
		name          string
		req           *csi.CreateVolumeRequest
		resp          *csi.CreateVolumeResponse
		initialBackup *BackupInfo
		expectErr     bool
	}{
		{
			name: "from default tier snapshot",
			req: &csi.CreateVolumeRequest{
				Name: testCSIVolume,
				VolumeContentSource: &csi.VolumeContentSource{
					Type: &csi.VolumeContentSource_Snapshot{
						Snapshot: &csi.VolumeContentSource_SnapshotSource{
							SnapshotId: "projects/test-project/locations/us-central1/backups/mybackup",
						},
					},
				},
				Parameters:         map[string]string{"tier": defaultTier},
				VolumeCapabilities: volumeCapabilities,
			},
			resp: &csi.CreateVolumeResponse{
				Volume: &csi.Volume{
					CapacityBytes: defaultTierMinSize,
					VolumeId:      testVolumeID,
					VolumeContext: map[string]string{
						attrIP:     testIP,
						attrVolume: newInstanceVolume,
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
			initialBackup: &BackupInfo{
				s: &file.ServiceInstance{
					Project:  testProject,
					Location: testZone,
					Name:     instanceName,
					Tier:     defaultTier,
					Volume: file.Volume{
						Name:      shareName,
						SizeBytes: defaultTierMinSize,
					},
				},
				backupName:     backupName,
				backupLocation: testRegion,
				SourceVolumeId: modeInstance + "/" + testZone + "/" + instanceName + "/" + shareName,
			},
		},
		{
			name: "from premium tier snapshot",
			req: &csi.CreateVolumeRequest{
				Name: testCSIVolume,
				VolumeContentSource: &csi.VolumeContentSource{
					Type: &csi.VolumeContentSource_Snapshot{
						Snapshot: &csi.VolumeContentSource_SnapshotSource{
							SnapshotId: "projects/test-project/locations/us-central1/backups/mybackup",
						},
					},
				},
				Parameters:         map[string]string{"tier": premiumTier},
				VolumeCapabilities: volumeCapabilities,
			},
			resp: &csi.CreateVolumeResponse{
				Volume: &csi.Volume{
					CapacityBytes: premiumTierMinSize,
					VolumeId:      testVolumeID,
					VolumeContext: map[string]string{
						attrIP:     testIP,
						attrVolume: newInstanceVolume,
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
			initialBackup: &BackupInfo{
				s: &file.ServiceInstance{
					Project:  testProject,
					Location: testZone,
					Name:     instanceName,
					Tier:     premiumTier,
					Volume: file.Volume{
						Name:      shareName,
						SizeBytes: premiumTierMinSize,
					},
				},
				backupName:     backupName,
				backupLocation: testRegion,
				SourceVolumeId: modeInstance + "/" + testZone + "/" + instanceName + "/" + shareName,
			},
		},
		{
			name: "from enterprise tier snapshot",
			req: &csi.CreateVolumeRequest{
				Name: testCSIVolume,
				VolumeContentSource: &csi.VolumeContentSource{
					Type: &csi.VolumeContentSource_Snapshot{
						Snapshot: &csi.VolumeContentSource_SnapshotSource{
							SnapshotId: "projects/test-project/locations/us-central1/backups/mybackup",
						},
					},
				},
				Parameters:         map[string]string{"tier": enterpriseTier},
				VolumeCapabilities: volumeCapabilities,
			},
			resp: &csi.CreateVolumeResponse{
				Volume: &csi.Volume{
					CapacityBytes: testBytes,
					VolumeId:      testVolumeID,
					VolumeContext: map[string]string{
						attrIP:     testIP,
						attrVolume: newInstanceVolume,
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
			initialBackup: &BackupInfo{
				s: &file.ServiceInstance{
					Project:  testProject,
					Location: testRegion,
					Name:     instanceName,
					Tier:     enterpriseTier,
					Volume: file.Volume{
						Name:      shareName,
						SizeBytes: testBytes,
					},
				},
				backupName:     backupName,
				backupLocation: testRegion,
				SourceVolumeId: modeInstance + "/" + testRegion + "/" + instanceName + "/" + shareName,
			},
		},
	}

	for _, test := range cases {
		cs := initTestController(t).(*controllerServer)

		//Create initial backup
		backupInfo := &file.BackupInfo{
			Project:            test.initialBackup.s.Project,
			Location:           test.initialBackup.backupLocation,
			SourceInstanceName: test.initialBackup.s.Name,
			SourceShare:        test.initialBackup.s.Volume.Name,
			Name:               test.initialBackup.backupName,
			SourceVolumeId:     test.initialBackup.SourceVolumeId,
			BackupURI:          test.resp.Volume.ContentSource.GetSnapshot().SnapshotId,
			Labels:             make(map[string]string),
		}

		cs.config.fileService.CreateBackup(context.TODO(), backupInfo)

		// Restore from backup
		resp, err := cs.CreateVolume(context.TODO(), test.req)
		if !test.expectErr && err != nil {
			t.Errorf("test %q failed: %v", test.name, err)
		}
		if test.expectErr && err == nil {
			t.Errorf("test %q failed; got success", test.name)
		}
		if !reflect.DeepEqual(resp, test.resp) {
			t.Errorf("test %q failed: got resp %+v, expected %+v", test.name, resp, test.resp)
		}
	}
}

func TestCreateVolume(t *testing.T) {
	cases := []struct {
		name      string
		req       *csi.CreateVolumeRequest
		resp      *csi.CreateVolumeResponse
		expectErr bool
	}{
		{
			name: "valid defaults",
			req: &csi.CreateVolumeRequest{
				Name: testCSIVolume,
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
					CapacityBytes: 1 * util.Tb,
					VolumeId:      testVolumeID,
					VolumeContext: map[string]string{
						attrIP:     testIP,
						attrVolume: newInstanceVolume,
					},
				},
			},
		},
		{
			name: "name empty",
			req: &csi.CreateVolumeRequest{
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
			expectErr: true,
		},
		{
			name: "invalid volume capability",
			req: &csi.CreateVolumeRequest{
				Name: testCSIVolume,
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "invalid create parameter",
			req: &csi.CreateVolumeRequest{
				Name: testCSIVolume,
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
				Parameters: map[string]string{
					"unknown-parameter": "foo",
				},
			},
			expectErr: true,
		},
		// TODO: create failed
		// TODO: instance already exists error
		// TODO: instance already exists invalid
		// TODO: instance already exists valid
	}

	for _, test := range cases {
		cs := initTestController(t)
		resp, err := cs.CreateVolume(context.TODO(), test.req)
		if !test.expectErr && err != nil {
			t.Errorf("test %q failed: %v", test.name, err)
		}
		if test.expectErr && err == nil {
			t.Errorf("test %q failed; got success", test.name)
		}
		if !reflect.DeepEqual(resp, test.resp) {
			t.Errorf("test %q failed: got resp %+v, expected %+v", test.name, resp, test.resp)
		}
	}
}

func TestDeleteVolume(t *testing.T) {
	cases := []struct {
		name      string
		req       *csi.DeleteVolumeRequest
		expectErr bool
	}{
		{
			name: "valid",
			req: &csi.DeleteVolumeRequest{
				VolumeId: testVolumeID,
			},
		},
		{
			name: "invalid id",
			req: &csi.DeleteVolumeRequest{
				VolumeId: testVolumeID + "/foo",
			},
		},
		{
			name:      "empty id",
			req:       &csi.DeleteVolumeRequest{},
			expectErr: true,
		},
		// TODO: delete failed
	}

	for _, test := range cases {
		cs := initTestController(t)
		_, err := cs.DeleteVolume(context.TODO(), test.req)
		if !test.expectErr && err != nil {
			t.Errorf("test %q failed: %v", test.name, err)
		}
		if test.expectErr && err == nil {
			t.Errorf("test %q failed; got success", test.name)
		}
	}
}

// TODO:
func TestValidateVolumeCapabilities(t *testing.T) {
}

// TODO:
func TestControllerGetCapabilities(t *testing.T) {
}

// TODO:
func TestControllerExpandVolume(t *testing.T) {
}

func TestGetRequestCapacity(t *testing.T) {
	cases := []struct {
		name          string
		capRange      *csi.CapacityRange
		bytes         int64
		tier          string
		errorExpected bool
	}{
		{
			name:  "default",
			bytes: 1 * util.Tb,
			tier:  defaultTier,
		},
		{
			name: "required below min, limit not provided",
			capRange: &csi.CapacityRange{
				RequiredBytes: 100 * util.Gb,
			},
			tier:          defaultTier,
			bytes:         1 * util.Tb,
			errorExpected: false,
		},
		{
			name: "required equals min",
			capRange: &csi.CapacityRange{
				RequiredBytes: 1 * util.Tb,
			},
			tier:  defaultTier,
			bytes: 1 * util.Tb,
		},
		{
			name: "required above min",
			capRange: &csi.CapacityRange{
				RequiredBytes: 1*util.Tb + 1*util.Gb,
			},
			tier:  defaultTier,
			bytes: 1*util.Tb + 1*util.Gb,
		},
		{
			name: "limit equals min",
			capRange: &csi.CapacityRange{
				LimitBytes: 1 * util.Tb,
			},
			tier:  defaultTier,
			bytes: 1 * util.Tb,
		},
		{
			name: "limit above min",
			capRange: &csi.CapacityRange{
				LimitBytes: 1*util.Tb + 1*util.Gb,
			},
			tier:  defaultTier,
			bytes: 1*util.Tb + 1*util.Gb,
		},
		{
			name: "required below min, limit above min",
			capRange: &csi.CapacityRange{
				RequiredBytes: 100 * util.Gb,
				LimitBytes:    2 * util.Tb,
			},
			tier:  defaultTier,
			bytes: 1 * util.Tb,
		},
		{
			name: "required below min, limit below min",
			capRange: &csi.CapacityRange{
				RequiredBytes: 100 * util.Gb,
				LimitBytes:    500 * util.Gb,
			},
			tier:          defaultTier,
			errorExpected: true,
		},
		{
			name: "required above limit",
			capRange: &csi.CapacityRange{
				RequiredBytes: 5 * util.Tb,
				LimitBytes:    2 * util.Tb,
			},
			tier:          defaultTier,
			errorExpected: true,
		},
		{
			name: "limit below min default",
			capRange: &csi.CapacityRange{
				LimitBytes: 100 * util.Gb,
			},
			tier:          defaultTier,
			errorExpected: true,
		},
		{
			name: "required above max default",
			capRange: &csi.CapacityRange{
				RequiredBytes: 100 * util.Tb,
			},
			tier:          defaultTier,
			errorExpected: true,
		},
		{
			name: "limit above max and no min provided",
			capRange: &csi.CapacityRange{
				LimitBytes: 100 * util.Tb,
			},
			tier:          defaultTier,
			bytes:         639 * util.Tb / 10,
			errorExpected: false,
		},
		{
			name: "limit above max but min in range",
			capRange: &csi.CapacityRange{
				LimitBytes:    100 * util.Tb,
				RequiredBytes: 15 * util.Tb,
			},
			tier:  defaultTier,
			bytes: 15 * util.Tb,
		},
		{
			name: "limit below min enterprise",
			capRange: &csi.CapacityRange{
				LimitBytes: 100 * util.Gb,
			},
			tier:          enterpriseTier,
			errorExpected: true,
		},
		{
			name: "required above max enterprise",
			capRange: &csi.CapacityRange{
				RequiredBytes: 100 * util.Tb,
			},
			tier:          enterpriseTier,
			errorExpected: true,
		},
		{
			name: "required and limit both in range enterprise",
			capRange: &csi.CapacityRange{
				RequiredBytes: 2 * util.Tb,
				LimitBytes:    3 * util.Tb,
			},
			tier:  enterpriseTier,
			bytes: 2 * util.Tb,
		},
		{
			name: "limit below min highScale",
			capRange: &csi.CapacityRange{
				LimitBytes: 5 * util.Tb,
			},
			tier:          highScaleTier,
			errorExpected: true,
		},
		{
			name: "required above max highScale",
			capRange: &csi.CapacityRange{
				RequiredBytes: 200 * util.Tb,
			},
			tier:          highScaleTier,
			errorExpected: true,
		},
		{
			name: "required and limit both in range highScale",
			capRange: &csi.CapacityRange{
				RequiredBytes: 20 * util.Tb,
				LimitBytes:    30 * util.Tb,
			},
			tier:  highScaleTier,
			bytes: 20 * util.Tb,
		},
		{
			name: "limit below min premium",
			capRange: &csi.CapacityRange{
				LimitBytes: 1 * util.Tb,
			},
			tier:          premiumTier,
			errorExpected: true,
		},
		{
			name: "required above max premium",
			capRange: &csi.CapacityRange{
				RequiredBytes: 70 * util.Tb,
			},
			tier:          premiumTier,
			errorExpected: true,
		},
		{
			name: "required and limit both in range premium",
			capRange: &csi.CapacityRange{
				RequiredBytes: 3 * util.Tb,
				LimitBytes:    60 * util.Tb,
			},
			tier:  premiumTier,
			bytes: 3 * util.Tb,
		},
		{
			name: "limit below min basicSSD",
			capRange: &csi.CapacityRange{
				LimitBytes: 1 * util.Tb,
			},
			tier:          basicSSDTier,
			errorExpected: true,
		},
		{
			name: "required above max basicSSD",
			capRange: &csi.CapacityRange{
				RequiredBytes: 70 * util.Tb,
			},
			tier:          basicSSDTier,
			errorExpected: true,
		},
		{
			name: "required and limit both in range basicSSD",
			capRange: &csi.CapacityRange{
				RequiredBytes: 3 * util.Tb,
				LimitBytes:    60 * util.Tb,
			},
			tier:  basicSSDTier,
			bytes: 3 * util.Tb,
		},
		{
			name: "limit below min basicHDD",
			capRange: &csi.CapacityRange{
				LimitBytes: 100 * util.Gb,
			},
			tier:          basicHDDTier,
			errorExpected: true,
		},
		{
			name: "required above max basicHDD",
			capRange: &csi.CapacityRange{
				RequiredBytes: 70 * util.Tb,
			},
			tier:          basicHDDTier,
			errorExpected: true,
		},
		{
			name: "required and limit both in range basicHDD",
			capRange: &csi.CapacityRange{
				RequiredBytes: 1 * util.Tb,
				LimitBytes:    60 * util.Tb,
			},
			tier:  basicHDDTier,
			bytes: 1 * util.Tb,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bytes, err := getRequestCapacity(tc.capRange, tc.tier)
			if err != nil && tc.errorExpected {
				return
			}

			if err == nil && tc.errorExpected {
				t.Errorf("Test %q failed: expected error", tc.name)
			}
			if bytes != tc.bytes {
				t.Errorf("test %q failed: got %v bytes, expected %v", tc.name, bytes, tc.bytes)
			}
		})

	}
}

func TestGenerateNewFileInstance(t *testing.T) {
	cases := []struct {
		name      string
		params    map[string]string
		toporeq   *csi.TopologyRequirement
		instance  *file.ServiceInstance
		expectErr bool
	}{
		{
			name: "default params, nil topology requirement",
			instance: &file.ServiceInstance{
				Project:  testProject,
				Name:     testCSIVolume,
				Location: testLocation,
				Tier:     defaultTier,
				Network: file.Network{
					Name:        defaultNetwork,
					ConnectMode: directPeering,
				},
				Volume: file.Volume{
					Name:      newInstanceVolume,
					SizeBytes: testBytes,
				},
			},
		},
		{
			name: "custom params, non-nil topology requirement",
			toporeq: &csi.TopologyRequirement{
				Requisite: []*csi.Topology{
					{
						Segments: map[string]string{
							TopologyKeyZone: "foo-location",
						},
					},
				},
				Preferred: []*csi.Topology{
					{
						Segments: map[string]string{
							TopologyKeyZone: "foo-location",
						},
					},
				},
			},
			params: map[string]string{
				paramTier:                       "foo-tier",
				paramNetwork:                    "foo-network",
				"csiProvisionerSecretName":      "foo-secret",
				"csiProvisionerSecretNamespace": "foo-namespace",
			},
			instance: &file.ServiceInstance{
				Project:  testProject,
				Name:     testCSIVolume,
				Location: "foo-location",
				Tier:     "foo-tier",
				Network: file.Network{
					Name:        "foo-network",
					ConnectMode: directPeering,
				},
				Volume: file.Volume{
					Name:      newInstanceVolume,
					SizeBytes: testBytes,
				},
			},
		},
		{
			name: "custom params, preferred topology requirement",
			toporeq: &csi.TopologyRequirement{
				Requisite: []*csi.Topology{
					{
						Segments: map[string]string{
							TopologyKeyZone: "foo-location",
						},
					},
					{
						Segments: map[string]string{
							TopologyKeyZone: "bar-location",
						},
					},
				},
				Preferred: []*csi.Topology{
					{
						Segments: map[string]string{
							TopologyKeyZone: "bar-location",
						},
					},
				},
			},
			params: map[string]string{
				paramTier:                       "foo-tier",
				paramNetwork:                    "foo-network",
				"csiProvisionerSecretName":      "foo-secret",
				"csiProvisionerSecretNamespace": "foo-namespace",
			},
			instance: &file.ServiceInstance{
				Project:  testProject,
				Name:     testCSIVolume,
				Location: "bar-location",
				Tier:     "foo-tier",
				Network: file.Network{
					Name:        "foo-network",
					ConnectMode: directPeering,
				},
				Volume: file.Volume{
					Name:      newInstanceVolume,
					SizeBytes: testBytes,
				},
			},
		},
		{
			name: "custom params, requisite topology requirement only",
			toporeq: &csi.TopologyRequirement{
				Requisite: []*csi.Topology{
					{
						Segments: map[string]string{
							TopologyKeyZone: "foo-location",
						},
					},
				},
			},
			params: map[string]string{
				paramTier:                       "foo-tier",
				paramNetwork:                    "foo-network",
				"csiProvisionerSecretName":      "foo-secret",
				"csiProvisionerSecretNamespace": "foo-namespace",
			},
			instance: &file.ServiceInstance{
				Project:  testProject,
				Name:     testCSIVolume,
				Location: "foo-location",
				Tier:     "foo-tier",
				Network: file.Network{
					Name:        "foo-network",
					ConnectMode: directPeering,
				},
				Volume: file.Volume{
					Name:      newInstanceVolume,
					SizeBytes: testBytes,
				},
			},
		},
		{
			name: "custom params, private connect mode",
			toporeq: &csi.TopologyRequirement{
				Requisite: []*csi.Topology{
					{
						Segments: map[string]string{
							TopologyKeyZone: "foo-location",
						},
					},
				},
				Preferred: []*csi.Topology{
					{
						Segments: map[string]string{
							TopologyKeyZone: "foo-location",
						},
					},
				},
			},
			params: map[string]string{
				paramTier:                       "foo-tier",
				paramNetwork:                    "foo-network",
				ParamConnectMode:                privateServiceAccess,
				"csiProvisionerSecretName":      "foo-secret",
				"csiProvisionerSecretNamespace": "foo-namespace",
			},
			instance: &file.ServiceInstance{
				Project:  testProject,
				Name:     testCSIVolume,
				Location: "foo-location",
				Tier:     "foo-tier",
				Network: file.Network{
					Name:        "foo-network",
					ConnectMode: privateServiceAccess,
				},
				Volume: file.Volume{
					Name:      newInstanceVolume,
					SizeBytes: testBytes,
				},
			},
		},
		{
			name: "custom params, customer kms key",
			params: map[string]string{
				paramTier:                       enterpriseTier,
				ParamInstanceEncryptionKmsKey:   "foo-key",
				"csiProvisionerSecretName":      "foo-secret",
				"csiProvisionerSecretNamespace": "foo-namespace",
			},
			instance: &file.ServiceInstance{
				Project:  testProject,
				Name:     testCSIVolume,
				Location: testRegion,
				Tier:     enterpriseTier,
				Network: file.Network{
					Name:        defaultNetwork,
					ConnectMode: directPeering,
				},
				Volume: file.Volume{
					Name:      newInstanceVolume,
					SizeBytes: testBytes,
				},
				KmsKeyName: "foo-key",
			},
		},
		{
			// not going to error here, instead, pushing the decision to the Filestore API
			name: "non-enterprise tier, customer kms key",
			params: map[string]string{
				paramTier:                       basicHDDTier,
				ParamInstanceEncryptionKmsKey:   "foo-key",
				"csiProvisionerSecretName":      "foo-secret",
				"csiProvisionerSecretNamespace": "foo-namespace",
			},
			instance: &file.ServiceInstance{
				Project:  testProject,
				Name:     testCSIVolume,
				Location: testLocation,
				Tier:     basicHDDTier,
				Network: file.Network{
					Name:        defaultNetwork,
					ConnectMode: directPeering,
				},
				Volume: file.Volume{
					Name:      newInstanceVolume,
					SizeBytes: testBytes,
				},
				KmsKeyName: "foo-key",
			},
		},
		{
			name: "invalid params",
			params: map[string]string{
				"foo-param": "bar",
			},
			expectErr: true,
		},
		{
			name: "invalid connect mode",
			params: map[string]string{
				ParamConnectMode: "CONNECT_MODE_UNSPECIFIED",
			},
			expectErr: true,
		},
	}

	for _, test := range cases {
		cs := initTestController(t)
		internalServer, ok := cs.(*controllerServer)
		if !ok {
			t.Fatalf("couldn't get internal controller")
		}

		filer, err := internalServer.generateNewFileInstance(testCSIVolume, testBytes, test.params, test.toporeq)
		if !test.expectErr && err != nil {
			t.Errorf("test %q failed: %v", test.name, err)
		}
		if test.expectErr && err == nil {
			t.Errorf("test %q failed; got success", test.name)
		}
		if !reflect.DeepEqual(filer, test.instance) {
			t.Errorf("test %q failed: got filer %+v, expected %+v", test.name, filer, test.instance)
		}
	}
}

func TestGetZoneFromSegment(t *testing.T) {
	cases := []struct {
		name         string
		seg          map[string]string
		expectErr    bool
		expectedZone string
	}{
		// Error cases
		{
			name:      "Empty segment map",
			seg:       make(map[string]string),
			expectErr: true,
		},
		{
			name: "Missing zone key in segment map",
			seg: map[string]string{
				"zonekey": "z1",
			},
			expectErr: true,
		},
		{
			name: "Unknown zone key in segment map",
			seg: map[string]string{
				TopologyKeyZone: "z1",
				"unknown_key":   "z2",
			},
			expectErr: true,
		},
		// Successful cases
		{
			name: "Found expected zone",
			seg: map[string]string{
				TopologyKeyZone: "z1",
			},
			expectedZone: "z1",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			z, err := getZoneFromSegment(tc.seg)
			if tc.expectErr && err == nil {
				t.Errorf("Expected error, got none")
			}

			if !tc.expectErr && err != nil {
				t.Errorf("Unexpected error %v", err)
			}

			if z != tc.expectedZone {
				t.Errorf("Unexpected zone %v, expected zone %v", z, tc.expectedZone)
			}
		})
	}
}

func TestGetZonesFromTopology(t *testing.T) {
	cases := []struct {
		name          string
		topo          []*csi.Topology
		expectErr     bool
		expectedZones []string
	}{
		// Error cases
		{
			name:          "nil topology list",
			topo:          nil,
			expectedZones: make([]string, 0),
		},
		{
			name:          "Empty topology list",
			topo:          make([]*csi.Topology, 0),
			expectedZones: make([]string, 0),
		},
		{
			name:      "Non-Empty topology list with missing segment",
			topo:      make([]*csi.Topology, 1),
			expectErr: true,
		},
		{
			name: "Non-Empty topology list with segment missing zone key",
			topo: []*csi.Topology{
				{
					Segments: map[string]string{},
				},
			},
			expectErr: true,
		},
		{
			name: "Non-Empty topology list with segment unknown zone key",
			topo: []*csi.Topology{
				{
					Segments: map[string]string{
						"unknown_zone_key": "z1",
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Non-Empty topology list with empty segment map",
			topo: []*csi.Topology{
				{
					Segments: make(map[string]string),
				},
			},
			expectErr: true,
		},
		// two elements, one with error.
		{
			name: "Non-Empty topology list with error in one of the elements",
			topo: []*csi.Topology{
				{
					Segments: map[string]string{
						"unknown_key": "z1",
					},
				},
				{
					Segments: map[string]string{
						TopologyKeyZone: "z2",
					},
				},
			},
			expectErr: true,
		},
		// Success cases
		{
			name: "Non-Empty topology list with valid segment",
			topo: []*csi.Topology{
				{
					Segments: map[string]string{
						TopologyKeyZone: "z1",
					},
				},
			},
			expectedZones: []string{"z1"},
		},
		{
			name: "Non-Empty topology list with multiple zones",
			topo: []*csi.Topology{
				{
					Segments: map[string]string{
						TopologyKeyZone: "z1",
					},
				},
				{
					Segments: map[string]string{
						TopologyKeyZone: "z2",
					},
				},
			},
			expectedZones: []string{"z1", "z2"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			z, err := getZonesFromTopology(tc.topo)
			if tc.expectErr && err == nil {
				t.Errorf("Expected error, got none")
			}

			if !tc.expectErr && err != nil {
				t.Errorf("Unexpected error %v", err)
			}

			if !sets.NewString(z...).Equal(sets.NewString(tc.expectedZones...)) {
				t.Errorf("Unexpected zone list %v, expected zone list %v", z, tc.expectedZones)
			}
		})
	}
}

type RequestConfig struct {
	CreateVolReq  *csi.CreateVolumeRequest
	DeleteVolReq  *csi.DeleteVolumeRequest
	CreateSnapReq *csi.CreateSnapshotRequest
	DeleteSnapReq *csi.DeleteSnapshotRequest
	ExpandVolReq  *csi.ControllerExpandVolumeRequest
}

func TestVolumeOperationLocks(t *testing.T) {
	// A channel of size 1 is sufficient, because the caller of runRequest() in below steps immediately blocks and retrieves the channel of empty struct from 'operationUnblocker' channel. The test steps are such that, atmost one function pushes items on the 'operationUnblocker' channel, to indicate that the function is blocked and waiting for a signal to proceed futher in the execution.
	operationUnblocker := make(chan chan struct{}, 1)
	cs := initBlockingTestController(t, operationUnblocker)
	runRequest := func(req *RequestConfig) <-chan error {
		resp := make(chan error)
		go func() {
			var err error
			if req.CreateVolReq != nil {
				_, err = cs.CreateVolume(context.Background(), req.CreateVolReq)
			} else if req.DeleteVolReq != nil {
				_, err = cs.DeleteVolume(context.Background(), req.DeleteVolReq)
			} else if req.CreateSnapReq != nil {
				_, err = cs.CreateSnapshot(context.Background(), req.CreateSnapReq)
			} else if req.DeleteSnapReq != nil {
				_, err = cs.DeleteSnapshot(context.Background(), req.DeleteSnapReq)
			} else if req.ExpandVolReq != nil {
				_, err = cs.ControllerExpandVolume(context.Background(), req.ExpandVolReq)
			}
			resp <- err
		}()
		return resp
	}

	req := &csi.CreateVolumeRequest{
		Name: testCSIVolume,
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
	}
	// Block first CreateVolume request after it has acquired the lock.
	resp := runRequest(&RequestConfig{CreateVolReq: req})
	createOpUnblocker := <-operationUnblocker

	// Second CreateVolume request on the same volume should fail to acquire lock and return Aborted error.
	createResp2 := runRequest(&RequestConfig{CreateVolReq: req})
	ValidateExpectedError(t, createResp2, operationUnblocker, codes.Aborted)

	// Delete Volume request on the same volume should fail to acquire lock and return Aborted error.
	delResp := runRequest(&RequestConfig{DeleteVolReq: &csi.DeleteVolumeRequest{
		VolumeId: testVolumeID,
	}})
	ValidateExpectedError(t, delResp, operationUnblocker, codes.Aborted)

	// Create a snapshot on the same volume should fail to acquire lock and return Aborted error.
	createSnapResp := runRequest(&RequestConfig{
		CreateSnapReq: &csi.CreateSnapshotRequest{
			Name:           "test-snap",
			SourceVolumeId: testVolumeID,
		},
	})
	ValidateExpectedError(t, createSnapResp, operationUnblocker, codes.Aborted)

	// ControllerExapnd request on the same volume should fail to acquire lock and return Aborted error.
	expandVolResp := runRequest(&RequestConfig{
		ExpandVolReq: &csi.ControllerExpandVolumeRequest{
			VolumeId: testVolumeID,
		},
	})
	ValidateExpectedError(t, expandVolResp, operationUnblocker, codes.Aborted)

	// Send a create volume request for a different volume. This is expected to succeed.
	vol2CreateVolResp := runRequest(&RequestConfig{CreateVolReq: &csi.CreateVolumeRequest{
		Name: testCSIVolume2,
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
	}})
	execVol2CreateVol := <-operationUnblocker
	execVol2CreateVol <- struct{}{}
	if err := <-vol2CreateVolResp; err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Unblock first CreateVolume request and let it run to completion.
	createOpUnblocker <- struct{}{}
	if err := <-resp; err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	// Delete the first volume, no error expected.
	delResp = runRequest(&RequestConfig{DeleteVolReq: &csi.DeleteVolumeRequest{
		VolumeId: testVolumeID,
	}})
	execDelVol := <-operationUnblocker
	execDelVol <- struct{}{}
	if err := <-delResp; err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func ValidateExpectedError(t *testing.T, errResp <-chan error, operationUnblocker chan chan struct{}, expectedErrorCode codes.Code) {
	select {
	case err := <-errResp:
		if err != nil {
			serverError, ok := status.FromError(err)
			if !ok {
				t.Fatalf("Could not get error status code from err: %v", err)
			}
			if serverError.Code() != codes.Aborted {
				t.Errorf("Expected error code: %v, got: %v. err : %v", codes.Aborted, serverError.Code(), err)
			}
		} else {
			t.Errorf("Expected error: %v, got no error", codes.Aborted)
		}
	case <-operationUnblocker:
		t.Errorf("The operation should have been aborted, but was started")
	}
}

func TestCreateSnapshot(t *testing.T) {
	type BackupTestInfo struct {
		backup *file.BackupInfo
		state  string
	}
	backupName := "mybackup"
	project := "test-project"
	zone := "us-central1-c"
	region := "us-central1"
	instanceName := "myinstance"
	shareName := "myshare"
	defaultBackupUri := fmt.Sprintf("projects/%s/locations/%s/backups/%s", project, region, backupName)
	cases := []struct {
		name          string
		req           *csi.CreateSnapshotRequest
		resp          *csi.CreateSnapshotResponse
		initialBackup *BackupTestInfo
		expectErr     bool
	}{
		// Failure test cases
		{
			name: "Existing backup found, with different volume Id (source zonal filestore instance), error expected",
			req: &csi.CreateSnapshotRequest{
				SourceVolumeId: modeInstance + "/" + zone + "/" + "myinstance1" + "/" + shareName,
				Name:           backupName,
				Parameters: map[string]string{
					util.VolumeSnapshotTypeKey: "backup",
				},
			},
			initialBackup: &BackupTestInfo{
				backup: &file.BackupInfo{
					Project:            project,
					Location:           region,
					SourceInstanceName: instanceName,
					SourceShare:        shareName,
					Name:               backupName,
					BackupURI:          defaultBackupUri,
					SourceVolumeId:     modeInstance + "/" + zone + "/" + instanceName + "/" + shareName,
				},
			},
			expectErr: true,
		},
		{
			name: "Existing backup found in state CREATING",
			req: &csi.CreateSnapshotRequest{
				SourceVolumeId: "modeInstance/us-central1/myinstance/myshare",
				Name:           backupName,
				Parameters: map[string]string{
					util.VolumeSnapshotTypeKey: "backup",
				},
			},
			initialBackup: &BackupTestInfo{
				backup: &file.BackupInfo{
					Project:            project,
					Location:           region,
					SourceInstanceName: instanceName,
					SourceShare:        shareName,
					Name:               backupName,
					BackupURI:          defaultBackupUri,
					SourceVolumeId:     "modeInstance/us-central1/myinstance/myshare",
				},
				state: "CREATING",
			},
			expectErr: true,
		},
		// Success test cases
		{
			name: "No backup found",
			req: &csi.CreateSnapshotRequest{
				SourceVolumeId: "modeInstance/us-central1/myinstance/myshare",
				Name:           backupName,
				Parameters: map[string]string{
					util.VolumeSnapshotTypeKey: "backup",
				},
			},
			resp: &csi.CreateSnapshotResponse{
				Snapshot: &csi.Snapshot{
					SizeBytes:      1 * util.Tb,
					SnapshotId:     defaultBackupUri,
					SourceVolumeId: "modeInstance/us-central1/myinstance/myshare",
					ReadyToUse:     true,
				},
			},
			initialBackup: nil,
		},
		{
			name: "No backup found, zonal source",
			req: &csi.CreateSnapshotRequest{
				SourceVolumeId: "modeInstance/us-central1-c/myinstance/myshare",
				Name:           backupName,
				Parameters: map[string]string{
					util.VolumeSnapshotTypeKey: "backup",
				},
			},
			resp: &csi.CreateSnapshotResponse{
				Snapshot: &csi.Snapshot{
					SizeBytes:      1 * util.Tb,
					SnapshotId:     defaultBackupUri,
					SourceVolumeId: "modeInstance/us-central1-c/myinstance/myshare",
					ReadyToUse:     true,
				},
			},
			initialBackup: nil,
		},
		{
			name: "No backup found, cross regional snapshot",
			req: &csi.CreateSnapshotRequest{
				SourceVolumeId: "modeInstance/us-central1-c/myinstance/myshare",
				Name:           backupName,
				Parameters: map[string]string{
					util.VolumeSnapshotTypeKey:     "backup",
					util.VolumeSnapshotLocationKey: "us-west1",
				},
			},
			resp: &csi.CreateSnapshotResponse{
				Snapshot: &csi.Snapshot{
					SizeBytes:      1 * util.Tb,
					SnapshotId:     fmt.Sprintf("projects/%s/locations/%s/backups/%s", project, "us-west1", backupName),
					SourceVolumeId: "modeInstance/us-central1-c/myinstance/myshare",
					ReadyToUse:     true,
				},
			},
			initialBackup: nil,
		},
		{
			name: "Existing backup found, with same source volume Id (source regional filestore instance)",
			req: &csi.CreateSnapshotRequest{
				SourceVolumeId: "modeInstance/us-central1/myinstance/myshare",
				Name:           backupName,
				Parameters: map[string]string{
					util.VolumeSnapshotTypeKey: "backup",
				},
			},
			initialBackup: &BackupTestInfo{
				backup: &file.BackupInfo{
					Project:            project,
					Location:           region,
					SourceInstanceName: instanceName,
					SourceShare:        shareName,
					Name:               backupName,
					BackupURI:          defaultBackupUri,
					SourceVolumeId:     "modeInstance/us-central1/myinstance/myshare",
				},
			},
		},
		{
			name: "Existing backup found, with same source volume Id (source zonal filestore instance)",
			req: &csi.CreateSnapshotRequest{
				SourceVolumeId: "modeInstance/us-central1-c/myinstance/myshare",
				Name:           backupName,
				Parameters: map[string]string{
					util.VolumeSnapshotTypeKey: "backup",
				},
			},
			initialBackup: &BackupTestInfo{
				backup: &file.BackupInfo{
					Project:            project,
					Location:           region,
					SourceInstanceName: instanceName,
					SourceShare:        shareName,
					Name:               backupName,
					BackupURI:          defaultBackupUri,
					SourceVolumeId:     "modeInstance/us-central1-c/myinstance/myshare",
				},
			},
		},
	}
	for _, test := range cases {
		fileService, err := file.NewFakeService()
		if err != nil {
			t.Fatalf("failed to initialize GCFS service: %v", err)
		}

		cloudProvider, err := cloud.NewFakeCloud()
		if err != nil {
			t.Fatalf("Failed to get cloud provider: %v", err)
		}
		cs := newControllerServer(&controllerServerConfig{
			driver:      initTestDriver(t),
			fileService: fileService,
			cloud:       cloudProvider,
			volumeLocks: util.NewVolumeLocks(),
		})

		if test.initialBackup != nil {
			existingBackup, err := fileService.CreateBackup(context.TODO(), test.initialBackup.backup)
			if err != nil {
				t.Errorf("test %q failed to create snapshot: %v", test.name, err)
			}
			if test.initialBackup.state != "" {
				klog.Infof("existingBackup looks like: %+v", existingBackup)

				existingBackup.State = test.initialBackup.state
			}
		}
		resp, err := cs.CreateSnapshot(context.TODO(), test.req)
		if !test.expectErr && err != nil {
			t.Errorf("test %q failed: %v", test.name, err)
		}
		if test.expectErr && err == nil {
			t.Errorf("test %q failed; got success", test.name)
		}
		if test.resp != nil {
			if resp.Snapshot.SizeBytes != test.resp.Snapshot.SizeBytes {
				t.Errorf("test %q failed, %v, mismatch, got %v, want %v", test.name, "SizeBytes", resp.Snapshot.SizeBytes, test.resp.Snapshot.SizeBytes)
			}
			if resp.Snapshot.SnapshotId != test.resp.Snapshot.SnapshotId {
				t.Errorf("test %q failed, %v, mismatch, got %v, want %v", test.name, "SnapshotId", resp.Snapshot.SnapshotId, test.resp.Snapshot.SnapshotId)
			}
			if resp.Snapshot.SourceVolumeId != test.resp.Snapshot.SourceVolumeId {
				t.Errorf("test %q failed, %v, mismatch, got %v, want %v", test.name, "SourceVolumeId", resp.Snapshot.SourceVolumeId, test.resp.Snapshot.SourceVolumeId)
			}
			if resp.Snapshot.ReadyToUse != test.resp.Snapshot.ReadyToUse {
				t.Errorf("test %q failed, %v, mismatch, got %v, want %v", test.name, "ReadyToUse", resp.Snapshot.ReadyToUse, test.resp.Snapshot.ReadyToUse)
			}
		}

		if !test.expectErr && test.initialBackup == nil {
			backup, _ := fileService.GetBackup(context.TODO(), resp.Snapshot.SnapshotId)
			if backup.Backup.Labels[tagKeyCreatedBy] != "test-driver" {
				t.Errorf("labels check for %v failed on test %q, got %v, want %v", tagKeyCreatedBy, test.name, backup.Backup.Labels[tagKeyCreatedBy], "test-driver")
			}
			if backup.Backup.Labels[tagKeySnapshotName] != test.req.Name {
				t.Errorf("labels check for %v failed on test %q, got %v, want %v", tagKeySnapshotName, test.name, backup.Backup.Labels[tagKeySnapshotName], test.req.Name)
			}
		}
	}
}

func TestCreateBackupURI(t *testing.T) {
	backupName := "mybackup"
	project := "test-project"
	region := "us-central1"
	cases := []struct {
		name            string
		backupName      string
		backupLocation  string
		serviceLocation string
		project         string
		expectedURL     string
		expectedRegion  string
		expectErr       bool
	}{
		//Failure cases
		{
			name:            "backupLocation is zone instead of region. Expect error",
			backupName:      backupName,
			backupLocation:  "us-west1-c",
			serviceLocation: "us-west1-c",
			project:         project,
			expectedURL:     "",
			expectedRegion:  "",
			expectErr:       true,
		},
		{
			name:            "Invalid location in ServiceInstance. Expect error",
			backupName:      backupName,
			backupLocation:  "",
			serviceLocation: "us-west1-c-b-d",
			project:         project,
			expectedURL:     "",
			expectedRegion:  "",
			expectErr:       true,
		},
		{
			name:            "Region is not provided. ServiceInstance is regional.",
			backupName:      backupName,
			backupLocation:  "",
			serviceLocation: "us-west1",
			project:         project,
			expectedURL:     "projects/test-project/locations/us-west1/backups/mybackup",
			expectedRegion:  "us-west1",
			expectErr:       false,
		},
		{
			name:            "Region is not provided. ServiceInstance is zonal.",
			backupName:      backupName,
			backupLocation:  "",
			serviceLocation: "us-west1-c",
			project:         project,
			expectedURL:     "projects/test-project/locations/us-west1/backups/mybackup",
			expectedRegion:  "us-west1",
			expectErr:       false,
		},
		{
			name:            "Region is provided and is different from ServiceInstance. Take region",
			backupName:      backupName,
			backupLocation:  region,
			serviceLocation: "us-west1-c",
			project:         project,
			expectedURL:     "projects/test-project/locations/us-central1/backups/mybackup",
			expectedRegion:  "us-central1",
			expectErr:       false,
		},
	}
	for _, test := range cases {
		returnedURL, returnedRegion, err := file.CreateBackupURI(test.serviceLocation, test.project, test.backupName, test.backupLocation)
		if !test.expectErr && err != nil {
			t.Errorf("test %q failed: %v", test.name, err)
		}
		if test.expectErr && err == nil {
			t.Errorf("test %q failed; got success", test.name)
		}
		if returnedURL != test.expectedURL {
			t.Errorf("test %q failed: got %v, want %v", test.name, returnedURL, test.expectedURL)
		}
		if returnedRegion != test.expectedRegion {
			t.Errorf("test %q failed: got %v, want %v", test.name, returnedRegion, test.expectedRegion)
		}
	}
}

func TestGetCloudInstancesReservedIPRanges(t *testing.T) {
	cases := []struct {
		name                       string
		initMultishareInstanceList []*file.MultishareInstance
		instance                   *file.ServiceInstance
		expectIPRange              map[string]bool
		expectErr                  bool
	}{
		{
			name: "existing instances in different vpc networks",
			initMultishareInstanceList: []*file.MultishareInstance{
				{
					Name:     "test-instance",
					Project:  testProject,
					Location: "us-west1",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
					},
					Network: file.Network{
						ReservedIpRange: "10.0.0.0/24",
						ConnectMode:     directPeering,
						Name:            testVPCNetwork,
						Ip:              "10.0.0.2",
					},
					Tier: enterpriseTier,
				},
				{
					Name:     "test-instance-1",
					Project:  testProject,
					Location: "us-west1",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
					},
					Network: file.Network{
						ReservedIpRange: "10.1.1.0/24",
						ConnectMode:     directPeering,
						Name:            defaultNetwork,
						Ip:              "10.1.1.2",
					},
					Tier: enterpriseTier,
				},
			},
			instance: &file.ServiceInstance{
				Project:  testProject,
				Name:     testCSIVolume,
				Location: testLocation,
				Tier:     defaultTier,
				Network: file.Network{
					Name:        defaultNetwork,
					ConnectMode: directPeering,
				},
			},
			expectIPRange: map[string]bool{"192.168.92.32/29": true, "192.168.92.40/29": true, "10.1.1.0/24": true},
		},
	}
	for _, test := range cases {
		cs := initTestController(t).(*controllerServer)
		for _, i := range test.initMultishareInstanceList {
			cs.config.fileService.StartCreateMultishareInstanceOp(context.Background(), i)
		}
		ipRange, err := cs.getCloudInstancesReservedIPRanges(context.Background(), test.instance)
		if !test.expectErr && err != nil {
			t.Errorf("test %q failed: %v", test.name, err)
		}
		if !reflect.DeepEqual(test.expectIPRange, ipRange) {
			t.Errorf("test %q failed; expected: %#v; got %#v", test.name, test.expectIPRange, ipRange)
		}
	}
}
