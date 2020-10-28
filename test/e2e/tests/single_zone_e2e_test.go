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

	filev1beta1 "google.golang.org/api/file/v1beta1"
	"k8s.io/apimachinery/pkg/util/uuid"

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

	It("Should create->stage->mount volume and check if it is writable, then unmount->unstage->delete and check if volume is deleted", func() {
		testContext := getRandomTestContext()

		client := testContext.Client
		instance := testContext.Instance

		// Create Disk
		volName := testNamePrefix + string(uuid.NewUUID())
		vol, err := client.CreateVolume(volName)
		Expect(err).To(BeNil(), "CreateVolume failed with error: %v", err)

		defer func() {
			// Delete Disk
			client.DeleteVolume(vol.GetVolumeId())
			Expect(err).To(BeNil(), "DeleteVolume failed")

			_, err := getDisk(volName, instance)
			Expect(err).NotTo(BeNil(), "Could get deleted disk from cloud directly")
		}()

		// Validate Disk Created
		inst, err := getDisk(volName, instance)
		Expect(err).To(BeNil(), "Could not get disk from cloud directly")
		Expect(inst.State).To(Equal(readyState))
		Expect(inst.Tier).To(Equal(defaultTier))
		Expect(inst.Networks[0].Network).To(Equal(defaultNetwork))
		Expect(inst.FileShares[0].CapacityGb).To(Equal(util.RoundBytesToGb(minVolumeSize)))

		stageDir := filepath.Join("/tmp/", volName, "stage")
		_, err = instance.SSH(fmt.Sprint("mkdir -p ", stageDir))

		err = client.NodeStageVolume(vol.GetVolumeId(), stageDir, vol.GetVolumeContext())
		Expect(err).To(BeNil(), "NodeStageVolume failed with error")

		// Mount Disk
		publishDir := filepath.Join("/tmp/", volName, "mount")
		// Make remote directory
		_, err = instance.SSH(fmt.Sprint("mkdir -p ", publishDir))
		Expect(err).To(BeNil(), "Failed to delete remote directory")
		defer func() {
			_, err = instance.SSH(fmt.Sprint("rm -rf /tmp/", volName))
			Expect(err).To(BeNil(), "Failed to delete remote directory")
		}()

		err = client.NodePublishVolume(vol.GetVolumeId(), stageDir, publishDir, vol.GetVolumeContext())
		Expect(err).To(BeNil(), "NodePublishVolume failed with error")

		err = testutils.ForceChmod(instance, publishDir, "777")
		Expect(err).To(BeNil(), "Chmod failed with error")

		// Write a file
		testFileContents := "test"
		testFile := filepath.Join(publishDir, "testfile")
		err = testutils.WriteFile(instance, testFile, testFileContents)
		Expect(err).To(BeNil(), "Failed to write file")

		// Unmount Disk
		err = client.NodeUnpublishVolume(vol.GetVolumeId(), publishDir)
		Expect(err).To(BeNil(), "NodeUnpublishVolume failed with error")

		// Mount disk somewhere else
		secondPublishDir := filepath.Join("/tmp/", volName, "secondmount")
		_, err = instance.SSH(fmt.Sprint("mkdir -p ", secondPublishDir))
		Expect(err).To(BeNil(), "Error while making directory on remote")

		err = client.NodePublishVolume(vol.GetVolumeId(), stageDir, secondPublishDir, vol.GetVolumeContext())
		Expect(err).To(BeNil(), "NodePublishVolume failed with error")

		err = testutils.ForceChmod(instance, secondPublishDir, "777")
		Expect(err).To(BeNil(), "Chmod failed with error")

		// Read File
		secondTestFile := filepath.Join(secondPublishDir, "testfile")
		readContents, err := testutils.ReadFile(instance, secondTestFile)
		Expect(err).To(BeNil(), "ReadFile failed with error")
		Expect(strings.TrimSpace(string(readContents))).To(Equal(testFileContents))

		// Unmount Disk
		err = client.NodeUnpublishVolume(vol.GetVolumeId(), secondPublishDir)
		Expect(err).To(BeNil(), "NodeUnpublishVolume failed with error")

		// unstage
		err = client.NodeUnstageVolume(vol.GetVolumeId(), stageDir)
		Expect(err).To(BeNil(), "NodeUnstageVolume failed with error")
	})
})

func Logf(format string, args ...interface{}) {
	fmt.Fprint(GinkgoWriter, args...)
}

func getDisk(volName string, instance *remote.InstanceInfo) (*filev1beta1.Instance, error) {
	proj, zone, _ := instance.GetIdentity()
	instanceURI := fmt.Sprintf(instanceURIFormat, proj, zone, volName)
	return fileInstancesService.Get(instanceURI).Do()
}
