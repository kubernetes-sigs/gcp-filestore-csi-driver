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
	"testing"

	sanity "github.com/kubernetes-csi/csi-test/pkg/sanity"
	"k8s.io/kubernetes/pkg/util/mount"
	cloud "sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider"
	driver "sigs.k8s.io/gcp-filestore-csi-driver/pkg/csi_driver"
)

func TestSanity(t *testing.T) {
	// Set up variables
	driverName := "test-driver"
	driverVersion := "test-driver-version"
	nodeID := "io.kubernetes.storage.mock"
	endpoint := "unix:/tmp/csi.sock"
	mountPath := "/tmp/csi/mount"
	stagePath := "/tmp/csi/stage"

	// Set up driver and env
	cloudProvider, err := cloud.NewFakeCloud()
	if err != nil {
		t.Fatalf("Failed to get cloud provider: %v", err)
	}
	mounter := &mount.FakeMounter{MountPoints: []mount.MountPoint{}, Log: []mount.FakeAction{}}

	driverConfig := &driver.GCFSDriverConfig{
		Name:          driverName,
		Version:       driverVersion,
		NodeID:        nodeID,
		RunController: true,
		RunNode:       true,
		Mounter:       mounter,
		Cloud:         cloudProvider,
	}
	gcfsDriver, err := driver.NewGCFSDriver(driverConfig)
	if err != nil {
		t.Fatalf("Failed to initialize GCE CSI Driver: %v", err)
	}

	go func() {
		gcfsDriver.Run(endpoint)
	}()

	// Run test
	testConfig := &sanity.Config{
		TargetPath:  mountPath,
		StagingPath: stagePath,
		Address:     endpoint,
	}
	sanity.Test(t, testConfig)
}
