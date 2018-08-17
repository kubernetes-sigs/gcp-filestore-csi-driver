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

package tests

import (
	"fmt"
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/util/uuid"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	testutils "sigs.k8s.io/gcp-filestore-csi-driver/test/e2e/utils"
	remote "sigs.k8s.io/gcp-filestore-csi-driver/test/remote"
)

const (
	testNamePrefix = "gcfs-csi-e2e-"

	defaultSizeGb int64 = 5
)

var _ = Describe("Google Cloud Filestore CSI Driver", func() {

	It("Should create->mount volume and check if it is writable, then unmount->delete and check if volume is deleted", func() {
		Expect(testInstances).NotTo(BeEmpty())
		testContext, err := testutils.GCFSClientAndDriverSetup(testInstances[0])
		Expect(err).To(BeNil(), "Set up new Driver and Client failed with error")
		defer func() {
			err := remote.TeardownDriverAndClient(testContext)
			Expect(err).To(BeNil(), "Teardown Driver and Client failed with error")
		}()

		client := testContext.Client
		instance := testContext.Instance

		// Create Disk
		volName := testNamePrefix + string(uuid.NewUUID())
		vol, err := client.CreateVolume(volName)
		Expect(err).To(BeNil(), "CreateVolume failed with error: %v", err)

		defer func() {
			// Delete Disk
			client.DeleteVolume(vol.GetId())
			Expect(err).To(BeNil(), "DeleteVolume failed")
		}()

		// TODO validate the Filestore instance creation at the cloud provider layer
		// Mount Disk
		publishDir := filepath.Join("/tmp/", volName, "mount")

		// Make remote directory
		_, err = instance.SSH(fmt.Sprint("mkdir -p ", publishDir))
		Expect(err).To(BeNil(), "Failed to delete remote directory")
		defer func() {
			_, err = instance.SSH(fmt.Sprint("rm -rf /tmp/", volName))
			Expect(err).To(BeNil(), "Failed to delete remote directory")
		}()

		err = client.NodePublishVolume(vol.GetId(), publishDir, vol.GetAttributes())
		Expect(err).To(BeNil(), "NodePublishVolume failed with error")

		err = testutils.ForceChmod(instance, publishDir, "777")
		Expect(err).To(BeNil(), "Chmod failed with error")

		// Write a file
		testFileContents := "test"
		testFile := filepath.Join(publishDir, "testfile")
		err = testutils.WriteFile(instance, testFile, testFileContents)
		Expect(err).To(BeNil(), "Failed to write file")

		// Unmount Disk
		err = client.NodeUnpublishVolume(vol.GetId(), publishDir)
		Expect(err).To(BeNil(), "NodeUnpublishVolume failed with error")

		// Mount disk somewhere else
		secondPublishDir := filepath.Join("/tmp/", volName, "secondmount")
		_, err = instance.SSH(fmt.Sprint("mkdir -p ", secondPublishDir))
		Expect(err).To(BeNil(), "Error while making directory on remote")

		err = client.NodePublishVolume(vol.GetId(), secondPublishDir, vol.GetAttributes())
		Expect(err).To(BeNil(), "NodePublishVolume failed with error")

		err = testutils.ForceChmod(instance, secondPublishDir, "777")
		Expect(err).To(BeNil(), "Chmod failed with error")

		// Read File
		secondTestFile := filepath.Join(secondPublishDir, "testfile")
		readContents, err := testutils.ReadFile(instance, secondTestFile)
		Expect(err).To(BeNil(), "ReadFile failed with error")
		Expect(strings.TrimSpace(string(readContents))).To(Equal(testFileContents))

		// Unmount Disk
		err = client.NodeUnpublishVolume(vol.GetId(), secondPublishDir)
		Expect(err).To(BeNil(), "NodeUnpublishVolume failed with error")

	})
})

func Logf(format string, args ...interface{}) {
	fmt.Fprint(GinkgoWriter, args...)
}
