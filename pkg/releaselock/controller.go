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
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiError "k8s.io/apimachinery/pkg/api/errors"
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

	leaseName = "filestore-csi-storage-gke-io-node"

	DefaultLeaderElection = true
	DefaultLeaseDuration  = 15 * time.Second
	DefaultRenewDeadline  = 10 * time.Second
	DefaultRetryPeriod    = 2 * time.Second
	// DefaultSyncPeriod     = 5 * time.Minute
	DefaultSyncPeriod = 1 * time.Minute
)

type LockReleaseController struct {
	client kubernetes.Interface

	// Identity of this controller, generated at creation time and not persisted
	// across restarts. Useful only for debugging, for seeing the source of events.
	id string

	// Whether to do kubernetes leader election at all. It should basically
	// always be done when possible to avoid duplicate Provision attempts.
	leaderElection          bool
	leaderElectionNamespace string
	// Parameters of leaderelection.LeaderElectionConfig.
	leaseDuration, renewDeadline, retryPeriod time.Duration

	// Reconcile loop frequency.
	syncPeriod time.Duration
}

func NewLockReleaseController(client kubernetes.Interface) (*LockReleaseController, error) {
	// Register rpc procedure for lock release.
	if err := RegisterLockReleaseProcedure(); err != nil {
		klog.Errorf("Error initializing lock release controller: %v", err)
		return nil, err
	}

	id, err := os.Hostname()
	if err != nil {
		klog.Errorf("Error getting hostname: %v", err)
		return nil, err
	}
	// Add a uniquifier so that two processes on the same host don't accidentally both become active.
	id = id + "_" + string(uuid.NewUUID())

	return &LockReleaseController{
		id:                      id,
		client:                  client,
		leaderElection:          DefaultLeaderElection,
		leaderElectionNamespace: util.ConfigMapNamespace,
		leaseDuration:           DefaultLeaseDuration,
		renewDeadline:           DefaultRenewDeadline,
		retryPeriod:             DefaultRetryPeriod,
		syncPeriod:              DefaultSyncPeriod,
	}, nil
}

func (c *LockReleaseController) Run(ctx context.Context) {
	run := func(ctx context.Context) {
		klog.Infof("Starting lock release controller %s", c.id)
		wait.Forever(func() {
			cmList, err := c.client.CoreV1().ConfigMaps(util.ConfigMapNamespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				klog.Errorf("Failed to list configmap in namespace %s: %v", util.ConfigMapNamespace, err)
			}
			klog.Infof("Listed %d configmaps in namespace %s", len(cmList.Items), util.ConfigMapNamespace)
			for _, cm := range cmList.Items {
				// Filter out root ca.
				if cm.Name == "kube-root-ca.crt" {
					continue
				}
				if err := c.syncLockInfo(ctx, &cm); err != nil {
					klog.Errorf("Failed to sync lock info for configmap %s/%s: %v", cm.Namespace, cm.Name, err)
					continue
				}
			}
		}, DefaultSyncPeriod)
	}

	rl, err := resourcelock.New("leases",
		c.leaderElectionNamespace,
		leaseName,
		nil,
		c.client.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity: c.id,
		})
	if err != nil {
		klog.Fatalf("Error creating lock: %v", err)
	}

	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: c.leaseDuration,
		RenewDeadline: c.renewDeadline,
		RetryPeriod:   c.retryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: run,
			OnStoppedLeading: func() {
				klog.Fatalf("%s no longer the leader, staying inactive.", c.id)
			},
		},
	})
}

func (c *LockReleaseController) syncLockInfo(ctx context.Context, cm *corev1.ConfigMap) error {
	nodeName, err := util.GKENodeNameFromConfigMap(cm)
	if err != nil {
		return err
	}

	klog.Infof("Getting GKE Node %s from API Server", nodeName)
	node, err := c.client.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil && !apiError.IsNotFound(err) {
		return err
	}

	remainingData := cm.DeepCopy().Data
	for key, filestoreIP := range cm.Data {
		_, _, _, _, gkeNodeID, gkeNodeInternalIP, err := util.ParseConfigMapKey(key)
		if err != nil {
			klog.Errorf("Failed to parse configmap key %s: %v", key, err)
			continue
		}
		klog.Infof("Verifying GKE node %s with nodeId %s nodeInternalIP %s exists or not", nodeName, gkeNodeID, gkeNodeInternalIP)
		if c.verifyNodeExists(node, gkeNodeID, gkeNodeInternalIP) {
			continue
		}
		klog.Infof("GKE node %s with nodeId %s nodeInternalIP %s no longer exists, releasing lock for GKE node IP %s Filestore IP %s", nodeName, gkeNodeID, gkeNodeInternalIP, gkeNodeInternalIP, filestoreIP)
		if err := ReleaseLock(filestoreIP, gkeNodeInternalIP); err != nil {
			klog.Errorf("Failed to release lock: %v", err)
			continue
		}
		klog.Infof("Removing key %s from configmap %s/%s", key, cm.Namespace, cm.Name)
		delete(remainingData, key)
	}
	if len(remainingData) == 0 {
		klog.Infof("Deleting configmap %s/%s since remaining configmap.data is empty", cm.Namespace, cm.Name)
		return util.DeleteConfigMap(ctx, cm, c.client)
	}
	klog.Infof("Updating configmap %s/%s with data %v", cm.Namespace, cm.Name, remainingData)
	if _, err := util.UpdateConfigMapWithData(ctx, cm, remainingData, c.client); err != nil {
		return err
	}
	return nil
}

// verifyNodeExists validates if the given node object has the exact nodeID, and nodeInternalIP.
func (c *LockReleaseController) verifyNodeExists(node *corev1.Node, expectedNodeID, expectedNodeInternalIP string) bool {
	if node == nil {
		return false
	}
	if node.Annotations == nil {
		klog.Warningf("Node %s is unhealthy: node.annotations is nil", node.Name)
		return false
	}
	instanceID, ok := node.Annotations[gceInstanceIDKey]
	if !ok {
		klog.Warningf("Node %s is unhealthy: missing key %s in node.annotations", node.Name, gceInstanceIDKey)
		return false
	}
	if instanceID != expectedNodeID {
		return false
	}
	for _, address := range node.Status.Addresses {
		if address.Type == corev1.NodeInternalIP && address.Address == expectedNodeInternalIP {
			return true
		}
	}
	return false
}
