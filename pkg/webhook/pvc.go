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

package webhook

import (
	"context"
	"fmt"
	"strings"

	v1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

var (
	// StorageClassV1GVR is GroupVersionResource for v1 StorageClass
	PVCV1GVR       = metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "persistentvolumeclaims"}
	Gi       int64 = 1024 * 1024 * 1024
	Ti       int64 = 1024 * Gi
)

func admitPVC(ar v1.AdmissionReview) *v1.AdmissionResponse {
	klog.Info("admit PVC called")
	reviewResponse := &v1.AdmissionResponse{
		Allowed: true,
		Result:  &metav1.Status{},
	}

	if !(ar.Request.Operation == v1.Create || ar.Request.Operation == v1.Update) {
		return reviewResponse
	}

	raw := ar.Request.Object.Raw
	deserializer := codecs.UniversalDeserializer()
	switch ar.Request.Resource {
	case PVCV1GVR:
		pvc := &corev1.PersistentVolumeClaim{}
		if _, _, err := deserializer.Decode(raw, nil, pvc); err != nil {
			klog.Error(err)
			return rejectV1AdmissionResponse(err)
		}
		return validatePVC(pvc)
		// klog.Info("admit PVC success")
		// return reviewResponse
	default:
		err := fmt.Errorf("expect resource to be %v", PVCV1GVR)
		klog.Error(err)
		return rejectV1AdmissionResponse(err)
	}
}

func validatePVC(pvc *corev1.PersistentVolumeClaim) *v1.AdmissionResponse {
	reviewResponse := &v1.AdmissionResponse{
		Allowed: true,
		Result:  &metav1.Status{},
	}
	scName := pvc.Spec.StorageClassName
	if scName != nil && *scName == "" {
		return reviewResponse
	}
	klog.Infof("for PVC %s, SC %s", pvc.Name, *scName)
	// Get SC object
	client, err := k8sClient()
	if err != nil {
		klog.Error(err.Error())
		return &v1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: fmt.Sprintf("failed to initialize k8s client, err: %v", err),
			},
		}
	}
	sc, err := client.StorageV1().StorageClasses().Get(context.TODO(), *scName, metav1.GetOptions{})
	if err != nil {
		klog.Error(err.Error())
		return &v1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: fmt.Sprintf("failed to get storage class %s for PVC %s, err: %v", *scName, pvc.Name, err),
			},
		}
	}

	if !isMultishareSC(sc) {
		return reviewResponse
	}
	// Validate PVC size ranges
	valBytes, err := validateMaxVolumeSize(sc)
	if err != nil {
		klog.Error(err.Error())
		return &v1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: fmt.Sprintf("failed to validate storage class %s params for PVC %s, err: %v", *scName, pvc.Name, err),
			},
		}
	}
	klog.Infof("SC MaxVolSize (bytes) %d", valBytes)
	_ = valBytes
	return reviewResponse
}

func k8sClient() (kubernetes.Interface, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return clientset, nil
}

func isMultishareSC(sc *storagev1.StorageClass) bool {
	if sc.Provisioner != FilestoreCSIDriver {
		return false
	}
	val, ok := sc.Parameters[Multishare]
	if !ok || strings.ToLower(val) == "false" {
		return false
	}
	return true
}

func validateMaxVolumeSize(sc *storagev1.StorageClass) (int64, error) {
	v, ok := sc.Parameters[MaxVolumeSize]
	if !ok {
		return Ti, nil
	}
	val, err := resource.ParseQuantity(v)
	if err != nil {
		return 0, err
	}
	valBytes := val.Value()
	switch valBytes {
	case 128 * Gi:
		return valBytes, nil
	case 256 * Gi:
		return valBytes, nil
	case 512 * Gi:
		return valBytes, nil
	case 1024 * Gi:
		return valBytes, nil
	}
	return 0, fmt.Errorf("invalid PVC size %d, allowed sizes are '128Gi', '256Gi', '512Gi', '1Ti'", valBytes)
}
