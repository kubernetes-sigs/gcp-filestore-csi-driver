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

package util

const (
	InstanceURISplitLen        = 6
	ShareURISplitLen           = 8
	MultishareCSIVolIdSplitLen = 6

	MinMultishareInstanceSizeBytes    int64 = 1 * Tb
	MaxMultishareInstanceSizeBytes    int64 = 10 * Tb
	MaxShareSizeBytes                 int64 = 1 * Tb
	MinShareSizeBytes                 int64 = 100 * Gb
	MaxSharesPerInstance                    = 10
	NewMultishareInstancePrefix             = "fs-"
	ParamMultishareInstanceScLabelKey       = "storage_gke_io_storage-class-id"

	// configurable max shares consts
	MinShareSizeConfigurableBytes int64 = 10 * Gb
)

type OperationType int

const (
	InstanceCreate OperationType = iota
	InstanceDelete
	InstanceUpdate
	ShareCreate
	ShareDelete
	ShareUpdate
	UnknownOp
)

func (o OperationType) String() string {
	switch o {
	case InstanceCreate:
		return "instance" + OpVerbCreate
	case InstanceDelete:
		return "instance" + OpVerbDelete
	case InstanceUpdate:
		return "instance" + OpVerbUpdate
	case ShareCreate:
		return "share" + OpVerbCreate
	case ShareDelete:
		return "share" + OpVerbDelete
	case ShareUpdate:
		return "share" + OpVerbUpdate
	default:
		return "unknown"
	}
}

type OperationStatus int

const (
	StatusSuccess OperationStatus = iota
	StatusRunning
	StatusFailed
	StatusUnknown
)

const (
	OpVerbCreate = "create"
	OpVerbDelete = "delete"
	OpVerbUpdate = "update"
)

func ConvertInstanceOpVerbToType(v string) OperationType {
	switch v {
	case OpVerbCreate:
		return InstanceCreate
	case OpVerbDelete:
		return InstanceDelete
	case OpVerbUpdate:
		return InstanceUpdate
	default:
		return UnknownOp
	}
}

func ConvertShareOpVerbToType(v string) OperationType {
	switch v {
	case OpVerbCreate:
		return ShareCreate
	case OpVerbDelete:
		return ShareDelete
	case OpVerbUpdate:
		return ShareUpdate
	default:
		return UnknownOp
	}
}
