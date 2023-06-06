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
	"time"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
	cloud "sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/file"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/metrics"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/util"
)

const (
	// premium tier min is 2.5 Tb, let GCFS error
	minVolumeSize     int64 = 1 * util.Tb
	modeInstance            = "modeInstance"
	newInstanceVolume       = "vol1"

	defaultTier    = "standard"
	enterpriseTier = "enterprise"
	defaultNetwork = "default"

	directPeering        = "DIRECT_PEERING"
	privateServiceAccess = "PRIVATE_SERVICE_ACCESS"

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
	paramTier                      = "tier"
	paramLocation                  = "location"
	paramNetwork                   = "network"
	paramReservedIPV4CIDR          = "reserved-ipv4-cidr"
	paramReservedIPRange           = "reserved-ip-range"
	paramConnectMode               = "connect-mode"
	paramMultishare                = "multishare"
	paramInstanceEncryptionKmsKey  = "instance-encryption-kms-key"
	paramMultishareInstanceScLabel = "instance-storageclass-label"

	// Keys for PV and PVC parameters as reported by external-provisioner
	ParameterKeyPVCName      = "csi.storage.k8s.io/pvc/name"
	ParameterKeyPVCNamespace = "csi.storage.k8s.io/pvc/namespace"
	ParameterKeyPVName       = "csi.storage.k8s.io/pv/name"

	// User provided labels
	ParameterKeyLabels = "labels"

	// Keys for tags to attach to the provisioned Filestore shares and instances.
	tagKeyCreatedForClaimNamespace = "kubernetes_io_created-for_pvc_namespace"
	tagKeyCreatedForClaimName      = "kubernetes_io_created-for_pvc_name"
	tagKeyCreatedForVolumeName     = "kubernetes_io_created-for_pv_name"
	tagKeyCreatedBy                = "storage_gke_io_created-by"
	tagKeyClusterName              = "storage_gke_io_cluster_name"
	tagKeyClusterLocation          = "storage_gke_io_cluster_location"
)

// controllerServer handles volume provisioning
type controllerServer struct {
	config *controllerServerConfig
}

type controllerServerConfig struct {
	driver               *GCFSDriver
	fileService          file.Service
	cloud                *cloud.Cloud
	ipAllocator          *util.IPAllocator
	volumeLocks          *util.VolumeLocks
	enableMultishare     bool
	multiShareController *MultishareController
	metricsManager       *metrics.MetricsManager
	ecfsDescription      string
	isRegional           bool
	clusterName          string
}

func newControllerServer(config *controllerServerConfig) csi.ControllerServer {
	cs := &controllerServer{config: config}
	config.ipAllocator = util.NewIPAllocator(make(map[string]bool))
	if config.enableMultishare {
		config.multiShareController = NewMultishareController(config)
		config.multiShareController.opsManager.controllerServer = cs
	}
	return cs
}

// CreateVolume creates a GCFS instance
func (s *controllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	if strings.ToLower(req.GetParameters()[paramMultishare]) == "true" {
		if s.config.multiShareController == nil {
			return nil, status.Error(codes.InvalidArgument, "multishare controller not enabled")
		}
		start := time.Now()
		response, err := s.config.multiShareController.CreateVolume(ctx, req)
		duration := time.Since(start)
		s.config.metricsManager.RecordOperationMetrics(err, methodCreateVolume, modeMultishare, duration)
		klog.Infof("CreateVolume response %+v error %v, for request %+v", response, err, req)
		return response, err
	}

	klog.V(4).Infof("CreateVolume called with request %+v", req)
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
	klog.V(5).Infof("Using capacity bytes %q for volume %q", capBytes, name)

	newFiler, err := s.generateNewFileInstance(name, capBytes, req.GetParameters(), req.GetAccessibilityRequirements())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	volumeID := getVolumeIDFromFileInstance(newFiler, modeInstance)
	if acquired := s.config.volumeLocks.TryAcquire(volumeID); !acquired {
		return nil, status.Errorf(codes.Aborted, util.VolumeOperationAlreadyExistsFmt, volumeID)
	}
	defer s.config.volumeLocks.Release(volumeID)

	sourceSnapshotId := ""
	if req.GetVolumeContentSource() != nil {
		if newFiler.Tier == enterpriseTier {
			return nil, status.Error(codes.InvalidArgument, "Enterprise tier Filestore does not support Backup yet")
		}
		if req.GetVolumeContentSource().GetVolume() != nil {
			return nil, status.Error(codes.InvalidArgument, "Unsupported volume content source")
		}

		if req.GetVolumeContentSource().GetSnapshot() != nil {
			id := req.GetVolumeContentSource().GetSnapshot().GetSnapshotId()
			isBackupSource, err := util.IsBackupHandle(id)
			if err != nil || !isBackupSource {
				return nil, status.Errorf(codes.InvalidArgument, "Unsupported volume content source %v", id)
			}
			_, err = s.config.fileService.GetBackup(ctx, id)
			if err != nil {
				klog.Errorf("Failed to get volume %v source snapshot %v: %v", name, id, err.Error())
				return nil, status.Error(*file.CodeForError(err), err.Error())
			}
			sourceSnapshotId = id
		}
	}

	// Check if the instance already exists
	filer, err := s.config.fileService.GetInstance(ctx, newFiler)
	// No error is returned if the instance is not found during CreateVolume.
	if err != nil && !file.IsNotFoundErr(err) {
		return nil, status.Error(*file.CodeForError(err), err.Error())
	}

	if filer != nil {
		klog.V(4).Infof("Found existing instance %+v, current instance %+v\n", filer, newFiler)
		// Instance already exists, check if it meets the request
		if err = file.CompareInstances(newFiler, filer); err != nil {
			return nil, status.Error(codes.AlreadyExists, err.Error())
		}
		// Check if the filestore instance is in the process of getting created.
		if filer.State == "CREATING" {
			msg := fmt.Sprintf("Volume %v not ready, current state: %v", name, filer.State)
			klog.V(4).Infof(msg)
			return nil, status.Error(codes.DeadlineExceeded, msg)
		}
		if filer.State != "READY" {
			msg := fmt.Sprintf("Volume %v not ready, current state: %v", name, filer.State)
			klog.V(4).Infof(msg)
			return nil, status.Error(codes.Internal, msg)
		}
	} else {
		param := req.GetParameters()
		// If we are creating a new instance, we need pick an unused CIDR range from reserved-ipv4-cidr
		// If the param was not provided, we default reservedIPRange to "" and cloud provider takes care of the allocation
		if newFiler.Network.ConnectMode == privateServiceAccess {
			if reservedIPRange, ok := param[paramReservedIPRange]; ok {
				if IsCIDR(reservedIPRange) {
					return nil, status.Errorf(codes.InvalidArgument, "When using connect mode PRIVATE_SERVICE_ACCESS, if reserved IP range is specified, it must be a named address range instead of direct CIDR value %v", reservedIPRange)
				}
				newFiler.Network.ReservedIpRange = reservedIPRange
			}
		} else if reservedIPV4CIDR, ok := param[paramReservedIPV4CIDR]; ok {
			reservedIPRange, err := s.reserveIPRange(ctx, newFiler, reservedIPV4CIDR)

			// Possible cases are 1) CreateInstanceAborted, 2)CreateInstance running in background
			// The ListInstances response will contain the reservedIPRange if the operation was started
			// In case of abort, the CIDR IP is released and available for reservation
			defer s.config.ipAllocator.ReleaseIPRange(reservedIPRange)
			if err != nil {
				return nil, err
			}

			// Adding the reserved IP range to the instance object
			newFiler.Network.ReservedIpRange = reservedIPRange
		}

		// Add labels.
		labels, err := extractLabels(param, s.config.driver.config.Name)
		if err != nil {
			return nil, err
		}
		newFiler.Labels = labels

		// Create the instance
		var createErr error
		if sourceSnapshotId != "" {
			filer, createErr = s.config.fileService.CreateInstanceFromBackupSource(ctx, newFiler, sourceSnapshotId)
		} else {
			filer, createErr = s.config.fileService.CreateInstance(ctx, newFiler)
		}
		if createErr != nil {
			klog.Errorf("Create volume for volume Id %s failed: %v", volumeID, createErr.Error())
			return nil, status.Error(*file.CodeForError(createErr), createErr.Error())
		}
	}
	resp := &csi.CreateVolumeResponse{Volume: fileInstanceToCSIVolume(filer, modeInstance, sourceSnapshotId)}
	klog.Infof("CreateVolume succeeded: %+v", resp)
	return resp, nil
}

// reserveIPRange returns the available IP in the cidr
func (s *controllerServer) reserveIPRange(ctx context.Context, filer *file.ServiceInstance, cidr string) (string, error) {
	cloudInstancesReservedIPRanges, err := s.getCloudInstancesReservedIPRanges(ctx, filer)
	if err != nil {
		return "", err
	}
	ipRangeSize := util.IpRangeSize
	if filer.Tier == enterpriseTier {
		ipRangeSize = util.IpRangeSizeEnterprise
	}
	unreservedIPBlock, err := s.config.ipAllocator.GetUnreservedIPRange(cidr, ipRangeSize, cloudInstancesReservedIPRanges)
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
	// Due to unreachable location some instances may not show up here.
	// TODO: create a new function to take a list of locations
	// and return error if unreachable contained the region of interest.
	multiShareInstances, err := s.config.fileService.ListMultishareInstances(ctx, &file.ListFilter{Project: filer.Project, Location: "-"})
	if err != nil {
		return nil, status.Error(codes.Aborted, err.Error())
	}

	// Initialize an empty reserved list. It will be populated with all the
	// reservedIPRanges obtained from the cloud instances in the same VPC network
	// as the ServiceInstance.
	cloudInstancesReservedIPRanges := make(map[string]bool)
	for _, instance := range instances {
		if strings.EqualFold(instance.Network.Name, filer.Network.Name) {
			cloudInstancesReservedIPRanges[instance.Network.ReservedIpRange] = true
		}
	}
	for _, instance := range multiShareInstances {
		if strings.EqualFold(instance.Network.Name, filer.Network.Name) {
			cloudInstancesReservedIPRanges[instance.Network.ReservedIpRange] = true
		}
	}
	return cloudInstancesReservedIPRanges, nil
}

// DeleteVolume deletes a GCFS instance
func (s *controllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	klog.Infof("DeleteVolume called with request %+v", req)
	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id is empty")
	}

	if isMultishareVolId(volumeID) {
		if s.config.multiShareController == nil {
			return nil, status.Error(codes.InvalidArgument, "multishare controller not enabled")
		}
		start := time.Now()
		response, err := s.config.multiShareController.DeleteVolume(ctx, req)
		duration := time.Since(start)
		s.config.metricsManager.RecordOperationMetrics(err, methodDeleteVolume, modeMultishare, duration)
		klog.Infof("Deletevolume response %+v error %v, for request: %+v", response, err, req)
		return response, err
	}

	filer, _, err := getFileInstanceFromID(volumeID)
	if err != nil {
		// An invalid ID should be treated as doesn't exist
		klog.V(5).Infof("failed to get instance for volume %v deletion: %v", volumeID, err)
		return &csi.DeleteVolumeResponse{}, nil
	}

	if acquired := s.config.volumeLocks.TryAcquire(volumeID); !acquired {
		return nil, status.Errorf(codes.Aborted, util.VolumeOperationAlreadyExistsFmt, volumeID)
	}
	defer s.config.volumeLocks.Release(volumeID)

	filer.Project = s.config.cloud.Project
	filer, err = s.config.fileService.GetInstance(ctx, filer)
	if err != nil {
		if file.IsNotFoundErr(err) {
			return &csi.DeleteVolumeResponse{}, nil
		}
		return nil, status.Error(*file.CodeForError(err), err.Error())
	}

	if filer.State == "DELETING" {
		return nil, status.Errorf(codes.DeadlineExceeded, "Volume %s is in state: %s", volumeID, filer.State)
	}

	err = s.config.fileService.DeleteInstance(ctx, filer)
	if err != nil {
		klog.Errorf("Delete volume for volume Id %s failed: %v", volumeID, err.Error())
		return nil, status.Error(*file.CodeForError(err), err.Error())
	}

	klog.Infof("DeleteVolume succeeded for volume %v", volumeID)
	return &csi.DeleteVolumeResponse{}, nil
}

func (s *controllerServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
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

	filer.Project = s.config.cloud.Project
	newFiler, err := s.config.fileService.GetInstance(ctx, filer)
	if err != nil && !file.IsNotFoundErr(err) {
		return nil, status.Error(*file.CodeForError(err), err.Error())
	}
	if newFiler == nil {
		return nil, status.Errorf(codes.NotFound, "volume %v doesn't exist", volumeID)
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
		return 0, fmt.Errorf("limit bytes %v is less than required bytes %v", lCap, rCap)
	}

	if lSet && lCap < minVolumeSize {
		return 0, fmt.Errorf("limit bytes %v is less than minimum instance size bytes %v", lCap, minVolumeSize)
	}

	if lSet {
		if rCap == 0 {
			// request not set
			return lCap, nil
		}
		// request set, round up to min
		return util.Max(rCap, minVolumeSize), nil
	}

	// limit not set
	return util.Max(rCap, minVolumeSize), nil
}

// generateNewFileInstance populates the GCFS Instance object using
// CreateVolume parameters
func (s *controllerServer) generateNewFileInstance(name string, capBytes int64, params map[string]string, topo *csi.TopologyRequirement) (*file.ServiceInstance, error) {
	location, err := s.pickZone(topo)
	if err != nil {
		return nil, fmt.Errorf("invalid topology error %w", err)
	}

	// Set default parameters
	tier := defaultTier
	network := defaultNetwork
	connectMode := directPeering
	kmsKeyName := ""

	// Validate parameters (case-insensitive).
	for k, v := range params {
		switch strings.ToLower(k) {
		// Cloud API will validate these
		case paramTier:
			tier = v
			if tier == enterpriseTier {
				region, err := util.GetRegionFromZone(location)
				if err != nil {
					return nil, fmt.Errorf("failed to get region from zone %s: %w", location, err)
				}
				location = region
			}
		case paramNetwork:
			network = v
		case paramConnectMode:
			connectMode = v
			if connectMode != directPeering && connectMode != privateServiceAccess {
				return nil, fmt.Errorf("connect mode can only be one of %q or %q", directPeering, privateServiceAccess)
			}
		case paramInstanceEncryptionKmsKey:
			kmsKeyName = v
		// Ignore the cidr flag as it is not passed to the cloud provider
		// It will be used to get unreserved IP in the reserveIPV4Range function
		// ignore IPRange flag as it will be handled at the same place as cidr
		case paramReservedIPV4CIDR, paramReservedIPRange:
			continue
		case ParameterKeyLabels, ParameterKeyPVCName, ParameterKeyPVCNamespace, ParameterKeyPVName:
		case "csiprovisionersecretname", "csiprovisionersecretnamespace":
		default:
			return nil, fmt.Errorf("invalid parameter %q", k)
		}
	}
	if kmsKeyName != "" && tier != enterpriseTier {
		return nil, fmt.Errorf("KMS Key data encryption is only supported for enterprise tier instances")
	}
	return &file.ServiceInstance{
		Project:  s.config.cloud.Project,
		Name:     name,
		Location: location,
		Tier:     tier,
		Network: file.Network{
			Name:        network,
			ConnectMode: connectMode,
		},
		Volume: file.Volume{
			Name:      newInstanceVolume,
			SizeBytes: capBytes,
		},
		KmsKeyName: kmsKeyName,
	}, nil
}

// fileInstanceToCSIVolume generates a CSI volume spec from the cloud Instance
func fileInstanceToCSIVolume(instance *file.ServiceInstance, mode, sourceSnapshotId string) *csi.Volume {
	resp := &csi.Volume{
		VolumeId:      getVolumeIDFromFileInstance(instance, mode),
		CapacityBytes: instance.Volume.SizeBytes,
		VolumeContext: map[string]string{
			attrIP:     instance.Network.Ip,
			attrVolume: instance.Volume.Name,
		},
	}
	if sourceSnapshotId != "" {
		contentSource := &csi.VolumeContentSource{
			Type: &csi.VolumeContentSource_Snapshot{
				Snapshot: &csi.VolumeContentSource_SnapshotSource{
					SnapshotId: sourceSnapshotId,
				},
			},
		}
		resp.ContentSource = contentSource
	}
	return resp
}

// ControllerExpandVolume expands a GCFS instance share.
func (s *controllerServer) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	klog.V(4).Infof("ControllerExpandVolume called with request %+v", req)
	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "ControllerExpandVolume volume ID must be provided")
	}

	if isMultishareVolId(volumeID) {
		if s.config.multiShareController == nil {
			return nil, status.Error(codes.InvalidArgument, "multishare controller not enabled")
		}
		start := time.Now()
		response, err := s.config.multiShareController.ControllerExpandVolume(ctx, req)
		duration := time.Since(start)
		s.config.metricsManager.RecordOperationMetrics(err, methodExpandVolume, modeMultishare, duration)
		klog.Infof("ControllerExpandVolume response %+v error %v, for request: %+v", response, err, req)
		return response, err
	}

	reqBytes, err := getRequestCapacity(req.GetCapacityRange())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if acquired := s.config.volumeLocks.TryAcquire(volumeID); !acquired {
		return nil, status.Errorf(codes.Aborted, util.VolumeOperationAlreadyExistsFmt, volumeID)
	}
	defer s.config.volumeLocks.Release(volumeID)

	filer, _, err := getFileInstanceFromID(volumeID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	filer.Project = s.config.cloud.Project
	filer, err = s.config.fileService.GetInstance(ctx, filer)
	if err != nil {
		return nil, status.Error(*file.CodeForError(err), err.Error())
	}
	if filer.State != "READY" {
		return nil, fmt.Errorf("lolume %q is not yet ready, current state %q", volumeID, filer.State)
	}

	if util.BytesToGb(reqBytes) <= util.BytesToGb(filer.Volume.SizeBytes) {
		klog.Infof("Controller expand volume succeeded for volume %v, existing size(bytes): %v", volumeID, filer.Volume.SizeBytes)
		return &csi.ControllerExpandVolumeResponse{
			CapacityBytes:         filer.Volume.SizeBytes,
			NodeExpansionRequired: false,
		}, nil
	}

	hasPendingOps, err := s.config.fileService.HasOperations(ctx, filer, "update", false /* done */)
	if err != nil {
		return nil, status.Error(*file.CodeForError(err), err.Error())
	}

	if hasPendingOps {
		return nil, status.Errorf(codes.DeadlineExceeded, "Update operation ongoing for volume %v", volumeID)
	}

	filer.Volume.SizeBytes = reqBytes
	newfiler, err := s.config.fileService.ResizeInstance(ctx, filer)
	if err != nil {
		return nil, status.Error(*file.CodeForError(err), err.Error())
	}

	klog.Infof("Controller expand volume succeeded for volume %v, new size(bytes): %v", volumeID, newfiler.Volume.SizeBytes)
	return &csi.ControllerExpandVolumeResponse{
		CapacityBytes:         newfiler.Volume.SizeBytes,
		NodeExpansionRequired: false,
	}, nil
}

func (s *controllerServer) pickZone(top *csi.TopologyRequirement) (string, error) {
	if top == nil {
		return s.config.cloud.Zone, nil
	}

	return pickZoneFromTopology(top)
}

// Pick the first available topology from preferred list or requisite list in that order.
func pickZoneFromTopology(top *csi.TopologyRequirement) (string, error) {
	reqZones, err := getZonesFromTopology(top.GetRequisite())
	if err != nil {
		return "", fmt.Errorf("could not get zones from requisite topology: %w", err)
	}

	prefZones, err := getZonesFromTopology(top.GetPreferred())
	if err != nil {
		return "", fmt.Errorf("could not get zones from preferred topology: %w", err)
	}

	if len(prefZones) == 0 && len(reqZones) == 0 {
		return "", fmt.Errorf("both requisite and preferred topology list empty")
	}

	if len(prefZones) != 0 {
		return prefZones[0], nil
	}
	return reqZones[0], nil
}

func listZonesFromTopology(top *csi.TopologyRequirement) ([]string, error) {
	reqZones, err := getZonesFromTopology(top.GetRequisite())
	if err != nil {
		return reqZones, fmt.Errorf("could not get zones from requisite topology: %w", err)
	}

	prefZones, err := getZonesFromTopology(top.GetPreferred())
	if err != nil {
		return prefZones, fmt.Errorf("could not get zones from preferred topology: %w", err)
	}

	return append(reqZones, prefZones...), nil
}

func getZonesFromTopology(topList []*csi.Topology) ([]string, error) {
	zones := []string{}
	for _, top := range topList {
		if top.GetSegments() == nil {
			return nil, fmt.Errorf("topologies specified but no segments")
		}

		zone, err := getZoneFromSegment(top.GetSegments())
		if err != nil {
			return nil, fmt.Errorf("could not get zone from topology: %w", err)
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
			return nil, fmt.Errorf("storage Class labels cannot contain metadata label key %s", k)
		}

		result[k] = v
	}

	return result, nil
}

func (s *controllerServer) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	klog.V(4).Infof("CreateSnapshot called with request %+v", req)
	if len(req.Name) == 0 {
		return nil, status.Error(codes.InvalidArgument, "CreateSnapshot name must be provided")
	}
	volumeID := req.GetSourceVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "CreateSnapshot source volume ID must be provided")
	}
	if isMultishareVolId(volumeID) {
		return nil, status.Error(codes.InvalidArgument, "CreateSnapshot is not supported for multishare backed volumes")
	}

	if acquired := s.config.volumeLocks.TryAcquire(volumeID); !acquired {
		return nil, status.Errorf(codes.Aborted, util.VolumeOperationAlreadyExistsFmt, volumeID)
	}
	defer s.config.volumeLocks.Release(volumeID)

	filer, _, err := getFileInstanceFromID(volumeID)
	if err != nil {
		klog.Errorf("Failed to get instance for volumeID %v snapshot, error: %v", volumeID, err.Error())
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	filer.Project = s.config.cloud.Project
	// If parameters are empty we assume 'backup' type by default.
	if req.GetParameters() != nil {
		if _, err := util.IsSnapshotTypeSupported(req.GetParameters()); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	}

	// Check for existing snapshot
	backupUri, _, err := file.CreateBackpURI(filer, req.Name, util.GetBackupLocation(req.GetParameters()))
	if err != nil {
		return nil, err
	}
	backupInfo, err := s.config.fileService.GetBackup(ctx, backupUri)
	if err != nil {
		if !file.IsNotFoundErr(err) {
			return nil, status.Error(*file.CodeForError(err), err.Error())
		}
	} else {
		backupSourceCSIHandle, err := util.BackupVolumeSourceToCSIVolumeHandle(backupInfo.SourceInstance, backupInfo.SourceShare)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Cannot determine volume handle from back source instance %s, share %s", backupInfo.SourceInstance, backupInfo.SourceShare)
		}
		if backupSourceCSIHandle != volumeID {
			return nil, status.Errorf(codes.AlreadyExists, "Backup already exists with a different source volume %s, input source volume %s", backupSourceCSIHandle, volumeID)
		}
		// Check if backup is in the process of getting created.
		if backupInfo.Backup.State == "CREATING" || backupInfo.Backup.State == "FINALIZING" {
			return nil, status.Errorf(codes.DeadlineExceeded, "Backup %v not yet ready, current state %s", backupInfo.Backup.Name, backupInfo.Backup.State)
		}
		if backupInfo.Backup.State != "READY" {
			return nil, status.Errorf(codes.Internal, "Backup %v not yet ready, current state %s", backupInfo.Backup.Name, backupInfo.Backup.State)
		}
		tp, err := util.ParseTimestamp(backupInfo.Backup.CreateTime)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to parse create timestamp for backup %v", backupInfo.Backup.Name)
		}
		klog.V(4).Infof("CreateSnapshot success for volume %v, Backup Id: %v", volumeID, backupInfo.Backup.Name)
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
		klog.Errorf("Create snapshot for volume Id %s failed: %v", volumeID, err.Error())
		if err != nil {
			return nil, status.Error(*file.CodeForError(err), err.Error())
		}
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
	klog.V(4).Infof("CreateSnapshot succeeded for volume %v, Backup Id: %v", volumeID, backupObj.Name)
	return resp, nil
}

func (s *controllerServer) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	id := req.GetSnapshotId()
	if len(id) == 0 {
		return nil, status.Error(codes.InvalidArgument, "DeleteSnapshot snapshot Id must be provided")
	}

	isBackup, err := util.IsBackupHandle(id)
	if err != nil {
		// Sanity tests expects delete to pass for invalid handles.
		klog.Warningf("Could not parse snapshot handle %v", id)
		return &csi.DeleteSnapshotResponse{}, nil
	}

	if !isBackup {
		klog.Errorf("Deletion of volume snapshot type %q not supported", id)
		return nil, status.Error(codes.InvalidArgument, "deletion is only supported for volume snapshots of type backup")
	}

	backupInfo, err := s.config.fileService.GetBackup(ctx, id)
	if err != nil {
		if file.IsNotFoundErr(err) {
			klog.Infof("Volume snapshot with ID %v not found", id)
			return &csi.DeleteSnapshotResponse{}, nil
		}
		return nil, status.Error(*file.CodeForError(err), err.Error())
	}

	if backupInfo.Backup.State == "DELETING" {
		return nil, status.Errorf(codes.DeadlineExceeded, "Volume snapshot with ID %v is in state %s", id, backupInfo.Backup.State)
	}

	if err = s.config.fileService.DeleteBackup(ctx, id); err != nil {
		klog.Errorf("Delete snapshot for backup Id %s failed: %v", id, err.Error())
		return nil, status.Error(*file.CodeForError(err), err.Error())
	}

	return &csi.DeleteSnapshotResponse{}, nil
}
