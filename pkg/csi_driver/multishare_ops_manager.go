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
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"sync"

	"github.com/container-storage-interface/spec/lib/go/csi"
	filev1beta1multishare "google.golang.org/api/file/v1beta1multishare"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog"
	cloud "sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/file"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/util"
)

type OpInfo struct {
	Id     string
	Type   util.OperationType
	Target string
}

// A workflow is defined as a sequence of steps to safely initiate instance or share operations.
type Workflow struct {
	instance *file.MultishareInstance
	share    *file.Share
	opType   util.OperationType
	opName   string
}

// MultishareOpsManager manages the lifecycle of all instance and share operations.
type MultishareOpsManager struct {
	sync.Mutex // Lock to perform thread safe multishare operations.
	cloud      *cloud.Cloud
}

func NewMultishareOpsManager(cloud *cloud.Cloud) *MultishareOpsManager {
	return &MultishareOpsManager{
		cloud: cloud,
	}
}

// setupEligibleInstanceAndStartWorkflow returns a workflow object (to indicate an instance or share level workflow is started), or a share object (if existing share already found), or error.
func (m *MultishareOpsManager) setupEligibleInstanceAndStartWorkflow(ctx context.Context, req *csi.CreateVolumeRequest, instance *file.MultishareInstance) (*Workflow, *file.Share, error) {
	m.Lock()
	defer m.Unlock()

	// Check ShareCreateMap if a share create is already in progress.
	shareName := util.ConvertVolToShareName(req.Name)
	instanceScPrefix, err := getInstanceSCPrefix(req)
	if err != nil {
		return nil, nil, status.Error(codes.InvalidArgument, err.Error())
	}

	ops, err := m.listMultishareResourceRunningOps(ctx)
	if err != nil {
		return nil, nil, status.Error(codes.Internal, err.Error())
	}
	createShareOp := containsOpWithShareName(shareName, util.ShareCreate, ops)
	if createShareOp != nil {
		msg := fmt.Sprintf("Share create op %s in progress", createShareOp.Id)
		klog.Infof(msg)
		return nil, nil, status.Error(codes.Aborted, msg)
	}

	// Check if share already part of an existing instance.
	regions, err := m.listRegions(req.GetAccessibilityRequirements())
	if err != nil {
		return nil, nil, status.Error(codes.InvalidArgument, err.Error())
	}
	for _, region := range regions {
		shares, err := m.cloud.File.ListShares(ctx, &file.ListFilter{Project: m.cloud.Project, Location: region, InstanceName: "-"})

		if err != nil {
			return nil, nil, err
		}
		for _, s := range shares {
			if s.Name == shareName {
				return nil, s, nil
			}
		}
	}

	// No share or running share create op found. Proceed to eligible instance check.
	eligible, numIneligible, err := m.runEligibleInstanceCheck(ctx, instanceScPrefix, ops)
	if err != nil {
		return nil, nil, status.Error(codes.Aborted, err.Error())
	}

	if len(eligible) > 0 {
		// pick a random eligible instance
		index := rand.Intn(len(eligible))
		klog.V(5).Infof("For share %s, using instance %s as placeholder", shareName, eligible[index].String())
		share, err := generateNewShare(shareName, eligible[index], req)
		if err != nil {
			return nil, nil, status.Error(codes.Internal, err.Error())
		}

		needExpand, targetBytes, err := m.instanceNeedsExpand(ctx, share, share.CapacityBytes)
		if err != nil {
			return nil, nil, status.Error(codes.Internal, err.Error())
		}

		if needExpand {
			eligible[index].CapacityBytes = targetBytes
			w, err := m.startInstanceWorkflow(ctx, &Workflow{instance: eligible[index], opType: util.InstanceUpdate}, ops)
			return w, nil, err
		}

		w, err := m.startShareWorkflow(ctx, &Workflow{share: share, opType: util.ShareCreate}, ops)
		return w, nil, err
	}

	if numIneligible > 0 {
		// some instances not ready yet. wait for more instances to be ready.
		return nil, nil, status.Errorf(codes.Aborted, " %d non-ready instances detected. No ready instance found", numIneligible)
	}

	w, err := m.startInstanceWorkflow(ctx, &Workflow{instance: instance, opType: util.InstanceCreate}, ops)
	return w, nil, err
}

func (m *MultishareOpsManager) listRegions(top *csi.TopologyRequirement) ([]string, error) {
	var allowedRegions []string
	clusterRegion, err := util.GetRegionFromZone(m.cloud.Zone)
	if err != nil {
		return allowedRegions, err
	}
	if top == nil {
		return append(allowedRegions, clusterRegion), nil
	}

	zones, err := listZonesFromTopology(top)
	if err != nil {
		return allowedRegions, err
	}

	seen := make(map[string]bool)
	for _, zone := range zones {
		region, err := util.GetRegionFromZone(zone)
		if err != nil {
			return allowedRegions, err
		}
		if !seen[region] {
			seen[region] = true
			allowedRegions = append(allowedRegions, region)
		}
	}

	if len(allowedRegions) == 0 {
		return append(allowedRegions, clusterRegion), nil
	}

	return allowedRegions, nil
}

func (m *MultishareOpsManager) startShareCreateWorkflowSafe(ctx context.Context, share *file.Share) (*Workflow, error) {
	m.Lock()
	defer m.Unlock()
	ops, err := m.listMultishareResourceRunningOps(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return m.startShareWorkflow(ctx, &Workflow{share: share, opType: util.ShareCreate}, ops)
}

func (m *MultishareOpsManager) startInstanceWorkflow(ctx context.Context, w *Workflow, ops []*OpInfo) (*Workflow, error) {
	// This function has 2 steps:
	// 1. verify no instance ops or share (belonging to the instance) ops running for the given instance.
	// 2. Start the instance op.
	if w.instance == nil {
		return nil, status.Errorf(codes.Internal, "instance not found in workflow object")
	}

	err := m.verifyNoRunningInstanceOrShareOpsForInstance(w.instance, ops)
	if err != nil {
		return nil, err
	}
	switch w.opType {
	case util.InstanceCreate:
		op, err := m.cloud.File.StartCreateMultishareInstanceOp(ctx, w.instance)
		if err != nil {
			return nil, err
		}
		w.opName = op.Name
	case util.InstanceUpdate:
		op, err := m.cloud.File.StartResizeMultishareInstanceOp(ctx, w.instance)
		if err != nil {
			return nil, err
		}
		w.opName = op.Name
	case util.InstanceDelete:
		op, err := m.cloud.File.StartDeleteMultishareInstanceOp(ctx, w.instance)
		if err != nil {
			return nil, err
		}
		w.opName = op.Name
	default:
		return nil, status.Errorf(codes.Internal, "for instance workflow, unknown op type %s", w.opType.String())
	}

	return w, nil
}

func (m *MultishareOpsManager) verifyNoRunningInstanceOps(instance *file.MultishareInstance, ops []*OpInfo) error {
	instanceUri, err := file.GenerateMultishareInstanceURI(instance)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to parse instance handle, err: %v", err)
	}

	for _, op := range ops {
		if op.Target == instanceUri {
			return status.Errorf(codes.Aborted, "Found running op %s type %s for target resource %s", op.Id, op.Type.String(), op.Target)
		}
	}

	return nil
}

func (m *MultishareOpsManager) verifyNoRunningShareOps(share *file.Share, ops []*OpInfo) error {
	shareUri, err := file.GenerateShareURI(share)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to parse share handle, err: %v", err)
	}
	for _, op := range ops {
		if op.Target == shareUri {
			return status.Errorf(codes.Aborted, "Found running op %s type %s for target resource %s", op.Id, op.Type.String(), op.Target)
		}
	}

	return nil
}

func (m *MultishareOpsManager) startShareWorkflow(ctx context.Context, w *Workflow, ops []*OpInfo) (*Workflow, error) {
	// This function has 3 distinct steps:
	// 1. verify no instance ops running for the instance hosting the given share.
	// 2. verify no running ops for the given share.
	// 3. Start the share op.
	if w.share == nil {
		return nil, status.Errorf(codes.Internal, "share not found in workflow object")
	}

	if w.share.Parent == nil {
		return nil, status.Errorf(codes.Internal, "share parent not found in workflow object")
	}

	// verify instance is ready.
	err := m.verifyNoRunningInstanceOps(w.share.Parent, ops)
	if err != nil {
		return nil, err
	}
	// Verify share is ready.
	err = m.verifyNoRunningShareOps(w.share, ops)
	if err != nil {
		return nil, err
	}
	switch w.opType {
	case util.ShareCreate:
		op, err := m.cloud.File.StartCreateShareOp(ctx, w.share)
		if err != nil {
			return nil, err
		}
		w.opName = op.Name
	case util.ShareUpdate:
		op, err := m.cloud.File.StartResizeShareOp(ctx, w.share)
		if err != nil {
			return nil, err
		}
		w.opName = op.Name
	case util.ShareDelete:
		op, err := m.cloud.File.StartDeleteShareOp(ctx, w.share)
		if err != nil {
			return nil, err
		}
		w.opName = op.Name
	default:
		return nil, status.Errorf(codes.Internal, "for share workflow, unknown op type %v", w.opType)
	}
	return w, nil
}

func (m *MultishareOpsManager) verifyNoRunningInstanceOrShareOpsForInstance(instance *file.MultishareInstance, ops []*OpInfo) error {
	instanceUri, err := file.GenerateMultishareInstanceURI(instance)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to parse instance handle, err: %v", err)
	}

	// Check for instance prefix in op target.
	for _, op := range ops {
		if op.Target == instanceUri || strings.Contains(op.Target, instanceUri+"/") {
			return status.Errorf(codes.Aborted, "Found running op %s, type %s, for target resource %s", op.Id, op.Type.String(), op.Target)
		}
	}
	return nil
}

// runEligibleInstanceCheck returns a list of ready and non-ready instances.
func (m *MultishareOpsManager) runEligibleInstanceCheck(ctx context.Context, instanceScPrefix string, ops []*OpInfo) ([]*file.MultishareInstance, int, error) {
	instances, err := m.listInstanceForStorageClassPrefix(ctx, instanceScPrefix)
	if err != nil {
		return nil, 0, err
	}
	// An instance is considered as eligible if and only if its state is 'READY', and there's no ops running against it.
	var readyEligibleInstances []*file.MultishareInstance
	// An instance is considered as non-ready if it's being created, or its state is 'READY' but running ops are found on it.
	nonReadyInstanceCount := 0

	for _, instance := range instances {
		if instance.State == "CREATING" {
			klog.Infof("Instance %s/%s/%s with state %s is not ready", instance.Project, instance.Location, instance.Name, instance.State)
			nonReadyInstanceCount += 1
			continue
		}
		if instance.State != "READY" {
			klog.Infof("Instance %s/%s/%s with state %s is not eligible", instance.Project, instance.Location, instance.Name, instance.State)
			continue
			// TODO: If we saw instance states other than "CREATING" and "READY", we may need to do some special handldiing in the future.
		}

		op, err := containsOpWithInstanceTargetPrefix(instance, ops)
		if err != nil {
			klog.Errorf("failed to check eligibility of instance %s", instance.Name)
			return nil, 0, err
		}

		if op == nil {
			shares, err := m.cloud.File.ListShares(ctx, &file.ListFilter{Project: instance.Project, Location: instance.Location, InstanceName: instance.Name})
			if err != nil {
				klog.Errorf("Failed to list shares of instance %s/%s/%s, err:%v", instance.Project, instance.Location, instance.Name, err)
				return nil, 0, err
			}
			if len(shares) >= util.MaxSharesPerInstance {
				continue
			}

			readyEligibleInstances = append(readyEligibleInstances, instance)
			klog.Infof("Adding instance %s to eligible list", instance.String())
			continue
		}

		klog.Infof("Instance %s/%s/%s is not ready with ongoing operation %s type %s", instance.Project, instance.Location, instance.Name, op.Id, op.Type.String())
		nonReadyInstanceCount += 1
		// TODO: If we see > 1 instances with 0 shares (these could be possibly leaked instances where the driver hit timeout during creation op was in progress), should we trigger delete op for such instances? Possibly yes. Given that instance create/delete and share create/delete is serialized, maybe yes.
	}

	return readyEligibleInstances, nonReadyInstanceCount, nil
}

func (m *MultishareOpsManager) instanceNeedsExpand(ctx context.Context, share *file.Share, capacityNeeded int64) (bool, int64, error) {
	if share == nil {
		return false, 0, fmt.Errorf("empty share")
	}
	if share.Parent == nil {
		return false, 0, fmt.Errorf("parent missing from share %q", share.Name)
	}

	shares, err := m.cloud.File.ListShares(ctx, &file.ListFilter{Project: share.Parent.Project, Location: share.Parent.Location, InstanceName: share.Parent.Name})
	if err != nil {
		return false, 0, err
	}

	var sumShareBytes int64
	for _, s := range shares {
		sumShareBytes = sumShareBytes + s.CapacityBytes
	}

	remainingBytes := share.Parent.CapacityBytes - sumShareBytes
	if remainingBytes < capacityNeeded {
		alignBytes := util.AlignBytes(capacityNeeded+sumShareBytes, util.GbToBytes(share.Parent.CapacityStepSizeGb))
		targetBytes := util.Min(alignBytes, util.MaxMultishareInstanceSizeBytes)
		return true, targetBytes, nil
	}
	return false, 0, nil
}

func (m *MultishareOpsManager) checkAndStartInstanceOrShareExpandWorkflow(ctx context.Context, share *file.Share, reqBytes int64) (*Workflow, error) {
	m.Lock()
	defer m.Unlock()

	ops, err := m.listMultishareResourceRunningOps(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	expandShareOp, err := containsOpWithShareTarget(share, util.ShareUpdate, ops)
	if err != nil {
		return nil, err
	}
	if expandShareOp != nil {
		return &Workflow{share: share, opName: expandShareOp.Id, opType: expandShareOp.Type}, nil
	}

	// no existing share Expansion, proceed to instance check
	err = m.verifyNoRunningInstanceOrShareOpsForInstance(share.Parent, ops)
	if err != nil {
		klog.Infof("Instance %v has running share or instnace Op, aborting volume expansion.", share.Parent.Name)
		return nil, status.Error(codes.Aborted, err.Error())
	}

	instance, err := m.cloud.File.GetMultishareInstance(ctx, share.Parent)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	needExpand, targetBytes, err := m.instanceNeedsExpand(ctx, share, reqBytes-share.CapacityBytes)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if needExpand {
		instance.CapacityBytes = targetBytes
		workflow, err := m.startInstanceWorkflow(ctx, &Workflow{instance: instance, opType: util.InstanceUpdate}, ops)
		return workflow, err
	}

	share.CapacityBytes = reqBytes
	return m.startShareWorkflow(ctx, &Workflow{share: share, opType: util.ShareUpdate}, ops)
}

func (m *MultishareOpsManager) startShareExpandWorkflowSafe(ctx context.Context, share *file.Share, reqBytes int64) (*Workflow, error) {
	m.Lock()
	defer m.Unlock()
	ops, err := m.listMultishareResourceRunningOps(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	share.CapacityBytes = reqBytes
	return m.startShareWorkflow(ctx, &Workflow{share: share, opType: util.ShareUpdate}, ops)
}

func (m *MultishareOpsManager) checkAndStartShareDeleteWorkflow(ctx context.Context, share *file.Share) (*Workflow, error) {
	m.Lock()
	defer m.Unlock()

	ops, err := m.listMultishareResourceRunningOps(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// If we find a running delete share op, poll for that to complete.
	deleteShareOp, err := containsOpWithShareTarget(share, util.ShareDelete, ops)
	if err != nil {
		return nil, err
	}
	if deleteShareOp != nil {
		return &Workflow{share: share, opName: deleteShareOp.Id, opType: deleteShareOp.Type}, nil
	}

	return m.startShareWorkflow(ctx, &Workflow{share: share, opType: util.ShareDelete}, ops)
}

func (m *MultishareOpsManager) checkAndStartInstanceDeleteOrShrinkWorkflow(ctx context.Context, instance *file.MultishareInstance) (*Workflow, error) {
	m.Lock()
	defer m.Unlock()

	ops, err := m.listMultishareResourceRunningOps(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	err = m.verifyNoRunningInstanceOrShareOpsForInstance(instance, ops)
	if err != nil {
		return nil, err
	}

	// At this point no new share create or delete would be attempted since the driver has the lock.
	// 1. GET instance . if not found its a no-op return success.
	// 2. evaluate 0 shares.
	// 3. else evaluate instance size with share size sum.
	instance, err = m.cloud.File.GetMultishareInstance(ctx, instance)
	if err != nil {
		if file.IsNotFoundErr(err) {
			return nil, nil
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	shares, err := m.cloud.File.ListShares(ctx, &file.ListFilter{Project: instance.Project, Location: instance.Location, InstanceName: instance.Name})
	if err != nil {
		if file.IsNotFoundErr(err) {
			return nil, nil
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Check for delete
	if len(shares) == 0 {
		w, err := m.startInstanceWorkflow(ctx, &Workflow{instance: instance, opType: util.InstanceDelete}, ops)
		if err != nil {
			if file.IsNotFoundErr(err) {
				return nil, nil
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
		return w, err
	}

	// check for shrink
	var totalShareCap int64
	for _, share := range shares {
		totalShareCap += share.CapacityBytes
	}
	if totalShareCap < instance.CapacityBytes && instance.CapacityBytes > util.MinMultishareInstanceSizeBytes {
		targetShrinkSizeBytes := util.AlignBytes(totalShareCap, util.GbToBytes(instance.CapacityStepSizeGb))
		targetShrinkSizeBytes = util.Max(targetShrinkSizeBytes, util.MinMultishareInstanceSizeBytes)
		if instance.CapacityBytes == targetShrinkSizeBytes {
			return nil, nil
		}

		instance.CapacityBytes = targetShrinkSizeBytes
		w, err := m.startInstanceWorkflow(ctx, &Workflow{instance: instance, opType: util.InstanceUpdate}, ops)
		if err != nil {
			if file.IsNotFoundErr(err) {
				return nil, nil
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
		return w, err
	}

	return nil, nil
}

// listMultishareOps reports all running ops related to multishare instances and share resources. The op target is of the form "projects/<>/locations/<>/instances/<>" or "projects/<>/locations/<>/instances/<>/shares/<>"
func (m *MultishareOpsManager) listMultishareResourceRunningOps(ctx context.Context) ([]*OpInfo, error) {
	ops, err := m.cloud.File.ListOps(ctx, &file.ListFilter{Project: m.cloud.Project, Location: "-"})
	if err != nil {
		return nil, err
	}

	var finalops []*OpInfo
	for _, op := range ops {
		if op.Done {
			continue
		}

		if op.Metadata == nil {
			continue
		}

		var meta filev1beta1multishare.OperationMetadata
		if err := json.Unmarshal(op.Metadata, &meta); err != nil {
			klog.Errorf("Failed to parse metadata for op %s", op.Name)
			continue
		}

		if file.IsInstanceTarget(meta.Target) {
			finalops = append(finalops, &OpInfo{Id: op.Name, Target: meta.Target, Type: util.ConvertInstanceOpVerbToType(meta.Verb)})
		} else if file.IsShareTarget(meta.Target) {
			finalops = append(finalops, &OpInfo{Id: op.Name, Target: meta.Target, Type: util.ConvertShareOpVerbToType(meta.Verb)})
		}
		// TODO: Add other resource types if needed, when we support snapshot/backups.
	}
	return finalops, nil
}

// Whether there is any op with target that is the given share name
func containsOpWithShareName(shareName string, opType util.OperationType, ops []*OpInfo) *OpInfo {
	for _, op := range ops {
		// share names are expected to be unique in the cluster
		if op.Type == opType && strings.Contains(op.Target, shareName) {
			return op
		}
	}

	return nil
}

func containsOpWithShareTarget(share *file.Share, opType util.OperationType, ops []*OpInfo) (*OpInfo, error) {
	shareUri, err := file.GenerateShareURI(share)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to parse share handle, err: %v", err)
	}

	for _, op := range ops {
		// share names are expected to be unique in the cluster
		if op.Type == opType && op.Target == shareUri {
			return op, nil
		}
	}

	return nil, nil
}

func containsOpWithInstanceTargetPrefix(instance *file.MultishareInstance, ops []*OpInfo) (*OpInfo, error) {
	instanceUri, err := file.GenerateMultishareInstanceURI(instance)
	if err != nil {
		return nil, err
	}
	for _, op := range ops {
		// For share targets (e.g projects/<>/locations/<>/instances/<>/shares/<>), explicity check with a "/", to avoid false positives of instances with same prefix name.
		if op.Target == instanceUri || strings.Contains(op.Target, instanceUri+"/") {
			return op, nil
		}
	}

	return nil, nil
}

func (m *MultishareOpsManager) listInstanceForStorageClassPrefix(ctx context.Context, prefix string) ([]*file.MultishareInstance, error) {
	instances, err := m.cloud.File.ListMultishareInstances(ctx, &file.ListFilter{Project: m.cloud.Project, Location: "-"})
	if err != nil {
		return nil, err
	}
	var finalInstances []*file.MultishareInstance
	for _, i := range instances {
		if val, ok := i.Labels[util.ParamMultishareInstanceScLabelKey]; ok && val == prefix {
			finalInstances = append(finalInstances, i)
		}
	}
	return finalInstances, nil
}
