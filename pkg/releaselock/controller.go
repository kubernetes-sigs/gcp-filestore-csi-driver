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
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/metrics"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/util"

	cache "k8s.io/client-go/tools/cache"
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

	config         *LockReleaseControllerConfig
	metricsManager *metrics.MetricsManager
	nodeInformer   *cache.SharedIndexInformer
}

type LockReleaseControllerConfig struct {
	// Parameters of leaderelection.LeaderElectionConfig.
	LeaseDuration, RenewDeadline, RetryPeriod time.Duration
	// Reconcile loop frequency.
	SyncPeriod time.Duration
	// HTTP endpoint and path to emit NFS lock release metrics.
	MetricEndpoint, MetricPath string
}

func NewLockReleaseController(
	client kubernetes.Interface,
	config *LockReleaseControllerConfig,
	nodeInformer *cache.SharedIndexInformer) (*LockReleaseController, error) {
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

	lc := &LockReleaseController{
		id:           id,
		hostname:     hostname,
		client:       client,
		config:       config,
		nodeInformer: nodeInformer,
	}

	if config.MetricEndpoint != "" {
		mm := metrics.NewMetricsManager()
		mm.InitializeHttpHandler(config.MetricEndpoint, config.MetricPath)
		mm.RegisterKubeAPIDurationMetric()
		mm.RegisterLockReleaseCountnMetric()
		lc.metricsManager = mm
	}

	return lc, nil
}

func (c *LockReleaseController) HandleCreateEvent(ctx context.Context, node *corev1.Node) error {
	start := time.Now()
	cmName := ConfigMapNamePrefix + node.Name
	cm, err := c.client.CoreV1().ConfigMaps(util.ManagedFilestoreCSINamespace).Get(ctx, cmName, metav1.GetOptions{})
	duration := time.Since(start)
	c.RecordKubeAPIMetrics(err, metrics.ConfigMapResourceType, metrics.GetOpType, metrics.ReconcilerOpSource, duration)

	if err != nil {
		klog.Errorf("Failed to get configmap in namespace %s: %v", util.ManagedFilestoreCSINamespace, err)
		return err
	}
	klog.Infof("Got configmap (%v) in namespace %s", cm, util.ManagedFilestoreCSINamespace)
	data := cm.DeepCopy().Data
	latestNode, err := c.client.CoreV1().Nodes().Get(ctx, node.Name, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Failed to get node in namespace %v", err)
		return err
	}

	for key, filestoreIP := range data {
		_, _, _, _, gceInstanceID, gkeNodeInternalIP, err := ParseConfigMapKey(key)
		if err != nil {
			klog.Errorf("Failed to parse configmap key %s: %v", key, err)
			continue
		}
		klog.V(6).Infof("Verifying GKE node %s with nodeId %s nodeInternalIP %s exists or not", node.Name, gceInstanceID, gkeNodeInternalIP)
		entryMatchesNode, err := c.verifyConfigMapEntry(node, gceInstanceID, gkeNodeInternalIP)
		if err != nil {
			klog.Errorf("Failed to verify GKE node %s with nodeId %s nodeInternalIP %s still exists: %v", node.Name, gceInstanceID, gkeNodeInternalIP, err)
			continue
		}
		if entryMatchesNode {
			klog.V(6).Infof("GKE node %s with nodeId %s nodeInternalIP %s still exists in API server, skip lock info reconciliation", node.Name, gceInstanceID, gkeNodeInternalIP)
			continue
		}

		// Try to match the latest node, to prevent incorrect releasing the lock in case of a lagging informer/watch
		entryMatchesLatestNode, err := c.verifyConfigMapEntry(latestNode, gceInstanceID, gkeNodeInternalIP)
		if err != nil {
			klog.Errorf("Failed to verify GKE node %s with nodeId %s nodeInternalIP %s still exists: %v", node.Name, gceInstanceID, gkeNodeInternalIP, err)
			continue
		}
		if entryMatchesLatestNode {
			klog.V(6).Infof("GKE node %s with nodeId %s nodeInternalIP %s exists in API server, skip lock info reconciliation", node.Name, gceInstanceID, gkeNodeInternalIP)
			continue
		}

		klog.Infof("GKE node %s with nodeId %s nodeInternalIP %s no longer exists, releasing lock for Filestore IP %s", node.Name, gceInstanceID, gkeNodeInternalIP, filestoreIP)
		opErr := ReleaseLock(filestoreIP, gkeNodeInternalIP)
		c.RecordLockReleaseMetrics(opErr)
		if opErr != nil {
			klog.Errorf("Failed to release lock: %v", opErr)
			continue
		}
		klog.Infof("Removing lock info key %s from configmap %s/%s with data %v", key, cm.Namespace, cm.Name, cm.Data)
		// Apply the "Get() and Update(), or retry" logic in RemoveKeyFromConfigMap().
		// This will increase the number of k8s api calls,
		// but reduce repetitive ReleaseLock() due to kubeclient api failures in each reconcile loop.
		if err := c.RemoveKeyFromConfigMapWithRetry(ctx, cm, key); err != nil {
			klog.Errorf("Failed to remove key %s from configmap %s/%s: %v", key, cm.Namespace, cm.Name, err)
		}
	}
	return nil
}

func (c *LockReleaseController) HandleUpdateEvent(ctx context.Context, oldNode *corev1.Node, newNode *corev1.Node) error {
	start := time.Now()
	nodeName := newNode.Name
	cmName := ConfigMapNamePrefix + nodeName
	cm, err := c.client.CoreV1().ConfigMaps(util.ManagedFilestoreCSINamespace).Get(ctx, cmName, metav1.GetOptions{})
	duration := time.Since(start)
	c.RecordKubeAPIMetrics(err, metrics.ConfigMapResourceType, metrics.GetOpType, metrics.ReconcilerOpSource, duration)

	if err != nil {
		klog.Errorf("Failed to get configmap in namespace %s: %v", util.ManagedFilestoreCSINamespace, err)
		return err
	}
	klog.Infof("Got configmap (%v) in namespace %s", cm, util.ManagedFilestoreCSINamespace)

	data := cm.DeepCopy().Data
	for key, filestoreIP := range data {
		_, _, _, _, gceInstanceID, gkeNodeInternalIP, err := ParseConfigMapKey(key)
		if err != nil {
			klog.Errorf("Failed to parse configmap key %s: %v", key, err)
			continue
		}
		klog.V(6).Infof("Verifying GKE node %s with nodeId %s nodeInternalIP %s exists or not", nodeName, gceInstanceID, gkeNodeInternalIP)
		entryMatchesNewNode, err := c.verifyConfigMapEntry(newNode, gceInstanceID, gkeNodeInternalIP)
		if err != nil {
			klog.Errorf("Failed to verify GKE node %s with nodeId %s nodeInternalIP %s still exists: %v", nodeName, gceInstanceID, gkeNodeInternalIP, err)
			continue
		}
		entryMatchesOldNode, err := c.verifyConfigMapEntry(oldNode, gceInstanceID, gkeNodeInternalIP)
		if err != nil {
			klog.Errorf("Failed to verify GKE node %s with nodeId %s nodeInternalIP %s still exists: %v", nodeName, gceInstanceID, gkeNodeInternalIP, err)
			continue
		}
		klog.Infof("Checked config map entry against old node(matching result %t), and new node(matching result %t)", entryMatchesOldNode, entryMatchesNewNode)
		if entryMatchesNewNode {
			klog.V(6).Infof("GKE node %s with nodeId %s nodeInternalIP %s still exists in API server, skip lock info reconciliation", nodeName, gceInstanceID, gkeNodeInternalIP)
			continue
		} else if entryMatchesOldNode {
			klog.Infof("GKE node %s with nodeId %s nodeInternalIP %s matches a node before update, releasing lock for Filestore IP %s", nodeName, gceInstanceID, gkeNodeInternalIP, filestoreIP)
			opErr := ReleaseLock(filestoreIP, gkeNodeInternalIP)
			c.RecordLockReleaseMetrics(opErr)
			if opErr != nil {
				klog.Errorf("Failed to release lock: %v", opErr)
				continue
			}
			klog.Infof("Removing lock info key %s from configmap %s/%s with data %v", key, cm.Namespace, cm.Name, cm.Data)

			if err := c.RemoveKeyFromConfigMapWithRetry(ctx, cm, key); err != nil {
				klog.Errorf("Failed to remove key %s from configmap %s/%s: %v", key, cm.Namespace, cm.Name, err)
			}
		}

	}
	return nil
}

// verifyConfigMapEntry validates if the given config map entry object has the exact nodeID, and nodeInternalIP.
func (c *LockReleaseController) verifyConfigMapEntry(node *corev1.Node, expectedGCEInstanceID, expectedNodeInternalIP string) (bool, error) {
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

func (c *LockReleaseController) ListNodes(ctx context.Context) (map[string]*corev1.Node, error) {
	nodeList, err := c.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	nodeMap := map[string]*corev1.Node{}
	for _, node := range nodeList.Items {
		nodeMap[node.Name] = node.DeepCopy()
	}
	return nodeMap, nil
}

func (c *LockReleaseController) RecordKubeAPIMetrics(opErr error, resourceType, opType, opSource string, opDuration time.Duration) {
	if c.metricsManager == nil {
		return
	}
	c.metricsManager.RecordKubeAPIMetrics(opErr, resourceType, opType, opSource, opDuration)
}

func (c *LockReleaseController) RecordLockReleaseMetrics(opErr error) {
	if c.metricsManager == nil {
		return
	}
	c.metricsManager.RecordLockReleaseMetrics(opErr)
}

// GetId returns the ID of the LockReleaseController.
func (c *LockReleaseController) GetId() string {
	return c.id
}

// GetHost returns the hostname where the lock release controller is running on.
func (c *LockReleaseController) GetHost() string {
	return c.hostname
}

// GetClient returns the kubernetes client of the LockReleaseController.
func (c *LockReleaseController) GetClient() kubernetes.Interface {
	return c.client
}
