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

import (
	"fmt"
	"sync"
)

type OperationType int

const (
	InstanceCreate OperationType = iota
	InstanceDelete
	InstanceExpand
	InstanceShrink
	ShareCreate
	ShareDelete
	ShareExpand
)

type OpInfo struct {
	Name string // Unique op identifier
	Type OperationType
}

type ShareCreateOpInfo struct {
	Instance string // Placeholder instance
	OpName   string // share create op
}

type InstanceKey string
type ShareKey string

func CreateInstanceKey(project, location, instanceName string) InstanceKey {
	return InstanceKey(fmt.Sprintf("%s/%s/%s", project, location, instanceName))
}

func CreateShareKey(project, location, instanceName, shareName string) ShareKey {
	return ShareKey(fmt.Sprintf("%s/%s/%s/%s", project, location, instanceName, shareName))
}

// Map of last known instance ops running for a given instance
type InstanceMap struct {
	m map[InstanceKey]OpInfo
}

func (s *InstanceMap) Get(key InstanceKey) *OpInfo {
	k, ok := s.m[key]
	if !ok {
		return nil
	}
	return &k
}

func (s *InstanceMap) Add(key InstanceKey, o OpInfo) {
	s.m[key] = o
}

func (s *InstanceMap) Delete(key InstanceKey, opName string) error {
	k, ok := s.m[key]
	if !ok {
		return nil
	}

	if k.Name != opName {
		return fmt.Errorf("For key %q, cannot clear op %q, cache already contains op %q", key, opName, k.Name)
	}

	delete(s.m, key)
	return nil
}

func NewInstanceMap() *InstanceMap {
	return &InstanceMap{
		m: make(map[InstanceKey]OpInfo),
	}
}

// Map of share name to last known share creation operation
type ShareCreateMap struct {
	m map[string]ShareCreateOpInfo
}

func (s *ShareCreateMap) Get(key string) *ShareCreateOpInfo {
	k, ok := s.m[key]
	if !ok {
		return nil
	}
	return &k
}

func (s *ShareCreateMap) Add(key string, v ShareCreateOpInfo) {
	s.m[key] = v
}

func (s *ShareCreateMap) Delete(key string, opName string) error {
	k, ok := s.m[key]
	if !ok {
		return nil
	}

	if k.OpName != opName {
		return fmt.Errorf("For key %q, cannot clear op %q, cache already contains op %q", key, opName, k.OpName)
	}
	delete(s.m, key)
	return nil
}

func NewShareCreateMap() *ShareCreateMap {
	return &ShareCreateMap{
		m: make(map[string]ShareCreateOpInfo),
	}
}

// Map of share handle to share operation details
type ShareOpsMap struct {
	m map[ShareKey]OpInfo
}

func NewShareOpsMap() *ShareOpsMap {
	return &ShareOpsMap{
		m: make(map[ShareKey]OpInfo),
	}
}

func (s *ShareOpsMap) Get(key ShareKey) *OpInfo {
	k, ok := s.m[key]
	if !ok {
		return nil
	}
	return &k
}

func (s *ShareOpsMap) Delete(key ShareKey, opName string) error {
	k, ok := s.m[key]
	if !ok {
		return nil
	}

	if k.Name != opName {
		return fmt.Errorf("For key %q, cannot clear op %q, cache already contains op %q", key, opName, k.Name)
	}

	delete(s.m, key)
	return nil
}

type StorageClassInfo struct {
	InstanceMap    *InstanceMap    // Map of Filestore instance handle to last known Instance ops.
	ShareCreateMap *ShareCreateMap // Map of share name to last known ongoing share creation operation on a given placeholder Filestore instance.
	ShareOpsMap    *ShareOpsMap    // Map of share handle (project/location/instance/share) to share operation details
	sync.Mutex
}

// Map of storageclass name to StorageClassInfo
type StorageClassInfoMap struct {
	ScInfoMap map[string]StorageClassInfo
	Ready     bool
}

func NewStorageClassInfoMap() *StorageClassInfoMap {
	return &StorageClassInfoMap{
		ScInfoMap: make(map[string]StorageClassInfo),
	}
}
