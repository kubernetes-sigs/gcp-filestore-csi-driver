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

package lockrelease

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestParseConfigMapKey(t *testing.T) {
	cases := []struct {
		name                   string
		key                    string
		expectedProjectID      string
		expectedLocation       string
		expectedFilestoreName  string
		expectedShareName      string
		expectedNodeID         string
		expectedNodeInternalIP string
		expectErr              bool
	}{
		{
			name:      "invalid configmap key, key does not contain all desired elements",
			key:       "test-project.us-central1.test-filestore.test-share.123456",
			expectErr: true,
		},
		{
			name:      "invalid configmap key, too many elements",
			key:       "test-project.us-central1.test-filestore.test-share.123456.192.168.1.1",
			expectErr: true,
		},
		{
			name:      "invalid configmap key, invalid node internal ip",
			key:       "test-project.us-central1.test-filestore.test-share.123456.192_168_1_1_1",
			expectErr: true,
		},
		{
			name:      "invalid configmap key, elements include empty string",
			key:       "test-project.us-central1.test-filestore.test-share..192_168_1_1",
			expectErr: true,
		},
		{
			name:                   "valid configmap key",
			key:                    "test-project.us-central1.test-filestore.test-share.123456.192_168_1_1",
			expectedProjectID:      "test-project",
			expectedLocation:       "us-central1",
			expectedFilestoreName:  "test-filestore",
			expectedShareName:      "test-share",
			expectedNodeID:         "123456",
			expectedNodeInternalIP: "192.168.1.1",
		},
	}
	for _, test := range cases {
		projectID, location, filestoreName, shareName, gkeNodeID, gkeNodeInternalIP, err := ParseConfigMapKey(test.key)
		if gotExpected := gotExpectedError(test.name, test.expectErr, err); gotExpected != nil {
			t.Fatal(gotExpected)
		}
		if projectID != test.expectedProjectID {
			t.Errorf("test %q failed: got projectID %s, expected %s", test.name, projectID, test.expectedProjectID)
		}
		if location != test.expectedLocation {
			t.Errorf("test %q failed: got location %s, expected %s", test.name, location, test.expectedLocation)
		}
		if filestoreName != test.expectedFilestoreName {
			t.Errorf("test %q failed: got filestoreName %s, expected %s", test.name, filestoreName, test.expectedFilestoreName)
		}
		if shareName != test.expectedShareName {
			t.Errorf("test %q failed: got shareName %s, expected %s", test.name, shareName, test.expectedShareName)
		}
		if gkeNodeID != test.expectedNodeID {
			t.Errorf("test %q failed: got gkeNodeID %s, expected %s", test.name, gkeNodeID, test.expectedNodeID)
		}
		if gkeNodeInternalIP != test.expectedNodeInternalIP {
			t.Errorf("test %q failed: got gkeNodeInternalIP %s, expected %s", test.name, gkeNodeInternalIP, test.expectedNodeInternalIP)
		}
	}
}

func TestGKENodeNameFromConfigMap(t *testing.T) {
	cases := []struct {
		name             string
		configmap        *corev1.ConfigMap
		expectedNodeName string
		expectErr        bool
	}{
		{
			name: "configmap name does not contain fscsi prefix",
			configmap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fscsi",
				},
			},
			expectErr: true,
		},
		{
			name: "configmap name does not contain node name",
			configmap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fscsi-",
				},
			},
			expectErr: true,
		},
		{
			name: "valid configmap name",
			configmap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fscsi-node-name",
				},
			},
			expectedNodeName: "node-name",
		},
	}
	for _, test := range cases {
		nodeName, err := GKENodeNameFromConfigMap(test.configmap)
		if gotExpected := gotExpectedError(test.name, test.expectErr, err); gotExpected != nil {
			t.Fatal(gotExpected)
		}
		if nodeName != test.expectedNodeName {
			t.Errorf("test %q failed: got GKENodeName %s, expected %q", test.name, nodeName, test.expectedNodeName)
		}
	}
}

func TestGetConfigMap(t *testing.T) {
	cases := []struct {
		name        string
		cmName      string
		cmNamespace string
		existingCM  *corev1.ConfigMap
		expectedCM  *corev1.ConfigMap
		expectErr   bool
	}{
		{
			name:        "configmap not exist",
			cmName:      "fscsi-node-name",
			cmNamespace: "gcp-filestore-csi-driver",
			existingCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fscsi-node-name",
					Namespace: "default",
				},
			},
		},
		{
			name:        "configmap found",
			cmName:      "fscsi-node-name",
			cmNamespace: "gcp-filestore-csi-driver",
			existingCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fscsi-node-name",
					Namespace: "gcp-filestore-csi-driver",
				},
			},
			expectedCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fscsi-node-name",
					Namespace: "gcp-filestore-csi-driver",
				},
			},
		},
	}
	for _, test := range cases {
		client := fake.NewSimpleClientset(test.existingCM)
		controller := NewFakeLockReleaseControllerWithClient(client)
		cm, err := controller.GetConfigMap(context.Background(), test.cmName, test.cmNamespace)
		if gotExpected := gotExpectedError(test.name, test.expectErr, err); gotExpected != nil {
			t.Fatal(gotExpected)
		}
		if diff := cmp.Diff(test.expectedCM, cm); diff != "" {
			t.Errorf("test %q failed: unexpected diff (-want +got):\n%s", test.name, diff)
		}
	}
}

func TestUpdateConfigMapWithKeyValue(t *testing.T) {
	cases := []struct {
		name       string
		existingCM *corev1.ConfigMap
		key        string
		value      string
		expectedCM *corev1.ConfigMap
		expectErr  bool
	}{
		{
			name: "key already exist in configmap.data",
			existingCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "fscsi-node-name",
					Namespace:  "gcp-filestore-csi-driver",
					Finalizers: []string{ConfigMapFinalzer},
				},
				Data: map[string]string{
					"test-project.us-central1.test-filestore.test-share.123456.192_168_1_1": "192.168.92.0",
				},
			},
			key:   "test-project.us-central1.test-filestore.test-share.123456.192_168_1_1",
			value: "192.168.92.0",
			expectedCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "fscsi-node-name",
					Namespace:  "gcp-filestore-csi-driver",
					Finalizers: []string{ConfigMapFinalzer},
				},
				Data: map[string]string{
					"test-project.us-central1.test-filestore.test-share.123456.192_168_1_1": "192.168.92.0",
				},
			},
		},
		{
			name: "key already exist in configmap.data, configmap missing finalizer",
			existingCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "fscsi-node-name",
					Namespace:  "gcp-filestore-csi-driver",
					Finalizers: []string{ConfigMapFinalzer},
				},
				Data: map[string]string{
					"test-project.us-central1.test-filestore.test-share.123456.192_168_1_1": "192.168.92.0",
				},
			},
			key:   "test-project.us-central1.test-filestore.test-share.123456.192_168_1_1",
			value: "192.168.92.0",
			expectedCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "fscsi-node-name",
					Namespace:  "gcp-filestore-csi-driver",
					Finalizers: []string{ConfigMapFinalzer},
				},
				Data: map[string]string{
					"test-project.us-central1.test-filestore.test-share.123456.192_168_1_1": "192.168.92.0",
				},
			},
		},
		{
			name: "adding key value pair into configmap.data succeed",
			existingCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "fscsi-node-name",
					Namespace:  "gcp-filestore-csi-driver",
					Finalizers: []string{ConfigMapFinalzer},
				},
			},
			key:   "test-project.us-central1.test-filestore.test-share.123456.192_168_1_1",
			value: "192.168.92.0",
			expectedCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "fscsi-node-name",
					Namespace:  "gcp-filestore-csi-driver",
					Finalizers: []string{ConfigMapFinalzer},
				},
				Data: map[string]string{
					"test-project.us-central1.test-filestore.test-share.123456.192_168_1_1": "192.168.92.0",
				},
			},
		},
	}
	for _, test := range cases {
		client := fake.NewSimpleClientset(test.existingCM)
		controller := NewFakeLockReleaseControllerWithClient(client)
		ctx := context.Background()
		err := controller.UpdateConfigMapWithKeyValue(ctx, test.existingCM, test.key, test.value)
		if gotExpected := gotExpectedError(test.name, test.expectErr, err); gotExpected != nil {
			t.Fatal(gotExpected)
		}
		updatedCM, err := controller.GetConfigMap(ctx, test.expectedCM.Name, test.expectedCM.Namespace)
		if err != nil {
			t.Fatalf("test %q failed: unexpected error: %v", test.name, err)
		}
		if diff := cmp.Diff(test.expectedCM, updatedCM); diff != "" {
			t.Errorf("test %q failed: unexpected diff (-want +got):\n%s", test.name, diff)
		}
	}
}

func TestRemoveKeyFromConfigMap(t *testing.T) {
	cases := []struct {
		name       string
		existingCM *corev1.ConfigMap
		key        string
		expectedCM *corev1.ConfigMap
		expectErr  bool
	}{
		{
			name: "key exists in configmap.data, finalier exists",
			existingCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "fscsi-node-name",
					Namespace:  "gcp-filestore-csi-driver",
					Finalizers: []string{ConfigMapFinalzer},
				},
				Data: map[string]string{
					"test-project.us-central1.test-filestore.test-share.123456.192_168_1_1": "192.168.92.0",
					"test-project.us-central1.test-filestore.test-share.123456.192_168_1_2": "192.168.92.1",
				},
			},
			key: "test-project.us-central1.test-filestore.test-share.123456.192_168_1_2",
			expectedCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "fscsi-node-name",
					Namespace:  "gcp-filestore-csi-driver",
					Finalizers: []string{ConfigMapFinalzer},
				},
				Data: map[string]string{
					"test-project.us-central1.test-filestore.test-share.123456.192_168_1_1": "192.168.92.0",
				},
			},
		},
		{
			name: "key not exist in configmap.data",
			existingCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "fscsi-node-name",
					Namespace:  "gcp-filestore-csi-driver",
					Finalizers: []string{ConfigMapFinalzer},
				},
				Data: map[string]string{
					"test-project.us-central1.test-filestore.test-share.123456.192_168_1_1": "192.168.92.0",
				},
			},
			key: "test-project.us-central1.test-filestore.test-share.123456.192_168_1_2",
			expectedCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "fscsi-node-name",
					Namespace:  "gcp-filestore-csi-driver",
					Finalizers: []string{ConfigMapFinalzer},
				},
				Data: map[string]string{
					"test-project.us-central1.test-filestore.test-share.123456.192_168_1_1": "192.168.92.0",
				},
			},
		},
		{
			name: "configmap.data becomes empty after removing key",
			existingCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "fscsi-node-name",
					Namespace:  "gcp-filestore-csi-driver",
					Finalizers: []string{ConfigMapFinalzer},
				},
				Data: map[string]string{
					"test-project.us-central1.test-filestore.test-share.123456.192_168_1_1": "192.168.92.0",
				},
			},
			key: "test-project.us-central1.test-filestore.test-share.123456.192_168_1_1",
			expectedCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "fscsi-node-name",
					Namespace:  "gcp-filestore-csi-driver",
					Finalizers: []string{ConfigMapFinalzer},
				},
				Data: map[string]string{},
			},
		},
	}
	for _, test := range cases {
		client := fake.NewSimpleClientset(test.existingCM)
		controller := NewFakeLockReleaseControllerWithClient(client)
		ctx := context.Background()
		err := controller.RemoveKeyFromConfigMap(ctx, test.existingCM, test.key)
		if gotExpected := gotExpectedError(test.name, test.expectErr, err); gotExpected != nil {
			t.Fatal(gotExpected)
		}
		updatedCM, err := controller.GetConfigMap(ctx, test.expectedCM.Name, test.expectedCM.Namespace)
		if err != nil {
			t.Fatalf("test %q failed: unexpected error: %v", test.name, err)
		}
		if diff := cmp.Diff(test.expectedCM, updatedCM); diff != "" {
			t.Errorf("test %q failed: unexpected diff (-want +got):\n%s", test.name, diff)
		}
	}
}

func TestRemoveKeyFromConfigMapWithRetry(t *testing.T) {
	cases := []struct {
		name       string
		existingCM *corev1.ConfigMap
		key        string
		expectedCM *corev1.ConfigMap
		expectErr  bool
	}{
		{
			name: "key exists in configmap.data, finalier exists",
			existingCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "fscsi-node-name",
					Namespace:  "gcp-filestore-csi-driver",
					Finalizers: []string{ConfigMapFinalzer},
				},
				Data: map[string]string{
					"test-project.us-central1.test-filestore.test-share.123456.192_168_1_1": "192.168.92.0",
					"test-project.us-central1.test-filestore.test-share.123456.192_168_1_2": "192.168.92.1",
				},
			},
			key: "test-project.us-central1.test-filestore.test-share.123456.192_168_1_2",
			expectedCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "fscsi-node-name",
					Namespace:  "gcp-filestore-csi-driver",
					Finalizers: []string{ConfigMapFinalzer},
				},
				Data: map[string]string{
					"test-project.us-central1.test-filestore.test-share.123456.192_168_1_1": "192.168.92.0",
				},
			},
		},
		{
			name: "key not exist in configmap.data",
			existingCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "fscsi-node-name",
					Namespace:  "gcp-filestore-csi-driver",
					Finalizers: []string{ConfigMapFinalzer},
				},
				Data: map[string]string{
					"test-project.us-central1.test-filestore.test-share.123456.192_168_1_1": "192.168.92.0",
				},
			},
			key: "test-project.us-central1.test-filestore.test-share.123456.192_168_1_2",
			expectedCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "fscsi-node-name",
					Namespace:  "gcp-filestore-csi-driver",
					Finalizers: []string{ConfigMapFinalzer},
				},
				Data: map[string]string{
					"test-project.us-central1.test-filestore.test-share.123456.192_168_1_1": "192.168.92.0",
				},
			},
		},
		{
			name: "configmap.data becomes empty after removing key",
			existingCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "fscsi-node-name",
					Namespace:  "gcp-filestore-csi-driver",
					Finalizers: []string{ConfigMapFinalzer},
				},
				Data: map[string]string{
					"test-project.us-central1.test-filestore.test-share.123456.192_168_1_1": "192.168.92.0",
				},
			},
			key: "test-project.us-central1.test-filestore.test-share.123456.192_168_1_1",
			expectedCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "fscsi-node-name",
					Namespace:  "gcp-filestore-csi-driver",
					Finalizers: []string{ConfigMapFinalzer},
				},
				Data: map[string]string{},
			},
		},
	}
	for _, test := range cases {
		client := fake.NewSimpleClientset(test.existingCM)
		controller := NewFakeLockReleaseControllerWithClient(client)
		ctx := context.Background()
		err := controller.RemoveKeyFromConfigMapWithRetry(ctx, test.existingCM, test.key)
		if gotExpected := gotExpectedError(test.name, test.expectErr, err); gotExpected != nil {
			t.Fatal(gotExpected)
		}
		updatedCM, err := controller.GetConfigMap(ctx, test.expectedCM.Name, test.expectedCM.Namespace)
		if err != nil {
			t.Fatalf("test %q failed: unexpected error: %v", test.name, err)
		}
		if diff := cmp.Diff(test.expectedCM, updatedCM); diff != "" {
			t.Errorf("test %q failed: unexpected diff (-want +got):\n%s", test.name, diff)
		}
	}
}

func gotExpectedError(testFunc string, wantErr bool, err error) error {
	if err != nil && !wantErr {
		return fmt.Errorf("%s got error %v, want nil", testFunc, err)
	}
	if err == nil && wantErr {
		return fmt.Errorf("%s got nil, want error", testFunc)
	}
	return nil
}
