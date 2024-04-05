/*
Copyright 2023 The Kubernetes Authors.

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
	"strings"
	"time"

	filev1beta1 "google.golang.org/api/file/v1beta1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	storageListers "k8s.io/client-go/listers/storage/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	v1 "sigs.k8s.io/gcp-filestore-csi-driver/pkg/apis/multishare/v1"
	clientset "sigs.k8s.io/gcp-filestore-csi-driver/pkg/client/clientset/versioned"
	informers "sigs.k8s.io/gcp-filestore-csi-driver/pkg/client/informers/externalversions/multishare/v1"
	listers "sigs.k8s.io/gcp-filestore-csi-driver/pkg/client/listers/multishare/v1"
	cloud "sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/file"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/util"
)

type Op struct {
	Id     string
	Type   util.OperationType
	Target string
	Err    error
}

type MultishareReconciler struct {
	clientset        clientset.Interface
	config           *GCFSDriverConfig
	cloud            *cloud.Cloud
	controllerServer *controllerServer

	shareLister       listers.ShareInfoLister
	shareListerSynced cache.InformerSynced

	instanceLister       listers.InstanceInfoLister
	instanceListerSynced cache.InformerSynced

	scLister storageListers.StorageClassLister
}

func NewMultishareReconciler(
	clientset clientset.Interface,
	config *GCFSDriverConfig,
	shareInformer informers.ShareInfoInformer,
	instanceInformar informers.InstanceInfoInformer,
	scLister storageListers.StorageClassLister,
) *MultishareReconciler {
	recon := &MultishareReconciler{
		clientset: clientset,
		cloud:     config.Cloud,
		config:    config,
		scLister:  scLister,
	}

	recon.shareLister = shareInformer.Lister()
	recon.shareListerSynced = shareInformer.Informer().HasSynced

	recon.instanceLister = instanceInformar.Lister()
	recon.instanceListerSynced = instanceInformar.Informer().HasSynced

	return recon
}

func (recon *MultishareReconciler) Run(stopCh <-chan struct{}) {
	defer klog.Infof("Shutting down multishare reconciler")

	klog.Infof("Starting cache sync")
	informerSynced := []cache.InformerSynced{recon.shareListerSynced, recon.instanceListerSynced}
	if !cache.WaitForCacheSync(stopCh, informerSynced...) {
		klog.Errorf("Cannot sync caches")
		return
	}

	klog.Infof("Cache synced, starting multishare reconciler")

	go wait.Until(recon.reconcileWorker, 1*time.Minute, stopCh)

	<-stopCh
}

func (recon *MultishareReconciler) reconcileWorker() {
	startTime := time.Now()

	// List out shares, instances managed by this driver.
	shares, err := recon.cloud.File.ListShares(context.TODO(), &file.ListFilter{Project: recon.cloud.Project, Location: "-", InstanceName: "-"})
	if err != nil {
		klog.Errorf("Reconciler Failed to list Shares: %v", err)
		return
	}

	shareListStamp := time.Now()
	klog.V(6).Infof("ListShare finished in %v", time.Since(startTime))

	instances, err := recon.cloud.File.ListMultishareInstances(context.TODO(), &file.ListFilter{Project: recon.cloud.Project, Location: "-"})
	if err != nil {
		klog.Errorf("Reconciler Failed to list Instances: %v", err)
		return
	}
	klog.V(5).Infof("Found %d shares and %d instances", len(shares), len(instances))

	instanceListStamp := time.Now()
	klog.V(6).Infof("ListInstance finished in %v", time.Since(shareListStamp))

	instances, shares, instanceShares, err := recon.managedInstanceAndShare(instances, shares)
	if err != nil {
		klog.Errorf("Failed to filter out managed instance and shares: %s", err.Error())
		return
	}

	// Create shareInfo objects if does not exist, update shareInfo.Status based on listed out shares' status.
	shareInfoMap := recon.createAndUpdateShareInfos(shares)

	shareInfoList, err := recon.shareLister.ShareInfos(util.ManagedFilestoreCSINamespace).List(labels.Everything())
	if err != nil {
		klog.Errorf("Filestore CSI driver cannot list ShareInfo objects: %v", err)
		return
	}
	klog.V(6).Infof("Listed out %d shareInfo objects", len(shareInfoList))
	for _, shareInfo := range shareInfoList {
		shareName := shareInfo.Name
		if _, ok := shareInfoMap[shareName]; !ok {
			shareInfoMap[shareName] = shareInfo
		}
	}

	// Create instanceInfo objects if it does not exist, update instanceInfo.Status based on listed out instances' status.
	instanceInfoMap := recon.createAndUpdateInstanceInfos(instances, instanceShares)

	instanceInfoList, err := recon.instanceLister.InstanceInfos(util.ManagedFilestoreCSINamespace).List(labels.Everything())
	if err != nil {
		klog.Errorf("Filestore CSI driver cannot list InstanceInfo objects: %v", err)
		return
	}
	klog.V(6).Infof("Listed out %d instanceInfo objects", len(instanceInfoList))
	for _, instanceInfo := range instanceInfoList {
		instanceURI := util.InstanceInfoNameToInstanceURI(instanceInfo.Name)
		if _, ok := instanceInfoMap[instanceURI]; !ok {
			instanceInfo, err := recon.maybeRemoveInstanceInfoFinalizer(instanceInfo)
			if err != nil {
				klog.Errorf("Error removing finalizer from InstanceInfo %q: %v", instanceInfo.Name, err)
				continue
			}
			if instanceInfo != nil {
				instanceInfoMap[instanceURI] = instanceInfo
			}
		}
	}

	reconstructionStamp := time.Now()
	klog.V(6).Infof("reconstruction finished in %v", time.Since(instanceListStamp))

	// Assign un-assigned shares to instances; update shareInfo and instanceInfo accordingly,
	// if there's inconsistency between share and instnace then share has source of truth.
	recon.assignSharesToInstances(shareInfoMap, instanceInfoMap, instanceShares)

	assignmentStamp := time.Now()
	klog.V(6).Infof("assignment finished in %v", time.Since(reconstructionStamp))

	ops, err := recon.listMultishareResourceOps(context.TODO())
	if err != nil {
		klog.Errorf("error listing ops: %s", err.Error())
		return
	}

	opStamp := time.Now()
	klog.V(6).Infof("List Op finished in %v", time.Since(assignmentStamp))

	recon.sendInstanceRequests(instanceInfoMap, ops)

	instanceReqStamp := time.Now()
	klog.V(6).Infof("instanceRequest finished in %v", time.Since(opStamp))

	recon.sendShareRequests(instanceInfoMap, shareInfoMap, instanceShares, ops)

	klog.V(6).Infof("shareRequest finished in %v", time.Since(instanceReqStamp))

	klog.Infof("Reconciliation round finished after %v", time.Since(startTime))
}

func (recon *MultishareReconciler) sendShareRequests(instanceInfos map[string]*v1.InstanceInfo, shareInfos map[string]*v1.ShareInfo, instanceShares map[string][]*file.Share, ops []*Op) {
	for _, shareInfo := range shareInfos {
		needDelete := shareInfo.DeletionTimestamp != nil

		if needDelete {
			if shareInfo.Status == nil {
				klog.Errorf("shareInfo %s marked for delete but Status subresource is nil", shareInfo.Name)
				continue
			}
			if shareInfo.Status.ShareStatus == v1.DELETED {
				klog.Infof("shareInfo status DELETED, skip sending share request for %s", shareInfo.Name)
				continue
			}

			if !shareExist(shareInfo, instanceShares) {
				newInstanceInfo, err := recon.maybeMarkShareInfoStatusDeleted(shareInfo, instanceInfos[shareInfo.Status.InstanceHandle])
				if err != nil {
					klog.Errorf("failed to mark shareInfo %s deleted : %s", shareInfo.Name, err.Error())
					continue
				}
				instanceInfos[shareInfo.Status.InstanceHandle] = newInstanceInfo
				continue
			}
		}

		if shareInfo.Status == nil || shareInfo.Status.InstanceHandle == "" {
			klog.Infof("share %s not assigned to any instance yet", shareInfo.Name)
			continue
		}

		if !needDelete && shareInfo.Spec.CapacityBytes == shareInfo.Status.CapacityBytes {
			klog.V(6).Infof("no need to send any share request for %s", shareInfo.Name)
			continue
		}

		assignedInstanceInfo, ok := instanceInfos[shareInfo.Status.InstanceHandle]
		if !ok {
			klog.Errorf("share %s is assigned to %s but instanceInfo does not exist", shareInfo.Name, shareInfo.Status.InstanceHandle)
			continue
		}
		if assignedInstanceInfo.Status == nil || assignedInstanceInfo.Status.InstanceStatus != v1.READY || assignedInstanceInfo.Status.CapacityBytes < assignedInstanceInfo.Spec.CapacityBytes {
			klog.Infof("Instance %s is not ready", assignedInstanceInfo.Name)
			if assignedInstanceInfo.Status != nil && assignedInstanceInfo.Status.Error != "" {
				recon.updateShareInfoErr(shareInfo, fmt.Errorf(assignedInstanceInfo.Status.Error))
			}
			continue
		}

		project, instanceRegion, name, err := util.ParseInstanceURI(shareInfo.Status.InstanceHandle)
		if err != nil {
			klog.Errorf("failed to parse instanceURI %q: %s", shareInfo.Status.InstanceHandle, err.Error())
			continue
		}
		share := &file.Share{
			Name: shareInfo.Spec.ShareName,
			Parent: &file.MultishareInstance{
				Project:  project,
				Location: instanceRegion,
				Name:     name,
			},
			CapacityBytes:  shareInfo.Spec.CapacityBytes,
			MountPointName: shareInfo.Spec.ShareName,
			Labels:         shareInfo.Labels,
		}

		shareURI, err := file.GenerateShareURI(share)
		if err != nil {
			klog.Errorf("error generating shareURI for %s: %s", shareInfo.Name, err.Error())
		}
		op, err := runningOpMaybeErrForTarget(shareURI, ops)
		if err != nil {
			recon.updateShareInfoErr(shareInfo, err)
		}

		if op == nil {
			klog.Infof("no running Op found for %s", shareURI)
			if needDelete {
				klog.Infof("Starting share Delete operation for %s", shareURI)
				_, err = recon.cloud.File.StartDeleteShareOp(context.TODO(), share)
			} else if shareInfo.Status.ShareStatus != v1.READY {
				klog.Infof("Starting share Create operation for %s", shareURI)
				_, err = recon.cloud.File.StartCreateShareOp(context.TODO(), share)
			} else if shareInfo.Status.CapacityBytes != 0 && shareInfo.Spec.CapacityBytes != shareInfo.Status.CapacityBytes {
				klog.Infof("Starting share Resize operation for %s", shareURI)
				_, err = recon.cloud.File.StartResizeShareOp(context.TODO(), share)
			}
		}
		if err != nil {
			recon.updateShareInfoErr(shareInfo, err)
		}
	}
}

// maybeUpdateShareInfoStatus will unassign share from instanceInfo, then upon success, mark shareInfo.Status.ShareStatus as DELETED.
func (recon *MultishareReconciler) maybeMarkShareInfoStatusDeleted(shareInfo *v1.ShareInfo, instanceInfo *v1.InstanceInfo) (*v1.InstanceInfo, error) {
	if instanceInfo == nil {
		return instanceInfo, fmt.Errorf("instanceInfo does not exist for shareInfo %s which is assigned to %q", shareInfo.Name, shareInfo.Status.InstanceHandle)
	}

	klog.Infof("mark shareInfo %s ShareStatus as deleted", shareInfo.Name)
	instanceInfoClone := instanceInfo.DeepCopy()
	instanceInfoClone, updated := recon.removeShareFromInstanceInfo(instanceInfoClone, shareInfo.Name)
	if updated {
		klog.Infof("trying to remove share %s from assigned instanceInfo %s", shareInfo.Name, instanceInfo.Name)
		var err error
		instanceInfoClone, err = recon.updateInstanceInfoStatus(context.TODO(), instanceInfoClone)
		if err != nil {
			return instanceInfo, fmt.Errorf("failed to remove share form instanceInfo Status: %s", err.Error())
		}
	}
	shareInfoClone := shareInfo.DeepCopy()
	if shareInfo.Status == nil {
		shareInfoClone.Status = &v1.ShareInfoStatus{}
	}
	shareInfoClone.Status.ShareStatus = v1.DELETED
	_, err := recon.updateShareInfoStatus(context.TODO(), shareInfoClone)
	if err != nil {
		return instanceInfoClone, fmt.Errorf("failed to update %s.Status.ShareStatus to DELETED: %s", shareInfo.Name, err.Error())
	}

	return instanceInfoClone, nil
}

func (recon *MultishareReconciler) sendInstanceRequests(instanceInfos map[string]*v1.InstanceInfo, ops []*Op) {
	for _, instanceInfo := range instanceInfos {
		needDelete := instanceInfo.DeletionTimestamp != nil
		if !needDelete && instanceInfo.Status != nil &&
			instanceInfo.Spec.CapacityBytes == instanceInfo.Status.CapacityBytes &&
			(instanceInfo.Status.InstanceStatus == v1.READY || instanceInfo.Status.InstanceStatus == v1.UPDATING) {
			// If the instance is in "UPDATING" state, it might have been deleted manually or is having some issue.
			// The reconciler should not try to call Instance Create API in this case.

			klog.V(6).Infof("no need to send any instance request for %s", instanceInfo.Name)
			continue
		}

		instanceURI := util.InstanceInfoNameToInstanceURI(instanceInfo.Name)
		op, err := runningOpMaybeErrForTarget(instanceURI, ops)
		if err != nil {
			recon.updateInstanceInfoErr(instanceInfo, err)
		}
		if op == nil {
			klog.Infof("no running Op found for %s", instanceURI)
			var instance *file.MultishareInstance
			instance, err = basicMultishareInstanceFromInstanceInfo(instanceInfo)
			if err != nil {
				klog.Errorf("error while generating instance for %s to call API: %s", instanceInfo.Name, err.Error())
				continue
			}

			if needDelete {
				klog.Infof("Starting instance Delete operation for %s", instanceURI)
				_, err = recon.cloud.File.StartDeleteMultishareInstanceOp(context.TODO(), instance)

			} else if instanceInfo.Status == nil || (instanceInfo.Status.InstanceStatus != v1.READY && instanceInfo.Status.InstanceStatus != v1.UPDATING) {
				instance, err = recon.generateNewMultishareInstance(instanceInfo)
				if err != nil {
					klog.Errorf("error while generating new instance for %s to call API: %s", instanceInfo.Name, err.Error())
					continue
				}
				klog.Infof("Starting instance Create operation for %s", instanceURI)
				_, err = recon.cloud.File.StartCreateMultishareInstanceOp(context.TODO(), instance)

				defer recon.controllerServer.config.ipAllocator.ReleaseIPRange(instance.Network.ReservedIpRange)

			} else if instanceInfo.Status != nil && instanceInfo.Status.CapacityBytes != 0 && instanceInfo.Spec.CapacityBytes != instanceInfo.Status.CapacityBytes {
				klog.Infof("Starting instance Resize operation for %s", instanceURI)
				_, err = recon.cloud.File.StartResizeMultishareInstanceOp(context.TODO(), instance)
			}
		}

		if err != nil {
			recon.updateInstanceInfoErr(instanceInfo, err)
		}
	}
}

func (recon *MultishareReconciler) updateInstanceInfoErr(instanceInfo *v1.InstanceInfo, err error) {
	klog.Infof("found error message for instance %s", instanceInfo.Name)
	instanceInfoClone := instanceInfo.DeepCopy()
	if instanceInfoClone.Status == nil {
		instanceInfoClone.Status = &v1.InstanceInfoStatus{}
	}

	if !strings.EqualFold(instanceInfoClone.Status.Error, err.Error()) {
		klog.V(6).Infof("previous Error message: %s", instanceInfoClone.Status.Error)
		instanceInfoClone.Status.Error = err.Error()
		klog.V(6).Infof("new error message found: %s, trying to update instanceInfo %s", err.Error(), instanceInfoClone.Name)
		_, err := recon.updateInstanceInfoStatus(context.TODO(), instanceInfoClone)
		if err != nil {
			klog.Errorf("failed to update instanceInfo %s: %s", instanceInfoClone.Name, err.Error())
		}
	}
}

func (recon *MultishareReconciler) updateShareInfoErr(shareInfo *v1.ShareInfo, err error) {
	shareInfoClone := shareInfo.DeepCopy()
	if shareInfoClone.Status == nil {
		shareInfoClone.Status = &v1.ShareInfoStatus{}
	}

	if !strings.EqualFold(shareInfoClone.Status.Error, err.Error()) {
		klog.V(6).Infof("previous Error message: %s", shareInfoClone.Status.Error)
		shareInfoClone.Status.Error = err.Error()
		klog.V(6).Infof("new error message found: %s, trying to update shareInfo %s", err.Error(), shareInfoClone.Name)
		_, err := recon.updateShareInfoStatus(context.TODO(), shareInfoClone)
		if err != nil {
			klog.Errorf("failed to update shareInfo %s: %s", shareInfoClone.Name, err.Error())
		}
	}
}

// basicMultishareInstanceFromInstanceInfo generates a MultishareInstance object with basic info for deletion and expansion purpose
func basicMultishareInstanceFromInstanceInfo(instanceInfo *v1.InstanceInfo) (*file.MultishareInstance, error) {
	instanceURI := util.InstanceInfoNameToInstanceURI(instanceInfo.Name)
	project, instanceRegion, name, err := util.ParseInstanceURI(instanceURI)
	if err != nil {
		return nil, fmt.Errorf("failed to parse instanceURI %q: %s", instanceURI, err.Error())
	}
	return &file.MultishareInstance{
		Project:       project,
		Name:          name,
		CapacityBytes: instanceInfo.Spec.CapacityBytes,
		Location:      instanceRegion,
		Tier:          enterpriseTier,
	}, nil
}

// generateNewMultishareInstance generates a MultishareInstance object for the purpose of instance creation.
// During this function's execution, it might call controllerServer.reserveIPRange to reserve an IP range.
func (recon *MultishareReconciler) generateNewMultishareInstance(instanceInfo *v1.InstanceInfo) (*file.MultishareInstance, error) {
	instanceURI := util.InstanceInfoNameToInstanceURI(instanceInfo.Name)
	project, instanceRegion, name, err := util.ParseInstanceURI(instanceURI)
	if err != nil {
		return nil, fmt.Errorf("failed to parse instanceURI %q: %s", instanceURI, err.Error())
	}

	network := defaultNetwork
	connectMode := directPeering
	kmsKeyName := ""

	storageClass, err := recon.scLister.Get(instanceInfo.Spec.StorageClassName)
	if err != nil || storageClass == nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to get storageClass %q : %v", instanceInfo.Spec.StorageClassName, err)
	}

	params := storageClass.Parameters
	for k, v := range params {
		switch strings.ToLower(k) {
		case paramTier:
			if v != enterpriseTier {
				klog.Errorf("only tier %q is supported for multishare. Ignoring %q", enterpriseTier, v)
			}
		case paramNetwork:
			network = v
		case ParamConnectMode:
			connectMode = v
			if connectMode != directPeering && connectMode != privateServiceAccess {
				return nil, status.Errorf(codes.InvalidArgument, "connect mode can only be one of %q or %q", directPeering, privateServiceAccess)
			}
		case ParamInstanceEncryptionKmsKey:
			kmsKeyName = v
		case ParamReservedIPV4CIDR, ParamReservedIPRange:

		case ParamMultishareInstanceScLabel, ParameterKeyLabels, ParameterKeyPVCName, ParameterKeyPVCNamespace, ParameterKeyPVName, paramMultishare:
		case "csiprovisionersecretname", "csiprovisionersecretnamespace":
		default:
			klog.Errorf("Ignoring invalid parameter %q", k)
		}
	}

	clusterLocation := recon.cloud.Zone
	if recon.config.IsRegional {
		var err error
		clusterLocation, err = util.GetRegionFromZone(clusterLocation)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get region for regional cluster: %v", err.Error())
		}
	}

	labels, err := extractInstanceLabels(params, recon.config.ExtraVolumeLabels, recon.config.Name, recon.config.ClusterName, clusterLocation)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	instance := &file.MultishareInstance{
		Project:       project,
		Name:          name,
		CapacityBytes: instanceInfo.Spec.CapacityBytes,
		Location:      instanceRegion,
		Tier:          enterpriseTier,
		Network: file.Network{
			Name:        network,
			ConnectMode: connectMode,
		},
		KmsKeyName:  kmsKeyName,
		Labels:      labels,
		Description: generateInstanceDescFromEcfsDesc(recon.config.EcfsDescription),
	}

	if recon.controllerServer.config.multiShareController.featureMaxSharePerInstance {
		instance.MaxShareCount = recon.parseMaxSharePerInstance(instanceInfo.Spec.Parameters)
	}

	// reserve ip range
	var reservedIPRange string
	if connectMode == privateServiceAccess {
		if reservedIPRange, ok := params[ParamReservedIPRange]; ok {
			if IsCIDR(reservedIPRange) {
				return nil, status.Error(codes.InvalidArgument, "When using connect mode PRIVATE_SERVICE_ACCESS, if reserved IP range is specified, it must be a named address range instead of direct CIDR value")
			}
			instance.Network.ReservedIpRange = reservedIPRange
		}
	} else if reservedIPV4CIDR, ok := params[ParamReservedIPV4CIDR]; ok {
		if instanceInfo.Status != nil && instanceInfo.Status.Cidr != "" {
			reservedIPRange = instanceInfo.Status.Cidr
		} else {
			klog.Infof("instanceInfo %s doesn't already have cidr, reserving IP range", instanceInfo.Name)
			var err error
			reservedIPRange, err = recon.controllerServer.reserveIPRange(context.TODO(), &file.ServiceInstance{
				Project:  instance.Project,
				Name:     instance.Name,
				Location: instance.Location,
				Tier:     instance.Tier,
				Network:  instance.Network,
			}, reservedIPV4CIDR)

			if err != nil {
				return nil, err
			}
		}

		// Adding the reserved IP range to the instance object
		instance.Network.ReservedIpRange = reservedIPRange
	}

	return instance, nil
}

func (recon *MultishareReconciler) assignSharesToInstances(shareInfos map[string]*v1.ShareInfo, instanceInfos map[string]*v1.InstanceInfo, instanceShares map[string][]*file.Share) {
	recon.fixTwoWayPointers(shareInfos, instanceInfos)

	recon.assignSharesToEligibleOrNewInstances(shareInfos, instanceInfos, instanceShares)

	// Have to call deleteOrResizeInstances() after assigning shares and/or fixing two way pointers because no resizing were attempted in
	// assignSharesToEligibleOrNewInstances() or fixTwoWayPointers()
	recon.deleteOrResizeInstances(instanceInfos)
}

// fixTwoWayPointers scans over all instanceInfo objects and try to fix the 2 way pointer between instanceInfo and shareInfo objects.
// If ShareInfo object does not have the pointer then use instanceInfo as source of truth because we assign to instanceInfo first; if
// they both have pointer but they don't point to each other, take the one in shareInfo as source of truth because share could already
// have been created and we couldn't move shares around.
// instanceInfo.Status.ShareNames -> shareInfo
// shareInfo.Status.InstanceHandle -> instanceInfo
func (recon *MultishareReconciler) fixTwoWayPointers(shareInfos map[string]*v1.ShareInfo, instanceInfos map[string]*v1.InstanceInfo) {
	for _, instanceInfo := range instanceInfos {
		if instanceInfo.Status == nil {
			klog.V(6).Infof("Instance %q has Status nil", instanceInfo.Name)
			continue
		}

		instanceInfoClone := instanceInfo.DeepCopy()
		updated := false

		instanceURI := util.InstanceInfoNameToInstanceURI(instanceInfo.Name)
		for _, shareName := range instanceInfo.Status.ShareNames {
			shareInfo, ok := shareInfos[shareName]
			if !ok {
				klog.Errorf("Share %q is assigned to instance %q but shareInfo does not exist", shareName, instanceURI)
				continue
			}
			if shareInfo.Status == nil || shareInfo.Status.InstanceHandle == "" {
				shareInfo, err := recon.assignInstanceToShareInfo(shareInfo, instanceURI)
				if err != nil {
					klog.Errorf("Cannot update instanceHandle to shareInfo %q: %v", shareName, err)
					continue
				}
				shareInfos[shareName] = shareInfo
			} else if shareInfo.Status.InstanceHandle != instanceURI {
				// This case should be rare, however, if there's a race or crash between shareInfo update and instanceInfo update, the 2 way pointer may not match.
				klog.Warningf("InstanceInfo %q has share %q but its shareInfo points to instance %q", instanceInfo.Name, shareName, shareInfo.Status.InstanceHandle)

				// If the share is already marked for deletion, don't try to add it to the instanceInfo it points to.
				if shareInfo.DeletionTimestamp == nil {
					klog.Infof("Deletion Timestamp not set on shareInfo %q, trying to update instanceInfo %q", shareName, shareInfo.Status.InstanceHandle)
					actualAssigned, ok := instanceInfos[shareInfo.Status.InstanceHandle]
					var err error
					if !ok {
						klog.Errorf("Share %q is assigned to instance %q but instanceInfo does not exist. Trying to create one", shareName, shareInfo.Status.InstanceHandle)
						actualAssigned, err = recon.generateInstanceInfo(shareInfo.Status.InstanceHandle, shareInfo.Spec.InstancePoolTag, shareInfo.Spec.Parameters)
						if err != nil {
							klog.Errorf("Failed to create instanceInfo %q: %v", shareInfo.Status.InstanceHandle, err)
							continue
						}
					}
					actualAssigned, err = recon.assignShareToInstanceInfo(actualAssigned, shareName)
					if err != nil {
						klog.Errorf("Failed to assign share %q to instanceInfo %q: %v", shareName, shareInfo.Status.InstanceHandle, err)
						continue
					}

					instanceInfos[shareInfo.Status.InstanceHandle] = actualAssigned
				}

				instanceInfoClone, updated = recon.removeShareFromInstanceInfo(instanceInfoClone, shareName)
			}
		}

		var err error
		if updated {
			klog.Infof("InstanceInfo %q has updated share assignment, trying to update object", instanceInfoClone.Name)
			instanceInfoClone, err = recon.updateInstanceInfoStatus(context.TODO(), instanceInfoClone)
			if err != nil {
				klog.Errorf("Failed to update status subresource of instanceInfo %q: %v", instanceInfo.Name, err)
				continue
			}
			instanceInfos[instanceURI] = instanceInfoClone
		}
	}
}

// assignSharesToEligibleOrNewInstances assigns shares that are not already assigned to eligible instances.
// If there're no eligible instances, generate a new one.
func (recon *MultishareReconciler) assignSharesToEligibleOrNewInstances(shareInfos map[string]*v1.ShareInfo, instanceInfos map[string]*v1.InstanceInfo, instanceShares map[string][]*file.Share) {
	for _, shareInfo := range shareInfos {
		if shareInfo.Status == nil || shareInfo.Status.InstanceHandle == "" {

			// If share is up to delete, don't assign it even if it's not yet assigned.
			// This situation should not happen because DeleteVolume shouldn't be called until CreateVolume already succeeded.
			if shareInfo.DeletionTimestamp != nil {
				klog.Warningf("Skipping share assignment for %q due to deletionTimestamp despite it is not assigned still", shareInfo.Name)
				continue
			}
			scTag := shareInfo.Spec.InstancePoolTag
			if scTag == "" {
				klog.Errorf("ShareInfo %q has empty instancePoolTag", shareInfo.Name)
				continue
			}

			var instanceURI string
			var err error
			for _, instanceInfo := range instanceInfos {
				_, ok := instanceShares[util.InstanceInfoNameToInstanceURI(instanceInfo.Name)]
				if !ok && instanceInfo.Status != nil && instanceInfo.Status.InstanceStatus != "" {
					// if InstanceStatus is not empty but instance is no longer present, instance might have been manually deleted by user and can no longer be used
					klog.Warningf("instanceInfo %s has non empty InstanceStatus but underlying instance does not exist. Skip assignment to that instance", instanceInfo.Name)
				}
				if recon.instanceFitShare(instanceInfo, shareInfo) {
					instanceURI = util.InstanceInfoNameToInstanceURI(instanceInfo.Name)
					instanceInfo, err = recon.assignShareToInstanceInfo(instanceInfo, shareInfo.Name)
					if err != nil {
						klog.Errorf("Failed to add share %q to instanceInfo %q: %v", shareInfo.Name, instanceInfo.Name, err)
						continue
					}
					klog.Infof("Share %q is now assigned to instance %q", shareInfo.Name, instanceURI)
					instanceInfos[instanceURI] = instanceInfo
					break
				}
			}
			if err != nil {
				continue
			}

			if instanceURI == "" {
				klog.Infof("Couldn't find instance to fit share %q, generating new instance", shareInfo.Name)

				instanceURI, _ = file.GenerateMultishareInstanceURI(&file.MultishareInstance{
					Project:  recon.cloud.Project,
					Location: shareInfo.Spec.Region,
					Name:     util.NewMultishareInstancePrefix + string(uuid.NewUUID()),
				})

				instanceInfo, err := recon.generateInstanceInfo(instanceURI, shareInfo.Spec.InstancePoolTag, shareInfo.Spec.Parameters)
				if err != nil {
					klog.Errorf("Failed to create new instanceInfo %q: %v", instanceURI, err)
					continue
				}
				instanceInfo, err = recon.assignShareToInstanceInfo(instanceInfo, shareInfo.Name)
				if err != nil {
					klog.Errorf("Failed to add share %q to instanceInfo %q: %v", shareInfo.Name, instanceInfo.Name, err)
					continue
				}
				klog.Infof("Share %q is now assigned to instance %q", shareInfo.Name, instanceURI)
				instanceInfos[instanceURI] = instanceInfo
			}

			shareInfo, err := recon.assignInstanceToShareInfo(shareInfo, instanceURI)
			if err != nil {
				klog.Errorf("Cannot update the instanceHandle of shareInfo %q: %v", shareInfo.Name, err)
				continue
			}
			shareInfos[shareInfo.Name] = shareInfo
		}
	}
}

// deleteOrResizeInstances takes a map of instanceUri -> instanceInfos and
// 1) add DeletionTimestamp for any instanceInfo that's empty (doesn't have share assigned to it).
// 2) calculates and updates the minimum viable Spec.CapacityBytes for instanceInfos that are not empty.
func (recon *MultishareReconciler) deleteOrResizeInstances(instanceInfos map[string]*v1.InstanceInfo) {
	for instanceURI, instanceInfo := range instanceInfos {
		if instanceInfo.DeletionTimestamp != nil {
			continue
		}

		instanceInfoClone := instanceInfo.DeepCopy()
		var updated bool
		if instanceEmpty(instanceInfo) {
			klog.Infof("InstanceInfo %q needs to be deleted, trying to add deletionTimestamp", instanceInfo.Name)
			err := recon.deleteInstanceInfo(context.TODO(), instanceInfoClone)
			if err != nil {
				klog.Errorf("Failed to add deletionTimestamp to instanceInfo %q: %v", instanceInfo.Name, err)
				continue
			}
		} else {
			instanceInfoClone, updated = recon.instanceInfoNewCapacity(instanceInfoClone)
			if updated {
				klog.Infof("InstanceInfo %q has updated capacity, trying to update object", instanceInfo.Name)
				instanceInfoClone, err := recon.updateInstanceInfo(context.TODO(), instanceInfoClone)
				if err != nil {
					klog.Errorf("Failed to update instanceInfo %q: %v", instanceInfo.Name, err)
					continue
				}
				instanceInfos[instanceURI] = instanceInfoClone
			}
		}
	}
}

// generateInstanceInfo generates and creates a new instanceInfo object based on instanceURI and storage class tag.
func (recon *MultishareReconciler) generateInstanceInfo(instanceURI string, scTag string, shareParams map[string]string) (*v1.InstanceInfo, error) {
	storageClass, err := recon.storageClassFromTag(scTag)
	if err != nil {
		return nil, err
	}
	newInstanceInfo := &v1.InstanceInfo{
		ObjectMeta: metav1.ObjectMeta{
			Name:       util.InstanceURIToInstanceInfoName(instanceURI),
			Finalizers: []string{util.FilestoreResourceCleanupFinalizer},
			Labels: map[string]string{
				ParamMultishareInstanceScLabel: storageClass.Parameters[ParamMultishareInstanceScLabel],
			},
		},
		Spec: v1.InstanceInfoSpec{
			CapacityBytes:    util.MinMultishareInstanceSizeBytes,
			StorageClassName: storageClass.Name,
			Parameters:       shareParams,
		},
	}
	return recon.createInstanceInfo(context.TODO(), newInstanceInfo)
}

// storageClassFromTag finds and returns the first storageclass with a matching scTag.
func (recon *MultishareReconciler) storageClassFromTag(scTag string) (*storagev1.StorageClass, error) {
	if scTag == "" {
		return nil, fmt.Errorf("storageClassTag cannot be empty")
	}
	storageClasses, err := recon.scLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	klog.V(5).Infof("Storage class Lister lists %d items", len(storageClasses))
	for _, sc := range storageClasses {
		klog.V(6).Infof("Storageclass %q has Parameters %v", sc.Name, sc.Parameters)
		if sc.Parameters[ParamMultishareInstanceScLabel] == scTag {
			return sc, nil
		}
	}
	return nil, fmt.Errorf("no storageclass match storageClassTag %q", scTag)
}

func (recon *MultishareReconciler) assignShareToInstanceInfo(instanceInfo *v1.InstanceInfo, shareName string) (*v1.InstanceInfo, error) {
	instanceInfoClone := instanceInfo.DeepCopy()
	if instanceInfoClone.Status == nil {
		instanceInfoClone.Status = &v1.InstanceInfoStatus{}
	}
	for _, name := range instanceInfoClone.Status.ShareNames {
		if name == shareName {
			return instanceInfo, nil
		}
	}
	klog.Infof("Appending share %q to %q instanceInfo's assigned share list", shareName, instanceInfoClone.Name)
	instanceInfoClone.Status.ShareNames = append(instanceInfoClone.Status.ShareNames, shareName)

	return recon.updateInstanceInfoStatus(context.TODO(), instanceInfoClone)
}

func (recon *MultishareReconciler) assignInstanceToShareInfo(shareInfo *v1.ShareInfo, instanceURI string) (*v1.ShareInfo, error) {
	shareInfoClone := shareInfo.DeepCopy()
	if shareInfoClone.Status == nil {
		shareInfoClone.Status = &v1.ShareInfoStatus{}
	}
	shareInfoClone.Status.InstanceHandle = instanceURI
	klog.Infof("Try to assign share %q to instance %q", shareInfo.Name, instanceURI)
	return recon.updateShareInfoStatus(context.TODO(), shareInfoClone)
}

// createAndUpdateInstanceInfos create instanceInfo objects if needed and updates their statuses to match with actual state of the world.
// InstanceInfo objects in the returned map must be treated as read only.
func (recon *MultishareReconciler) createAndUpdateInstanceInfos(instances []*file.MultishareInstance, instanceShares map[string][]*file.Share) map[string]*v1.InstanceInfo {
	instanceInfoMap := make(map[string]*v1.InstanceInfo)

	for _, instance := range instances {
		instanceURI, err := file.GenerateMultishareInstanceURI(instance)
		if err != nil {
			klog.Errorf("Couldn't generate instanceURI: %v for instance %q", err, instance.Name)
			continue
		}
		iiName := util.InstanceURIToInstanceInfoName(instanceURI)

		instanceInfo, err := recon.instanceLister.InstanceInfos(util.ManagedFilestoreCSINamespace).Get(iiName)
		if err != nil {
			if !errors.IsNotFound(err) {
				klog.Errorf("Error getting instanceInfo %q from informer: %v", iiName, err)
				continue
			}
			instanceInfo, err = recon.clientset.MultishareV1().InstanceInfos(util.ManagedFilestoreCSINamespace).Get(context.TODO(), iiName, metav1.GetOptions{})
			if err != nil {
				if !errors.IsNotFound(err) {
					klog.Errorf("Error getting instanceInfo %q from api server: %v", iiName, err)
					continue
				}
				klog.V(4).Infof("InstanceInfo object for instance %q not found in API server", instance.Name)
				instanceInfo = nil
			} else {
				klog.V(4).Infof("InstanceInfo object for instance %q not found in informer cache but found in API server", instance.Name)
			}
		}

		if instanceInfo == nil {
			instanceInfo, err = recon.reconstructInstanceInfo(iiName, instance, instanceInfo)
		}
		if err != nil {
			klog.Errorf("Error creating InstanceInfo %q: %v", iiName, err)
			continue
		}
		instanceInfoMap[instanceURI] = instanceInfo

		instanceInfo, err = recon.maybeUpdateInstanceInfoStatus(instance, instanceInfo, instanceShares)
		if err != nil {
			klog.Errorf("Error updating instanceInfo %q: %v", iiName, err)
			continue
		}
		instanceInfoMap[instanceURI] = instanceInfo
	}

	return instanceInfoMap
}

// createAndUpdateShareInfos create shareInfo objects if needed and updates their statuses to match with actual state of the world.
// ShareInfo objects in the returned map must be treated as read only.
func (recon *MultishareReconciler) createAndUpdateShareInfos(shares []*file.Share) map[string]*v1.ShareInfo {
	shareInfoMap := make(map[string]*v1.ShareInfo)

	// Create ShareInfo that are not reflected.
	for _, share := range shares {
		shareInfoName := util.ShareToShareInfoName(share.Name)
		shareInfo, err := recon.shareLister.ShareInfos(util.ManagedFilestoreCSINamespace).Get(shareInfoName)
		if err != nil {
			if !errors.IsNotFound(err) {
				klog.Errorf("Error getting shareInfo %q from informer: %v", shareInfoName, err)
				continue
			}
			shareInfo, err = recon.clientset.MultishareV1().ShareInfos(util.ManagedFilestoreCSINamespace).Get(context.TODO(), shareInfoName, metav1.GetOptions{})
			if err != nil {
				if !errors.IsNotFound(err) {
					klog.Errorf("Error getting shareInfo %q from api server: %v", shareInfoName, err)
					continue
				}
				// shareInfo does not exist for share
				klog.V(4).Infof("ShareInfo object for share %q not found in API server", shareInfoName)
				shareInfo = nil
			} else {
				// shareInfo exist in api server but not cache
				klog.V(4).Infof("ShareInfo object for share %q not found in informer cache but found in api server", shareInfoName)
			}
		}

		if shareInfo == nil {
			shareInfo, err = recon.createShareInfo(share, shareInfo)
		}
		if err != nil {
			klog.Errorf("Error creating ShareInfo %q: %v", shareInfoName, err)
			continue
		}
		shareInfoMap[shareInfoName] = shareInfo

		shareInfo, err = recon.maybeUpdateShareInfoStatus(share, shareInfo)
		if err != nil {
			klog.Errorf("Error updating ShareInfo %q: %v", shareInfoName, err)
			continue
		}
		shareInfoMap[shareInfoName] = shareInfo
	}

	return shareInfoMap
}

func (recon *MultishareReconciler) reconstructInstanceInfo(iiName string, instance *file.MultishareInstance, instanceInfo *v1.InstanceInfo) (*v1.InstanceInfo, error) {

	// In the reconstruct case, the instance is already present with a scTag. And we cannot change the instances' property anymore.
	// We don't need to have storageClassName field (the storageclass might not exist either) and it will act like a distinguisher
	// between migrated instances and driver-created instances.
	instanceInfo = &v1.InstanceInfo{
		ObjectMeta: metav1.ObjectMeta{
			Name:       iiName,
			Finalizers: []string{util.FilestoreResourceCleanupFinalizer},
			Labels: map[string]string{
				ParamMultishareInstanceScLabel: instance.Labels[util.ParamMultishareInstanceScLabelKey],
			},
		},
		Spec: v1.InstanceInfoSpec{
			CapacityBytes: instance.CapacityBytes,
		},
	}
	return recon.createInstanceInfo(context.TODO(), instanceInfo)
}

func (recon *MultishareReconciler) createShareInfo(share *file.Share, shareInfo *v1.ShareInfo) (*v1.ShareInfo, error) {
	shareInfo = &v1.ShareInfo{
		ObjectMeta: metav1.ObjectMeta{
			Name:       util.ShareToShareInfoName(share.Name),
			Finalizers: []string{util.FilestoreResourceCleanupFinalizer},
		},
		Spec: v1.ShareInfoSpec{
			ShareName:     share.Name,
			CapacityBytes: share.CapacityBytes,
			Region:        share.Parent.Location,
		},
	}
	klog.Infof("Trying to create ShareInfo %s", shareInfo.Name)
	result, err := recon.clientset.MultishareV1().ShareInfos(util.ManagedFilestoreCSINamespace).Create(context.TODO(), shareInfo, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// removeShareFromInstanceInfo removes share assignment from instanceInfo object in place but does not re-calculate required instance Size.
func (recon *MultishareReconciler) removeShareFromInstanceInfo(instanceInfoClone *v1.InstanceInfo, shareName string) (*v1.InstanceInfo, bool) {
	if instanceInfoClone.Status == nil || len(instanceInfoClone.Status.ShareNames) == 0 {
		klog.Warningf("Trying to remove share %q from instanceInfo %q but it does not have Status subresource or no assigned shares", shareName, instanceInfoClone.Name)
		return instanceInfoClone, false
	}
	newShareNames := make([]string, 0, len(instanceInfoClone.Status.ShareNames))
	updated := false
	for _, name := range instanceInfoClone.Status.ShareNames {
		if name == shareName {
			klog.V(5).Infof("Found share %q in instanceInfo %q, exclude it in updated object", shareName, instanceInfoClone.Name)
			updated = true
			continue
		}
		newShareNames = append(newShareNames, name)
	}
	if updated {
		instanceInfoClone.Status.ShareNames = newShareNames
	}
	return instanceInfoClone, updated
}

func (recon *MultishareReconciler) instanceInfoNewCapacity(instanceInfoClone *v1.InstanceInfo) (*v1.InstanceInfo, bool) {
	if instanceInfoClone.Status == nil || len(instanceInfoClone.Status.ShareNames) == 0 {
		return instanceInfoClone, false
	}
	// if we don't know what's the step size, use the min instance size as step size.
	stepSizeGb := util.DefaultStepSizeGb
	if instanceInfoClone.Status.CapacityStepSizeGb != 0 {
		stepSizeGb = instanceInfoClone.Status.CapacityStepSizeGb
	}
	var targetInstanceSizeByte int64 = 0
	for _, shareName := range instanceInfoClone.Status.ShareNames {
		shareInfo, err := recon.shareLister.ShareInfos(util.ManagedFilestoreCSINamespace).Get(shareName)
		if err != nil {
			klog.Warningf("Error getting ShareInfo %q: %v", shareName, err)
			continue
		}
		targetInstanceSizeByte += shareInfo.Spec.CapacityBytes
	}
	targetInstanceSizeByte = util.AlignBytes(targetInstanceSizeByte, util.GbToBytes(stepSizeGb))

	// bound InstanceSizeByte to max and min of Multishare instance size
	targetInstanceSizeByte = util.Max(targetInstanceSizeByte, util.MinMultishareInstanceSizeBytes)
	targetInstanceSizeByte = util.Min(targetInstanceSizeByte, util.MaxMultishareInstanceSizeBytes)

	if targetInstanceSizeByte == instanceInfoClone.Spec.CapacityBytes {
		return instanceInfoClone, false
	}
	klog.Infof("Updating instanceInfo %q capacity from %d to %d bytes in updated object", instanceInfoClone.Name, instanceInfoClone.Spec.CapacityBytes, targetInstanceSizeByte)
	instanceInfoClone.Spec.CapacityBytes = targetInstanceSizeByte
	return instanceInfoClone, true
}

// If instanceInfo has been deleted (deletionTimestamp is not nil), and only have 1 finalizer, this method removes that finalizer.
// The object will then be automatically cleaned up by the API server
func (recon *MultishareReconciler) maybeRemoveInstanceInfoFinalizer(instanceInfo *v1.InstanceInfo) (*v1.InstanceInfo, error) {
	instanceInfoClone := instanceInfo.DeepCopy()
	if instanceInfo.DeletionTimestamp == nil {
		klog.V(6).Infof("InstanceInfo %q doesn't have deletion timestamp, it shouldn't be deleted", instanceInfo.Name)
		if instanceInfo.Status != nil && instanceInfo.Status.InstanceStatus != "" {
			klog.Errorf("instance %s does not exist but InstanceStatus is %s", instanceInfo.Name, instanceInfo.Status.InstanceStatus)
		}
		return instanceInfo, nil
	}
	if instanceInfo.Status == nil {
		klog.Warningf("InstanceInfo %q marked to be deleted but Status is nil", instanceInfo.Name)
		return instanceInfo, nil
	}

	// InstanceInfo should not have other finalizers.
	if len(instanceInfoClone.Finalizers) != 1 {
		err := fmt.Errorf("InstanceInfo %q does not have exactly 1 Finalizer as expected, got %v", instanceInfo.Name, instanceInfoClone.Finalizers)
		return instanceInfo, err
	}
	instanceInfoClone.Finalizers = make([]string, 0)

	klog.Infof("Trying to remove Finalizers on InstanceInfo %s", instanceInfo.Name)
	instanceInfoClone, err := recon.updateInstanceInfo(context.TODO(), instanceInfoClone)
	if err != nil {
		return instanceInfoClone, err
	}
	return nil, nil
}

func (recon *MultishareReconciler) maybeUpdateInstanceInfoStatus(instance *file.MultishareInstance, instanceInfo *v1.InstanceInfo, instanceShares map[string][]*file.Share) (*v1.InstanceInfo, error) {
	status, err := util.InstanceStateToCRDStatus(instance.State)
	if err != nil {
		return instanceInfo, err
	}

	// If there's actual share on instance that's not reflected in instanceInfo's status.ShareNames, add them.
	// Do not remove entry from status.ShareNames if corresponding share does not exist because they could yet being created.
	instanceUri, _ := file.GenerateMultishareInstanceURI(instance)
	shares := instanceShares[instanceUri]
	instanceInfoClone := instanceInfo.DeepCopy()
	shareNames := make(map[string]bool)
	if instanceInfoClone.Status != nil {
		for _, name := range instanceInfoClone.Status.ShareNames {
			shareNames[name] = true
		}
	}
	shareNamesUpdated := false
	for _, share := range shares {
		siName := util.ShareToShareInfoName(share.Name)
		if !shareNames[siName] {
			klog.V(4).Infof("Found share %q in instance %q bu not in instanceInfo, adding %q to instanceInfo", share.Name, instance.Name, siName)
			shareNames[siName] = true
			shareNamesUpdated = true
		}
	}

	if instanceInfo.Status != nil &&
		instance.CapacityBytes == instanceInfo.Status.CapacityBytes &&
		status == instanceInfo.Status.InstanceStatus &&
		instance.CapacityStepSizeGb == instanceInfo.Status.CapacityStepSizeGb &&
		!shareNamesUpdated {
		return instanceInfo, nil
	}

	shareNameList := make([]string, 0, len(shareNames))
	for name := range shareNames {
		shareNameList = append(shareNameList, name)
	}
	newStatus := &v1.InstanceInfoStatus{
		CapacityBytes:      instance.CapacityBytes,
		InstanceStatus:     status,
		ShareNames:         shareNameList,
		CapacityStepSizeGb: instance.CapacityStepSizeGb,
		Cidr:               instance.Network.ReservedIpRange,
	}
	if instanceInfoClone.Status != nil {
		newStatus.Error = instanceInfoClone.Status.Error
	}
	instanceInfoClone.Status = newStatus
	klog.Infof("Trying to update InstanceInfo %s Status to %v", instanceInfo.Name, instanceInfoClone.Status)

	return recon.updateInstanceInfoStatus(context.TODO(), instanceInfoClone)
}

// maybeUpdateShareInfoStatus updates shareInfo.Status, if needed, to match with share's status.
func (recon *MultishareReconciler) maybeUpdateShareInfoStatus(share *file.Share, shareInfo *v1.ShareInfo) (*v1.ShareInfo, error) {
	status, err := util.ShareStateToCRDStatus(share.State)
	if err != nil {
		return nil, err
	}
	shareInfoClone := shareInfo.DeepCopy()
	instanceHandle, err := file.GenerateMultishareInstanceURI(share.Parent)
	if err != nil {
		return shareInfo, fmt.Errorf("Error generating instanceHandle from share %q: %v", share.Name, err)
	}

	if shareInfo.Status != nil &&
		share.CapacityBytes == shareInfo.Status.CapacityBytes &&
		status == shareInfo.Status.ShareStatus &&
		instanceHandle == shareInfo.Status.InstanceHandle {
		return shareInfo, nil
	}

	newStatus := &v1.ShareInfoStatus{
		CapacityBytes:  share.CapacityBytes,
		ShareStatus:    status,
		InstanceHandle: instanceHandle,
	}
	if shareInfoClone.Status != nil {
		newStatus.Error = shareInfoClone.Status.Error
	}
	shareInfoClone.Status = newStatus
	klog.Infof("Trying to update ShareInfo %q status to %v", shareInfo.Name, shareInfoClone.Status)

	return recon.updateShareInfoStatus(context.TODO(), shareInfoClone)
}

func (recon *MultishareReconciler) createInstanceInfo(ctx context.Context, instanceInfo *v1.InstanceInfo) (*v1.InstanceInfo, error) {
	klog.Infof("Trying to create instanceInfo %s", instanceInfo.Name)
	result, err := recon.clientset.MultishareV1().InstanceInfos(util.ManagedFilestoreCSINamespace).Create(context.TODO(), instanceInfo, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (recon *MultishareReconciler) deleteInstanceInfo(ctx context.Context, instanceInfo *v1.InstanceInfo) error {
	if len(instanceInfo.Finalizers) == 0 {
		return fmt.Errorf("Need to have finalizer to prevent auto gc of instanceInfo object")
	}
	klog.Infof("Trying to add deletionTimestamp to instanceInfo %s", instanceInfo.Name)
	return recon.clientset.MultishareV1().InstanceInfos(util.ManagedFilestoreCSINamespace).Delete(context.TODO(), instanceInfo.Name, metav1.DeleteOptions{})
}

func (recon *MultishareReconciler) updateShareInfoStatus(ctx context.Context, shareInfoClone *v1.ShareInfo) (*v1.ShareInfo, error) {
	result, err := recon.clientset.MultishareV1().ShareInfos(util.ManagedFilestoreCSINamespace).UpdateStatus(ctx, shareInfoClone, metav1.UpdateOptions{})
	if err != nil {
		return result, err
	}
	return result, nil
}

func (recon *MultishareReconciler) updateInstanceInfoStatus(ctx context.Context, instanceInfoClone *v1.InstanceInfo) (*v1.InstanceInfo, error) {
	result, err := recon.clientset.MultishareV1().InstanceInfos(util.ManagedFilestoreCSINamespace).UpdateStatus(ctx, instanceInfoClone, metav1.UpdateOptions{})
	if err != nil {
		return result, err
	}
	return result, nil
}

func (recon *MultishareReconciler) updateInstanceInfo(ctx context.Context, instanceInfoClone *v1.InstanceInfo) (*v1.InstanceInfo, error) {
	result, err := recon.clientset.MultishareV1().InstanceInfos(util.ManagedFilestoreCSINamespace).Update(ctx, instanceInfoClone, metav1.UpdateOptions{})
	if err != nil {
		return result, err
	}
	return result, nil
}

// managedInstanceAndShare filters out instances and shares that are not managed by current cluster.
// The returned values should be treated as read only.
func (recon *MultishareReconciler) managedInstanceAndShare(instances []*file.MultishareInstance, shares []*file.Share) ([]*file.MultishareInstance, []*file.Share, map[string][]*file.Share, error) {
	var managedInstances []*file.MultishareInstance
	var managedShares []*file.Share

	instanceShare := make(map[string][]*file.Share)
	clusterLocation := recon.cloud.Zone
	if recon.config.IsRegional {
		var err error
		clusterLocation, err = util.GetRegionFromZone(clusterLocation)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to get region for regional cluster: %v", err.Error())
		}
	}

	for _, instance := range instances {
		klog.V(6).Infof("Processing instance %v", instance)
		location, ok := instance.Labels[TagKeyClusterLocation]
		if !ok {
			klog.Infof("Label %q missing in target instance %q", TagKeyClusterLocation, instance.Name)
			continue
		}
		clusterName, ok := instance.Labels[TagKeyClusterName]
		if !ok {
			klog.Infof("Label %q missing in target instance %q", TagKeyClusterName, instance.Name)
			continue
		}
		// check for storage class tag on instance
		_, ok = instance.Labels[util.ParamMultishareInstanceScLabelKey]
		if !ok {
			klog.Infof("Label %q missing in target instance %q", util.ParamMultishareInstanceScLabelKey, instance.Name)
			continue
		}
		if clusterLocation == location && clusterName == recon.config.ClusterName {
			managedInstances = append(managedInstances, instance)
			instanceURI, _ := file.GenerateMultishareInstanceURI(instance)
			instanceShare[instanceURI] = make([]*file.Share, 0)
		}
	}

	for _, share := range shares {
		parentURI, err := file.GenerateMultishareInstanceURI(share.Parent)
		if err != nil {
			klog.Errorf("Share %s does not have fully specified parent instance", share.Name)
			continue
		}
		if _, ok := instanceShare[parentURI]; ok {
			managedShares = append(managedShares, share)
			instanceShare[parentURI] = append(instanceShare[parentURI], share)
		}
	}

	return managedInstances, managedShares, instanceShare, nil
}

// listMultishareOps reports all running or error ops related to multishare instances and share resources. The op target is of the form "projects/<>/locations/<>/instances/<>" or "projects/<>/locations/<>/instances/<>/shares/<>".
func (recon *MultishareReconciler) listMultishareResourceOps(ctx context.Context) ([]*Op, error) {
	ops, err := recon.cloud.File.ListOps(ctx, &file.ListFilter{Project: recon.cloud.Project, Location: "-"})
	if err != nil {
		return nil, err
	}

	var finalops []*Op
	for _, op := range ops {
		if op.Done && op.Error == nil {
			continue
		}

		if op.Metadata == nil {
			continue
		}

		var meta filev1beta1.OperationMetadata
		if err := json.Unmarshal(op.Metadata, &meta); err != nil {
			klog.Errorf("Failed to parse metadata for op %s", op.Name)
			continue
		}

		klog.V(6).Infof("creation time: %s", meta.CreateTime)
		var err error
		if op.Done && op.Error != nil {
			// filter out error Op that's more than util.ErrRetention old
			var createTime time.Time
			createTime, err = time.Parse(time.RFC3339Nano, meta.CreateTime)
			if err != nil {
				klog.Errorf("failed to parse creation Time %q with error: %s", meta.CreateTime, err.Error())
			} else if createTime.Before(time.Now().Add(-util.ErrRetention)) {
				continue
			}
			err = status.Error(codes.Code(op.Error.Code), op.Error.Message)
		}

		if file.IsInstanceTarget(meta.Target) {
			finalops = append(finalops, &Op{Id: op.Name, Target: meta.Target, Type: util.ConvertInstanceOpVerbToType(meta.Verb), Err: err})
		} else if file.IsShareTarget(meta.Target) {
			finalops = append(finalops, &Op{Id: op.Name, Target: meta.Target, Type: util.ConvertShareOpVerbToType(meta.Verb), Err: err})
		}
		// TODO: Add other resource types if needed, when we support snapshot/backups.
	}
	return finalops, nil
}

// instanceFitShare returns true if shareInfo can be assigned to instanceInfo.
func (recon *MultishareReconciler) instanceFitShare(instanceInfo *v1.InstanceInfo, shareInfo *v1.ShareInfo) bool {
	// Instance needs to be:
	// 1. not up for delete 2.of the same storage class and 3. has less than max number of shares assigned already.
	if instanceInfo.DeletionTimestamp != nil ||
		instanceInfo.Labels[ParamMultishareInstanceScLabel] != shareInfo.Spec.InstancePoolTag {
		return false
	}

	maxSharePerInstance := recon.parseMaxSharePerInstance(instanceInfo.Spec.Parameters)

	if instanceInfo.Status != nil && len(instanceInfo.Status.ShareNames) >= maxSharePerInstance {
		return false
	}

	if instanceInfo.Status != nil && instanceInfo.Status.InstanceStatus == v1.UPDATING {
		// If the instance status is UPDATING, it means it's not in a ready state and may be unhealthy or being deleted.
		// Do not assign share to that instance.
		return false
	}

	return true
}

// parseMaxSharePerInstance assumes that params has valid parameter of max volume size and returns max share per instance
func (recon *MultishareReconciler) parseMaxSharePerInstance(params map[string]string) int {
	maxSharePerInstance, _, _ := recon.controllerServer.config.multiShareController.parseMaxVolumeSizeParam(params)
	if maxSharePerInstance == 0 {
		maxSharePerInstance = util.MaxSharesPerInstance
	}
	return maxSharePerInstance
}

// instanceEmpty returns true if instanceInfo.Status.ShareNames has zero entries.
func instanceEmpty(instanceInfo *v1.InstanceInfo) bool {
	if instanceInfo.Status == nil || len(instanceInfo.Status.ShareNames) != 0 {
		return false
	}
	return true
}

func runningOpMaybeErrForTarget(targetURI string, ops []*Op) (*Op, error) {
	var runningOp *Op
	var err error
	// Check for instance prefix in op target.
	for _, op := range ops {
		if runningOp != nil {
			break
		}
		if op.Target == targetURI {
			if op.Err != nil {
				klog.Infof("found err Op for target %s with err %s", targetURI, op.Err.Error())
				err = op.Err
			} else {
				klog.Infof("found running Op for target %s", targetURI)
				runningOp = op
			}
		}
	}
	return runningOp, err
}

// returns true if shareInfo.Status is not nil and the actual share exists in assigned instance
func shareExist(shareInfo *v1.ShareInfo, instanceShares map[string][]*file.Share) bool {
	if shareInfo.Status == nil {
		return false
	}
	for _, share := range instanceShares[shareInfo.Status.InstanceHandle] {
		if share.Name == shareInfo.Spec.ShareName {
			return true
		}
	}
	klog.Infof("share %s does not exist in instance %s from list share", shareInfo.Name, shareInfo.Status.InstanceHandle)
	return false
}
