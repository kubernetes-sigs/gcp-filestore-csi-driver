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

package remote

import (
	"fmt"
	"os"
	"os/exec"

	"k8s.io/klog/v2"
)

func CreateDriverArchive(archiveName, pkgPath, binPath string) (string, error) {
	klog.V(2).Infof("Building archive...")
	tarDir, err := os.MkdirTemp("", "driver-temp-archive")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory %v", err)
	}
	defer os.RemoveAll(tarDir)

	// Call the suite function to setup the test package.
	err = setupBinaries(tarDir, pkgPath, binPath)
	if err != nil {
		return "", fmt.Errorf("failed to setup test package %q: %v", tarDir, err)
	}

	// Build the tar into a unique temp file to avoid collisions when tests run
	// in parallel (multiple goroutines creating the same filename).
	tmpFile, err := os.CreateTemp("", "e2e_driver_binaries-*.tar.gz")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file for archive: %v", err)
	}
	tmpPath := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close temp file %s: %v", tmpPath, err)
	}

	// Build the tar into the temp path
	out, err := exec.Command("tar", "-zcvf", tmpPath, "-C", tarDir, ".").CombinedOutput()
	if err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to build tar %v.  Output:\n%s", err, out)
	}

	return tmpPath, nil
}

func setupBinaries(tarDir, pkgPath, binPath string) error {
	klog.V(4).Infof("Making binaries and copying to temp dir...")
	out, err := exec.Command("make", "driver", "-C", pkgPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to make at %s: %v: %v", pkgPath, string(out), err)
	}

	// Copy binaries
	if _, err := os.Stat(binPath); err != nil {
		return fmt.Errorf("failed to locate test binary %s: %v", binPath, err)
	}
	out, err = exec.Command("cp", binPath, tarDir).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to copy %q: %v Output: %q", binPath, err, out)
	}

	return nil
}
