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

	csi "github.com/container-storage-interface/spec/lib/go/csi"
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

	// Keys for Topology.
	TopologyKeyZone = "topology.gke.io/zone"
)

// Volume attributes
const (
	attrIP     = "ip"
	attrVolume = "volume"
)

// CreateVolume parameters
const (
	paramTier             = "tier"
	paramLocation         = "location"
	paramNetwork          = "network"
	paramReservedIPV4CIDR = "reserved-ipv4-cidr"

	// Keys for PV and PVC parameters as reported by external-provisioner
	ParameterKeyPVCName      = "csi.storage.k8s.io/pvc/name"
	ParameterKeyPVCNamespace = "csi.storage.k8s.io/pvc/namespace"
	ParameterKeyPVName       = "csi.storage.k8s.io/pv/name"

	// User provided labels
	ParameterKeyLabels = "labels"

	// Keys for tags to attach to the provisioned disk.
	tagKeyCreatedForClaimNamespace = "kubernetes_io_created-for_pvc_namespace"
	tagKeyCreatedForClaimName      = "kubernetes_io_created-for_pvc_name"
	tagKeyCreatedForVolumeName     = "kubernetes_io_created-for_pv_name"
	tagKeyCreatedBy                = "storage_gke_io_created-by"
)

// controllerServer handles volume provisioning
type controllerServer struct {
	config *controllerServerConfig
}

type controllerServerConfig struct {
	driver      *GCFSDriver
	fileService file.Service
	metaService metadata.Service
	ipAllocator *util.IPAllocator
}

func newControllerServer(config *controllerServerConfig) csi.ControllerServer {
	config.ipAllocator = util.NewIPAllocator(make(map[string]bool))
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

	capBytes, err := getRequestCapacity(req.GetCapacityRange())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	glog.V(5).Infof("Using capacity bytes %q for volume %q", capBytes, name)
	newFiler, err := s.generateNewFileInstance(name, capBytes, req.GetParameters(), req.GetAccessibilityRequirements())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// TODO: The workflow to provision a volume from a volume snapshot source needs to be implemented.
	// The block of code is added to make sanity tests happy, because if the driver supports CREATE_DELETE_VOLUME,
	// it is expected to support provision a volume from a volume snapshot source.
	if req.GetVolumeContentSource() != nil {
		if req.GetVolumeContentSource().GetVolume() != nil {
			return nil, status.Error(codes.InvalidArgument, "Unsupported volume content source")
		}

		if req.GetVolumeContentSource().GetSnapshot() != nil {
			id := req.GetVolumeContentSource().GetSnapshot().GetSnapshotId()
			isBackupSource, err := util.IsBackupHandle(id)
			if err != nil || !isBackupSource {
				return nil, status.Error(codes.NotFound, fmt.Sprintf("Unsupported volume content source %v", id))
			}
			_, err = s.config.fileService.GetBackup(ctx, id)
			if err != nil {
				if file.IsNotFoundErr(err) {
					return nil, status.Error(codes.NotFound, fmt.Sprintf("Failed to get snapshot %v", id))
				}
				return nil, status.Error(codes.Internal, fmt.Sprintf("Failed to get snapshot %v: %v", id, err))
			}
		}
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
		// If we are creating a new instance, we need pick an unused /29 range from reserved-ipv4-cidr
		// If the param was not provided, we default reservedIPRange to "" and cloud provider takes care of the allocation
		if reservedIPV4CIDR, ok := req.GetParameters()[paramReservedIPV4CIDR]; ok {
			reservedIPRange, err := s.reserveIPRange(ctx, newFiler, reservedIPV4CIDR)

			// Possible cases are 1) CreateInstanceAborted, 2)CreateInstance running in background
			// The ListInstances response will contain the reservedIPRange if the operation was started
			// In case of abort, the /29 IP is released and available for reservation
			defer s.config.ipAllocator.ReleaseIPRange(reservedIPRange)
			if err != nil {
				return nil, err
			}

			// Adding the reserved IP range to the instance object
			newFiler.Network.ReservedIpRange = reservedIPRange
		}

		// Add labels.
		labels, err := extractLabels(req.GetParameters(), s.config.driver.config.Name)
		if err != nil {
			return nil, err
		}
		newFiler.Labels = labels

		// Create the instance
		filer, err = s.config.fileService.CreateInstance(ctx, newFiler)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	return &csi.CreateVolumeResponse{Volume: fileInstanceToCSIVolume(filer, modeInstance)}, nil
}

// reserveIPRange returns the available IP in the cidr
func (s *controllerServer) reserveIPRange(ctx context.Context, filer *file.ServiceInstance, cidr string) (string, error) {
	cloudInstancesReservedIPRanges, err := s.getCloudInstancesReservedIPRanges(ctx, filer)
	if err != nil {
		return "", err
	}
	unreservedIPBlock, err := s.config.ipAllocator.GetUnreservedIPRange(cidr, cloudInstancesReservedIPRanges)
	if err != nil {
		return "", err
	}
	return unreservedIPBlock, nil
}

// getCloudInstancesReservedIPRanges gets the list of reservedIPRanges from cloud instances
func (s *controllerServer) getCloudInstancesReservedIPRanges(ctx context.Context, filer *file.ServiceInstance) (map[string]bool, error) {
	instances, err := s.config.fileService.ListInstances(ctx, filer)
	if err != nil {
		return nil, status.Error(codes.Aborted, err.Error())
	}
	// Initialize an empty reserved list. It will be populated with all the reservedIPRanges obtained from the cloud instances
	cloudInstancesReservedIPRanges := make(map[string]bool)
	for _, instance := range instances {
		cloudInstancesReservedIPRanges[instance.Network.ReservedIpRange] = true
	}
	return cloudInstancesReservedIPRanges, nil
}

// DeleteVolume deletes a GCFS instance
func (s *controllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	glog.V(4).Infof("DeleteVolume called with request %v", *req)

	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is empty")
	}
	filer, _, err := getFileInstanceFromID(volumeID)
	if err != nil {
		// An invalid ID should be treated as doesn't exist
		glog.V(5).Infof("failed to get instance for volume %v deletion: %v", volumeID, err)
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
	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is empty")
	}
	caps := req.GetVolumeCapabilities()
	if len(caps) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume capabilities is empty")
	}

	// Check that the volume exists
	filer, _, err := getFileInstanceFromID(volumeID)
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
		return nil, status.Error(codes.NotFound, fmt.Sprintf("volume %v doesn't exist", volumeID))
	}

	// Validate that the volume matches the capabilities
	// Note that there is nothing in the instance that we actually need to validate
	if err := s.config.driver.validateVolumeCapabilities(caps); err != nil {
		return &csi.ValidateVolumeCapabilitiesResponse{
			Message: err.Error(),
		}, status.Error(codes.InvalidArgument, err.Error())
	}

	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeContext:      req.GetVolumeContext(),
			VolumeCapabilities: req.GetVolumeCapabilities(),
			Parameters:         req.GetParameters(),
		},
	}, nil
}

func (s *controllerServer) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: s.config.driver.cscap,
	}, nil
}

// getRequestCapacity returns the volume size that should be provisioned
func getRequestCapacity(capRange *csi.CapacityRange) (int64, error) {
	if capRange == nil {
		return minVolumeSize, nil
	}

	rCap := capRange.GetRequiredBytes()
	rSet := rCap > 0
	lCap := capRange.GetLimitBytes()
	lSet := lCap > 0

	if lSet && rSet && lCap < rCap {
		return 0, fmt.Errorf("Limit bytes %v is less than required bytes %v", lCap, rCap)
	}

	if lSet && lCap < minVolumeSize {
		return 0, fmt.Errorf("Limit bytes %v is less than minimum instance size bytes %v", lCap, minVolumeSize)
	}

	if lCap > 0 {
		if rCap == 0 {
			// request not set
			return lCap, nil
		}
		// request set, round up to min
		return util.Min(util.Max(rCap, minVolumeSize), lCap), nil
	}

	// limit not set
	return util.Max(rCap, minVolumeSize), nil
}

// generateNewFileInstance populates the GCFS Instance object using
// CreateVolume parameters
func (s *controllerServer) generateNewFileInstance(name string, capBytes int64, params map[string]string, topo *csi.TopologyRequirement) (*file.ServiceInstance, error) {
	location, err := s.pickZone(topo)
	if err != nil {
		return nil, fmt.Errorf("invalid topology error %v", err.Error())
	}

	// Set default parameters
	tier := defaultTier
	network := defaultNetwork

	// Validate parameters (case-insensitive).
	for k, v := range params {
		switch strings.ToLower(k) {
		// Cloud API will validate these
		case paramTier:
			tier = v
		case paramNetwork:
			network = v
		// Ignore the cidr flag as it is not passed to the cloud provider
		// It will be used to get unreserved IP in the reserveIPV4Range function
		case paramReservedIPV4CIDR:
			continue
		case ParameterKeyLabels, ParameterKeyPVCName, ParameterKeyPVCNamespace, ParameterKeyPVName:
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
		VolumeId:      getVolumeIDFromFileInstance(instance, mode),
		CapacityBytes: instance.Volume.SizeBytes,
		VolumeContext: map[string]string{
			attrIP:     instance.Network.Ip,
			attrVolume: instance.Volume.Name,
		},
	}
}

// ControllerExpandVolume expands a GCFS instance share.
func (s *controllerServer) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "ControllerExpandVolume volume ID must be provided")
	}

	reqBytes, err := getRequestCapacity(req.GetCapacityRange())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	filer, _, err := getFileInstanceFromID(volumeID)
	if err != nil {
		glog.Errorf("failed to get instance for volumeID %v expansion, error: %v", volumeID, err)
		return nil, err
	}

	filer.Project = s.config.metaService.GetProject()
	filer.Volume.SizeBytes = reqBytes
	newfiler, err := s.config.fileService.ResizeInstance(ctx, filer)
	if err != nil {
		glog.Errorf("failed to resize volumeID %v, error: %v", volumeID, err)
		return nil, err
	}

	return &csi.ControllerExpandVolumeResponse{
		CapacityBytes:         newfiler.Volume.SizeBytes,
		NodeExpansionRequired: false,
	}, nil
}

func (s *controllerServer) pickZone(top *csi.TopologyRequirement) (string, error) {
	if top == nil {
		return s.config.metaService.GetZone(), nil
	}

	return pickZoneFromTopology(top)
}

// Pick the first available topology from preferred list or requisite list in that order.
func pickZoneFromTopology(top *csi.TopologyRequirement) (string, error) {
	reqZones, err := getZonesFromTopology(top.GetRequisite())
	if err != nil {
		return "", fmt.Errorf("could not get zones from requisite topology: %v", err)
	}

	prefZones, err := getZonesFromTopology(top.GetPreferred())
	if err != nil {
		return "", fmt.Errorf("could not get zones from preferred topology: %v", err)
	}

	if len(prefZones) == 0 && len(reqZones) == 0 {
		return "", fmt.Errorf("both requisite and preferred topology list empty")
	}

	if len(prefZones) != 0 {
		return prefZones[0], nil
	}
	return reqZones[0], nil
}

func getZonesFromTopology(topList []*csi.Topology) ([]string, error) {
	zones := []string{}
	for _, top := range topList {
		if top.GetSegments() == nil {
			return nil, fmt.Errorf("topologies specified but no segments")
		}

		zone, err := getZoneFromSegment(top.GetSegments())
		if err != nil {
			return nil, fmt.Errorf("could not get zone from topology: %v", err)
		}
		zones = append(zones, zone)
	}
	return zones, nil
}

func getZoneFromSegment(seg map[string]string) (string, error) {
	var zone string
	for k, v := range seg {
		switch k {
		case TopologyKeyZone:
			zone = v
		default:
			return "", fmt.Errorf("topology segment has unknown key %v", k)
		}
	}

	if len(zone) == 0 {
		return "", fmt.Errorf("topology specified but could not find zone in segment: %v", seg)
	}
	return zone, nil
}

func extractLabels(parameters map[string]string, driverName string) (map[string]string, error) {
	labels := make(map[string]string)
	scLables := make(map[string]string)
	for k, v := range parameters {
		switch strings.ToLower(k) {
		case ParameterKeyPVCName:
			labels[tagKeyCreatedForClaimName] = v
		case ParameterKeyPVCNamespace:
			labels[tagKeyCreatedForClaimNamespace] = v
		case ParameterKeyPVName:
			labels[tagKeyCreatedForVolumeName] = v
		case ParameterKeyLabels:
			var err error
			scLables, err = util.ConvertLabelsStringToMap(v)
			if err != nil {
				return nil, fmt.Errorf("parameters contain invalid labels parameter: %w", err)
			}
		}
	}

	labels[tagKeyCreatedBy] = strings.ReplaceAll(driverName, ".", "_")
	return mergeLabels(scLables, labels)
}

func mergeLabels(scLabels map[string]string, metedataLabels map[string]string) (map[string]string, error) {
	result := make(map[string]string)
	for k, v := range metedataLabels {
		result[k] = v
	}

	for k, v := range scLabels {
		if _, ok := result[k]; ok {
			return nil, fmt.Errorf("Storage Class labels cannot contain metadata label key %s", k)
		}

		result[k] = v
	}

	return result, nil
}

func (s *controllerServer) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	if len(req.Name) == 0 {
		return nil, status.Error(codes.InvalidArgument, "CreateSnapshot name must be provided")
	}
	volumeID := req.GetSourceVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "CreateSnapshot source volume ID must be provided")
	}
	filer, _, err := getFileInstanceFromID(volumeID)
	if err != nil {
		glog.Errorf("Failed to get instance for volumeID %v snapshot, error: %v", volumeID, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	filer.Project = s.config.metaService.GetProject()
	// If parameters are empty we assume 'backup' type by default.
	if req.GetParameters() != nil {
		if _, err := util.IsSnapshotTypeSupported(req.GetParameters()); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	// Check for exisitng snapshot
	backupUri, _, err := file.CreateBackpURI(filer, req.Name, util.GetBackupLocation(req.GetParameters()))
	if err != nil {
		return nil, err
	}
	backupInfo, err := s.config.fileService.GetBackup(ctx, backupUri)
	if err != nil {
		if !file.IsNotFoundErr(err) {
			return nil, status.Error(codes.Internal, err.Error())
		}
	} else {
		backupSourceCSIHandle, err := util.BackupVolumeSourceToCSIVolumeHandle(backupInfo.SourceVolumeHandle)
		if err != nil {
			return nil, status.Error(codes.Internal, fmt.Sprintf("Cannot determine volume handle from back source %s", backupInfo.SourceVolumeHandle))
		}
		if backupSourceCSIHandle != volumeID {
			return nil, status.Error(codes.AlreadyExists, fmt.Sprintf("Backup already exists with a different source volume %s, input source volume %s", backupInfo.SourceVolumeHandle, volumeID))
		}
		// Check if backup is ready.
		if backupInfo.Backup.State != "READY" {
			return nil, status.Error(codes.Internal, fmt.Sprintf("Backup %v not yet ready, current state %s", backupInfo.Backup.Name, backupInfo.Backup.State))
		}
		tp, err := util.ParseTimestamp(backupInfo.Backup.CreateTime)
		if err != nil {
			return nil, status.Error(codes.Internal, fmt.Sprintf("failed to parse create timestamp for backup %v", backupInfo.Backup.Name))
		}
		return &csi.CreateSnapshotResponse{
			Snapshot: &csi.Snapshot{
				SizeBytes:      util.GbToBytes(backupInfo.Backup.CapacityGb),
				SnapshotId:     backupInfo.Backup.Name,
				SourceVolumeId: volumeID,
				CreationTime:   tp,
				ReadyToUse:     true,
			},
		}, nil
	}

	backupObj, err := s.config.fileService.CreateBackup(ctx, filer, req.Name, util.GetBackupLocation(req.GetParameters()))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	tp, err := util.ParseTimestamp(backupObj.CreateTime)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	resp := &csi.CreateSnapshotResponse{
		Snapshot: &csi.Snapshot{
			SizeBytes:      util.GbToBytes(backupObj.CapacityGb),
			SnapshotId:     backupObj.Name,
			SourceVolumeId: volumeID,
			CreationTime:   tp,
			ReadyToUse:     true,
		},
	}
	glog.Infof("CreateSnapshot succeeded for Id %s on volume %s", backupObj.Name, volumeID)
	return resp, nil
}

func (s *controllerServer) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	// Validate arguments
	id := req.GetSnapshotId()
	if len(id) == 0 {
		return nil, status.Error(codes.InvalidArgument, "DeleteSnapshot snapshot Id must be provided")
	}

	isBackup, err := util.IsBackupHandle(id)
	if err != nil {
		// Sanity tests expects delete to pass for invalid handles.
		glog.Warningf("Could not parse snapshot handle %v", id)
		return &csi.DeleteSnapshotResponse{}, nil
	}

	if !isBackup {
		glog.Errorf("Deletion of snapshot type %q not supported", id)
		return nil, status.Error(codes.Internal, "Deletion of snapshot type not supported")
	}

	if err = s.config.fileService.DeleteBackup(ctx, id); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &csi.DeleteSnapshotResponse{}, nil
}
