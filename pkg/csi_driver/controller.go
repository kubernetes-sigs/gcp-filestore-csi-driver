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

package driver

import (
	"fmt"
	"strings"

	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	"github.com/golang/glog"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/file"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/metadata"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/util"
)

const (
	// premium tier min is 2.5 Tb, let GCFS error
	minVolumeSize     int64 = 1 * util.Tb
	modeInstance            = "modeInstance"
	newInstanceVolume       = "vol1"

	defaultTier    = "standard"
	defaultNetwork = "default"
)

// Volume attributes
const (
	attrIp     = "ip"
	attrVolume = "volume"
)

// CreateVolume parameters
const (
	paramTier     = "tier"
	paramLocation = "location"
	paramNetwork  = "network"
)

// controllerServer handles volume provisioning
type controllerServer struct {
	config *controllerServerConfig
}

type controllerServerConfig struct {
	driver      *GCFSDriver
	fileService file.Service
	metaService metadata.Service
}

func newControllerServer(config *controllerServerConfig) csi.ControllerServer {
	return &controllerServer{config: config}
}

// CreateVolume creates a GCFS instance
func (s *controllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	glog.V(4).Infof("CreateVolume called with request %v", *req)

	// Validate arguments
	name := req.GetName()
	if len(name) == 0 {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume name must be provided")
	}

	if err := s.config.driver.validateVolumeCapabilities(req.GetVolumeCapabilities()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	capBytes := getRequestCapacity(req.GetCapacityRange())
	glog.V(5).Infof("Using capacity bytes %q for volume %q", capBytes, name)

	newFiler, err := s.generateNewFileInstance(name, capBytes, req.GetParameters())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// Check if the instance already exists
	filer, err := s.config.fileService.GetInstance(ctx, newFiler)
	if err != nil && !file.IsNotFoundErr(err) {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if filer != nil {
		// Instance already exists, check if it meets the request
		if err = file.CompareInstances(newFiler, filer); err != nil {
			return nil, status.Error(codes.AlreadyExists, err.Error())
		}
	} else {
		// Create the instance
		filer, err = s.config.fileService.CreateInstance(ctx, newFiler)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	return &csi.CreateVolumeResponse{Volume: fileInstanceToCSIVolume(filer, modeInstance)}, nil
}

// DeleteVolume deletes a GCFS instance
func (s *controllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	glog.V(4).Infof("DeleteVolume called with request %v", *req)

	volumeId := req.GetVolumeId()
	if volumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is empty")
	}
	filer, _, err := getFileInstanceFromId(volumeId)
	if err != nil {
		// An invalid ID should be treated as doesn't exist
		glog.V(5).Infof("failed to get instance for deletion: %v", volumeId, err)
		return &csi.DeleteVolumeResponse{}, nil
	}

	filer.Project = s.config.metaService.GetProject()
	err = s.config.fileService.DeleteInstance(ctx, filer)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &csi.DeleteVolumeResponse{}, nil
}

func (s *controllerServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	// Validate arguments
	volumeId := req.GetVolumeId()
	if volumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is empty")
	}
	caps := req.GetVolumeCapabilities()
	if len(caps) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume capabilities is empty")
	}

	// Check that the volume exists
	filer, _, err := getFileInstanceFromId(volumeId)
	if err != nil {
		// An invalid id format is treated as doesn't exist
		return nil, status.Error(codes.NotFound, err.Error())
	}

	filer.Project = s.config.metaService.GetProject()
	newFiler, err := s.config.fileService.GetInstance(ctx, filer)
	if err != nil && !file.IsNotFoundErr(err) {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if newFiler == nil {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("volume %v doesn't exist", volumeId))
	}

	// Validate that the volume matches the capabilities
	// Note that there is nothing in the instance that we actually need to validate
	if err := s.config.driver.validateVolumeCapabilities(caps); err != nil {
		return &csi.ValidateVolumeCapabilitiesResponse{
			Supported: false,
			Message:   err.Error(),
		}, status.Error(codes.InvalidArgument, err.Error())
	}

	return &csi.ValidateVolumeCapabilitiesResponse{
		Supported: true,
	}, nil
}

func (s *controllerServer) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: s.config.driver.cscap,
	}, nil
}

// getRequestCapacity returns the volume size that should be provisioned
func getRequestCapacity(capRange *csi.CapacityRange) int64 {
	if capRange == nil {
		return minVolumeSize
	}

	rCap := capRange.GetRequiredBytes()
	lCap := capRange.GetLimitBytes()

	if lCap > 0 {
		if rCap == 0 {
			// request not set
			return lCap
		} else {
			// request set, round up to min
			return util.Min(util.Max(rCap, minVolumeSize), lCap)
		}
	}

	// limit not set
	return util.Max(rCap, minVolumeSize)
}

// generateNewFileInstance populates the GCFS Instance object using
// CreateVolume parameters
func (s *controllerServer) generateNewFileInstance(name string, capBytes int64, params map[string]string) (*file.ServiceInstance, error) {
	// Set default parameters
	tier := defaultTier
	network := defaultNetwork
	location := s.config.metaService.GetZone()

	// Validate parameters (case-insensitive).
	for k, v := range params {
		switch strings.ToLower(k) {
		// Cloud API will validate these
		case paramTier:
			tier = v
		case paramLocation:
			location = v
		case paramNetwork:
			network = v
		// Unused
		case "csiprovisionersecretname", "csiprovisionersecretnamespace":
		default:
			return nil, fmt.Errorf("invalid parameter %q", k)
		}
	}
	return &file.ServiceInstance{
		Project:  s.config.metaService.GetProject(),
		Name:     name,
		Location: location,
		Tier:     tier,
		Network: file.Network{
			Name: network,
			// ReservedIpRange: "10.3.0.0/29", // TODO
		},
		Volume: file.Volume{
			Name:      newInstanceVolume,
			SizeBytes: capBytes,
		},
	}, nil
}

// fileInstanceToCSIVolume generates a CSI volume spec from the cloud Instance
func fileInstanceToCSIVolume(instance *file.ServiceInstance, mode string) *csi.Volume {
	return &csi.Volume{
		Id:            getVolumeIdFromFileInstance(instance, mode),
		CapacityBytes: instance.Volume.SizeBytes,
		Attributes: map[string]string{
			attrIp:     instance.Network.Ip,
			attrVolume: instance.Volume.Name,
		},
	}
}
