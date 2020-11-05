/*
Copyright 2020 The Kubernetes Authors.

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

package tests

import (
	"fmt"
	"path/filepath"
	"strings"

	filev1beta1 "google.golang.org/api/file/v1beta1"
	"k8s.io/apimachinery/pkg/util/uuid"

	"github.com/container-storage-interface/spec/lib/go/csi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/util"
	testutils "sigs.k8s.io/gcp-filestore-csi-driver/test/e2e/utils"
	remote "sigs.k8s.io/gcp-filestore-csi-driver/test/remote"
)

const (
	testNamePrefix = "gcfs-csi-e2e-"

	instanceURIFormat = "projects/%s/locations/%s/instances/%s"

	readyState           = "READY"
	defaultTier          = "STANDARD"
	defaultNetwork       = "default"
	minVolumeSize  int64 = 1 * util.Tb
)

var _ = Describe("Google Cloud Filestore CSI Driver", func() {

	It("Should create -> write/read -> offline resize -> online resize -> delete", func() {
		testContext := getRandomTestContext()

		diskInfo := createDisk(testContext)
		defer func() {
			err := testContext.Client.DeleteVolume(diskInfo.Volume.GetVolumeId())
			Expect(err).To(BeNil(), "DeleteVolume failed")

			_, err = getDisk(diskInfo)
			Expect(err).NotTo(BeNil(), "Could get deleted disk from cloud directly")
		}()
		validateDisk(diskInfo)

		writeAndReadDisk(diskInfo)

		offlineResizeDisk(diskInfo)

		onlineResizeDisk(diskInfo)
	})
})

func writeAndReadDisk(di *DiskInfo) {
	var err error
	instance := di.TestCtx.Instance
	volName := di.Name

	// Create Directories
	stageDir := filepath.Join("/tmp/", volName, "stage")
	publishDir := filepath.Join("/tmp/", volName, "mount")
	secondPublishDir := filepath.Join("/tmp/", volName, "secondmount")
	for _, dir := range []string{stageDir, publishDir, secondPublishDir} {
		err = testutils.MkdirAll(instance, dir)
		Expect(err).To(BeNil(), "Mkdir failed with error")
	}
	defer func() {
		// delete remote directory
		fp := filepath.Join("/tmp/", volName)
		err = testutils.RmAll(instance, fp)
		Expect(err).To(BeNil(), "Failed to delete remote directory")
	}()

	cleanup := stageAndPublish(stageDir, publishDir, di)
	defer cleanup()

	// Write a file
	testFileName := "testfile"
	testFileContents := "test"
	validateWrite(publishDir, testFileName, testFileContents, instance)

	// Mount disk somewhere else
	err = di.TestCtx.Client.NodePublishVolume(di.Volume.GetVolumeId(), stageDir, secondPublishDir, di.Volume.GetVolumeContext())
	Expect(err).To(BeNil(), "NodePublishVolume failed with error")
	defer func() {
		// Unmount Disk
		err = di.TestCtx.Client.NodeUnpublishVolume(di.Volume.GetVolumeId(), secondPublishDir)
		Expect(err).To(BeNil(), "NodeUnpublishVolume failed with error")
	}()

	validateRead(secondPublishDir, testFileName, "test", instance)
}

func onlineResizeDisk(di *DiskInfo) {
	var err error
	instance := di.TestCtx.Instance
	volName := di.Name

	// Create Directories
	stageDir := filepath.Join("/tmp/", volName, "stage")
	publishDir := filepath.Join("/tmp/", volName, "mount")
	for _, dir := range []string{stageDir, publishDir} {
		err = testutils.MkdirAll(instance, dir)
		Expect(err).To(BeNil(), "Mkdir failed with error")
	}
	defer func() {
		// delete remote directory
		fp := filepath.Join("/tmp/", volName)
		err = testutils.RmAll(instance, fp)
		Expect(err).To(BeNil(), "Failed to delete remote directory")
	}()

	cleanup := stageAndPublish(stageDir, publishDir, di)
	defer cleanup()

	// Resize

	// Pre Resize Write
	testFileName := "testfile-preresize"
	testFileContents := "test"
	validateWrite(publishDir, testFileName, testFileContents, instance)

	// Resize without limit to nearest Gb
	newSizeBytes := minVolumeSize + (10 * util.Gb) + 1
	err = di.TestCtx.Client.ControllerExpandVolume(di.Volume.GetVolumeId(), newSizeBytes)
	Expect(err).To(BeNil(), "Controller expand volume failed for resize without limit to nearest Gb")
	validateResizeDisk(newSizeBytes, di, "online resize - resize to nearest Gb")

	// Resize with limit too small
	oldSizeBytes := newSizeBytes
	newSizeBytes = minVolumeSize * 3
	newSizeLimit := minVolumeSize * 2
	err = di.TestCtx.Client.ControllerExpandVolumeWithLimit(di.Volume.GetVolumeId(), newSizeBytes, newSizeLimit)
	Expect(err).ToNot(BeNil(), "Controller expand volume unexpected success for resize with invalid limit")
	validateResizeDisk(oldSizeBytes, di, "online resize - resize with invalid limit")

	// Resize with limit
	newSizeBytes = minVolumeSize * 2
	newSizeLimit = minVolumeSize * 3
	err = di.TestCtx.Client.ControllerExpandVolumeWithLimit(di.Volume.GetVolumeId(), newSizeBytes, newSizeLimit)
	Expect(err).To(BeNil(), "Controller expand volume failed for resize with valid limit")
	validateResizeDisk(newSizeBytes, di, "online resize - resize with valid limit")

	// Invalid resize to smaller amount
	err = di.TestCtx.Client.ControllerExpandVolume(di.Volume.GetVolumeId(), minVolumeSize)
	Expect(err).To(BeNil(), "Controller expand volume unexpected failure for resize to invalid amount")
	validateResizeDisk(newSizeBytes, di, "online resize - resize with invalid amount")

	// Post Resize Read of Pre Resize Write
	validateRead(publishDir, testFileName, testFileContents, instance)

	// Post Resize Write/Read
	testFileName = "testfile-post"
	testFileContents = "testing-1-2-3"
	validateWrite(publishDir, testFileName, testFileContents, instance)
	validateRead(publishDir, testFileName, testFileContents, instance)
}

func offlineResizeDisk(di *DiskInfo) {
	var err error
	instance := di.TestCtx.Instance
	volName := di.Name

	// Resize controller
	var newSizeBytes int64 = minVolumeSize + (1 * util.Gb)
	err = di.TestCtx.Client.ControllerExpandVolume(di.Volume.GetVolumeId(), newSizeBytes)
	Expect(err).To(BeNil(), "Controller expand volume failed")
	validateResizeDisk(newSizeBytes, di, "offline resize")

	// Create Directories
	stageDir := filepath.Join("/tmp/", volName, "stage")
	publishDir := filepath.Join("/tmp/", volName, "mount")
	for _, dir := range []string{stageDir, publishDir} {
		err = testutils.MkdirAll(instance, dir)
		Expect(err).To(BeNil(), "Mkdir failed with error")
	}
	defer func() {
		// delete remote directory
		fp := filepath.Join("/tmp/", volName)
		err = testutils.RmAll(instance, fp)
		Expect(err).To(BeNil(), "Failed to delete remote directory")
	}()

	cleanup := stageAndPublish(stageDir, publishDir, di)
	defer cleanup()

	testFileName := "test-offline-resize"
	testFileContents := "testing"
	validateWrite(publishDir, testFileName, testFileContents, instance)
	validateRead(publishDir, testFileName, testFileContents, instance)
}

// DiskInfo contains information related to the filestore instance.
type DiskInfo struct {
	TestCtx *remote.TestContext
	Name    string
	Volume  *csi.Volume
}

func createDisk(tc *remote.TestContext) *DiskInfo {
	name := testNamePrefix + string(uuid.NewUUID())
	vol, err := tc.Client.CreateVolume(name)
	Expect(err).To(BeNil(), "CreateVolume failed with error: %v", err)
	return &DiskInfo{
		TestCtx: tc,
		Name:    name,
		Volume:  vol,
	}
}

func getDisk(di *DiskInfo) (*filev1beta1.Instance, error) {
	proj, zone, _ := di.TestCtx.Instance.GetIdentity()
	instanceURI := fmt.Sprintf(instanceURIFormat, proj, zone, di.Name)
	return fileInstancesService.Get(instanceURI).Do()
}

func stageAndPublish(stageDir, publishDir string, di *DiskInfo) func() {
	err := di.TestCtx.Client.NodeStageVolume(di.Volume.GetVolumeId(), stageDir, di.Volume.GetVolumeContext())
	Expect(err).To(BeNil(), "NodeStageVolume failed with error")

	err = di.TestCtx.Client.NodePublishVolume(di.Volume.GetVolumeId(), stageDir, publishDir, di.Volume.GetVolumeContext())
	Expect(err).To(BeNil(), "NodePublishVolume failed with error")

	return func() {
		err := di.TestCtx.Client.NodeUnpublishVolume(di.Volume.GetVolumeId(), publishDir)
		Expect(err).To(BeNil(), "NodeUnpublishVolume failed with error")

		err = di.TestCtx.Client.NodeUnstageVolume(di.Volume.GetVolumeId(), stageDir)
		Expect(err).To(BeNil(), "NodeUnstageVolume failed with error")
	}
}

func validateWrite(publishDir, testFileName, testFileContents string, instance *remote.InstanceInfo) {
	err := testutils.ForceChmod(instance, publishDir, "777")
	Expect(err).To(BeNil(), "Chmod failed with error")

	testFile := filepath.Join(publishDir, testFileName)
	err = testutils.WriteFile(instance, testFile, testFileContents)
	Expect(err).To(BeNil(), "Failed to write file")
}

func validateRead(publishDir, testFileName, testFileContents string, instance *remote.InstanceInfo) {
	err := testutils.ForceChmod(instance, publishDir, "777")
	Expect(err).To(BeNil(), "Chmod failed with error")

	testFile := filepath.Join(publishDir, testFileName)
	readContents, err := testutils.ReadFile(instance, testFile)
	Expect(err).To(BeNil(), "Failed to read file")
	Expect(strings.TrimSpace(string(readContents))).To(Equal(testFileContents))
}

func validateDisk(di *DiskInfo) {
	inst, err := getDisk(di)
	Expect(err).To(BeNil(), "Could not get disk from cloud directly")
	Expect(inst.State).To(Equal(readyState))
	Expect(inst.Tier).To(Equal(defaultTier))
	Expect(inst.Networks[0].Network).To(Equal(defaultNetwork))
	Expect(inst.FileShares[0].CapacityGb).To(Equal(util.RoundBytesToGb(minVolumeSize)))
}

func validateResizeDisk(newSizeBytes int64, di *DiskInfo, testDescription string) {
	inst, err := getDisk(di)
	Expect(err).To(BeNil(), "Get cloud disk failed")
	Expect(inst.FileShares[0].CapacityGb).To(Equal(util.BytesToGb(newSizeBytes)), testDescription)
}
