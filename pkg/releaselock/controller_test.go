package rpc

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestVerifyNodeExists(t *testing.T) {
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
		controller := LockReleaseController{}
		nodeExists, err := controller.verifyNodeExists(test.node, test.gceInstanceID, test.nodeInternalIP)
		if gotExpected := gotExpectedError(test.name, test.expectErr, err); gotExpected != nil {
			t.Errorf("%v", gotExpected)
		}
		if nodeExists != test.expectExists {
			t.Errorf("test %q failed: got nodeExists %t, expected %t", test.name, nodeExists, test.expectExists)
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
