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

package metadata

import (
	"fmt"

	"cloud.google.com/go/compute/metadata"
)

type Service interface {
	GetZone() string
	GetProject() string
	GetInternalIP() string
	GetInstanceID() string
}

type metadataServiceManager struct {
	// Current zone the driver is running in
	zone       string
	project    string
	instanceID string
	internalIP string
}

var _ Service = &metadataServiceManager{}

func NewMetadataService() (Service, error) {
	zone, err := metadata.Zone()
	if err != nil {
		return nil, fmt.Errorf("failed to get current zone: %w", err)
	}
	projectID, err := metadata.ProjectID()
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}
	instanceID, err := metadata.InstanceID()
	if err != nil {
		return nil, fmt.Errorf("failed to get instance id: %w", err)
	}
	internalIP, err := metadata.InternalIP()
	if err != nil {
		return nil, fmt.Errorf("failed to get internal IP: %w", err)
	}

	return &metadataServiceManager{
		project:    projectID,
		zone:       zone,
		instanceID: instanceID,
		internalIP: internalIP,
	}, nil
}

func (manager *metadataServiceManager) GetZone() string {
	return manager.zone
}

func (manager *metadataServiceManager) GetProject() string {
	return manager.project
}

func (manager *metadataServiceManager) GetInstanceID() string {
	return manager.instanceID
}

func (manager *metadataServiceManager) GetInternalIP() string {
	return manager.internalIP
}
