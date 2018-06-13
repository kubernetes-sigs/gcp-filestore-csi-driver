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
	"runtime"
	"strings"

	"github.com/golang/glog"
	"google.golang.org/api/googleapi"

	beta "sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/generated/file/v1beta1"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/util"
)

type Instance struct {
	Project  string
	Name     string
	Location string
	Tier     string
	Network  Network
	Volume   Volume
}

type Volume struct {
	Name      string
	SizeBytes int64
}

type Network struct {
	Name            string
	ReservedIpRange string
	Ip              string
}

type Service interface {
	CreateInstance(ctx context.Context, obj *Instance) (*Instance, error)
	DeleteInstance(ctx context.Context, obj *Instance) error
	GetInstance(ctx context.Context, obj *Instance) (*Instance, error)
}

type gcfsServiceManager struct {
	fileService       *beta.Service
	instancesService  *beta.ProjectsLocationsInstancesService
	operationsService *beta.ProjectsLocationsOperationsService
}

const (
	locationURIFmt = "projects/%s/locations/%s"
	instanceURIFmt = locationURIFmt + "/instances/%s"
)

var _ Service = &gcfsServiceManager{}

func NewGCFSService(version string) (Service, error) {
	client, err := newOauthClient()
	if err != nil {
		return nil, err
	}

	fileService, err := beta.New(client)
	if err != nil {
		return nil, err
	}
	fileService.UserAgent = fmt.Sprintf("Google Cloud Filestore CSI Driver/%s (%s %s)", version, runtime.GOOS, runtime.GOARCH)

	return &gcfsServiceManager{
		fileService:       fileService,
		instancesService:  beta.NewProjectsLocationsInstancesService(fileService),
		operationsService: beta.NewProjectsLocationsOperationsService(fileService),
	}, nil
}

func (manager *gcfsServiceManager) CreateInstance(ctx context.Context, obj *Instance) (*Instance, error) {
	// TODO: add some labels to to tag this plugin
	betaObj := &beta.Instance{
		Tier: obj.Tier,
		FileShares: []*beta.FileShareConfig{
			{
				Name:       obj.Volume.Name,
				CapacityGb: util.RoundBytesToGb(obj.Volume.SizeBytes),
			},
		},
		Networks: []*beta.NetworkConfig{
			{
				Network:         obj.Network.Name,
				Modes:           []string{"MODE_IPV4"},
				ReservedIpRange: obj.Network.ReservedIpRange,
			},
		},
	}

	glog.Infof("Starting CreateInstance cloud operation")
	glog.V(4).Infof("Creating instance %v: location %v, tier %v, capacity %v, network %v, ipRange %v",
		obj.Name,
		obj.Location,
		betaObj.Tier,
		betaObj.FileShares[0].CapacityGb,
		betaObj.Networks[0].Network,
		betaObj.Networks[0].ReservedIpRange)
	_, err := manager.instancesService.Create(locationURI(obj.Project, obj.Location), betaObj).InstanceId(obj.Name).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("CreateInstance operation failed: %v", err)
	}

	// Always return error and check for instance in the subsequent calls
	return nil, fmt.Errorf("CreateInstance operation started")
}

func (manager *gcfsServiceManager) GetInstance(ctx context.Context, obj *Instance) (*Instance, error) {
	instance, err := manager.instancesService.Get(instanceURI(obj.Project, obj.Location, obj.Name)).Context(ctx).Do()
	if err != nil && !isNotFoundErr(err) {
		return nil, fmt.Errorf("GetInstance operation failed: %v", err)
	}
	if instance != nil {
		newInstance := cloudInstanceToServiceInstance(instance)
		switch instance.State {
		case "READY":
			return newInstance, nil
		default:
			// Instance exists but is not usable
			return newInstance, fmt.Errorf("instance %v is %v", obj.Name, instance.State)
		}
	}
	return nil, nil
}

func cloudInstanceToServiceInstance(instance *beta.Instance) *Instance {
	return &Instance{
		Name: instance.Name,
		Tier: instance.Tier,
		Volume: Volume{
			Name:      instance.FileShares[0].Name,
			SizeBytes: util.GbToBytes(instance.FileShares[0].CapacityGb),
		},
		Network: Network{
			Name:            instance.Networks[0].Network,
			Ip:              instance.Networks[0].IpAddresses[0],
			ReservedIpRange: instance.Networks[0].ReservedIpRange,
		},
	}
}

func CompareInstances(a, b *Instance) error {
	mismatches := []string{}
	if strings.ToLower(a.Tier) != strings.ToLower(b.Tier) {
		mismatches = append(mismatches, "tier")
	}
	if a.Volume.Name != b.Volume.Name {
		mismatches = append(mismatches, "volume name")
	}
	if util.RoundBytesToGb(a.Volume.SizeBytes) != util.RoundBytesToGb(b.Volume.SizeBytes) {
		mismatches = append(mismatches, "volume size")
	}
	if a.Network.Name != b.Network.Name {
		mismatches = append(mismatches, "network name")
	}

	if len(mismatches) > 0 {
		return fmt.Errorf("instance %v already exists but doesn't match expected: %+v", a.Name, mismatches)
	}
	return nil
}

func (manager *gcfsServiceManager) DeleteInstance(ctx context.Context, obj *Instance) error {
	instance, err := manager.GetInstance(ctx, obj)
	if err != nil {
		return err
	}
	if instance == nil {
		glog.Infof("Instance %v not found", obj.Name)
		return nil
	}

	glog.Infof("Starting DeleteInstance cloud operation")
	_, err = manager.instancesService.Delete(instanceURI(obj.Project, obj.Location, obj.Name)).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("DeleteInstance operation failed: %v", err)
	}

	// Always return error and check for instance in the subsequent calls
	return fmt.Errorf("DeleteInstance operation started")
}

func locationURI(project, location string) string {
	return fmt.Sprintf(locationURIFmt, project, location)
}

func instanceURI(project, location, name string) string {
	return fmt.Sprintf(instanceURIFmt, project, location, name)
}

func isNotFoundErr(err error) bool {
	apiErr, ok := err.(*googleapi.Error)
	if !ok {
		return false
	}

	for _, e := range apiErr.Errors {
		if e.Reason == "notFound" {
			return true
		}
	}
	return false
}
