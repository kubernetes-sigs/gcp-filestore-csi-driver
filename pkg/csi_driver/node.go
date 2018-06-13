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
	"os"

	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	"github.com/golang/glog"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/kubernetes/pkg/util/mount"
)

// nodeServer handles mounting and unmounting of GCFS volumes on a node
type nodeServer struct {
	driver  *GCFSDriver
	mounter mount.Interface
}

func newNodeServer(driver *GCFSDriver, mounter mount.Interface) csi.NodeServer {
	return &nodeServer{
		driver:  driver,
		mounter: mounter,
	}
}

// NodePublishVolume mounts the GCFS volume
func (s *nodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	glog.V(4).Infof("NodePublishVolume called with req: %#v", req)

	// TODO: make this idempotent. Multiple requests for the same volume can come in parallel, this needs to be seralized
	// We need something like the nestedpendingoperations

	// Validate arguments
	readOnly := req.GetReadonly()
	targetPath := req.GetTargetPath()
	if len(targetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "NodePublishVolume target path must be provided")
	}

	if err := s.driver.validateVolumeCapabilities([]*csi.VolumeCapability{req.GetVolumeCapability()}); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// Validate volume attributes
	attr := req.GetVolumeAttributes()
	if err := validateVolumeAttributes(attr); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// Check if mount already exists
	mounted, err := s.isDirMounted(targetPath)
	if err != nil {
		return nil, err
	}
	if mounted {
		// Already mounted
		// TODO: validate it's the corret mount
		return &csi.NodePublishVolumeResponse{}, nil
	}

	// Mount source
	source := fmt.Sprintf("%s:/%s", attr[attrIp], attr[attrVolume])

	// Mount options
	options := []string{}
	if readOnly {
		options = append(options, "ro")
	}
	if capMount := req.GetVolumeCapability().GetMount(); capMount != nil {
		options = append(options, capMount.GetMountFlags()...)
	}

	err = s.mounter.Mount(source, targetPath, "nfs", options)
	if err != nil {
		glog.Errorf("Mount %q failed, cleaning up", targetPath)
		if unmntErr := s.unmountPath(targetPath); unmntErr != nil {
			glog.Errorf("Unmount %q failed: %v", targetPath, unmntErr)
		}
		return nil, status.Error(codes.Internal, fmt.Sprintf("mount %q failed: %v", targetPath, err))
	}

	glog.V(4).Infof("Successfully mounted %s", targetPath)
	return &csi.NodePublishVolumeResponse{}, nil
}

// NodeUnpublishVolume unmounts the GCFS volume
func (s *nodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	glog.V(4).Infof("NodeUnpublishVolume called with args: %v", req)

	// TODO: make this idempotent

	// Validate arguments
	targetPath := req.GetTargetPath()
	if len(targetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "NodeUnpublishVolume target path must be provided")
	}

	if err := s.unmountPath(targetPath); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (s *nodeServer) NodeGetId(ctx context.Context, req *csi.NodeGetIdRequest) (*csi.NodeGetIdResponse, error) {
	glog.V(5).Infof("NodeGetId called with req: %#v", req)

	return &csi.NodeGetIdResponse{
		NodeId: s.driver.config.NodeID,
	}, nil
}

func (s *nodeServer) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	glog.V(5).Infof("NodeGetInfo called with req: %#v", req)

	return &csi.NodeGetInfoResponse{
		NodeId: s.driver.config.NodeID,
	}, nil
}

func (s *nodeServer) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	glog.V(5).Infof("NodeGetCapabilities called with req: %#v", req)

	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: s.driver.nscap,
	}, nil
}

// validateVolumeAttributes checks for all the necessary fields for mounting the volume
func validateVolumeAttributes(attr map[string]string) error {
	// TODO: validate ip syntax
	if attr[attrIp] == "" {
		return fmt.Errorf("volume attribute %v not set", attrIp)
	}
	// TODO: validate allowed characters
	if attr[attrVolume] == "" {
		return fmt.Errorf("volume attribute %v not set", attrVolume)
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

// unmountPath unmounts the given path if it a mount point
func (s *nodeServer) unmountPath(targetPath string) error {
	mounted, err := s.isDirMounted(targetPath)
	if os.IsNotExist(err) {
		// Volume already unmounted
		glog.V(4).Infof("Mount point %q already unmounted", targetPath)
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to check mount point %q: %v", targetPath, err)
	}

	if mounted {
		glog.V(4).Infof("Unmounting %q", targetPath)
		err := s.mounter.Unmount(targetPath)
		if err != nil {
			return fmt.Errorf("unmount %q failed: %v", targetPath, err)
		}
	}
	return nil
}
