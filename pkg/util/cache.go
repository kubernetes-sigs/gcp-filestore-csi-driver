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
)

type OpInfo struct {
	Name string // Unique op identifier
	Type OperationType
}

type ShareCreateOpInfo struct {
	InstanceHandle string // Placeholder instance of the form project/location/instanceName
	OpName         string // share create op
}

type ShareCreateMapItem struct {
	Key    string
	OpInfo ShareCreateOpInfo
}

type ShareOpsMapItem struct {
	Key    ShareKey
	OpInfo OpInfo
}

type InstanceMapItem struct {
	Key    InstanceKey
	OpInfo OpInfo
}

type InstanceKey string
type ShareKey string

func CreateInstanceKey(project, location, instanceName string) InstanceKey {
	return InstanceKey(fmt.Sprintf("%s/%s/%s", project, location, instanceName))
}

func CreateShareKey(project, location, instanceName, shareName string) ShareKey {
	return ShareKey(fmt.Sprintf("%s/%s/%s/%s", project, location, instanceName, shareName))
}

// This map serves two purpose. To capture any ongoing instance ops, and to keep track of list of known filestore instances that belong to a given storage class.
type InstanceMap map[InstanceKey]OpInfo

func (s InstanceMap) Get(key InstanceKey) *OpInfo {
	k, ok := s[key]
	if !ok {
		return nil
	}
	return &k
}

func (s InstanceMap) Add(key InstanceKey, o OpInfo) {
	s[key] = o
}

func (s InstanceMap) DeleteKey(key InstanceKey) {
	delete(s, key)
}

func (s InstanceMap) DeleteValue(key InstanceKey, opName string) error {
	k, ok := s[key]
	if !ok {
		return nil
	}

	if k.Name != opName {
		return fmt.Errorf("For key %q, cannot clear op %q, cache already contains op %q", key, opName, k.Name)
	}

	s[key] = OpInfo{Name: "", Type: UnknownOp}
	return nil
}

func (s InstanceMap) Keys() []InstanceKey {
	keys := make([]InstanceKey, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	return keys
}

func (s InstanceMap) Items() []InstanceMapItem {
	var items []InstanceMapItem
	for k, v := range s {
		items = append(items, InstanceMapItem{Key: k, OpInfo: v})
	}
	return items
}

// Map of share name to last known share creation operation
type ShareCreateMap map[string]ShareCreateOpInfo

func (s ShareCreateMap) Get(key string) *ShareCreateOpInfo {
	k, ok := s[key]
	if !ok {
		return nil
	}
	return &k
}

func (s ShareCreateMap) Add(key string, v ShareCreateOpInfo) {
	s[key] = v
}

func (s ShareCreateMap) Delete(key string, opName string) error {
	k, ok := s[key]
	if !ok {
		return nil
	}

	if k.OpName != opName {
		return fmt.Errorf("For key %q, cannot clear op %q, cache already contains op %q", key, opName, k.OpName)
	}
	delete(s, key)
	return nil
}

func (s ShareCreateMap) Keys() []string {
	keys := make([]string, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	return keys
}

func (s ShareCreateMap) Items() []ShareCreateMapItem {
	var items []ShareCreateMapItem
	for k, v := range s {
		items = append(items, ShareCreateMapItem{Key: k, OpInfo: v})
	}
	return items
}

// Map of share handle to share operation details
type ShareOpsMap map[ShareKey]OpInfo

func (s ShareOpsMap) Get(key ShareKey) *OpInfo {
	k, ok := s[key]
	if !ok {
		return nil
	}
	return &k
}

func (s ShareOpsMap) Add(key ShareKey, v OpInfo) {
	s[key] = v
}

func (s ShareOpsMap) Delete(key ShareKey, opName string) error {
	k, ok := s[key]
	if !ok {
		return nil
	}

	if k.Name != opName {
		return fmt.Errorf("For key %q, cannot clear op %q, cache already contains op %q", key, opName, k.Name)
	}

	delete(s, key)
	return nil
}

func (s ShareOpsMap) Keys() []ShareKey {
	var keys []ShareKey
	for k := range s {
		keys = append(keys, k)
	}
	return keys
}

func (s ShareOpsMap) Items() []ShareOpsMapItem {
	var items []ShareOpsMapItem
	for k, v := range s {
		items = append(items, ShareOpsMapItem{Key: k, OpInfo: v})
	}
	return items
}

type StorageClassInfo struct {
	InstanceMap    InstanceMap    // Map of Filestore instance handle to last known Instance ops.
	ShareCreateMap ShareCreateMap // Map of share name to last known ongoing share creation operation on a given placeholder Filestore instance.
	ShareOpsMap    ShareOpsMap    // Map of share handle (project/location/instance/share) to share operation details
}

// Map of storageclass name to StorageClassInfo
type StorageClassInfoCache struct {
	ScInfoMap map[string]StorageClassInfo
	Ready     bool
}

func NewStorageClassInfoCache() *StorageClassInfoCache {
	return &StorageClassInfoCache{
		ScInfoMap: make(map[string]StorageClassInfo),
	}
}

func (m *StorageClassInfoCache) GetInstanceMap(scInfoMapKey string) InstanceMap {
	v, ok := m.ScInfoMap[scInfoMapKey]
	if !ok {
		return nil
	}
	return v.InstanceMap
}

func (m *StorageClassInfoCache) AddInstanceOp(scInfoMapKey string, instanceKey InstanceKey, opInfo OpInfo) {
	_, ok := m.ScInfoMap[scInfoMapKey]
	if !ok {
		m.ScInfoMap[scInfoMapKey] = StorageClassInfo{
			InstanceMap:    make(InstanceMap),
			ShareCreateMap: make(ShareCreateMap),
			ShareOpsMap:    make(ShareOpsMap),
		}
	}
	m.ScInfoMap[scInfoMapKey].InstanceMap.Add(instanceKey, opInfo)
}

func (m *StorageClassInfoCache) GetInstanceOp(scInfoMapKey string, instanceKey InstanceKey) *OpInfo {
	v, ok := m.ScInfoMap[scInfoMapKey]
	if !ok {
		return nil
	}
	return v.InstanceMap.Get(instanceKey)
}

func (m *StorageClassInfoCache) DeleteInstanceOp(scInfoMapKey string, instanceKey InstanceKey, opName string) error {
	v, ok := m.ScInfoMap[scInfoMapKey]
	if !ok {
		return nil
	}
	return v.InstanceMap.DeleteValue(instanceKey, opName)
}

func (m *StorageClassInfoCache) DeleteInstance(scInfoMapKey string, instanceKey InstanceKey) {
	v, ok := m.ScInfoMap[scInfoMapKey]
	if !ok {
		return
	}
	v.InstanceMap.DeleteKey(instanceKey)
}

func (m *StorageClassInfoCache) GetShareCreateMap(scInfoMapKey string) ShareCreateMap {
	v, ok := m.ScInfoMap[scInfoMapKey]
	if !ok {
		return nil
	}
	return v.ShareCreateMap
}

func (m *StorageClassInfoCache) GetShareOpsMap(scInfoMapKey string) ShareOpsMap {
	v, ok := m.ScInfoMap[scInfoMapKey]
	if !ok {
		return nil
	}
	return v.ShareOpsMap
}

func (m *StorageClassInfoCache) AddShareCreateOp(scInfoMapKey string, shareName string, opInfo ShareCreateOpInfo) error {
	if _, ok := m.ScInfoMap[scInfoMapKey]; !ok {
		return fmt.Errorf("missing key %s", scInfoMapKey)
	}
	m.ScInfoMap[scInfoMapKey].ShareCreateMap.Add(shareName, opInfo)
	return nil
}

func (m *StorageClassInfoCache) GetShareCreateOp(scInfoMapKey string, shareName string) *ShareCreateOpInfo {
	v, ok := m.ScInfoMap[scInfoMapKey]
	if !ok {
		return nil
	}
	return v.ShareCreateMap.Get(shareName)
}

func (m *StorageClassInfoCache) DeleteShareCreateOp(scInfoMapKey string, shareName string, opName string) error {
	v, ok := m.ScInfoMap[scInfoMapKey]
	if !ok {
		return nil
	}
	return v.ShareCreateMap.Delete(shareName, opName)
}

func (m *StorageClassInfoCache) AddShareOp(scInfoMapKey string, shareKey ShareKey, opInfo OpInfo) error {
	if _, ok := m.ScInfoMap[scInfoMapKey]; !ok {
		return fmt.Errorf("missing key %s", scInfoMapKey)
	}
	m.ScInfoMap[scInfoMapKey].ShareOpsMap.Add(shareKey, opInfo)
	return nil
}

func (m *StorageClassInfoCache) GetShareOp(scInfoMapKey string, shareKey ShareKey) *OpInfo {
	_, ok := m.ScInfoMap[scInfoMapKey]
	if !ok {
		return nil
	}
	return m.ScInfoMap[scInfoMapKey].ShareOpsMap.Get(shareKey)
}

func (m *StorageClassInfoCache) DeleteShareOp(scInfoMapKey string, shareKey ShareKey, opName string) error {
	_, ok := m.ScInfoMap[scInfoMapKey]
	if !ok {
		return nil
	}
	return m.ScInfoMap[scInfoMapKey].ShareOpsMap.Delete(shareKey, opName)
}
