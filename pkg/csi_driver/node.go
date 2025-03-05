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
	"net"
	"os"
	"runtime"
	"strings"
	"time"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	mount "k8s.io/mount-utils"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/metadata"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/metrics"
	lockrelease "sigs.k8s.io/gcp-filestore-csi-driver/pkg/releaselock"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/util"
)

const (
	optionSmbUser     = "smbUser"
	optionSmbPassword = "smbPassword"
)

var (
	// For testing purposes
	goOs = runtime.GOOS
)

// nodeServer handles mounting and unmounting of GCFS volumes on a node
// TODO(b/375481562): refactor config map utils & remove node driver's dependency on lockReleaseController
type nodeServer struct {
	driver                *GCFSDriver
	mounter               *mount.SafeFormatAndMount
	metaService           metadata.Service
	volumeLocks           *util.VolumeLocks
	lockReleaseController *lockrelease.LockReleaseController
	features              *GCFSDriverFeatureOptions
}

func newNodeServer(driver *GCFSDriver, mounter *mount.SafeFormatAndMount, metaService metadata.Service, featureOptions *GCFSDriverFeatureOptions) (csi.NodeServer, error) {
	ns := &nodeServer{
		driver:      driver,
		mounter:     mounter,
		metaService: metaService,
		volumeLocks: util.NewVolumeLocks(),
		features:    featureOptions,
	}
	if ns.features.FeatureLockRelease.Enabled {
		config, err := rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
		client, err := kubernetes.NewForConfig(config)
		if err != nil {
			return nil, err
		}
		lc, err := lockrelease.NewLockReleaseController(client, ns.features.FeatureLockRelease.Config, nil)
		if err != nil {
			return nil, err
		}
		ns.lockReleaseController = lc
	}
	return ns, nil
}

// NodePublishVolume bind mounts from the source staging path, where the GCFS volume is mounted.
func (s *nodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	// Validate arguments
	readOnly := req.GetReadonly()
	targetPath := req.GetTargetPath()
	stagingTargetPath := req.GetStagingTargetPath()
	if len(targetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "NodePublishVolume target path must be provided")
	}
	if len(stagingTargetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "NodePublishVolume stagingTargetPath path must be provided")
	}

	if err := s.driver.validateVolumeCapabilities([]*csi.VolumeCapability{req.GetVolumeCapability()}); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// Acquire a lock on the target path instead of volumeID, since we do not want to serialize multiple node publish calls on the same volume.
	if acquired := s.volumeLocks.TryAcquire(targetPath); !acquired {
		return nil, status.Errorf(codes.Aborted, util.VolumeOperationAlreadyExistsFmt, targetPath)
	}
	defer s.volumeLocks.Release(targetPath)

	var err error
	// FileSystem type
	fstype := "nfs"
	// Mount options
	options := []string{"bind"}
	// Windows specific values (TODO: Revisit windows specific logic for bind mount)
	if goOs == "windows" {
		fstype = "cifs"

		// Login credentials
		secrets := req.GetSecrets()
		if err := validateSmbNodePublishSecrets(secrets); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		options = append(options, secrets[optionSmbUser])
		options = append(options, secrets[optionSmbPassword])

		//TODO: Remove this workaround after https://github.com/kubernetes/kubernetes/issues/75535
		if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		// TODO: If target path does not exist create it and then proceed to mount.
		// (https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/issues/47)
		// Check kubernetes/kubernetes#75535. CO may create only the parent directory.
		mounted, err := s.isDirMounted(targetPath)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		if mounted {
			return &csi.NodePublishVolumeResponse{}, nil
		}
		if os.IsNotExist(err) {
			if mkdirErr := os.MkdirAll(targetPath, 0750); mkdirErr != nil {
				return nil, status.Errorf(codes.Internal, "mkdir failed on path %s (%v)", targetPath, mkdirErr.Error())
			}
		}
	}

	if readOnly {
		options = append(options, "ro")
	}
	if capMount := req.GetVolumeCapability().GetMount(); capMount != nil {
		options = append(options, capMount.GetMountFlags()...)
	}

	err = s.mounter.Mount(stagingTargetPath, targetPath, fstype, options)
	if err != nil {
		klog.Errorf("Mount %q failed on node %s, cleaning up", targetPath, s.driver.config.NodeName)
		if unmntErr := mount.CleanupMountPoint(stagingTargetPath, s.mounter, false /* extensiveMountPointCheck */); unmntErr != nil {
			klog.Errorf("Unmount %q failed on node %s: %v", targetPath, s.driver.config.NodeName, unmntErr.Error())
		}

		return nil, status.Errorf(codes.Internal, "mount %q failed: %v", targetPath, err.Error())
	}

	klog.V(4).Infof("Successfully mounted %s on node %s", targetPath, s.driver.config.NodeName)
	return &csi.NodePublishVolumeResponse{}, nil
}

// NodeUnpublishVolume unmounts the GCFS volume
func (s *nodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	// Validate arguments
	targetPath := req.GetTargetPath()
	if len(targetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "NodeUnpublishVolume target path must be provided")
	}

	// Acquire a lock on the target path instead of volumeID, since we do not want to serialize multiple node unpublish calls on the same volume.
	if acquired := s.volumeLocks.TryAcquire(targetPath); !acquired {
		return nil, status.Errorf(codes.Aborted, util.VolumeOperationAlreadyExistsFmt, targetPath)
	}
	defer s.volumeLocks.Release(targetPath)

	if err := mount.CleanupMountPoint(targetPath, s.mounter, false /* extensiveMountPointCheck */); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (s *nodeServer) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{
		NodeId: s.driver.config.NodeName,
		AccessibleTopology: &csi.Topology{
			Segments: map[string]string{TopologyKeyZone: s.metaService.GetZone()},
		},
	}, nil
}

func (s *nodeServer) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: s.driver.nscap,
	}, nil
}

func (s *nodeServer) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	if len(req.VolumeId) == 0 {
		return nil, status.Error(codes.InvalidArgument, "NodeGetVolumeStats volume ID was empty")
	}
	if len(req.VolumePath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "NodeGetVolumeStats volume path was empty")
	}

	_, err := os.Lstat(req.VolumePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Errorf(codes.NotFound, "path %s does not exist", req.VolumePath)
		}
		return nil, status.Errorf(codes.Internal, "unknown error when stat on %s: %v", req.VolumePath, err.Error())
	}

	available, capacity, used, inodesFree, inodes, inodesUsed, err := getFSStat(req.VolumePath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get fs info on path %s: %v", req.VolumePath, err.Error())
	}

	return &csi.NodeGetVolumeStatsResponse{
		Usage: []*csi.VolumeUsage{
			{
				Unit:      csi.VolumeUsage_BYTES,
				Available: available,
				Total:     capacity,
				Used:      used,
			},
			{
				Unit:      csi.VolumeUsage_INODES,
				Available: inodesFree,
				Total:     inodes,
				Used:      inodesUsed,
			},
		},
	}, nil

}

func (s *nodeServer) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	// Validate Arguments
	volumeID := req.GetVolumeId()
	stagingTargetPath := req.GetStagingTargetPath()
	volumeCapability := req.GetVolumeCapability()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "NodeStageVolume Volume ID must be provided")
	}
	if len(stagingTargetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "NodeStageVolume Staging Target Path must be provided")
	}
	if volumeCapability == nil {
		return nil, status.Error(codes.InvalidArgument, "NodeStageVolume Volume Capability must be provided")
	}

	if err := validateVolumeCapability(volumeCapability); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "VolumeCapability is invalid: %v", err.Error())
	}

	// Validate volume attributes
	var source string
	attr := req.GetVolumeContext()
	if isMultishareVolId(volumeID) {
		if err := validateMultishareVolumeAttributes(attr); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		_, _, _, _, shareName, err := parseMultishareVolId(volumeID)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		source = fmt.Sprintf("%s:/%s", attr[attrIP], shareName)
	} else {
		if err := validateVolumeAttributes(attr); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		source = fmt.Sprintf("%s:/%s", attr[attrIP], attr[attrVolume])
	}

	if acquired := s.volumeLocks.TryAcquire(volumeID); !acquired {
		return nil, status.Errorf(codes.Aborted, util.VolumeOperationAlreadyExistsFmt, volumeID)
	}
	defer s.volumeLocks.Release(volumeID)

	// Mount source
	mounted, err := s.isDirMounted(stagingTargetPath)
	needsCreateDir := false
	if err != nil {
		if os.IsNotExist(err) {
			needsCreateDir = true
		} else {
			return nil, err
		}
	}

	fileProtocol, ok := attr[attrFileProtocol]
	if !ok {
		fileProtocol = v3FileProtocol
	}

	if mounted {
		if fileProtocol == v3FileProtocol && s.features.FeatureLockRelease.Enabled {
			klog.V(4).Infof("NodeStageVolume mounted volume %v to staging target path %s, mount already exists on node %s. Proceed to lock info configmap updates", volumeID, stagingTargetPath, s.driver.config.NodeName)
			if err := s.nodeStageVolumeUpdateLockInfo(ctx, req); err != nil {
				return nil, status.Errorf(codes.Internal, "failed to store lock info after NodeStageVolume succeeded on volume %v to path %s: %v", volumeID, stagingTargetPath, err.Error())
			}
		}
		klog.V(4).Infof("NodeStageVolume succeeded on volume %v to staging target path %s on node %s, mount already exists.", volumeID, stagingTargetPath, s.driver.config.NodeName)
		return &csi.NodeStageVolumeResponse{}, nil
	}

	if needsCreateDir {
		klog.V(4).Infof("NodeStageVolume attempting mkdir for path %s on node %s", stagingTargetPath, s.driver.config.NodeName)
		if err := os.MkdirAll(stagingTargetPath, 0750); err != nil {
			return nil, fmt.Errorf("mkdir failed for path %s (%w)", stagingTargetPath, err)
		}
	}

	fstype := "nfs"
	options := []string{}
	if mnt := volumeCapability.GetMount(); mnt != nil {
		for _, flag := range mnt.MountFlags {
			options = append(options, flag)
		}
	}

	err = s.mounter.Mount(source, stagingTargetPath, fstype, options)
	if err != nil {
		klog.Errorf("Mount %q failed on node %s, cleaning up", stagingTargetPath, s.driver.config.NodeName)
		if unmntErr := mount.CleanupMountPoint(stagingTargetPath, s.mounter, false /* extensiveMountPointCheck */); unmntErr != nil {
			klog.Errorf("Unmount %q failed on node %s: %v", stagingTargetPath, s.driver.config.NodeName, unmntErr.Error())
		}
		return nil, status.Errorf(codes.Internal, "mount %q failed on node %s: %v", stagingTargetPath, s.driver.config.NodeName, err.Error())
	}

	if fileProtocol == v3FileProtocol && s.features.FeatureLockRelease.Enabled {
		klog.V(4).Infof("NodeStageVolume mounted volume %v to staging target path %s on node %s, proceed to lock info configmap updates.", volumeID, stagingTargetPath, s.driver.config.NodeName)
		if err := s.nodeStageVolumeUpdateLockInfo(ctx, req); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to store lock info after NodeStageVolume succeeded on volume %v to path %s: %v", volumeID, stagingTargetPath, err.Error())
		}
	}

	klog.V(4).Infof("NodeStageVolume succeeded on volume %v to path %s on node %s", volumeID, stagingTargetPath, s.driver.config.NodeName)
	return &csi.NodeStageVolumeResponse{}, nil
}

func (s *nodeServer) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	// Validate arguments
	volumeID := req.GetVolumeId()
	stagingTargetPath := req.GetStagingTargetPath()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "NodeUnstageVolume Volume ID must be provided")
	}
	if len(stagingTargetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "NodeUnstageVolume Staging Target Path must be provided")
	}

	if acquired := s.volumeLocks.TryAcquire(volumeID); !acquired {
		return nil, status.Errorf(codes.Aborted, util.VolumeOperationAlreadyExistsFmt, volumeID)
	}
	defer s.volumeLocks.Release(volumeID)

	if err := mount.CleanupMountPoint(stagingTargetPath, s.mounter, false /* extensiveMountPointCheck */); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	if s.features.FeatureLockRelease.Enabled {
		klog.V(4).Infof("NodeUnstageVolume succeeded on volume %v from staging target path %s on node %s, proceed to lock info configmap updates", volumeID, stagingTargetPath, s.driver.config.NodeName)
		if err := s.nodeUnstageVolumeUpdateLockInfo(ctx, req); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to update lock info after NodeUnstageVolume succeeded on volume %v from staging target path %s on node %s: %v", volumeID, stagingTargetPath, s.driver.config.NodeName, err.Error())
		}
	}

	klog.V(4).Infof("NodeUnstageVolume succeeded on volume %v from staging target path %s on node %s", volumeID, stagingTargetPath, s.driver.config.NodeName)
	return &csi.NodeUnstageVolumeResponse{}, nil
}

func validateVolumeCapability(vc *csi.VolumeCapability) error {
	if err := validateAccessMode(vc.GetAccessMode()); err != nil {
		return err
	}

	blk := vc.GetBlock()
	mnt := vc.GetMount()
	if mnt == nil && blk == nil {
		return fmt.Errorf("must specify an access type")
	}

	if mnt != nil && blk != nil {
		return fmt.Errorf("specified both mount and block access types")
	}

	if blk != nil {
		return fmt.Errorf("block access type not supported")
	}
	return nil
}

func validateAccessMode(am *csi.VolumeCapability_AccessMode) error {
	if am == nil {
		return fmt.Errorf("access mode is nil")
	}

	switch am.GetMode() {
	case csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER:
	case csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY:
	case csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY:
	case csi.VolumeCapability_AccessMode_MULTI_NODE_SINGLE_WRITER:
	case csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER:
	default:
		return fmt.Errorf("unkown access mode %v", am.GetMode())
	}
	return nil
}

// validateVolumeAttributes checks for all the necessary fields for mounting the volume
func validateVolumeAttributes(attr map[string]string) error {
	instanceip, ok := attr[attrIP]
	if !ok {
		return fmt.Errorf("volume attribute key %v not set", attrIP)
	}
	// Check for valid IPV4 address.
	if net.ParseIP(instanceip) == nil {
		return fmt.Errorf("invalid IP address %v in volume attributes", instanceip)
	}

	_, ok = attr[attrVolume]
	if !ok {
		return fmt.Errorf("volume attribute key %v not set", attrVolume)
	}
	// TODO: validate allowed characters
	if attr[attrVolume] == "" {
		return fmt.Errorf("volume attribute %v not set", attrVolume)
	}
	return nil
}

func validateSmbNodePublishSecrets(secrets map[string]string) error {
	if secrets[optionSmbUser] == "" {
		return fmt.Errorf("secret %v not set", optionSmbUser)
	}

	if secrets[optionSmbPassword] == "" {
		return fmt.Errorf("secret %v not set", optionSmbPassword)
	}
	return nil
}

// isDirMounted checks if the path is already a mount point
func (s *nodeServer) isDirMounted(targetPath string) (bool, error) {
	// Check if mount already exists
	// TODO(msau): check why in-tree uses IsNotMountPoint
	// something related to squash and not having permissions to lstat
	notMnt, err := s.mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil {
		return false, err
	}
	if !notMnt {
		// Already mounted
		return true, nil
	}
	return false, nil
}

func validateMultishareVolumeAttributes(attr map[string]string) error {
	instanceip, ok := attr[attrIP]
	if !ok {
		return fmt.Errorf("volume attribute key %v not set", attrIP)
	}
	// Check for valid IPV4 address.
	if net.ParseIP(instanceip) == nil {
		return fmt.Errorf("invalid IP address %v in volume attributes", instanceip)
	}
	return nil
}

func getFSStat(path string) (available, capacity, used, inodesFree, inodes, inodesUsed int64, err error) {
	statfs := &unix.Statfs_t{}
	err = unix.Statfs(path, statfs)
	if err != nil {
		err = fmt.Errorf("failed to get fs info on path %s: %w", path, err)
		return
	}

	// Available is blocks available * fragment size to root user
	available = int64(statfs.Bfree) * int64(statfs.Bsize)
	// Capacity is total block count * fragment size
	capacity = int64(statfs.Blocks) * int64(statfs.Bsize)
	// Usage is block being used * fragment size (aka block size).
	used = (int64(statfs.Blocks) - int64(statfs.Bfree)) * int64(statfs.Bsize)
	inodes = int64(statfs.Files)
	inodesFree = int64(statfs.Ffree)
	inodesUsed = inodes - inodesFree
	return
}

// nodeStageVolumeUpdateLockInfo updates lock info after NodeStageVolume succeed.
func (s *nodeServer) nodeStageVolumeUpdateLockInfo(ctx context.Context, req *csi.NodeStageVolumeRequest) error {
	volumeID := req.GetVolumeId()
	// No-op if filestore instance not support lock release.
	attr := req.GetVolumeContext()
	if val, ok := attr[attrSupportLockRelease]; !ok || strings.ToLower(val) != "true" {
		klog.Infof("Lock release is not support on volume %s: missing volume attribute %s. The volume is not dynamically provisioned or the filestore instance is not in enterprise tier", volumeID, attrSupportLockRelease)
		return nil
	}

	// Update the configMap after successful nfs mount operation.
	nodeName := s.driver.config.NodeName
	configmapName := lockrelease.ConfigMapNamePrefix + nodeName
	klog.Infof("NodeStageVolume getting configmap %s/%s for volume %s", util.ManagedFilestoreCSINamespace, configmapName, volumeID)
	start := time.Now()
	cm, err := s.lockReleaseController.GetConfigMap(ctx, configmapName, util.ManagedFilestoreCSINamespace)
	duration := time.Since(start)
	s.lockReleaseController.RecordKubeAPIMetrics(err, metrics.ConfigMapResourceType, metrics.GetOpType, metrics.NodeStageOpSource, duration)
	if err != nil {
		klog.Errorf("NodeStageVolume failed to get configmap %s/%s for volume %s: %v", util.ManagedFilestoreCSINamespace, configmapName, volumeID, err)
		return err
	}

	lockInfoKey, err := s.generateLockInfoKeyFromVolumeID(volumeID)
	if err != nil {
		klog.Errorf("NodeStageVolume failed to generate lock info key for volume %s: %v", volumeID, err)
		return err
	}

	// Create or update the configmap with lock info.
	filestoreIP := attr[attrIP]
	if cm == nil {
		data := map[string]string{lockInfoKey: filestoreIP}
		klog.Infof("NodeStageVolume creating configmap %+v with data %v for volume %s", klog.KObj(cm), data, volumeID)
		start := time.Now()
		cm, err := s.lockReleaseController.CreateConfigMapWithData(ctx, configmapName, util.ManagedFilestoreCSINamespace, data)
		duration := time.Since(start)
		s.lockReleaseController.RecordKubeAPIMetrics(err, metrics.ConfigMapResourceType, metrics.CreateOpType, metrics.NodeStageOpSource, duration)
		if err != nil {
			klog.Errorf("NodeStageVolume failed to create configmap %+v with data %s for volume %s: %v", klog.KObj(cm), data, volumeID, err)
			return err
		}
		klog.Infof("NodeStageVolume successfully created configmap %+v with data %v for volume %s", klog.KObj(cm), cm.Data, volumeID)
		return nil
	}

	if err := s.lockReleaseController.UpdateConfigMapWithKeyValue(ctx, cm, lockInfoKey, filestoreIP); err != nil {
		klog.Errorf("NodeStageVolume failed to update configmap %+v with lock info {%s: %s} for volume %s: %v", klog.KObj(cm), lockInfoKey, filestoreIP, volumeID, err)
		return err
	}

	return nil
}

// nodeUnstageVolumeUpdateLockInfo updates lock info after NodeUnStageVolume succeed.
func (s *nodeServer) nodeUnstageVolumeUpdateLockInfo(ctx context.Context, req *csi.NodeUnstageVolumeRequest) error {
	volumeID := req.GetVolumeId()
	nodeName := s.driver.config.NodeName
	configmapName := lockrelease.ConfigMapNamePrefix + nodeName
	klog.Infof("NodeUnstageVolume getting configmap %s/%s for volume %s", util.ManagedFilestoreCSINamespace, configmapName, volumeID)
	start := time.Now()
	cm, err := s.lockReleaseController.GetConfigMap(ctx, configmapName, util.ManagedFilestoreCSINamespace)
	duration := time.Since(start)
	s.lockReleaseController.RecordKubeAPIMetrics(err, metrics.ConfigMapResourceType, metrics.GetOpType, metrics.NodeUnstageOpSource, duration)
	if err != nil {
		klog.Errorf("NodeStageVolume failed to get configmap %+v for volume %s: %v", klog.KObj(cm), volumeID, err)
		return err
	}
	if cm == nil {
		klog.Infof("NodeUnstageVolume skipped updating lock info for volume %s: configmap %s/%s not found", volumeID, util.ManagedFilestoreCSINamespace, configmapName)
		return nil
	}

	lockInfoKey, err := s.generateLockInfoKeyFromVolumeID(volumeID)
	if err != nil {
		klog.Errorf("NodeUnstageVolume failed to generate lock info key for volume %s: %v", volumeID, err)
		return err
	}

	if err := s.lockReleaseController.RemoveKeyFromConfigMap(ctx, cm, lockInfoKey); err != nil {
		klog.Infof("NodeUnstageVolume failed to remove key %s from configmap %+v for volume %s: %v", lockInfoKey, klog.KObj(cm), volumeID, err)
		return err
	}

	return nil
}

// generateLockInfoKeyFromVolumeID generates a configmap key for the given volumeID.
// The configmap will store key-value pairs in format:
// {projectID}.{location}.{filestoreName}.{shareName}.{gkeNodeID}.{gkeNodeInternalIP}: <filestoreIP>
func (s *nodeServer) generateLockInfoKeyFromVolumeID(volumeID string) (string, error) {
	var lockInfoKey string
	nodeID := s.metaService.GetInstanceID()
	nodeInternalIP := s.metaService.GetInternalIP()

	if isMultishareVolId(volumeID) {
		_, project, location, filestoreName, shareName, err := parseMultishareVolId(volumeID)
		if err != nil {
			return "", err
		}
		lockInfoKey = lockrelease.GenerateConfigMapKey(project, location, filestoreName, shareName, nodeID, nodeInternalIP)
		return lockInfoKey, nil
	}

	filestoreInstance, _, err := getFileInstanceFromID(volumeID)
	if err != nil {
		return "", err
	}
	project := s.metaService.GetProject()
	lockInfoKey = lockrelease.GenerateConfigMapKey(project, filestoreInstance.Location, filestoreInstance.Name, filestoreInstance.Volume.Name, nodeID, nodeInternalIP)
	return lockInfoKey, nil
}
