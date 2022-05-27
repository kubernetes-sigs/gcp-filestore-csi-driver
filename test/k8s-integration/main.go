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

package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"k8s.io/apimachinery/pkg/util/uuid"
	apimachineryversion "k8s.io/apimachinery/pkg/util/version"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/klog"
	testutils "sigs.k8s.io/gcp-filestore-csi-driver/test/e2e/utils"
)

var (
	// Kubernetes cluster flags
	teardownCluster  = flag.Bool("teardown-cluster", true, "teardown the cluster after the e2e test")
	teardownDriver   = flag.Bool("teardown-driver", true, "teardown the driver after the e2e test")
	bringupCluster   = flag.Bool("bringup-cluster", true, "build kubernetes and bringup a cluster")
	gceZone          = flag.String("gce-zone", "", "zone that the gce k8s cluster is created/found in")
	kubeVersion      = flag.String("kube-version", "", "version of Kubernetes to download and use for the cluster")
	testVersion      = flag.String("test-version", "", "version of Kubernetes to download and use for tests")
	kubeFeatureGates = flag.String("kube-feature-gates", "", "feature gates to set on new kubernetes cluster")
	localK8sDir      = flag.String("local-k8s-dir", "", "local prebuilt kubernetes/kubernetes directory to use for cluster and test binaries")
	deploymentStrat  = flag.String("deployment-strategy", "gce", "choose between deploying on gce or gke")
	numNodes         = flag.Int("num-nodes", -1, "the number of nodes in the test cluster")
	imageType        = flag.String("image-type", "cos_containerd", "the image type to use for the cluster")

	// Test infrastructure flags
	boskosResourceType = flag.String("boskos-resource-type", "gce-project", "name of the boskos resource type to reserve")
	storageClassFiles  = flag.String("storageclass-files", "fs-sc-basic-hdd.yaml", "name of storageclass yaml file to use for test relative to test/k8s-integration/config. This may be a comma-separated list to test multiple storage classes")
	snapshotClassFile  = flag.String("snapshotclass-file", "fs-backup-volumesnapshotclass.yaml", "name of snapshotclass yaml file to use for test relative to test/k8s-integration/config")
	inProw             = flag.Bool("run-in-prow", false, "is the test running in PROW")

	// Driver flags
	stagingImage      = flag.String("staging-image", "", "name of image to stage to")
	saFile            = flag.String("service-account-file", "", "path of service account file")
	deployOverlayName = flag.String("deploy-overlay-name", "", "which kustomize overlay to deploy the driver with")
	doDriverBuild     = flag.Bool("do-driver-build", true, "building the driver from source")
	useManagedDriver  = flag.Bool("use-gke-driver", false, "use GKE managed Filestore CSI driver for the tests")

	// Test flags
	testFocus = flag.String("test-focus", "External.Storage", "test focus for Kubernetes e2e")
	parallel  = flag.Int("parallel", 3, "the number of parallel tests setting for ginkgo parallelism")

	// SA for dev overlay
	devOverlaySA = flag.String("dev-overlay-sa", "", "default SA that will be plumbed to the GCE instances")

	// GKE specific flags
	gkeClusterVer        = flag.String("gke-cluster-version", "", "version of Kubernetes master and node for gke")
	gkeReleaseChannel    = flag.String("gke-release-channel", "", "GKE release channel to be used for cluster deploy. One of 'rapid', 'stable' or 'regular'")
	gkeTestClusterPrefix = flag.String("gke-cluster-prefix", "fs-csi", "Prefix of GKE cluster names. A random suffix will be appended to form the full name.")
	gkeTestClusterName   = flag.String("gke-cluster-name", "", "GKE cluster name")
	gkeNodeVersion       = flag.String("gke-node-version", "", "GKE cluster worker node version")
	gceRegion            = flag.String("gce-region", "", "region that gke regional cluster should be created in")
)

const (
	fsImagePlaceholder      = "k8s.gcr.io/cloud-provider-gcp/gcp-filestore-csi-driver"
	externalDriverNamespace = "gcp-filestore-csi-driver"
	// If the network name is changed, the same network needs to be provided in storage class template passed in 'storageClassFiles' flag.
	gceInstanceNetwork = "csi-filestore-test-network"
)

type testParameters struct {
	stagingVersion     string
	goPath             string
	pkgDir             string
	testParentDir      string
	testDir            string
	testFocus          string
	testSkip           string
	snapshotClassFile  string
	deploymentStrategy string
	outputDir          string
	clusterVersion     string
	cloudProviderArgs  []string
	imageType          string
	nodeVersion        string
	parallel           int
}

func init() {
	flag.Set("logtostderr", "true")
}

func main() {
	klog.InitFlags(nil)
	flag.Set("logtostderr", "true")
	flag.Parse()

	if !*inProw && *doDriverBuild {
		ensureVariable(stagingImage, true, "staging-image is a required flag, please specify the name of image to stage to")
	}

	if *useManagedDriver {
		ensureVariableVal(deploymentStrat, "gke", "'deployment-strategy' must be GKE for using managed driver")
		ensureFlag(doDriverBuild, false, "'do-driver-build' must be false when using GKE managed driver")
		ensureFlag(teardownDriver, false, "'teardown-driver' must be false when using GKE managed driver")
		ensureVariable(stagingImage, false, "'staging-image' must not be set when using GKE managed driver")
		ensureVariable(deployOverlayName, false, "'deploy-overlay-name' must not be set when using GKE managed driver")
	}

	if !*useManagedDriver {
		ensureVariable(deployOverlayName, true, "deploy-overlay-name is a required flag")
		if *deployOverlayName != "dev" {
			ensureVariable(saFile, true, "service-account-file is a required flag")
		}
	}

	if *deployOverlayName == "dev" {
		if *deploymentStrat != "gce" {
			klog.Fatalf("dev overlay only supported for gce deployment")
		}
		ensureVariable(devOverlaySA, true, "for dev overlay, an SA is needed with correct roles needed for setting up GCE node instances")
	}

	ensureVariable(testFocus, true, "test-focus is a required flag")

	if len(*gceRegion) != 0 {
		ensureVariable(gceZone, false, "gce-zone and gce-region cannot both be set")
	} else {
		ensureVariable(gceZone, true, "One of gce-zone or gce-region must be set")
	}

	if !*bringupCluster {
		ensureVariable(kubeFeatureGates, false, "kube-feature-gates set but not bringing up new cluster")
	} else {
		ensureVariable(imageType, true, "image type is a required flag. A good default is 'cos_containerd'")
	}

	if *deploymentStrat == "gce" {
		ensureVariable(gceZone, true, "gce-zone required for 'gce' deployment")
	} else if *deploymentStrat == "gke" {
		ensureVariable(kubeVersion, false, "Cannot set kube-version when using deployment strategy 'gke'. Use gke-cluster-version.")
		ensureExactlyOneVariableSet([]*string{gkeClusterVer, gkeReleaseChannel},
			"For GKE cluster deployment, exactly one of 'gke-cluster-version' or 'gke-release-channel' must be set")
		ensureVariable(kubeFeatureGates, false, "Cannot set feature gates when using deployment strategy 'gke'.")
		if len(*localK8sDir) == 0 {
			ensureVariable(testVersion, true, "Must set either test-version or local k8s dir when using deployment strategy 'gke'.")
			ensureVariableNotVal(testVersion, "master", "Must not set test-version to master when using deployment strategy 'gke'.")
		}
		if len(*gkeTestClusterName) == 0 {
			randSuffix := string(uuid.NewUUID())[0:4]
			*gkeTestClusterName = *gkeTestClusterPrefix + "-" + randSuffix
		}
	}

	if len(*localK8sDir) != 0 {
		ensureVariable(kubeVersion, false, "Cannot set a kube version when using a local k8s dir.")
		ensureVariable(testVersion, false, "Cannot set a test version when using a local k8s dir.")
	}

	if *numNodes == -1 && *bringupCluster {
		klog.Fatalf("num-nodes must be set to number of nodes in cluster")
	}

	err := handle()
	if err != nil {
		klog.Fatalf("Failed to run integration test: %v", err)
	}
}

func handle() error {
	oldmask := syscall.Umask(0000)
	defer syscall.Umask(oldmask)

	testParams := &testParameters{
		testFocus:          *testFocus,
		snapshotClassFile:  *snapshotClassFile,
		stagingVersion:     string(uuid.NewUUID()),
		deploymentStrategy: *deploymentStrat,
		imageType:          *imageType,
		parallel:           *parallel,
	}

	goPath, ok := os.LookupEnv("GOPATH")
	if !ok {
		return fmt.Errorf("could not find env variable GOPATH")
	}
	testParams.goPath = goPath
	testParams.pkgDir = filepath.Join(goPath, "src", "sigs.k8s.io", "gcp-filestore-csi-driver")
	// If running in Prow, then acquire and set up a project through Boskos
	if *inProw {
		oldProject, err := exec.Command("gcloud", "config", "get-value", "project").CombinedOutput()
		oldProjectStr := strings.TrimSpace(string(oldProject))
		if err != nil {
			return fmt.Errorf("failed to get gcloud project: %s, err: %v", oldProject, err)
		}

		newproject, _ := testutils.SetupProwConfig(*boskosResourceType)
		err = setEnvProject(newproject)
		if err != nil {
			return fmt.Errorf("failed to set project environment to %s: %v", newproject, err)
		}

		defer func() {
			err = setEnvProject(oldProjectStr)
			if err != nil {
				klog.Errorf("failed to set project environment to %s: %v", oldProject, err)
			}
		}()

		if *doDriverBuild {
			*stagingImage = fmt.Sprintf("gcr.io/%s/gcp-filestore-csi-driver", newproject)
		}
		if _, ok := os.LookupEnv("USER"); !ok {
			err = os.Setenv("USER", "prow")
			if err != nil {
				return fmt.Errorf("failed to set user in prow to prow: %v", err)
			}
		}
	}

	if *doDriverBuild {
		err := pushImage(testParams.pkgDir, *stagingImage, testParams.stagingVersion)
		if err != nil {
			return fmt.Errorf("failed pushing image: %v", err)
		}
		defer func() {
			if *teardownCluster {
				err := deleteImage(*stagingImage, testParams.stagingVersion)
				if err != nil {
					klog.Errorf("failed to delete image: %v", err)
				}
			}
		}()
	}

	// Create temporary directories for kubernetes builds
	k8sParentDir := generateUniqueTmpDir()
	testParams.testDir = filepath.Join(k8sParentDir, "kubernetes")
	defer removeDir(k8sParentDir)

	// If kube version is set, then download and build Kubernetes for cluster creation
	// Otherwise, a prebuild local K8s dir is being used
	if len(*kubeVersion) != 0 {
		err := downloadKubernetesSource(testParams.pkgDir, k8sParentDir, *kubeVersion)
		if err != nil {
			return fmt.Errorf("failed to download Kubernetes source: %v", err)
		}
		err = buildKubernetes(testParams.testDir, "quick-release")
		if err != nil {
			return fmt.Errorf("failed to build Kubernetes: %v", err)
		}
	} else {
		testParams.testDir = *localK8sDir
	}

	if *bringupCluster {
		var err error = nil
		switch *deploymentStrat {
		case "gce":
			err = clusterUpGCE(testParams.testDir, *gceZone, *numNodes, testParams.imageType)
		case "gke":
			err = clusterUpGKE(*gceZone, *gceRegion, *numNodes, testParams.imageType, *useManagedDriver)
		default:
			err = fmt.Errorf("deployment-strategy must be set to 'gce' or 'gke', but is: %s", *deploymentStrat)
		}
		if err != nil {
			return fmt.Errorf("failed to cluster up: %v", err)
		}
	}

	if *teardownCluster {
		defer func() {
			switch *deploymentStrat {
			case "gce":
				err := clusterDownGCE(testParams.testDir)
				if err != nil {
					klog.Errorf("failed to cluster down: %v", err)
				}
			case "gke":
				err := clusterDownGKE(*gceZone, *gceRegion)
				if err != nil {
					klog.Errorf("failed to cluster down: %v", err)
				}
			default:
				klog.Errorf("deployment-strategy must be set to 'gce', but is: %s", *deploymentStrat)
			}
		}()
	}

	if !*useManagedDriver {
		err := installDriver(testParams.goPath, testParams.pkgDir, *stagingImage, testParams.stagingVersion, *deployOverlayName, *doDriverBuild)
		if *teardownDriver {
			defer func() {
				if teardownErr := deleteDriver(testParams.goPath, testParams.pkgDir, *deployOverlayName); teardownErr != nil {
					klog.Errorf("failed to delete driver: %v", teardownErr)
				}
			}()
		}
		if err != nil {
			return fmt.Errorf("failed to install CSI Driver: %v", err)
		}
	}

	cancel, err := dumpDriverLogs()
	if err != nil {
		return fmt.Errorf("failed to start driver logging: %v", err)
	}
	defer func() {
		if cancel != nil {
			cancel()
		}
	}()

	switch testParams.deploymentStrategy {
	case "gke":
		testParams.cloudProviderArgs, err = getGKEKubeTestArgs(*gceZone, *gceRegion)
		if err != nil {
			return fmt.Errorf("failed to build GKE kubetest args: %v", err)
		}
	case "gce":
		testParams.cloudProviderArgs = []string{
			// This flag tells kubetest2 what "repo-root" is.
			// If --legacy-mode is set, kubernetes/kubernetes is used;
			// otherwise kubernetes/cloud-provider-gcp is used.
			"--legacy-mode",
			fmt.Sprintf("--repo-root=%s", testParams.testDir),
		}
	}

	// For clusters deployed on GCE, use the apimachinery version utils (which supports non-gke based semantic versioning).
	testParams.clusterVersion = mustGetKubeClusterVersion()
	klog.Infof("kubernetes cluster server version: %s", testParams.clusterVersion)
	switch *deploymentStrat {
	case "gce":
		testParams.testSkip = generateGCETestSkip(testParams)
	case "gke":
		testParams.nodeVersion = *gkeNodeVersion
		testParams.testSkip = generateGKETestSkip(testParams)
	default:
		return fmt.Errorf("unknown deployment strategy %s", *deploymentStrat)
	}

	// Run the tests using the testDir kubernetes
	if len(*storageClassFiles) != 0 {
		applicableStorageClassFiles := []string{}
		for _, rawScFile := range strings.Split(*storageClassFiles, ",") {
			scFile := strings.TrimSpace(rawScFile)
			if len(scFile) == 0 {
				continue
			}
			applicableStorageClassFiles = append(applicableStorageClassFiles, scFile)
		}
		if len(applicableStorageClassFiles) == 0 {
			return fmt.Errorf("no applicable storage classes found")
		}
		var ginkgoErrors []string
		var testOutputDirs []string
		for _, scFile := range applicableStorageClassFiles {
			outputDir := strings.TrimSuffix(scFile, ".yaml")
			testOutputDirs = append(testOutputDirs, outputDir)
			if err = runCSITests(testParams, scFile, outputDir); err != nil {
				ginkgoErrors = append(ginkgoErrors, err.Error())
			}
		}
		if err = mergeArtifacts(testOutputDirs); err != nil {
			return fmt.Errorf("artifact merging failed: %w", err)
		}
		if err != nil {
			return fmt.Errorf("failed to run tests: %w", err)
		}
		if ginkgoErrors != nil {
			return fmt.Errorf("runCSITests failed: %v", strings.Join(ginkgoErrors, " "))
		}
	} else {
		return fmt.Errorf("did not run either CSI test")
	}

	return nil
}

func generateGCETestSkip(testParams *testParameters) string {
	skipString := "\\[Disruptive\\]|\\[Serial\\]"
	v := apimachineryversion.MustParseSemantic(testParams.clusterVersion)

	// "volumeMode should not mount / map unused volumes in a pod" tests a
	// (https://github.com/kubernetes/kubernetes/pull/81163)
	// bug-fix introduced in 1.16
	if v.LessThan(apimachineryversion.MustParseSemantic("1.16.0")) {
		skipString = skipString + "|volumeMode\\sshould\\snot\\smount\\s/\\smap\\sunused\\svolumes\\sin\\sa\\spod"
	}
	if v.LessThan(apimachineryversion.MustParseSemantic("1.17.0")) {
		skipString = skipString + "|VolumeSnapshotDataSource"
	}

	if v.LessThan(apimachineryversion.MustParseSemantic("1.20.0")) {
		skipString = skipString + "|fsgroupchangepolicy"
	}
	return skipString
}

func generateGKETestSkip(testParams *testParameters) string {
	skipString := "\\[Disruptive\\]|\\[Serial\\]"
	curVer := mustParseVersion(testParams.clusterVersion)
	var nodeVer *version
	if testParams.nodeVersion != "" {
		nodeVer = mustParseVersion(testParams.nodeVersion)
	}

	// Skip fsgroup change policy based on GKE cluster version.
	if curVer.lessThan(mustParseVersion("1.20.0")) || (nodeVer != nil && nodeVer.lessThan(mustParseVersion("1.20.0"))) {
		skipString = skipString + "|fsgroupchangepolicy"
	}

	// Running snapshot tests on GKE clusters which does not support v1 snapshot CRDs needs 1.19x storage
	// e2e testsuite (until GKE supports v1 snapshot APIs). And to run 1.19x test suite for filestore,
	// https://github.com/kubernetes/kubernetes/pull/96042 needs to be cherry-picked to 1.19 branch, so that
	// configurable timeouts can be setup for filestore instance provisioning.
	if (*useManagedDriver && curVer.lessThan(mustParseVersion("1.20.7-gke.6"))) ||
		(!*useManagedDriver && (*curVer).lessThan(mustParseVersion("1.20.7"))) {
		skipString = skipString + "|VolumeSnapshotDataSource"
	}

	// https://github.com/kubernetes/kubernetes/pull/107065 changes the node staging path and introduces
	// this new test which filestore breaks on older node versions.
	if curVer.lessThan(mustParseVersion("1.24.0")) || (nodeVer != nil && nodeVer.lessThan(mustParseVersion("1.24.0"))) {
		skipString = skipString + "|should.mount.multiple.PV.pointing.to.the.same.storage.on.the.same.node"
	}

	return skipString
}

func runCSITests(testParams *testParameters, storageClassFile, reportPrefix string) error {
	testDriverConfigFile, err := generateDriverConfigFile(testParams, storageClassFile)
	if err != nil {
		return err
	}
	testConfigArg := fmt.Sprintf("--storage.testdriver=%s", testDriverConfigFile)
	return runTestsWithConfig(testParams, testConfigArg, reportPrefix)
}

func runTestsWithConfig(testParams *testParameters, testConfigArg, reportPrefix string) error {

	kubeconfig, err := getKubeConfig()
	if err != nil {
		return err
	}
	os.Setenv("KUBECONFIG", kubeconfig)

	artifactsDir, ok := os.LookupEnv("ARTIFACTS")
	kubetestDumpDir := ""
	if ok {
		if len(reportPrefix) > 0 {
			kubetestDumpDir = filepath.Join(artifactsDir, reportPrefix)
			if err := os.MkdirAll(kubetestDumpDir, 0755); err != nil {
				return err
			}
		} else {
			kubetestDumpDir = artifactsDir
		}
	}

	focus := testParams.testFocus
	skip := testParams.testSkip

	// kubetest2 flags
	var runID string
	if uid, exists := os.LookupEnv("PROW_JOB_ID"); exists && uid != "" {
		// reuse uid for CI use cases
		runID = uid
	} else {
		runID = string(uuid.NewUUID())
	}

	// Usage: kubetest2 <deployer> [Flags] [DeployerFlags] -- [TesterArgs]
	// [Flags]
	kubeTest2Args := []string{
		*deploymentStrat,
		fmt.Sprintf("--run-id=%s", runID),
		"--test=ginkgo",
	}

	// [DeployerFlags]
	kubeTest2Args = append(kubeTest2Args, testParams.cloudProviderArgs...)
	if kubetestDumpDir != "" {
		kubeTest2Args = append(kubeTest2Args, fmt.Sprintf("--artifacts=%s", kubetestDumpDir))
	}

	kubeTest2Args = append(kubeTest2Args, "--")

	// [TesterArgs]
	if len(*testVersion) != 0 {
		if *testVersion == "master" {
			// the kubernetes binaries should've already been built above because of `--kube-version`
			// or by the user if --local-k8s-dir was set, these binaries should be copied to the
			// path sent to kubetest2 through its --artifacts path

			// pkg/_artifacts is the default value that kubetests uses for --artifacts
			kubernetesTestBinariesPath := filepath.Join(testParams.pkgDir, "_artifacts")
			if kubetestDumpDir != "" {
				// a custom artifacts dir was set
				kubernetesTestBinariesPath = kubetestDumpDir
			}
			kubernetesTestBinariesPath = filepath.Join(kubernetesTestBinariesPath, runID)

			klog.Infof("Copying kubernetes binaries to path=%s to run the tests", kubernetesTestBinariesPath)
			err := copyKubernetesTestBinaries(testParams.testDir, kubernetesTestBinariesPath)
			if err != nil {
				return fmt.Errorf("failed to copy the kubernetes test binaries, err=%v", err)
			}
			kubeTest2Args = append(kubeTest2Args, "--use-built-binaries")
		} else {
			kubeTest2Args = append(kubeTest2Args, fmt.Sprintf("--test-package-marker=latest-%s.txt", *testVersion))
		}
	}
	kubeTest2Args = append(kubeTest2Args, fmt.Sprintf("--focus-regex=%s", focus))
	kubeTest2Args = append(kubeTest2Args, fmt.Sprintf("--skip-regex=%s", skip))
	kubeTest2Args = append(kubeTest2Args, fmt.Sprintf("--parallel=%d", testParams.parallel))
	kubeTest2Args = append(kubeTest2Args, fmt.Sprintf("--test-args=%s", testConfigArg))

	err = runCommand("Running Tests", exec.Command("kubetest2", kubeTest2Args...))
	if err != nil {
		return fmt.Errorf("failed to run tests on e2e cluster: %v", err)
	}

	return nil
}

func setEnvProject(project string) error {
	out, err := exec.Command("gcloud", "config", "set", "project", project).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set gcloud project to %s: %s, err: %v", project, out, err)
	}

	err = os.Setenv("PROJECT", project)
	if err != nil {
		return err
	}
	return nil
}

// copyKubernetesBinariesForTest copies the common test binaries to the output directory
func copyKubernetesTestBinaries(kuberoot string, outroot string) error {
	const dockerizedOutput = "_output/dockerized"
	var (
		kubernetesTestBinaries = []string{
			"kubectl",
			"e2e.test",
			"ginkgo",
		}
	)
	root := filepath.Join(kuberoot, dockerizedOutput, "bin", runtime.GOOS, runtime.GOARCH)
	for _, binary := range kubernetesTestBinaries {
		source := filepath.Join(root, binary)
		dest := filepath.Join(outroot, binary)
		if _, err := os.Stat(source); err == nil {
			klog.Infof("copying %s to %s", source, dest)
			if err := CopyFile(source, dest); err != nil {
				return fmt.Errorf("failed to copy %s to %s: %v", source, dest, err)
			}
		} else {
			return fmt.Errorf("could not find %s: %v", source, err)
		}
	}
	return nil
}
