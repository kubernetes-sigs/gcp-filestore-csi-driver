package lockrelease

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
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

func TestListNodes(t *testing.T) {
	node1 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1",
			Annotations: map[string]string{
				gceInstanceIDKey: "node1-id",
			},
		},
	}
	node2 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node2",
			Annotations: map[string]string{
				gceInstanceIDKey: "node2-id",
			},
		},
	}
	controller := LockReleaseController{client: fake.NewSimpleClientset(node1, node2)}
	expectedMap := map[string]*corev1.Node{
		"node1": {
			ObjectMeta: metav1.ObjectMeta{
				Name: "node1",
				Annotations: map[string]string{
					gceInstanceIDKey: "node1-id",
				},
			},
		},
		"node2": {
			ObjectMeta: metav1.ObjectMeta{
				Name: "node2",
				Annotations: map[string]string{
					gceInstanceIDKey: "node2-id",
				},
			},
		},
	}
	nodes, err := controller.listNodes(context.Background())
	if err != nil {
		t.Fatalf("test listNodes failed: unexpected error: %v", err)
	}
	if diff := cmp.Diff(expectedMap, nodes); diff != "" {
		t.Errorf("test listNodes failed: unexpected diff (-want +got):%s", diff)
	}
}
