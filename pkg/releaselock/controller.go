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
	"errors"
	"fmt"
	"os"
	"time"

	"golang.org/x/time/rate"

	corev1 "k8s.io/api/core/v1"
	apiError "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/metrics"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/util"

	cache "k8s.io/client-go/tools/cache"
)

const (
	gceInstanceIDKey = "container.googleapis.com/instance_id"
	LeaseName        = "filestore-csi-storage-gke-io-node"
	// Root CA configmap in each namespace.
	rootCA = "kube-root-ca.crt"
)

type NodeUpdatePair struct {
	OldObj *corev1.Node
	NewObj *corev1.Node
}

type LockService interface {
	ReleaseLock(hostIP, clientIP string) error
}

type EventProcessor interface {
	processConfigMapEntryOnNodeCreation(ctx context.Context, key string, filestoreIP string, node *corev1.Node, cm *corev1.ConfigMap) error
	processConfigMapEntryOnNodeUpdate(ctx context.Context, key string, filestoreIP string, newNode *corev1.Node, oldNode *corev1.Node, cm *corev1.ConfigMap) error
	SetController(ctrl *LockReleaseController)
}

type DefaultEventProcessor struct {
	ctrl *LockReleaseController
}

func (p *DefaultEventProcessor) SetController(ctrl *LockReleaseController) {
	p.ctrl = ctrl
}

func (p *DefaultEventProcessor) processConfigMapEntryOnNodeCreation(ctx context.Context, key string, filestoreIP string, node *corev1.Node, cm *corev1.ConfigMap) error {
	if p.ctrl == nil {
		return fmt.Errorf("controller not set")
	}

	c := p.ctrl
	_, _, _, _, gceInstanceID, gkeNodeInternalIP, err := ParseConfigMapKey(key)
	if err != nil {
		return fmt.Errorf("failed to parse configmap key %s: %v", key, err)
	}
	klog.V(6).Infof("Verifying GKE node %s with nodeId %s nodeInternalIP %s exists or not", node.Name, gceInstanceID, gkeNodeInternalIP)
	entryMatchesNode, err := c.verifyConfigMapEntry(node, gceInstanceID, gkeNodeInternalIP)
	if err != nil {
		return fmt.Errorf("failed to verify GKE node %s with nodeId %s nodeInternalIP %s still exists: %v", node.Name, gceInstanceID, gkeNodeInternalIP, err)
	}
	if entryMatchesNode {
		klog.V(6).Infof("GKE node %s with nodeId %s nodeInternalIP %s still exists in API server, skip lock info reconciliation", node.Name, gceInstanceID, gkeNodeInternalIP)
		return nil
	}

	// Try to match the latest node, to prevent incorrect releasing the lock in case of a lagging informer/watch
	latestNode, err := c.client.CoreV1().Nodes().Get(ctx, node.Name, metav1.GetOptions{})
	if err != nil {
		if apiError.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to get node in namespace %v", err)
	}
	entryMatchesLatestNode, err := c.verifyConfigMapEntry(latestNode, gceInstanceID, gkeNodeInternalIP)
	if err != nil {
		return fmt.Errorf("failed to verify GKE node %s with nodeId %s nodeInternalIP %s still exists: %v", node.Name, gceInstanceID, gkeNodeInternalIP, err)
	}
	if entryMatchesLatestNode {
		klog.V(6).Infof("GKE node %s with nodeId %s nodeInternalIP %s exists in API server, skip lock info reconciliation", node.Name, gceInstanceID, gkeNodeInternalIP)
		return nil
	}

	klog.Infof("GKE node %s with nodeId %s nodeInternalIP %s no longer exists, releasing lock for Filestore IP %s", node.Name, gceInstanceID, gkeNodeInternalIP, filestoreIP)
	opErr := c.lockService.ReleaseLock(filestoreIP, gkeNodeInternalIP)
	c.RecordLockReleaseMetrics(opErr)
	if opErr != nil {
		return fmt.Errorf("failed to release lock: %v", opErr)
	}
	klog.Infof("Removing lock info key %s from configmap %s/%s with data %v", key, cm.Namespace, cm.Name, cm.Data)
	// Apply the "Get() and Update(), or retry" logic in RemoveKeyFromConfigMap().
	// This will increase the number of k8s api calls,
	// but reduce repetitive ReleaseLock() due to kubeclient api failures in each reconcile loop.
	if err := c.RemoveKeyFromConfigMapWithRetry(ctx, cm, key); err != nil {
		return fmt.Errorf("failed to remove key %s from configmap %s/%s: %v", key, cm.Namespace, cm.Name, err)
	}
	return nil
}

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

	updateEventQueue workqueue.RateLimitingInterface
	createEventQueue workqueue.RateLimitingInterface

	eventProcessor EventProcessor
	lockService    LockService
}

type LockReleaseControllerConfig struct {
	// Parameters of leaderelection.LeaderElectionConfig.
	LeaseDuration, RenewDeadline, RetryPeriod time.Duration
	// Parameters of workQueue rate limiters.
	WorkQueueRateLimiterBaseDelay, WorkQueueRateLimiterMaxDelay time.Duration
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
	createRatelimiter := workqueue.NewMaxOfRateLimiter(
		workqueue.NewItemExponentialFailureRateLimiter(config.WorkQueueRateLimiterBaseDelay, config.WorkQueueRateLimiterMaxDelay),
		&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(50), 300)},
	)
	updateRateLimiter := workqueue.NewMaxOfRateLimiter(
		workqueue.NewItemExponentialFailureRateLimiter(config.WorkQueueRateLimiterBaseDelay, config.WorkQueueRateLimiterMaxDelay),
		&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(20), 100)},
	)

	eventProcessor := &DefaultEventProcessor{}
	lockService := &FileStoreRPCClient{}

	lc := &LockReleaseController{
		id:               id,
		hostname:         hostname,
		client:           client,
		config:           config,
		nodeInformer:     nodeInformer,
		updateEventQueue: workqueue.NewRateLimitingQueue(updateRateLimiter),
		createEventQueue: workqueue.NewRateLimitingQueue(createRatelimiter),
		eventProcessor:   eventProcessor,
		lockService:      lockService,
	}

	if config.MetricEndpoint != "" {
		mm := metrics.NewMetricsManager()
		mm.InitializeHttpHandler(config.MetricEndpoint, config.MetricPath)
		mm.RegisterKubeAPIDurationMetric()
		mm.RegisterLockReleaseCountnMetric()
		lc.metricsManager = mm
	}

	eventProcessor.SetController(lc)
	return lc, nil
}

// TODO(b/377771989): Deperacte Run once lock release controller V2 is rolled out.
func (c *LockReleaseController) Run(ctx context.Context) {
	run := func(ctx context.Context) {
		klog.Infof("Lock release controller %s started leading on node %s", c.id, c.hostname)
		wait.Forever(func() {
			start := time.Now()
			cmList, err := c.client.CoreV1().ConfigMaps(util.ManagedFilestoreCSINamespace).List(ctx, metav1.ListOptions{})
			duration := time.Since(start)
			c.RecordKubeAPIMetrics(err, metrics.ConfigMapResourceType, metrics.ListOpType, metrics.ReconcilerOpSource, duration)
			if err != nil {
				klog.Errorf("Failed to list configmap in namespace %s: %v", util.ManagedFilestoreCSINamespace, err)
				return
			}
			klog.Infof("Listed %d configmaps in namespace %s", len(cmList.Items), util.ManagedFilestoreCSINamespace)

			start = time.Now()
			nodes, err := c.listNodes(ctx)
			duration = time.Since(start)
			c.RecordKubeAPIMetrics(err, metrics.NodeResourceType, metrics.ListOpType, metrics.ReconcilerOpSource, duration)
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
		util.ManagedFilestoreCSINamespace,
		LeaseName,
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

// TODO(b/377771989): Deperacte listNodes once lock release controller V2 is rolled out.
func (c *LockReleaseController) syncLockInfo(ctx context.Context, cm *corev1.ConfigMap, nodes map[string]*corev1.Node) error {
	nodeName, err := GKENodeNameFromConfigMap(cm)
	if err != nil {
		klog.Errorf("Failed to get GKE node name from configmap %s/%s: %v", cm.Namespace, cm.Name, err)
		return err
	}

	node := nodes[nodeName]
	data := cm.DeepCopy().Data
	for key, filestoreIP := range data {
		_, _, _, _, gceInstanceID, gkeNodeInternalIP, err := ParseConfigMapKey(key)
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
		opErr := c.lockService.ReleaseLock(filestoreIP, gkeNodeInternalIP)
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

// verifyNodeExists validates if the given node object has the exact nodeID, and nodeInternalIP.
// TODO(b/377771989): Deperacte verifyNodeExists once lock release controller V2 is rolled out.
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

// TODO(b/377771989): Deperacte listNodes once lock release controller V2 is rolled out.
func (c *LockReleaseController) listNodes(ctx context.Context) (map[string]*corev1.Node, error) {
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

func (c *LockReleaseController) RunEventWorkers(ctx context.Context) error {
	defer utilruntime.HandleCrash()
	defer c.updateEventQueue.ShutDown()
	defer c.createEventQueue.ShutDown()
	if !cache.WaitForCacheSync(ctx.Done(), (*c.nodeInformer).HasSynced) {
		klog.Fatal("Timed out waiting for caches to sync")
	}
	klog.Info("Cache sync completed successfully.")
	go wait.UntilWithContext(ctx, c.runCreateEventWorker, time.Second)
	go wait.UntilWithContext(ctx, c.runUpdateEventWorker, time.Second)
	klog.Info("Started workers")
	<-ctx.Done()
	klog.Info("Shutting down workers")
	return nil
}

func (c *LockReleaseController) runCreateEventWorker(ctx context.Context) {
	for c.processNextCreateEvent(ctx) {
	}
}

// TODO(b/374327452): interface rpc calls for mocking and create unit tests for handleCreateEvent and handleUpdateEvent
func (c *LockReleaseController) handleCreateEvent(ctx context.Context, obj interface{}) error {
	node := obj.(*corev1.Node)
	start := time.Now()
	cmName := ConfigMapNamePrefix + node.Name
	cm, err := c.client.CoreV1().ConfigMaps(util.ManagedFilestoreCSINamespace).Get(ctx, cmName, metav1.GetOptions{})
	duration := time.Since(start)
	c.RecordKubeAPIMetrics(err, metrics.ConfigMapResourceType, metrics.GetOpType, metrics.ReconcilerOpSource, duration)

	if err != nil {
		if apiError.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to get configmap in namespace %s: %w", util.ManagedFilestoreCSINamespace, err)
	}
	klog.Infof("Got configmap (%v) in namespace %s", cm, util.ManagedFilestoreCSINamespace)
	data := cm.DeepCopy().Data

	var configMapReconcileErrors []error
	for key, filestoreIP := range data {
		eventProcessor := c.eventProcessor
		err = eventProcessor.processConfigMapEntryOnNodeCreation(ctx, key, filestoreIP, node, cm)
		if err != nil {
			configMapReconcileErrors = append(configMapReconcileErrors, err)
		}
	}
	klog.Infof("skipped processing %d entries in config map", len(configMapReconcileErrors))
	if len(configMapReconcileErrors) > 0 {
		return errors.Join(configMapReconcileErrors...)
	}
	return nil

}

func (c *LockReleaseController) processNextCreateEvent(ctx context.Context) bool {
	obj, shutdown := c.createEventQueue.Get()
	if shutdown {
		return false
	}
	defer c.createEventQueue.Done(obj)

	err := c.handleCreateEvent(ctx, obj)
	if err == nil {
		// If no error occurs then we Forget this item so it does not
		// get queued again until another change happens.
		c.createEventQueue.Forget(obj)
		klog.V(8).Infof("Successfully processed node create event object %v", obj)
		return true
	}

	klog.Errorf("Requeue node create event due to error: %v", err)
	c.createEventQueue.AddRateLimited(obj)
	return true

}

func (c *LockReleaseController) runUpdateEventWorker(ctx context.Context) {
	for c.processNextUpdateEventWorkItem(ctx) {
	}
}

func (c *LockReleaseController) processNextUpdateEventWorkItem(ctx context.Context) bool {
	obj, shutdown := c.updateEventQueue.Get()
	if shutdown {
		return false
	}
	defer c.updateEventQueue.Done(obj)
	nodeUpdatePair, ok := obj.(*NodeUpdatePair)
	if !ok {
		klog.Error("unable to convert update event object to nodeUpdatePair")
		return true
	}

	// Access old and new objects:
	oldObj := nodeUpdatePair.OldObj
	newObj := nodeUpdatePair.NewObj

	err := c.handleUpdateEvent(ctx, oldObj, newObj)
	if err == nil {
		// If no error occurs then we Forget this item so it does not
		// get queued again until another change happens.
		c.updateEventQueue.Forget(obj)
		klog.V(8).Infof("Successfully processed node update event object %v", obj)
		return true
	}

	klog.Errorf("Requeue node update event due to error: %v", err)
	c.updateEventQueue.AddRateLimited(obj)
	return true
}

func (c *LockReleaseController) handleUpdateEvent(ctx context.Context, oldObj interface{}, newObj interface{}) error {
	newNode := newObj.(*corev1.Node)
	oldNode := oldObj.(*corev1.Node)
	start := time.Now()
	nodeName := newNode.Name
	cmName := ConfigMapNamePrefix + nodeName
	cm, err := c.client.CoreV1().ConfigMaps(util.ManagedFilestoreCSINamespace).Get(ctx, cmName, metav1.GetOptions{})
	duration := time.Since(start)
	c.RecordKubeAPIMetrics(err, metrics.ConfigMapResourceType, metrics.GetOpType, metrics.ReconcilerOpSource, duration)

	if err != nil {
		if apiError.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to get configmap in namespace %s: %w", util.ManagedFilestoreCSINamespace, err)
	}
	klog.Infof("Got configmap (%v) in namespace %s", cm, util.ManagedFilestoreCSINamespace)

	data := cm.DeepCopy().Data
	var configMapReconcileErrors []error
	for key, filestoreIP := range data {
		err = c.eventProcessor.processConfigMapEntryOnNodeUpdate(ctx, key, filestoreIP, newNode, oldNode, cm)
		if err != nil {
			configMapReconcileErrors = append(configMapReconcileErrors, err)
		}
	}
	if len(configMapReconcileErrors) > 0 {
		return errors.Join(configMapReconcileErrors...)
	}
	return nil
}

func (p *DefaultEventProcessor) processConfigMapEntryOnNodeUpdate(ctx context.Context, key string, filestoreIP string, newNode *corev1.Node, oldNode *corev1.Node, cm *corev1.ConfigMap) error {
	if p.ctrl == nil {
		return fmt.Errorf("controller not set")
	}
	c := p.ctrl
	_, _, _, _, gceInstanceID, gkeNodeInternalIP, err := ParseConfigMapKey(key)
	if err != nil {
		return fmt.Errorf("failed to parse configmap key %s: %w", key, err)
	}
	klog.V(6).Infof("Verifying GKE node %s with nodeId %s nodeInternalIP %s exists or not", newNode.Name, gceInstanceID, gkeNodeInternalIP)
	entryMatchesNewNode, err := c.verifyConfigMapEntry(newNode, gceInstanceID, gkeNodeInternalIP)
	if err != nil {
		return fmt.Errorf("failed to verify GKE node %s with nodeId %s nodeInternalIP %s still exists: %w", newNode.Name, gceInstanceID, gkeNodeInternalIP, err)
	}
	if entryMatchesNewNode {
		klog.V(6).Infof("GKE node %s with nodeId %s nodeInternalIP %s still exists in API server, skip lock info reconciliation", newNode.Name, gceInstanceID, gkeNodeInternalIP)
		return nil
	}

	entryMatchesOldNode, err := c.verifyConfigMapEntry(oldNode, gceInstanceID, gkeNodeInternalIP)
	if err != nil {
		return fmt.Errorf("failed to verify GKE node %s with nodeId %s nodeInternalIP %s still exists: %v", newNode.Name, gceInstanceID, gkeNodeInternalIP, err)
	}
	klog.Infof("Checked config map entry against old node(matching result %t), and new node(matching result %t)", entryMatchesOldNode, entryMatchesNewNode)

	if entryMatchesOldNode {
		klog.Infof("GKE node %s with nodeId %s nodeInternalIP %s matches a node before update, releasing lock for Filestore IP %s", newNode.Name, gceInstanceID, gkeNodeInternalIP, filestoreIP)
		opErr := c.lockService.ReleaseLock(filestoreIP, gkeNodeInternalIP)
		c.RecordLockReleaseMetrics(opErr)
		if opErr != nil {
			return fmt.Errorf("failed to release lock: %w", opErr)
		}
		klog.Infof("Removing lock info key %s from configmap %s/%s with data %v", key, cm.Namespace, cm.Name, cm.Data)

		if err := c.RemoveKeyFromConfigMapWithRetry(ctx, cm, key); err != nil {
			return fmt.Errorf("failed to remove key %s from configmap %s/%s: %w", key, cm.Namespace, cm.Name, err)
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

// EnqueueCreateEvent adds an object to the createEventQueue of the LockReleaseController.
func (c *LockReleaseController) EnqueueCreateEventObject(obj interface{}) {
	c.createEventQueue.Add(obj)
}

// EnqueueUpdateEvent adds a NodeUpdatePair to the updateEventQueue.
func (c *LockReleaseController) EnqueueUpdateEventObject(oldObj, newObj interface{}) {
	nodeUpdatePair := &NodeUpdatePair{
		OldObj: oldObj.(*corev1.Node), // Type assertion to *v1.Node
		NewObj: newObj.(*corev1.Node), // Type assertion to *v1.Node
	}
	c.updateEventQueue.Add(nodeUpdatePair)
}
