/*
Copyright 2022 The Kubernetes Authors.

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
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/klog"
	cloud "sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/file"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/util"
)

const (
	modeMultishare = "modeMultishare"

	methodCreateVolume         = "CreateVolume"
	methodDeleteVolume         = "DeleteVolume"
	methodExpandVolume         = "ExpandVolume"
	ecfsDataPlaneVersionFormat = "GoogleReserved-CustomVMImage=clh.image.ems.path:projects/%s/global/images/ems-filestore-scaleout-%s"
)

// MultishareController handles CSI calls for volumes which use Filestore multishare instances.
type MultishareController struct {
	driver          *GCFSDriver
	fileService     file.Service
	cloud           *cloud.Cloud
	opsManager      *MultishareOpsManager
	volumeLocks     *util.VolumeLocks
	ecfsDescription string
}

func NewMultishareController(driver *GCFSDriver, fileService file.Service, cloud *cloud.Cloud, volumeLock *util.VolumeLocks, ecfsDescription string) *MultishareController {
	return &MultishareController{
		opsManager:      NewMultishareOpsManager(cloud),
		driver:          driver,
		fileService:     fileService,
		cloud:           cloud,
		volumeLocks:     volumeLock,
		ecfsDescription: ecfsDescription,
	}
}

func (m *MultishareController) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	name := req.GetName()
	if len(name) == 0 {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume name must be provided")
	}
	if err := m.driver.validateVolumeCapabilities(req.GetVolumeCapabilities()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	instanceScPrefix, err := getInstanceSCPrefix(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	klog.Infof("CreateVolume called for multishare with request %+v", req)
	if acquired := m.volumeLocks.TryAcquire(name); !acquired {
		return nil, status.Errorf(codes.Aborted, util.VolumeOperationAlreadyExistsFmt, name)
	}
	defer m.volumeLocks.Release(name)

	// If no eligible instance found, the ops manager may decide to create a new instance. Prepare a multishare instacne object for such a scenario.
	instance, err := m.generateNewMultishareInstance(util.NewMultishareInstancePrefix+string(uuid.NewUUID()), req)
	if err != nil {
		return nil, err
	}

	workflow, share, err := m.opsManager.setupEligibleInstanceAndStartWorkflow(ctx, req, instance)
	if err != nil {
		return nil, err
	}

	if share != nil {
		return m.getShareAndGenerateCSICreateVolumeResponse(ctx, instanceScPrefix, share)
	}

	// lock released. poll for op.
	err = m.waitOnWorkflow(ctx, workflow)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Create Volume failed, operation %q poll error: %v", workflow.opName, err)
	}

	klog.Infof("Poll for operation %s (type %s) completed", workflow.opName, workflow.opType.String())
	if workflow.opType == util.ShareCreate {
		return m.getShareAndGenerateCSICreateVolumeResponse(ctx, instanceScPrefix, workflow.share)
	}

	var shareCreateWorkflow *Workflow
	var newShare *file.Share
	switch workflow.opType {
	case util.InstanceCreate, util.InstanceUpdate:
		newShare, err = generateNewShare(util.ConvertVolToShareName(req.Name), workflow.instance, req)
		if err != nil {
			return nil, err
		}
		shareCreateWorkflow, err = m.opsManager.startShareCreateWorkflowSafe(ctx, newShare)
		if err != nil {
			return nil, err
		}
	default:
		return nil, status.Errorf(codes.Internal, "Create Volume failed, unknown workflow %v detected", workflow.opType)
	}

	// lock released. poll for share create op.
	err = m.waitOnWorkflow(ctx, shareCreateWorkflow)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s operation %q poll error: %v", shareCreateWorkflow.opType.String(), shareCreateWorkflow.opName, err)
	}
	return m.getShareAndGenerateCSICreateVolumeResponse(ctx, instanceScPrefix, newShare)
}

func (m *MultishareController) getShareAndGenerateCSICreateVolumeResponse(ctx context.Context, instancePrefix string, s *file.Share) (*csi.CreateVolumeResponse, error) {
	share, err := m.cloud.File.GetShare(ctx, s)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	if share.State != "READY" {
		return nil, status.Errorf(codes.Aborted, "share %s not ready, state %s", share.Name, share.State)
	}
	return generateCSICreateVolumeResponse(instancePrefix, share)
}

func (m *MultishareController) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	_, project, location, instanceName, shareName, err := parseMultishareVolId(req.VolumeId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	klog.V(4).Infof("DeleteVolume called for multishare with request %+v", req)

	if acquired := m.volumeLocks.TryAcquire(req.VolumeId); !acquired {
		return nil, status.Errorf(codes.Aborted, util.VolumeOperationAlreadyExistsFmt, req.VolumeId)
	}
	defer m.volumeLocks.Release(req.VolumeId)

	share, err := m.cloud.File.GetShare(ctx, &file.Share{
		Parent: &file.MultishareInstance{
			Project:  project,
			Location: location,
			Name:     instanceName,
		},
		Name: shareName,
	})
	if err != nil {
		// If share not found, proceed to instance/shrink check.
		if file.IsNotFoundErr(err) {
			err = m.startAndWaitForInstanceDeleteOrShrink(ctx, req.VolumeId)
			if err != nil {
				return nil, err
			}
			return &csi.DeleteVolumeResponse{}, nil
		}

		return nil, status.Error(codes.Internal, err.Error())
	}

	workflow, err := m.opsManager.checkAndStartShareDeleteWorkflow(ctx, share)
	if err != nil {
		return nil, err
	}

	// Poll for share delete to complete
	if workflow != nil {
		err = m.waitOnWorkflow(ctx, workflow)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "%s operation %q poll error: %v", workflow.opType.String(), workflow.opName, err)
		}
	}

	// Check whether instance can be shrinked or deleted.
	err = m.startAndWaitForInstanceDeleteOrShrink(ctx, req.VolumeId)
	if err != nil {
		return nil, err
	}
	return &csi.DeleteVolumeResponse{}, nil
}

func (m *MultishareController) startAndWaitForInstanceDeleteOrShrink(ctx context.Context, csiVolId string) error {
	_, project, location, instanceName, _, err := parseMultishareVolId(csiVolId)
	if err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}

	// Check whether instance can be shrinked or deleted.
	workflow, err := m.opsManager.checkAndStartInstanceDeleteOrShrinkWorkflow(ctx, &file.MultishareInstance{
		Project:  project,
		Location: location,
		Name:     instanceName,
	})
	if err != nil {
		return err
	}

	// return if no-op
	if workflow == nil {
		return nil
	}
	err = m.waitOnWorkflow(ctx, workflow)
	if err != nil {
		return status.Errorf(codes.Internal, "%s operation %q poll error: %v", workflow.opType.String(), workflow.opName, err)
	}
	return nil
}

func (m *MultishareController) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	// Handle higher level csi params validation, try locks
	// Initiate share workflow by calling Multishare OpsManager functions
	// Prepare and return csi response

	volumeId := req.GetVolumeId()
	if len(volumeId) == 0 {
		return nil, status.Error(codes.InvalidArgument, "ControllerExpandVolume volume ID must be provided")
	}

	reqBytes, err := getShareRequestCapacity(req.GetCapacityRange())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	_, project, location, instanceName, shareName, err := parseMultishareVolId(req.VolumeId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	klog.Infof("ControllerExpandVolume called for multishare with request %+v", req)
	if acquired := m.volumeLocks.TryAcquire(volumeId); !acquired {
		return nil, status.Errorf(codes.Aborted, util.VolumeOperationAlreadyExistsFmt, volumeId)
	}
	defer m.volumeLocks.Release(volumeId)

	share, err := m.cloud.File.GetShare(ctx, &file.Share{
		Parent: &file.MultishareInstance{
			Project:  project,
			Location: location,
			Name:     instanceName,
		},
		Name: shareName,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}
	if share == nil || file.IsNotFoundErr(err) {
		return nil, status.Errorf(codes.Internal, "Couldn't find share with name %q", volumeId)
	}

	if share.CapacityBytes >= reqBytes {
		klog.Infof("Controller expand volume succeeded for volume %v, existing size(bytes): %v", volumeId, share.CapacityBytes)
		return &csi.ControllerExpandVolumeResponse{
			CapacityBytes:         share.CapacityBytes,
			NodeExpansionRequired: false,
		}, nil
	}

	workflow, err := m.opsManager.checkAndStartInstanceOrShareExpandWorkflow(ctx, share, reqBytes)
	if err != nil {
		return nil, err
	}

	err = m.waitOnWorkflow(ctx, workflow)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "wait on %s operation %q failed with error: %v", workflow.opType.String(), workflow.opName, err)
	}
	klog.Infof("Wait for operation %s (type %s) completed", workflow.opName, workflow.opType.String())

	switch workflow.opType {
	case util.InstanceUpdate:
		workflow, err = m.opsManager.startShareExpandWorkflowSafe(ctx, share, reqBytes)
		if err != nil {
			return nil, err
		}
	case util.ShareUpdate:
		return m.getShareAndGenerateCSIControllerExpandVolumeResponse(ctx, share)
	default:
		return nil, status.Errorf(codes.Internal, "Controller Expand Volume failed, unknown workflow %v detected", workflow.opType)
	}

	err = m.waitOnWorkflow(ctx, workflow)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "wait on share expansion op %q failed with error: %v", workflow.opName, err)
	}

	return m.getShareAndGenerateCSIControllerExpandVolumeResponse(ctx, share)
}

func (m *MultishareController) getShareAndGenerateCSIControllerExpandVolumeResponse(ctx context.Context, share *file.Share) (*csi.ControllerExpandVolumeResponse, error) {
	share, err := m.cloud.File.GetShare(ctx, share)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}
	return &csi.ControllerExpandVolumeResponse{
		CapacityBytes:         share.CapacityBytes,
		NodeExpansionRequired: false,
	}, nil
}

func (m *MultishareController) waitOnWorkflow(ctx context.Context, workflow *Workflow) (err error) {
	timeout, pollInterval, err := util.GetMultishareOpsTimeoutConfig(workflow.opType)
	if err != nil {
		return
	}
	err = m.cloud.File.WaitForOpWithOpts(ctx, workflow.opName, file.PollOpts{Timeout: timeout, Interval: pollInterval})
	return
}

func getInstanceSCPrefix(req *csi.CreateVolumeRequest) (string, error) {
	params := req.GetParameters()
	v, ok := params[paramMultishareInstanceScLabel]
	if !ok {
		return "", fmt.Errorf("failed to find instance prefix key")
	}

	if v == "" {
		return "", fmt.Errorf("instance prefix is empty")
	}

	return v, nil
}

func (m *MultishareController) generateNewMultishareInstance(instanceName string, req *csi.CreateVolumeRequest) (*file.MultishareInstance, error) {
	region, err := m.pickRegion(req.GetAccessibilityRequirements())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	tier := enterpriseTier
	network := defaultNetwork
	connectMode := directPeering
	kmsKeyName := ""
	for k, v := range req.GetParameters() {
		switch strings.ToLower(k) {
		case paramTier:
			tier = v
		case paramNetwork:
			network = v
		case paramConnectMode:
			connectMode = v
			if connectMode != directPeering && connectMode != privateServiceAccess {
				return nil, status.Errorf(codes.InvalidArgument, fmt.Sprintf("connect mode can only be one of %q or %q", directPeering, privateServiceAccess))
			}
		case paramInstanceEncryptionKmsKey:
			kmsKeyName = v
		// Ignore the cidr flag as it is not passed to the cloud provider
		// It will be used to get unreserved IP in the reserveIPV4Range function
		// ignore IPRange flag as it will be handled at the same place as cidr
		case paramReservedIPV4CIDR, paramReservedIPRange:
			continue
		case paramMultishareInstanceScLabel:
			continue
		case ParameterKeyLabels, ParameterKeyPVCName, ParameterKeyPVCNamespace, ParameterKeyPVName, paramMultishare:
		case "csiprovisionersecretname", "csiprovisionersecretnamespace":
		default:
			return nil, status.Errorf(codes.InvalidArgument, fmt.Sprintf("invalid parameter %q", k))
		}
	}

	if tier != enterpriseTier {
		return nil, status.Errorf(codes.InvalidArgument, fmt.Sprintf("tier %q not supported for multishare volumes", tier))
	}

	labels, err := extractInstanceLabels(req.GetParameters(), m.driver.config.Name)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	return &file.MultishareInstance{
		Project:       m.cloud.Project,
		Name:          instanceName,
		CapacityBytes: util.MinMultishareInstanceSizeBytes,
		Location:      region,
		Tier:          tier,
		Network: file.Network{
			Name:        network,
			ConnectMode: connectMode,
		},
		KmsKeyName:  kmsKeyName,
		Labels:      labels,
		Description: generateInstanceDescFromEcfsDesc(m.ecfsDescription),
	}, nil
}

func generateNewShare(name string, parent *file.MultishareInstance, req *csi.CreateVolumeRequest) (*file.Share, error) {
	if parent == nil {
		return nil, status.Error(codes.Internal, "parent mulishare instance is empty")
	}
	targetSizeBytes, err := getShareRequestCapacity(req.CapacityRange)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return &file.Share{
		Name:           name,
		Parent:         parent,
		CapacityBytes:  targetSizeBytes,
		Labels:         extractShareLabels(req.Parameters),
		MountPointName: name,
	}, nil
}

func (m *MultishareController) pickRegion(top *csi.TopologyRequirement) (string, error) {
	if top == nil {
		region, err := util.GetRegionFromZone(m.cloud.Zone)
		if err != nil {
			return "", err
		}

		return region, nil
	}

	zone, err := pickZoneFromTopology(top)
	if err != nil {
		return "", err
	}
	region, err := util.GetRegionFromZone(zone)
	if err != nil {
		return "", err
	}
	return region, nil
}

func extractInstanceLabels(parameters map[string]string, driverName string) (map[string]string, error) {
	instanceLabels := make(map[string]string)
	userProvidedLabels := make(map[string]string)
	for k, v := range parameters {
		switch strings.ToLower(k) {
		case ParameterKeyLabels:
			var err error
			userProvidedLabels, err = util.ConvertLabelsStringToMap(v)
			if err != nil {
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
		case paramMultishareInstanceScLabel:
			err := util.CheckLabelValueRegex(v)
			if err != nil {
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
			instanceLabels[util.ParamMultishareInstanceScLabelKey] = v
		}
	}

	instanceLabels[tagKeyCreatedBy] = strings.ReplaceAll(driverName, ".", "_")
	finalInstanceLabels, err := mergeLabels(userProvidedLabels, instanceLabels)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return finalInstanceLabels, nil
}

func extractShareLabels(parameters map[string]string) map[string]string {
	shareLabels := make(map[string]string)
	for k, v := range parameters {
		switch strings.ToLower(k) {
		case ParameterKeyPVCName:
			shareLabels[tagKeyCreatedForClaimName] = v
		case ParameterKeyPVCNamespace:
			shareLabels[tagKeyCreatedForClaimNamespace] = v
		case ParameterKeyPVName:
			shareLabels[tagKeyCreatedForVolumeName] = v
		}
	}
	return shareLabels
}

func getShareRequestCapacity(capRange *csi.CapacityRange) (int64, error) {
	if capRange == nil {
		return util.MinShareSizeBytes, nil
	}

	rCap := capRange.GetRequiredBytes()
	rSet := rCap > 0
	lCap := capRange.GetLimitBytes()
	lSet := lCap > 0

	if !lSet && !rSet {
		return 0, status.Errorf(codes.InvalidArgument, "Neither Limit bytes or Required bytes set")
	}

	if lSet && rSet && lCap < rCap {
		return 0, status.Errorf(codes.InvalidArgument, "Limit bytes %v is less than required bytes %v", lCap, rCap)
	}

	// Check bounds of limit and request.
	if lSet {
		if lCap < util.MinShareSizeBytes {
			return 0, status.Errorf(codes.InvalidArgument, "Limit bytes %v is less than minimum share size bytes %v", lCap, util.MinShareSizeBytes)
		}

		if lCap > util.MaxShareSizeBytes {
			return 0, status.Errorf(codes.InvalidArgument, "Limit bytes %v is greater than maximum share size bytes %v", lCap, util.MaxShareSizeBytes)
		}
	}

	if rSet {
		if rCap < util.MinShareSizeBytes {
			return 0, status.Errorf(codes.InvalidArgument, "Request bytes %v is less than minimum share size bytes %v", lCap, util.MinShareSizeBytes)
		}

		if rCap > util.MaxShareSizeBytes {
			return 0, status.Errorf(codes.InvalidArgument, "Request bytes %v is greater than maximum share size bytes %v", lCap, util.MaxShareSizeBytes)
		}
	}

	if lSet {
		return lCap, nil
	}

	return rCap, nil
}

func generateCSICreateVolumeResponse(instancePrefix string, s *file.Share) (*csi.CreateVolumeResponse, error) {
	volId, err := generateMultishareVolumeIdFromShare(instancePrefix, s)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	resp := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      volId,
			CapacityBytes: s.CapacityBytes,
			VolumeContext: map[string]string{
				attrIP: s.Parent.Network.Ip,
			},
		},
	}
	klog.Infof("CreateVolume resp: %+v", resp)
	return resp, nil
}

func containsInstancePrefix(shareHandle string, project, location, instanceName string) bool {
	targetInstance := fmt.Sprintf("%s/%s/%s", project, location, instanceName)
	return strings.Contains(shareHandle, targetInstance)
}

func generateInstanceDescFromEcfsDesc(desc string) string {
	if desc == "" {
		return desc
	}

	parts := strings.Split(desc, ",")
	descMap := make(map[string]string)
	for _, part := range parts {
		pair := strings.Split(part, "=")
		if len(pair) != 2 {
			continue
		}
		descMap[pair[0]] = pair[1]
	}

	const (
		ecfsVersionKey    = "ecfs-version"
		imageProjectIdKey = "image-project-id"
	)
	var (
		ecfsVersion    string
		imageProjectId string
	)
	for k, v := range descMap {
		switch k {
		case ecfsVersionKey:
			ecfsVersion = v
		case imageProjectIdKey:
			imageProjectId = v
		}
	}

	if ecfsVersion == "" || imageProjectId == "" {
		return ""
	}

	d := fmt.Sprintf(ecfsDataPlaneVersionFormat, imageProjectId, ecfsVersion)
	klog.V(4).Infof("generated description for multishare instance %s", d)
	return d
}
