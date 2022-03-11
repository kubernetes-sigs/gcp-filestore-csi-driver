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
	cloud "sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/file"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/util"
)

const (
	modeMultishare = "modeMultishare"
)

// MultishareController handles CSI calls for volumes which use Filestore multishare instances.
type MultishareController struct {
	driver      *GCFSDriver
	fileService file.Service
	cloud       *cloud.Cloud
	opsManager  *MultishareOpsManager
	volumeLocks *util.VolumeLocks
}

func NewMultishareController(driver *GCFSDriver, fileService file.Service, cloud *cloud.Cloud, volumeLock *util.VolumeLocks) *MultishareController {
	return &MultishareController{
		opsManager:  NewMultishareOpsManager(cloud),
		driver:      driver,
		fileService: fileService,
		cloud:       cloud,
		volumeLocks: volumeLock,
	}
}

func (m *MultishareController) Run() {
	m.opsManager.Run()
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
		return m.getShareAndGenerateCSIResponse(ctx, instanceScPrefix, share)
	}

	timeout, pollInterval, err := util.GetMultishareOpsTimeoutConfig(workflow.opType)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}
	// cache lock released. poll for op.
	err = m.cloud.File.WaitForOpWithOpts(ctx, workflow.opName, file.PollOpts{Timeout: timeout, Interval: pollInterval})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Create Volume failed, operation %q poll error: %v", workflow.opName, err)
	}

	if workflow.opType == util.ShareCreate {
		return m.getShareAndGenerateCSIResponse(ctx, instanceScPrefix, workflow.share)
	}

	var shareCreateWorkflow *Workflow
	var newShare *file.Share
	switch workflow.opType {
	case util.InstanceCreate, util.InstanceExpand:
		newShare, err = generateNewShare(util.ConvertVolToShareName(req.Name), workflow.instance, req)
		if err != nil {
			return nil, err
		}
		shareCreateWorkflow, err = m.opsManager.startShareCreateWorkflowSafe(ctx, instanceScPrefix, newShare)
		if err != nil {
			return nil, err
		}
	default:
		return nil, status.Errorf(codes.Internal, "Create Volume failed, unknown workflow %v detected", workflow.opType)
	}

	// cache lock released. poll for share create op.
	shareCreatetimeout, shareCreatePollInterval, err := util.GetMultishareOpsTimeoutConfig(shareCreateWorkflow.opType)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}
	err = m.cloud.File.WaitForOpWithOpts(ctx, shareCreateWorkflow.opName, file.PollOpts{Timeout: shareCreatetimeout, Interval: shareCreatePollInterval})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "operation %q poll error: %v", workflow.opName, err)
	}

	return m.getShareAndGenerateCSIResponse(ctx, instanceScPrefix, newShare)
}

func (m *MultishareController) getShareAndGenerateCSIResponse(ctx context.Context, instancePrefix string, s *file.Share) (*csi.CreateVolumeResponse, error) {
	share, err := m.fileService.GetShare(ctx, s)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	// TODO: do we need any further validation here?
	return generateCSICreateVolumeResponse(instancePrefix, share)
}

func (m *MultishareController) DeleteVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	// Handle higher level csi params validation, try locks
	// Initiate share workflow by calling Multishare OpsManager functions
	// Prepare and return csi response
	return nil, nil
}

func (m *MultishareController) ControllerExpandVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	// Handle higher level csi params validation, try locks
	// Initiate share workflow by calling Multishare OpsManager functions
	// Prepare and return csi response
	return nil, nil
}

func getInstanceSCPrefix(req *csi.CreateVolumeRequest) (string, error) {
	params := req.GetParameters()
	v, ok := params[paramMultishareInstanceScLabel]
	if ok {
		return "", fmt.Errorf("Failed to find instance prefix key")
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
		case strings.ToLower(util.ParamMultishareInstanceScLabel):
			continue
		case ParameterKeyLabels, ParameterKeyPVCName, ParameterKeyPVCNamespace, ParameterKeyPVName:
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
		KmsKeyName: kmsKeyName,
		Labels:     labels,
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
		Name:          name,
		Parent:        parent,
		CapacityBytes: targetSizeBytes,
		Labels:        extractShareLabels(req.Parameters),
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
		case strings.ToLower(util.ParamMultishareInstanceScLabel):
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
	return resp, nil
}

func containsInstancePrefix(shareHandle string, project, location, instanceName string) bool {
	targetInstance := fmt.Sprintf("%s/%s/%s", project, location, instanceName)
	return strings.Contains(shareHandle, targetInstance)
}
