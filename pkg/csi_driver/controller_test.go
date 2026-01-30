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
	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
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
	testReservedIPV4CIDR   = "10.0.0.0/22"
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
		tagManager:  cloud.NewFakeTagManager(),
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
		tagManager:  cloud.NewFakeTagManager(),
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
	features := &GCFSDriverFeatureOptions{
		FeatureNFSExportOptionsOnCreate: &FeatureNFSExportOptionsOnCreate{
			Enabled: true,
		},
		FeatureLockRelease: &FeatureLockRelease{},
		FeatureNFSv4Support: &FeatureNFSv4Support{
			Enabled: true,
		},
	}

	cases := []struct {
		name            string
		req             *csi.CreateVolumeRequest
		resp            *csi.CreateVolumeResponse
		initialBackup   *BackupInfo
		expectedOptions []*file.NfsExportOptions
		expectErr       bool
		features        *GCFSDriverFeatureOptions
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
				Parameters: map[string]string{
					"tier":             defaultTier,
					ParameterKeyLabels: "key1=value1",
				},
				VolumeCapabilities: volumeCapabilities,
			},
			resp: &csi.CreateVolumeResponse{
				Volume: &csi.Volume{
					CapacityBytes: defaultTierMinSize,
					VolumeId:      testVolumeID,
					VolumeContext: map[string]string{
						attrIP:           testIP,
						attrVolume:       newInstanceVolume,
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
						attrIP:           testIP,
						attrVolume:       newInstanceVolume,
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
				Parameters:         map[string]string{"tier": enterpriseTier, paramFileProtocol: v4_1FileProtocol},
				VolumeCapabilities: volumeCapabilities,
			},
			features: features,
			resp: &csi.CreateVolumeResponse{
				Volume: &csi.Volume{
					CapacityBytes: testBytes,
					VolumeId:      testVolumeID,
					VolumeContext: map[string]string{
						attrIP:           testIP,
						attrVolume:       newInstanceVolume,
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
		{
			name: "from enterprise tier snapshot and nfsExportOptions set",
			req: &csi.CreateVolumeRequest{
				Name: testCSIVolume,
				VolumeContentSource: &csi.VolumeContentSource{
					Type: &csi.VolumeContentSource_Snapshot{
						Snapshot: &csi.VolumeContentSource_SnapshotSource{
							SnapshotId: "projects/test-project/locations/us-central1/backups/mybackup",
						},
					},
				},
				Parameters: map[string]string{
					"tier": enterpriseTier,
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
				VolumeCapabilities: volumeCapabilities,
			},
			features: features,
			resp: &csi.CreateVolumeResponse{
				Volume: &csi.Volume{
					CapacityBytes: testBytes,
					VolumeId:      testVolumeID,
					VolumeContext: map[string]string{
						attrIP:           testIP,
						attrVolume:       newInstanceVolume,
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
		{
			name: "Parameters contain misconfigured labels(invalid KV separator(:) used)",
			req: &csi.CreateVolumeRequest{
				Name: testCSIVolume,
				Parameters: map[string]string{
					"tier":             enterpriseTier,
					ParameterKeyLabels: "key1:value1",
				},
			},
			resp:            nil,
			expectedOptions: nil,
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
			expectErr: true,
		},
	}

	for _, test := range cases {
		cs := initTestController(t).(*controllerServer)
		if test.features != nil {
			cs.config.features = test.features
		}

		cs.config.tagManager.(*cloud.FakeTagServiceManager).
			On("AttachResourceTags", context.TODO(), cloud.FilestoreInstance, testCSIVolume, testLocation, test.req.GetName(), test.req.GetParameters()).
			Return(nil)

		//Create initial backup
		backupInfo := &file.BackupInfo{
			Project:            test.initialBackup.s.Project,
			Location:           test.initialBackup.backupLocation,
			SourceInstanceName: test.initialBackup.s.Name,
			SourceShare:        test.initialBackup.s.Volume.Name,
			Name:               test.initialBackup.backupName,
			SourceVolumeId:     test.initialBackup.SourceVolumeId,
			Labels:             make(map[string]string),
		}
		if test.resp != nil {
			backupInfo.BackupURI = test.resp.Volume.ContentSource.GetSnapshot().SnapshotId
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
		if !test.expectErr && test.req.Parameters[ParamNfsExportOptions] != "" {
			instance, err := cs.config.fileService.GetInstance(context.TODO(), &file.ServiceInstance{Name: test.req.Name})
			if err != nil {
				t.Errorf("test %q failed: couldn't get instance %v: %v", test.name, resp.Volume.VolumeId, err)
				return
			}
			for i := range test.expectedOptions {
				if !reflect.DeepEqual(instance.NfsExportOptions[i], test.expectedOptions[i]) {
					t.Errorf("test %q failed; nfs export options not equal at index %d: got %+v, expected %+v", test.name, i, instance.NfsExportOptions[i], test.expectedOptions[i])
				}
			}
		}
		if !cmp.Equal(resp, test.resp, protocmp.Transform()) {
			t.Errorf("test %q failed: got resp %+v, expected %+v, diff: %s", test.name, resp, test.resp, cmp.Diff(resp, test.resp, protocmp.Transform()))
		}
	}
}

func TestCreateVolume(t *testing.T) {
	features := &GCFSDriverFeatureOptions{
		FeatureNFSExportOptionsOnCreate: &FeatureNFSExportOptionsOnCreate{
			Enabled: true,
		},
		FeatureLockRelease: &FeatureLockRelease{},
		FeatureNFSv4Support: &FeatureNFSv4Support{
			Enabled: true,
		},
	}
	cases := []struct {
		name            string
		req             *csi.CreateVolumeRequest
		resp            *csi.CreateVolumeResponse
		expectErr       bool
		features        *GCFSDriverFeatureOptions
		expectedOptions []*file.NfsExportOptions
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
				Parameters: map[string]string{
					"tier":     zonalTier,
					"protocol": v4_1FileProtocol,
				},
			},
			resp: &csi.CreateVolumeResponse{
				Volume: &csi.Volume{
					CapacityBytes: 1 * util.Tb,
					VolumeId:      testVolumeID,
					VolumeContext: map[string]string{
						attrIP:           testIP,
						attrVolume:       newInstanceVolume,
						attrFileProtocol: v4_1FileProtocol,
					},
				},
			},
			features: features,
		},
		{
			name: "create volume without providing protocol for basic",
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
					"tier": basicHDDTier,
				},
			},
			resp: &csi.CreateVolumeResponse{
				Volume: &csi.Volume{
					CapacityBytes: 100 * util.Gb,
					VolumeId:      testVolumeID,
					VolumeContext: map[string]string{
						attrIP:           testIP,
						attrVolume:       newInstanceVolume,
						attrFileProtocol: v3FileProtocol,
					},
				},
			},
			features: features,
		},
		// Failure Scenarios
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
		{
			name: "adding tags to filestore instance fails(failure scenario mocked)",
			req: &csi.CreateVolumeRequest{
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
				Parameters: map[string]string{
					"network":                      "test",
					cloud.ParameterKeyResourceTags: "kubernetes/test1/test1",
				},
			},
			expectErr: true,
		},
		// TODO: create failed
		// TODO: instance already exists error
		// TODO: instance already exists invalid
		// TODO: instance already exists valid
		{
			name: "nfsExportOptions feature is disabled",
			req: &csi.CreateVolumeRequest{
				Name: testCSIVolume,
				VolumeContentSource: &csi.VolumeContentSource{
					Type: &csi.VolumeContentSource_Snapshot{
						Snapshot: &csi.VolumeContentSource_SnapshotSource{
							SnapshotId: "projects/test-project/locations/us-central1/backups/mybackup",
						},
					},
				},
				Parameters: map[string]string{
					"tier": enterpriseTier,
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
			features: &GCFSDriverFeatureOptions{
				FeatureNFSExportOptionsOnCreate: &FeatureNFSExportOptionsOnCreate{
					Enabled: false,
				},
				FeatureLockRelease: &FeatureLockRelease{},
			},
			expectErr: true,
		},
		// Success Scenarios
		{
			name: "adding nfs-export-options",
			req: &csi.CreateVolumeRequest{
				Name: testCSIVolume,
				Parameters: map[string]string{
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
			features: features,
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
			resp: &csi.CreateVolumeResponse{
				Volume: &csi.Volume{
					CapacityBytes: 100 * util.Gb,
					VolumeId:      testVolumeID,
					VolumeContext: map[string]string{
						attrIP:           testIP,
						attrVolume:       newInstanceVolume,
						attrFileProtocol: v3FileProtocol,
					},
				},
			},
		},
		{
			name: "create volume with mount options",
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
					"tier":          zonalTier,
					"protocol":      v4_1FileProtocol,
					"mount-options": "noatime,nodiratime",
				},
			},
			resp: &csi.CreateVolumeResponse{
				Volume: &csi.Volume{
					CapacityBytes: 1 * util.Tb,
					VolumeId:      testVolumeID,
					VolumeContext: map[string]string{
						attrIP:           testIP,
						attrVolume:       newInstanceVolume,
						attrFileProtocol: v4_1FileProtocol,
						attrMountOptions: "noatime,nodiratime",
					},
				},
			},
			features: features,
		},
		{
			name: "create volume with regional tier",
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
					"tier":               regionalTier,
					"reserved-ipv4-cidr": testReservedIPV4CIDR,
				},
			},
			resp: &csi.CreateVolumeResponse{
				Volume: &csi.Volume{
					CapacityBytes: 100 * util.Gb,
					VolumeId:      testVolumeID,
					VolumeContext: map[string]string{
						attrIP:           testIP,
						attrVolume:       newInstanceVolume,
						attrFileProtocol: v3FileProtocol,
					},
				},
			},
			features: features,
		},
		{
			name: "create volume with valid performance config",
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
					"tier": zonalTier,
				},
				MutableParameters: map[string]string{
					ParamMaxIOPS: "3000",
				},
			},
			resp: &csi.CreateVolumeResponse{
				Volume: &csi.Volume{
					CapacityBytes: 1 * util.Tb,
					VolumeId:      testVolumeID,
					VolumeContext: map[string]string{
						attrIP:           testIP,
						attrVolume:       newInstanceVolume,
						attrFileProtocol: v3FileProtocol,
					},
				},
			},
			features: features,
		},
		{
			name: "create volume with invalid performance config",
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
					"tier": zonalTier,
				},
				MutableParameters: map[string]string{
					ParamMaxIOPS: "1999", // Below minimum of 2000
				},
			},
			expectErr: true,
			features:  features,
		},
	}

	for _, test := range cases {
		cs := initTestController(t).(*controllerServer)
		cs.config.features = test.features
		cs.config.tagManager.(*cloud.FakeTagServiceManager).
			On("AttachResourceTags", context.TODO(), cloud.FilestoreInstance, testCSIVolume, testLocation, test.req.GetName(), test.req.GetParameters()).
			Return(nil)
		cs.config.tagManager.(*cloud.FakeTagServiceManager).
			On("AttachResourceTags", context.TODO(), cloud.FilestoreInstance, testCSIVolume2, testLocation, test.req.GetName(), test.req.GetParameters()).
			Return(fmt.Errorf("mock failure: error while adding tags to filestore instance"))

		resp, err := cs.CreateVolume(context.TODO(), test.req)

		if !test.expectErr && err != nil {
			t.Errorf("test %q failed: %v", test.name, err)
		}
		if test.expectErr && err == nil {
			t.Errorf("test %q failed; got success", test.name)
		}
		if !cmp.Equal(resp, test.resp, protocmp.Transform()) {
			t.Errorf("test %q failed: got resp %+v, expected %+v, diff: %s", test.name, resp, test.resp, cmp.Diff(resp, test.resp, protocmp.Transform()))
		}

		if !test.expectErr && test.req.Parameters[ParamNfsExportOptions] != "" {
			instance, err := cs.config.fileService.GetInstance(context.TODO(), &file.ServiceInstance{Name: test.req.Name})
			if err != nil {
				t.Errorf("test %q failed: couldn't get instance %v: %v", test.name, test.req.Name, err)
				return
			}
			for i := range test.expectedOptions {
				if !reflect.DeepEqual(instance.NfsExportOptions[i], test.expectedOptions[i]) {
					t.Errorf("test %q failed; nfs export options not equal at index %d: got %+v, expected %+v", test.name, i, instance.NfsExportOptions[i], test.expectedOptions[i])
				}
			}
		}
		if !test.expectErr && test.req.Parameters[ParamReservedIPV4CIDR] != "" {
			expectedReservedIpRange := "10.0.0.0/24"
			instance, err := cs.config.fileService.GetInstance(context.TODO(), &file.ServiceInstance{Name: test.req.Name})
			if err != nil {
				t.Errorf("test %q failed: couldn't get instance %v: %v", test.name, test.req.Name, err)
				return
			}
			if instance.Network.ReservedIpRange != expectedReservedIpRange {
				t.Errorf("test %q failed; ReservedIpRange = %v; want %v", test.name, instance.Network.ReservedIpRange, expectedReservedIpRange)
			}
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

func TestControllerExpandVolume(t *testing.T) {
	type testCase struct {
		name        string
		currentSize int64
		requiredCap int64
		tier        string
		shouldError bool
	}

	cases := []testCase{
		{
			name:        "expand zonal small tier within bounds",
			currentSize: 2 * util.Tb,
			requiredCap: 5 * util.Tb,
			tier:        zonalTier,
			shouldError: false,
		},
		{
			name:        "expand zonal from small to large tier",
			currentSize: zonalSmallTierMaxSize,
			requiredCap: 11 * util.Tb,
			tier:        zonalTier,
			shouldError: true,
		},
		{
			name:        "expand zonal within large tier bounds",
			currentSize: 11 * util.Tb,
			requiredCap: 50 * util.Tb,
			tier:        zonalTier,
			shouldError: false,
		},
		{
			name:        "expand regional small tier within bounds",
			currentSize: 500 * util.Gb,
			requiredCap: 2 * util.Tb,
			tier:        regionalTier,
			shouldError: false,
		},
		{
			name:        "expand regional from 100 GB (new minimum) within bounds",
			currentSize: regionalSmallTierMinSize,
			requiredCap: 500 * util.Gb,
			tier:        regionalTier,
			shouldError: false,
		},
		{
			name:        "expand regional from small to large tier",
			currentSize: regionalSmallTierMaxSize,
			requiredCap: 11 * util.Tb,
			tier:        regionalTier,
			shouldError: true,
		},
		{
			name:        "expand regional within large tier bounds",
			currentSize: 11 * util.Tb,
			requiredCap: 50 * util.Tb,
			tier:        regionalTier,
			shouldError: false,
		},
		{
			name:        "expand regional small tier at new minimum boundary",
			currentSize: regionalSmallTierMinSize,
			requiredCap: 3 * util.Tb,
			tier:        regionalTier,
			shouldError: false,
		},
		{
			name:        "expand regional just below max small tier",
			currentSize: 9 * util.Tb,
			requiredCap: 9500 * util.Gb,
			tier:        regionalTier,
			shouldError: false,
		},
		{
			name:        "expand regional at max small tier to large tier",
			currentSize: regionalSmallTierMaxSize,
			requiredCap: regionalSmallTierMaxSize + 1,
			tier:        regionalTier,
			shouldError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Create fake file service with an instance
			fs, err := file.NewFakeService()
			if err != nil {
				t.Fatalf("failed to init fake file service: %v", err)
			}

			instanceName := "test-expand-instance"
			_, err = fs.CreateInstance(context.Background(), &file.ServiceInstance{
				Name:     instanceName,
				Location: testZone,
				Tier:     tc.tier,
				Volume:   file.Volume{Name: newInstanceVolume, SizeBytes: tc.currentSize},
			})
			if err != nil {
				t.Fatalf("failed to create instance: %v", err)
			}

			// Create controller
			cloudProvider, err := cloud.NewFakeCloud()
			if err != nil {
				t.Fatalf("failed to create cloud provider: %v", err)
			}

			ctrl := newControllerServer(&controllerServerConfig{
				driver:      initTestDriver(t),
				fileService: fs,
				cloud:       cloudProvider,
				volumeLocks: util.NewVolumeLocks(),
				features:    &GCFSDriverFeatureOptions{FeatureLockRelease: &FeatureLockRelease{}},
				tagManager:  cloud.NewFakeTagManager(),
			})

			// Create volumeID from instance
			volumeID := getVolumeIDFromFileInstance(&file.ServiceInstance{
				Name:     instanceName,
				Location: testZone,
				Volume:   file.Volume{Name: newInstanceVolume},
			}, modeInstance)

			// Call ControllerExpandVolume
			req := &csi.ControllerExpandVolumeRequest{
				VolumeId: volumeID,
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: tc.requiredCap,
				},
			}

			resp, err := ctrl.ControllerExpandVolume(context.Background(), req)

			if tc.shouldError {
				if err == nil {
					t.Errorf("Test %q failed: expected error but got success", tc.name)
				}
			} else {
				if err != nil {
					t.Errorf("Test %q failed: expected success but got error: %v", tc.name, err)
				}
				if resp == nil {
					t.Errorf("Test %q failed: expected non-nil response", tc.name)
				}
			}
		})
	}
}

func TestControllerModifyVolume_EmptyVolumeID(t *testing.T) {
	ctrl := initTestController(t)

	req := &csi.ControllerModifyVolumeRequest{VolumeId: ""}
	resp, err := ctrl.ControllerModifyVolume(context.Background(), req)

	if resp != nil {
		t.Fatalf("expected nil response, got: %v", resp)
	}
	if err == nil {
		t.Fatalf("expected error for empty volume id, got nil")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument for empty volume id, got: %v", err)
	}
}

func TestControllerModifyVolume_NoMutableParamsNoop(t *testing.T) {
	// create fake service and instance
	fs, err := file.NewFakeService()
	if err != nil {
		t.Fatalf("failed to init fake file service: %v", err)
	}
	// create an instance matching testVolumeID's instance name ("test-csi")
	_, err = fs.CreateInstance(context.Background(), &file.ServiceInstance{Name: "test-csi", Location: testZone, Tier: zonalTier, Volume: file.Volume{Name: "vol1", SizeBytes: testBytes}})
	if err != nil {
		t.Fatalf("failed to create fake instance: %v", err)
	}

	cloudProvider, err := cloud.NewFakeCloud()
	if err != nil {
		t.Fatalf("Failed to get cloud provider: %v", err)
	}

	ctrl := newControllerServer(&controllerServerConfig{
		driver:      initTestDriver(t),
		fileService: fs,
		cloud:       cloudProvider,
		volumeLocks: util.NewVolumeLocks(),
		features:    &GCFSDriverFeatureOptions{FeatureLockRelease: &FeatureLockRelease{}},
		tagManager:  cloud.NewFakeTagManager(),
	})

	req := &csi.ControllerModifyVolumeRequest{VolumeId: testVolumeID}
	resp, err := ctrl.ControllerModifyVolume(context.Background(), req)
	if err != nil {
		t.Fatalf("expected success for no-op modify, got error: %v", err)
	}
	if resp == nil {
		t.Fatalf("expected non-nil response for no-op modify")
	}
}

func TestControllerModifyVolume_InvalidPerformanceParams(t *testing.T) {
	fs, err := file.NewFakeService()
	if err != nil {
		t.Fatalf("failed to init fake file service: %v", err)
	}
	// create an instance with zonal tier so performance params are supported
	_, err = fs.CreateInstance(context.Background(), &file.ServiceInstance{Name: "test-csi", Location: testZone, Tier: zonalTier, Volume: file.Volume{Name: "vol1", SizeBytes: testBytes}})
	if err != nil {
		t.Fatalf("failed to create fake instance: %v", err)
	}

	cloudProvider, err := cloud.NewFakeCloud()
	if err != nil {
		t.Fatalf("Failed to get cloud provider: %v", err)
	}

	ctrl := newControllerServer(&controllerServerConfig{
		driver:      initTestDriver(t),
		fileService: fs,
		cloud:       cloudProvider,
		volumeLocks: util.NewVolumeLocks(),
		features:    &GCFSDriverFeatureOptions{FeatureLockRelease: &FeatureLockRelease{}},
		tagManager:  cloud.NewFakeTagManager(),
	})

	// set fixed iops below minimum (1999)
	req := &csi.ControllerModifyVolumeRequest{
		VolumeId:          testVolumeID,
		MutableParameters: map[string]string{ParamMaxIOPS: "1999"},
	}
	_, err = ctrl.ControllerModifyVolume(context.Background(), req)
	if err == nil {
		t.Fatalf("expected error for invalid performance params, got nil")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument for invalid performance params, got: %v", err)
	}
}

func TestControllerModifyVolume_UpdateSuccess(t *testing.T) {
	fs, err := file.NewFakeService()
	if err != nil {
		t.Fatalf("failed to init fake file service: %v", err)
	}
	// create an instance with zonal tier so performance params are supported
	_, err = fs.CreateInstance(context.Background(), &file.ServiceInstance{Name: "test-csi", Location: testZone, Tier: zonalTier, Volume: file.Volume{Name: "vol1", SizeBytes: testBytes}})
	if err != nil {
		t.Fatalf("failed to create fake instance: %v", err)
	}

	cloudProvider, err := cloud.NewFakeCloud()
	if err != nil {
		t.Fatalf("Failed to get cloud provider: %v", err)
	}

	ctrl := newControllerServer(&controllerServerConfig{
		driver:      initTestDriver(t),
		fileService: fs,
		cloud:       cloudProvider,
		volumeLocks: util.NewVolumeLocks(),
		features:    &GCFSDriverFeatureOptions{FeatureLockRelease: &FeatureLockRelease{}},
		tagManager:  cloud.NewFakeTagManager(),
	})

	// set valid fixed iops
	req := &csi.ControllerModifyVolumeRequest{
		VolumeId:          testVolumeID,
		MutableParameters: map[string]string{ParamMaxIOPS: "3000"},
	}
	resp, err := ctrl.ControllerModifyVolume(context.Background(), req)
	if err != nil {
		t.Fatalf("expected success for valid update, got error: %v", err)
	}
	if resp == nil {
		t.Fatalf("expected non-nil response for successful update")
	}
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
			bytes: 100 * util.Gb,
			tier:  defaultTier,
		},
		{
			name: "required equals min",
			capRange: &csi.CapacityRange{
				RequiredBytes: 100 * util.Gb,
			},
			tier:          defaultTier,
			bytes:         100 * util.Gb,
			errorExpected: false,
		},
		{
			name: "required above small zonal",
			capRange: &csi.CapacityRange{
				RequiredBytes: 11 * util.Tb,
			},
			tier:  zonalTier,
			bytes: 11 * util.Tb,
		},
		{
			name: "required in between small and large zonal",
			capRange: &csi.CapacityRange{
				RequiredBytes: 9990 * util.Gb,
			},
			tier:  zonalTier,
			bytes: 10 * util.Tb,
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
				LimitBytes: 100 * util.Gb,
			},
			tier:  defaultTier,
			bytes: 100 * util.Gb,
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
				RequiredBytes: 80 * util.Gb,
				LimitBytes:    2 * util.Tb,
			},
			tier:  defaultTier,
			bytes: 100 * util.Gb,
		},
		{
			name: "required below min, limit below min",
			capRange: &csi.CapacityRange{
				RequiredBytes: 50 * util.Gb,
				LimitBytes:    90 * util.Gb,
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
				LimitBytes: 10 * util.Gb,
			},
			tier:          defaultTier,
			errorExpected: true,
		},
		{
			name: "required above max default with limit set",
			capRange: &csi.CapacityRange{
				LimitBytes:    50 * util.Tb,
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
			name: "required above max enterprise with limit set",
			capRange: &csi.CapacityRange{
				LimitBytes:    50 * util.Tb,
				RequiredBytes: 100 * util.Tb,
			},
			tier:          enterpriseTier,
			errorExpected: true,
		},
		{
			name: "required above max enterprise without limit",
			capRange: &csi.CapacityRange{
				RequiredBytes: 100 * util.Tb,
			},
			tier:  enterpriseTier,
			bytes: 100 * util.Tb,
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
			name: "required above max highScale with limit set",
			capRange: &csi.CapacityRange{
				LimitBytes:    100 * util.Tb,
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
			name: "required above max premium with limit set",
			capRange: &csi.CapacityRange{
				LimitBytes:    60 * util.Tb,
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
			name: "required above max basicSSD with limit set",
			capRange: &csi.CapacityRange{
				LimitBytes:    639 * util.Tb / 10,
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
				LimitBytes: 10 * util.Gb,
			},
			tier:          basicHDDTier,
			errorExpected: true,
		},
		{
			name: "required above max basicHDD with limit set",
			capRange: &csi.CapacityRange{
				LimitBytes:    639 * util.Tb / 10,
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
		{
			name: "required in range ZONAL all cap",
			capRange: &csi.CapacityRange{
				RequiredBytes: 100 * util.Tb,
			},
			tier:  "ZONAL",
			bytes: 100 * util.Tb,
		},
		{
			name: "required above max BASIC_SSD all cap with limit set",
			capRange: &csi.CapacityRange{
				LimitBytes:    639 * util.Tb / 10,
				RequiredBytes: 70 * util.Tb,
			},
			tier:          "BASIC_SSD",
			errorExpected: true,
		},
		{
			name: "required and limit both in range BASIC_SSD all cap",
			capRange: &csi.CapacityRange{
				RequiredBytes: 3 * util.Tb,
				LimitBytes:    60 * util.Tb,
			},
			tier:  "BASIC_SSD",
			bytes: 3 * util.Tb,
		},
		{
			name: "required less than small ZONAL minimum capacity",
			capRange: &csi.CapacityRange{
				RequiredBytes: 100 * util.Gb,
			},
			tier:  "ZONAL",
			bytes: 1 * util.Tb,
		},
		{
			name: "required in small ZONAL range",
			capRange: &csi.CapacityRange{
				RequiredBytes: 3 * util.Tb,
			},
			tier:  "ZONAL",
			bytes: 3 * util.Tb,
		},
		{
			name: "required in small ZONAL range all cap",
			capRange: &csi.CapacityRange{
				RequiredBytes: 9984 * util.Gb,
			},
			tier:  "ZONAL",
			bytes: 9984 * util.Gb,
		},
		{
			name: "required in large ZONAL range",
			capRange: &csi.CapacityRange{
				RequiredBytes: 10 * util.Tb,
			},
			tier:  "ZONAL",
			bytes: 10 * util.Tb,
		},
		{
			name: "required in between small and large ZONAL range",
			capRange: &csi.CapacityRange{
				RequiredBytes: 9985 * util.Gb,
			},
			tier:  "ZONAL",
			bytes: 10 * util.Tb,
		},
		{
			name: "required above large ZONAL range with limit set",
			capRange: &csi.CapacityRange{
				LimitBytes:    100 * util.Tb,
				RequiredBytes: 110 * util.Tb,
			},
			tier:          "ZONAL",
			errorExpected: true,
		},
		{
			name: "required above small regional",
			capRange: &csi.CapacityRange{
				RequiredBytes: 11 * util.Tb,
			},
			tier:  regionalTier,
			bytes: 11 * util.Tb,
		},
		{
			name: "required in between small and large regional",
			capRange: &csi.CapacityRange{
				RequiredBytes: 9990 * util.Gb,
			},
			tier:  regionalTier,
			bytes: 10 * util.Tb,
		},
		{
			name: "required equals small REGIONAL minimum capacity",
			capRange: &csi.CapacityRange{
				RequiredBytes: regionalSmallTierMinSize,
			},
			tier:  regionalTier,
			bytes: regionalSmallTierMinSize,
		},
		{
			name: "required 50 GB below small REGIONAL minimum capacity",
			capRange: &csi.CapacityRange{
				RequiredBytes: 50 * util.Gb,
			},
			tier:  regionalTier,
			bytes: regionalSmallTierMinSize,
		},
		{
			name: "required 1 GB above small REGIONAL minimum capacity",
			capRange: &csi.CapacityRange{
				RequiredBytes: 101 * util.Gb,
			},
			tier:  regionalTier,
			bytes: 101 * util.Gb,
		},
		{
			name: "limit equals small REGIONAL minimum capacity",
			capRange: &csi.CapacityRange{
				LimitBytes: regionalSmallTierMinSize,
			},
			tier:  regionalTier,
			bytes: regionalSmallTierMinSize,
		},
		{
			name: "limit below small REGIONAL minimum capacity",
			capRange: &csi.CapacityRange{
				LimitBytes: 50 * util.Gb,
			},
			tier:          regionalTier,
			errorExpected: true,
		},
		{
			name: "required below min, limit equals min REGIONAL",
			capRange: &csi.CapacityRange{
				RequiredBytes: 50 * util.Gb,
				LimitBytes:    regionalSmallTierMinSize,
			},
			tier:  regionalTier,
			bytes: regionalSmallTierMinSize,
		},
		{
			name: "required in small REGIONAL range",
			capRange: &csi.CapacityRange{
				RequiredBytes: 3 * util.Tb,
			},
			tier:  regionalTier,
			bytes: 3 * util.Tb,
		},
		{
			name: "required in small REGIONAL range all cap",
			capRange: &csi.CapacityRange{
				RequiredBytes: regionalSmallTierMaxSize,
			},
			tier:  regionalTier,
			bytes: regionalSmallTierMaxSize,
		},
		{
			name: "required in large REGIONAL range",
			capRange: &csi.CapacityRange{
				RequiredBytes: 10 * util.Tb,
			},
			tier:  regionalTier,
			bytes: 10 * util.Tb,
		},
		{
			name: "required in between small and large REGIONAL range",
			capRange: &csi.CapacityRange{
				RequiredBytes: regionalSmallTierMaxSize + 1,
			},
			tier:  regionalTier,
			bytes: 10 * util.Tb,
		},
		{
			name: "required above large REGIONAL range with limit set",
			capRange: &csi.CapacityRange{
				LimitBytes:    100 * util.Tb,
				RequiredBytes: 110 * util.Tb,
			},
			tier:          regionalTier,
			errorExpected: true,
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
				Protocol: v3FileProtocol,
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
				Protocol: v3FileProtocol,
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
				Protocol: v3FileProtocol,
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
				Protocol: v3FileProtocol,
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
				Protocol: v3FileProtocol,
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
				Protocol:   v3FileProtocol,
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
				Protocol:   v3FileProtocol,
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
		{
			name: "regional tier sets region as location",
			params: map[string]string{
				paramTier: regionalTier,
			},
			toporeq: &csi.TopologyRequirement{
				Preferred: []*csi.Topology{
					{
						Segments: map[string]string{
							TopologyKeyZone: "us-central1-c",
						},
					},
				},
			},
			instance: &file.ServiceInstance{
				Project:  testProject,
				Name:     testCSIVolume,
				Location: testRegion, // Should be region, not zone
				Tier:     regionalTier,
				Network: file.Network{
					Name:        defaultNetwork,
					ConnectMode: directPeering,
				},
				Volume: file.Volume{
					Name:      newInstanceVolume,
					SizeBytes: testBytes,
				},
				Protocol: v3FileProtocol,
			},
		},
	}

	for _, test := range cases {
		cs := initTestController(t)
		internalServer, ok := cs.(*controllerServer)
		if !ok {
			t.Fatalf("couldn't get internal controller")
		}

		filer, err := internalServer.generateNewFileInstance(testCSIVolume, testBytes, test.params, nil, test.toporeq)
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
	cs := initBlockingTestController(t, operationUnblocker).(*controllerServer)
	cs.config.tagManager.(*cloud.FakeTagServiceManager).
		On("AttachResourceTags", context.Background(), cloud.FilestoreInstance, testCSIVolume, testLocation, testCSIVolume, map[string]string(nil)).
		Return(nil)
	cs.config.tagManager.(*cloud.FakeTagServiceManager).
		On("AttachResourceTags", context.Background(), cloud.FilestoreInstance, testCSIVolume2, testLocation, testCSIVolume2, map[string]string(nil)).
		Return(nil)
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
	backupName2 := "mybackup2"
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
		{
			name: "Parameters contain misconfigured labels(invalid KV separator(:) used)",
			req: &csi.CreateSnapshotRequest{
				SourceVolumeId: "modeInstance/us-central1/myinstance/myshare",
				Name:           backupName,
				Parameters: map[string]string{
					util.VolumeSnapshotTypeKey: "backup",
					ParameterKeyLabels:         "key1:value1",
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
		{
			// If the incorrect labels were added, labels processing will not happen for already
			// existing backup resources.
			name: "Existing backup found, in state READY. Labels will not be processed.",
			req: &csi.CreateSnapshotRequest{
				SourceVolumeId: "modeInstance/us-central1-c/myinstance/myshare",
				Name:           backupName,
				Parameters: map[string]string{
					util.VolumeSnapshotTypeKey: "backup",
					ParameterKeyLabels:         "key1:value1",
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
		{
			name: "adding tags to filestore backup fails(failure scenario mocked)",
			req: &csi.CreateSnapshotRequest{
				SourceVolumeId: "modeInstance/us-central1/myinstance/myshare",
				Name:           backupName2,
				Parameters: map[string]string{
					util.VolumeSnapshotTypeKey:     "backup",
					cloud.ParameterKeyResourceTags: "kubernetes/test1/test1",
				},
			},
			initialBackup: nil,
			expectErr:     true,
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
			tagManager:  cloud.NewFakeTagManager(),
		}).(*controllerServer)

		cs.config.tagManager.(*cloud.FakeTagServiceManager).
			On("AttachResourceTags", context.TODO(), cloud.FilestoreBackUp, backupName, region, test.req.GetName(), test.req.GetParameters()).
			Return(nil)
		cs.config.tagManager.(*cloud.FakeTagServiceManager).
			On("AttachResourceTags", context.TODO(), cloud.FilestoreBackUp, backupName, "us-west1", test.req.GetName(), test.req.GetParameters()).
			Return(nil)
		cs.config.tagManager.(*cloud.FakeTagServiceManager).
			On("AttachResourceTags", context.TODO(), cloud.FilestoreBackUp, backupName2, region, test.req.GetName(), test.req.GetParameters()).
			Return(fmt.Errorf("mock failure: error while adding tags to filestore backup"))

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

func TestDeleteSnapshot(t *testing.T) {
	backupName := "mybackup"
	project := "test-project"
	zone := "us-central1-c"
	region := "us-central1"
	instanceName := "myinstance"
	shareName := "myshare"
	cases := []struct {
		name        string
		createReq   *csi.CreateSnapshotRequest
		deleteReq   *csi.DeleteSnapshotRequest
		backupState string
		expectErr   bool
	}{
		{
			name: "Create singleshare snapshot and delete it",
			createReq: &csi.CreateSnapshotRequest{
				SourceVolumeId: fmt.Sprintf("modeInstance/%s/%s/%s", zone, instanceName, shareName),
				Name:           backupName,
			},
			deleteReq: &csi.DeleteSnapshotRequest{
				SnapshotId: fmt.Sprintf("projects/%s/locations/%s/backups/%s", project, region, backupName),
			},
			expectErr: false,
		},
		{
			name: "Backup is already in state DELETING. Expect error",
			createReq: &csi.CreateSnapshotRequest{
				SourceVolumeId: fmt.Sprintf("modeInstance/%s/%s/%s", zone, instanceName, shareName),
				Name:           backupName,
			},
			deleteReq: &csi.DeleteSnapshotRequest{
				SnapshotId: fmt.Sprintf("projects/%s/locations/%s/backups/%s", project, region, backupName),
			},
			expectErr:   true,
			backupState: "DELETING",
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
			tagManager:  cloud.NewFakeTagManager(),
		}).(*controllerServer)

		cs.config.tagManager.(*cloud.FakeTagServiceManager).
			On("AttachResourceTags", context.TODO(), cloud.FilestoreBackUp, backupName, region, test.createReq.GetName(), test.createReq.GetParameters()).
			Return(nil)

		_, err = cs.CreateSnapshot(context.TODO(), test.createReq)
		if err != nil {
			t.Errorf("test %q failed: %v", test.name, err)
		}

		if test.backupState != "" {
			backup, _ := fileService.GetBackup(context.TODO(), test.deleteReq.SnapshotId)
			backup.Backup.State = test.backupState
		}
		_, err = cs.DeleteSnapshot(context.TODO(), test.deleteReq)
		if !test.expectErr && err != nil {
			t.Errorf("test %q failed: %v", test.name, err)
		}
		if test.expectErr && err == nil {
			t.Errorf("test %q failed; got success", test.name)
		}
		if !test.expectErr {
			backup, err := fileService.GetBackup(context.TODO(), test.deleteReq.SnapshotId)
			if err == nil {
				t.Errorf("test %q failed; expected backup %+v to be deleted", test.name, backup)
			}
			if !file.IsNotFoundErr(err) {
				t.Errorf("test %q failed; expected NotFound error, got  %+v", test.name, err)
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

func TestParsingNfsExportOptions(t *testing.T) {
	cases := []struct {
		name            string
		optionsString   string
		expectedOptions []*file.NfsExportOptions
		expectErr       bool
	}{
		{
			name: "Base case single options",
			optionsString: `[
						{
							"accessMode": "READ_WRITE",
							"ipRanges": [
								"10.0.0.0/24"
							],
							"squashMode": "ROOT_SQUASH",
							"anonUid": "1003",
							"anonGid": "1003"
						}
					]`,
			expectedOptions: []*file.NfsExportOptions{
				{
					AccessMode: "READ_WRITE",
					IpRanges:   []string{"10.0.0.0/24"},
					SquashMode: "ROOT_SQUASH",
					AnonGid:    1003,
					AnonUid:    1003,
				},
			},
			expectErr: false,
		},
		{
			name: "Base case multiple options",
			optionsString: `[
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
			expectErr: false,
		},
		{
			name: "Invalid extra keys throw an error",
			optionsString: `[
						{
							"accessMode": "READ_WRITE",
							"ipRanges": [
								"10.0.0.0/24"
							],
							"squashMode": "ROOT_SQUASH",
							"anonUid": "1003",
							"anonGid": "1003",
							"INVALID_KEY": "INVALID_VALUE"
						}
					]`,
			expectErr: true,
		},
		{
			name:            "Empty string returns nil",
			optionsString:   "",
			expectedOptions: nil,
			expectErr:       false,
		},
		{
			name: "Invalid json returns error",
			optionsString: `
						{
							"accessMode": "READ_WRITE",
							"ipRanges": [
								"10.0.0.0/24"
							],
							`,
			expectErr: true,
		},
	}
	for _, test := range cases {
		parsedOptions, err := parseNfsExportOptions(test.optionsString)
		if !test.expectErr && err != nil {
			t.Errorf("test %q failed: %v", test.name, err)
		}
		if !reflect.DeepEqual(test.expectedOptions, parsedOptions) {
			t.Errorf("test %q failed; expected: %#v; got %#v", test.name, test.expectedOptions, parsedOptions)
		}
	}
}

func TestExtractLabels(t *testing.T) {
	var (
		driverName      = "test_driver"
		pvcName         = "test_pvc"
		pvcNamespace    = "test_pvc_namespace"
		pvName          = "test_pv"
		parameterLabels = "key1=value1,key2=value2"
	)

	cases := []struct {
		name         string
		parameters   map[string]string
		cliLabels    map[string]string
		expectLabels map[string]string
		expectError  string
	}{
		{
			name: "Success case",
			parameters: map[string]string{
				ParameterKeyPVCName:      pvcName,
				ParameterKeyPVCNamespace: pvcNamespace,
				ParameterKeyPVName:       pvName,
				ParameterKeyLabels:       parameterLabels,
			},
			cliLabels: map[string]string{
				"key3": "value3",
				"key4": "value4",
			},
			expectLabels: map[string]string{
				"key1":                         "value1",
				"key2":                         "value2",
				"key3":                         "value3",
				"key4":                         "value4",
				tagKeyCreatedForVolumeName:     pvName,
				tagKeyCreatedForClaimName:      pvcName,
				tagKeyCreatedForClaimNamespace: pvcNamespace,
				tagKeyCreatedBy:                driverName,
			},
		},
		{
			name: "Parsing labels in storageClass fails(invalid KV separator(:) used)",
			parameters: map[string]string{
				ParameterKeyPVCName:      pvcName,
				ParameterKeyPVCNamespace: pvcNamespace,
				ParameterKeyPVName:       pvName,
				ParameterKeyLabels:       "key1:value1,key2:value2",
			},
			cliLabels: map[string]string{
				"key3": "value3",
				"key4": "value4",
			},
			expectLabels: nil,
			expectError:  `parameters contain invalid labels parameter: labels "key1:value1,key2:value2" are invalid, correct format: 'key1=value1,key2=value2'`,
		},
		{
			name: "storageClass labels contain reserved metadata label(kubernetes_io_created-for_pv_name)",
			parameters: map[string]string{
				ParameterKeyPVCName:      pvcName,
				ParameterKeyPVCNamespace: pvcNamespace,
				ParameterKeyPVName:       pvName,
				ParameterKeyLabels:       "key1=value1,key2=value2,kubernetes_io_created-for_pv_name=test",
			},
			cliLabels: map[string]string{
				"key3": "value3",
				"key4": "value4",
			},
			expectLabels: nil,
			expectError:  `storage Class labels cannot contain metadata label key kubernetes_io_created-for_pv_name`,
		},
		{
			name: "storageClass labels parameter not present, only the CLI labels are defined",
			parameters: map[string]string{
				ParameterKeyPVCName:      pvcName,
				ParameterKeyPVCNamespace: pvcNamespace,
				ParameterKeyPVName:       pvName,
			},
			cliLabels: map[string]string{
				"key3": "value3",
				"key4": "value4",
			},
			expectLabels: map[string]string{
				"key3":                         "value3",
				"key4":                         "value4",
				tagKeyCreatedForVolumeName:     pvName,
				tagKeyCreatedForClaimName:      pvcName,
				tagKeyCreatedForClaimNamespace: pvcNamespace,
				tagKeyCreatedBy:                driverName,
			},
		},
		{
			name: "CLI labels not defined, labels are defined only in the storageClass object",
			parameters: map[string]string{
				ParameterKeyPVCName:      pvcName,
				ParameterKeyPVCNamespace: pvcNamespace,
				ParameterKeyPVName:       pvName,
				ParameterKeyLabels:       parameterLabels,
			},
			cliLabels: nil,
			expectLabels: map[string]string{
				"key1":                         "value1",
				"key2":                         "value2",
				tagKeyCreatedForVolumeName:     pvName,
				tagKeyCreatedForClaimName:      pvcName,
				tagKeyCreatedForClaimNamespace: pvcNamespace,
				tagKeyCreatedBy:                driverName,
			},
		},
		{
			name: "CLI labels and storageClass labels parameter not defined",
			parameters: map[string]string{
				ParameterKeyPVCName:      pvcName,
				ParameterKeyPVCNamespace: pvcNamespace,
				ParameterKeyPVName:       pvName,
			},
			cliLabels: nil,
			expectLabels: map[string]string{
				tagKeyCreatedForVolumeName:     pvName,
				tagKeyCreatedForClaimName:      pvcName,
				tagKeyCreatedForClaimNamespace: pvcNamespace,
				tagKeyCreatedBy:                driverName,
			},
		},
		{
			name: "CLI labels and storageClass labels has duplicates",
			parameters: map[string]string{
				ParameterKeyPVCName:      pvcName,
				ParameterKeyPVCNamespace: pvcNamespace,
				ParameterKeyPVName:       pvName,
				ParameterKeyLabels:       parameterLabels,
			},
			cliLabels: map[string]string{
				"key1": "value1",
				"key2": "value202",
			},
			expectLabels: map[string]string{
				"key1":                         "value1",
				"key2":                         "value2",
				tagKeyCreatedForVolumeName:     pvName,
				tagKeyCreatedForClaimName:      pvcName,
				tagKeyCreatedForClaimNamespace: pvcNamespace,
				tagKeyCreatedBy:                driverName,
			},
		},
	}
	for _, test := range cases {
		labels, err := extractLabels(test.parameters, test.cliLabels, driverName)
		if (err != nil || test.expectError != "") && err.Error() != test.expectError {
			t.Errorf("extractLabels(): %s: got: %v, expectErr: %v", test.name, err, test.expectError)
		}
		if !reflect.DeepEqual(test.expectLabels, labels) {
			t.Errorf("extractLabels(): %s: got: %v, want: %v", test.name, labels, test.expectLabels)
		}
	}
}

func TestExtractBackupLabels(t *testing.T) {
	var (
		driverName      = "test_driver"
		snapshotName    = "test_snapshot"
		pvcName         = "test_pvc"
		pvcNamespace    = "test_pvc_namespace"
		pvName          = "test_pv"
		parameterLabels = "key1=value1,key2=value2"
	)

	cases := []struct {
		name         string
		parameters   map[string]string
		cliLabels    map[string]string
		expectLabels map[string]string
		expectError  string
	}{
		{
			name: "Success case",
			parameters: map[string]string{
				ParameterKeyPVCName:      pvcName,
				ParameterKeyPVCNamespace: pvcNamespace,
				ParameterKeyPVName:       pvName,
				ParameterKeyLabels:       parameterLabels,
			},
			cliLabels: map[string]string{
				"key3": "value3",
				"key4": "value4",
			},
			expectLabels: map[string]string{
				"key1":                         "value1",
				"key2":                         "value2",
				"key3":                         "value3",
				"key4":                         "value4",
				tagKeyCreatedForVolumeName:     pvName,
				tagKeyCreatedForClaimName:      pvcName,
				tagKeyCreatedForClaimNamespace: pvcNamespace,
				tagKeyCreatedBy:                driverName,
				tagKeySnapshotName:             snapshotName,
			},
		},
		{
			name: "Parsing labels in storageClass fails(invalid KV separator(:) used)",
			parameters: map[string]string{
				ParameterKeyPVCName:      pvcName,
				ParameterKeyPVCNamespace: pvcNamespace,
				ParameterKeyPVName:       pvName,
				ParameterKeyLabels:       "key1:value1,key2:value2",
			},
			cliLabels: map[string]string{
				"key3": "value3",
				"key4": "value4",
			},
			expectLabels: nil,
			expectError:  `parameters contain invalid labels parameter: labels "key1:value1,key2:value2" are invalid, correct format: 'key1=value1,key2=value2'`,
		},
		{
			name: "storageClass labels contain reserved metadata label(kubernetes_io_created-for_pv_name)",
			parameters: map[string]string{
				ParameterKeyPVCName:      pvcName,
				ParameterKeyPVCNamespace: pvcNamespace,
				ParameterKeyPVName:       pvName,
				ParameterKeyLabels:       "key1=value1,key2=value2,kubernetes_io_created-for_pv_name=test",
			},
			cliLabels: map[string]string{
				"key3": "value3",
				"key4": "value4",
			},
			expectLabels: nil,
			expectError:  `storage Class labels cannot contain metadata label key kubernetes_io_created-for_pv_name`,
		},
		{
			name: "storageClass labels parameter not present, only the CLI labels are defined",
			parameters: map[string]string{
				ParameterKeyPVCName:      pvcName,
				ParameterKeyPVCNamespace: pvcNamespace,
				ParameterKeyPVName:       pvName,
			},
			cliLabels: map[string]string{
				"key3": "value3",
				"key4": "value4",
			},
			expectLabels: map[string]string{
				"key3":                         "value3",
				"key4":                         "value4",
				tagKeyCreatedForVolumeName:     pvName,
				tagKeyCreatedForClaimName:      pvcName,
				tagKeyCreatedForClaimNamespace: pvcNamespace,
				tagKeyCreatedBy:                driverName,
				tagKeySnapshotName:             snapshotName,
			},
		},
		{
			name: "CLI labels not defined, labels are defined only in the storageClass object",
			parameters: map[string]string{
				ParameterKeyPVCName:      pvcName,
				ParameterKeyPVCNamespace: pvcNamespace,
				ParameterKeyPVName:       pvName,
				ParameterKeyLabels:       parameterLabels,
			},
			cliLabels: nil,
			expectLabels: map[string]string{
				"key1":                         "value1",
				"key2":                         "value2",
				tagKeyCreatedForVolumeName:     pvName,
				tagKeyCreatedForClaimName:      pvcName,
				tagKeyCreatedForClaimNamespace: pvcNamespace,
				tagKeyCreatedBy:                driverName,
				tagKeySnapshotName:             snapshotName,
			},
		},
		{
			name: "CLI labels and storageClass labels parameter not defined",
			parameters: map[string]string{
				ParameterKeyPVCName:      pvcName,
				ParameterKeyPVCNamespace: pvcNamespace,
				ParameterKeyPVName:       pvName,
			},
			cliLabels: nil,
			expectLabels: map[string]string{
				tagKeyCreatedForVolumeName:     pvName,
				tagKeyCreatedForClaimName:      pvcName,
				tagKeyCreatedForClaimNamespace: pvcNamespace,
				tagKeyCreatedBy:                driverName,
				tagKeySnapshotName:             snapshotName,
			},
		},
		{
			name: "CLI labels and storageClass labels has duplicates",
			parameters: map[string]string{
				ParameterKeyPVCName:      pvcName,
				ParameterKeyPVCNamespace: pvcNamespace,
				ParameterKeyPVName:       pvName,
				ParameterKeyLabels:       parameterLabels,
			},
			cliLabels: map[string]string{
				"key1": "value1",
				"key2": "value202",
			},
			expectLabels: map[string]string{
				"key1":                         "value1",
				"key2":                         "value2",
				tagKeyCreatedForVolumeName:     pvName,
				tagKeyCreatedForClaimName:      pvcName,
				tagKeyCreatedForClaimNamespace: pvcNamespace,
				tagKeyCreatedBy:                driverName,
				tagKeySnapshotName:             snapshotName,
			},
		},
	}
	for _, test := range cases {
		labels, err := extractBackupLabels(test.parameters, test.cliLabels, driverName, snapshotName)
		if (err != nil || test.expectError != "") && err.Error() != test.expectError {
			t.Errorf("extractBackupLabels(): %s: got: %v, expectErr: %v", test.name, err, test.expectError)
		}
		if !reflect.DeepEqual(test.expectLabels, labels) {
			t.Errorf("extractBackupLabels(): %s: got: %v, want: %v", test.name, labels, test.expectLabels)
		}
	}
}
