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
	"strconv"
	"strings"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	cloud "sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/file"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/util"
)

const (
	modeMultishare = "modeMultishare"

	methodCreateVolume              = "CreateVolume"
	methodDeleteVolume              = "DeleteVolume"
	methodExpandVolume              = "ExpandVolume"
	methodCreateSnapshot            = "CreateSnapshot"
	methodDeleteSnapshot            = "DeleteSnapshot"
	ecfsDataPlaneVersionFormat      = "GoogleReserved-CustomVMImage=clh.image.ems.path:projects/%s/global/images/ems-filestore-scaleout-%s"
	ecfsCustom100sharesConfigFormat = "GoogleReservedOverrides={\"CustomMultiShareConfig\":{\"MaxShareCount\": %d, \"MinShareSizeGB\":%d}}"

	// volume context attributes
	attrMaxShareSize = "max-share-size"
)

// MultishareController handles CSI calls for volumes which use Filestore multishare instances.
type MultishareController struct {
	driver                     *GCFSDriver
	fileService                file.Service
	cloud                      *cloud.Cloud
	opsManager                 *MultishareOpsManager
	volumeLocks                *util.VolumeLocks
	ecfsDescription            string
	isRegional                 bool
	clustername                string
	featureMaxSharePerInstance bool

	// Filestore instance description overrides
	descOverrideMaxSharesPerInstance string
	descOverrideMinShareSizeBytes    string

	pvLister       corelisters.PersistentVolumeLister
	pvListerSynced cache.InformerSynced
	kubeClient     *kubernetes.Clientset
	factory        informers.SharedInformerFactory
}

func NewMultishareController(config *controllerServerConfig) *MultishareController {
	c := &MultishareController{
		driver:          config.driver,
		fileService:     config.fileService,
		cloud:           config.cloud,
		volumeLocks:     config.volumeLocks,
		ecfsDescription: config.ecfsDescription,
		isRegional:      config.isRegional,
		clustername:     config.clusterName,
	}
	c.opsManager = NewMultishareOpsManager(config.cloud, c)
	if config.features != nil && config.features.FeatureMaxSharesPerInstance != nil {
		c.featureMaxSharePerInstance = config.features.FeatureMaxSharesPerInstance.Enabled
		c.descOverrideMaxSharesPerInstance = config.features.FeatureMaxSharesPerInstance.DescOverrideMaxSharesPerInstance
		c.descOverrideMinShareSizeBytes = config.features.FeatureMaxSharesPerInstance.DescOverrideMinShareSizeGB
		c.kubeClient = config.features.FeatureMaxSharesPerInstance.KubeClient
		c.factory = informers.NewSharedInformerFactory(c.kubeClient, config.features.FeatureMaxSharesPerInstance.CoreInformerResync)
		pvInformer := c.factory.Core().V1().PersistentVolumes()
		c.pvLister = pvInformer.Lister()
		c.pvListerSynced = pvInformer.Informer().HasSynced
	}

	return c
}

func (m *MultishareController) Run(stopCh <-chan struct{}) {
	if !m.featureMaxSharePerInstance {
		return
	}

	m.factory.Start(stopCh)
	klog.Info("core Informer factory started")
	if !cache.WaitForCacheSync(stopCh, m.pvListerSynced) {
		klog.Errorf("Cannot sync caches")
	}
	klog.Infof("Informer cache sycned successfully %v", m.pvListerSynced())
}

func (m *MultishareController) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	klog.Infof("CreateVolume called for multishare with request %+v", req)
	name := req.GetName()
	if len(name) == 0 {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume name must be provided")
	}
	if err := m.driver.validateVolumeCapabilities(req.GetVolumeCapabilities()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if req.GetVolumeContentSource() != nil {
		return nil, status.Error(codes.InvalidArgument, "Multishare backed volumes do not support volume content source")
	}

	instanceScPrefix, err := getInstanceSCLabel(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	maxSharesPerInstance, maxShareSizeSizeBytes, err := m.parseMaxVolumeSizeParam(req.GetParameters())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	var reqBytes int64
	if m.featureMaxSharePerInstance {
		reqBytes, err = getShareRequestCapacity(req.GetCapacityRange(), util.ConfigurablePackMinShareSizeBytes, maxShareSizeSizeBytes)
	} else {
		reqBytes, err = getShareRequestCapacity(req.GetCapacityRange(), util.MinShareSizeBytes, util.MaxShareSizeBytes)
	}
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if !util.IsAligned(reqBytes, util.Gb) {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("requested size(bytes) %d is not a multiple of 1GiB", reqBytes))
	}
	if acquired := m.volumeLocks.TryAcquire(name); !acquired {
		return nil, status.Errorf(codes.Aborted, util.VolumeOperationAlreadyExistsFmt, name)
	}
	defer m.volumeLocks.Release(name)

	// If no eligible instance found, the ops manager may decide to create a new instance. Prepare a multishare instacne object for such a scenario.
	instance, err := m.generateNewMultishareInstance(util.NewMultishareInstancePrefix+string(uuid.NewUUID()), req, maxSharesPerInstance)
	if err != nil {
		return nil, file.StatusError(err)
	}

	if m.featureMaxSharePerInstance && m.descOverrideMaxSharesPerInstance != "" && m.descOverrideMinShareSizeBytes != "" {
		sharesPerInstance, err := strconv.Atoi(m.descOverrideMaxSharesPerInstance)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("invalid description override value %s", m.descOverrideMaxSharesPerInstance))
		}
		minShareSizeGB, err := strconv.Atoi(m.descOverrideMinShareSizeBytes)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("invalid description override value %s", m.descOverrideMinShareSizeBytes))
		}
		instance.Description = fmt.Sprintf(ecfsCustom100sharesConfigFormat, sharesPerInstance, minShareSizeGB)
	}

	workflow, share, err := m.opsManager.setupEligibleInstanceAndStartWorkflow(ctx, req, instance)
	if err != nil {
		return nil, file.StatusError(err)
	}

	if share != nil {
		resp, err := m.getShareAndGenerateCSICreateVolumeResponse(ctx, instanceScPrefix, share, maxShareSizeSizeBytes)
		return resp, file.StatusError(err)
	}

	// lock released. poll for op.
	err = m.waitOnWorkflow(ctx, workflow)
	if err != nil {
		return nil, file.StatusError(fmt.Errorf("Create Volume failed, operation %q poll error: %w", workflow.opName, err))
	}

	klog.Infof("Poll for operation %s (type %s) completed", workflow.opName, workflow.opType.String())
	if workflow.opType == util.ShareCreate {
		resp, err := m.getShareAndGenerateCSICreateVolumeResponse(ctx, instanceScPrefix, workflow.share, maxShareSizeSizeBytes)
		return resp, file.StatusError(err)
	}

	var shareCreateWorkflow *Workflow
	var newShare *file.Share
	switch workflow.opType {
	case util.InstanceCreate, util.InstanceUpdate:
		newShare, err = generateNewShare(util.ConvertVolToShareName(req.Name), workflow.instance, req)
		if err != nil {
			return nil, file.StatusError(err)
		}
		shareCreateWorkflow, err = m.opsManager.startShareCreateWorkflowSafe(ctx, newShare)
		if err != nil {
			return nil, file.StatusError(err)
		}
	default:
		return nil, status.Errorf(codes.Internal, "Create Volume failed, unknown workflow %v detected", workflow.opType)
	}

	// lock released. poll for share create op.
	err = m.waitOnWorkflow(ctx, shareCreateWorkflow)
	if err != nil {
		return nil, file.StatusError(fmt.Errorf("%v operation %q poll error: %w", shareCreateWorkflow.opType, shareCreateWorkflow.opName, err))
	}
	resp, err := m.getShareAndGenerateCSICreateVolumeResponse(ctx, instanceScPrefix, newShare, maxShareSizeSizeBytes)
	return resp, file.StatusError(err)
}

func (m *MultishareController) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	klog.Infof("CreateSnapshot called for multishare with request %+v", req)
	name := req.GetName()
	volumeID := req.GetSourceVolumeId()

	if acquired := m.volumeLocks.TryAcquire(volumeID); !acquired {
		return nil, status.Errorf(codes.Aborted, util.VolumeOperationAlreadyExistsFmt, volumeID)
	}
	defer m.volumeLocks.Release(volumeID)

	if req.GetParameters() != nil {
		if _, err := util.IsSnapshotTypeSupported(req.GetParameters()); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	}
	_, location, instanceName, shareName, err := parseSourceVolId(volumeID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	project := m.cloud.Project

	backupLocation := util.GetBackupLocation(req.GetParameters()) //Optional provided locaiton for cross-region backups
	backupURI, backupRegion, err := file.CreateBackupURI(location, project, name, backupLocation)
	if err != nil {
		klog.Errorf("Failed to create backup URI from given name %s and location %s, error: %v", req.Name, backupLocation, err.Error())
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	existingBackup, err := m.cloud.File.GetBackup(ctx, backupURI)
	backupExists, err := file.CheckBackupExists(existingBackup, err)
	if err != nil {
		return nil, file.StatusError(err)
	}

	if backupExists {
		// process existing backup

		snapshot, err := file.ProcessExistingBackup(ctx, existingBackup, volumeID, modeMultishare)
		if err != nil {
			return nil, err
		}
		return &csi.CreateSnapshotResponse{
			Snapshot: snapshot,
		}, nil
	} else {
		//no existing backup
		backupInfo := &file.BackupInfo{
			Name:               name,
			SourceVolumeId:     volumeID,
			Project:            project,
			Location:           backupRegion,
			SourceShare:        shareName,
			SourceInstanceName: instanceName,
			BackupURI:          backupURI,
		}

		labels, err := extractBackupLabels(req.GetParameters(), m.driver.config.Name, req.Name)
		if err != nil {
			return nil, err
		}
		backupInfo.Labels = labels
		snapshot, err := m.createNewBackup(ctx, backupInfo)
		if err != nil {
			return nil, err
		}

		resp := &csi.CreateSnapshotResponse{
			Snapshot: snapshot,
		}
		return resp, nil
	}
}

func (m *MultishareController) createNewBackup(ctx context.Context, backupInfo *file.BackupInfo) (*csi.Snapshot, error) {

	backupObj, err := m.cloud.File.CreateBackup(ctx, backupInfo)
	if err != nil {
		klog.Errorf("Create snapshot for volume Id %s failed: %v", backupInfo.SourceVolumeId, err.Error())
		return nil, file.StatusError(err)
	}
	tp, err := util.ParseTimestamp(backupObj.CreateTime)
	if err != nil {
		return nil, file.StatusError(err)
	}
	snapshot := &csi.Snapshot{
		SizeBytes:      util.GbToBytes(backupObj.CapacityGb),
		SnapshotId:     backupObj.Name,
		SourceVolumeId: backupInfo.SourceVolumeId,
		CreationTime:   tp,
		ReadyToUse:     true,
	}

	return snapshot, nil
}

func (m *MultishareController) getShareAndGenerateCSICreateVolumeResponse(ctx context.Context, instancePrefix string, s *file.Share, maxShareSizeSizeBytes int64) (*csi.CreateVolumeResponse, error) {
	share, err := m.cloud.File.GetShare(ctx, s)
	if err != nil {
		return nil, err
	}

	if share.State != "READY" {
		return nil, status.Errorf(codes.Aborted, "share %s not ready, state %s", share.Name, share.State)
	}
	return m.generateCSICreateVolumeResponse(instancePrefix, share, maxShareSizeSizeBytes)
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
			if err == nil { // If NO error
				return &csi.DeleteVolumeResponse{}, nil
			}
		}

		return nil, file.StatusError(err)
	}

	workflow, err := m.opsManager.checkAndStartShareDeleteWorkflow(ctx, share)
	if err != nil {
		return nil, file.StatusError(err)
	}

	// Poll for share delete to complete
	if workflow != nil {
		err = m.waitOnWorkflow(ctx, workflow)
		if err != nil {
			return nil, file.StatusError(fmt.Errorf("%v operation %q poll error: %w", workflow.opType, workflow.opName, err))
		}
	}

	// Check whether instance can be shrinked or deleted.
	err = m.startAndWaitForInstanceDeleteOrShrink(ctx, req.VolumeId)
	if err != nil {
		return nil, file.StatusError(err)
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
		return fmt.Errorf("%v operation %q poll error: %w", workflow.opType, workflow.opName, err)
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

	maxShareSizeBytes := util.MaxShareSizeBytes
	if m.featureMaxSharePerInstance {
		var err error
		maxShareSizeBytes, err = m.GetShareMaxSizeFromPV(ctx, volumeId)
		if err != nil {
			return nil, file.StatusError(err)
		}
		klog.Infof("maxShareSizeBytes %d", maxShareSizeBytes)
	}
	reqBytes, err := getShareRequestCapacity(req.GetCapacityRange(), util.ConfigurablePackMinShareSizeBytes, maxShareSizeBytes)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if !util.IsAligned(reqBytes, util.Gb) {
		return nil, status.Errorf(codes.InvalidArgument, "requested size(bytes) %d is not a multiple of 1GiB", reqBytes)
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
	if share == nil || file.IsNotFoundErr(err) {
		return nil, status.Errorf(codes.NotFound, "Couldn't find share with name %q", volumeId)
	}
	if err != nil {
		return nil, file.StatusError(err)
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
		return nil, file.StatusError(err)
	}

	err = m.waitOnWorkflow(ctx, workflow)
	if err != nil {
		return nil, file.StatusError(fmt.Errorf("wait on %v operation %q failed with error: %w", workflow.opType, workflow.opName, err))
	}
	klog.Infof("Wait for operation %s (type %s) completed", workflow.opName, workflow.opType.String())

	switch workflow.opType {
	case util.InstanceUpdate:
		workflow, err = m.opsManager.startShareExpandWorkflowSafe(ctx, share, reqBytes)
		if err != nil {
			return nil, file.StatusError(err)
		}
	case util.ShareUpdate:
		resp, err := m.getShareAndGenerateCSIControllerExpandVolumeResponse(ctx, share, reqBytes)
		return resp, file.StatusError(err)
	default:
		return nil, status.Errorf(codes.Internal, "Controller Expand Volume failed, unknown workflow %v detected", workflow.opType)
	}

	err = m.waitOnWorkflow(ctx, workflow)
	if err != nil {
		return nil, file.StatusError(fmt.Errorf("wait on share expansion op %q failed with error: %w", workflow.opName, err))
	}

	resp, err := m.getShareAndGenerateCSIControllerExpandVolumeResponse(ctx, share, reqBytes)
	return resp, file.StatusError(err)
}

func (m *MultishareController) getShareAndGenerateCSIControllerExpandVolumeResponse(ctx context.Context, share *file.Share, reqBytes int64) (*csi.ControllerExpandVolumeResponse, error) {
	share, err := m.cloud.File.GetShare(ctx, share)
	if err != nil {
		return nil, err
	}
	if share.CapacityBytes < reqBytes {
		return nil, status.Errorf(codes.Aborted, "expand volume operation succeeded but share capacity [%d]bytes smaller than requested [%d]bytes", share.CapacityBytes, reqBytes)
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

func getInstanceSCLabel(req *csi.CreateVolumeRequest) (string, error) {
	params := req.GetParameters()
	v, ok := params[ParamMultishareInstanceScLabel]
	if !ok {
		return "", fmt.Errorf("failed to find instance prefix key")
	}

	if v == "" {
		return "", fmt.Errorf("instance prefix is empty")
	}

	return v, nil
}

func (m *MultishareController) generateNewMultishareInstance(instanceName string, req *csi.CreateVolumeRequest, maxShareCount int) (*file.MultishareInstance, error) {
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
		case ParamConnectMode:
			connectMode = v
			if connectMode != directPeering && connectMode != privateServiceAccess {
				return nil, status.Errorf(codes.InvalidArgument, "connect mode can only be one of %q or %q", directPeering, privateServiceAccess)
			}
		case ParamInstanceEncryptionKmsKey:
			kmsKeyName = v
		// Ignore the cidr flag as it is not passed to the cloud provider
		// It will be used to get unreserved IP in the reserveIPV4Range function
		// ignore IPRange flag as it will be handled at the same place as cidr
		case ParamReservedIPV4CIDR, ParamReservedIPRange:
			continue
		case ParamMultishareInstanceScLabel:
			continue
		case paramMaxVolumeSize:
			continue
		case ParameterKeyLabels, ParameterKeyPVCName, ParameterKeyPVCNamespace, ParameterKeyPVName, paramMultishare:
		case "csiprovisionersecretname", "csiprovisionersecretnamespace":
		default:
			return nil, status.Errorf(codes.InvalidArgument, "invalid parameter %q", k)
		}
	}

	if tier != enterpriseTier {
		return nil, status.Errorf(codes.InvalidArgument, "tier %q not supported for multishare volumes", tier)
	}

	location := m.cloud.Zone
	if m.isRegional {
		location, err = util.GetRegionFromZone(location)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "failed to get region for regional cluster: %v", err.Error())
		}
	}
	labels, err := extractInstanceLabels(req.GetParameters(), m.driver.config.Name, m.clustername, location)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	f := &file.MultishareInstance{
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
	}
	if m.featureMaxSharePerInstance {
		f.MaxShareCount = maxShareCount
	}
	return f, nil
}

func generateNewShare(name string, parent *file.MultishareInstance, req *csi.CreateVolumeRequest) (*file.Share, error) {
	if parent == nil {
		return nil, status.Error(codes.Internal, "parent multishare instance is empty")
	}
	// The share size request is already validated in CreateVolume call
	targetSizeBytes, err := getShareRequestCapacity(req.CapacityRange, util.ConfigurablePackMinShareSizeBytes, util.MaxShareSizeBytes)
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

func extractInstanceLabels(parameters map[string]string, driverName, clusterName, location string) (map[string]string, error) {
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
		case ParamMultishareInstanceScLabel:
			err := util.CheckLabelValueRegex(v)
			if err != nil {
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
			instanceLabels[util.ParamMultishareInstanceScLabelKey] = v
		}
	}

	instanceLabels[tagKeyCreatedBy] = strings.ReplaceAll(driverName, ".", "_")
	instanceLabels[TagKeyClusterName] = clusterName
	instanceLabels[TagKeyClusterLocation] = location
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

func getShareRequestCapacity(capRange *csi.CapacityRange, minShareSizeBytes, maxShareSizeBytes int64) (int64, error) {
	if capRange == nil {
		return minShareSizeBytes, nil
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
		if lCap < minShareSizeBytes {
			return 0, status.Errorf(codes.InvalidArgument, "Limit bytes %v is less than minimum share size bytes %v", lCap, minShareSizeBytes)
		}

		if lCap > maxShareSizeBytes {
			return 0, status.Errorf(codes.InvalidArgument, "Limit bytes %v is greater than maximum share size bytes %v", lCap, maxShareSizeBytes)
		}
	}

	if rSet {
		if rCap < minShareSizeBytes {
			return 0, status.Errorf(codes.InvalidArgument, "Request bytes %v is less than minimum share size bytes %v", rCap, minShareSizeBytes)
		}

		if rCap > maxShareSizeBytes {
			return 0, status.Errorf(codes.InvalidArgument, "Request bytes %v is greater than maximum share size bytes %v", rCap, maxShareSizeBytes)
		}
	}

	if lSet {
		return lCap, nil
	}

	return rCap, nil
}

func (m *MultishareController) generateCSICreateVolumeResponse(instancePrefix string, s *file.Share, maxShareSizeBytes int64) (*csi.CreateVolumeResponse, error) {
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
	if m.driver.config.FeatureOptions.FeatureLockRelease.Enabled {
		resp.Volume.VolumeContext[attrSupportLockRelease] = "true"
	}
	if m.featureMaxSharePerInstance {
		resp.Volume.VolumeContext[attrMaxShareSize] = strconv.Itoa(int(maxShareSizeBytes))
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

func (m *MultishareController) parseMaxVolumeSizeParam(params map[string]string) (int, int64, error) {
	v, ok := params[paramMaxVolumeSize]
	if !m.featureMaxSharePerInstance && ok {
		return 0, 0, fmt.Errorf("configurable max shares per instance feature not enabled")
	}
	if !ok {
		return util.MaxSharesPerInstance, util.MaxShareSizeBytes, nil
	}
	if v == "" {
		return 0, 0, fmt.Errorf("value is empty for %q key", paramMaxVolumeSize)
	}
	val, err := resource.ParseQuantity(v)
	if err != nil {
		return 0, 0, err
	}

	valBytes := val.Value()
	sharesPerInstance, err := getSharesPerInstance(valBytes)
	if err != nil {
		return 0, 0, err
	}
	return sharesPerInstance, valBytes, nil
}

func getSharesPerInstance(volSizeBytes int64) (int, error) {
	if !isValidMaxVolSize(volSizeBytes) {
		return 0, fmt.Errorf("unsupported max volume size %d, supported sizes: '128Gi', '256Gi', '512Gi', '1024Gi'", volSizeBytes)
	}
	return int(util.MaxMultishareInstanceSizeBytes / volSizeBytes), nil
}

func isValidMaxVolSize(val int64) bool {
	switch val {
	case 128 * util.Gb:
		return true
	case 256 * util.Gb:
		return true
	case 512 * util.Gb:
		return true
	case util.Tb:
		return true
	}
	return false
}

func (m *MultishareController) GetShareMaxSizeFromPV(ctx context.Context, volHandle string) (int64, error) {
	// Even if the feature `featureMaxSharePerInstance` is disabled, we still need to handle the case of their being existing PVs which have share capacity range context saved in volumeAttributes
	var targetPV *v1.PersistentVolume
	var err error
	if m.pvListerSynced() {
		targetPV, err = m.findTargetPVFromInformer(volHandle)
		if err != nil {
			klog.Warningf("failed to list PV from informer cache, err %v", err)
		}
	} else {
		klog.Warningf("PV informer cache not intialized, lookup PV list from kube-api server")
	}

	if targetPV == nil {
		targetPV, err = m.findTargetPVFromKubeApiServer(ctx, volHandle)
		if err != nil {
			return 0, file.StatusError(err)
		}
	}

	if targetPV == nil {
		return 0, status.Errorf(codes.InvalidArgument, "target PV with volume handle %v not found, cannot determine the capacity range for the volume", volHandle)
	}

	// If volume atttributes does not capture the max share size, it must be created with the feature disabled.
	v, ok := targetPV.Spec.CSI.VolumeAttributes[attrMaxShareSize]
	if !ok {
		return util.MaxShareSizeBytes, nil
	}
	val, err := resource.ParseQuantity(v)
	if err != nil {
		return 0, file.StatusError(err)
	}
	return val.Value(), nil
}

func isTargetPV(pv *v1.PersistentVolume, volHandle string) bool {
	return pv.Spec.CSI != nil && pv.Spec.CSI.VolumeHandle == volHandle
}

func (m *MultishareController) findTargetPVFromInformer(volHandle string) (*v1.PersistentVolume, error) {
	pvList, err := m.pvLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	for _, pv := range pvList {
		if isTargetPV(pv, volHandle) {
			return pv, nil
		}
	}
	return nil, nil
}

func (m *MultishareController) findTargetPVFromKubeApiServer(ctx context.Context, volHandle string) (*v1.PersistentVolume, error) {
	pvList, err := m.kubeClient.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, pv := range pvList.Items {
		if isTargetPV(&pv, volHandle) {
			return &pv, nil
		}
	}
	return nil, nil
}
