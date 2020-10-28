package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

type driverConfig struct {
	StorageClassFile     string
	StorageClass         string
	SnapshotClassFile    string
	Capabilities         []string
	SupportedFsType      []string
	MinimumVolumeSize    string
	NumAllowedTopologies int
}

const (
	testConfigDir      = "test/k8s-integration/config"
	configTemplateFile = "test-config-template.in"
	configFile         = "test-config.yaml"
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
		"topology",
	}

	switch testParams.deploymentStrategy {
	case "gce":
		caps = append(caps, "controllerExpansion")
	default:
		return "", fmt.Errorf("got unknown deployment strat %s, expected gce or gke", testParams.deploymentStrategy)
	}

	var absSnapshotClassFilePath string
	// If snapshot class is passed in as argument, include snapshot specific driver capabiltiites.
	if testParams.snapshotClassFile != "" {
		caps = append(caps, "snapshotDataSource")
		// Update the absolute file path pointing to the snapshot class file, if it is provided as an argument.
		absSnapshotClassFilePath = filepath.Join(testParams.pkgDir, testConfigDir, testParams.snapshotClassFile)
	}

	minimumVolumeSize := "1Ti"
	numAllowedTopologies := 1
	params := driverConfig{
		StorageClassFile:     filepath.Join(testParams.pkgDir, testConfigDir, storageClassFile),
		StorageClass:         storageClassFile[:strings.LastIndex(storageClassFile, ".")],
		SnapshotClassFile:    absSnapshotClassFilePath,
		Capabilities:         caps,
		MinimumVolumeSize:    minimumVolumeSize,
		NumAllowedTopologies: numAllowedTopologies,
	}

	// Write config file
	err = t.Execute(w, params)
	if err != nil {
		return "", err
	}

	return configFilePath, nil
}
