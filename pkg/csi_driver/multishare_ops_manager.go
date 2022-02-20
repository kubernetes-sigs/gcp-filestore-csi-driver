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
	cloud "sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/file"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/util"
)

// MultishareOpsManager manages storage class cache, manages the lifecycle of all instance and share operations.
type MultishareOpsManager struct {
	cache       *util.StorageClassInfoMap
	fileService file.Service
	cloud       *cloud.Cloud
}

func NewMultishareOpsManager(fileService file.Service, cloud *cloud.Cloud) *MultishareOpsManager {
	return &MultishareOpsManager{
		cache:       util.NewStorageClassInfoMap(),
		fileService: fileService,
		cloud:       cloud,
	}
}

func (m *MultishareOpsManager) Run() {
	// TODO: Start periodic cache hydration
	// TODO: Start periodic instance inspection for delete and shrink
}
