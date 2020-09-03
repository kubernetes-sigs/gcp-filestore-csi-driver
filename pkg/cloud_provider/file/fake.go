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

package file

import (
	"context"
	"fmt"

	"google.golang.org/api/googleapi"
)

type fakeServiceManager struct {
	createdInstances map[string]*ServiceInstance
}

var _ Service = &fakeServiceManager{}

func NewFakeService() (Service, error) {
	return &fakeServiceManager{
		createdInstances: map[string]*ServiceInstance{},
	}, nil
}

func (manager *fakeServiceManager) CreateInstance(ctx context.Context, obj *ServiceInstance) (*ServiceInstance, error) {
	instance := &ServiceInstance{
		Project:  "test-project",
		Location: "test-location",
		Name:     obj.Name,
		Tier:     obj.Tier,
		Volume: Volume{
			Name:      obj.Volume.Name,
			SizeBytes: obj.Volume.SizeBytes,
		},
		Network: Network{
			Name:            obj.Network.Name,
			Ip:              "test-ip",
			ReservedIpRange: obj.Network.ReservedIpRange,
		},
		Labels: obj.Labels,
	}

	manager.createdInstances[obj.Name] = instance
	return instance, nil
}

func (manager *fakeServiceManager) DeleteInstance(ctx context.Context, obj *ServiceInstance) error {
	return nil
}

func (manager *fakeServiceManager) GetInstance(ctx context.Context, obj *ServiceInstance) (*ServiceInstance, error) {
	instance, exists := manager.createdInstances[obj.Name]
	if exists {
		return instance, nil
	}
	return nil, &googleapi.Error{
		Errors: []googleapi.ErrorItem{
			{
				Reason: "notFound",
			},
		},
	}
}

func (manager *fakeServiceManager) ListInstances(ctx context.Context, obj *ServiceInstance) ([]*ServiceInstance, error) {
	instances := []*ServiceInstance{
		{
			Project:  "test-project",
			Location: "test-location",
			Name:     "test",
			Tier:     "test_tier",
			Network: Network{
				ReservedIpRange: "192.168.92.32/29",
			},
		},
		{
			Project:  "test-project",
			Location: "test-location",
			Name:     "test",
			Tier:     "test_tier",
			Network: Network{
				ReservedIpRange: "192.168.92.40/29",
			},
		},
	}
	return instances, nil
}

func (manager *fakeServiceManager) ResizeInstance(ctx context.Context, obj *ServiceInstance) (*ServiceInstance, error) {
	instance, ok := manager.createdInstances[obj.Name]
	if !ok {
		return nil, fmt.Errorf("Instance %v not found", obj.Name)
	}

	instance.Volume.SizeBytes = obj.Volume.SizeBytes
	manager.createdInstances[obj.Name] = instance
	return instance, nil
}
