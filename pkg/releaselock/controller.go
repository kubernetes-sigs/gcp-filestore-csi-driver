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

package rpc

import (
	"context"
	"fmt"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/util"

	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

const (
	gceInstanceIDKey = "container.googleapis.com/instance_id"
	leaseName        = "filestore-csi-storage-gke-io-node"
	// Root CA configmap in each namespace.
	rootCA = "kube-root-ca.crt"
)

type LockReleaseController struct {
	client kubernetes.Interface

	// Identity of this controller, generated at creation time and not persisted
	// across restarts. Useful only for debugging, for seeing the source of events.
	id string
	// hostname is the GKE node name where the lock release controller is running on.
	hostname string

	config *LockReleaseControllerConfig
}

type LockReleaseControllerConfig struct {
	// Parameters of leaderelection.LeaderElectionConfig.
	LeaseDuration, RenewDeadline, RetryPeriod time.Duration
	// Reconcile loop frequency.
	SyncPeriod time.Duration
}

func NewLockReleaseController(client kubernetes.Interface, config *LockReleaseControllerConfig) (*LockReleaseController, error) {
	// Register rpc procedure for lock release.
	if err := RegisterLockReleaseProcedure(); err != nil {
		klog.Errorf("Error initializing lockrelease controller: %v", err)
		return nil, err
	}

	hostname, err := os.Hostname()
	if err != nil {
		klog.Errorf("Failed to get hostname for lockrelease controller: %v", err)
		return nil, err
	}
	// Add a uniquifier so that two processes on the same host don't accidentally both become active.
	id := hostname + "_" + string(uuid.NewUUID())

	return &LockReleaseController{
		id:       id,
		hostname: hostname,
		client:   client,
		config:   config,
	}, nil
}

func (c *LockReleaseController) Run(ctx context.Context) {
	run := func(ctx context.Context) {
		klog.Infof("Lock release controller %s started leading on node %s", c.id, c.hostname)
		wait.Forever(func() {
			cmList, err := c.client.CoreV1().ConfigMaps(util.ConfigMapNamespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				klog.Errorf("Failed to list configmap in namespace %s: %v", util.ConfigMapNamespace, err)
				return
			}
			klog.Infof("Listed %d configmaps in namespace %s", len(cmList.Items), util.ConfigMapNamespace)

			nodes, err := c.listNodes(ctx)
			if err != nil {
				klog.Errorf("Failed to list nodes: %v", err)
				return
			}
			klog.Infof("Listed %d nodes", len(nodes))

			for _, cm := range cmList.Items {
				// Filter out root ca.
				if cm.Name == rootCA {
					continue
				}
				if err := c.syncLockInfo(ctx, &cm, nodes); err != nil {
					klog.Errorf("Failed to sync lock info for configmap %s/%s: %v", cm.Namespace, cm.Name, err)
				}
			}
		}, c.config.SyncPeriod)
	}

	rl, err := resourcelock.New(
		resourcelock.LeasesResourceLock,
		util.ConfigMapNamespace,
		leaseName,
		nil,
		c.client.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity: c.id,
		})
	if err != nil {
		klog.Fatalf("Error creating resourcelock: %v", err)
	}

	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: c.config.LeaseDuration,
		RenewDeadline: c.config.RenewDeadline,
		RetryPeriod:   c.config.RetryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: run,
			OnStoppedLeading: func() {
				klog.Fatalf("%s no longer the leader", c.id)
			},
		},
	})
}

func (c *LockReleaseController) syncLockInfo(ctx context.Context, cm *corev1.ConfigMap, nodes map[string]*corev1.Node) error {
	nodeName, err := util.GKENodeNameFromConfigMap(cm)
	if err != nil {
		klog.Errorf("Failed to get GKE node name from configmap %s/%s: %v", cm.Namespace, cm.Name, err)
		return err
	}

	node := nodes[nodeName]
	data := cm.DeepCopy().Data
	for key, filestoreIP := range data {
		_, _, _, _, gceInstanceID, gkeNodeInternalIP, err := util.ParseConfigMapKey(key)
		if err != nil {
			klog.Errorf("Failed to parse configmap key %s: %v", key, err)
			continue
		}
		klog.V(6).Infof("Verifying GKE node %s with nodeId %s nodeInternalIP %s exists or not", nodeName, gceInstanceID, gkeNodeInternalIP)
		nodeExists, err := c.verifyNodeExists(node, gceInstanceID, gkeNodeInternalIP)
		if err != nil {
			klog.Errorf("Failed to verify GKE node %s with nodeId %s nodeInternalIP %s still exists: %v", nodeName, gceInstanceID, gkeNodeInternalIP, err)
			continue
		}
		if nodeExists {
			klog.V(6).Infof("GKE node %s with nodeId %s nodeInternalIP %s still exists in API server, skip lock info reconciliation", nodeName, gceInstanceID, gkeNodeInternalIP)
			continue
		}
		klog.Infof("GKE node %s with nodeId %s nodeInternalIP %s no longer exists, releasing lock for Filestore IP %s", nodeName, gceInstanceID, gkeNodeInternalIP, filestoreIP)
		if err := ReleaseLock(filestoreIP, gkeNodeInternalIP); err != nil {
			klog.Errorf("Failed to release lock: %v", err)
			continue
		}
		klog.Infof("Removing lock info key %s from configmap %s/%s with data %v", key, cm.Namespace, cm.Name, cm.Data)
		// Apply the "Get() and Update(), or retry" logic in RemoveKeyFromConfigMap().
		// This will increase the number of k8s api calls,
		// but reduce repetitive ReleaseLock() due to kubeclient api failures in each reconcile loop.
		if err := util.RemoveKeyFromConfigMapWithRetry(ctx, cm, key, c.client); err != nil {
			klog.Errorf("Failed to remove key %s from configmap %s/%s: %v", key, cm.Namespace, cm.Name, err)
		}
	}
	return nil
}

// verifyNodeExists validates if the given node object has the exact nodeID, and nodeInternalIP.
func (c *LockReleaseController) verifyNodeExists(node *corev1.Node, expectedGCEInstanceID, expectedNodeInternalIP string) (bool, error) {
	if node == nil {
		return false, nil
	}
	if node.Annotations == nil {
		return false, fmt.Errorf("node %s annotations is nil", node.Name)
	}
	instanceID, ok := node.Annotations[gceInstanceIDKey]
	if !ok {
		klog.Warningf("Node %s missing key %s in node.annotations", node.Name, gceInstanceIDKey)
		return false, nil
	}
	if instanceID != expectedGCEInstanceID {
		return false, nil
	}
	for _, address := range node.Status.Addresses {
		if address.Type == corev1.NodeInternalIP && address.Address == expectedNodeInternalIP {
			return true, nil
		}
	}
	return false, nil
}

func (c *LockReleaseController) listNodes(ctx context.Context) (map[string]*corev1.Node, error) {
	nodeList, err := c.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	nodeMap := map[string]*corev1.Node{}
	for _, node := range nodeList.Items {
		nodeMap[node.Name] = &node
	}
	return nodeMap, nil
}
