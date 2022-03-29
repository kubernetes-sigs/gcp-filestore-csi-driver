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
	"testing"
)

const (
	testProject  = "test-project"
	testLocation = "us-central1"
	testInstance = "testInstance"
	testShare    = "testShane"
)

type Item struct {
	scKey          string
	instanceKey    InstanceKey
	shareKey       ShareKey
	shareCreateKey string
	op             OpInfo
	createOp       ShareCreateOpInfo
}

type AddItem struct {
	item          Item
	skipInitSCKey bool
	expectError   bool
}

type ClearItem struct {
	item        Item
	clearValue  bool
	clearKey    bool
	expectError bool
}
type OutputItem struct {
	item             Item
	expectEmpty      bool
	expectEmptyValue bool
}

func TestInstanceMap(t *testing.T) {
	tests := []struct {
		name   string
		add    []Item
		clear  []ClearItem
		output []OutputItem
	}{
		{
			name: "tc1 - add ops for 1 storage class",
			add: []Item{
				{
					scKey:       "sc-1",
					instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"1")),
					op: OpInfo{
						Name: "op-1",
						Type: InstanceCreate,
					},
				},
				{
					scKey:       "sc-1",
					instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"2")),
					op: OpInfo{
						Name: "op-2",
						Type: InstanceUpdate,
					},
				},
			},
			output: []OutputItem{
				{
					item: Item{
						scKey:       "sc-1",
						instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"1")),
						op: OpInfo{
							Name: "op-1",
							Type: InstanceCreate,
						},
					},
				},
				{
					item: Item{
						scKey:       "sc-1",
						instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"2")),
						op: OpInfo{
							Name: "op-2",
							Type: InstanceUpdate,
						},
					},
				},
			},
		},
		{
			name: "tc2 - add ops for 2 storage classes",
			add: []Item{
				{
					scKey:       "sc-1",
					instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"1")),
					op: OpInfo{
						Name: "op-1",
						Type: InstanceCreate,
					},
				},
				{
					scKey:       "sc-2",
					instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"2")),
					op: OpInfo{
						Name: "op-2",
						Type: InstanceUpdate,
					},
				},
			},
			output: []OutputItem{
				{
					item: Item{
						scKey:       "sc-1",
						instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"1")),
						op: OpInfo{
							Name: "op-1",
							Type: InstanceCreate,
						},
					},
				},
				{
					item: Item{
						scKey:       "sc-2",
						instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"2")),
						op: OpInfo{
							Name: "op-2",
							Type: InstanceUpdate,
						},
					},
				},
			},
		},
		{
			name: "tc3 - add 2 ops, clear the ops for the instances",
			add: []Item{
				{
					scKey:       "sc-1",
					instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"1")),
					op: OpInfo{
						Name: "op-1",
						Type: InstanceCreate,
					},
				},
				{
					scKey:       "sc-2",
					instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"2")),
					op: OpInfo{
						Name: "op-2",
						Type: InstanceUpdate,
					},
				},
			},
			clear: []ClearItem{
				{
					item: Item{
						scKey:       "sc-1",
						instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"1")),
						op: OpInfo{
							Name: "op-1",
							Type: InstanceCreate,
						},
					},
					clearValue: true,
				},
				{
					item: Item{
						scKey:       "sc-2",
						instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"2")),
						op: OpInfo{
							Name: "op-2",
							Type: InstanceUpdate,
						},
					},
					clearValue: true,
				},
			},
			output: []OutputItem{
				{
					item: Item{
						scKey:       "sc-1",
						instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"1")),
					},
					expectEmptyValue: true,
				},
				{
					item: Item{
						scKey:       "sc-2",
						instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"2")),
					},
					expectEmptyValue: true,
				},
			},
		},
		{
			name: "tc4 - add 2 ops, clear op value of one instance",
			add: []Item{
				{
					scKey:       "sc-1",
					instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"1")),
					op: OpInfo{
						Name: "op-1",
						Type: InstanceCreate,
					},
				},
				{
					scKey:       "sc-2",
					instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"2")),
					op: OpInfo{
						Name: "op-2",
						Type: InstanceUpdate,
					},
				},
			},
			clear: []ClearItem{
				{
					item: Item{
						scKey:       "sc-2",
						instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"2")),
						op: OpInfo{
							Name: "op-2",
							Type: InstanceUpdate,
						},
					},
					clearValue: true,
				},
			},
			output: []OutputItem{
				{
					item: Item{
						scKey:       "sc-1",
						instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"1")),
						op: OpInfo{
							Name: "op-1",
							Type: InstanceCreate,
						},
					},
				},
				{
					item: Item{
						scKey:       "sc-2",
						instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"2")),
						op: OpInfo{
							Name: "op-2",
							Type: InstanceUpdate,
						},
					},
					expectEmptyValue: true,
				},
			},
		},
		{
			name: "tc5 - clear op attempt for non-existent sc",
			add: []Item{
				{
					scKey:       "sc-1",
					instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"1")),
					op: OpInfo{
						Name: "op-1",
						Type: InstanceCreate,
					},
				},
				{
					scKey:       "sc-2",
					instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"2")),
					op: OpInfo{
						Name: "op-2",
						Type: InstanceUpdate,
					},
				},
			},
			clear: []ClearItem{
				{
					item: Item{
						scKey:       "sc-3",
						instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"2")),
						op: OpInfo{
							Name: "op-2",
							Type: InstanceUpdate,
						},
					},
					clearValue: true,
				},
			},
			output: []OutputItem{
				{
					item: Item{
						scKey:       "sc-1",
						instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"1")),
						op: OpInfo{
							Name: "op-1",
							Type: InstanceCreate,
						},
					},
				},
				{
					item: Item{
						scKey:       "sc-2",
						instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"2")),
						op: OpInfo{
							Name: "op-2",
							Type: InstanceUpdate,
						},
					},
				},
			},
		},
		{
			name: "tc6 - clear op attempt for existing sc, error expected in clear",
			add: []Item{
				{
					scKey:       "sc-1",
					instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"1")),
					op: OpInfo{
						Name: "op-1",
						Type: InstanceCreate,
					},
				},
				{
					scKey:       "sc-2",
					instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"2")),
					op: OpInfo{
						Name: "op-2",
						Type: InstanceUpdate,
					},
				},
			},
			clear: []ClearItem{
				{
					item: Item{
						scKey:       "sc-2",
						instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"2")),
						op: OpInfo{
							Name: "op-3",
							Type: InstanceUpdate,
						},
					},
					expectError: true,
					clearValue:  true,
				},
			},
			output: []OutputItem{
				{
					item: Item{
						scKey:       "sc-1",
						instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"1")),
						op: OpInfo{
							Name: "op-1",
							Type: InstanceCreate,
						},
					},
				},
				{
					item: Item{
						scKey:       "sc-2",
						instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"2")),
						op: OpInfo{
							Name: "op-2",
							Type: InstanceUpdate,
						},
					},
				},
			},
		},
		{
			name: "tc7 - clear non-existent instance key for existing sc",
			add: []Item{
				{
					scKey:       "sc-1",
					instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"1")),
					op: OpInfo{
						Name: "op-1",
						Type: InstanceCreate,
					},
				},
				{
					scKey:       "sc-2",
					instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"2")),
					op: OpInfo{
						Name: "op-2",
						Type: InstanceUpdate,
					},
				},
			},
			clear: []ClearItem{
				{
					item: Item{
						scKey:       "sc-2",
						instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"3")),
						op: OpInfo{
							Name: "op-3",
							Type: InstanceUpdate,
						},
					},
					clearValue: true,
				},
			},
			output: []OutputItem{
				{
					item: Item{
						scKey:       "sc-1",
						instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"1")),
						op: OpInfo{
							Name: "op-1",
							Type: InstanceCreate,
						},
					},
				},
				{
					item: Item{
						scKey:       "sc-2",
						instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"2")),
						op: OpInfo{
							Name: "op-2",
							Type: InstanceUpdate,
						},
					},
				},
				{
					item: Item{
						scKey:       "sc-2",
						instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"3")),
					},
					expectEmpty: true,
				},
			},
		},
		{
			name: "tc8 - add 2 ops, clear the keys for the instances",
			add: []Item{
				{
					scKey:       "sc-1",
					instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"1")),
					op: OpInfo{
						Name: "op-1",
						Type: InstanceCreate,
					},
				},
				{
					scKey:       "sc-2",
					instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"2")),
					op: OpInfo{
						Name: "op-2",
						Type: InstanceUpdate,
					},
				},
			},
			clear: []ClearItem{
				{
					item: Item{
						scKey:       "sc-1",
						instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"1")),
						op: OpInfo{
							Name: "op-1",
							Type: InstanceCreate,
						},
					},
					clearKey: true,
				},
				{
					item: Item{
						scKey:       "sc-2",
						instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"2")),
						op: OpInfo{
							Name: "op-2",
							Type: InstanceUpdate,
						},
					},
					clearKey: true,
				},
			},
			output: []OutputItem{
				{
					item: Item{
						scKey:       "sc-1",
						instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"1")),
					},
					expectEmpty: true,
				},
				{
					item: Item{
						scKey:       "sc-2",
						instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"2")),
					},
					expectEmpty: true,
				},
			},
		},
		{
			name: "tc9 - add 2 ops, clear key of one instance",
			add: []Item{
				{
					scKey:       "sc-1",
					instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"1")),
					op: OpInfo{
						Name: "op-1",
						Type: InstanceCreate,
					},
				},
				{
					scKey:       "sc-2",
					instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"2")),
					op: OpInfo{
						Name: "op-2",
						Type: InstanceUpdate,
					},
				},
			},
			clear: []ClearItem{
				{
					item: Item{
						scKey:       "sc-2",
						instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"2")),
						op: OpInfo{
							Name: "op-2",
							Type: InstanceUpdate,
						},
					},
					clearKey: true,
				},
			},
			output: []OutputItem{
				{
					item: Item{
						scKey:       "sc-1",
						instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"1")),
						op: OpInfo{
							Name: "op-1",
							Type: InstanceCreate,
						},
					},
				},
				{
					item: Item{
						scKey:       "sc-2",
						instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"2")),
						op: OpInfo{
							Name: "op-2",
							Type: InstanceUpdate,
						},
					},
					expectEmpty: true,
				},
			},
		},
		{
			name: "tc10 - clear key attempt for non-existent sc",
			add: []Item{
				{
					scKey:       "sc-1",
					instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"1")),
					op: OpInfo{
						Name: "op-1",
						Type: InstanceCreate,
					},
				},
				{
					scKey:       "sc-2",
					instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"2")),
					op: OpInfo{
						Name: "op-2",
						Type: InstanceUpdate,
					},
				},
			},
			clear: []ClearItem{
				{
					item: Item{
						scKey:       "sc-3",
						instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"2")),
						op: OpInfo{
							Name: "op-2",
							Type: InstanceUpdate,
						},
					},
					clearKey: true,
				},
			},
			output: []OutputItem{
				{
					item: Item{
						scKey:       "sc-1",
						instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"1")),
						op: OpInfo{
							Name: "op-1",
							Type: InstanceCreate,
						},
					},
				},
				{
					item: Item{
						scKey:       "sc-2",
						instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"2")),
						op: OpInfo{
							Name: "op-2",
							Type: InstanceUpdate,
						},
					},
				},
			},
		},
		{
			name: "tc11 - clear non-existent instance key for existing sc",
			add: []Item{
				{
					scKey:       "sc-1",
					instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"1")),
					op: OpInfo{
						Name: "op-1",
						Type: InstanceCreate,
					},
				},
				{
					scKey:       "sc-2",
					instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"2")),
					op: OpInfo{
						Name: "op-2",
						Type: InstanceUpdate,
					},
				},
			},
			clear: []ClearItem{
				{
					item: Item{
						scKey:       "sc-2",
						instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"3")),
						op: OpInfo{
							Name: "op-3",
							Type: InstanceUpdate,
						},
					},
					clearKey: true,
				},
			},
			output: []OutputItem{
				{
					item: Item{
						scKey:       "sc-1",
						instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"1")),
						op: OpInfo{
							Name: "op-1",
							Type: InstanceCreate,
						},
					},
				},
				{
					item: Item{
						scKey:       "sc-2",
						instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"2")),
						op: OpInfo{
							Name: "op-2",
							Type: InstanceUpdate,
						},
					},
				},
				{
					item: Item{
						scKey:       "sc-2",
						instanceKey: InstanceKey(CreateInstanceKey(testProject, testLocation, testInstance+"3")),
					},
					expectEmpty: true,
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cache := NewStorageClassInfoCache()
			for _, item := range tc.add {
				cache.AddInstanceOp(item.scKey, item.instanceKey, item.op)
			}
			for _, item := range tc.clear {
				if item.clearValue {
					err := cache.DeleteInstanceOp(item.item.scKey, item.item.instanceKey, item.item.op.Name)
					if item.expectError && err == nil {
						t.Errorf("expected error")
					}
					if !item.expectError && err != nil {
						t.Errorf("unexpected error")
					}
				}

				if item.clearKey {
					cache.DeleteInstance(item.item.scKey, item.item.instanceKey)
				}
			}

			for _, item := range tc.output {
				op := cache.GetInstanceOp(item.item.scKey, item.item.instanceKey)
				if item.expectEmpty {
					if op != nil {
						t.Errorf("want nil, got %+v", op)
					}
					continue
				}

				if item.expectEmptyValue {
					if op.Name != "" || op.Type != UnknownOp {
						t.Errorf("want empty op, got %+v", op)
					}
					continue
				}

				if op.Name != item.item.op.Name || op.Type != item.item.op.Type {
					t.Errorf("want %+v, got : %+v", item.item.op, op)
				}
			}
		})
	}
}

func TestShareOpsMap(t *testing.T) {
	tests := []struct {
		name   string
		add    []AddItem
		clear  []ClearItem
		output []OutputItem
	}{
		{
			name: "tc1 - add ops for 2 shares on same instance, for 1 storage class",
			add: []AddItem{
				{
					item: Item{
						scKey:    "sc-1",
						shareKey: CreateShareKey(testProject, testLocation, testInstance, testShare+"1"),
						op: OpInfo{
							Name: "op-1",
							Type: ShareUpdate,
						},
					},
				},
				{
					item: Item{
						scKey:    "sc-1",
						shareKey: CreateShareKey(testProject, testLocation, testInstance, testShare+"2"),
						op: OpInfo{
							Name: "op-2",
							Type: ShareDelete,
						},
					},
				},
			},
			output: []OutputItem{
				{
					item: Item{
						scKey:    "sc-1",
						shareKey: CreateShareKey(testProject, testLocation, testInstance, testShare+"1"),
						op: OpInfo{
							Name: "op-1",
							Type: ShareUpdate,
						},
					},
				},
				{
					item: Item{
						scKey:    "sc-1",
						shareKey: CreateShareKey(testProject, testLocation, testInstance, testShare+"2"),
						op: OpInfo{
							Name: "op-2",
							Type: ShareDelete,
						},
					},
				},
			},
		},
		{
			name: "tc2 - add 2 share ops on different instances, for 2 storage classes",
			add: []AddItem{
				{
					item: Item{
						scKey:    "sc-1",
						shareKey: CreateShareKey(testProject, testLocation, testInstance+"1", testShare+"1"),
						op: OpInfo{
							Name: "op-1",
							Type: ShareDelete,
						},
					},
				},
				{
					item: Item{
						scKey:    "sc-2",
						shareKey: CreateShareKey(testProject, testLocation, testInstance+"2", testShare+"2"),
						op: OpInfo{
							Name: "op-2",
							Type: ShareUpdate,
						},
					},
				},
			},
			output: []OutputItem{
				{
					item: Item{
						scKey:    "sc-1",
						shareKey: CreateShareKey(testProject, testLocation, testInstance+"1", testShare+"1"),
						op: OpInfo{
							Name: "op-1",
							Type: ShareDelete,
						},
					},
				},
				{
					item: Item{
						scKey:    "sc-2",
						shareKey: CreateShareKey(testProject, testLocation, testInstance+"2", testShare+"2"),
						op: OpInfo{
							Name: "op-2",
							Type: ShareUpdate,
						},
					},
				},
			},
		},
		{
			name: "tc3 - add 2 ops, clear 2 ops",
			add: []AddItem{
				{
					item: Item{
						scKey:    "sc-1",
						shareKey: CreateShareKey(testProject, testLocation, testInstance+"1", testShare+"1"),
						op: OpInfo{
							Name: "op-1",
							Type: ShareDelete,
						},
					},
				},
				{
					item: Item{
						scKey:    "sc-2",
						shareKey: CreateShareKey(testProject, testLocation, testInstance+"2", testShare+"2"),
						op: OpInfo{
							Name: "op-2",
							Type: ShareUpdate,
						},
					},
				},
			},
			clear: []ClearItem{
				{
					item: Item{
						scKey:    "sc-1",
						shareKey: CreateShareKey(testProject, testLocation, testInstance+"1", testShare+"1"),
						op: OpInfo{
							Name: "op-1",
						},
					},
				},
				{
					item: Item{
						scKey:    "sc-2",
						shareKey: CreateShareKey(testProject, testLocation, testInstance+"2", testShare+"2"),
						op: OpInfo{
							Name: "op-2",
						},
					},
				},
			},
		},
		{
			name: "tc4 - add 2 ops, clear 1 op",
			add: []AddItem{
				{
					item: Item{
						scKey:    "sc-1",
						shareKey: CreateShareKey(testProject, testLocation, testInstance+"1", testShare+"1"),
						op: OpInfo{
							Name: "op-1",
							Type: ShareDelete,
						},
					},
				},
				{
					item: Item{
						scKey:    "sc-2",
						shareKey: CreateShareKey(testProject, testLocation, testInstance+"2", testShare+"2"),
						op: OpInfo{
							Name: "op-2",
							Type: ShareUpdate,
						},
					},
				},
			},
			clear: []ClearItem{
				{
					item: Item{
						scKey:    "sc-2",
						shareKey: CreateShareKey(testProject, testLocation, testInstance+"2", testShare+"2"),
						op: OpInfo{
							Name: "op-2",
						},
					},
				},
			},
			output: []OutputItem{
				{
					item: Item{
						scKey:    "sc-1",
						shareKey: CreateShareKey(testProject, testLocation, testInstance+"1", testShare+"1"),
						op: OpInfo{
							Name: "op-1",
							Type: ShareDelete,
						},
					},
				},
				{
					item: Item{
						scKey:    "sc-2",
						shareKey: CreateShareKey(testProject, testLocation, testInstance+"2", testShare+"2"),
						op: OpInfo{
							Name: "op-2",
						},
					},
					expectEmpty: true,
				},
			},
		},
		{
			name: "tc5 - clear non-existent op for non-existent sc",
			add: []AddItem{
				{
					item: Item{
						scKey:    "sc-1",
						shareKey: CreateShareKey(testProject, testLocation, testInstance+"1", testShare+"1"),
						op: OpInfo{
							Name: "op-1",
							Type: ShareDelete,
						},
					},
				},
				{
					item: Item{
						scKey:    "sc-2",
						shareKey: CreateShareKey(testProject, testLocation, testInstance+"2", testShare+"2"),
						op: OpInfo{
							Name: "op-2",
							Type: ShareUpdate,
						},
					},
				},
			},
			clear: []ClearItem{
				{
					item: Item{
						scKey:    "sc-3",
						shareKey: CreateShareKey(testProject, testLocation, testInstance+"2", testShare+"2"),
						op: OpInfo{
							Name: "op-3",
						},
					},
				},
			},
			output: []OutputItem{
				{
					item: Item{
						scKey:    "sc-1",
						shareKey: CreateShareKey(testProject, testLocation, testInstance+"1", testShare+"1"),
						op: OpInfo{
							Name: "op-1",
							Type: ShareDelete,
						},
					},
				},
				{
					item: Item{
						scKey:    "sc-2",
						shareKey: CreateShareKey(testProject, testLocation, testInstance+"2", testShare+"2"),
						op: OpInfo{
							Name: "op-2",
							Type: ShareUpdate,
						},
					},
				},
			},
		},
		{
			name: "tc6 - clear non-existent op for existing sc, error expected in clear",
			add: []AddItem{
				{
					item: Item{
						scKey:    "sc-1",
						shareKey: CreateShareKey(testProject, testLocation, testInstance+"1", testShare+"1"),
						op: OpInfo{
							Name: "op-1",
							Type: ShareDelete,
						},
					},
				},
				{
					item: Item{
						scKey:    "sc-2",
						shareKey: CreateShareKey(testProject, testLocation, testInstance+"2", testShare+"2"),
						op: OpInfo{
							Name: "op-2",
							Type: ShareUpdate,
						},
					},
				},
			},
			clear: []ClearItem{
				{
					item: Item{
						scKey:    "sc-2",
						shareKey: CreateShareKey(testProject, testLocation, testInstance+"2", testShare+"2"),
						op: OpInfo{
							Name: "op-3",
						},
					},
					expectError: true,
				},
			},
			output: []OutputItem{
				{
					item: Item{
						scKey:    "sc-1",
						shareKey: CreateShareKey(testProject, testLocation, testInstance+"1", testShare+"1"),
						op: OpInfo{
							Name: "op-1",
							Type: ShareDelete,
						},
					},
				},
				{
					item: Item{
						scKey:    "sc-2",
						shareKey: CreateShareKey(testProject, testLocation, testInstance+"2", testShare+"2"),
						op: OpInfo{
							Name: "op-2",
							Type: ShareUpdate,
						},
					},
				},
			},
		},
		{
			name: "tc7 - clear non-existent share key (different share) for existing sc",
			add: []AddItem{
				{
					item: Item{
						scKey:    "sc-1",
						shareKey: CreateShareKey(testProject, testLocation, testInstance+"1", testShare+"1"),
						op: OpInfo{
							Name: "op-1",
							Type: ShareDelete,
						},
					},
				},
				{
					item: Item{
						scKey:    "sc-2",
						shareKey: CreateShareKey(testProject, testLocation, testInstance+"2", testShare+"2"),
						op: OpInfo{
							Name: "op-2",
							Type: ShareUpdate,
						},
					},
				},
			},
			clear: []ClearItem{
				{
					item: Item{
						scKey:    "sc-2",
						shareKey: CreateShareKey(testProject, testLocation, testInstance+"2", testShare+"3"),
						op: OpInfo{
							Name: "op-3",
						},
					},
				},
			},
			output: []OutputItem{
				{
					item: Item{
						scKey:    "sc-1",
						shareKey: CreateShareKey(testProject, testLocation, testInstance+"1", testShare+"1"),
						op: OpInfo{
							Name: "op-1",
							Type: ShareDelete,
						},
					},
				},
				{
					item: Item{
						scKey:    "sc-2",
						shareKey: CreateShareKey(testProject, testLocation, testInstance+"2", testShare+"2"),
						op: OpInfo{
							Name: "op-2",
							Type: ShareUpdate,
						},
					},
				},
			},
		},
		{
			name: "tc7 - clear non-existent share key (different instance) for existing sc",
			add: []AddItem{
				{
					item: Item{
						scKey:    "sc-1",
						shareKey: CreateShareKey(testProject, testLocation, testInstance+"1", testShare+"1"),
						op: OpInfo{
							Name: "op-1",
							Type: ShareDelete,
						},
					},
				},
				{
					item: Item{
						scKey:    "sc-2",
						shareKey: CreateShareKey(testProject, testLocation, testInstance+"2", testShare+"2"),
						op: OpInfo{
							Name: "op-2",
							Type: ShareUpdate,
						},
					},
				},
			},
			clear: []ClearItem{
				{
					item: Item{
						scKey:    "sc-2",
						shareKey: CreateShareKey(testProject, testLocation, testInstance+"3", testShare+"2"),
						op: OpInfo{
							Name: "op-3",
						},
					},
				},
			},
			output: []OutputItem{
				{
					item: Item{
						scKey:    "sc-1",
						shareKey: CreateShareKey(testProject, testLocation, testInstance+"1", testShare+"1"),
						op: OpInfo{
							Name: "op-1",
							Type: ShareDelete,
						},
					},
				},
				{
					item: Item{
						scKey:    "sc-2",
						shareKey: CreateShareKey(testProject, testLocation, testInstance+"2", testShare+"2"),
						op: OpInfo{
							Name: "op-2",
							Type: ShareUpdate,
						},
					},
				},
			},
		},
		{
			name: "tc8 - non-existent sc key, add op failure",
			add: []AddItem{
				{
					item: Item{
						scKey:    "sc-1",
						shareKey: CreateShareKey(testProject, testLocation, testInstance+"1", testShare+"1"),
						op: OpInfo{
							Name: "op-1",
							Type: ShareDelete,
						},
					},
					skipInitSCKey: true,
					expectError:   true,
				},
				{
					item: Item{
						scKey:    "sc-2",
						shareKey: CreateShareKey(testProject, testLocation, testInstance+"2", testShare+"2"),
						op: OpInfo{
							Name: "op-2",
							Type: ShareUpdate,
						},
					},
					skipInitSCKey: true,
					expectError:   true,
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cache := NewStorageClassInfoCache()
			for _, item := range tc.add {
				if !item.skipInitSCKey {
					if _, ok := cache.ScInfoMap[item.item.scKey]; !ok {
						cache.ScInfoMap[item.item.scKey] = StorageClassInfo{
							InstanceMap:    make(InstanceMap),
							ShareCreateMap: make(ShareCreateMap),
							ShareOpsMap:    make(ShareOpsMap),
						}
					}
				}
				err := cache.AddShareOp(item.item.scKey, item.item.shareKey, item.item.op)
				if !item.expectError && err != nil {
					t.Errorf("unexpected error")
				}
				if item.expectError && err == nil {
					t.Errorf("expected error not found")
				}
			}
			for _, item := range tc.clear {
				err := cache.DeleteShareOp(item.item.scKey, item.item.shareKey, item.item.op.Name)
				if item.expectError && err == nil {
					t.Errorf("expected error")
				}
				if !item.expectError && err != nil {
					t.Errorf("unexpected error")
				}
			}

			for _, item := range tc.output {
				op := cache.GetShareOp(item.item.scKey, item.item.shareKey)
				if item.expectEmpty {
					if op != nil {
						t.Errorf("want nil, got %+v", op)
					}
					return
				}

				if op.Name != item.item.op.Name || op.Type != item.item.op.Type {
					t.Errorf("want %+v, got : %+v", item.item.op, op)
				}
			}
		})
	}

}

func TestShareCreateOpMap(t *testing.T) {
	tests := []struct {
		name   string
		add    []AddItem
		clear  []ClearItem
		output []OutputItem
	}{
		{
			name: "tc1 - add ops for 2 shares on same instance, for 1 storage class",
			add: []AddItem{
				{
					item: Item{
						scKey:          "sc-1",
						shareCreateKey: testShare + "1",
						createOp: ShareCreateOpInfo{
							InstanceHandle: CreateInstanceKey(testProject, testLocation, testInstance),
							OpName:         "op-1",
						},
					},
				},
				{
					item: Item{
						scKey:          "sc-1",
						shareCreateKey: testShare + "2",
						createOp: ShareCreateOpInfo{
							InstanceHandle: CreateInstanceKey(testProject, testLocation, testInstance),
							OpName:         "op-2",
						},
					},
				},
			},
			output: []OutputItem{
				{
					item: Item{
						scKey:          "sc-1",
						shareCreateKey: testShare + "1",
						createOp: ShareCreateOpInfo{
							InstanceHandle: CreateInstanceKey(testProject, testLocation, testInstance),
							OpName:         "op-1",
						},
					},
				},
				{
					item: Item{
						scKey:          "sc-1",
						shareCreateKey: testShare + "2",
						createOp: ShareCreateOpInfo{
							InstanceHandle: CreateInstanceKey(testProject, testLocation, testInstance),
							OpName:         "op-2",
						},
					},
				},
			},
		},
		{
			name: "tc2 - add 2 share ops on different instances, for 2 storage classes",
			add: []AddItem{
				{
					item: Item{
						scKey:          "sc-1",
						shareCreateKey: testShare + "1",
						createOp: ShareCreateOpInfo{
							InstanceHandle: CreateInstanceKey(testProject, testLocation, testInstance+"1"),
							OpName:         "op-1",
						},
					},
				},
				{
					item: Item{
						scKey:          "sc-2",
						shareCreateKey: testShare + "2",
						createOp: ShareCreateOpInfo{
							InstanceHandle: CreateInstanceKey(testProject, testLocation, testInstance+"2"),
							OpName:         "op-2",
						},
					},
				},
			},
			output: []OutputItem{
				{
					item: Item{
						scKey:          "sc-1",
						shareCreateKey: testShare + "1",
						createOp: ShareCreateOpInfo{
							InstanceHandle: CreateInstanceKey(testProject, testLocation, testInstance+"1"),
							OpName:         "op-1",
						},
					},
				},
				{
					item: Item{
						scKey:          "sc-2",
						shareCreateKey: testShare + "2",
						createOp: ShareCreateOpInfo{
							InstanceHandle: CreateInstanceKey(testProject, testLocation, testInstance+"2"),
							OpName:         "op-2",
						},
					},
				},
			},
		},
		{
			name: "tc3 - add 2 share ops and 2 clear ops, for 2 storage classes",
			add: []AddItem{
				{
					item: Item{
						scKey:          "sc-1",
						shareCreateKey: testShare + "1",
						createOp: ShareCreateOpInfo{
							InstanceHandle: CreateInstanceKey(testProject, testLocation, testInstance+"1"),
							OpName:         "op-1",
						},
					},
				},
				{
					item: Item{
						scKey:          "sc-2",
						shareCreateKey: testShare + "2",
						createOp: ShareCreateOpInfo{
							InstanceHandle: CreateInstanceKey(testProject, testLocation, testInstance+"2"),
							OpName:         "op-2",
						},
					},
				},
			},
			clear: []ClearItem{
				{
					item: Item{
						scKey:          "sc-1",
						shareCreateKey: testShare + "1",
						createOp: ShareCreateOpInfo{
							OpName: "op-1",
						},
					},
				},
				{
					item: Item{
						scKey:          "sc-2",
						shareCreateKey: testShare + "2",
						createOp: ShareCreateOpInfo{
							OpName: "op-2",
						},
					},
				},
			},
		},
		{
			name: "tc4 - add 2 share ops and 1 clear op, for 2 storage classes",
			add: []AddItem{
				{
					item: Item{
						scKey:          "sc-1",
						shareCreateKey: testShare + "1",
						createOp: ShareCreateOpInfo{
							InstanceHandle: CreateInstanceKey(testProject, testLocation, testInstance+"1"),
							OpName:         "op-1",
						},
					},
				},
				{
					item: Item{
						scKey:          "sc-2",
						shareCreateKey: testShare + "2",
						createOp: ShareCreateOpInfo{
							InstanceHandle: CreateInstanceKey(testProject, testLocation, testInstance+"2"),
							OpName:         "op-2",
						},
					},
				},
			},
			clear: []ClearItem{
				{
					item: Item{
						scKey:          "sc-2",
						shareCreateKey: testShare + "2",
						createOp: ShareCreateOpInfo{
							OpName: "op-2",
						},
					},
				},
			},
			output: []OutputItem{
				{
					item: Item{
						scKey:          "sc-1",
						shareCreateKey: testShare + "1",
						createOp: ShareCreateOpInfo{
							InstanceHandle: CreateInstanceKey(testProject, testLocation, testInstance+"1"),
							OpName:         "op-1",
						},
					},
				},
				{
					item: Item{
						scKey:          "sc-2",
						shareCreateKey: testShare + "2",
						createOp: ShareCreateOpInfo{
							InstanceHandle: CreateInstanceKey(testProject, testLocation, testInstance+"2"),
							OpName:         "op-2",
						},
					},
					expectEmpty: true,
				},
			},
		},
		{
			name: "tc5 - clear non-existent sc",
			add: []AddItem{
				{
					item: Item{
						scKey:          "sc-1",
						shareCreateKey: testShare + "1",
						createOp: ShareCreateOpInfo{
							InstanceHandle: CreateInstanceKey(testProject, testLocation, testInstance+"1"),
							OpName:         "op-1",
						},
					},
				},
				{
					item: Item{
						scKey:          "sc-2",
						shareCreateKey: testShare + "2",
						createOp: ShareCreateOpInfo{
							InstanceHandle: CreateInstanceKey(testProject, testLocation, testInstance+"2"),
							OpName:         "op-2",
						},
					},
				},
			},
			clear: []ClearItem{
				{
					item: Item{
						scKey:          "sc-3",
						shareCreateKey: testShare + "2",
						createOp: ShareCreateOpInfo{
							OpName: "op-2",
						},
					},
				},
			},
			output: []OutputItem{
				{
					item: Item{
						scKey:          "sc-1",
						shareCreateKey: testShare + "1",
						createOp: ShareCreateOpInfo{
							InstanceHandle: CreateInstanceKey(testProject, testLocation, testInstance+"1"),
							OpName:         "op-1",
						},
					},
				},
				{
					item: Item{
						scKey:          "sc-2",
						shareCreateKey: testShare + "2",
						createOp: ShareCreateOpInfo{
							InstanceHandle: CreateInstanceKey(testProject, testLocation, testInstance+"2"),
							OpName:         "op-2",
						},
					},
				},
			},
		},
		{
			name: "tc6 - clear non-existent share key",
			add: []AddItem{
				{
					item: Item{
						scKey:          "sc-1",
						shareCreateKey: testShare + "1",
						createOp: ShareCreateOpInfo{
							InstanceHandle: CreateInstanceKey(testProject, testLocation, testInstance+"1"),
							OpName:         "op-1",
						},
					},
				},
				{
					item: Item{
						scKey:          "sc-2",
						shareCreateKey: testShare + "2",
						createOp: ShareCreateOpInfo{
							InstanceHandle: CreateInstanceKey(testProject, testLocation, testInstance+"2"),
							OpName:         "op-2",
						},
					},
				},
			},
			clear: []ClearItem{
				{
					item: Item{
						scKey:          "sc-1",
						shareCreateKey: testShare + "3",
						createOp: ShareCreateOpInfo{
							OpName: "op-3",
						},
					},
				},
			},
			output: []OutputItem{
				{
					item: Item{
						scKey:          "sc-1",
						shareCreateKey: testShare + "1",
						createOp: ShareCreateOpInfo{
							InstanceHandle: CreateInstanceKey(testProject, testLocation, testInstance+"1"),
							OpName:         "op-1",
						},
					},
				},
				{
					item: Item{
						scKey:          "sc-2",
						shareCreateKey: testShare + "2",
						createOp: ShareCreateOpInfo{
							InstanceHandle: CreateInstanceKey(testProject, testLocation, testInstance+"2"),
							OpName:         "op-2",
						},
					},
				},
			},
		},
		{
			name: "tc7 - clear mistched op for share key, error expected in clear",
			add: []AddItem{
				{
					item: Item{
						scKey:          "sc-1",
						shareCreateKey: testShare + "1",
						createOp: ShareCreateOpInfo{
							InstanceHandle: CreateInstanceKey(testProject, testLocation, testInstance+"1"),
							OpName:         "op-1",
						},
					},
				},
				{
					item: Item{
						scKey:          "sc-2",
						shareCreateKey: testShare + "2",
						createOp: ShareCreateOpInfo{
							InstanceHandle: CreateInstanceKey(testProject, testLocation, testInstance+"2"),
							OpName:         "op-2",
						},
					},
				},
			},
			clear: []ClearItem{
				{
					item: Item{
						scKey:          "sc-1",
						shareCreateKey: testShare + "1",
						createOp: ShareCreateOpInfo{
							OpName: "op-3",
						},
					},
					expectError: true,
				},
			},
			output: []OutputItem{
				{
					item: Item{
						scKey:          "sc-1",
						shareCreateKey: testShare + "1",
						createOp: ShareCreateOpInfo{
							InstanceHandle: CreateInstanceKey(testProject, testLocation, testInstance+"1"),
							OpName:         "op-1",
						},
					},
				},
				{
					item: Item{
						scKey:          "sc-2",
						shareCreateKey: testShare + "2",
						createOp: ShareCreateOpInfo{
							InstanceHandle: CreateInstanceKey(testProject, testLocation, testInstance+"2"),
							OpName:         "op-2",
						},
					},
				},
			},
		},
		{
			name: "tc8 - missing sc key, add op failure",
			add: []AddItem{
				{
					item: Item{
						scKey:          "sc-1",
						shareCreateKey: testShare + "1",
						createOp: ShareCreateOpInfo{
							InstanceHandle: CreateInstanceKey(testProject, testLocation, testInstance+"1"),
							OpName:         "op-1",
						},
					},
					skipInitSCKey: true,
					expectError:   true,
				},
				{
					item: Item{
						scKey:          "sc-2",
						shareCreateKey: testShare + "2",
						createOp: ShareCreateOpInfo{
							InstanceHandle: CreateInstanceKey(testProject, testLocation, testInstance+"2"),
							OpName:         "op-2",
						},
					},
					skipInitSCKey: true,
					expectError:   true,
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cache := NewStorageClassInfoCache()
			for _, item := range tc.add {
				if !item.skipInitSCKey {
					if _, ok := cache.ScInfoMap[item.item.scKey]; !ok {
						cache.ScInfoMap[item.item.scKey] = StorageClassInfo{
							InstanceMap:    make(InstanceMap),
							ShareCreateMap: make(ShareCreateMap),
							ShareOpsMap:    make(ShareOpsMap),
						}
					}
				}
				err := cache.AddShareCreateOp(item.item.scKey, item.item.shareCreateKey, item.item.createOp)
				if !item.expectError && err != nil {
					t.Errorf("unexpected error")
				}
				if item.expectError && err == nil {
					t.Errorf("expected error not found")
				}
			}
			for _, item := range tc.clear {
				err := cache.DeleteShareCreateOp(item.item.scKey, item.item.shareCreateKey, item.item.createOp.OpName)
				if item.expectError && err == nil {
					t.Errorf("expected error")
				}
				if !item.expectError && err != nil {
					t.Errorf("unexpected error")
				}
			}

			for _, item := range tc.output {
				op := cache.GetShareCreateOp(item.item.scKey, item.item.shareCreateKey)
				if item.expectEmpty {
					if op != nil {
						t.Errorf("want nil, got %+v", op)
					}
					return
				}

				if op.OpName != item.item.createOp.OpName || op.InstanceHandle != item.item.createOp.InstanceHandle {
					t.Errorf("want %+v, got : %+v", item.item.createOp, op)
				}
			}
		})
	}

}

func TestCreateKey(t *testing.T) {
	tests := []struct {
		name                string
		project             string
		location            string
		instance            string
		share               string
		expectedShareKey    string
		expectedInstanceKey string
	}{
		{
			name:             "tc1",
			project:          testProject,
			location:         testLocation,
			instance:         testInstance,
			share:            testShare,
			expectedShareKey: fmt.Sprintf("%s/%s/%s/%s", testProject, testLocation, testInstance, testShare),
		},
		{
			name:                "tc2",
			project:             testProject,
			location:            testLocation,
			instance:            testInstance,
			expectedInstanceKey: fmt.Sprintf("%s/%s/%s", testProject, testLocation, testInstance),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.expectedShareKey != "" {
				k := CreateShareKey(tc.project, tc.location, tc.instance, tc.share)
				if string(k) != tc.expectedShareKey {
					t.Errorf("want %v, got %v", tc.expectedShareKey, k)
				}
			}
			if tc.expectedInstanceKey != "" {
				k := CreateInstanceKey(tc.project, tc.location, tc.instance)
				if string(k) != tc.expectedInstanceKey {
					t.Errorf("want %v, got %v", tc.expectedInstanceKey, k)
				}
			}
		})
	}
}
