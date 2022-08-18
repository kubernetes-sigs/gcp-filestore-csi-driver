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
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	filev1beta1 "google.golang.org/api/file/v1beta1"
	filev1beta1multishare "google.golang.org/api/file/v1beta1"
	"google.golang.org/api/googleapi"
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/grpc/codes"
)

const (
	defaultProject    = "test-project"
	defaultZone       = "us-central1-c"
	defaultRegion     = "us-central1"
	defaultTier       = "BASIC_HDD"
	defaultCapacityGb = 1024
)

type fakeServiceManager struct {
	createdInstances          map[string]*ServiceInstance
	backups                   map[string]*BackupInfo
	createdMultishareInstance map[string]*MultishareInstance
	createdMultishares        map[string]*Share
	multishareops             []*filev1beta1multishare.Operation
}

var _ Service = &fakeServiceManager{}

func NewFakeService() (Service, error) {
	return &fakeServiceManager{
		createdInstances:          map[string]*ServiceInstance{},
		backups:                   map[string]*BackupInfo{},
		createdMultishareInstance: make(map[string]*MultishareInstance),
		createdMultishares:        make(map[string]*Share),
	}, nil
}

func NewFakeServiceForMultishare(instances []*MultishareInstance, shares []*Share, ops []*filev1beta1multishare.Operation) (Service, error) {
	s := &fakeServiceManager{
		createdInstances:          map[string]*ServiceInstance{},
		backups:                   map[string]*BackupInfo{},
		createdMultishareInstance: make(map[string]*MultishareInstance),
		createdMultishares:        make(map[string]*Share),
		multishareops:             make([]*filev1beta1multishare.Operation, 0),
	}

	for _, instance := range instances {
		s.createdMultishareInstance[instance.Name] = instance
	}
	for _, share := range shares {
		s.createdMultishares[share.Name] = share
	}
	s.multishareops = append(s.multishareops, ops...)
	return s, nil
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

func (m *fakeServiceManager) HasOperations(ctx context.Context, obj *ServiceInstance, operationType string, done bool) (bool, error) {
	return false, nil
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
	OperationUnblocker  chan chan struct{}
	MultishareUnblocker chan chan Signal
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

func (m *fakeBlockingServiceManager) HasOperations(ctx context.Context, obj *ServiceInstance, operationType string, done bool) (bool, error) {
	return false, nil
}

// Multishare fake functions defined here
func (manager *fakeServiceManager) GetMultishareInstance(ctx context.Context, obj *MultishareInstance) (*MultishareInstance, error) {
	instance, ok := manager.createdMultishareInstance[obj.Name]
	if !ok {
		return nil, &googleapi.Error{
			Code: int(code.Code_NOT_FOUND),
			Errors: []googleapi.ErrorItem{
				{
					Reason: "notFound",
				},
			},
		}
	}
	return instance, nil
}

func (manager *fakeServiceManager) ListMultishareInstances(ctx context.Context, filter *ListFilter) ([]*MultishareInstance, error) {
	var ilist []*MultishareInstance
	for _, v := range manager.createdMultishareInstance {
		ilist = append(ilist, v)
	}
	return ilist, nil
}

func (manager *fakeServiceManager) StartCreateMultishareInstanceOp(ctx context.Context, obj *MultishareInstance) (*filev1beta1multishare.Operation, error) {
	instance := &MultishareInstance{
		Project:       defaultProject,
		Location:      obj.Location,
		Name:          obj.Name,
		Tier:          obj.Tier,
		CapacityBytes: obj.CapacityBytes,
		Network:       obj.Network,
		KmsKeyName:    obj.KmsKeyName,
		Labels:        obj.Labels,
		State:         "READY",
	}
	manager.createdMultishareInstance[obj.Name] = instance
	meta := &filev1beta1multishare.OperationMetadata{
		Target: fmt.Sprintf(instanceURIFmt, instance.Project, instance.Location, instance.Name),
		Verb:   "create",
	}
	metaBytes, _ := json.Marshal(meta)
	op := &filev1beta1multishare.Operation{
		Name:     "operation-" + uuid.New().String(),
		Metadata: metaBytes,
	}
	return op, nil
}

type Signal struct {
	ReportError             bool
	ReportNotFoundError     bool
	ReportRunning           bool
	ReportOpWithErrorStatus bool
}

func (manager *fakeServiceManager) StartDeleteMultishareInstanceOp(ctx context.Context, obj *MultishareInstance) (*filev1beta1multishare.Operation, error) {
	delete(manager.createdMultishareInstance, obj.Name)
	meta := &filev1beta1multishare.OperationMetadata{
		Target: fmt.Sprintf(instanceURIFmt, obj.Project, obj.Location, obj.Name),
		Verb:   "create",
	}
	metaBytes, _ := json.Marshal(meta)
	op := &filev1beta1multishare.Operation{
		Name:     "operation-" + uuid.New().String(),
		Metadata: metaBytes,
	}
	return op, nil
}

func (manager *fakeServiceManager) StartResizeMultishareInstanceOp(ctx context.Context, obj *MultishareInstance) (*filev1beta1multishare.Operation, error) {
	manager.createdMultishareInstance[obj.Name].CapacityBytes = obj.CapacityBytes
	meta := &filev1beta1multishare.OperationMetadata{
		Target: fmt.Sprintf(instanceURIFmt, obj.Project, obj.Location, obj.Name),
		Verb:   "update",
	}
	metaBytes, _ := json.Marshal(meta)
	op := &filev1beta1multishare.Operation{
		Name:     "operation-" + uuid.New().String(),
		Metadata: metaBytes,
	}
	return op, nil
}

func (manager *fakeServiceManager) StartCreateShareOp(ctx context.Context, obj *Share) (*filev1beta1multishare.Operation, error) {
	if _, ok := manager.createdMultishareInstance[obj.Parent.Name]; !ok {
		return nil, fmt.Errorf("host instance %s not found", obj.Parent.Name)
	}

	parent := &MultishareInstance{
		Project:       obj.Parent.Project,
		Location:      obj.Parent.Location,
		Name:          obj.Parent.Name,
		Tier:          obj.Parent.Tier,
		CapacityBytes: obj.CapacityBytes,
		Network: Network{
			Name:            obj.Parent.Network.Name,
			Ip:              obj.Parent.Network.Ip,
			ReservedIpRange: obj.Parent.Network.ReservedIpRange,
		},
		Labels: obj.Parent.Labels,
		State:  "READY",
	}
	share := &Share{
		Name:           obj.Name,
		Parent:         parent,
		CapacityBytes:  obj.CapacityBytes,
		Labels:         obj.Labels,
		MountPointName: obj.Name,
		State:          "READY",
	}
	manager.createdMultishares[share.Name] = share

	meta := &filev1beta1.OperationMetadata{
		Target: fmt.Sprintf(shareURIFmt, share.Parent.Project, share.Parent.Location, share.Parent.Name, share.Name),
		Verb:   "create",
	}
	metaBytes, _ := json.Marshal(meta)
	op := &filev1beta1multishare.Operation{
		Name:     "operation-" + uuid.New().String(),
		Metadata: metaBytes,
	}

	return op, nil
}

func (manager *fakeServiceManager) StartDeleteShareOp(ctx context.Context, obj *Share) (*filev1beta1multishare.Operation, error) {
	delete(manager.createdMultishares, obj.Name)

	meta := &filev1beta1multishare.OperationMetadata{
		Target: fmt.Sprintf(shareURIFmt, obj.Parent.Project, obj.Parent.Location, obj.Parent.Name, obj.Name),
		Verb:   "DELETE",
	}
	metaBytes, _ := json.Marshal(meta)
	op := &filev1beta1multishare.Operation{
		Name:     "operation-" + uuid.New().String(),
		Metadata: metaBytes,
	}

	return op, nil
}

func (manager *fakeServiceManager) StartResizeShareOp(ctx context.Context, obj *Share) (*filev1beta1multishare.Operation, error) {
	manager.createdMultishares[obj.Name].CapacityBytes = obj.CapacityBytes
	meta := &filev1beta1multishare.OperationMetadata{
		Target: fmt.Sprintf(shareURIFmt, obj.Parent.Project, obj.Parent.Location, obj.Parent.Name, obj.Name),
		Verb:   "update",
	}
	metaBytes, _ := json.Marshal(meta)
	op := &filev1beta1multishare.Operation{
		Name:     "operation-" + uuid.New().String(),
		Metadata: metaBytes,
	}

	return op, nil
}

func (manager *fakeServiceManager) WaitForOpWithOpts(ctx context.Context, op string, opts PollOpts) error {
	return nil
}

func (manager *fakeServiceManager) GetOp(ctx context.Context, opName string) (*filev1beta1multishare.Operation, error) {
	op := &filev1beta1multishare.Operation{
		Name: opName,
		Done: true,
	}
	return op, nil
}

func (manager *fakeServiceManager) IsOpDone(*filev1beta1multishare.Operation) (bool, error) {
	return true, nil
}

func (manager *fakeServiceManager) GetShare(ctx context.Context, obj *Share) (*Share, error) {
	share, ok := manager.createdMultishares[obj.Name]
	if !ok {
		return nil, notFoundError()
	}
	return share, nil
}

func (manager *fakeServiceManager) ListShares(ctx context.Context, filter *ListFilter) ([]*Share, error) {
	var slist []*Share
	for _, v := range manager.createdMultishares {
		slist = append(slist, v)
	}
	return slist, nil
}

func (manager *fakeServiceManager) AddMultishareOps(ops []*filev1beta1multishare.Operation) {
	manager.multishareops = append(manager.multishareops, ops...)
}

func (manager *fakeServiceManager) ListOps(ctx context.Context, resource *ListFilter) ([]*filev1beta1multishare.Operation, error) {
	return manager.multishareops, nil
}

func NewFakeBlockingServiceForMultishare(unblocker chan chan Signal) (Service, error) {
	return &fakeBlockingServiceManager{
		fakeServiceManager: &fakeServiceManager{
			createdMultishareInstance: make(map[string]*MultishareInstance),
			createdMultishares:        make(map[string]*Share),
		},
		MultishareUnblocker: unblocker,
	}, nil
}

func (manager *fakeBlockingServiceManager) GetMultishareInstance(ctx context.Context, instance *MultishareInstance) (*MultishareInstance, error) {
	execute := make(chan Signal)
	manager.MultishareUnblocker <- execute
	val := <-execute
	if val.ReportError {
		return nil, fmt.Errorf("mock error")
	}
	if val.ReportNotFoundError {
		return nil, notFoundError()
	}
	return manager.fakeServiceManager.GetMultishareInstance(ctx, instance)
}

func (manager *fakeBlockingServiceManager) GetOp(ctx context.Context, opName string) (*filev1beta1multishare.Operation, error) {
	execute := make(chan Signal)
	manager.MultishareUnblocker <- execute
	val := <-execute
	if val.ReportError {
		return nil, fmt.Errorf("mock error")
	}

	op := &filev1beta1multishare.Operation{
		Name: opName,
		Done: true,
	}
	if val.ReportOpWithErrorStatus {
		op.Error = &filev1beta1multishare.Status{Code: int64(codes.Internal)}
		return op, nil
	}

	if val.ReportRunning {
		op.Done = false
	}
	return op, nil
}

func (manager *fakeBlockingServiceManager) IsOpDone(*filev1beta1multishare.Operation) (bool, error) {
	execute := make(chan Signal)
	manager.MultishareUnblocker <- execute
	val := <-execute
	if val.ReportError {
		return !val.ReportRunning, fmt.Errorf("mock error")
	}

	return !val.ReportRunning, nil
}
