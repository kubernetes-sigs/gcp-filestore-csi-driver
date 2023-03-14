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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ShareInfo is a specification for a Foo resource
type ShareInfo struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ShareInfoSpec `json:"spec"`
	// +optional
	Status *ShareInfoStatus `json:"status"`
}

// ShareInfoSpec is the spec for a Foo resource
type ShareInfoSpec struct {
	ShareName       string `json:"shareName"`
	CapacityBytes   int64  `json:"capacityBytes"`
	InstancePoolTag string `json:"instancePoolTag"`
}

// ShareInfoStatus is the status for a Foo resource
type ShareInfoStatus struct {
	InstanceHandle string          `json:"instanceHandle"`
	CapacityBytes  int64           `json:"capacityBytes,omitempty"`
	ShareStatus    FilestoreStatus `json:"shareStatus,omitempty"`
	Error          string          `json:"error"`
}

// FilestoreShareStatusType identifies a specific share status.
type FilestoreStatus string

// These are valid conditions of a FilestoreShareStatus.
const (
	CREATING FilestoreStatus = "creating"
	READY    FilestoreStatus = "ready"
	UPDATING FilestoreStatus = "updating"
	DELETED  FilestoreStatus = "deleted"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ShareInfoList is a list of Foo resources
type ShareInfoList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ShareInfo `json:"items"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ShareInfo is a specification for a Foo resource
type InstanceInfo struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec InstanceInfoSpec `json:"spec"`
	// +optional
	Status *InstanceInfoStatus `json:"status"`
}

// ShareInfoSpec is the spec for a Foo resource
type InstanceInfoSpec struct {
	CapacityBytes    int64  `json:"capacityBytes"`
	StorageClassName string `json:"storageClassName"`
}

// ShareInfoStatus is the status for a Foo resource
type InstanceInfoStatus struct {
	ShareNames         []string        `json:"shareNames"`
	CapacityBytes      int64           `json:"capacityBytes,omitempty"`
	InstanceStatus     FilestoreStatus `json:"instanceStatus,omitempty"`
	CapacityStepSizeGb int64           `json:"capacityStepSizeGb,omitempty"`
	Cidr               string          `json:"cidr"`
	Error              string          `json:"error"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ShareInfoList is a list of Foo resources
type InstanceInfoList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []InstanceInfo `json:"items"`
}
