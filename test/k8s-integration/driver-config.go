package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	apimachineryversion "k8s.io/apimachinery/pkg/util/version"
)

type driverConfig struct {
	StorageClassFile     string
	StorageClass         string
	SnapshotClassFile    string
	Capabilities         []string
	SupportedFsType      []string
	MinimumVolumeSize    string
	NumAllowedTopologies int
	Timeouts             map[string]string
}

const (
	testConfigDir      = "test/k8s-integration/config"
	configTemplateFile = "test-config-template.in"
	configFile         = "test-config.yaml"
	enterpriseTier     = "enterprise"

	// configurable timeouts for the k8s e2e testsuites.
	podStartTimeout                 = "600s"
	claimProvisionTimeout           = "600s"
	enterpriseClaimProvisionTimeout = "1800s"

	// These are keys for the configurable timeout map.
	podStartTimeoutKey       = "PodStart"
	claimProvisionTimeoutKey = "ClaimProvision"
)

// generateDriverConfigFile loads a testdriver config template and creates a file
// with the test-specific configuration
func generateDriverConfigFile(testParams *testParameters, storageClassFile string) (string, error) {
	// Load template
	t, err := template.ParseFiles(filepath.Join(testParams.pkgDir, testConfigDir, configTemplateFile))
	if err != nil {
		return "", err
	}

	// Create destination
	configFilePath := filepath.Join(testParams.pkgDir, testConfigDir, configFile)
	f, err := os.Create(configFilePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	defer w.Flush()

	// Fill in template parameters. Capabilities can be found here:
	// https://github.com/kubernetes/kubernetes/blob/b717be8269a4f381ab6c23e711e8924bc1f64c93/test/e2e/storage/testsuites/testdriver.go#L136

	caps := []string{
		"persistence",
		"exec",
		"RWX",
		"multipods",
		"controllerExpansion",
	}

	var absSnapshotClassFilePath string
	// If snapshot class is passed in as argument, include snapshot specific driver capabiltiites.
	if testParams.snapshotClassFile != "" {
		caps = append(caps, "snapshotDataSource")
		// Update the absolute file path pointing to the snapshot class file, if it is provided as an argument.
		absSnapshotClassFilePath = filepath.Join(testParams.pkgDir, testConfigDir, testParams.snapshotClassFile)
	}

	switch testParams.deploymentStrategy {
	case "gke":
		var gkeVer *version
		// The node version is what matters for fsgroup support, as the code resides in kubelet. If the node version
		// is not given, we assume it's the same as the cluster master version.
		if testParams.nodeVersion != "" {
			gkeVer = mustParseVersion(testParams.nodeVersion)
		} else {
			gkeVer = mustParseVersion(testParams.clusterVersion)
		}
		if gkeVer.lessThan(mustParseVersion("1.20.0")) {
			// "CSIVolumeFSGroupPolicy" is beta 1.20+
		} else {
			caps = append(caps, "fsGroup")
		}
	case "gce":
		v := apimachineryversion.MustParseSemantic(testParams.clusterVersion)
		if v.LessThan(apimachineryversion.MustParseSemantic("1.20.0")) {
			// "CSIVolumeFSGroupPolicy" is beta 1.20+
		} else {
			caps = append(caps, "fsGroup")
		}
	default:
		return "", fmt.Errorf("got unknown deployment strat %s, expected gce or gke", testParams.deploymentStrategy)
	}

	minimumVolumeSize := "1Ti"
	// Filestore instance takes in the order of minutes to be provisioned, and with dynamic provisioning (WaitForFirstCustomer policy),
	// some e2e tests need a longer pod start timeout.
	timeouts := map[string]string{
		claimProvisionTimeoutKey: claimProvisionTimeout,
		podStartTimeoutKey:       podStartTimeout,
	}
	if strings.Contains(storageClassFile, enterpriseTier) {
		timeouts[claimProvisionTimeoutKey] = enterpriseClaimProvisionTimeout
	}
	params := driverConfig{
		StorageClassFile:  filepath.Join(testParams.pkgDir, testConfigDir, storageClassFile),
		StorageClass:      storageClassFile[:strings.LastIndex(storageClassFile, ".")],
		SnapshotClassFile: absSnapshotClassFilePath,
		Capabilities:      caps,
		MinimumVolumeSize: minimumVolumeSize,
		Timeouts:          timeouts,
	}

	// Write config file
	err = t.Execute(w, params)
	if err != nil {
		return "", err
	}

	return configFilePath, nil
}
