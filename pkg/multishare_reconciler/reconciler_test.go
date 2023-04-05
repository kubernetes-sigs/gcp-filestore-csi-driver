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

package multishare_reconciler

import (
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/strings/slices"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/apis/multishare/v1alpha1"
	cloud "sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/file"
	driver "sigs.k8s.io/gcp-filestore-csi-driver/pkg/csi_driver"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/util"
)

func TestManagedInstanceAndShare(t *testing.T) {
	testClusterName := "testCluster"
	usCentral1c := "us-central1-c"
	usEast1c := "us-east1-c"
	usCentral1 := "us-central1"
	usEast1 := "us-East1"
	instance1 := "instance-1"
	share1 := "share-1"
	share2 := "share-2"
	testProject := "testProject"

	cases := []struct {
		name                         string
		clusterName                  string
		cloudZone                    string
		isRegional                   bool
		instances                    []*file.MultishareInstance
		shares                       []*file.Share
		expectedInstanceNames        []string
		expectedShareNames           []string
		expectedinstanceShareMapping map[string][]string
	}{
		{
			name:                         "empty results",
			clusterName:                  testClusterName,
			cloudZone:                    usCentral1c,
			isRegional:                   true,
			instances:                    []*file.MultishareInstance{},
			shares:                       []*file.Share{},
			expectedInstanceNames:        []string{},
			expectedShareNames:           []string{},
			expectedinstanceShareMapping: map[string][]string{},
		},
		{
			name:        "matching instance and shares",
			clusterName: testClusterName,
			cloudZone:   usCentral1c,
			isRegional:  true,
			instances: []*file.MultishareInstance{
				{
					Name:     instance1,
					Project:  testProject,
					Location: usCentral1,
					Labels: map[string]string{
						driver.TagKeyClusterLocation:           usCentral1,
						driver.TagKeyClusterName:               testClusterName,
						util.ParamMultishareInstanceScLabelKey: "enterprise-rwx",
					},
				},
			},
			shares: []*file.Share{
				{
					Name: share1,
					Parent: &file.MultishareInstance{
						Project:  testProject,
						Name:     instance1,
						Location: usCentral1,
					},
				},
				{
					Name: share2,
					Parent: &file.MultishareInstance{
						Project:  testProject,
						Name:     instance1,
						Location: usCentral1,
					},
				},
			},
			expectedInstanceNames: []string{instance1},
			expectedShareNames:    []string{share1, share2},
			expectedinstanceShareMapping: map[string][]string{
				instanceURI(testProject, usCentral1, instance1): {share1, share2},
			},
		},
		{
			name:        "instance tag not matching",
			clusterName: testClusterName,
			cloudZone:   usEast1c,
			isRegional:  false,
			instances: []*file.MultishareInstance{
				{
					Name:     instance1,
					Project:  testProject,
					Location: usCentral1,
					Labels: map[string]string{
						driver.TagKeyClusterLocation:           usCentral1,
						driver.TagKeyClusterName:               testClusterName,
						util.ParamMultishareInstanceScLabelKey: "enterprise-rwx",
					},
				},
			},
			shares: []*file.Share{
				{
					Name: share1,
					Parent: &file.MultishareInstance{
						Project:  testProject,
						Name:     instance1,
						Location: usCentral1,
					},
				},
				{
					Name: share2,
					Parent: &file.MultishareInstance{
						Project:  testProject,
						Name:     instance1,
						Location: usCentral1,
					},
				},
			},
			expectedInstanceNames: []string{},
			expectedShareNames:    []string{},
			expectedinstanceShareMapping: map[string][]string{
				instanceURI(testProject, usCentral1, instance1): {},
			},
		},
		{
			name:        "share parent not matching instance",
			clusterName: testClusterName,
			cloudZone:   usCentral1c,
			isRegional:  true,
			instances: []*file.MultishareInstance{
				{
					Name:     instance1,
					Project:  testProject,
					Location: usCentral1,
					Labels: map[string]string{
						driver.TagKeyClusterLocation:           usCentral1,
						driver.TagKeyClusterName:               testClusterName,
						util.ParamMultishareInstanceScLabelKey: "enterprise-rwx",
					},
				},
			},
			shares: []*file.Share{
				{
					Name: share1,
					Parent: &file.MultishareInstance{
						Project:  testProject,
						Name:     instance1,
						Location: usEast1,
					},
				},
				{
					Name: share2,
					Parent: &file.MultishareInstance{
						Project:  testProject,
						Name:     instance1,
						Location: usCentral1,
					},
				},
			},
			expectedInstanceNames: []string{instance1},
			expectedShareNames:    []string{share2},
			expectedinstanceShareMapping: map[string][]string{
				instanceURI(testProject, usCentral1, instance1): {share2},
			},
		},
	}

	for _, test := range cases {
		recon := &multishareReconciler{
			config: &driver.GCFSDriverConfig{
				ClusterName: test.clusterName,
				IsRegional:  test.isRegional,
			},
			cloud: &cloud.Cloud{
				Zone: test.cloudZone,
			},
		}
		instances, shares, shareMap, _ := recon.managedInstanceAndShare(test.instances, test.shares)
		if len(instances) != len(test.expectedInstanceNames) {
			t.Errorf("want %d instances, got %d", len(test.expectedInstanceNames), len(instances))
		}
		if len(shares) != len(test.expectedShareNames) {
			t.Errorf("want %d shares, got %d", len(test.expectedShareNames), len(shares))
		}
		for _, instance := range instances {
			if !slices.Contains(test.expectedInstanceNames, instance.Name) {
				t.Errorf("want instance with names %v but got %q", test.expectedInstanceNames, instance.Name)
			}
		}
		for _, share := range shares {
			if !slices.Contains(test.expectedShareNames, share.Name) {
				t.Errorf("want instances with names %v but got %q", test.expectedShareNames, share.Name)
			}
		}
		for instanceURI, shareList := range shareMap {
			for _, share := range shareList {
				if !slices.Contains(test.expectedinstanceShareMapping[instanceURI], share.Name) {
					t.Errorf("instance %q should map to %v but got %q", instanceURI, test.expectedinstanceShareMapping[instanceURI], share.Name)
				}
			}
		}
	}
}

func TestInstanceEmpty(t *testing.T) {
	cases := []struct {
		name     string
		instance *v1alpha1.InstanceInfo
		expected bool
	}{
		{
			name: "empty instance",
			instance: &v1alpha1.InstanceInfo{
				Status: &v1alpha1.InstanceInfoStatus{
					ShareNames: []string{},
				},
			},
			expected: true,
		},
		{
			name: "non-empty instance",
			instance: &v1alpha1.InstanceInfo{
				Status: &v1alpha1.InstanceInfoStatus{
					ShareNames: []string{"share1"},
				},
			},
			expected: false,
		},
		{
			name:     "nil status instance",
			instance: &v1alpha1.InstanceInfo{},
			expected: false,
		},
	}

	for _, test := range cases {
		empty := instanceEmpty(test.instance)
		if empty != test.expected {
			t.Errorf("case %s result %v, want %v", test.name, empty, test.expected)
		}
	}
}

func TestMaybeAddCleanupFinalizer(t *testing.T) {
	now := metav1.Now()
	cases := []struct {
		name     string
		instance *v1alpha1.InstanceInfo
		expected bool
	}{
		{
			name:     "no deletionTimestamp",
			instance: &v1alpha1.InstanceInfo{},
			expected: true,
		},
		{
			name: "deletionTimestamp set",
			instance: &v1alpha1.InstanceInfo{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &now,
				},
			},
			expected: false,
		},
		{
			name: "replace finalizer",
			instance: &v1alpha1.InstanceInfo{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{"thisNThat"},
				},
			},
			expected: true,
		},
	}

	for _, test := range cases {
		instanceInfo, updated := maybeAddCleanupFinalizer(test.instance)
		if updated != test.expected {
			t.Errorf("case %s produce %v, want %v", test.name, updated, test.expected)
		}
		if updated {
			if len(instanceInfo.Finalizers) != 1 || instanceInfo.Finalizers[0] != util.FilestoreResourceCleanupFinalizer {
				t.Errorf("case %s wants Finalizer to be [%s] but got %v", test.name, util.FilestoreResourceCleanupFinalizer, instanceInfo.Finalizers)
			}
		}
	}
}

func instanceURI(project, location, name string) string {
	return fmt.Sprintf("projects/%s/locations/%s/instances/%s", project, location, name)
}
