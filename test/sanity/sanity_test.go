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

package sanitytest

import (
	"os"
	"testing"

	sanity "github.com/kubernetes-csi/csi-test/v3/pkg/sanity"
	"google.golang.org/grpc"
	mount "k8s.io/mount-utils"
	cloud "sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/metadata"
	driver "sigs.k8s.io/gcp-filestore-csi-driver/pkg/csi_driver"
)

const (
	Gb = 1024 * 1024 * 1024
	Tb = 1024 * Gb
)

func TestSanity(t *testing.T) {
	// Set up variables
	driverName := "test-driver"
	driverVersion := "test-driver-version"
	nodeID := "io.kubernetes.storage.mock"
	endpoint := "unix:/tmp/csi.sock"
	mountPath := "/tmp/csi/mount"
	stagePath := "/tmp/csi/stage"

	tmpDir := "/tmp/csi"
	err := os.MkdirAll(tmpDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create sanity temp working dir %s: %v", tmpDir, err)
	}
	defer func() {
		if err = os.RemoveAll(tmpDir); err != nil {
			t.Fatalf("Failed to clean up sanity temp working dir %s: %v", tmpDir, err)
		}
	}()

	// Set up driver and env
	cloudProvider, err := cloud.NewFakeCloud()
	if err != nil {
		t.Fatalf("Failed to get cloud provider: %v", err)
	}
	fakeMounter := &mount.FakeMounter{MountPoints: []mount.MountPoint{}}
	mounter := &mount.SafeFormatAndMount{
		Interface: fakeMounter,
	}

	meta, err := metadata.NewFakeService()
	if err != nil {
		t.Fatalf("Failed to get metadata service: %v", err)
	}
	driverConfig := &driver.GCFSDriverConfig{
		Name:            driverName,
		Version:         driverVersion,
		NodeName:        nodeID,
		RunController:   true,
		RunNode:         true,
		Mounter:         mounter,
		Cloud:           cloudProvider,
		MetadataService: meta,
		FeatureOptions:  &driver.GCFSDriverFeatureOptions{FeatureLockRelease: &driver.FeatureLockRelease{}},
		TagManager:      cloud.NewFakeTagManagerForSanityTests(),
	}
	gcfsDriver, err := driver.NewGCFSDriver(driverConfig)
	if err != nil {
		t.Fatalf("Failed to initialize GCE CSI Driver: %v", err)
	}

	go func() {
		gcfsDriver.Run(endpoint)
	}()

	// Run test
	testConfig := sanity.TestConfig{
		TargetPath:     mountPath,
		StagingPath:    stagePath,
		Address:        endpoint,
		DialOptions:    []grpc.DialOption{grpc.WithInsecure()},
		IDGen:          &sanity.DefaultIDGenerator{},
		TestVolumeSize: int64(1 * Tb),
	}
	sanity.Test(t, testConfig)
}
