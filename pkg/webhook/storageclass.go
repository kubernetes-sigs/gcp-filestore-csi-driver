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
	"regexp"
	"strings"

	v1 "k8s.io/api/admission/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

var (
	// StorageClassV1GVR is GroupVersionResource for v1 StorageClass
	StorageClassV1GVR         = metav1.GroupVersionResource{Group: "storage.k8s.io", Version: "v1", Resource: "storageclasses"}
	FilestoreCSIDriver        = "filestore.csi.storage.gke.io"
	TierEnterprise            = "enterprise"
	InstanceStorageClassLabel = "instance-storageclass-label"
	Multishare                = "multishare"
)

func rejectV1AdmissionResponse(err error) *v1.AdmissionResponse {
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

	if ar.Request.Operation != v1.Create {
		return reviewResponse
	}

	raw := ar.Request.Object.Raw

	deserializer := codecs.UniversalDeserializer()
	switch ar.Request.Resource {
	case StorageClassV1GVR:
		sc := &storagev1.StorageClass{}
		if _, _, err := deserializer.Decode(raw, nil, sc); err != nil {
			klog.Error(err)
			return rejectV1AdmissionResponse(err)
		}
		klog.Infof("check patch for storageClass %s", sc.Name)
		return applyV1StorageClassPatch(sc)
	default:
		err := fmt.Errorf("expect resource to be %v", StorageClassV1GVR)
		klog.Error(err)
		return rejectV1AdmissionResponse(err)
	}
}

func applyV1StorageClassPatch(sc *storagev1.StorageClass) *v1.AdmissionResponse {
	reviewResponse := &v1.AdmissionResponse{
		Allowed: true,
		Result:  &metav1.Status{},
	}

	if sc.Provisioner != FilestoreCSIDriver {
		return reviewResponse
	}

	isMultishare, ok := sc.Parameters[Multishare]
	if !ok || strings.ToLower(isMultishare) == "false" {
		return reviewResponse
	}

	if strings.ToLower(isMultishare) != "true" {
		return rejectV1AdmissionResponse(fmt.Errorf("the acceptable values for %q are 'True', 'true', 'false' or 'False'", Multishare))
	}

	tier, ok := sc.Parameters["tier"]
	if !ok || tier != TierEnterprise {
		return rejectV1AdmissionResponse(fmt.Errorf("mutlishare is only supported on %q tier instances", TierEnterprise))
	}

	if instanceLabel, ok := sc.Parameters[InstanceStorageClassLabel]; ok {
		if validateInstanceLabel(instanceLabel) {
			return reviewResponse
		} else {
			return rejectV1AdmissionResponse(fmt.Errorf("%q can contain only lowercase letters, numeric characters, underscores, and dashes and have a maximum length of 63 characters", InstanceStorageClassLabel))
		}
	}

	instanceLabel := strings.ToLower(sc.Name)
	if !validateInstanceLabel(instanceLabel) {
		return rejectV1AdmissionResponse(fmt.Errorf("if using storageclass name as %q, it can contain only letters, numeric characters, underscores, and dashes and have a maximum length of 63 characters", InstanceStorageClassLabel))
	}

	scPatch := fmt.Sprintf(`[{"op":"add", "path":"/parameters/%s","value": "%s"}]`, InstanceStorageClassLabel, instanceLabel)
	klog.Infof("patching value: %s", scPatch)
	reviewResponse.Patch = []byte(scPatch)
	pt := v1.PatchTypeJSONPatch
	reviewResponse.PatchType = &pt
	return reviewResponse
}

func validateInstanceLabel(label string) bool {
	// https://cloud.google.com/filestore/docs/managing-labels#requirements
	regex, _ := regexp.Compile(`^(([a-z][a-z0-9_-]{0,61})?[a-z0-9])?$`)
	return regex.MatchString(label)
}
