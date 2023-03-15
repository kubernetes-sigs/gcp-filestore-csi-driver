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

package driver

import (
	"context"
	"reflect"
	"testing"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/apis/multishare/v1alpha1"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/client/clientset/versioned/fake"
	informers "sigs.k8s.io/gcp-filestore-csi-driver/pkg/client/informers/externalversions"
	cloud "sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/file"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/util"
)

func initTestMultishareStatefulController(t *testing.T) *MultishareStatefulController {
	fileService, err := file.NewFakeService()
	if err != nil {
		t.Fatalf("failed to initialize GCFS service: %v", err)
	}

	cloudProvider, err := cloud.NewFakeCloud()
	if err != nil {
		t.Fatalf("Failed to get cloud provider: %v", err)
	}
	config := &controllerServerConfig{
		driver:          initTestDriver(t),
		fileService:     fileService,
		cloud:           cloudProvider,
		volumeLocks:     util.NewVolumeLocks(),
		ecfsDescription: "",
		isRegional:      true,
		clusterName:     testClusterName,
	}
	config.features = &GCFSDriverFeatureOptions{
		FeatureStateful: &FeatureStateful{},
	}

	mc := NewMultishareController(config)
	msc := NewMultishareStatefulController(config)
	msc.mc = mc

	client := fake.NewSimpleClientset()
	factory := informers.NewSharedInformerFactory(client, 0)
	msc.clientset = client
	msc.shareLister = factory.Multishare().V1alpha1().ShareInfos().Lister()
	return msc
}

// this test does not support creation successful case
func TestStatefulCreateVolume(t *testing.T) {
	testVolName_0 := "pvc-" + string(uuid.NewUUID())
	testShareName_0 := util.ConvertVolToShareName(testVolName_0)
	testInstanceName_0 := "fs-" + string(uuid.NewUUID())

	tests := []struct {
		name          string
		initSi        []*v1alpha1.ShareInfo
		req           *csi.CreateVolumeRequest
		resp          *csi.CreateVolumeResponse
		errorExpected bool
		expectedSi    []*v1alpha1.ShareInfo
	}{
		{
			name: "first create call, return waiting error",
			req: &csi.CreateVolumeRequest{
				Name: testVolName_0,
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 100 * util.Gb,
				},
				Parameters: map[string]string{
					ParamMultishareInstanceScLabel: testInstanceScPrefix,
				},
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
				},
			},
			errorExpected: true,
			expectedSi: []*v1alpha1.ShareInfo{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:       testVolName_0,
						Finalizers: []string{util.FilestoreResourceCleanupFinalizer},
						Labels:     map[string]string{},
					},
					Spec: v1alpha1.ShareInfoSpec{
						ShareName:       testShareName_0,
						CapacityBytes:   100 * util.Gb,
						InstancePoolTag: testInstanceScPrefix,
						Region:          testRegion,
					},
				},
			},
		},
		{
			name: "shareInfo with invalid instanceHandle",
			req: &csi.CreateVolumeRequest{
				Name: testVolName_0,
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 100 * util.Gb,
				},
				Parameters: map[string]string{
					ParamMultishareInstanceScLabel: testInstanceScPrefix,
				},
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
				},
			},
			errorExpected: true,
			initSi: []*v1alpha1.ShareInfo{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:       testVolName_0,
						Finalizers: []string{util.FilestoreResourceCleanupFinalizer},
						Labels:     map[string]string{},
					},
					Spec: v1alpha1.ShareInfoSpec{
						ShareName:       testShareName_0,
						CapacityBytes:   100 * util.Gb,
						InstancePoolTag: testInstanceScPrefix,
						Region:          testRegion,
					},
					Status: &v1alpha1.ShareInfoStatus{
						InstanceHandle: testInstanceName_0,
						CapacityBytes:  100 * util.Gb,
						ShareStatus:    v1alpha1.READY,
					},
				},
			},
		},
	}

	for _, tc := range tests {
		msc := initTestMultishareStatefulController(t)
		for _, si := range tc.initSi {
			msc.clientset.MultishareV1alpha1().ShareInfos().Create(context.TODO(), si, metav1.CreateOptions{})
		}
		resp, err := msc.CreateVolume(context.TODO(), tc.req)
		if tc.errorExpected && err == nil {
			t.Errorf("expected error not found")
		}
		if !tc.errorExpected && err != nil {
			t.Errorf("unexpected error %s", err.Error())
		}
		if !reflect.DeepEqual(resp, tc.resp) {
			t.Errorf("got resp %+v, expected %+v", resp, tc.resp)
		}
		shareInfoList, err := msc.clientset.MultishareV1alpha1().ShareInfos().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			t.Errorf("unexpected list error")
		}
		shareInfos := shareInfoList.Items
		for _, expected := range tc.expectedSi {
			found := false
			for _, shareInfo := range shareInfos {
				if reflect.DeepEqual(shareInfo, *expected) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected si object %+v not found", expected)
			}
		}
	}
}
