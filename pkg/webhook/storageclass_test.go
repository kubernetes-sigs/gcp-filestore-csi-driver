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
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	v1 "k8s.io/api/admission/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestMutateStorageClass(t *testing.T) {
	storageClassName := "filestore-multishare"
	labelName := "multishare-label"

	testCases := []struct {
		name         string
		storageClass *storagev1.StorageClass
		operation    v1.Operation
		shouldAdmit  bool
		patch        string
		msg          string
	}{
		{
			name: "create with non-multishare should be allowed",
			storageClass: &storagev1.StorageClass{
				ObjectMeta:  metav1.ObjectMeta{Name: storageClassName},
				Provisioner: FilestoreCSIDriver,
			},
			operation:   v1.Create,
			shouldAdmit: true,
		},
		{
			name: "create with other provisioner should be allowed",
			storageClass: &storagev1.StorageClass{
				ObjectMeta:  metav1.ObjectMeta{Name: storageClassName},
				Provisioner: "pd.csi.storage.gke.io",
			},
			operation:   v1.Create,
			shouldAdmit: true,
		},
		{
			name: "create with multishare but default tier should not be allowed",
			storageClass: &storagev1.StorageClass{
				ObjectMeta:  metav1.ObjectMeta{Name: storageClassName},
				Provisioner: FilestoreCSIDriver,
				Parameters: map[string]string{
					"multishare": "true",
				},
			},
			operation:   v1.Create,
			shouldAdmit: false,
			msg:         fmt.Errorf("mutlishare is only supported on %q tier instances", TierEnterprise).Error(),
		},
		{
			name: "create with multishare not true or false should not be allowed",
			storageClass: &storagev1.StorageClass{
				ObjectMeta:  metav1.ObjectMeta{Name: storageClassName},
				Provisioner: FilestoreCSIDriver,
				Parameters: map[string]string{
					"multishare": "blah",
				},
			},
			operation:   v1.Create,
			shouldAdmit: false,
			msg:         fmt.Errorf("the acceptable values for %q are 'True', 'true', 'false' or 'False'", Multishare).Error(),
		},
		{
			name: "create with multishare empty should not be allowed",
			storageClass: &storagev1.StorageClass{
				ObjectMeta:  metav1.ObjectMeta{Name: storageClassName},
				Provisioner: FilestoreCSIDriver,
				Parameters: map[string]string{
					"multishare": "",
				},
			},
			operation:   v1.Create,
			shouldAdmit: false,
			msg:         fmt.Errorf("the acceptable values for %q are 'True', 'true', 'false' or 'False'", Multishare).Error(),
		},
		{
			name: "create with multishare but not enterprise tier should not be allowed",
			storageClass: &storagev1.StorageClass{
				ObjectMeta:  metav1.ObjectMeta{Name: storageClassName},
				Provisioner: FilestoreCSIDriver,
				Parameters: map[string]string{
					"multishare": "true",
					"tier":       "performance",
				},
			},
			operation:   v1.Create,
			shouldAdmit: false,
			msg:         fmt.Errorf("mutlishare is only supported on %q tier instances", TierEnterprise).Error(),
		},
		{
			name: "should fill in instanceStorageClassLabel if not present",
			storageClass: &storagev1.StorageClass{
				ObjectMeta:  metav1.ObjectMeta{Name: storageClassName},
				Provisioner: FilestoreCSIDriver,
				Parameters: map[string]string{
					"multishare": "true",
					"tier":       TierEnterprise,
				},
			},
			operation:   v1.Create,
			shouldAdmit: true,
			patch:       fmt.Sprintf(`[{"op":"add", "path":"/parameters/%s","value": "%s"}]`, InstanceStorageClassLabel, storageClassName),
		},
		{
			name: "should fill in instanceStorageClassLabel if not present and convert to lower case",
			storageClass: &storagev1.StorageClass{
				ObjectMeta:  metav1.ObjectMeta{Name: strings.ToUpper(storageClassName)},
				Provisioner: FilestoreCSIDriver,
				Parameters: map[string]string{
					"multishare": "true",
					"tier":       TierEnterprise,
				},
			},
			operation:   v1.Create,
			shouldAdmit: true,
			patch:       fmt.Sprintf(`[{"op":"add", "path":"/parameters/%s","value": "%s"}]`, InstanceStorageClassLabel, storageClassName),
		},
		{
			name: "should not change instanceStorageClassLabel if already set",
			storageClass: &storagev1.StorageClass{
				ObjectMeta:  metav1.ObjectMeta{Name: storageClassName},
				Provisioner: FilestoreCSIDriver,
				Parameters: map[string]string{
					"multishare":              "true",
					"tier":                    TierEnterprise,
					InstanceStorageClassLabel: labelName,
				},
			},
			operation:   v1.Create,
			shouldAdmit: true,
		},
		{
			name: "should not allow invalid instanceStorageClassLabel if already set",
			storageClass: &storagev1.StorageClass{
				ObjectMeta:  metav1.ObjectMeta{Name: storageClassName},
				Provisioner: FilestoreCSIDriver,
				Parameters: map[string]string{
					"multishare":              "true",
					"tier":                    TierEnterprise,
					InstanceStorageClassLabel: "label-*",
				},
			},
			operation:   v1.Create,
			shouldAdmit: false,
			msg:         fmt.Errorf("%q can contain only lowercase letters, numeric characters, underscores, and dashes and have a maximum length of 63 characters", InstanceStorageClassLabel).Error(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sc := tc.storageClass
			raw, err := json.Marshal(sc)
			if err != nil {
				t.Fatal(err)
			}
			review := v1.AdmissionReview{
				Request: &v1.AdmissionRequest{
					Object: runtime.RawExtension{
						Raw: raw,
					},
					Resource:  StorageClassV1GVR,
					Operation: tc.operation,
				},
			}
			response := mutateStorageClass(review)
			admit := response.Allowed
			msg := response.Result.Message
			patch := string(response.Patch)

			if admit != tc.shouldAdmit {
				t.Errorf("expected admit %t but got %t", tc.shouldAdmit, admit)
			}
			if msg != tc.msg {
				t.Errorf("expected msg %q but got %q", tc.msg, msg)
			}
			if patch != tc.patch {
				t.Errorf("expected patch %q but got %q", tc.patch, patch)
			}
		})
	}
}

func TestValidateInstanceLabel(t *testing.T) {
	testCases := []struct {
		name    string
		label   string
		isValid bool
	}{
		{
			name:    "valid label",
			label:   "abc-123_s",
			isValid: true,
		},
		{
			name:    "label has upper case letters",
			label:   "UPPER_letter",
			isValid: false,
		},
		{
			name:    "empty label",
			label:   "",
			isValid: true,
		},
		{
			name:    "label too long",
			label:   strings.Repeat("a", 64),
			isValid: false,
		},
		{
			name:    "label at length",
			label:   strings.Repeat("a", 63),
			isValid: true,
		},
		{
			name:    "label has special char",
			label:   "abc-*",
			isValid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := validateInstanceLabel(tc.label)

			if result != tc.isValid {
				t.Errorf("expected the validity of label %q to be %t but got %t", tc.label, tc.isValid, result)
			}
		})
	}
}

func TestValidateMaxVolumeSize(t *testing.T) {
	storageClassName := "filestore-multishare"
	tests := []struct {
		name        string
		sc          *storagev1.StorageClass
		errExpected bool
	}{
		// Failure cases
		{
			name: "max-volume-size key set, value empty",
			sc: &storagev1.StorageClass{
				ObjectMeta:  metav1.ObjectMeta{Name: storageClassName},
				Provisioner: FilestoreCSIDriver,
				Parameters: map[string]string{
					"max-volume-size": "",
				},
			},
			errExpected: true,
		},
		{
			name: "max-volume-size key set, negative value",
			sc: &storagev1.StorageClass{
				ObjectMeta:  metav1.ObjectMeta{Name: storageClassName},
				Provisioner: FilestoreCSIDriver,
				Parameters: map[string]string{
					"max-volume-size": "-10",
				},
			},
			errExpected: true,
		},
		{
			name: "max-volume-size key set, value invalid - test1",
			sc: &storagev1.StorageClass{
				ObjectMeta:  metav1.ObjectMeta{Name: storageClassName},
				Provisioner: FilestoreCSIDriver,
				Parameters: map[string]string{
					"max-volume-size": "100",
				},
			},
			errExpected: true,
		},
		{
			name: "max-volume-size key set, value invalid - test2",
			sc: &storagev1.StorageClass{
				ObjectMeta:  metav1.ObjectMeta{Name: storageClassName},
				Provisioner: FilestoreCSIDriver,
				Parameters: map[string]string{
					"max-volume-size": "100Gi",
				},
			},
			errExpected: true,
		},
		// Successul cases
		{
			name: "max-volume-size key set, value valid - test1",
			sc: &storagev1.StorageClass{
				ObjectMeta:  metav1.ObjectMeta{Name: storageClassName},
				Provisioner: FilestoreCSIDriver,
				Parameters: map[string]string{
					"max-volume-size": "128Gi",
				},
			},
		},
		{
			name: "max-volume-size key set, value valid - test2",
			sc: &storagev1.StorageClass{
				ObjectMeta:  metav1.ObjectMeta{Name: storageClassName},
				Provisioner: FilestoreCSIDriver,
				Parameters: map[string]string{
					"max-volume-size": "256Gi",
				},
			},
		},
		{
			name: "max-volume-size key set, value valid - test3",
			sc: &storagev1.StorageClass{
				ObjectMeta:  metav1.ObjectMeta{Name: storageClassName},
				Provisioner: FilestoreCSIDriver,
				Parameters: map[string]string{
					"max-volume-size": "512Gi",
				},
			},
		},
		{
			name: "max-volume-size key set, value valid - test4",
			sc: &storagev1.StorageClass{
				ObjectMeta:  metav1.ObjectMeta{Name: storageClassName},
				Provisioner: FilestoreCSIDriver,
				Parameters: map[string]string{
					"max-volume-size": "1024Gi",
				},
			},
		},
		{
			name: "max-volume-size key set, value valid - test5",
			sc: &storagev1.StorageClass{
				ObjectMeta:  metav1.ObjectMeta{Name: storageClassName},
				Provisioner: FilestoreCSIDriver,
				Parameters: map[string]string{
					"max-volume-size": "1Ti",
				},
			},
		},
		{
			name: "max-volume-size key set, value valid in Mi - test6",
			sc: &storagev1.StorageClass{
				ObjectMeta:  metav1.ObjectMeta{Name: storageClassName},
				Provisioner: FilestoreCSIDriver,
				Parameters: map[string]string{
					"max-volume-size": "131072Mi",
				},
			},
		},
		{
			name: "max-volume-size key set, value valid in bytes - test7",
			sc: &storagev1.StorageClass{
				ObjectMeta:  metav1.ObjectMeta{Name: storageClassName},
				Provisioner: FilestoreCSIDriver,
				Parameters: map[string]string{
					"max-volume-size": "137438953472",
				},
			},
		},
		{
			name: "max-volume-size key set, value valid in bytes - test8",
			sc: &storagev1.StorageClass{
				ObjectMeta:  metav1.ObjectMeta{Name: storageClassName},
				Provisioner: FilestoreCSIDriver,
				Parameters: map[string]string{
					"max-volume-size": "274877906944",
				},
			},
		},
		{
			name: "max-volume-size key set, value valid in bytes - test9",
			sc: &storagev1.StorageClass{
				ObjectMeta:  metav1.ObjectMeta{Name: storageClassName},
				Provisioner: FilestoreCSIDriver,
				Parameters: map[string]string{
					"max-volume-size": "549755813888",
				},
			},
		},
		{
			name: "max-volume-size key set, value valid in bytes - test10",
			sc: &storagev1.StorageClass{
				ObjectMeta:  metav1.ObjectMeta{Name: storageClassName},
				Provisioner: FilestoreCSIDriver,
				Parameters: map[string]string{
					"max-volume-size": "1099511627776",
				},
			},
		},
	}
	originalfeatureValue := featureMaxSharesPerInstance
	featureMaxSharesPerInstance = true
	defer func() {
		featureMaxSharesPerInstance = originalfeatureValue
	}()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateMaxVolumeSizeParam(tc.sc)
			if err != nil && !tc.errExpected {
				t.Errorf("got unexpected error %s", err)
			}
			if err == nil && tc.errExpected {
				t.Errorf("expected error got nil")
			}
		})
	}
}
