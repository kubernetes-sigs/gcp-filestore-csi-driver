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

	filev1beta1 "google.golang.org/api/file/v1beta1"
	"google.golang.org/api/googleapi"
)

const (
	defaultProject    = "test-project"
	defaultZone       = "us-central1-c"
	defaultTier       = "BASIC_HDD"
	defaultCapacityGb = 1024
)

type fakeServiceManager struct {
	createdInstances map[string]*ServiceInstance
	backups          map[string]*BackupInfo
}

var _ Service = &fakeServiceManager{}

func NewFakeService() (Service, error) {
	return &fakeServiceManager{
		createdInstances: map[string]*ServiceInstance{},
		backups:          map[string]*BackupInfo{},
	}, nil
}

func (manager *fakeServiceManager) CreateInstance(ctx context.Context, obj *ServiceInstance) (*ServiceInstance, error) {
	instance := &ServiceInstance{
		Project:  defaultProject,
		Location: defaultZone,
		Name:     obj.Name,
		Tier:     obj.Tier,
		Volume: Volume{
			Name:      obj.Volume.Name,
			SizeBytes: obj.Volume.SizeBytes,
		},
		Network: Network{
			Name:            obj.Network.Name,
			Ip:              "1.1.1.1",
			ReservedIpRange: obj.Network.ReservedIpRange,
		},
		Labels: obj.Labels,
		State:  "READY",
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
			Project:  defaultProject,
			Location: defaultZone,
			Name:     "test",
			Tier:     defaultTier,
			Network: Network{
				ReservedIpRange: "192.168.92.32/29",
			},
			State: "READY",
		},
		{
			Project:  defaultProject,
			Location: defaultZone,
			Name:     "test",
			Tier:     defaultTier,
			Network: Network{
				ReservedIpRange: "192.168.92.40/29",
			},
			State: "READY",
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

func (manager *fakeServiceManager) CreateBackup(ctx context.Context, obj *ServiceInstance, backupName string, backupLocation string) (*filev1beta1.Backup, error) {
	backupUri, _, err := CreateBackpURI(obj, backupName, backupLocation)
	if err != nil {
		return nil, err
	}

	backupSource := fmt.Sprintf("projects/%s/locations/%s/instances/%s", obj.Project, obj.Location, obj.Name)
	if backupInfo, ok := manager.backups[backupUri]; ok {
		if backupInfo.SourceVolumeHandle != backupSource {
			return nil, fmt.Errorf("Mismatch in source volume handle for existing snapshot")
		}
		return backupInfo.Backup, nil
	}

	backupToCreate := &filev1beta1.Backup{
		Name:            backupUri,
		SourceFileShare: obj.Volume.Name,
		SourceInstance:  backupSource,
		CreateTime:      "2020-10-02T15:01:23Z",
		State:           "READY",
		CapacityGb:      defaultCapacityGb,
	}
	manager.backups[backupUri] = &BackupInfo{
		Backup:             backupToCreate,
		SourceVolumeHandle: backupSource,
	}
	return backupToCreate, nil
}

func (manager *fakeServiceManager) DeleteBackup(ctx context.Context, backupName string) error {
	delete(manager.backups, backupName)
	return nil
}

func (manager *fakeServiceManager) GetBackup(ctx context.Context, backupUri string) (*BackupInfo, error) {
	backupInfo, ok := manager.backups[backupUri]
	if !ok || backupInfo.Backup == nil {
		return nil, notFoundError()
	}

	return backupInfo, nil
}

func (manager *fakeServiceManager) CreateInstanceFromBackupSource(ctx context.Context, obj *ServiceInstance, sourceSnapshotId string) (*ServiceInstance, error) {
	instance := &ServiceInstance{
		Project:  defaultProject,
		Location: defaultZone,
		Name:     obj.Name,
		Tier:     obj.Tier,
		Volume: Volume{
			Name:      obj.Volume.Name,
			SizeBytes: obj.Volume.SizeBytes,
		},
		Network: Network{
			Name:            obj.Network.Name,
			Ip:              "1.1.1.1",
			ReservedIpRange: obj.Network.ReservedIpRange,
		},
		Labels: obj.Labels,
		State:  "READY",
	}

	manager.createdInstances[obj.Name] = instance
	return instance, nil
}

func notFoundError() *googleapi.Error {
	return &googleapi.Error{
		Errors: []googleapi.ErrorItem{
			{
				Reason: "notFound",
			},
		},
	}
}

type fakeBlockingServiceManager struct {
	*fakeServiceManager
	// 'OperationUnblocker' channel is used to block the execution of the respective function using it. This is done by sending a channel of empty struct over 'OperationUnblocker' channel, and wait until the tester gives a go-ahead to proceed further in the execution of the function.
	OperationUnblocker chan chan struct{}
}

func NewFakeBlockingService(operationUnblocker chan chan struct{}) (Service, error) {
	return &fakeBlockingServiceManager{
		fakeServiceManager: &fakeServiceManager{
			createdInstances: map[string]*ServiceInstance{},
			backups:          map[string]*BackupInfo{},
		},
		OperationUnblocker: operationUnblocker,
	}, nil
}

func (m *fakeBlockingServiceManager) CreateInstance(ctx context.Context, obj *ServiceInstance) (*ServiceInstance, error) {
	execute := make(chan struct{})
	m.OperationUnblocker <- execute
	<-execute
	return m.fakeServiceManager.CreateInstance(ctx, obj)
}

func (m *fakeBlockingServiceManager) DeleteInstance(ctx context.Context, obj *ServiceInstance) error {
	execute := make(chan struct{})
	m.OperationUnblocker <- execute
	<-execute
	return m.fakeServiceManager.DeleteInstance(ctx, obj)
}
