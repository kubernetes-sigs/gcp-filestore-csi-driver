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

package multishare_reconciler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	filev1beta1 "google.golang.org/api/file/v1beta1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	storageListers "k8s.io/client-go/listers/storage/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/apis/multishare/v1alpha1"
	clientset "sigs.k8s.io/gcp-filestore-csi-driver/pkg/client/clientset/versioned"
	informers "sigs.k8s.io/gcp-filestore-csi-driver/pkg/client/informers/externalversions/multishare/v1alpha1"
	listers "sigs.k8s.io/gcp-filestore-csi-driver/pkg/client/listers/multishare/v1alpha1"
	cloud "sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/file"
	driver "sigs.k8s.io/gcp-filestore-csi-driver/pkg/csi_driver"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/util"
)

type Op struct {
	Id     string
	Type   util.OperationType
	Target string
	Err    error
}

type multishareReconciler struct {
	clientset clientset.Interface
	config    *driver.GCFSDriverConfig
	cloud     *cloud.Cloud

	shareLister       listers.ShareInfoLister
	shareListerSynced cache.InformerSynced

	instanceLister       listers.InstanceInfoLister
	instanceListerSynced cache.InformerSynced

	scLister storageListers.StorageClassLister
}

func NewMultishareReconciler(
	clientset clientset.Interface,
	config *driver.GCFSDriverConfig,
	shareInformer informers.ShareInfoInformer,
	instanceInformar informers.InstanceInfoInformer,
	scLister storageListers.StorageClassLister,
) *multishareReconciler {
	recon := &multishareReconciler{
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

func (recon *multishareReconciler) Run(stopCh <-chan struct{}) {
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

func (recon *multishareReconciler) reconcileWorker() {
	startTime := time.Now()

	// List out shares, instances managed by this driver.
	shares, err := recon.cloud.File.ListShares(context.TODO(), &file.ListFilter{Project: recon.cloud.Project, Location: "-", InstanceName: "-"})
	if err != nil {
		klog.Errorf("Reconciler Failed to list Shares: %v", err)
		return
	}
	instances, err := recon.cloud.File.ListMultishareInstances(context.TODO(), &file.ListFilter{Project: recon.cloud.Project, Location: "-"})
	if err != nil {
		klog.Errorf("Reconciler Failed to list Instances: %v", err)
		return
	}
	klog.V(5).Infof("Found %d shares and %d instances", len(shares), len(instances))
	instances, shares, instanceShares, err := recon.managedInstanceAndShare(instances, shares)
	if err != nil {
		klog.Errorf("Failed to filter out managed instance and shares: %s", err.Error())
		return
	}

	// Create shareInfo objects if does not exist, update shareInfo.Status based on listed out shares' status.
	shareInfoMap := recon.createAndUpdateShareInfos(shares)

	shareInfoList, err := recon.shareLister.List(labels.Everything())
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

	instanceInfoList, err := recon.instanceLister.List(labels.Everything())
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

	// Assign un-assigned shares to instances; update shareInfo and instanceInfo accordingly,
	// if there's inconsistency between share and instnace then share has source of truth.
	recon.assignSharesToInstances(shareInfoMap, instanceInfoMap)

	klog.Infof("Reconciliation round finished after %v", time.Since(startTime))
}

func (recon *multishareReconciler) assignSharesToInstances(shareInfos map[string]*v1alpha1.ShareInfo, instanceInfos map[string]*v1alpha1.InstanceInfo) {
	recon.fixTwoWayPointers(shareInfos, instanceInfos)

	recon.assignSharesToEligibleOrNewInstances(shareInfos, instanceInfos)

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
func (recon *multishareReconciler) fixTwoWayPointers(shareInfos map[string]*v1alpha1.ShareInfo, instanceInfos map[string]*v1alpha1.InstanceInfo) {
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
						actualAssigned, err = recon.generateInstanceInfo(shareInfo.Status.InstanceHandle, shareInfo.Spec.InstancePoolTag)
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
func (recon *multishareReconciler) assignSharesToEligibleOrNewInstances(shareInfos map[string]*v1alpha1.ShareInfo, instanceInfos map[string]*v1alpha1.InstanceInfo) {
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
				if instanceFitShare(instanceInfo, shareInfo) {
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

				instanceInfo, err := recon.generateInstanceInfo(instanceURI, shareInfo.Spec.InstancePoolTag)
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
func (recon *multishareReconciler) deleteOrResizeInstances(instanceInfos map[string]*v1alpha1.InstanceInfo) {
	for instanceURI, instanceInfo := range instanceInfos {
		if instanceInfo.DeletionTimestamp != nil {
			continue
		}

		instanceInfoClone := instanceInfo.DeepCopy()
		var updated bool
		if instanceEmpty(instanceInfo) {
			instanceInfoClone, updated = maybeAddCleanupFinalizer(instanceInfoClone)
			if updated {
				klog.Infof("InstanceInfo %q needs to be deleted, trying to add finalizer", instanceInfo.Name)
				instanceInfoClone, err := recon.updateInstanceInfo(context.TODO(), instanceInfoClone)
				if err != nil {
					klog.Errorf("Failed to update instanceInfo %q: %v", instanceInfo.Name, err)
					continue
				}
				err = recon.deleteInstanceInfo(context.TODO(), instanceInfoClone)
				if err != nil {
					klog.Errorf("Failed to add deletionTimestamp to instanceInfo %q: %v", instanceInfo.Name, err)
					continue
				}
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
func (recon *multishareReconciler) generateInstanceInfo(instanceURI string, scTag string) (*v1alpha1.InstanceInfo, error) {
	storageClass, err := recon.storageClassFromTag(scTag)
	if err != nil {
		return nil, err
	}
	newInstanceInfo := &v1alpha1.InstanceInfo{
		ObjectMeta: metav1.ObjectMeta{
			Name: util.InstanceURIToInstanceInfoName(instanceURI),
			Labels: map[string]string{
				driver.ParamMultishareInstanceScLabel: storageClass.Parameters[driver.ParamMultishareInstanceScLabel],
			},
		},
		Spec: v1alpha1.InstanceInfoSpec{
			CapacityBytes:    util.MinMultishareInstanceSizeBytes,
			StorageClassName: storageClass.Name,
		},
	}
	return recon.createInstanceInfo(context.TODO(), newInstanceInfo)
}

// storageClassFromTag finds and returns the first storageclass with a matching scTag.
func (recon *multishareReconciler) storageClassFromTag(scTag string) (*v1.StorageClass, error) {
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
		if sc.Parameters[driver.ParamMultishareInstanceScLabel] == scTag {
			return sc, nil
		}
	}
	return nil, fmt.Errorf("no storageclass match storageClassTag %q", scTag)
}

func (recon *multishareReconciler) assignShareToInstanceInfo(instanceInfo *v1alpha1.InstanceInfo, shareName string) (*v1alpha1.InstanceInfo, error) {
	instanceInfoClone := instanceInfo.DeepCopy()
	if instanceInfoClone.Status == nil {
		instanceInfoClone.Status = &v1alpha1.InstanceInfoStatus{}
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

func (recon *multishareReconciler) assignInstanceToShareInfo(shareInfo *v1alpha1.ShareInfo, instanceURI string) (*v1alpha1.ShareInfo, error) {
	shareInfoClone := shareInfo.DeepCopy()
	if shareInfoClone.Status == nil {
		shareInfoClone.Status = &v1alpha1.ShareInfoStatus{}
	}
	shareInfoClone.Status.InstanceHandle = instanceURI
	klog.Infof("Try to assign share %q to instance %q", shareInfo.Name, instanceURI)
	return recon.updateShareInfoStatus(context.TODO(), shareInfoClone)
}

// createAndUpdateInstanceInfos create instanceInfo objects if needed and updates their statuses to match with actual state of the world.
// InstanceInfo objects in the returned map must be treated as read only.
func (recon *multishareReconciler) createAndUpdateInstanceInfos(instances []*file.MultishareInstance, instanceShares map[string][]*file.Share) map[string]*v1alpha1.InstanceInfo {
	instanceInfoMap := make(map[string]*v1alpha1.InstanceInfo)

	for _, instance := range instances {
		instanceURI, err := file.GenerateMultishareInstanceURI(instance)
		if err != nil {
			klog.Errorf("Couldn't generate instanceURI: %v for instance %q", err, instance.Name)
			continue
		}
		iiName := util.InstanceURIToInstanceInfoName(instanceURI)

		instanceInfo, err := recon.instanceLister.Get(iiName)
		if err != nil {
			if !errors.IsNotFound(err) {
				klog.Errorf("Error getting instanceInfo %q from informer: %v", iiName, err)
				continue
			}
			instanceInfo, err = recon.clientset.MultishareV1alpha1().InstanceInfos().Get(context.TODO(), iiName, metav1.GetOptions{})
			if err != nil {
				if !errors.IsNotFound(err) {
					klog.Errorf("Error getting instanceInfo %q from api server: %v", iiName, err)
					continue
				}
				klog.V(4).Infof("InstanceInfo object for instance %q not found in API server", instance.Name)
				instanceInfo = nil
			}
			klog.V(4).Infof("InstanceInfo object for instance %q not found in informer cache but found in API server", instance.Name)
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
func (recon *multishareReconciler) createAndUpdateShareInfos(shares []*file.Share) map[string]*v1alpha1.ShareInfo {
	shareInfoMap := make(map[string]*v1alpha1.ShareInfo)

	// Create ShareInfo that are not reflected.
	for _, share := range shares {
		shareInfoName := util.ShareToShareInfoName(share.Name)
		shareInfo, err := recon.shareLister.Get(shareInfoName)
		if err != nil {
			if !errors.IsNotFound(err) {
				klog.Errorf("Error getting shareInfo %q from informer: %v", shareInfoName, err)
				continue
			}
			shareInfo, err = recon.clientset.MultishareV1alpha1().ShareInfos().Get(context.TODO(), shareInfoName, metav1.GetOptions{})
			if err != nil {
				if !errors.IsNotFound(err) {
					klog.Errorf("Error getting shareInfo %q from api server: %v", shareInfoName, err)
					continue
				}
				// shareInfo does not exist for share
				klog.V(4).Infof("ShareInfo object for share %q not found in API server", shareInfoName)
				shareInfo = nil
			}
			// shareInfo exist in api server but not cache
			klog.V(4).Infof("ShareInfo object for share %q not found in informer cache but found in api server", shareInfoName)
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

func (recon *multishareReconciler) reconstructInstanceInfo(iiName string, instance *file.MultishareInstance, instanceInfo *v1alpha1.InstanceInfo) (*v1alpha1.InstanceInfo, error) {

	instanceInfo = &v1alpha1.InstanceInfo{
		ObjectMeta: metav1.ObjectMeta{
			Name: iiName,
			Labels: map[string]string{
				driver.ParamMultishareInstanceScLabel: instance.Labels[util.ParamMultishareInstanceScLabelKey],
			},
		},
		Spec: v1alpha1.InstanceInfoSpec{
			CapacityBytes: instance.CapacityBytes,
		},
	}
	return recon.createInstanceInfo(context.TODO(), instanceInfo)
}

func (recon *multishareReconciler) createShareInfo(share *file.Share, shareInfo *v1alpha1.ShareInfo) (*v1alpha1.ShareInfo, error) {
	shareInfo = &v1alpha1.ShareInfo{
		ObjectMeta: metav1.ObjectMeta{
			Name: util.ShareToShareInfoName(share.Name),
		},
		Spec: v1alpha1.ShareInfoSpec{
			ShareName:     share.Name,
			CapacityBytes: share.CapacityBytes,
			Region:        share.Parent.Location,
		},
	}
	klog.Infof("Trying to create ShareInfo %s", shareInfo.Name)
	result, err := recon.clientset.MultishareV1alpha1().ShareInfos().Create(context.TODO(), shareInfo, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// maybeUpdateShareInfoDeleted update the shareInfo object as status "DELETED" if it has deletion timestamp set.
func (recon *multishareReconciler) maybeUpdateShareInfoDeleted(shareInfo *v1alpha1.ShareInfo) (*v1alpha1.ShareInfo, error) {
	if shareInfo.DeletionTimestamp == nil {
		klog.V(6).Infof("ShareInfo %q doesn't have deletion timestamp, its status shouldn't be marked as deleted", shareInfo.Name)
		return shareInfo, nil
	}
	shareInfoClone := shareInfo.DeepCopy()
	if shareInfoClone.Status == nil {
		// if the status is nil, and later the updateStatus succeed, it means that the shareInfo.Status was never populated.
		// delete should not have happened before shareInfo.Status.ShareStatus show READY
		return shareInfo, fmt.Errorf("ShareInfo %q marked to be deleted but Status is nil", shareInfoClone.Name)
	}
	shareInfoClone.Status.ShareStatus = v1alpha1.DELETED
	klog.Infof("Trying to mark ShareInfo %s as DELETED", shareInfo.Name)

	return recon.updateShareInfoStatus(context.TODO(), shareInfoClone)
}

// removeShareFromInstanceInfo removes share assignment from instanceInfo object in place but does not re-calculate required instance Size.
func (recon *multishareReconciler) removeShareFromInstanceInfo(instanceInfoClone *v1alpha1.InstanceInfo, shareName string) (*v1alpha1.InstanceInfo, bool) {
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

func (recon *multishareReconciler) instanceInfoNewCapacity(instanceInfoClone *v1alpha1.InstanceInfo) (*v1alpha1.InstanceInfo, bool) {
	if instanceInfoClone.Status == nil || len(instanceInfoClone.Status.ShareNames) == 0 {
		return instanceInfoClone, false
	}
	// if we don't know what's the step size, use the min instance size as step size.
	stepSizeGb := util.MinMultishareInstanceSizeBytes
	if instanceInfoClone.Status.CapacityStepSizeGb != 0 {
		stepSizeGb = instanceInfoClone.Status.CapacityStepSizeGb
	}
	var targetInstanceSizeByte int64 = 0
	for _, shareName := range instanceInfoClone.Status.ShareNames {
		shareInfo, err := recon.shareLister.Get(shareName)
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
func (recon *multishareReconciler) maybeRemoveInstanceInfoFinalizer(instanceInfo *v1alpha1.InstanceInfo) (*v1alpha1.InstanceInfo, error) {
	if instanceInfo.DeletionTimestamp == nil {
		klog.V(6).Infof("InstanceInfo %q doesn't have deletion timestamp, it shouldn't be deleted", instanceInfo.Name)
		return instanceInfo, nil
	}
	if instanceInfo.Status == nil {
		klog.Warningf("InstanceInfo %q marked to be deleted but Status is nil", instanceInfo.Name)
		return instanceInfo, nil
	}

	instanceInfoClone := instanceInfo.DeepCopy()
	// instanceInfo should not have other finalizers.
	if len(instanceInfoClone.Finalizers) != 1 {
		err := fmt.Errorf("InstanceInfo %q does not have exactly 1 Finalizer as expected, got %v", instanceInfo.Name, instanceInfoClone.Finalizers)
		return instanceInfo, err
	}
	instanceInfoClone.Finalizers = make([]string, 0)

	klog.Infof("Trying to remove Finalizers on InstanceInfo %s", instanceInfo.Name)
	instanceInfo, err := recon.clientset.MultishareV1alpha1().InstanceInfos().Update(context.TODO(), instanceInfoClone, metav1.UpdateOptions{})
	if err != nil {
		return instanceInfo, err
	}
	return nil, nil
}

func (recon *multishareReconciler) maybeUpdateInstanceInfoStatus(instance *file.MultishareInstance, instanceInfo *v1alpha1.InstanceInfo, instanceShares map[string][]*file.Share) (*v1alpha1.InstanceInfo, error) {
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
	newStatus := &v1alpha1.InstanceInfoStatus{
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

func (recon *multishareReconciler) maybeUpdateShareInfoStatus(share *file.Share, shareInfo *v1alpha1.ShareInfo) (*v1alpha1.ShareInfo, error) {
	status, err := util.ShareStateToCRDStatus(share.State)
	if err != nil {
		return nil, err
	}
	if shareInfo.Status != nil && share.CapacityBytes == shareInfo.Status.CapacityBytes && status == shareInfo.Status.ShareStatus {
		return shareInfo, nil
	}
	shareInfoClone := shareInfo.DeepCopy()
	instanceHandle, err := file.GenerateMultishareInstanceURI(share.Parent)
	if err != nil {
		return shareInfo, fmt.Errorf("Error generating instanceHandle from share %q: %v", share.Name, err)
	}
	newStatus := &v1alpha1.ShareInfoStatus{
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

func (recon *multishareReconciler) createInstanceInfo(ctx context.Context, instanceInfo *v1alpha1.InstanceInfo) (*v1alpha1.InstanceInfo, error) {
	klog.Infof("Trying to create instanceInfo %s", instanceInfo.Name)
	result, err := recon.clientset.MultishareV1alpha1().InstanceInfos().Create(context.TODO(), instanceInfo, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (recon *multishareReconciler) deleteInstanceInfo(ctx context.Context, instanceInfo *v1alpha1.InstanceInfo) error {
	if len(instanceInfo.Finalizers) == 0 {
		return fmt.Errorf("Need to have finalizer to prevent auto gc of instanceInfo object")
	}
	klog.Infof("Trying to add deletionTimestamp to instanceInfo %s", instanceInfo.Name)
	return recon.clientset.MultishareV1alpha1().InstanceInfos().Delete(context.TODO(), instanceInfo.Name, metav1.DeleteOptions{})
}

func (recon *multishareReconciler) updateShareInfoStatus(ctx context.Context, shareInfoClone *v1alpha1.ShareInfo) (*v1alpha1.ShareInfo, error) {
	result, err := recon.clientset.MultishareV1alpha1().ShareInfos().UpdateStatus(ctx, shareInfoClone, metav1.UpdateOptions{})
	if err != nil {
		return result, err
	}
	return result, nil
}

func (recon *multishareReconciler) updateInstanceInfoStatus(ctx context.Context, instanceInfoClone *v1alpha1.InstanceInfo) (*v1alpha1.InstanceInfo, error) {
	result, err := recon.clientset.MultishareV1alpha1().InstanceInfos().UpdateStatus(ctx, instanceInfoClone, metav1.UpdateOptions{})
	if err != nil {
		return result, err
	}
	return result, nil
}

func (recon *multishareReconciler) updateInstanceInfo(ctx context.Context, instanceInfoClone *v1alpha1.InstanceInfo) (*v1alpha1.InstanceInfo, error) {
	result, err := recon.clientset.MultishareV1alpha1().InstanceInfos().Update(ctx, instanceInfoClone, metav1.UpdateOptions{})
	if err != nil {
		return result, err
	}
	return result, nil
}

// managedInstanceAndShare filters out instances and shares that are not managed by current cluster.
// The returned values should be treated as read only.
func (recon *multishareReconciler) managedInstanceAndShare(instances []*file.MultishareInstance, shares []*file.Share) ([]*file.MultishareInstance, []*file.Share, map[string][]*file.Share, error) {
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
		location, ok := instance.Labels[driver.TagKeyClusterLocation]
		if !ok {
			klog.Infof("Label %q missing in target instance %q", driver.TagKeyClusterLocation, instance.Name)
			continue
		}
		clusterName, ok := instance.Labels[driver.TagKeyClusterName]
		if !ok {
			klog.Infof("Label %q missing in target instance %q", driver.TagKeyClusterName, instance.Name)
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
func (recon *multishareReconciler) listMultishareResourceOps(ctx context.Context) ([]*Op, error) {
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

		var err error
		if op.Error != nil {
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

func instanceFitShare(instanceInfo *v1alpha1.InstanceInfo, shareInfo *v1alpha1.ShareInfo) bool {
	// Instance needs to be:
	// 1. not up for delete 2.of the same storage class and 3. has less than max number of shares assigned already.
	if instanceInfo.DeletionTimestamp != nil ||
		instanceInfo.Labels[driver.ParamMultishareInstanceScLabel] != shareInfo.Spec.InstancePoolTag {
		return false
	}

	if instanceInfo.Status != nil && len(instanceInfo.Status.ShareNames) >= util.MaxSharesPerInstance {
		return false
	}

	return true
}

// instanceEmpty returns true if instanceInfo.Status.ShareNames has zero entries.
func instanceEmpty(instanceInfo *v1alpha1.InstanceInfo) bool {
	if instanceInfo.Status == nil || len(instanceInfo.Status.ShareNames) != 0 {
		return false
	}
	return true
}

func maybeAddCleanupFinalizer(instanceInfoClone *v1alpha1.InstanceInfo) (*v1alpha1.InstanceInfo, bool) {
	if instanceInfoClone.DeletionTimestamp != nil {
		klog.Infof("InstanceInfo %q has deletionTimestamp so must have already been processed")
		return instanceInfoClone, false
	}
	if len(instanceInfoClone.Finalizers) != 0 {
		klog.Errorf("InstanceInfo %q should not have any finalizer when it's DeletionTimestamp is not set but got %v. Attempting to remove all of them", instanceInfoClone.Name, instanceInfoClone.Finalizers)
	}
	instanceInfoClone.Finalizers = []string{util.FilestoreResourceCleanupFinalizer}
	return instanceInfoClone, true
}
