/*
Copyright 2022 The Kubernetes Authors.

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

package driver

import (
	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"
	cloud "sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/file"
)

// MultishareController handles CSI calls for volumes which use Filestore multishare instances.
type MultishareController struct {
	driver      *GCFSDriver
	fileService file.Service
	cloud       *cloud.Cloud
	opsManager  *MultishareOpsManager
}

func NewMultishareController(driver *GCFSDriver, fileService file.Service, cloud *cloud.Cloud) *MultishareController {
	return &MultishareController{
		opsManager:  NewMultishareOpsManager(fileService, cloud),
		driver:      driver,
		fileService: fileService,
		cloud:       cloud,
	}
}

func (m *MultishareController) Run() {
	m.opsManager.Run()
}

func (m *MultishareController) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	// Handle higher level csi params validation, try locks
	// Prepare instacne and initiate instance workflow by calling Multishare OpsManager functions
	// Initiate share workflow by calling Multishare OpsManager functions
	// Prepare and return csi response
	return nil, nil
}

func (m *MultishareController) DeleteVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	// Handle higher level csi params validation, try locks
	// Initiate share workflow by calling Multishare OpsManager functions
	// Prepare and return csi response
	return nil, nil
}

func (m *MultishareController) ControllerExpandVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	// Handle higher level csi params validation, try locks
	// Initiate share workflow by calling Multishare OpsManager functions
	// Prepare and return csi response
	return nil, nil
}
