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
	"flag"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	compute "google.golang.org/api/compute/v1"
	filev1beta1 "google.golang.org/api/file/v1beta1"
	"k8s.io/klog/v2"
	testutils "sigs.k8s.io/gcp-filestore-csi-driver/test/e2e/utils"
	remote "sigs.k8s.io/gcp-filestore-csi-driver/test/remote"
)

const (
	boskosResourceType = "gce-project"
)

var (
	project         = flag.String("project", "", "Project to run tests in")
	serviceAccount  = flag.String("service-account", "", "Service account to bring up instance with")
	runInProw       = flag.Bool("run-in-prow", false, "If true, use a Boskos loaned project and special CI service accounts and ssh keys")
	deleteInstances = flag.Bool("delete-instances", false, "Delete the instances after tests run")

	testContexts         = []*remote.TestContext{}
	zones                []string
	computeService       *compute.Service
	fileService          *filev1beta1.Service
	fileInstancesService *filev1beta1.ProjectsLocationsInstancesService
	fileBackupsService   *filev1beta1.ProjectsLocationsBackupsService
)

func init() {
	klog.InitFlags(flag.CommandLine)
}

func TestE2E(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Google Cloud Filestore CSI Driver Tests")
}

var _ = BeforeSuite(func() {
	var err error
	tcc := make(chan *remote.TestContext)
	defer close(tcc)
	zones = []string{"us-central1-c", "us-central1-b"}

	rand.Seed(time.Now().UnixNano())

	computeService, err = remote.GetComputeClient()
	Expect(err).To(BeNil())

	fileService, err = remote.GetFileClient()
	Expect(err).To(BeNil())

	fileInstancesService = filev1beta1.NewProjectsLocationsInstancesService(fileService)
	fileBackupsService = filev1beta1.NewProjectsLocationsBackupsService(fileService)

	if *runInProw {
		*project, *serviceAccount = testutils.SetupProwConfig(boskosResourceType)
	}

	Expect(*project).ToNot(BeEmpty(), "Project should not be empty")
	Expect(*serviceAccount).ToNot(BeEmpty(), "Service account should not be empty")

	for _, zone := range zones {
		go func(curZone string) {
			defer GinkgoRecover()
			nodeID := fmt.Sprintf("gcfs-csi-e2e-%s", curZone)

			i, err := remote.SetupInstance(*project, curZone, nodeID, *serviceAccount, computeService)
			if err != nil {
				tcc <- nil
			}
			Expect(err).To(BeNil(), "Set up Instance failed with error")

			// Create new driver and client
			testContext, err := testutils.GCFSClientAndDriverSetup(i)
			tcc <- testContext
			Expect(err).To(BeNil(), "Set up new Driver and Client failed with error")
		}(zone)
	}

	for i := 0; i < len(zones); i++ {
		tc := <-tcc
		if tc != nil {
			testContexts = append(testContexts, tc)
		}
	}
	Expect(len(testContexts)).Should(BeNumerically(">", 0), "Not enough instances available to run tests")
})

var _ = AfterSuite(func() {
	var wg sync.WaitGroup
	for _, tc := range testContexts {
		wg.Add(1)
		go func(curTC *remote.TestContext, wg *sync.WaitGroup) {
			defer wg.Done()
			defer GinkgoRecover()
			err := remote.TeardownDriverAndClient(curTC)
			Expect(err).To(BeNil(), "Teardown Driver and Client failed with error")
			if *deleteInstances {
				curTC.Instance.DeleteInstance()
			}
		}(tc, &wg)
	}
	wg.Wait()
})

func getRandomTestContext() *remote.TestContext {
	Expect(testContexts).ToNot(BeEmpty())
	rn := rand.Intn(len(testContexts))
	return testContexts[rn]
}
