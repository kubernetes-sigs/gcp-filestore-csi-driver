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
	"fmt"

	v1 "k8s.io/api/admission/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

var (
	// StorageClassV1GVR is GroupVersionResource for v1 StorageClass
	StorageClassV1GVR  = metav1.GroupVersionResource{Group: "storage.k8s.io", Version: "v1", Resource: "storageclasses"}
	FilestoreCSIDriver = "filestore.csi.storage.gke.io"
)

func admitStorageClass(ar v1.AdmissionReview) *v1.AdmissionResponse {
	klog.Info("admitting storageClass")
	reviewResponse := &v1.AdmissionResponse{
		Allowed: true,
		Result:  &metav1.Status{},
	}

	// Admit requests other than Update and Create
	if !(ar.Request.Operation == v1.Update || ar.Request.Operation == v1.Create) {
		return reviewResponse
	}
	isUpdate := ar.Request.Operation == v1.Update

	raw := ar.Request.Object.Raw
	oldRaw := ar.Request.OldObject.Raw

	deserializer := codecs.UniversalDeserializer()
	switch ar.Request.Resource {
	case StorageClassV1GVR:
		sc := &storagev1.StorageClass{}
		if _, _, err := deserializer.Decode(raw, nil, sc); err != nil {
			klog.Error(err)
			return toV1AdmissionResponse(err)
		}
		oldSc := &storagev1.StorageClass{}
		if _, _, err := deserializer.Decode(oldRaw, nil, oldSc); err != nil {
			klog.Error(err)
			return toV1AdmissionResponse(err)
		}
		return decideV1StorageClass(sc, oldSc, isUpdate)
	default:
		err := fmt.Errorf("expect resource to be %s", StorageClassV1GVR)
		klog.Error(err)
		return toV1AdmissionResponse(err)
	}
}

func decideV1StorageClass(sc, oldsc *storagev1.StorageClass, isUpdate bool) *v1.AdmissionResponse {
	reviewResponse := &v1.AdmissionResponse{
		Allowed: true,
		Result:  &metav1.Status{},
	}

	if isUpdate {
		// TBD
		return reviewResponse
	}

	if err := ValidateV1StorageClass(sc); err != nil {
		reviewResponse.Allowed = false
		reviewResponse.Result.Message = err.Error()
	}
	return reviewResponse
}

func ValidateV1StorageClass(sc *storagev1.StorageClass) error {
	if sc == nil {
		return fmt.Errorf("StorageClass is nil")
	}

	if sc.Provisioner != FilestoreCSIDriver {
		return nil
	}

	params := sc.Parameters
	if params == nil {
		return nil
	}

	if _, ok := params["multishare"]; ok {
		klog.Infof("Multishare Filestore CSI storage class detected")
	}

	// TBD: Fill the validation logic here.
	return nil
}

func toV1AdmissionResponse(err error) *v1.AdmissionResponse {
	return &v1.AdmissionResponse{
		Result: &metav1.Status{
			Message: err.Error(),
		},
	}
}
func mutateStorageClass(ar v1.AdmissionReview) *v1.AdmissionResponse {
	klog.Info("mutating storageClass")
	reviewResponse := &v1.AdmissionResponse{
		Allowed: true,
		Result:  &metav1.Status{},
	}

	if !(ar.Request.Operation == v1.Update || ar.Request.Operation == v1.Create) {
		return reviewResponse
	}

	raw := ar.Request.Object.Raw
	oldRaw := ar.Request.OldObject.Raw

	deserializer := codecs.UniversalDeserializer()
	switch ar.Request.Resource {
	case StorageClassV1GVR:
		sc := &storagev1.StorageClass{}
		if _, _, err := deserializer.Decode(raw, nil, sc); err != nil {
			klog.Error(err)
			return toV1AdmissionResponse(err)
		}
		oldSc := &storagev1.StorageClass{}
		if _, _, err := deserializer.Decode(oldRaw, nil, oldSc); err != nil {
			klog.Error(err)
			return toV1AdmissionResponse(err)
		}
		klog.Infof("check patch for storageClass %s", sc.Name)
		return applyV1StorageClassPatch(sc)
	default:
		err := fmt.Errorf("expect resource to be %s", StorageClassV1GVR)
		klog.Error(err)
		return toV1AdmissionResponse(err)
	}
}

func applyV1StorageClassPatch(sc *storagev1.StorageClass) *v1.AdmissionResponse {
	reviewResponse := &v1.AdmissionResponse{
		Allowed: true,
	}

	if sc.Provisioner != FilestoreCSIDriver {
		return reviewResponse
	}

	isMultishare, ok := sc.Parameters["multishare"]
	if !ok || isMultishare == "false" || isMultishare == "False" {
		return reviewResponse
	}

	if _, ok := sc.Parameters["instancePrefix"]; ok {
		return reviewResponse
	}

	scPatch := fmt.Sprintf(`[{"op":"add", "path":"/parameters/instancePrefix","value": "%s"}]`, sc.Name)
	klog.Infof("patching value: %s", scPatch)
	reviewResponse.Patch = []byte(scPatch)
	pt := v1.PatchTypeJSONPatch
	reviewResponse.PatchType = &pt
	return reviewResponse
}
