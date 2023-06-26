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
	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	v1 "sigs.k8s.io/gcp-filestore-csi-driver/pkg/apis/multishare/v1"
	clientset "sigs.k8s.io/gcp-filestore-csi-driver/pkg/client/clientset/versioned"
	listers "sigs.k8s.io/gcp-filestore-csi-driver/pkg/client/listers/multishare/v1"
	cloud "sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/file"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/util"
)

// MultishareController handles CSI calls for volumes which use Filestore multishare instances.
type MultishareStatefulController struct {
	//TODO: support variable share count per Filestore instance feature.
	driver *GCFSDriver
	zone   string
	cloud  *cloud.Cloud
	mc     *MultishareController

	clientset   clientset.Interface
	shareLister listers.ShareInfoLister
}

func NewMultishareStatefulController(config *controllerServerConfig) *MultishareStatefulController {
	return &MultishareStatefulController{
		driver:      config.driver,
		zone:        config.cloud.Zone,
		cloud:       config.cloud,
		clientset:   config.features.FeatureStateful.DriverClientSet,
		shareLister: config.features.FeatureStateful.ShareLister,
	}
}

func (m *MultishareStatefulController) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	klog.Infof("CreateVolume called for multishare with request %+v", req)
	pvName := req.GetName()
	if len(pvName) == 0 {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume name must be provided")
	}
	if err := m.driver.validateVolumeCapabilities(req.GetVolumeCapabilities()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if req.GetVolumeContentSource() != nil {
		return nil, status.Error(codes.InvalidArgument, "Multishare backed volumes do not support volume content source")
	}

	instanceSCLabel, err := getInstanceSCLabel(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	_, maxShareSizeBytes, err := m.mc.parseMaxVolumeSizeParam(req.GetParameters())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	var reqBytes int64
	if m.mc.featureMaxSharePerInstance {
		reqBytes, err = getShareRequestCapacity(req.GetCapacityRange(), util.ConfigurablePackMinShareSizeBytes, maxShareSizeBytes)
	} else {
		reqBytes, err = getShareRequestCapacity(req.GetCapacityRange(), util.MinShareSizeBytes, util.MaxShareSizeBytes)
	}

	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if !util.IsAligned(reqBytes, util.Gb) {
		return nil, status.Errorf(codes.InvalidArgument, "requested size(bytes) %d is not a multiple of 1GiB", reqBytes)
	}

	shareInfo, err := m.shareLister.ShareInfos(util.ManagedFilestoreCSINamespace).Get(pvName)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, status.Errorf(codes.Internal, "error getting shareInfo %q from informer: %s", pvName, err.Error())
		}
		klog.Infof("querying ShareInfo %q from api server", pvName)
		shareInfo, err = m.clientset.MultishareV1().ShareInfos(util.ManagedFilestoreCSINamespace).Get(context.TODO(), pvName, metav1.GetOptions{})
		if err != nil {
			if !errors.IsNotFound(err) {
				return nil, status.Errorf(codes.Internal, "error getting shareInfo %q from api server: %s", pvName, err.Error())
			}
			klog.V(6).Infof("shareInfo object for share %q not found in API server", pvName)
			shareInfo = nil
		} else {
			klog.Infof("shareInfo object for share %q not found in informer cache but found in api server", pvName)
		}
	}

	if shareInfo == nil {
		region, err := m.mc.pickRegion(req.GetAccessibilityRequirements())
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		shareInfo = &v1.ShareInfo{
			ObjectMeta: metav1.ObjectMeta{
				Name:       pvName,
				Finalizers: []string{util.FilestoreResourceCleanupFinalizer},
				Labels:     extractShareLabels(req.Parameters),
			},
			Spec: v1.ShareInfoSpec{
				ShareName:       util.ConvertVolToShareName(pvName),
				CapacityBytes:   reqBytes,
				InstancePoolTag: instanceSCLabel,
				Region:          region,
				Parameters:      req.GetParameters(),
			},
		}
		klog.V(6).Infof("trying to create shareInfo object: %v", shareInfo)
		shareInfo, err = m.createShareInfo(ctx, shareInfo)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "error creating share object: %s", err.Error())
		}
	}

	if shareInfo.Status == nil || shareInfo.Status.InstanceHandle == "" {
		return nil, status.Errorf(codes.Aborted, "share %s is not assigned to an instance yet", pvName)
	}

	if shareInfo.Status.ShareStatus != v1.READY {
		if shareInfo.Status.Error != "" {
			return nil, status.Errorf(codes.Internal, "internal error: %s", shareInfo.Status.Error)
		}
		return nil, status.Errorf(codes.Aborted, "share %s is not ready yet", pvName)
	}

	share, err := generateFileShareFromShareInfo(shareInfo)
	if err != nil {
		return nil, err
	}
	return m.mc.getShareAndGenerateCSICreateVolumeResponse(ctx, instanceSCLabel, share, maxShareSizeBytes)
}

func (m *MultishareStatefulController) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	_, project, location, instanceName, shareName, err := parseMultishareVolId(req.VolumeId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	klog.V(4).Infof("DeleteVolume called for multishare with request %+v", req)

	siName := util.ShareToShareInfoName(shareName)
	shareInfo, err := m.shareLister.ShareInfos(util.ManagedFilestoreCSINamespace).Get(siName)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, status.Errorf(codes.Internal, "error deleting volume %q due to informer error: %s", req.VolumeId, err.Error())
		}
		// check with api if share exist
		klog.V(6).Infof("shareInfo %s does not exist in cache, checking if share is already deleted", siName)
		_, err := m.cloud.File.GetShare(ctx, &file.Share{
			Parent: &file.MultishareInstance{
				Project:  project,
				Location: location,
				Name:     instanceName,
			},
			Name: shareName,
		})
		if err != nil {
			if file.IsNotFoundErr(err) {
				return &csi.DeleteVolumeResponse{}, nil
			}

			return nil, status.Error(codes.Internal, err.Error())
		}
		return nil, status.Errorf(codes.Aborted, "waiting to express intent for volume %s to be deleted", req.VolumeId)
	}

	if shareInfo.DeletionTimestamp == nil {
		if len(shareInfo.Finalizers) == 0 {
			klog.Errorf("shareInfo %s shouldn't have no finalizer before deletion marking", siName)
			return nil, status.Errorf(codes.Internal, "error deleting volume %s due to driver state error", req.VolumeId)
		}

		err := m.deleteShareInfo(ctx, siName)
		if err != nil {
			klog.Errorf("error marking the shareInfo object as deleted: %s", err.Error())
		}
		return nil, status.Errorf(codes.Aborted, "expressed intent for volume %s to be deleted, waiting.", req.VolumeId)
	}

	if shareInfo.Status == nil {
		klog.Errorf("shareInfo %s marked to be deleted but shareInfo.Status == nil", siName)
		return nil, status.Errorf(codes.Aborted, "waiting for volume %s to be deleted.", req.VolumeId)
	}

	if shareInfo.Status.ShareStatus == v1.DELETED {
		// remove finalizer and return success
		klog.V(6).Infof("trying to remove finalizer from %s because share deleted", siName)
		shareInfoClone := shareInfo.DeepCopy()
		shareInfoClone.Finalizers = []string{}
		shareInfo, err = m.updateShareInfo(ctx, shareInfoClone)
		if err != nil {
			klog.Errorf("failed to remove finalizer from %s: %s", siName, err.Error())
			return nil, status.Errorf(codes.Internal, "error deleting volume %s due to failed internal update", req.VolumeId)
		}
		return &csi.DeleteVolumeResponse{}, nil
	}

	if shareInfo.Status.Error != "" {
		return nil, status.Errorf(codes.Internal, "internal error: %s", shareInfo.Status.Error)
	}

	return nil, status.Errorf(codes.Aborted, "waiting for the Filestore share supporting volume %s to be deleted", req.VolumeId)
}

func (m *MultishareStatefulController) updateShareInfo(ctx context.Context, shareInfoClone *v1.ShareInfo) (*v1.ShareInfo, error) {
	result, err := m.clientset.MultishareV1().ShareInfos(util.ManagedFilestoreCSINamespace).Update(ctx, shareInfoClone, metav1.UpdateOptions{})
	if err != nil {
		return result, err
	}
	return result, nil
}

func (m *MultishareStatefulController) createShareInfo(ctx context.Context, shareInfo *v1.ShareInfo) (*v1.ShareInfo, error) {
	result, err := m.clientset.MultishareV1().ShareInfos(util.ManagedFilestoreCSINamespace).Create(ctx, shareInfo, metav1.CreateOptions{})
	if err != nil {
		return result, err
	}
	return result, nil
}

func (m *MultishareStatefulController) deleteShareInfo(ctx context.Context, siName string) error {
	return m.clientset.MultishareV1().ShareInfos(util.ManagedFilestoreCSINamespace).Delete(ctx, siName, metav1.DeleteOptions{})
}

func (m *MultishareStatefulController) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	volumeId := req.GetVolumeId()
	if len(volumeId) == 0 {
		return nil, status.Error(codes.InvalidArgument, "ControllerExpandVolume volume ID must be provided")
	}

	maxShareSizeBytes := util.MaxShareSizeBytes
	if m.mc.featureMaxSharePerInstance {
		var err error
		maxShareSizeBytes, err = m.mc.GetShareMaxSizeFromPV(ctx, volumeId)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
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
	_, _, _, _, shareName, err := parseMultishareVolId(req.VolumeId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	klog.Infof("ControllerExpandVolume called for multishare with request %+v", req)

	siName := util.ShareToShareInfoName(shareName)
	shareInfo, err := m.shareLister.ShareInfos(util.ManagedFilestoreCSINamespace).Get(siName)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, status.Errorf(codes.Internal, "error getting shareInfo %q from informer: %s", siName, err.Error())
		}
		klog.Infof("shareInfo %s does not exist in cache", siName)
		return nil, status.Errorf(codes.Aborted, "waiting to express intent for volume %s to be expanded", siName)
	}

	if shareInfo.Spec.CapacityBytes < reqBytes {
		// update Spec.CapacityBytes
		shareInfoClone := shareInfo.DeepCopy()
		shareInfoClone.Spec.CapacityBytes = reqBytes
		shareInfo, err = m.updateShareInfo(ctx, shareInfoClone)
		if err != nil {
			klog.Errorf("failed to update shareInfo %s: %s", siName, err.Error())
			return nil, status.Errorf(codes.Internal, "error expanding volume %s due to failed internal update", siName)
		}
		return nil, status.Errorf(codes.Aborted, "expressed intent for volume %s to be expanded", siName)
	}

	if shareInfo.Status == nil {
		klog.Errorf("ControllerExpandVolume called for %s but shareInfo.Status is nil", siName)
		return nil, status.Errorf(codes.Internal, "volume %s is not yet created", siName)
	}

	if shareInfo.Status.CapacityBytes >= reqBytes && shareInfo.Status.ShareStatus == v1.READY {
		klog.Infof("Controller expand volume succeeded for volume %v, size(bytes): %v", volumeId, shareInfo.Status.CapacityBytes)

		share, err := generateFileShareFromShareInfo(shareInfo)
		if err != nil {
			return nil, err
		}
		return m.mc.getShareAndGenerateCSIControllerExpandVolumeResponse(ctx, share, reqBytes)
	}

	if shareInfo.Status.Error != "" {
		return nil, status.Errorf(codes.Internal, "internal error: %s", shareInfo.Status.Error)
	}

	return nil, status.Errorf(codes.Aborted, "waiting for volume %s to be expanded", siName)
}

func generateFileShareFromShareInfo(shareInfo *v1.ShareInfo) (*file.Share, error) {
	instanceUri := shareInfo.Status.InstanceHandle
	project, location, instanceName, err := util.ParseInstanceURI(instanceUri)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "couldn't parse instanceURI %q: %s", instanceUri, err.Error())
	}
	return &file.Share{
		Name: shareInfo.Spec.ShareName,
		Parent: &file.MultishareInstance{
			Project:  project,
			Location: location,
			Name:     instanceName,
		},
	}, nil
}
