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
)

func NewFakeCloud() (*Cloud, error) {
	file, err := file.NewFakeService()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Filestore service: %w", err)
	}

	return &Cloud{
		File:    file,
		Project: "test-project",
		Zone:    "us-central1-c",
	}, nil
}

func NewFakeCloudWithFiler(filer file.Service, project, location string) (*Cloud, error) {
	return &Cloud{
		File:    filer,
		Project: project,
		Zone:    location,
	}, nil
}
