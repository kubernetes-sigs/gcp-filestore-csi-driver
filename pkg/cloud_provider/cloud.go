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

package cloud

import (
	"fmt"

	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/file"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/metadata"
)

type Cloud struct {
	File file.Service
	Meta metadata.Service
}

func NewCloud(version string) (*Cloud, error) {
	file, err := file.NewGCFSService(version)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Filestore service: %v", err)
	}
	meta, err := metadata.NewMetadataService()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Metadata service: %v", err)
	}

	return &Cloud{
		File: file,
		Meta: meta,
	}, nil
}
