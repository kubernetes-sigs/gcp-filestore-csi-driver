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

package driver

import (
	"encoding/json"
	"fmt"
	"testing"

	"golang.org/x/net/context"
	filev1beta1multishare "google.golang.org/api/file/v1beta1multishare"
	cloud "sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/file"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/util"
)

const (
	testInstanceScPrefix = "testinstancescprefix"
	testInstanceName     = "testInstanceName"
	testShareName        = "testShareName"
)

var (
	testInstanceHandle = util.CreateInstanceKey(testProject, testRegion, testInstanceName)
)

type Item struct {
	scKey          string
	instanceKey    util.InstanceKey
	shareKey       util.ShareKey
	shareCreateKey string
	op             util.OpInfo
	createOp       util.ShareCreateOpInfo
}

func initCloudProviderWithBlockingFileService(t *testing.T, opUnblocker chan chan file.Signal) *cloud.Cloud {
	fbs, err := file.NewFakeBlockingServiceForMultishare(opUnblocker)
	if err != nil {
		t.Errorf("failed to initialize blocking GCFS service: %v", err)
	}

	cloudProvider, err := cloud.NewFakeCloudWithFiler(fbs, testProject, testLocation)
	if err != nil {
		t.Errorf("failed to initialize blocking GCFS service: %v", err)
	}
	return cloudProvider
}

type MockOpStatus struct {
	reportRunning           bool
	reportError             bool
	reportNotFoundError     bool
	reportOpWithErrorStatus bool
}

type Response struct {
	opStatus             util.OperationStatus
	createOp             *util.ShareCreateOpInfo
	shareOp              *util.OpInfo
	verified             bool
	readyInstances       []*file.MultishareInstance
	numNonReadyInstances int
	instanceNeedsExpand  bool
	targetBytes          int64
	err                  error
}

func instanceTarget(project, location, instanceName string) string {
	return fmt.Sprintf("projects/%s/locations/%s/instances/%s", project, location, instanceName)
}

func shareTarget(instanceTarget, share string) string {
	return fmt.Sprintf("%s/shares/%s", instanceTarget, share)
}

func genOp(opName, target, verb string, done bool) filev1beta1multishare.Operation {
	meta, _ := json.Marshal(filev1beta1multishare.OperationMetadata{
		Verb:   verb,
		Target: target,
	})
	return filev1beta1multishare.Operation{
		Done:     done,
		Name:     opName,
		Metadata: meta,
	}
}

func TestPopulateCache(t *testing.T) {
	singleInstance := file.MultishareInstance{
		Project:  testProject,
		Location: testLocation,
		Name:     testInstanceName,
		Tier:     enterpriseTier,
	}
	createSingleInstanceMeta, _ := json.Marshal(filev1beta1multishare.OperationMetadata{
		Verb:   util.VerbCreate,
		Target: instanceTarget(testProject, testLocation, testInstanceName),
	})
	createSingleInstance := filev1beta1multishare.Operation{
		Done:     false,
		Name:     "createSingleInstance",
		Metadata: createSingleInstanceMeta,
	}
	multishareInstance1 := file.MultishareInstance{
		Project:  testProject,
		Location: testLocation,
		Name:     util.NewMultishareInstancePrefix + testInstanceName + "-1",
		Tier:     enterpriseTier,
		Labels: map[string]string{
			util.ParamMultishareInstanceScLabelKey: testInstanceScPrefix,
		},
	}
	targetMultishareInstance1 := instanceTarget(testProject, testLocation, multishareInstance1.Name)
	createMultishareInstance1 := genOp("createMultishareInstance1", targetMultishareInstance1, util.VerbCreate, false)
	deleteMultishareInstance1 := genOp("deleteMultishareInstance1", targetMultishareInstance1, util.VerbDelete, false)
	deleteMultishareInstance1Done := genOp("deleteMultishareInstance1", targetMultishareInstance1, util.VerbDelete, true)
	updateMultishareInstance1 := genOp("updateMultishareInstance1", targetMultishareInstance1, util.VerbUpdate, false)
	updateMultishareInstance1Done := genOp("updateMultishareInstance1", targetMultishareInstance1, util.VerbUpdate, true)
	instanceKey1 := util.CreateInstanceKey(multishareInstance1.Project, multishareInstance1.Location, multishareInstance1.Name)

	shareName1 := testShareName + "-1"
	targetShare1 := shareTarget(targetMultishareInstance1, shareName1)
	shareKey1 := util.CreateShareKey(multishareInstance1.Project, multishareInstance1.Location, multishareInstance1.Name, shareName1)
	createShare1 := genOp("createShare1", targetShare1, util.VerbCreate, false)
	createShare1Done := genOp("createShare1", targetShare1, util.VerbCreate, true)
	updateShare1 := genOp("updateShare1", targetShare1, util.VerbUpdate, false)
	updateShare1Done := genOp("updateShare1", targetShare1, util.VerbUpdate, true)
	deleteShare1 := genOp("deleteShare1", targetShare1, util.VerbDelete, false)

	tests := []struct {
		name               string
		initInstances      []file.MultishareInstance
		initOps            []*filev1beta1multishare.Operation
		desiredScInfoMap   map[string]util.StorageClassInfo
		desiredCreateStage util.InstanceMap
	}{
		{
			name: "basic case, no Ops",
			initInstances: []file.MultishareInstance{
				singleInstance,
				multishareInstance1,
			},
			initOps: []*filev1beta1multishare.Operation{},
			desiredScInfoMap: map[string]util.StorageClassInfo{
				testInstanceScPrefix: {
					InstanceMap: util.InstanceMap{
						instanceKey1: util.DummyOp(),
					},
					ShareCreateMap: make(util.ShareCreateMap),
					ShareOpsMap:    make(util.ShareOpsMap),
				},
			},
			desiredCreateStage: make(util.InstanceMap),
		},
		{
			name:             "no existing instance, single and multishare instance create Ops",
			initInstances:    []file.MultishareInstance{},
			initOps:          []*filev1beta1multishare.Operation{&createSingleInstance, &createMultishareInstance1},
			desiredScInfoMap: map[string]util.StorageClassInfo{},
			desiredCreateStage: util.InstanceMap{
				instanceKey1: util.OpInfo{Name: createMultishareInstance1.Name, Type: util.InstanceCreate},
			},
		},
		{
			name: "instanceDelete done on non-existing instance",
			initInstances: []file.MultishareInstance{
				singleInstance,
			},
			initOps: []*filev1beta1multishare.Operation{
				&deleteMultishareInstance1Done,
			},
			desiredScInfoMap:   map[string]util.StorageClassInfo{},
			desiredCreateStage: util.InstanceMap{},
		},
		{
			name: "instanceDelete done on existing instance",
			initInstances: []file.MultishareInstance{
				singleInstance,
				multishareInstance1,
			},
			initOps: []*filev1beta1multishare.Operation{
				&deleteMultishareInstance1Done,
			},
			desiredScInfoMap:   map[string]util.StorageClassInfo{testInstanceScPrefix: util.NewStorageClassInfo()},
			desiredCreateStage: util.InstanceMap{},
		},
		{
			name: "instanceDelete on existing instance",
			initInstances: []file.MultishareInstance{
				singleInstance,
				multishareInstance1,
			},
			initOps: []*filev1beta1multishare.Operation{
				&deleteMultishareInstance1,
			},
			desiredScInfoMap: map[string]util.StorageClassInfo{
				testInstanceScPrefix: {
					InstanceMap: util.InstanceMap{
						instanceKey1: {Name: deleteMultishareInstance1.Name, Type: util.InstanceDelete},
					},
					ShareCreateMap: make(util.ShareCreateMap),
					ShareOpsMap:    make(util.ShareOpsMap),
				},
			},
			desiredCreateStage: util.InstanceMap{},
		},
		{
			name: "instanceUpdate on instance",
			initInstances: []file.MultishareInstance{
				singleInstance,
				multishareInstance1,
			},
			initOps: []*filev1beta1multishare.Operation{
				&updateMultishareInstance1,
			},
			desiredScInfoMap: map[string]util.StorageClassInfo{
				testInstanceScPrefix: {
					InstanceMap: util.InstanceMap{
						instanceKey1: {Name: updateMultishareInstance1.Name, Type: util.InstanceUpdate},
					},
					ShareCreateMap: make(util.ShareCreateMap),
					ShareOpsMap:    make(util.ShareOpsMap),
				},
			},
			desiredCreateStage: util.InstanceMap{},
		},
		{
			name: "instanceUpdate done on instance",
			initInstances: []file.MultishareInstance{
				singleInstance,
				multishareInstance1,
			},
			initOps: []*filev1beta1multishare.Operation{
				&updateMultishareInstance1Done,
			},
			desiredScInfoMap: map[string]util.StorageClassInfo{
				testInstanceScPrefix: {
					InstanceMap: util.InstanceMap{
						instanceKey1: util.DummyOp(),
					},
					ShareCreateMap: make(util.ShareCreateMap),
					ShareOpsMap:    make(util.ShareOpsMap),
				},
			},
			desiredCreateStage: util.InstanceMap{},
		},
		{
			name: "shareCreate on instance",
			initInstances: []file.MultishareInstance{
				multishareInstance1,
			},
			initOps: []*filev1beta1multishare.Operation{
				&createShare1,
			},
			desiredScInfoMap: map[string]util.StorageClassInfo{
				testInstanceScPrefix: {
					InstanceMap: util.InstanceMap{
						instanceKey1: util.DummyOp(),
					},
					ShareCreateMap: util.ShareCreateMap{
						shareName1: util.ShareCreateOpInfo{InstanceHandle: instanceKey1, OpName: createShare1.Name},
					},
					ShareOpsMap: make(util.ShareOpsMap),
				},
			},
			desiredCreateStage: util.InstanceMap{},
		},
		{
			name: "shareCreate done on instance",
			initInstances: []file.MultishareInstance{
				multishareInstance1,
			},
			initOps: []*filev1beta1multishare.Operation{
				&createShare1Done,
			},
			desiredScInfoMap: map[string]util.StorageClassInfo{
				testInstanceScPrefix: {
					InstanceMap: util.InstanceMap{
						instanceKey1: util.DummyOp(),
					},
					ShareCreateMap: make(util.ShareCreateMap),
					ShareOpsMap:    make(util.ShareOpsMap),
				},
			},
			desiredCreateStage: util.InstanceMap{},
		},
		{
			name: "shareUpdate on instance",
			initInstances: []file.MultishareInstance{
				multishareInstance1,
			},
			initOps: []*filev1beta1multishare.Operation{
				&updateShare1,
				&createShare1Done,
			},
			desiredScInfoMap: map[string]util.StorageClassInfo{
				testInstanceScPrefix: {
					InstanceMap: util.InstanceMap{
						instanceKey1: util.DummyOp(),
					},
					ShareCreateMap: make(util.ShareCreateMap),
					ShareOpsMap: util.ShareOpsMap{
						shareKey1: util.OpInfo{Name: updateShare1.Name, Type: util.ShareUpdate},
					},
				},
			},
			desiredCreateStage: util.InstanceMap{},
		},
		{
			name: "shareUpdate done followed by shareDelete",
			initInstances: []file.MultishareInstance{
				multishareInstance1,
			},
			initOps: []*filev1beta1multishare.Operation{
				&deleteShare1,
				&updateShare1Done,
			},
			desiredScInfoMap: map[string]util.StorageClassInfo{
				testInstanceScPrefix: {
					InstanceMap: util.InstanceMap{
						instanceKey1: util.DummyOp(),
					},
					ShareCreateMap: make(util.ShareCreateMap),
					ShareOpsMap: util.ShareOpsMap{
						shareKey1: util.OpInfo{Name: deleteShare1.Name, Type: util.ShareDelete},
					},
				},
			},
			desiredCreateStage: util.InstanceMap{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fbs := file.NewFakeServiceForMultishareWithState(test.initInstances, test.initOps)

			cloudProvider, err := cloud.NewFakeCloudWithFiler(fbs, testProject, testLocation)
			if err != nil {
				t.Errorf("failed to initialize blocking GCFS service: %v", err)
			}
			manager := NewMultishareOpsManager(cloudProvider)

			manager.populateCache()

			actualScInfoMap := manager.cache.ScInfoMap
			if len(test.desiredScInfoMap) != len(actualScInfoMap) {
				t.Fatalf("Cache state not match, EXPECTED: %v ACTUAL:%v", test.desiredScInfoMap, actualScInfoMap)
			}
			for scKey, scInfo := range manager.cache.ScInfoMap {
				desiredScInfo, ok := test.desiredScInfoMap[scKey]
				if !ok {
					t.Errorf("Cache state not match, EXPECTED: %v ACTUAL:%v", test.desiredScInfoMap, actualScInfoMap)
				}
				if !scInfo.Equals(desiredScInfo) {
					t.Errorf("Cache state not match, EXPECTED: %v ACTUAL:%v", test.desiredScInfoMap, actualScInfoMap)
				}
			}
			if !test.desiredCreateStage.Equals(*manager.createStaging) {
				t.Errorf("Cache state not match, EXPECTED: %v ACTUAL:%v", test.desiredCreateStage, *manager.createStaging)
			}
		})
	}
}

func TestCheckAndUpdateShareCreateOp(t *testing.T) {
	tests := []struct {
		name                      string
		scKey                     string
		shareName                 string
		initShareCreateOpMap      []Item
		signalGetOp               bool
		signalIsOpDone            bool
		getOpStatus               *MockOpStatus
		isOpDoneStatus            *MockOpStatus
		expectedShareCreateOpInfo *util.ShareCreateOpInfo
		expectedOpStaus           util.OperationStatus
		shareKeyExpectedInCache   bool
	}{
		{
			name:            "tc1 - no share create op in cache, unknown op status",
			scKey:           testInstanceScPrefix,
			shareName:       testShareName,
			expectedOpStaus: util.StatusUnknown,
		},
		{
			name:      "tc2 - get share create op returns error, unknown op status, cache entry not cleared",
			scKey:     testInstanceScPrefix,
			shareName: testShareName,
			initShareCreateOpMap: []Item{
				{
					scKey:          testInstanceScPrefix,
					shareCreateKey: testShareName,
					createOp: util.ShareCreateOpInfo{
						InstanceHandle: testInstanceHandle,
						OpName:         "op-1",
					},
				},
			},
			signalGetOp: true,
			getOpStatus: &MockOpStatus{
				reportError: true,
			},
			expectedOpStaus: util.StatusUnknown,
			expectedShareCreateOpInfo: &util.ShareCreateOpInfo{
				InstanceHandle: testInstanceHandle,
				OpName:         "op-1",
			},
			shareKeyExpectedInCache: true,
		},
		{
			name:      "tc3 - IsOpDone returns error, return failed op status, cache entry cleared",
			scKey:     testInstanceScPrefix,
			shareName: testShareName,
			initShareCreateOpMap: []Item{
				{
					scKey:          testInstanceScPrefix,
					shareCreateKey: testShareName,
					createOp: util.ShareCreateOpInfo{
						InstanceHandle: testInstanceHandle,
						OpName:         "op-1",
					},
				},
			},
			signalGetOp:    true,
			signalIsOpDone: true,
			isOpDoneStatus: &MockOpStatus{
				reportError: true,
			},
			expectedOpStaus: util.StatusFailed,
			expectedShareCreateOpInfo: &util.ShareCreateOpInfo{
				InstanceHandle: testInstanceHandle,
				OpName:         "op-1",
			},
		},
		{
			name:      "tc4 - IsOpDone false, return running op status, cache entry not cleared",
			scKey:     testInstanceScPrefix,
			shareName: testShareName,
			initShareCreateOpMap: []Item{
				{
					scKey:          testInstanceScPrefix,
					shareCreateKey: testShareName,
					createOp: util.ShareCreateOpInfo{
						InstanceHandle: testInstanceHandle,
						OpName:         "op-1",
					},
				},
			},
			signalGetOp:    true,
			signalIsOpDone: true,
			isOpDoneStatus: &MockOpStatus{
				reportRunning: true,
			},
			expectedOpStaus: util.StatusRunning,
			expectedShareCreateOpInfo: &util.ShareCreateOpInfo{
				InstanceHandle: testInstanceHandle,
				OpName:         "op-1",
			},
			shareKeyExpectedInCache: true,
		},
		{
			name:      "tc4 - IsOpDone true, return success op status, cache entry cleared",
			scKey:     testInstanceScPrefix,
			shareName: testShareName,
			initShareCreateOpMap: []Item{
				{
					scKey:          testInstanceScPrefix,
					shareCreateKey: testShareName,
					createOp: util.ShareCreateOpInfo{
						InstanceHandle: testInstanceHandle,
						OpName:         "op-1",
					},
				},
			},
			signalGetOp:     true,
			signalIsOpDone:  true,
			expectedOpStaus: util.StatusSuccess,
			expectedShareCreateOpInfo: &util.ShareCreateOpInfo{
				InstanceHandle: testInstanceHandle,
				OpName:         "op-1",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opUnblocker := make(chan chan file.Signal)
			cloudProvider := initCloudProviderWithBlockingFileService(t, opUnblocker)
			manager := NewMultishareOpsManager(cloudProvider)

			runRequest := func(ctx context.Context, instanceSCPrefix, shareName string) <-chan Response {
				responseChannel := make(chan Response)
				go func() {
					op, status, err := manager.checkAndUpdateShareCreateOp(ctx, instanceSCPrefix, shareName)
					responseChannel <- Response{
						opStatus: status,
						createOp: op,
						err:      err,
					}
				}()
				return responseChannel
			}

			// Pre-populate the cache.
			for _, v := range tc.initShareCreateOpMap {
				if _, ok := manager.cache.ScInfoMap[v.scKey]; !ok {
					manager.cache.ScInfoMap[v.scKey] = util.NewStorageClassInfo()
				}
				err := manager.cache.AddShareCreateOp(v.scKey, v.shareCreateKey, v.createOp)
				if err != nil {
					t.Errorf("failed to add share create op")
				}
			}

			respChannel := runRequest(context.Background(), tc.scKey, tc.shareName)

			// Inject mock response
			if tc.signalGetOp {
				s := file.Signal{}
				if tc.getOpStatus != nil {
					s.ReportError = tc.getOpStatus.reportError
					s.ReportOpWithErrorStatus = tc.getOpStatus.reportOpWithErrorStatus
					s.ReportRunning = tc.getOpStatus.reportRunning
				}
				execute := <-opUnblocker
				execute <- s
			}

			// Inject mock response
			if tc.signalIsOpDone {
				s := file.Signal{}
				if tc.isOpDoneStatus != nil {
					s.ReportError = tc.isOpDoneStatus.reportError
					s.ReportOpWithErrorStatus = tc.isOpDoneStatus.reportOpWithErrorStatus
					s.ReportRunning = tc.isOpDoneStatus.reportRunning
				}
				execute := <-opUnblocker
				execute <- s
			}

			// Verify response
			response := <-respChannel
			if response.opStatus != tc.expectedOpStaus {
				t.Errorf("op status want %v, got %v", tc.expectedOpStaus, response.opStatus)
			}
			if tc.expectedShareCreateOpInfo == nil && response.createOp != nil {
				t.Errorf("unexpected share create op found")
			}

			if tc.expectedShareCreateOpInfo != nil && response.createOp == nil {
				t.Errorf("expected share create op not found")
			}

			if tc.expectedShareCreateOpInfo != nil && response.createOp != nil {
				if tc.expectedShareCreateOpInfo.InstanceHandle != response.createOp.InstanceHandle {
					t.Errorf("want %v, got %v", tc.expectedShareCreateOpInfo.InstanceHandle, response.createOp.InstanceHandle)
				}
				if tc.expectedShareCreateOpInfo.OpName != response.createOp.OpName {
					t.Errorf("want %v, got %v", tc.expectedShareCreateOpInfo.OpName, response.createOp.OpName)
				}
			}

			// Verify cache content
			shareCreateOp := manager.cache.GetShareCreateOp(tc.scKey, tc.shareName)
			if tc.shareKeyExpectedInCache && shareCreateOp == nil {
				t.Errorf("expcted share key not found")
			}
			if !tc.shareKeyExpectedInCache && shareCreateOp != nil {
				t.Errorf("unexpcted share key found")
			}
		})
	}
}

func TestCheckAndUpdateShareOp(t *testing.T) {
	testShare := &file.Share{
		Name: testShareName,
		Parent: &file.MultishareInstance{
			Project:  testProject,
			Location: testRegion,
			Name:     testInstanceName,
		},
	}
	tests := []struct {
		name                    string
		scKey                   string
		shareKey                util.ShareKey
		share                   *file.Share
		initShareOpMap          []Item
		signalGetOp             bool
		signalIsOpDone          bool
		getOpStatus             *MockOpStatus
		isOpDoneStatus          *MockOpStatus
		expectedShareOpInfo     *util.OpInfo
		expectedOpStaus         util.OperationStatus
		shareKeyExpectedInCache bool
	}{
		{
			name:            "tc1 - no share op in cache, unknown op status",
			scKey:           testInstanceScPrefix,
			share:           testShare,
			expectedOpStaus: util.StatusUnknown,
		},
		{
			name:     "tc2 - get share op returns error, unknown op status, cache entry not cleared",
			scKey:    testInstanceScPrefix,
			share:    testShare,
			shareKey: util.CreateShareKey(testProject, testRegion, testInstanceName, testShareName),
			initShareOpMap: []Item{
				{
					scKey:    testInstanceScPrefix,
					shareKey: util.CreateShareKey(testProject, testRegion, testInstanceName, testShareName),
					op: util.OpInfo{
						Name: "op-1",
						Type: util.ShareDelete,
					},
				},
			},
			signalGetOp: true,
			getOpStatus: &MockOpStatus{
				reportError: true,
			},
			expectedOpStaus: util.StatusUnknown,
			expectedShareOpInfo: &util.OpInfo{
				Name: "op-1",
				Type: util.ShareDelete,
			},
			shareKeyExpectedInCache: true,
		},
		{
			name:     "tc3 - IsOpDone returns error, return failed op status, cache entry cleared",
			scKey:    testInstanceScPrefix,
			share:    testShare,
			shareKey: util.CreateShareKey(testProject, testRegion, testInstanceName, testShareName),
			initShareOpMap: []Item{
				{
					scKey:    testInstanceScPrefix,
					shareKey: util.CreateShareKey(testProject, testRegion, testInstanceName, testShareName),
					op: util.OpInfo{
						Name: "op-1",
						Type: util.ShareDelete,
					},
				},
			},
			signalGetOp:    true,
			signalIsOpDone: true,
			isOpDoneStatus: &MockOpStatus{
				reportError: true,
			},
			expectedOpStaus: util.StatusFailed,
			expectedShareOpInfo: &util.OpInfo{
				Name: "op-1",
				Type: util.ShareDelete,
			},
		},
		{
			name:     "tc4 - IsOpDone false, return running op status, cache entry not cleared",
			scKey:    testInstanceScPrefix,
			share:    testShare,
			shareKey: util.CreateShareKey(testProject, testRegion, testInstanceName, testShareName),
			initShareOpMap: []Item{
				{
					scKey:    testInstanceScPrefix,
					shareKey: util.CreateShareKey(testProject, testRegion, testInstanceName, testShareName),
					op: util.OpInfo{
						Name: "op-1",
						Type: util.ShareDelete,
					},
				},
			},
			signalGetOp:    true,
			signalIsOpDone: true,
			isOpDoneStatus: &MockOpStatus{
				reportRunning: true,
			},
			expectedOpStaus: util.StatusRunning,
			expectedShareOpInfo: &util.OpInfo{
				Name: "op-1",
				Type: util.ShareDelete,
			},
			shareKeyExpectedInCache: true,
		},
		{
			name:     "tc4 - IsOpDone true, return success op status, cache entry cleared",
			scKey:    testInstanceScPrefix,
			share:    testShare,
			shareKey: util.CreateShareKey(testProject, testRegion, testInstanceName, testShareName),
			initShareOpMap: []Item{
				{
					scKey:    testInstanceScPrefix,
					shareKey: util.CreateShareKey(testProject, testRegion, testInstanceName, testShareName),
					op: util.OpInfo{
						Name: "op-1",
						Type: util.ShareDelete,
					},
				},
			},
			signalGetOp:     true,
			signalIsOpDone:  true,
			expectedOpStaus: util.StatusSuccess,
			expectedShareOpInfo: &util.OpInfo{
				Name: "op-1",
				Type: util.ShareDelete,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opUnblocker := make(chan chan file.Signal)
			cloudProvider := initCloudProviderWithBlockingFileService(t, opUnblocker)
			manager := NewMultishareOpsManager(cloudProvider)

			runRequest := func(ctx context.Context, instanceSCPrefix string, share *file.Share) <-chan Response {
				responseChannel := make(chan Response)
				go func() {
					op, status, err := manager.checkAndUpdateShareOp(ctx, instanceSCPrefix, share)
					responseChannel <- Response{
						opStatus: status,
						shareOp:  op,
						err:      err,
					}
				}()
				return responseChannel
			}

			// Pre-populate the cache.
			for _, v := range tc.initShareOpMap {
				if _, ok := manager.cache.ScInfoMap[v.scKey]; !ok {
					manager.cache.ScInfoMap[v.scKey] = util.NewStorageClassInfo()
				}
				err := manager.cache.AddShareOp(v.scKey, v.shareKey, v.op)
				if err != nil {
					t.Errorf("failed to add share op to map")
				}
			}

			respChannel := runRequest(context.Background(), tc.scKey, tc.share)

			// Inject mock response
			if tc.signalGetOp {
				s := file.Signal{}
				if tc.getOpStatus != nil {
					s.ReportError = tc.getOpStatus.reportError
					s.ReportOpWithErrorStatus = tc.getOpStatus.reportOpWithErrorStatus
					s.ReportRunning = tc.getOpStatus.reportRunning
				}
				execute := <-opUnblocker
				execute <- s
			}

			// Inject mock response
			if tc.signalIsOpDone {
				s := file.Signal{}
				if tc.isOpDoneStatus != nil {
					s.ReportError = tc.isOpDoneStatus.reportError
					s.ReportOpWithErrorStatus = tc.isOpDoneStatus.reportOpWithErrorStatus
					s.ReportRunning = tc.isOpDoneStatus.reportRunning
				}
				execute := <-opUnblocker
				execute <- s
			}

			// Verify response
			response := <-respChannel
			if response.opStatus != tc.expectedOpStaus {
				t.Errorf("op status want %v, got %v", tc.expectedOpStaus, response.opStatus)
			}
			if tc.expectedShareOpInfo == nil && response.shareOp != nil {
				t.Errorf("unexpected share create op found")
			}

			if tc.expectedShareOpInfo != nil && response.shareOp == nil {
				t.Errorf("expected share create op not found")
			}

			if tc.expectedShareOpInfo != nil && response.shareOp != nil {
				if tc.expectedShareOpInfo.Name != response.shareOp.Name {
					t.Errorf("want %v, got %v", tc.expectedShareOpInfo.Name, response.shareOp.Name)
				}
				if tc.expectedShareOpInfo.Type != response.shareOp.Type {
					t.Errorf("want %v, got %v", tc.expectedShareOpInfo.Type, response.shareOp.Type)
				}
			}

			// Verify cache content
			shareOp := manager.cache.GetShareOp(tc.scKey, tc.shareKey)
			if tc.shareKeyExpectedInCache && shareOp == nil {
				t.Errorf("expcted share key not found")
			}
			if !tc.shareKeyExpectedInCache && shareOp != nil {
				t.Errorf("unexpcted share key found")
			}
		})
	}
}

func TestVerifyNoRunningInstanceOps(t *testing.T) {
	tests := []struct {
		name                       string
		scKey                      string
		instanceKey                util.InstanceKey
		instance                   *file.MultishareInstance
		initInstanceOpMap          []Item
		signalGetOp                bool
		signalIsOpDone             bool
		getOpStatus                *MockOpStatus
		isOpDoneStatus             *MockOpStatus
		errorExpected              bool
		expectedVerificationStaus  bool
		instanceKeyExpectedInCache bool
		emptyOpExpected            bool
	}{
		{
			name:        "tc1 - no instance op in cache, verified",
			scKey:       testInstanceScPrefix,
			instanceKey: util.CreateInstanceKey(testProject, testRegion, testInstanceName),
			instance: &file.MultishareInstance{
				Project:  testProject,
				Location: testRegion,
				Name:     testInstanceName,
			},
			expectedVerificationStaus: true,
		},
		{
			name:        "tc2 - get instance op returns empty op, verified",
			scKey:       testInstanceScPrefix,
			instanceKey: util.CreateInstanceKey(testProject, testRegion, testInstanceName),
			instance: &file.MultishareInstance{
				Project:  testProject,
				Location: testRegion,
				Name:     testInstanceName,
			},
			initInstanceOpMap: []Item{
				{
					scKey:       testInstanceScPrefix,
					instanceKey: util.CreateInstanceKey(testProject, testRegion, testInstanceName),
					op:          util.OpInfo{},
				},
			},
			instanceKeyExpectedInCache: true,
			emptyOpExpected:            true,
			expectedVerificationStaus:  true,
		},

		{
			name:        "tc3 - get instance op returns error, status not verified",
			scKey:       testInstanceScPrefix,
			instanceKey: util.CreateInstanceKey(testProject, testRegion, testInstanceName),
			instance: &file.MultishareInstance{
				Project:  testProject,
				Location: testRegion,
				Name:     testInstanceName,
			},
			initInstanceOpMap: []Item{
				{
					scKey:       testInstanceScPrefix,
					instanceKey: util.CreateInstanceKey(testProject, testRegion, testInstanceName),
					op: util.OpInfo{
						Name: "op-1",
						Type: util.InstanceCreate,
					},
				},
			},
			signalGetOp: true,
			getOpStatus: &MockOpStatus{
				reportError: true,
			},
			errorExpected:              true,
			instanceKeyExpectedInCache: true,
		},
		{
			name:        "tc4 - IsOpDone returns error for completeed op, status verified",
			scKey:       testInstanceScPrefix,
			instanceKey: util.CreateInstanceKey(testProject, testRegion, testInstanceName),
			instance: &file.MultishareInstance{
				Project:  testProject,
				Location: testRegion,
				Name:     testInstanceName,
			},
			initInstanceOpMap: []Item{
				{
					scKey:       testInstanceScPrefix,
					instanceKey: util.CreateInstanceKey(testProject, testRegion, testInstanceName),
					op: util.OpInfo{
						Name: "op-1",
						Type: util.InstanceCreate,
					},
				},
			},
			signalGetOp:    true,
			signalIsOpDone: true,
			isOpDoneStatus: &MockOpStatus{
				reportError: true,
			},
			instanceKeyExpectedInCache: true,
			expectedVerificationStaus:  true,
		},
		{
			name:        "tc5 - IsOpDone returns false, status not verified",
			scKey:       testInstanceScPrefix,
			instanceKey: util.CreateInstanceKey(testProject, testRegion, testInstanceName),
			instance: &file.MultishareInstance{
				Project:  testProject,
				Location: testRegion,
				Name:     testInstanceName,
			},
			initInstanceOpMap: []Item{
				{
					scKey:       testInstanceScPrefix,
					instanceKey: util.CreateInstanceKey(testProject, testRegion, testInstanceName),
					op: util.OpInfo{
						Name: "op-1",
						Type: util.InstanceCreate,
					},
				},
			},
			signalGetOp:    true,
			signalIsOpDone: true,
			isOpDoneStatus: &MockOpStatus{
				reportRunning: true,
			},
			instanceKeyExpectedInCache: true,
			expectedVerificationStaus:  false,
		},
		{
			name:        "tc5 - IsOpDone true, status verified",
			scKey:       testInstanceScPrefix,
			instanceKey: util.CreateInstanceKey(testProject, testRegion, testInstanceName),
			instance: &file.MultishareInstance{
				Project:  testProject,
				Location: testRegion,
				Name:     testInstanceName,
			},
			initInstanceOpMap: []Item{
				{
					scKey:       testInstanceScPrefix,
					instanceKey: util.CreateInstanceKey(testProject, testRegion, testInstanceName),
					op: util.OpInfo{
						Name: "op-1",
						Type: util.InstanceCreate,
					},
				},
			},
			signalGetOp:    true,
			signalIsOpDone: true,
			isOpDoneStatus: &MockOpStatus{
				reportRunning: true,
			},
			instanceKeyExpectedInCache: true,
			expectedVerificationStaus:  false,
		}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opUnblocker := make(chan chan file.Signal)
			cloudProvider := initCloudProviderWithBlockingFileService(t, opUnblocker)
			manager := NewMultishareOpsManager(cloudProvider)

			runRequest := func(ctx context.Context, instanceSCPrefix string, instance *file.MultishareInstance) <-chan Response {
				responseChannel := make(chan Response)
				go func() {
					status, err := manager.verifyNoRunningInstanceOps(ctx, instanceSCPrefix, instance)
					responseChannel <- Response{
						verified: status,
						err:      err,
					}
				}()
				return responseChannel
			}

			// Pre-populate the cache.
			for _, v := range tc.initInstanceOpMap {
				manager.cache.AddInstanceOp(v.scKey, v.instanceKey, v.op)
			}

			respChannel := runRequest(context.Background(), tc.scKey, tc.instance)

			// Inject mock response
			if tc.signalGetOp {
				s := file.Signal{}
				if tc.getOpStatus != nil {
					s.ReportError = tc.getOpStatus.reportError
					s.ReportOpWithErrorStatus = tc.getOpStatus.reportOpWithErrorStatus
					s.ReportRunning = tc.getOpStatus.reportRunning
				}
				execute := <-opUnblocker
				execute <- s
			}

			// Inject mock response
			if tc.signalIsOpDone {
				s := file.Signal{}
				if tc.isOpDoneStatus != nil {
					s.ReportError = tc.isOpDoneStatus.reportError
					s.ReportOpWithErrorStatus = tc.isOpDoneStatus.reportOpWithErrorStatus
					s.ReportRunning = tc.isOpDoneStatus.reportRunning
				}
				execute := <-opUnblocker
				execute <- s
			}

			// Verify response
			response := <-respChannel
			if response.verified != tc.expectedVerificationStaus {
				t.Errorf("verify status want %v, got %v", tc.expectedVerificationStaus, response.verified)
			}

			if tc.errorExpected && response.err == nil {
				t.Errorf("expected error not found")
			}
			if !tc.errorExpected && response.err != nil {
				t.Errorf("unexpected error found")
			}

			// Verify cache content
			instanceOp := manager.cache.GetInstanceOp(tc.scKey, tc.instanceKey)
			if tc.instanceKeyExpectedInCache && instanceOp == nil {
				t.Errorf("expcted instance key not found")
			}
			if !tc.instanceKeyExpectedInCache && instanceOp != nil {
				t.Errorf("unexpcted instance key found")
			}
			if tc.emptyOpExpected && instanceOp.Name != "" {
				t.Errorf("expcted empty op")
			}
		})
	}
}

func TestVerifyNoRunningShareOp(t *testing.T) {
	tests := []struct {
		name                         string
		scKey                        string
		share                        *file.Share
		initShareCreateOpMap         []Item
		initShareOpMap               []Item
		signalGetOpForShareCreate    bool
		signalIsOpDoneForShareCreate bool
		getOpStatusforShareCreate    *MockOpStatus
		isOpDoneStatusForShareCreate *MockOpStatus
		signalGetOpForShareOp        bool
		signalIsOpDoneForShareOp     bool
		getOpStatusforShareOp        *MockOpStatus
		isOpDoneStatusForShareOp     *MockOpStatus

		errorExpected             bool
		expectedVerificationStaus bool
	}{
		{
			name:  "tc1 - no share create op in cache, no share op in cache",
			scKey: testInstanceScPrefix,
			share: &file.Share{
				Name: testShareName,
				Parent: &file.MultishareInstance{
					Project:  testProject,
					Location: testLocation,
					Name:     testInstanceName,
				},
			},
			expectedVerificationStaus: true,
		},
		{
			name:  "tc2 - check for share create op returns error, status not verified",
			scKey: testInstanceScPrefix,
			share: &file.Share{
				Name: testShareName,
				Parent: &file.MultishareInstance{
					Project:  testProject,
					Location: testLocation,
					Name:     testInstanceName,
				},
			},
			initShareCreateOpMap: []Item{
				{
					scKey:          testInstanceScPrefix,
					shareCreateKey: testShareName,
					createOp: util.ShareCreateOpInfo{
						InstanceHandle: testInstanceHandle,
						OpName:         "op-1",
					},
				},
			},
			signalGetOpForShareCreate: true,
			getOpStatusforShareCreate: &MockOpStatus{
				reportError: true,
			},
			expectedVerificationStaus: false,
			errorExpected:             true,
		},
		{
			name:  "tc3 - check for share create op returns a stale op status complete in isOpDone, check for share op returns error in get op, status not verified",
			scKey: testInstanceScPrefix,
			share: &file.Share{
				Name: testShareName,
				Parent: &file.MultishareInstance{
					Project:  testProject,
					Location: testRegion,
					Name:     testInstanceName,
				},
			},
			initShareCreateOpMap: []Item{
				{
					scKey:          testInstanceScPrefix,
					shareCreateKey: testShareName,
					createOp: util.ShareCreateOpInfo{
						InstanceHandle: testInstanceHandle,
						OpName:         "op-1",
					},
				},
			},
			initShareOpMap: []Item{
				{
					scKey:    testInstanceScPrefix,
					shareKey: util.CreateShareKey(testProject, testRegion, testInstanceName, testShareName),
					op: util.OpInfo{
						Name: "op-1",
						Type: util.ShareDelete,
					},
				},
			},
			signalGetOpForShareCreate:    true,
			signalIsOpDoneForShareCreate: true,
			signalGetOpForShareOp:        true,
			getOpStatusforShareOp: &MockOpStatus{
				reportError: true,
			},
			expectedVerificationStaus: false,
			errorExpected:             true,
		},
		{
			name:  "tc4 - check for share create op returns nil, check for share op returns op fialed, status verified",
			scKey: testInstanceScPrefix,
			share: &file.Share{
				Name: testShareName,
				Parent: &file.MultishareInstance{
					Project:  testProject,
					Location: testRegion,
					Name:     testInstanceName,
				},
			},
			initShareOpMap: []Item{
				{
					scKey:    testInstanceScPrefix,
					shareKey: util.CreateShareKey(testProject, testRegion, testInstanceName, testShareName),
					op: util.OpInfo{
						Name: "op-1",
						Type: util.ShareDelete,
					},
				},
			},
			signalGetOpForShareOp:    true,
			signalIsOpDoneForShareOp: true,
			isOpDoneStatusForShareOp: &MockOpStatus{
				reportError: true,
			},
			expectedVerificationStaus: true,
			errorExpected:             false,
		},
		{
			name:  "tc4 - check for share create op returns nil, check for share op returns op success, status verified",
			scKey: testInstanceScPrefix,
			share: &file.Share{
				Name: testShareName,
				Parent: &file.MultishareInstance{
					Project:  testProject,
					Location: testRegion,
					Name:     testInstanceName,
				},
			},
			initShareOpMap: []Item{
				{
					scKey:    testInstanceScPrefix,
					shareKey: util.CreateShareKey(testProject, testRegion, testInstanceName, testShareName),
					op: util.OpInfo{
						Name: "op-1",
						Type: util.ShareDelete,
					},
				},
			},
			signalGetOpForShareOp:     true,
			signalIsOpDoneForShareOp:  true,
			expectedVerificationStaus: true,
			errorExpected:             false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opUnblocker := make(chan chan file.Signal)
			cloudProvider := initCloudProviderWithBlockingFileService(t, opUnblocker)
			manager := NewMultishareOpsManager(cloudProvider)

			runRequest := func(ctx context.Context, instanceSCPrefix string, share *file.Share) <-chan Response {
				responseChannel := make(chan Response)
				go func() {
					status, err := manager.verifyNoRunningShareOp(ctx, instanceSCPrefix, share)
					responseChannel <- Response{
						verified: status,
						err:      err,
					}
				}()
				return responseChannel
			}
			// Pre-populate the share create op map
			for _, v := range tc.initShareCreateOpMap {
				if _, ok := manager.cache.ScInfoMap[v.scKey]; !ok {
					manager.cache.ScInfoMap[v.scKey] = util.NewStorageClassInfo()
				}
				manager.cache.AddShareCreateOp(v.scKey, v.shareCreateKey, v.createOp)
			}

			// Pre-populate the share op map
			for _, v := range tc.initShareOpMap {
				if _, ok := manager.cache.ScInfoMap[v.scKey]; !ok {
					manager.cache.ScInfoMap[v.scKey] = util.NewStorageClassInfo()
				}
				manager.cache.AddShareOp(v.scKey, v.shareKey, v.op)
			}

			respChannel := runRequest(context.Background(), tc.scKey, tc.share)

			// Inject mock response
			if tc.signalGetOpForShareCreate {
				s := file.Signal{}
				if tc.getOpStatusforShareCreate != nil {
					s.ReportError = tc.getOpStatusforShareCreate.reportError
					s.ReportOpWithErrorStatus = tc.getOpStatusforShareCreate.reportOpWithErrorStatus
					s.ReportRunning = tc.getOpStatusforShareCreate.reportRunning
				}
				execute := <-opUnblocker
				execute <- s
			}

			// Inject mock response
			if tc.signalIsOpDoneForShareCreate {
				s := file.Signal{}
				if tc.isOpDoneStatusForShareCreate != nil {
					s.ReportError = tc.isOpDoneStatusForShareCreate.reportError
					s.ReportOpWithErrorStatus = tc.isOpDoneStatusForShareCreate.reportOpWithErrorStatus
					s.ReportRunning = tc.isOpDoneStatusForShareCreate.reportRunning
				}
				execute := <-opUnblocker
				execute <- s
			}

			// Inject mock response
			if tc.signalGetOpForShareOp {
				s := file.Signal{}
				if tc.getOpStatusforShareOp != nil {
					s.ReportError = tc.getOpStatusforShareOp.reportError
					s.ReportOpWithErrorStatus = tc.getOpStatusforShareOp.reportOpWithErrorStatus
					s.ReportRunning = tc.getOpStatusforShareOp.reportRunning
				}
				execute := <-opUnblocker
				execute <- s
			}

			// Inject mock response
			if tc.signalIsOpDoneForShareOp {
				s := file.Signal{}
				if tc.isOpDoneStatusForShareOp != nil {
					s.ReportError = tc.isOpDoneStatusForShareOp.reportError
					s.ReportOpWithErrorStatus = tc.isOpDoneStatusForShareOp.reportOpWithErrorStatus
					s.ReportRunning = tc.isOpDoneStatusForShareOp.reportRunning
				}
				execute := <-opUnblocker
				execute <- s
			}
			// Verify response
			response := <-respChannel
			if response.verified != tc.expectedVerificationStaus {
				t.Errorf("op status want %v, got %v", tc.expectedVerificationStaus, response.verified)
			}
			if response.err != nil && !tc.errorExpected {
				t.Errorf("got unexpected error")
			}
			if response.err == nil && tc.errorExpected {
				t.Errorf("expected error")
			}
		})
	}
}

func TestRunEligibleInstanceCheck(t *testing.T) {
	tests := []struct {
		name                              string
		scKey                             string
		initInstance                      []file.MultishareInstance
		initShares                        []file.Share
		initInstanceOpMap                 []Item
		numSignalGetOpForInstance         int
		numsignalIsOpDoneForInstance      int
		isOpDoneStatusForInstance         []MockOpStatus
		numSignalGetInstance              int
		reportErrorForGetInstance         []bool
		reportNotFoundErrorForGetInstance []bool
		expectedNumReadyInstances         int
		expectedNumNonReadyInstances      int
		expectedError                     bool
	}{
		{
			name:  "tc1-empty instance map",
			scKey: testInstanceScPrefix,
		},
		{
			name:  "single instance in map, failed to parse handle",
			scKey: testInstanceScPrefix,
			initInstanceOpMap: []Item{
				{
					instanceKey: util.InstanceKey("blah"),
					op:          util.OpInfo{},
				},
			},
		},
		{
			name:  "tc2-single ready instance in map, GET failed",
			scKey: testInstanceScPrefix,
			initInstanceOpMap: []Item{
				{
					instanceKey: util.CreateInstanceKey(testProject, testRegion, testInstanceName),
					op:          util.OpInfo{},
				},
			},
			numSignalGetInstance:              1,
			reportErrorForGetInstance:         []bool{false},
			reportNotFoundErrorForGetInstance: []bool{false},
		},
		{
			name:  "tc3-single ready instance with 0 shares in map, GET success",
			scKey: testInstanceScPrefix,
			initInstanceOpMap: []Item{
				{
					instanceKey: util.CreateInstanceKey(testProject, testRegion, testInstanceName),
					op:          util.OpInfo{},
				},
			},
			initInstance: []file.MultishareInstance{
				{
					Name:     testInstanceName,
					Project:  testProject,
					Location: testRegion,
				},
			},
			numSignalGetInstance:              1,
			reportErrorForGetInstance:         []bool{false},
			reportNotFoundErrorForGetInstance: []bool{false},
			expectedNumReadyInstances:         1,
		},
		{
			name:  "tc4-two ready instance (no op in map) with 0 shares in map, GET success",
			scKey: testInstanceScPrefix,
			initInstanceOpMap: []Item{
				{
					instanceKey: util.CreateInstanceKey(testProject, testRegion, testInstanceName+"1"),
					op:          util.OpInfo{},
				},
				{
					instanceKey: util.CreateInstanceKey(testProject, testRegion, testInstanceName+"2"),
					op:          util.OpInfo{},
				},
			},
			initInstance: []file.MultishareInstance{
				{
					Name:     testInstanceName + "1",
					Project:  testProject,
					Location: testRegion,
				},
				{
					Name:     testInstanceName + "2",
					Project:  testProject,
					Location: testRegion,
				},
			},
			numSignalGetInstance:              2,
			reportErrorForGetInstance:         []bool{false, false},
			reportNotFoundErrorForGetInstance: []bool{false, false},
			expectedNumReadyInstances:         2,
		},
		{
			name:  "tc5-two ready instance (op in map, that is complete) with 0 shares in map, GET success",
			scKey: testInstanceScPrefix,
			initInstanceOpMap: []Item{
				{
					instanceKey: util.CreateInstanceKey(testProject, testRegion, testInstanceName+"1"),
					op: util.OpInfo{
						Name: "op-1",
						Type: util.InstanceCreate,
					},
				},
				{
					instanceKey: util.CreateInstanceKey(testProject, testRegion, testInstanceName+"2"),
					op: util.OpInfo{
						Name: "op-2",
						Type: util.InstanceCreate,
					},
				},
			},
			initInstance: []file.MultishareInstance{
				{
					Name:     testInstanceName + "1",
					Project:  testProject,
					Location: testRegion,
				},
				{
					Name:     testInstanceName + "2",
					Project:  testProject,
					Location: testRegion,
				},
			},
			numSignalGetOpForInstance:    2,
			numsignalIsOpDoneForInstance: 2,
			isOpDoneStatusForInstance: []MockOpStatus{
				{
					reportRunning:           false,
					reportNotFoundError:     false,
					reportOpWithErrorStatus: false,
				},
				{
					reportRunning:           false,
					reportNotFoundError:     false,
					reportOpWithErrorStatus: false,
				},
			},
			numSignalGetInstance:              2,
			reportErrorForGetInstance:         []bool{false, false},
			reportNotFoundErrorForGetInstance: []bool{false, false},
			expectedNumReadyInstances:         2,
		},
		{
			name:  "tc6-two instance (delete op in map, that is complete) with 0 shares in map",
			scKey: testInstanceScPrefix,
			initInstanceOpMap: []Item{
				{
					instanceKey: util.CreateInstanceKey(testProject, testRegion, testInstanceName+"1"),
					op: util.OpInfo{
						Name: "op-1",
						Type: util.InstanceDelete,
					},
				},
				{
					instanceKey: util.CreateInstanceKey(testProject, testRegion, testInstanceName+"2"),
					op: util.OpInfo{
						Name: "op-2",
						Type: util.InstanceDelete,
					},
				},
			},
			initInstance: []file.MultishareInstance{
				{
					Name:     testInstanceName + "1",
					Project:  testProject,
					Location: testRegion,
				},
				{
					Name:     testInstanceName + "2",
					Project:  testProject,
					Location: testRegion,
				},
			},
			numSignalGetOpForInstance:    2,
			numsignalIsOpDoneForInstance: 2,
			isOpDoneStatusForInstance: []MockOpStatus{
				{
					reportRunning:           false,
					reportNotFoundError:     false,
					reportOpWithErrorStatus: false,
				},
				{
					reportRunning:           false,
					reportNotFoundError:     false,
					reportOpWithErrorStatus: false,
				},
			},
			numSignalGetInstance:              2,
			reportErrorForGetInstance:         []bool{false, false},
			reportNotFoundErrorForGetInstance: []bool{true, true},
		},
		{
			name:  "tc7-two instance (one ready, one not ready) with 0 shares in map",
			scKey: testInstanceScPrefix,
			initInstanceOpMap: []Item{
				{
					instanceKey: util.CreateInstanceKey(testProject, testRegion, testInstanceName+"1"),
					op: util.OpInfo{
						Name: "op-1",
						Type: util.InstanceCreate,
					},
				},
				{
					instanceKey: util.CreateInstanceKey(testProject, testRegion, testInstanceName+"2"),
					op: util.OpInfo{
						Name: "op-2",
						Type: util.InstanceUpdate,
					},
				},
			},
			initInstance: []file.MultishareInstance{
				{
					Name:     testInstanceName + "1",
					Project:  testProject,
					Location: testRegion,
				},
				{
					Name:     testInstanceName + "2",
					Project:  testProject,
					Location: testRegion,
				},
			},
			numSignalGetOpForInstance:    2,
			numsignalIsOpDoneForInstance: 2,
			isOpDoneStatusForInstance: []MockOpStatus{
				{
					reportRunning:           false,
					reportNotFoundError:     false,
					reportOpWithErrorStatus: false,
				},
				{
					reportRunning:           true,
					reportNotFoundError:     false,
					reportOpWithErrorStatus: false,
				},
			},
			numSignalGetInstance:              1,
			reportErrorForGetInstance:         []bool{false},
			reportNotFoundErrorForGetInstance: []bool{false},
			expectedNumNonReadyInstances:      1,
			expectedNumReadyInstances:         1,
		},
		{
			name:  "tc8-one instance (delete op in progress) with 0 shares in map",
			scKey: testInstanceScPrefix,
			initInstanceOpMap: []Item{
				{
					instanceKey: util.CreateInstanceKey(testProject, testRegion, testInstanceName+"1"),
					op: util.OpInfo{
						Name: "op-1",
						Type: util.InstanceDelete,
					},
				},
			},
			initInstance: []file.MultishareInstance{
				{
					Name:     testInstanceName + "1",
					Project:  testProject,
					Location: testRegion,
				},
			},
			numSignalGetOpForInstance:    1,
			numsignalIsOpDoneForInstance: 1,
			isOpDoneStatusForInstance: []MockOpStatus{
				{
					reportRunning:           true,
					reportNotFoundError:     false,
					reportOpWithErrorStatus: false,
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opUnblocker := make(chan chan file.Signal, 1)
			cloudProvider := initCloudProviderWithBlockingFileService(t, opUnblocker)
			manager := NewMultishareOpsManager(cloudProvider)

			runRequest := func(ctx context.Context, instanceSCPrefix string) <-chan Response {
				responseChannel := make(chan Response)
				go func() {
					ready, numNonReady, err := manager.runEligibleInstanceCheck(ctx, instanceSCPrefix)
					responseChannel <- Response{
						readyInstances:       ready,
						numNonReadyInstances: numNonReady,
						err:                  err,
					}
				}()
				return responseChannel
			}
			// Prepopulate known instances and shares
			for _, instance := range tc.initInstance {
				manager.cloud.File.StartCreateMultishareInstanceOp(context.Background(), &instance)
			}
			for _, share := range tc.initShares {
				manager.cloud.File.StartCreateShareOp(context.Background(), &share)
			}
			for _, item := range tc.initInstanceOpMap {
				manager.cache.AddInstanceOp(tc.scKey, item.instanceKey, item.op)
			}

			respChannel := runRequest(context.Background(), tc.scKey)
			// Inject mock response for GetOp
			for i := 0; i < tc.numSignalGetOpForInstance; i++ {
				s := file.Signal{}
				execute := <-opUnblocker
				execute <- s
			}

			// Inject mock response for IsOpDone
			for i := 0; i < tc.numsignalIsOpDoneForInstance; i++ {
				s := file.Signal{}
				s.ReportError = tc.isOpDoneStatusForInstance[i].reportError
				s.ReportOpWithErrorStatus = tc.isOpDoneStatusForInstance[i].reportOpWithErrorStatus
				s.ReportRunning = tc.isOpDoneStatusForInstance[i].reportRunning
				execute := <-opUnblocker
				execute <- s
			}

			for i := 0; i < tc.numSignalGetInstance; i++ {
				s := file.Signal{}
				s.ReportError = tc.reportErrorForGetInstance[i]
				s.ReportNotFoundError = tc.reportNotFoundErrorForGetInstance[i]
				execute := <-opUnblocker
				execute <- s
			}
			// Verify response
			response := <-respChannel
			if response.numNonReadyInstances != tc.expectedNumNonReadyInstances {
				t.Errorf("want %v, got %v", tc.expectedNumNonReadyInstances, response.numNonReadyInstances)
			}
			if len(response.readyInstances) != tc.expectedNumReadyInstances {
				t.Errorf("unexpected instance %v", response.readyInstances)
			}
			if !tc.expectedError && response.err != nil {
				t.Errorf("unexpected error")
			}
			if tc.expectedError && response.err == nil {
				t.Errorf("expecteded error")
			}
		})
	}
}

func TestInstanceNeedsExpand(t *testing.T) {
	tests := []struct {
		name                    string
		scKey                   string
		initShares              []file.Share
		targetShareToAccomodate *file.Share
		expectedNeedsExpand     bool
		targetBytes             int64
		expectError             bool
	}{
		{
			name:  "0 shares in 1 T instance,  new 100G share",
			scKey: testInstanceScPrefix,
			targetShareToAccomodate: &file.Share{
				Name:          testShareName,
				CapacityBytes: 100 * util.Gb,
				Parent: &file.MultishareInstance{
					Project:       testProject,
					Location:      testRegion,
					Name:          testInstanceName,
					CapacityBytes: 1 * util.Tb,
				},
			},
		},
		{
			name:  "1 existing 100G share in 1 T instance,  new 100G share",
			scKey: testInstanceScPrefix,
			initShares: []file.Share{
				{
					Name:          testShareName + "1",
					CapacityBytes: 100 * util.Gb,
					Parent: &file.MultishareInstance{
						Project:       testProject,
						Location:      testRegion,
						Name:          testInstanceName,
						CapacityBytes: 1 * util.Tb,
					},
				},
			},
			targetShareToAccomodate: &file.Share{
				Name:          testShareName + "2",
				CapacityBytes: 100 * util.Gb,
				Parent: &file.MultishareInstance{
					Project:       testProject,
					Location:      testRegion,
					Name:          testInstanceName,
					CapacityBytes: 1 * util.Tb,
				},
			},
		},
		{
			name:  "9 existing 100G share in 1 T instance, new 100G share",
			scKey: testInstanceScPrefix,
			initShares: []file.Share{
				{
					Name:          testShareName + "1",
					CapacityBytes: 100 * util.Gb,
					Parent: &file.MultishareInstance{
						Project:       testProject,
						Location:      testRegion,
						Name:          testInstanceName,
						CapacityBytes: 1 * util.Tb,
					},
				},
				{
					Name:          testShareName + "2",
					CapacityBytes: 100 * util.Gb,
					Parent: &file.MultishareInstance{
						Project:       testProject,
						Location:      testRegion,
						Name:          testInstanceName,
						CapacityBytes: 1 * util.Tb,
					},
				},
				{
					Name:          testShareName + "3",
					CapacityBytes: 100 * util.Gb,
					Parent: &file.MultishareInstance{
						Project:       testProject,
						Location:      testRegion,
						Name:          testInstanceName,
						CapacityBytes: 1 * util.Tb,
					},
				},
				{
					Name:          testShareName + "4",
					CapacityBytes: 100 * util.Gb,
					Parent: &file.MultishareInstance{
						Project:       testProject,
						Location:      testRegion,
						Name:          testInstanceName,
						CapacityBytes: 1 * util.Tb,
					},
				},
				{
					Name:          testShareName + "5",
					CapacityBytes: 100 * util.Gb,
					Parent: &file.MultishareInstance{
						Project:       testProject,
						Location:      testRegion,
						Name:          testInstanceName,
						CapacityBytes: 1 * util.Tb,
					},
				},
				{
					Name:          testShareName + "6",
					CapacityBytes: 100 * util.Gb,
					Parent: &file.MultishareInstance{
						Project:       testProject,
						Location:      testRegion,
						Name:          testInstanceName,
						CapacityBytes: 1 * util.Tb,
					},
				},
				{
					Name:          testShareName + "7",
					CapacityBytes: 100 * util.Gb,
					Parent: &file.MultishareInstance{
						Project:       testProject,
						Location:      testRegion,
						Name:          testInstanceName,
						CapacityBytes: 1 * util.Tb,
					},
				},
				{
					Name:          testShareName + "8",
					CapacityBytes: 100 * util.Gb,
					Parent: &file.MultishareInstance{
						Project:       testProject,
						Location:      testRegion,
						Name:          testInstanceName,
						CapacityBytes: 1 * util.Tb,
					},
				},
				{
					Name:          testShareName + "9",
					CapacityBytes: 100 * util.Gb,
					Parent: &file.MultishareInstance{
						Project:       testProject,
						Location:      testRegion,
						Name:          testInstanceName,
						CapacityBytes: 1 * util.Tb,
					},
				},
			},
			targetShareToAccomodate: &file.Share{
				Name:          testShareName + "10",
				CapacityBytes: 100 * util.Gb,
				Parent: &file.MultishareInstance{
					Project:       testProject,
					Location:      testRegion,
					Name:          testInstanceName,
					CapacityBytes: 1 * util.Tb,
				},
			},
		},
		{
			name:  "1 existing 100G share in 1 T instance,  new 1T share",
			scKey: testInstanceScPrefix,
			initShares: []file.Share{
				{
					Name:          testShareName + "1",
					CapacityBytes: 100 * util.Gb,
					Parent: &file.MultishareInstance{
						Project:       testProject,
						Location:      testRegion,
						Name:          testInstanceName,
						CapacityBytes: 1 * util.Tb,
					},
				},
			},
			targetShareToAccomodate: &file.Share{
				Name:          testShareName + "2",
				CapacityBytes: 1 * util.Tb,
				Parent: &file.MultishareInstance{
					Project:       testProject,
					Location:      testRegion,
					Name:          testInstanceName,
					CapacityBytes: 1 * util.Tb,
				},
			},
			expectedNeedsExpand: true,
			targetBytes:         1*util.Tb + (1*util.Tb - (1*util.Tb - 100*util.Gb)),
		},
		{
			name:  "2 existing 100G share in 1 T instance,  new 900G share",
			scKey: testInstanceScPrefix,
			initShares: []file.Share{
				{
					Name:          testShareName + "1",
					CapacityBytes: 100 * util.Gb,
					Parent: &file.MultishareInstance{
						Project:       testProject,
						Location:      testRegion,
						Name:          testInstanceName,
						CapacityBytes: 1 * util.Tb,
					},
				},
				{
					Name:          testShareName + "2",
					CapacityBytes: 100 * util.Gb,
					Parent: &file.MultishareInstance{
						Project:       testProject,
						Location:      testRegion,
						Name:          testInstanceName,
						CapacityBytes: 1 * util.Tb,
					},
				},
				{
					Name:          testShareName + "3",
					CapacityBytes: 100 * util.Gb,
					Parent: &file.MultishareInstance{
						Project:       testProject,
						Location:      testRegion,
						Name:          testInstanceName,
						CapacityBytes: 1 * util.Tb,
					},
				},
				{
					Name:          testShareName + "4",
					CapacityBytes: 100 * util.Gb,
					Parent: &file.MultishareInstance{
						Project:       testProject,
						Location:      testRegion,
						Name:          testInstanceName,
						CapacityBytes: 1 * util.Tb,
					},
				},
				{
					Name:          testShareName + "5",
					CapacityBytes: 100 * util.Gb,
					Parent: &file.MultishareInstance{
						Project:       testProject,
						Location:      testRegion,
						Name:          testInstanceName,
						CapacityBytes: 1 * util.Tb,
					},
				},
				{
					Name:          testShareName + "6",
					CapacityBytes: 100 * util.Gb,
					Parent: &file.MultishareInstance{
						Project:       testProject,
						Location:      testRegion,
						Name:          testInstanceName,
						CapacityBytes: 1 * util.Tb,
					},
				},
				{
					Name:          testShareName + "7",
					CapacityBytes: 100 * util.Gb,
					Parent: &file.MultishareInstance{
						Project:       testProject,
						Location:      testRegion,
						Name:          testInstanceName,
						CapacityBytes: 1 * util.Tb,
					},
				},
				{
					Name:          testShareName + "8",
					CapacityBytes: 100 * util.Gb,
					Parent: &file.MultishareInstance{
						Project:       testProject,
						Location:      testRegion,
						Name:          testInstanceName,
						CapacityBytes: 1 * util.Tb,
					},
				},
				{
					Name:          testShareName + "9",
					CapacityBytes: 100 * util.Gb,
					Parent: &file.MultishareInstance{
						Project:       testProject,
						Location:      testRegion,
						Name:          testInstanceName,
						CapacityBytes: 1 * util.Tb,
					},
				},
			},
			targetShareToAccomodate: &file.Share{
				Name:          testShareName + "10",
				CapacityBytes: 1 * util.Tb,
				Parent: &file.MultishareInstance{
					Project:       testProject,
					Location:      testRegion,
					Name:          testInstanceName,
					CapacityBytes: 1 * util.Tb,
				},
			},
			expectedNeedsExpand: true,
			targetBytes:         1*util.Tb + (1*util.Tb - (1*util.Tb - 9*100*util.Gb)),
		},
		{
			name:  "9 existing 100G share in 1 T instance,  new 1T share",
			scKey: testInstanceScPrefix,
			initShares: []file.Share{
				{
					Name:          testShareName + "1",
					CapacityBytes: 100 * util.Gb,
					Parent: &file.MultishareInstance{
						Project:       testProject,
						Location:      testRegion,
						Name:          testInstanceName,
						CapacityBytes: 1 * util.Tb,
					},
				},
				{
					Name:          testShareName + "2",
					CapacityBytes: 100 * util.Gb,
					Parent: &file.MultishareInstance{
						Project:       testProject,
						Location:      testRegion,
						Name:          testInstanceName,
						CapacityBytes: 1 * util.Tb,
					},
				},
			},
			targetShareToAccomodate: &file.Share{
				Name:          testShareName + "3",
				CapacityBytes: 900 * util.Gb,
				Parent: &file.MultishareInstance{
					Project:       testProject,
					Location:      testRegion,
					Name:          testInstanceName,
					CapacityBytes: 1 * util.Tb,
				},
			},
			expectedNeedsExpand: true,
			targetBytes:         1*util.Tb + (900*util.Gb - (1*util.Tb - 2*100*util.Gb)),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opUnblocker := make(chan chan file.Signal, 1)
			cloudProvider := initCloudProviderWithBlockingFileService(t, opUnblocker)
			manager := NewMultishareOpsManager(cloudProvider)

			runRequest := func(ctx context.Context, share *file.Share) <-chan Response {
				responseChannel := make(chan Response)
				go func() {
					needsExpand, targetBytes, err := manager.instanceNeedsExpand(context.Background(), share)
					responseChannel <- Response{
						instanceNeedsExpand: needsExpand,
						targetBytes:         targetBytes,
						err:                 err,
					}
				}()
				return responseChannel
			}

			for _, share := range tc.initShares {
				if share.Parent != nil {
					manager.cloud.File.StartCreateMultishareInstanceOp(context.Background(), share.Parent)
				}
				manager.cloud.File.StartCreateShareOp(context.Background(), &share)
			}

			respChannel := runRequest(context.Background(), tc.targetShareToAccomodate)
			response := <-respChannel
			if tc.expectError && response.err == nil {
				t.Errorf("expected error")
			}
			if !tc.expectError && response.err != nil {
				t.Errorf("unexpectded error")
			}
			if tc.expectedNeedsExpand != response.instanceNeedsExpand {
				t.Errorf("want %v, got %v", tc.expectedNeedsExpand, response.instanceNeedsExpand)
			}
			if tc.targetBytes != response.targetBytes {
				t.Errorf("want %v, got %v", tc.targetBytes, response.targetBytes)
			}
		})
	}
}
