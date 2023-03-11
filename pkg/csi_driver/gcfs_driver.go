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
	"context"
	"fmt"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
	mount "k8s.io/mount-utils"
	cloud "sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider"
	metadataservice "sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/metadata"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/metrics"
	lockrelease "sigs.k8s.io/gcp-filestore-csi-driver/pkg/releaselock"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/util"
)

type GCFSDriverConfig struct {
	Name             string          // Driver name
	Version          string          // Driver version
	NodeName         string          // Node name
	RunController    bool            // Run CSI controller service
	RunNode          bool            // Run CSI node service
	Mounter          mount.Interface // Mount library
	Cloud            *cloud.Cloud    // Cloud provider
	MetadataService  metadataservice.Service
	EnableMultishare bool
	Metrics          *metrics.MetricsManager
	EcfsDescription  string
	IsRegional       bool
	ClusterName      string
	FeatureOptions   *GCFSDriverFeatureOptions
}

type GCFSDriver struct {
	config *GCFSDriverConfig

	// CSI RPC servers
	ids csi.IdentityServer
	ns  csi.NodeServer
	cs  csi.ControllerServer

	// Plugin capabilities
	vcap  map[csi.VolumeCapability_AccessMode_Mode]*csi.VolumeCapability_AccessMode
	cscap []*csi.ControllerServiceCapability
	nscap []*csi.NodeServiceCapability
}

type GCFSDriverFeatureOptions struct {
	// FeatureLockRelease will enable the NFS lock release feature if sets to true.
	FeatureLockRelease *FeatureLockRelease
}

type FeatureLockRelease struct {
	Enabled bool
	Config  *lockrelease.LockReleaseControllerConfig
}

func NewGCFSDriver(config *GCFSDriverConfig) (*GCFSDriver, error) {
	if config.Name == "" {
		return nil, fmt.Errorf("driver name missing")
	}
	if config.Version == "" {
		return nil, fmt.Errorf("driver version missing")
	}
	if !config.RunController && !config.RunNode {
		return nil, fmt.Errorf("must run at least one controller or node service")
	}

	driver := &GCFSDriver{
		config: config,
		vcap:   map[csi.VolumeCapability_AccessMode_Mode]*csi.VolumeCapability_AccessMode{},
	}

	vcam := []csi.VolumeCapability_AccessMode_Mode{
		csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
		csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY,
		csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY,
		csi.VolumeCapability_AccessMode_MULTI_NODE_SINGLE_WRITER,
		csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
	}
	driver.addVolumeCapabilityAccessModes(vcam)

	// Setup RPC servers
	driver.ids = newIdentityServer(driver)
	if config.RunNode {
		nscap := []csi.NodeServiceCapability_RPC_Type{
			csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
			csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
		}
		ns, err := newNodeServer(driver, config.Mounter, config.MetadataService, config.FeatureOptions)
		if err != nil {
			return nil, err
		}
		driver.ns = ns
		driver.addNodeServiceCapabilities(nscap)
	}
	if config.RunController {
		csc := []csi.ControllerServiceCapability_RPC_Type{
			csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
			csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
			csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT,
		}
		driver.addControllerServiceCapabilities(csc)

		// Configure controller server
		driver.cs = newControllerServer(&controllerServerConfig{
			driver:           driver,
			fileService:      config.Cloud.File,
			cloud:            config.Cloud,
			volumeLocks:      util.NewVolumeLocks(),
			enableMultishare: config.EnableMultishare,
			metricsManager:   config.Metrics,
			ecfsDescription:  config.EcfsDescription,
			isRegional:       config.IsRegional,
			clusterName:      config.ClusterName,
			features:         config.FeatureOptions,
		})
	}

	return driver, nil
}

func (driver *GCFSDriver) addVolumeCapabilityAccessModes(vc []csi.VolumeCapability_AccessMode_Mode) error {
	for _, c := range vc {
		klog.Infof("Enabling volume access mode: %v", c.String())
		mode := NewVolumeCapabilityAccessMode(c)
		driver.vcap[mode.Mode] = mode
	}
	return nil
}

func (driver *GCFSDriver) validateVolumeCapabilities(caps []*csi.VolumeCapability) error {
	if len(caps) == 0 {
		return fmt.Errorf("volume capabilities must be provided")
	}

	for _, c := range caps {
		if err := driver.validateVolumeCapability(c); err != nil {
			return err
		}
	}
	return nil
}

func (driver *GCFSDriver) validateVolumeCapability(c *csi.VolumeCapability) error {
	if c == nil {
		return fmt.Errorf("volume capability must be provided")
	}

	// Validate access mode
	accessMode := c.GetAccessMode()
	if accessMode == nil {
		return fmt.Errorf("volume capability access mode not set")
	}
	if driver.vcap[accessMode.Mode] == nil {
		return fmt.Errorf("driver does not support access mode: %v", accessMode.Mode.String())
	}

	// Validate access type
	accessType := c.GetAccessType()
	if accessType == nil {
		return fmt.Errorf("volume capability access type not set")
	}
	mountType := c.GetMount()
	if mountType == nil {
		return fmt.Errorf("driver only supports mount access type volume capability")
	}
	if mountType.FsType != "" {
		// TODO: uncomment after https://github.com/kubernetes-csi/external-provisioner/issues/328 is fixed.
		// return fmt.Errorf("driver does not support fstype %v", mountType.FsType)
	}
	// TODO: check if we want to whitelist/blacklist certain mount options
	return nil
}

func (driver *GCFSDriver) addControllerServiceCapabilities(cl []csi.ControllerServiceCapability_RPC_Type) error {
	var csc []*csi.ControllerServiceCapability
	for _, c := range cl {
		klog.Infof("Enabling controller service capability: %v", c.String())
		csc = append(csc, NewControllerServiceCapability(c))
	}
	driver.cscap = csc
	return nil
}

func (driver *GCFSDriver) addNodeServiceCapabilities(nl []csi.NodeServiceCapability_RPC_Type) error {
	var nsc []*csi.NodeServiceCapability
	for _, n := range nl {
		klog.Infof("Enabling node service capability: %v", n.String())
		nsc = append(nsc, NewNodeServiceCapability(n))
	}
	driver.nscap = nsc
	return nil
}

func (driver *GCFSDriver) ValidateControllerServiceRequest(c csi.ControllerServiceCapability_RPC_Type) error {
	if c == csi.ControllerServiceCapability_RPC_UNKNOWN {
		return nil
	}

	for _, cap := range driver.cscap {
		if c == cap.GetRpc().Type {
			return nil
		}
	}

	return status.Error(codes.InvalidArgument, "Invalid controller service request")
}

func (driver *GCFSDriver) Run(endpoint string) {
	klog.Infof("Running driver: %v", driver.config.Name)

	// Start the nonblocking GRPC.
	s := NewNonBlockingGRPCServer()
	s.Start(endpoint, driver.ids, driver.cs, driver.ns)
	if driver.config.RunNode && driver.config.FeatureOptions.FeatureLockRelease.Enabled {
		// Start the lock release controller on node driver.
		driver.ns.(*nodeServer).lockReleaseController.Run(context.Background())
	}
	s.Wait()
}
