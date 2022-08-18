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

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"
	filev1beta1multishare "google.golang.org/api/file/v1beta1"
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
	testInstanceHandle = fmt.Sprintf("%s/%s/%s", testProject, testRegion, testInstanceName)
)

type Item struct {
	scKey          string
	shareCreateKey string
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
	verified             bool
	readyInstances       []*file.MultishareInstance
	numNonReadyInstances int
	instanceNeedsExpand  bool
	targetBytes          int64
	err                  error
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

			runRequest := func(ctx context.Context, share *file.Share, capNeeded int64) <-chan Response {
				responseChannel := make(chan Response)
				go func() {
					needsExpand, targetBytes, err := manager.instanceNeedsExpand(context.Background(), share, capNeeded)
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

			respChannel := runRequest(context.Background(), tc.targetShareToAccomodate, tc.targetShareToAccomodate.CapacityBytes)
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

func TestListMatchedInstances(t *testing.T) {
	found := func(inputList []*file.MultishareInstance, i *file.MultishareInstance) bool {
		for _, f := range inputList {
			if f.Project == i.Project && f.Location == i.Location && f.Name == i.Name {
				return true
			}
		}
		return false
	}

	tests := []struct {
		name             string
		initInstanceList []*file.MultishareInstance
		expectedList     []*file.MultishareInstance
		req              *csi.CreateVolumeRequest
		target           *file.MultishareInstance
		expectError      bool
	}{
		{
			name: "empty init inistance list",
			req: &csi.CreateVolumeRequest{
				Parameters: map[string]string{
					paramMultishareInstanceScLabel: "testprefix",
				},
			},
			target: &file.MultishareInstance{
				Name:     "test-target-instance",
				Project:  "test-project",
				Location: "us-central1",
				Labels: map[string]string{
					util.ParamMultishareInstanceScLabelKey: "testprefix",
				},
			},
		},
		{
			name: "non-empty init inistance list",
			req: &csi.CreateVolumeRequest{
				Parameters: map[string]string{
					paramMultishareInstanceScLabel: "testprefix",
				},
			},
			target: &file.MultishareInstance{
				Name:     "test-target-instance",
				Project:  "test-project",
				Location: "us-central1",
				Labels: map[string]string{
					util.ParamMultishareInstanceScLabelKey: "testprefix",
				},
			},
			initInstanceList: []*file.MultishareInstance{
				{
					Name:     "test-instance",
					Project:  "test-project",
					Location: "us-central1",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
				},
			},
			expectedList: []*file.MultishareInstance{
				{
					Name:     "test-instance",
					Project:  "test-project",
					Location: "us-central1",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
				},
			},
		},
		{
			name: "non-empty init inistance list, 1 instance match",
			req: &csi.CreateVolumeRequest{
				Parameters: map[string]string{
					paramMultishareInstanceScLabel: "testprefix-1",
				},
			},
			target: &file.MultishareInstance{
				Name:     "test-target-instance",
				Project:  "test-project",
				Location: "us-central1",
				Labels: map[string]string{
					util.ParamMultishareInstanceScLabelKey: "testprefix-1",
				},
			},
			initInstanceList: []*file.MultishareInstance{
				{
					Name:     "test-instance-1",
					Project:  "test-project",
					Location: "us-central1",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix-1",
					},
				},
				{
					Name:     "test-instance-2",
					Project:  "test-project",
					Location: "us-central1",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix-2",
					},
				},
			},
			expectedList: []*file.MultishareInstance{
				{
					Name:     "test-instance-1",
					Project:  "test-project",
					Location: "us-central1",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix-1",
					},
				},
			},
		},
		{
			name: "non-empty init inistance list, 2 instances match",
			req: &csi.CreateVolumeRequest{
				Parameters: map[string]string{
					paramMultishareInstanceScLabel: "testprefix",
				},
			},
			target: &file.MultishareInstance{
				Name:     "test-target-instance",
				Project:  "test-project",
				Location: "us-central1",
				Labels: map[string]string{
					util.ParamMultishareInstanceScLabelKey: "testprefix",
				},
			},
			initInstanceList: []*file.MultishareInstance{
				{
					Name:     "test-instance-1",
					Project:  "test-project",
					Location: "us-central1",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
				},
				{
					Name:     "test-instance-2",
					Project:  "test-project",
					Location: "us-central1",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
				},
				{
					Name:     "test-instance-3",
					Project:  "test-project",
					Location: "us-central1",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix-3",
					},
				},
			},
			expectedList: []*file.MultishareInstance{
				{
					Name:     "test-instance-1",
					Project:  "test-project",
					Location: "us-central1",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
				},
				{
					Name:     "test-instance-2",
					Project:  "test-project",
					Location: "us-central1",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
				},
			},
		},
		{
			name: "non-specified sc prefix in init instance list",
			req: &csi.CreateVolumeRequest{
				Parameters: map[string]string{
					paramMultishareInstanceScLabel: "testprefix",
				},
			},
			target: &file.MultishareInstance{
				Name:     "test-target-instance",
				Project:  "test-project",
				Location: "us-central1",
				Labels: map[string]string{
					util.ParamMultishareInstanceScLabelKey: "testprefix",
				},
			},
			initInstanceList: []*file.MultishareInstance{
				{
					Name:     "test-instance",
					Project:  "test-project",
					Location: "us-central1",
					Labels:   map[string]string{},
				},
			},
		},
		{
			name: "1 ip address within, 1 out of reserved-ipv4-cidr",
			req: &csi.CreateVolumeRequest{
				Parameters: map[string]string{
					paramMultishareInstanceScLabel: "testprefix",
					paramReservedIPV4CIDR:          "10.0.0.0/24",
				},
			},
			target: &file.MultishareInstance{
				Name:     "test-target-instance",
				Project:  "test-project",
				Location: "us-central1",
				Labels: map[string]string{
					util.ParamMultishareInstanceScLabelKey: "testprefix",
				},
				Network: file.Network{
					ReservedIpRange: "10.0.0.0/24",
				},
			},
			initInstanceList: []*file.MultishareInstance{
				{
					Name:     "test-instance-0",
					Project:  "test-project",
					Location: "us-central1",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
					Network: file.Network{
						ReservedIpRange: "10.0.0.0/21",
						Ip:              "10.0.0.1",
					},
				},
				{
					Name:     "test-instance-1",
					Project:  "test-project",
					Location: "us-central1",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
					Network: file.Network{
						ReservedIpRange: "11.0.0.0/24",
						Ip:              "11.0.0.1",
					},
				},
			},
			expectedList: []*file.MultishareInstance{
				{
					Name:     "test-instance-0",
					Project:  "test-project",
					Location: "us-central1",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
					Network: file.Network{
						ReservedIpRange: "10.0.0.0/24",
						Ip:              "10.0.0.1",
					},
				},
			},
		},
		{
			name: "location, tier, network, connect-mode and cmek alignment test",
			req: &csi.CreateVolumeRequest{
				Parameters: map[string]string{
					paramMultishareInstanceScLabel: "testprefix",
					paramReservedIPV4CIDR:          "10.0.0.0/24",
					paramTier:                      "enterprise",
					paramNetwork:                   "shared-network",
					paramInstanceEncryptionKmsKey:  "projects/test-project/locations/us-central1/keyRings/test-cmek-key-ring/cryptoKeys/test-cmek-key",
				},
			},
			target: &file.MultishareInstance{
				Name:     "test-target-instance",
				Project:  "test-project",
				Location: "us-central1",
				Labels: map[string]string{
					util.ParamMultishareInstanceScLabelKey: "testprefix",
				},
				Network: file.Network{
					ReservedIpRange: "10.0.0.0/24",
					ConnectMode:     directPeering,
					Name:            "shared-network",
				},
				Tier:       "enterprise",
				KmsKeyName: "projects/test-project/locations/us-central1/keyRings/test-cmek-key-ring/cryptoKeys/test-cmek-key",
			},
			initInstanceList: []*file.MultishareInstance{
				{
					Name:     "test-instance-0",
					Project:  "test-project",
					Location: "us-west1",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
					Network: file.Network{
						ReservedIpRange: "10.0.0.0/24",
						ConnectMode:     directPeering,
						Name:            "shared-network",
						Ip:              "10.0.0.2",
					},
					Tier:       "enterprise",
					KmsKeyName: "projects/test-project/locations/us-central1/keyRings/test-cmek-key-ring/cryptoKeys/test-cmek-key",
				},
				{
					Name:     "test-instance-1",
					Project:  "test-project",
					Location: "us-central1",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
					Network: file.Network{
						ReservedIpRange: "10.0.0.0/24",
						ConnectMode:     directPeering,
						Name:            "shared-network",
						Ip:              "10.0.0.2",
					},
					Tier:       "standard",
					KmsKeyName: "projects/test-project/locations/us-central1/keyRings/test-cmek-key-ring/cryptoKeys/test-cmek-key",
				},
				{
					Name:     "test-instance-2",
					Project:  "test-project",
					Location: "us-central1",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
					Network: file.Network{
						ReservedIpRange: "10.0.0.0/24",
						ConnectMode:     directPeering,
						Name:            "default",
						Ip:              "10.0.0.2",
					},
					Tier:       "enterprise",
					KmsKeyName: "projects/test-project/locations/us-central1/keyRings/test-cmek-key-ring/cryptoKeys/test-cmek-key",
				},
				{
					Name:     "test-instance-3",
					Project:  "test-project",
					Location: "us-central1",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
					Network: file.Network{
						ReservedIpRange: "10.0.0.0/24",
						ConnectMode:     directPeering,
						Name:            "shared-network",
						Ip:              "10.0.0.2",
					},
					Tier:       "enterprise",
					KmsKeyName: "projects/test-project/locations/us-central1/keyRings/test-cmek-key-ring/cryptoKeys/test-cmek-key-1",
				},
				{
					Name:     "test-instance-4",
					Project:  "test-project",
					Location: "us-central1",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
					Network: file.Network{
						ReservedIpRange: "10.0.0.0/24",
						ConnectMode:     directPeering,
						Name:            "shared-network",
						Ip:              "10.0.0.2",
					},
					Tier: "enterprise",
				},
				{
					Name:     "test-instance-5",
					Project:  "test-project",
					Location: "us-central1",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
					Network: file.Network{
						ReservedIpRange: "10.0.0.0/21",
						ConnectMode:     directPeering,
						Name:            "shared-network",
						Ip:              "10.0.0.2",
					},
					Tier:       "enterprise",
					KmsKeyName: "projects/test-project/locations/us-central1/keyRings/test-cmek-key-ring/cryptoKeys/test-cmek-key",
				},
			},
			expectedList: []*file.MultishareInstance{
				{
					Name:     "test-instance-5",
					Project:  "test-project",
					Location: "us-central1",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
					Network: file.Network{
						ReservedIpRange: "10.0.0.0/21",
						ConnectMode:     directPeering,
						Name:            "shared-network",
						Ip:              "10.0.0.2",
					},
					Tier:       "enterprise",
					KmsKeyName: "projects/test-project/locations/us-central1/keyRings/test-cmek-key-ring/cryptoKeys/test-cmek-key",
				},
			},
		},
		{
			name: "invalid reserved-ipv4-cidr",
			req: &csi.CreateVolumeRequest{
				Parameters: map[string]string{
					paramMultishareInstanceScLabel: "testprefix",
					paramReservedIPV4CIDR:          "test-ip-range",
				},
			},
			target: &file.MultishareInstance{
				Name:     "test-target-instance",
				Project:  "test-project",
				Location: "us-central1",
				Labels: map[string]string{
					util.ParamMultishareInstanceScLabelKey: "testprefix",
				},
			},
			initInstanceList: []*file.MultishareInstance{
				{
					Name:     "test-instance",
					Project:  "test-project",
					Location: "us-central1",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
				},
			},
			expectError: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cloudProvider, err := cloud.NewFakeCloud()
			if err != nil {
				t.Fatalf("failed to initialize Provider: %v", err)
			}

			for _, i := range tc.initInstanceList {
				cloudProvider.File.StartCreateMultishareInstanceOp(context.Background(), i)
			}

			manager := NewMultishareOpsManager(cloudProvider)
			filteredList, err := manager.listMatchedInstances(context.Background(), tc.req, tc.target)
			if tc.expectError && err == nil {
				t.Errorf("expected error: %v", err)
			}
			if !tc.expectError && err != nil {
				t.Errorf("unexpectded error: %v", err)
			}
			for _, fi := range filteredList {
				if !found(tc.expectedList, fi) {
					t.Errorf("Failed to find instance")
				}
			}
		})
	}
}

func TestContainsOpWithInstanceTargetPrefix(t *testing.T) {
	tests := []struct {
		name          string
		inputInstance *file.MultishareInstance
		inputOps      []*OpInfo
		opExpected    bool
		errorExpected bool
	}{
		{
			name: "empty ops list",
			inputInstance: &file.MultishareInstance{
				Name:     "test-instance",
				Project:  "test-project",
				Location: "us-central1",
			},
		},
		{
			name: "invalid instance, missing location",
			inputInstance: &file.MultishareInstance{
				Name:    "test-instance",
				Project: "test-project",
			},
			errorExpected: true,
		},
		{
			name: "invalid instance, missing project",
			inputInstance: &file.MultishareInstance{
				Name:     "test-instance",
				Location: "us-central1",
			},
			errorExpected: true,
		},
		{
			name: "invalid instance, missing name",
			inputInstance: &file.MultishareInstance{
				Location: "us-central1",
				Project:  "test-project",
			},
			errorExpected: true,
		},
		{
			name: "valid instance, no running instance prefixed op",
			inputInstance: &file.MultishareInstance{
				Location: "us-central1",
				Project:  "test-project",
				Name:     "test-instance",
			},
			inputOps: []*OpInfo{
				{
					Id:     "op1",
					Type:   util.InstanceCreate,
					Target: "projects/test-project/locations/us-central1/instances/test-instance1",
				},
			},
		},
		{
			name: "valid instance, running instance op",
			inputInstance: &file.MultishareInstance{
				Location: "us-central1",
				Project:  "test-project",
				Name:     "test-instance",
			},
			inputOps: []*OpInfo{
				{
					Id:     "op1",
					Type:   util.InstanceCreate,
					Target: "projects/test-project/locations/us-central1/instances/test-instance",
				},
			},
			opExpected: true,
		},
		{
			name: "valid instance, running share op",
			inputInstance: &file.MultishareInstance{
				Location: "us-central1",
				Project:  "test-project",
				Name:     "test-instance",
			},
			inputOps: []*OpInfo{
				{
					Id:     "op1",
					Type:   util.InstanceCreate,
					Target: "projects/test-project/locations/us-central1/instances/test-instance/shares/test-share",
				},
			},
			opExpected: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			op, err := containsOpWithInstanceTargetPrefix(tc.inputInstance, tc.inputOps)
			if tc.errorExpected && err == nil {
				t.Errorf("expected error, found none")
			}
			if !tc.errorExpected && err != nil {
				t.Errorf("unexpected error")
			}

			if tc.opExpected && op == nil {
				t.Errorf("expected op, found none")
			}
			if !tc.opExpected && op != nil {
				t.Errorf("unexpected op")
			}
		})
	}
}

func TestContainsOpWithShareName(t *testing.T) {
	tests := []struct {
		name       string
		shareName  string
		opType     util.OperationType
		inputops   []*OpInfo
		opExpected bool
	}{
		{
			name:      "empty input ops",
			shareName: "test-share",
		},
		{
			name:      "share not found in input ops",
			shareName: "test-share",
			inputops: []*OpInfo{
				{
					Id:     "op1",
					Type:   util.InstanceCreate,
					Target: "projects/test-project/locations/us-central1/instances/test-instance",
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			op := containsOpWithShareName(tc.shareName, tc.opType, tc.inputops)
			if tc.opExpected && op == nil {
				t.Errorf("expected op, found none")
			}
			if !tc.opExpected && op != nil {
				t.Errorf("unexpected op")
			}
		})
	}
}

func TestListMultishareResourceRunningOps(t *testing.T) {
	found := func(inputList []*OpInfo, i *OpInfo) bool {
		for _, f := range inputList {
			if i.Id == f.Id && i.Target == f.Target && i.Type == f.Type {
				return true
			}
		}
		return false
	}

	type OpItem struct {
		id     string
		target string
		verb   string
		done   bool
	}
	tests := []struct {
		name        string
		initOps     []*OpItem
		expectedOps []*OpInfo
	}{
		{
			name: "filter out done ops",
			initOps: []*OpItem{
				{
					id:     "op1",
					target: "projects/test-project/locations/us-central1/instances/test-instance",
					verb:   "create",
					done:   true,
				},
				{
					id:     "op2",
					target: "projects/test-project/locations/us-central1/instances/test-instance",
					verb:   "update",
				},
			},
			expectedOps: []*OpInfo{
				{
					Id:     "op2",
					Target: "projects/test-project/locations/us-central1/instances/test-instance",
					Type:   util.InstanceUpdate,
				},
			},
		},
		{
			name: "filter out done ops",
			initOps: []*OpItem{
				{
					id:     "op1",
					target: "projects/test-project/locations/us-central1/instances/test-instance",
					verb:   "create",
					done:   true,
				},
				{
					id:     "op2",
					target: "projects/test-project/locations/us-central1/instances/test-instance",
					verb:   "update",
				},
			},
			expectedOps: []*OpInfo{
				{
					Id:     "op2",
					Target: "projects/test-project/locations/us-central1/instances/test-instance",
					Type:   util.InstanceUpdate,
				},
			},
		},
		{
			name: "skip resources other than instance and shares",
			initOps: []*OpItem{
				{
					id:     "op1",
					target: "projects/test-project/locations/us-central1/instances/test-instance-1",
					verb:   "create",
				},
				{
					id:     "op2",
					target: "projects/test-project/locations/us-central1/instances/test-instance-2",
					verb:   "update",
				},
				{
					id:     "op3",
					target: "projects/test-project/locations/us-central1/backups/test-backup",
					verb:   "create",
				},
				{
					id:     "op4",
					target: "projects/test-project/locations/us-central1/snapshots/test-snapshot",
					verb:   "create",
				},
			},
			expectedOps: []*OpInfo{
				{
					Id:     "op1",
					Target: "projects/test-project/locations/us-central1/instances/test-instance-1",
					Type:   util.InstanceCreate,
				},
				{
					Id:     "op2",
					Target: "projects/test-project/locations/us-central1/instances/test-instance-2",
					Type:   util.InstanceUpdate,
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var v1beta1ops []*filev1beta1multishare.Operation
			for _, item := range tc.initOps {
				var meta filev1beta1multishare.OperationMetadata
				meta.Target = item.target
				meta.Verb = item.verb
				bytes, _ := json.Marshal(meta)
				v1beta1ops = append(v1beta1ops, &filev1beta1multishare.Operation{
					Name:     item.id,
					Done:     item.done,
					Metadata: bytes,
				})
			}

			s, err := file.NewFakeServiceForMultishare(nil, nil, v1beta1ops)
			if err != nil {
				t.Fatalf("failed to fake service: %v", err)
			}
			cloudProvider, err := cloud.NewFakeCloud()
			cloudProvider.File = s
			manager := NewMultishareOpsManager(cloudProvider)
			ops, err := manager.listMultishareResourceRunningOps(context.Background())
			if err != nil {
				t.Fatalf("failed to initialize GCFS service: %v", err)
			}
			for _, o := range ops {
				if !found(tc.expectedOps, o) {
					t.Errorf("unexpected op")
				}
			}
		})
	}
}

func TestVerifyNoRunningInstanceOps(t *testing.T) {
	tests := []struct {
		name          string
		ops           []*OpInfo
		instance      *file.MultishareInstance
		errorExpected bool
	}{
		{
			name: "no error",
			ops: []*OpInfo{
				{
					Id:     "op1",
					Target: "projects/test-project/locations/us-central1/instances/test-instance-1",
				},
			},
			instance: &file.MultishareInstance{
				Name:     "test-instance-2",
				Project:  "test-project",
				Location: "us-central1",
			},
		},
		{
			name: "invalid instance case1",
			instance: &file.MultishareInstance{
				Project:  "test-project",
				Location: "us-central1",
			},
			errorExpected: true,
		},
		{
			name: "invalid instance case2",
			instance: &file.MultishareInstance{
				Name:     "test-instance-2",
				Location: "us-central1",
			},
			errorExpected: true,
		},
		{
			name: "invalid instance case3",
			instance: &file.MultishareInstance{
				Name:    "test-instance-2",
				Project: "test-project",
			},
			errorExpected: true,
		},
		{
			name: "error found running op",
			instance: &file.MultishareInstance{
				Name:     "test-instance-1",
				Project:  "test-project",
				Location: "us-central1",
			},
			ops: []*OpInfo{
				{
					Id:     "op1",
					Target: "projects/test-project/locations/us-central1/instances/test-instance-1",
				},
			},
			errorExpected: true,
		},
		{
			name: "no running op match",
			instance: &file.MultishareInstance{
				Name:     "test-instance-1",
				Project:  "test-project",
				Location: "us-central1",
			},
			ops: []*OpInfo{
				{
					Id:     "op1",
					Target: "projects/test-project/locations/us-central1/instances/test-instance-12",
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, err := file.NewFakeServiceForMultishare(nil, nil, nil)
			if err != nil {
				t.Fatalf("failed to fake service: %v", err)
			}
			cloudProvider, err := cloud.NewFakeCloud()
			cloudProvider.File = s
			manager := NewMultishareOpsManager(cloudProvider)
			err = manager.verifyNoRunningInstanceOps(tc.instance, tc.ops)
			if tc.errorExpected && err == nil {
				t.Errorf("expected error, found none")
			}
			if !tc.errorExpected && err != nil {
				t.Errorf("unexpected error")
			}
		})
	}
}

func TestVerifyNoRunningInstanceOrShareOpsForInstance(t *testing.T) {
	tests := []struct {
		name          string
		ops           []*OpInfo
		instance      *file.MultishareInstance
		errorExpected bool
	}{
		{
			name: "no error, no matching instance",
			ops: []*OpInfo{
				{
					Id:     "op1",
					Target: "projects/test-project/locations/us-central1/instances/test-instance-12",
				},
				{
					Id:     "op2",
					Target: "projects/test-project/locations/us-central1/instances/test-instance-12/shares/share-1",
				},
			},
			instance: &file.MultishareInstance{
				Name:     "test-instance-1",
				Project:  "test-project",
				Location: "us-central1",
			},
		},
		{
			name: "invalid instance case1",
			instance: &file.MultishareInstance{
				Project:  "test-project",
				Location: "us-central1",
			},
			errorExpected: true,
		},
		{
			name: "invalid instance case2",
			instance: &file.MultishareInstance{
				Name:     "test-instance-2",
				Location: "us-central1",
			},
			errorExpected: true,
		},
		{
			name: "invalid instance case3",
			instance: &file.MultishareInstance{
				Name:    "test-instance-2",
				Project: "test-project",
			},
			errorExpected: true,
		},
		{
			name: "error, matching instance op",
			ops: []*OpInfo{
				{
					Id:     "op1",
					Target: "projects/test-project/locations/us-central1/instances/test-instance-1",
				},
			},
			instance: &file.MultishareInstance{
				Name:     "test-instance-1",
				Project:  "test-project",
				Location: "us-central1",
			},
			errorExpected: true,
		},
		{
			name: "error, matching share op with instance prefix",
			ops: []*OpInfo{
				{
					Id:     "op2",
					Target: "projects/test-project/locations/us-central1/instances/test-instance-1/shares/share-1",
				},
			},
			instance: &file.MultishareInstance{
				Name:     "test-instance-1",
				Project:  "test-project",
				Location: "us-central1",
			},
			errorExpected: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, err := file.NewFakeServiceForMultishare(nil, nil, nil)
			if err != nil {
				t.Fatalf("failed to fake service: %v", err)
			}
			cloudProvider, err := cloud.NewFakeCloud()
			cloudProvider.File = s
			manager := NewMultishareOpsManager(cloudProvider)
			err = manager.verifyNoRunningInstanceOrShareOpsForInstance(tc.instance, tc.ops)
			if tc.errorExpected && err == nil {
				t.Errorf("expected error, found none")
			}
			if !tc.errorExpected && err != nil {
				t.Errorf("unexpected error")
			}
		})
	}
}

func TestVerifyNoRunningShareOps(t *testing.T) {
	tests := []struct {
		name          string
		ops           []*OpInfo
		share         *file.Share
		errorExpected bool
	}{
		{
			name: "no error, no matching op",
			ops: []*OpInfo{
				{
					Id:     "op1",
					Target: "projects/test-project/locations/us-central1/instances/test-instance-12",
				},
				{
					Id:     "op2",
					Target: "projects/test-project/locations/us-central1/instances/test-instance-12/shares/share-1",
				},
			},
			share: &file.Share{
				Parent: &file.MultishareInstance{
					Name:     "test-instance-1",
					Project:  "test-project",
					Location: "us-central1",
				},
				Name: "share-1",
			},
		},
		{
			name: "invalid share case1",
			share: &file.Share{
				Parent: &file.MultishareInstance{
					Name:     "test-instance-1",
					Location: "us-central1",
				},
				Name: "share-1",
			},
			errorExpected: true,
		},
		{
			name: "invalid share case2",
			share: &file.Share{
				Parent: &file.MultishareInstance{
					Name:    "test-instance-1",
					Project: "test-project",
				},
				Name: "share-1",
			},
			errorExpected: true,
		},
		{
			name: "invalid share case3",
			share: &file.Share{
				Parent: &file.MultishareInstance{
					Project:  "test-project",
					Location: "us-central1",
				},
				Name: "share-1",
			},
			errorExpected: true,
		},
		{
			name: "invalid share case3",
			share: &file.Share{
				Parent: &file.MultishareInstance{
					Name:     "test-instance-1",
					Project:  "test-project",
					Location: "us-central1",
				},
			},
			errorExpected: true,
		},
		{
			name: "error, found matching share op",
			ops: []*OpInfo{
				{
					Id:     "op1",
					Target: "projects/test-project/locations/us-central1/instances/test-instance-1/shares/share-1",
				},
			},
			share: &file.Share{
				Parent: &file.MultishareInstance{
					Name:     "test-instance-1",
					Project:  "test-project",
					Location: "us-central1",
				},
				Name: "share-1",
			},
			errorExpected: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, err := file.NewFakeServiceForMultishare(nil, nil, nil)
			if err != nil {
				t.Fatalf("failed to fake service: %v", err)
			}
			cloudProvider, err := cloud.NewFakeCloud()
			cloudProvider.File = s
			manager := NewMultishareOpsManager(cloudProvider)
			err = manager.verifyNoRunningShareOps(tc.share, tc.ops)
			if tc.errorExpected && err == nil {
				t.Errorf("expected error, found none")
			}
			if !tc.errorExpected && err != nil {
				t.Errorf("unexpected error")
			}
		})
	}
}

func TestRunEligibleInstanceCheck(t *testing.T) {
	found := func(inputList []*file.MultishareInstance, i *file.MultishareInstance) bool {
		for _, f := range inputList {
			if f.Project == i.Project && f.Location == i.Location && f.Name == i.Name {
				return true
			}
		}
		return false
	}
	tests := []struct {
		name                  string
		ops                   []*OpInfo
		initInstances         []*file.MultishareInstance
		initShares            []*file.Share
		expectedNonReadyCount int
		expectedReadyInstance []*file.MultishareInstance
		req                   *csi.CreateVolumeRequest
		target                *file.MultishareInstance
	}{
		{
			name: "no instances",
		},
		{
			name: "all ready instances",
			req: &csi.CreateVolumeRequest{
				Parameters: map[string]string{
					paramMultishareInstanceScLabel: "testprefix",
				},
			},
			target: &file.MultishareInstance{
				Name:     "test-target-instance",
				Project:  "test-project",
				Location: "us-central1",
				Labels: map[string]string{
					util.ParamMultishareInstanceScLabelKey: "testprefix",
				},
			},
			initInstances: []*file.MultishareInstance{
				{
					Name:     "test-instance-1",
					Location: "us-central1",
					Project:  "test-project",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
					State: "READY",
				},
				{
					Name:     "test-instance-1",
					Location: "us-central1",
					Project:  "test-project",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
					State: "READY",
				},
			},
			expectedReadyInstance: []*file.MultishareInstance{
				{
					Name:     "test-instance-1",
					Location: "us-central1",
					Project:  "test-project",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
					State: "READY",
				},
				{
					Name:     "test-instance-2",
					Location: "us-central1",
					Project:  "test-project",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
					State: "READY",
				},
			},
		},
		{
			name: "non-ready instances (instance update)",
			req: &csi.CreateVolumeRequest{
				Parameters: map[string]string{
					paramMultishareInstanceScLabel: "testprefix",
				},
			},
			target: &file.MultishareInstance{
				Name:     "test-target-instance",
				Project:  "test-project",
				Location: "us-central1",
				Labels: map[string]string{
					util.ParamMultishareInstanceScLabelKey: "testprefix",
				},
			},
			initInstances: []*file.MultishareInstance{
				{
					Name:     "instance-1",
					Location: "us-central1",
					Project:  "test-project",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
					State: "READY",
				},
			},
			expectedNonReadyCount: 1,
			ops: []*OpInfo{
				{
					Id:     "op1",
					Target: "projects/test-project/locations/us-central1/instances/instance-1",
					Type:   util.InstanceUpdate,
				},
			},
		},
		{
			name: "non-ready instances (share create)",
			req: &csi.CreateVolumeRequest{
				Parameters: map[string]string{
					paramMultishareInstanceScLabel: "testprefix",
				},
			},
			target: &file.MultishareInstance{
				Name:     "test-target-instance",
				Project:  "test-project",
				Location: "us-central1",
				Labels: map[string]string{
					util.ParamMultishareInstanceScLabelKey: "testprefix",
				},
			},
			initInstances: []*file.MultishareInstance{
				{
					Name:     "instance-1",
					Location: "us-central1",
					Project:  "test-project",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
					State: "READY",
				},
			},
			expectedNonReadyCount: 1,
			ops: []*OpInfo{
				{
					Id:     "op1",
					Target: "projects/test-project/locations/us-central1/instances/instance-1/shares/share-1",
					Type:   util.ShareCreate,
				},
			},
		},
		{
			name: "non-ready instances (share update)",
			req: &csi.CreateVolumeRequest{
				Parameters: map[string]string{
					paramMultishareInstanceScLabel: "testprefix",
				},
			},
			target: &file.MultishareInstance{
				Name:     "test-target-instance",
				Project:  "test-project",
				Location: "us-central1",
				Labels: map[string]string{
					util.ParamMultishareInstanceScLabelKey: "testprefix",
				},
			},
			initInstances: []*file.MultishareInstance{
				{
					Name:     "instance-1",
					Location: "us-central1",
					Project:  "test-project",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
					State: "READY",
				},
			},
			expectedNonReadyCount: 1,
			ops: []*OpInfo{
				{
					Id:     "op1",
					Target: "projects/test-project/locations/us-central1/instances/instance-1/shares/share-1",
					Type:   util.ShareUpdate,
				},
			},
		},
		{
			name: "non-ready instances (share delete)",
			req: &csi.CreateVolumeRequest{
				Parameters: map[string]string{
					paramMultishareInstanceScLabel: "testprefix",
				},
			},
			target: &file.MultishareInstance{
				Name:     "test-target-instance",
				Project:  "test-project",
				Location: "us-central1",
				Labels: map[string]string{
					util.ParamMultishareInstanceScLabelKey: "testprefix",
				},
			},
			initInstances: []*file.MultishareInstance{
				{
					Name:     "instance-1",
					Location: "us-central1",
					Project:  "test-project",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
					State: "READY",
				},
			},
			expectedNonReadyCount: 1,
			ops: []*OpInfo{
				{
					Id:     "op1",
					Target: "projects/test-project/locations/us-central1/instances/instance-1/shares/share-1",
					Type:   util.ShareDelete,
				},
			},
		},
		{
			name: "non-ready instances 0, instance delete not counted as ready",
			req: &csi.CreateVolumeRequest{
				Parameters: map[string]string{
					paramMultishareInstanceScLabel: "testprefix",
				},
			},
			target: &file.MultishareInstance{
				Name:     "test-target-instance",
				Project:  "test-project",
				Location: "us-central1",
				Labels: map[string]string{
					util.ParamMultishareInstanceScLabelKey: "testprefix",
				},
			},
			initInstances: []*file.MultishareInstance{
				{
					Name:     "instance-1",
					Location: "us-central1",
					Project:  "test-project",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
					State: "DELETING",
				},
			},
			ops: []*OpInfo{
				{
					Id:     "op1",
					Target: "projects/test-project/locations/us-central1/instances/instance-1",
					Type:   util.InstanceDelete,
				},
			},
		},
		{
			name: "non-ready instances (share delete), ready instance",
			req: &csi.CreateVolumeRequest{
				Parameters: map[string]string{
					paramMultishareInstanceScLabel: "testprefix",
				},
			},
			target: &file.MultishareInstance{
				Name:     "test-target-instance",
				Project:  "test-project",
				Location: "us-central1",
				Labels: map[string]string{
					util.ParamMultishareInstanceScLabelKey: "testprefix",
				},
			},
			initInstances: []*file.MultishareInstance{
				{
					Name:     "instance-1",
					Location: "us-central1",
					Project:  "test-project",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
					State: "READY",
				},
				{
					Name:     "instance-2",
					Location: "us-central1",
					Project:  "test-project",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
					State: "READY",
				},
			},
			expectedReadyInstance: []*file.MultishareInstance{
				{
					Name:     "instance-2",
					Location: "us-central1",
					Project:  "test-project",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
					State: "READY",
				},
			},
			expectedNonReadyCount: 1,
			ops: []*OpInfo{
				{
					Id:     "op1",
					Target: "projects/test-project/locations/us-central1/instances/instance-1/shares/share-1",
					Type:   util.ShareDelete,
				},
			},
		},
		{
			name: "no ready instance, no non-ready instance, instance with 10 shares not eligible",
			req: &csi.CreateVolumeRequest{
				Parameters: map[string]string{
					paramMultishareInstanceScLabel: "testprefix",
				},
			},
			target: &file.MultishareInstance{
				Name:     "test-target-instance",
				Project:  "test-project",
				Location: "us-central1",
				Labels: map[string]string{
					util.ParamMultishareInstanceScLabelKey: "testprefix",
				},
			},
			initInstances: []*file.MultishareInstance{
				{
					Name:     "instance-1",
					Location: "us-central1",
					Project:  "test-project",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
					State: "READY",
				},
			},
			initShares: []*file.Share{
				{
					Name: "share-1",
					Parent: &file.MultishareInstance{
						Name:     "instance-1",
						Project:  "test-project",
						Location: "us-central1",
						Labels: map[string]string{
							util.ParamMultishareInstanceScLabelKey: "testprefix",
						},
					},
				},
				{
					Name: "share-2",
					Parent: &file.MultishareInstance{
						Name:     "instance-1",
						Project:  "test-project",
						Location: "us-central1",
						Labels: map[string]string{
							util.ParamMultishareInstanceScLabelKey: "testprefix",
						},
					},
				},
				{
					Name: "share-3",
					Parent: &file.MultishareInstance{
						Name:     "instance-1",
						Project:  "test-project",
						Location: "us-central1",
						Labels: map[string]string{
							util.ParamMultishareInstanceScLabelKey: "testprefix",
						},
					},
				},
				{
					Name: "share-4",
					Parent: &file.MultishareInstance{
						Name:     "instance-1",
						Project:  "test-project",
						Location: "us-central1",
						Labels: map[string]string{
							util.ParamMultishareInstanceScLabelKey: "testprefix",
						},
					},
				},
				{
					Name: "share-5",
					Parent: &file.MultishareInstance{
						Name:     "instance-1",
						Project:  "test-project",
						Location: "us-central1",
						Labels: map[string]string{
							util.ParamMultishareInstanceScLabelKey: "testprefix",
						},
					},
				},
				{
					Name: "share-6",
					Parent: &file.MultishareInstance{
						Name:     "instance-1",
						Project:  "test-project",
						Location: "us-central1",
						Labels: map[string]string{
							util.ParamMultishareInstanceScLabelKey: "testprefix",
						},
					},
				},
				{
					Name: "share-7",
					Parent: &file.MultishareInstance{
						Name:     "instance-1",
						Project:  "test-project",
						Location: "us-central1",
						Labels: map[string]string{
							util.ParamMultishareInstanceScLabelKey: "testprefix",
						},
					},
				},
				{
					Name: "share-8",
					Parent: &file.MultishareInstance{
						Name:     "instance-1",
						Project:  "test-project",
						Location: "us-central1",
						Labels: map[string]string{
							util.ParamMultishareInstanceScLabelKey: "testprefix",
						},
					},
				},
				{
					Name: "share-9",
					Parent: &file.MultishareInstance{
						Name:     "instance-1",
						Project:  "test-project",
						Location: "us-central1",
						Labels: map[string]string{
							util.ParamMultishareInstanceScLabelKey: "testprefix",
						},
					},
				},
				{
					Name: "share-10",
					Parent: &file.MultishareInstance{
						Name:     "instance-1",
						Project:  "test-project",
						Location: "us-central1",
						Labels: map[string]string{
							util.ParamMultishareInstanceScLabelKey: "testprefix",
						},
					},
				},
			},
		},
		{
			name: "ready instance, non-ready instances, other instance state not count",
			req: &csi.CreateVolumeRequest{
				Parameters: map[string]string{
					paramMultishareInstanceScLabel: "testprefix",
				},
			},
			target: &file.MultishareInstance{
				Name:     "test-target-instance",
				Project:  "test-project",
				Location: "us-central1",
				Labels: map[string]string{
					util.ParamMultishareInstanceScLabelKey: "testprefix",
				},
			},
			initInstances: []*file.MultishareInstance{
				{
					Name:     "instance-1",
					Location: "us-central1",
					Project:  "test-project",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
					State: "CREATING",
				},
				{
					Name:     "instance-2",
					Location: "us-central1",
					Project:  "test-project",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
					State: "REPAIRING",
				},
				{
					Name:     "instance-3",
					Location: "us-central1",
					Project:  "test-project",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
					State: "READY",
				},
				{
					Name:     "instance-4",
					Location: "us-central1",
					Project:  "test-project",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
					State: "READY",
				},
				{
					Name:     "instance-5",
					Location: "us-central1",
					Project:  "test-project",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
					State: "ERROR",
				},
				{
					Name:     "instance-6",
					Location: "us-central1",
					Project:  "test-project",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
					State: "SUSPENDED",
				},
			},
			expectedReadyInstance: []*file.MultishareInstance{
				{
					Name:     "instance-3",
					Location: "us-central1",
					Project:  "test-project",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
					State: "READY",
				},
			},
			expectedNonReadyCount: 3,
			ops: []*OpInfo{
				{
					Id:     "op1",
					Target: "projects/test-project/locations/us-central1/instances/instance-4/shares/share-1",
					Type:   util.ShareDelete,
				},
			},
		},
		{
			name: "creating instance count as non-ready",
			req: &csi.CreateVolumeRequest{
				Parameters: map[string]string{
					paramMultishareInstanceScLabel: "testprefix",
				},
			},
			target: &file.MultishareInstance{
				Name:     "test-target-instance",
				Project:  "test-project",
				Location: "us-central1",
				Labels: map[string]string{
					util.ParamMultishareInstanceScLabelKey: "testprefix",
				},
			},
			initInstances: []*file.MultishareInstance{
				{
					Name:     "instance-1",
					Location: "us-central1",
					Project:  "test-project",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
					State: "CREATING",
				},
				{
					Name:     "instance-2",
					Location: "us-central1",
					Project:  "test-project",
					Labels: map[string]string{
						util.ParamMultishareInstanceScLabelKey: "testprefix",
					},
					State: "ERROR",
				},
			},
			expectedNonReadyCount: 1,
			ops: []*OpInfo{
				{
					Id:     "op1",
					Target: "projects/test-project/locations/us-central1/instances/instance-1",
					Type:   util.InstanceCreate,
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, err := file.NewFakeServiceForMultishare(tc.initInstances, tc.initShares, nil)
			if err != nil {
				t.Fatalf("failed to fake service: %v", err)
			}
			cloudProvider, err := cloud.NewFakeCloud()
			cloudProvider.File = s
			manager := NewMultishareOpsManager(cloudProvider)
			ready, nonReady, err := manager.runEligibleInstanceCheck(context.Background(), tc.req, tc.ops, tc.target)
			if err != nil {
				t.Errorf("unexpected error")
			}
			if nonReady != tc.expectedNonReadyCount {
				t.Errorf("got %d, want %d", nonReady, tc.expectedNonReadyCount)
			}
			for _, r := range ready {
				if !found(tc.expectedReadyInstance, r) {
					t.Errorf("expected instance not ready")
				}
			}
		})
	}
}
