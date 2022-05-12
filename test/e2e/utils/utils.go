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

package utils

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path"
	"time"

	"golang.org/x/oauth2/google"
	cloudresourcemanager "google.golang.org/api/cloudresourcemanager/v1"
	"k8s.io/klog"
	boskosclient "sigs.k8s.io/boskos/client"
	remote "sigs.k8s.io/gcp-filestore-csi-driver/test/remote"
)

var (
	boskos, _ = boskosclient.NewClient(os.Getenv("JOB_NAME"), "http://boskos", "", "")
)

func GCFSClientAndDriverSetup(instance *remote.InstanceInfo) (*remote.TestContext, error) {
	port := fmt.Sprintf("%v", 1024+rand.Intn(10000))
	goPath, ok := os.LookupEnv("GOPATH")
	if !ok {
		return nil, fmt.Errorf("Could not find environment variable GOPATH")
	}
	pkgPath := path.Join(goPath, "src/sigs.k8s.io/gcp-filestore-csi-driver/")
	binPath := path.Join(pkgPath, "bin/gcp-filestore-csi-driver")

	// Install NFS Libraries
	_, err := instance.SSH("apt-get", "install", "-y", "nfs-common")
	if err != nil {
		return nil, err
	}
	endpoint := fmt.Sprintf("tcp://localhost:%s", port)

	workspace := remote.NewWorkspaceDir("gcfs-csi-e2e-")
	driverRunCmd := fmt.Sprintf("sh -c '/usr/bin/nohup %s/gcp-filestore-csi-driver --endpoint=%s --nodeid=%s --controller=true --node=true > %s/prog.out 2> %s/prog.err < /dev/null &'",
		workspace, endpoint, instance.GetName(), workspace, workspace)

	config := &remote.ClientConfig{
		PkgPath:      pkgPath,
		BinPath:      binPath,
		WorkspaceDir: workspace,
		RunDriverCmd: driverRunCmd,
		Port:         port,
	}

	return remote.SetupNewDriverAndClient(instance, config)
}

func SetupProwConfig(resourceType string) (project, serviceAccount string) {
	// Try to get a Boskos project
	klog.V(4).Infof("Running in PROW")
	klog.V(4).Infof("Fetching a Boskos loaned project")

	p, err := boskos.Acquire(resourceType, "free", "busy")
	if err != nil {
		klog.Fatalf("boskos failed to acquire project: %v", err)
	}

	if p == nil {
		klog.Fatalf("boskos does not have a free gce-project at the moment")
	}

	project = p.Name

	go func(c *boskosclient.Client, proj string) {
		for range time.Tick(time.Minute * 5) {
			if err := c.UpdateOne(p.Name, "busy", nil); err != nil {
				klog.Warningf("[Boskos] Update %v failed with %v", p, err)
			}
		}
	}(boskos, p.Name)

	// If we're on CI overwrite the service account
	klog.V(4).Infof("Fetching the default compute service account")

	c, err := google.DefaultClient(context.TODO(), cloudresourcemanager.CloudPlatformScope)
	if err != nil {
		klog.Fatalf("Failed to get Google Default Client: %v", err)
	}

	cloudresourcemanagerService, err := cloudresourcemanager.New(c)
	if err != nil {
		klog.Fatalf("Failed to create new cloudresourcemanager: %v", err)
	}

	resp, err := cloudresourcemanagerService.Projects.Get(project).Do()
	if err != nil {
		klog.Fatalf("Failed to get project %v from Cloud Resource Manager: %v", project, err)
	}

	// Default Compute Engine service account
	// [PROJECT_NUMBER]-compute@developer.gserviceaccount.com
	serviceAccount = fmt.Sprintf("%v-compute@developer.gserviceaccount.com", resp.ProjectNumber)
	klog.Infof("Prow config utilizing:\n- project %q\n- project number %q\n- service account %q", project, resp.ProjectNumber, serviceAccount)
	return project, serviceAccount
}

func ForceChmod(instance *remote.InstanceInfo, filePath string, perms string) error {
	originalumask, err := instance.SSHNoSudo("umask")
	if err != nil {
		return fmt.Errorf("failed to umask. Output: %v, errror: %v", originalumask, err)
	}
	output, err := instance.SSHNoSudo("umask", "0000")
	if err != nil {
		return fmt.Errorf("failed to umask. Output: %v, errror: %v", output, err)
	}
	output, err = instance.SSH("chmod", "-R", perms, filePath)
	if err != nil {
		return fmt.Errorf("failed to chmod file %s. Output: %v, errror: %v", filePath, output, err)
	}
	output, err = instance.SSHNoSudo("umask", originalumask)
	if err != nil {
		return fmt.Errorf("failed to umask. Output: %v, errror: %v", output, err)
	}
	return nil
}

func WriteFile(instance *remote.InstanceInfo, filePath, fileContents string) error {
	output, err := instance.SSHNoSudo("echo", fileContents, ">", filePath)
	if err != nil {
		return fmt.Errorf("failed to write test file %s. Output: %v, errror: %v", filePath, output, err)
	}
	return nil
}

// generate a random file with given size in MiB
func GenerateRandomFile(instance *remote.InstanceInfo, filePath string, size int64) error {
	output, err := instance.SSHNoSudo("dd", "if=/dev/urandom", fmt.Sprintf("of=%s/file", filePath), "bs=1MiB", fmt.Sprintf("count=%d", size))
	if err != nil {
		return fmt.Errorf("failed to write test file %s. Output: %v, errror: %v", filePath, output, err)
	}
	return nil
}

func ReadFile(instance *remote.InstanceInfo, filePath string) (string, error) {
	output, err := instance.SSHNoSudo("cat", filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read test file %s. Output: %v, errror: %v", filePath, output, err)
	}
	return output, nil
}

func MkdirAll(instance *remote.InstanceInfo, dir string) error {
	output, err := instance.SSH("mkdir", "-p", dir)
	if err != nil {
		return fmt.Errorf("failed to mkdir -p %s. Output: %v, errror: %v", dir, output, err)
	}
	return nil
}

func RmAll(instance *remote.InstanceInfo, filePath string) error {
	output, err := instance.SSH("rm", "-rf", filePath)
	if err != nil {
		return fmt.Errorf("failed to delete all %s. Output: %v, errror: %v", filePath, output, err)
	}
	return nil
}
