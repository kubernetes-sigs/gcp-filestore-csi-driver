package lockrelease

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

type MockEventProcessor struct {
	mock.Mock
}

func (m *MockEventProcessor) processConfigMapEntryOnNodeCreation(ctx context.Context, key string, filestoreIP string, node *corev1.Node, cm *corev1.ConfigMap) error {
	args := m.Called(ctx) // Pass the arguments used in On()
	if args.Error(0) != nil {
		return args.Error(0)
	}
	return nil
}

func (m *MockEventProcessor) processConfigMapEntryOnNodeUpdate(ctx context.Context, key string, filestoreIP string, newNode *corev1.Node, oldNode *corev1.Node, cm *corev1.ConfigMap) error {
	args := m.Called(ctx)
	if args.Error(0) != nil {
		return args.Error(0)
	}
	return nil
}

type MockLockService struct {
	mock.Mock
}

func (m *MockLockService) ReleaseLock(hostIP, clientIP string) error {
	args := m.Called()
	if args.Error(0) != nil {
		return args.Error(0)
	}
	return nil
}

func (m *MockEventProcessor) SetController(ctrl *LockReleaseController) {}

func TestVerifyConfigMapEntry(t *testing.T) {
	cases := []struct {
		name           string
		node           *corev1.Node
		gceInstanceID  string
		nodeInternalIP string
		expectExists   bool
		expectErr      bool
	}{
		{
			name:           "node is nil",
			gceInstanceID:  "12345",
			nodeInternalIP: "127.0.0.1",
			expectExists:   false,
		},
		{
			name: "node missing annotation",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{{Address: "127.0.0.1", Type: corev1.NodeInternalIP}},
				},
			},
			gceInstanceID:  "12345",
			nodeInternalIP: "127.0.0.1",
			expectExists:   false,
			expectErr:      true,
		},
		{
			name: "node missing instance id annotation",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-node",
					Annotations: map[string]string{},
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{{Address: "127.0.0.1", Type: corev1.NodeInternalIP}},
				},
			},
			gceInstanceID:  "12345",
			nodeInternalIP: "127.0.0.1",
			expectExists:   false,
		},
		{
			name: "node id changed",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
					Annotations: map[string]string{
						gceInstanceIDKey: "123456",
					},
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{{Address: "127.0.0.1", Type: corev1.NodeInternalIP}},
				},
			},
			gceInstanceID:  "12345",
			nodeInternalIP: "127.0.0.1",
			expectExists:   false,
		},
		{
			name: "node internal ip changed",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
					Annotations: map[string]string{
						gceInstanceIDKey: "12345",
					},
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{{Address: "127.0.0.2", Type: corev1.NodeInternalIP}},
				},
			},
			gceInstanceID:  "12345",
			nodeInternalIP: "127.0.0.1",
			expectExists:   false,
		},
		{
			name: "node still exists",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
					Annotations: map[string]string{
						gceInstanceIDKey: "12345",
					},
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{{Address: "127.0.0.1", Type: corev1.NodeInternalIP}},
				},
			},
			gceInstanceID:  "12345",
			nodeInternalIP: "127.0.0.1",
			expectExists:   true,
		},
	}
	for _, test := range cases {
		controller := NewControllerBuilder().Build()
		nodeExists, err := controller.verifyConfigMapEntry(test.node, test.gceInstanceID, test.nodeInternalIP)
		if gotExpected := gotExpectedError(test.name, test.expectErr, err); gotExpected != nil {
			t.Errorf("%v", gotExpected)
		}
		if nodeExists != test.expectExists {
			t.Errorf("test %q failed: got nodeExists %t, expected %t", test.name, nodeExists, test.expectExists)
		}
	}
}

func TestProcessConfigMapEntryOnNodeCreation(t *testing.T) {
	cases := []struct {
		name                  string
		key                   string
		filestoreIP           string
		node                  *corev1.Node
		cm                    *corev1.ConfigMap
		lockReleaseError      bool
		expectedError         bool
		expectedConfigMapSize int
	}{
		{
			name:        "should keep the entry",
			key:         "test-project.us-central1.test-filestore.test-share.123456.192_168_1_1",
			filestoreIP: "192.168.92.0",
			cm: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fscsi-node-name",
					Namespace: "gke-managed-filestorecsi",
				},
				Data: map[string]string{
					"test-project.us-central1.test-filestore.test-share.123456.192_168_1_1": "192.168.92.0",
				},
			},

			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-name",
					Annotations: map[string]string{
						gceInstanceIDKey: "123456",
					},
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{{Address: "192.168.1.1", Type: corev1.NodeInternalIP}},
				},
			},
			lockReleaseError:      false,
			expectedError:         false,
			expectedConfigMapSize: 1,
		},
		{
			name:        "should remove the entry due to node's absence in config map",
			key:         "test-project.us-central1.test-filestore.test-share.123456.192_168_1_1",
			filestoreIP: "192.168.92.0",
			cm: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fscsi-node-name",
					Namespace: "gke-managed-filestorecsi",
				},
				Data: map[string]string{
					"test-project.us-central1.test-filestore.test-share.123456.192_168_1_1": "192.168.92.0",
				},
			},

			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-name",
					Annotations: map[string]string{
						gceInstanceIDKey: "changed_key",
					},
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{{Address: "192.168.1.1", Type: corev1.NodeInternalIP}},
				},
			},
			lockReleaseError:      false,
			expectedError:         false,
			expectedConfigMapSize: 0,
		},
		{
			name:        "fail to remove the entry due to rpc call failure",
			key:         "test-project.us-central1.test-filestore.test-share.123456.192_168_1_1",
			filestoreIP: "192.168.92.0",
			cm: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fscsi-node-name",
					Namespace: "gke-managed-filestorecsi",
				},
				Data: map[string]string{
					"test-project.us-central1.test-filestore.test-share.123456.192_168_1_1": "192.168.92.0",
				},
			},

			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-name",
					Annotations: map[string]string{
						gceInstanceIDKey: "changed_key",
					},
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{{Address: "192.168.1.1", Type: corev1.NodeInternalIP}},
				},
			},
			lockReleaseError:      true,
			expectedError:         true,
			expectedConfigMapSize: 1,
		},
	}
	for _, test := range cases {
		client := fake.NewSimpleClientset(test.cm, test.node)
		eventProcessor := &DefaultEventProcessor{}
		lockService := &MockLockService{}
		if test.lockReleaseError {
			lockService.On("ReleaseLock").Return(fmt.Errorf("fake lock release rpc call error"))
		} else {
			lockService.On("ReleaseLock").Return(nil)
		}

		c := NewControllerBuilder().WithClient(client).WithProcessor(eventProcessor).WithLockService(lockService).Build()
		err := eventProcessor.processConfigMapEntryOnNodeCreation(context.Background(), test.key, test.filestoreIP, test.node, test.cm)
		fmt.Printf("test case: %s processConfigMapEntryOnNodeCreation result, %v", test.name, err)
		if err != nil && !test.expectedError {
			t.Errorf("got an unexpected error")
		}

		if err == nil && test.expectedError {
			t.Errorf("expected error but no error returned")
		}
		updatedCM, err := c.GetConfigMap(context.Background(), test.cm.Name, test.cm.Namespace)
		if err != nil {
			t.Error("error getting config map")
		}
		if got, want := len(updatedCM.Data), test.expectedConfigMapSize; got != want {
			t.Errorf("expected resulting config map size: %d, but got %d", want, got)
		}
	}
}

func TestProcessConfigMapEntryOnNodeUpdate(t *testing.T) {
	cases := []struct {
		name                  string
		key                   string
		filestoreIP           string
		newNode               *corev1.Node
		oldNode               *corev1.Node
		cm                    *corev1.ConfigMap
		lockReleaseError      bool
		expectedError         bool
		expectedConfigMapSize int
	}{
		{
			name:        "should keep the entry because new node matches config map entry",
			key:         "test-project.us-central1.test-filestore.test-share.123456.192_168_1_1",
			filestoreIP: "192.168.92.0",
			cm: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fscsi-node-name",
					Namespace: "gke-managed-filestorecsi",
				},
				Data: map[string]string{
					"test-project.us-central1.test-filestore.test-share.123456.192_168_1_1": "192.168.92.0",
				},
			},

			newNode: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-name",
					Annotations: map[string]string{
						gceInstanceIDKey: "123456",
					},
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{{Address: "192.168.1.1", Type: corev1.NodeInternalIP}},
				},
			},
			oldNode:               &corev1.Node{},
			lockReleaseError:      false,
			expectedError:         false,
			expectedConfigMapSize: 1,
		},
		{
			name:        "should remove the entry because old node matches config map entry but new node does not",
			key:         "test-project.us-central1.test-filestore.test-share.123456.192_168_1_1",
			filestoreIP: "192.168.92.0",
			cm: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fscsi-node-name",
					Namespace: "gke-managed-filestorecsi",
				},
				Data: map[string]string{
					"test-project.us-central1.test-filestore.test-share.123456.192_168_1_1": "192.168.92.0",
				},
			},

			newNode: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-name",
					Annotations: map[string]string{
						gceInstanceIDKey: "changed_key",
					},
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{{Address: "192.168.1.1", Type: corev1.NodeInternalIP}},
				},
			},
			oldNode: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-name",
					Annotations: map[string]string{
						gceInstanceIDKey: "123456",
					},
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{{Address: "192.168.1.1", Type: corev1.NodeInternalIP}},
				},
			},
			lockReleaseError:      false,
			expectedError:         false,
			expectedConfigMapSize: 0,
		},
		{
			name:        "fail to remove the entry due to rpc call failure",
			key:         "test-project.us-central1.test-filestore.test-share.123456.192_168_1_1",
			filestoreIP: "192.168.92.0",
			cm: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fscsi-node-name",
					Namespace: "gke-managed-filestorecsi",
				},
				Data: map[string]string{
					"test-project.us-central1.test-filestore.test-share.123456.192_168_1_1": "192.168.92.0",
				},
			},

			newNode: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-name",
					Annotations: map[string]string{
						gceInstanceIDKey: "changed_key",
					},
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{{Address: "192.168.1.1", Type: corev1.NodeInternalIP}},
				},
			},
			oldNode: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-name",
					Annotations: map[string]string{
						gceInstanceIDKey: "123456",
					},
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{{Address: "192.168.1.1", Type: corev1.NodeInternalIP}},
				},
			},
			lockReleaseError:      true,
			expectedError:         true,
			expectedConfigMapSize: 1,
		},
	}
	for _, test := range cases {
		client := fake.NewSimpleClientset(test.cm)
		eventProcessor := &DefaultEventProcessor{}
		lockService := &MockLockService{}
		if test.lockReleaseError {
			lockService.On("ReleaseLock").Return(fmt.Errorf("fake lock release rpc call error"))
		} else {
			lockService.On("ReleaseLock").Return(nil)
		}

		c := NewControllerBuilder().WithClient(client).WithProcessor(eventProcessor).WithLockService(lockService).Build()
		err := eventProcessor.processConfigMapEntryOnNodeUpdate(context.Background(), test.key, test.filestoreIP, test.newNode, test.oldNode, test.cm)
		fmt.Printf("test case: %s processConfigMapEntryOnNodeUpdate result, %v", test.name, err)
		if err != nil && !test.expectedError {
			t.Errorf("got an unexpected error")
		}

		if err == nil && test.expectedError {
			t.Errorf("expected error but no error returned")
		}
		updatedCM, err := c.GetConfigMap(context.Background(), test.cm.Name, test.cm.Namespace)
		if err != nil {
			t.Error("error getting config map")
		}
		if got, want := len(updatedCM.Data), test.expectedConfigMapSize; got != want {
			t.Errorf("expected resulting config map size: %d, but got %d", want, got)
		}
	}
}

func TestHandleCreateEvent(t *testing.T) {
	cases := []struct {
		name                string
		existingCM          *corev1.ConfigMap
		obj                 interface{}
		eventProcessorError bool
		expectedError       bool
	}{
		{
			name: "config map does not exist",
			existingCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fscsi-not-exist",
					Namespace: "gke-managed-filestorecsi",
				},
			},
			obj: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-name",
					Annotations: map[string]string{
						gceInstanceIDKey: "node1-id",
					},
				},
			},
			expectedError: false,
		},
		{
			name: "config map is found but config map processing returns error",
			existingCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fscsi-node-name",
					Namespace: "gke-managed-filestorecsi",
				},
				Data: map[string]string{
					"test-project.us-central1.test-filestore.test-share.123456.192_168_1_1":  "192.168.92.0",
					"test-project.us-central1.test-filestore1.test-share.123456.192_168_1_1": "192.168.92.1",
				},
			},
			obj: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-name",
					Annotations: map[string]string{
						gceInstanceIDKey: "node2-id",
					},
				},
			},
			eventProcessorError: true,
			expectedError:       true,
		},
		{
			name: "config map is found and all entries are processed successfully",
			existingCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fscsi-node-name",
					Namespace: "gke-managed-filestorecsi",
				},
				Data: map[string]string{
					"test-project.us-central1.test-filestore.test-share.123456.192_168_1_1": "192.168.92.0",
				},
			},
			obj: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-name",
					Annotations: map[string]string{
						gceInstanceIDKey: "node2-id",
					},
				},
			},
			eventProcessorError: false,
			expectedError:       false,
		},
	}
	for _, test := range cases {
		client := fake.NewSimpleClientset(test.existingCM)
		eventProcessor := &MockEventProcessor{}
		if test.eventProcessorError {
			eventProcessor.On("processConfigMapEntryOnNodeCreation", mock.Anything).Return(fmt.Errorf("mock processor error"))
		} else {
			eventProcessor.On("processConfigMapEntryOnNodeCreation", mock.Anything).Return(nil)
		}
		controller := NewControllerBuilder().WithClient(client).WithProcessor(eventProcessor).Build()
		err := controller.handleCreateEvent(context.Background(), test.obj)
		fmt.Printf("test case: %s handleCreateEvent result, %v", test.name, err)
		if err != nil && !test.expectedError {
			t.Errorf("got an unexpected error")
		}

		if err == nil && test.expectedError {
			t.Errorf("expected error but no error returned")
		}
	}
}

func TestHandleUpdateEvent(t *testing.T) {
	cases := []struct {
		name                string
		existingCM          *corev1.ConfigMap
		oldObj              interface{}
		newObj              interface{}
		eventProcessorError bool
		expectedError       bool
	}{
		{
			name: "config map does not exist",
			existingCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fscsi-not-exist",
					Namespace: "gke-managed-filestorecsi",
				},
			},
			newObj: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-name",
					Annotations: map[string]string{
						gceInstanceIDKey: "node1-id",
					},
				},
			},
			oldObj:        &corev1.Node{},
			expectedError: false,
		},
		{
			name: "config map is found but config map processing returns error",
			existingCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fscsi-node-name",
					Namespace: "gke-managed-filestorecsi",
				},
				Data: map[string]string{
					"test-project.us-central1.test-filestore.test-share.123456.192_168_1_1":  "192.168.92.0",
					"test-project.us-central1.test-filestore1.test-share.123456.192_168_1_1": "192.168.92.1",
				},
			},
			newObj: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-name",
					Annotations: map[string]string{
						gceInstanceIDKey: "node2-id",
					},
				},
			},
			oldObj: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-name",
					Annotations: map[string]string{
						gceInstanceIDKey: "node2-id",
					},
				},
			},
			eventProcessorError: true,
			expectedError:       true,
		},
		{
			name: "config map is found and all entries are processed successfully",
			existingCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fscsi-node-name",
					Namespace: "gke-managed-filestorecsi",
				},
				Data: map[string]string{
					"test-project.us-central1.test-filestore.test-share.123456.192_168_1_1": "192.168.92.0",
				},
			},
			newObj: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-name",
					Annotations: map[string]string{
						gceInstanceIDKey: "node2-id",
					},
				},
			},
			oldObj: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-name",
					Annotations: map[string]string{
						gceInstanceIDKey: "node2-id",
					},
				},
			},
			eventProcessorError: false,
			expectedError:       false,
		},
	}
	for _, test := range cases {
		client := fake.NewSimpleClientset(test.existingCM)
		eventProcessor := &MockEventProcessor{}
		if test.eventProcessorError {
			eventProcessor.On("processConfigMapEntryOnNodeUpdate", mock.Anything).Return(fmt.Errorf("mock processor error"))
		} else {
			eventProcessor.On("processConfigMapEntryOnNodeUpdate", mock.Anything).Return(nil)
		}
		controller := NewControllerBuilder().WithClient(client).WithProcessor(eventProcessor).Build()
		err := controller.handleUpdateEvent(context.Background(), test.oldObj, test.newObj)
		fmt.Printf("test case: %s handleUpdateEvent result, %v", test.name, err)
		if err != nil && !test.expectedError {
			t.Errorf("got an unexpected error")
		}

		if err == nil && test.expectedError {
			t.Errorf("expected error but no error returned")
		}
	}
}
