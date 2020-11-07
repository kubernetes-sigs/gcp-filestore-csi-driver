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
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	testutils "sigs.k8s.io/gcp-filestore-csi-driver/test/e2e/utils"
)

var _ = Describe("Google Cloud Filestore CSI Driver", func() {

	It("Should create -> write -> backup in zone 1 -> restore to zone 2 -> read -> delete", func() {
		var err error
		testContext := getRandomTestContext()

		diskInfo, cleanupDisk := createDisk("", "", nil, testContext)
		defer cleanupDisk()
		validateDisk(diskInfo)

		// Create Directories
		stageDir := filepath.Join("/tmp/", diskInfo.Name, "stage")
		publishDir := filepath.Join("/tmp/", diskInfo.Name, "mount")
		for _, dir := range []string{stageDir, publishDir} {
			err = testutils.MkdirAll(testContext.Instance, dir)
			Expect(err).To(BeNil(), "Mkdir failed with error")
		}
		defer func() {
			// delete remote directory
			fp := filepath.Join("/tmp/", diskInfo.Name)
			err = testutils.RmAll(testContext.Instance, fp)
			Expect(err).To(BeNil(), "Failed to delete remote directory")
		}()

		testFileName := "testfile-snapshot"
		testFileContents := "test"

		snapshotURI, cleanupSnapshot := writeAndBackupDisk(testFileName, testFileContents, stageDir, publishDir, diskInfo)
		defer cleanupSnapshot()

		restoreBackupAndRead(testFileName, testFileContents, stageDir, publishDir, snapshotURI, diskInfo)
	})
})

func writeAndBackupDisk(testFileName, testFileContents, stageDir, publishDir string, di *DiskInfo) (string, func()) {
	cleanupMount := stageAndPublish(stageDir, publishDir, di)
	defer cleanupMount()

	// Pre-backup write
	validateWrite(publishDir, testFileName, testFileContents, di.TestCtx.Instance)

	// Backup disk
	snapshotName := di.Name + "-snapshot"
	snapshotURI, err := di.TestCtx.Client.CreateSnapshot(snapshotName, di.Volume.GetVolumeId())
	Expect(err).To(BeNil(), "Create snapshot failed with error")

	// Validate backup exists
	_, err = fileBackupsService.Get(snapshotURI).Do()
	Expect(err).To(BeNil(), "Could not get snapshot from cloud directly")

	return snapshotURI, func() {
		err := di.TestCtx.Client.DeleteSnapshot(snapshotURI)
		Expect(err).To(BeNil(), "Delete snapshot failed with error")

		_, err = fileBackupsService.Get(snapshotURI).Do()
		Expect(err).NotTo(BeNil(), "Could get deleted snapshot from cloud directly")
	}
}

func restoreBackupAndRead(testFileName, testFileContents, stageDir, publishDir, snapshotURI string, di *DiskInfo) {
	zone := pickZoneForRestore(di)
	restoreDiskInfo, cleanupRestoredDisk := createDisk(zone, snapshotURI, nil, di.TestCtx)
	defer cleanupRestoredDisk()
	validateDisk(restoreDiskInfo)

	cleanupMount := stageAndPublish(stageDir, publishDir, restoreDiskInfo)
	defer cleanupMount()

	validateRead(publishDir, testFileName, testFileContents, restoreDiskInfo.TestCtx.Instance)
}

func pickZoneForRestore(di *DiskInfo) string {
	// Use a different zone than instance exists in if another zone exists
	if len(zones) > 1 {
		for _, z := range zones {
			_, zone, _ := di.TestCtx.Instance.GetIdentity()
			if z != zone {
				return z
			}
		}
	}
	return ""
}
