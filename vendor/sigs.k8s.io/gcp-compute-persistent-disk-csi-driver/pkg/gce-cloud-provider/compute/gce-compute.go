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

package gcecloudprovider

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang/glog"
	"golang.org/x/net/context"
	compute "google.golang.org/api/compute/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/gcp-compute-persistent-disk-csi-driver/pkg/common"
)

type GCECompute interface {
	// Getters
	GetProject() string
	GetZone() string
	// Disk Methods
	GetDiskOrError(ctx context.Context, volumeZone, volumeName string) (*compute.Disk, error)
	GetAndValidateExistingDisk(ctx context.Context, configuredZone, name, diskType string, reqBytes, limBytes int64) (exists bool, err error)
	InsertDisk(ctx context.Context, zone string, diskToCreate *compute.Disk) (*compute.Operation, error)
	DeleteDisk(ctx context.Context, zone, name string) (*compute.Operation, error)
	AttachDisk(ctx context.Context, zone, instanceName string, attachedDisk *compute.AttachedDisk) (*compute.Operation, error)
	DetachDisk(ctx context.Context, volumeZone, instanceName, volumeName string) (*compute.Operation, error)
	GetDiskSourceURI(disk *compute.Disk, zone string) string
	GetDiskTypeURI(zone, diskType string) string
	WaitForAttach(ctx context.Context, zone, diskName, instanceName string) error
	// Instance Methods
	GetInstanceOrError(ctx context.Context, instanceZone, instanceName string) (*compute.Instance, error)
	// Operation Methods
	WaitForOp(ctx context.Context, op *compute.Operation, zone string) error
}

func (cloud *CloudProvider) GetProject() string {
	return cloud.project
}

func (cloud *CloudProvider) GetZone() string {
	return cloud.zone
}

func (cloud *CloudProvider) GetDiskOrError(ctx context.Context, volumeZone, volumeName string) (*compute.Disk, error) {
	svc := cloud.service
	project := cloud.project
	glog.Infof("Getting disk %v from zone %v", volumeName, volumeZone)
	disk, err := svc.Disks.Get(project, volumeZone, volumeName).Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	glog.Infof("Got disk %v from zone %v", volumeName, volumeZone)
	return disk, nil
}

func (cloud *CloudProvider) GetAndValidateExistingDisk(ctx context.Context, configuredZone, name, diskType string, reqBytes, limBytes int64) (exists bool, err error) {
	svc := cloud.service
	project := cloud.project
	resp, err := svc.Disks.Get(project, configuredZone, name).Context(ctx).Do()
	if err != nil {
		if IsGCEError(err, "notFound") {
			glog.Infof("Disk %v does not already exist. Continuing with creation.", name)
		} else {
			glog.Warningf("Unknown disk GET error: %v", err)
		}
	}

	if resp != nil {
		// Disk already exists
		requestValid := common.GbToBytes(resp.SizeGb) >= reqBytes && reqBytes != 0
		responseValid := common.GbToBytes(resp.SizeGb) <= limBytes && limBytes != 0
		if !requestValid || !responseValid {
			return true, status.Error(codes.AlreadyExists, fmt.Sprintf(
				"Disk already exists with incompatible capacity. Need %v (Required) < %v (Existing) < %v (Limit)",
				reqBytes, common.GbToBytes(resp.SizeGb), limBytes))
		}

		respType := strings.Split(resp.Type, "/")
		typeMatch := respType[len(respType)-1] != diskType
		typeDefault := diskType == "" && respType[len(respType)-1] == "pd-standard"
		if !typeMatch && !typeDefault {
			return true, status.Error(codes.AlreadyExists, fmt.Sprintf(
				"Disk already exists with incompatible type. Need %v. Got %v",
				diskType, respType[len(respType)-1]))
		}

		// Volume exists with matching name, capacity, type.
		glog.Infof("Compatible disk already exists. Reusing existing.")
		return true, nil
	}

	return false, nil
}

func (cloud *CloudProvider) InsertDisk(ctx context.Context, zone string, diskToCreate *compute.Disk) (*compute.Operation, error) {
	return cloud.service.Disks.Insert(cloud.project, zone, diskToCreate).Context(ctx).Do()
}

func (cloud *CloudProvider) DeleteDisk(ctx context.Context, zone, name string) (*compute.Operation, error) {
	return cloud.service.Disks.Delete(cloud.project, zone, name).Context(ctx).Do()
}

func (cloud *CloudProvider) AttachDisk(ctx context.Context, zone, instanceName string, attachedDisk *compute.AttachedDisk) (*compute.Operation, error) {
	return cloud.service.Instances.AttachDisk(cloud.project, zone, instanceName, attachedDisk).Context(ctx).Do()
}

func (cloud *CloudProvider) DetachDisk(ctx context.Context, volumeZone, instanceName, volumeName string) (*compute.Operation, error) {
	return cloud.service.Instances.DetachDisk(cloud.project, volumeZone, instanceName, volumeName).Context(ctx).Do()
}

func (cloud *CloudProvider) GetDiskSourceURI(disk *compute.Disk, zone string) string {
	projectsApiEndpoint := gceComputeAPIEndpoint + "projects/"
	if cloud.service != nil {
		projectsApiEndpoint = cloud.service.BasePath
	}

	return projectsApiEndpoint + fmt.Sprintf(
		diskSourceURITemplateSingleZone,
		cloud.project,
		zone,
		disk.Name)
}

func (cloud *CloudProvider) GetDiskTypeURI(zone, diskType string) string {
	return fmt.Sprintf(diskTypeURITemplateSingleZone, cloud.project, zone, diskType)
}

func (cloud *CloudProvider) WaitForOp(ctx context.Context, op *compute.Operation, zone string) error {
	svc := cloud.service
	project := cloud.project
	// TODO: Double check that these timeouts are reasonable
	return wait.Poll(3*time.Second, 5*time.Minute, func() (bool, error) {
		pollOp, err := svc.ZoneOperations.Get(project, zone, op.Name).Context(ctx).Do()
		if err != nil {
			glog.Errorf("WaitForOp(op: %#v, zone: %#v) failed to poll the operation", op, zone)
			return false, err
		}
		done := opIsDone(pollOp)
		return done, err
	})
}

func (cloud *CloudProvider) WaitForAttach(ctx context.Context, zone, diskName, instanceName string) error {
	return wait.Poll(5*time.Second, 2*time.Minute, func() (bool, error) {
		disk, err := cloud.GetDiskOrError(ctx, zone, diskName)
		if err != nil {
			glog.Errorf("GetDiskOrError failed to get disk: %v", err)
			return false, err
		}

		if disk == nil {
			return false, fmt.Errorf("Disk %v could not be found in zone %v", diskName, zone)
		}

		for _, user := range disk.Users {
			if strings.Contains(user, instanceName) && strings.Contains(user, zone) {
				return true, nil
			}
		}
		return false, nil
	})
}

func opIsDone(op *compute.Operation) bool {
	return op != nil && op.Status == "DONE"
}

func (cloud *CloudProvider) GetInstanceOrError(ctx context.Context, instanceZone, instanceName string) (*compute.Instance, error) {
	svc := cloud.service
	project := cloud.project
	glog.Infof("Getting instance %v from zone %v", instanceName, instanceZone)
	instance, err := svc.Instances.Get(project, instanceZone, instanceName).Do()
	if err != nil {
		return nil, err
	}
	glog.Infof("Got instance %v from zone %v", instanceName, instanceZone)
	return instance, nil
}
