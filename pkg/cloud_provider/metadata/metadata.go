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
	"strings"

	"cloud.google.com/go/compute/metadata"
	"github.com/golang/glog"
)

type Service interface {
	GetZone() string
	GetProject() string
	GetNetwork() string
}

type metadataServiceManager struct {
	// Current zone the driver is running in
	zone    string
	project string
	network string
}

const (
	MetadataNetworkSuffix = "instance/network-interfaces/0/network"
)

var _ Service = &metadataServiceManager{}

func NewMetadataService() (Service, error) {
	zone, err := metadata.Zone()
	if err != nil {
		return nil, fmt.Errorf("failed to get current zone: %v", err)
	}
	projectID, err := metadata.ProjectID()
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %v", err)
	}

	network, err := getNetwork()
	if err != nil {
		glog.Warningf("Failed to fetch network name for the instance running driver controller")
	}

	return &metadataServiceManager{
		project: projectID,
		zone:    zone,
		network: network,
	}, nil
}

func (manager *metadataServiceManager) GetZone() string {
	return manager.zone
}

func (manager *metadataServiceManager) GetProject() string {
	return manager.project
}

func (manager *metadataServiceManager) GetNetwork() string {
	return manager.network
}

func getNetwork() (string, error) {
	s, err := metadata.Get(MetadataNetworkSuffix)
	// network returned has the complete path e.g. projects/<Project-Id>/networks/<Network-Name>
	if err == nil {
		s = strings.TrimSpace(s)
		s = s[strings.LastIndex(s, "/")+1:]
		glog.Infof("Found network %v", s)
		return s, nil
	}
	return "", err
}
