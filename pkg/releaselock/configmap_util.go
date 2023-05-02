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
	"net"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiError "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/metrics"
)

// Ordering of elements in configmap key
// key is in form {projectID}.{location}.{filestoreName}.{shareName}.{gkeNodeID}.{gkeNodeInternalIP}
// Adding a new element should always go at the end
const (
	projectID = iota
	location
	filestoreName
	shareName
	gkeNodeID         // GCE instance ID for the GKE node
	gkeNodeInternalIP // GKE node internal IP concatenated by underscores
	totalKeyElements  // Always last
)

const (
	ConfigMapNamePrefix = "fscsi-"

	// ConfigMapFinalzer is the finalizer which will be added during configmap creation.
	ConfigMapFinalzer = "filestore.csi.storage.gke.io/lock-release"

	// Concatenation in configmap.
	dot        = "."
	underscore = "_"
)

// ParseConfigMapKey converts the a configmap key into projectID, location, filestoreName, shareName, nodeID, and nodeInternalIP.
// Throws an error if the input key is not in the format of {projectID}.{location}.{filestoreName}.{shareName}.{nodeID}.{nodeInternalIP}
func ParseConfigMapKey(key string) (string, string, string, string, string, string, error) {
	tokens := strings.Split(key, dot)
	if len(tokens) != totalKeyElements {
		return "", "", "", "", "", "", fmt.Errorf("invalid configmap key %s", key)
	}
	projectID := tokens[0]
	location := tokens[1]
	filestoreName := tokens[2]
	shareName := tokens[3]
	gkeNodeID := tokens[4]
	// Convert gkeNodeInternalIP from underscore to dot concatenation.
	gkeNodeInternalIP := strings.ReplaceAll(tokens[5], underscore, dot)
	if net.ParseIP(gkeNodeInternalIP) == nil {
		return "", "", "", "", "", "", fmt.Errorf("invalid GKE node internal IP %s", gkeNodeInternalIP)
	}
	if projectID == "" || location == "" || filestoreName == "" || shareName == "" || gkeNodeID == "" || gkeNodeInternalIP == "" {
		return "", "", "", "", "", "", fmt.Errorf("invalid configmap key %s", key)
	}
	return projectID, location, filestoreName, shareName, gkeNodeID, gkeNodeInternalIP, nil
}

// GenerateConfigMapKey generates a configmap key for the given filestore and GKE node info strings.
// The generated key will be in format {projectID}.{location}.{filestoreName}.{shareName}.{nodeID}.{nodeInternalIP}
// The input gkeNodeInternalIP has to a valid IPV4 address.
// The output nodeInternalIP will be in underscore concatenation.
func GenerateConfigMapKey(projectID, location, filestoreName, shareName, gkeNodeID, gkeNodeInternalIP string) string {
	nodeInternalIP := strings.ReplaceAll(gkeNodeInternalIP, dot, underscore)
	return fmt.Sprintf("%s.%s.%s.%s.%s.%s", projectID, location, filestoreName, shareName, gkeNodeID, nodeInternalIP)
}

// GKENodeNameFromConfigMap extracts the GKE node name from configmap.
// The name of a configmap which stores lock info should start with "fscsi-",
// and will be in format "fscsi-{GKE_node_name}".
func GKENodeNameFromConfigMap(cm *corev1.ConfigMap) (string, error) {
	cmName := cm.Name
	if !strings.HasPrefix(cmName, ConfigMapNamePrefix) {
		return "", fmt.Errorf("invalid configmap name %s", cmName)
	}
	nodeName := cmName[len(ConfigMapNamePrefix):]
	if nodeName == "" {
		return "", fmt.Errorf("invalid configmap name %s", cmName)
	}
	return nodeName, nil
}

// GetConfigMap gets the configmap from the api server.
// Returns nil if the expected configmap is not found.
func (c *LockReleaseController) GetConfigMap(ctx context.Context, cmName, cmNamespace string) (*corev1.ConfigMap, error) {
	cm, err := c.client.CoreV1().ConfigMaps(cmNamespace).Get(ctx, cmName, metav1.GetOptions{})
	if err != nil {
		if apiError.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return cm, nil
}

// CreateConfigMapWithData creates a configmap in the api server.
// Returns the api server's representation of the configmap, and an error, if there is any.
func (c *LockReleaseController) CreateConfigMapWithData(ctx context.Context, cmName, cmNamespace string, data map[string]string) (*corev1.ConfigMap, error) {
	obj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:       cmName,
			Namespace:  cmNamespace,
			Finalizers: []string{ConfigMapFinalzer},
		},
		Data: data,
	}
	cm, err := c.client.CoreV1().ConfigMaps(cmNamespace).Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return cm, nil
}

// UpdateConfigMapWithKeyValue adds a key value pair into configmap.data, and updates the configmap in the api server.
// No-op if the key already exists in configmap.data.
// Returns the server's representation of the configMap, and an error, if there is any.
// UpdateConfigMapWithKeyValue is only called in NodeStageVolume.
func (c *LockReleaseController) UpdateConfigMapWithKeyValue(ctx context.Context, cm *corev1.ConfigMap, key, value string) error {
	if cm.Data == nil {
		cm.Data = map[string]string{}
	}
	// No-op if lock info key already exists in configmap.
	if _, keyExists := cm.Data[key]; keyExists {
		klog.Infof("NodeStageVolume skippped storing lock info {%s: %s} in configmap %s/%s since key %s already exists in configmap.data %v", key, value, cm.Namespace, cm.Name, cm.Data)
		return nil
	}
	klog.Infof("NodeStageVolume storing lock info {%s: %s} in configmap %s/%s with data %v", key, value, cm.Namespace, cm.Name, cm.Data)
	cm.Data[key] = value
	start := time.Now()
	updatedCM, err := c.client.CoreV1().ConfigMaps(cm.Namespace).Update(ctx, cm, metav1.UpdateOptions{})
	duration := time.Since(start)
	c.RecordKubeAPIMetrics(err, metrics.ConfigMapResourceType, metrics.UpdateOpType, metrics.NodeStageOpSource, duration)
	if err != nil {
		return err
	}
	klog.Infof("NodeStageVolume successfully stored lock info {%s: %s} in configmap %s/%s with new data %v", key, value, updatedCM.Namespace, updatedCM.Name, updatedCM.Data)
	return nil
}

// RemoveKeyFromConfigMap deletes the key from configmap.data, then updates the configmap.
// No-op if the key does not exist.
// RemoveKeyFromConfigMap is only called in NodeUnstageVolume.
func (c *LockReleaseController) RemoveKeyFromConfigMap(ctx context.Context, cm *corev1.ConfigMap, key string) error {
	if _, keyExists := cm.Data[key]; !keyExists {
		klog.Infof("NodeUnstageVolume skipped updating configmap %s/%s since key %s not found in configmap.data", cm.Namespace, cm.Name, key)
		return nil
	}

	klog.Infof("NodeUnstageVolume removing key %s from configmap %s/%s with data %v", key, cm.Namespace, cm.Name, cm.Data)
	delete(cm.Data, key)
	start := time.Now()
	updatedCM, err := c.client.CoreV1().ConfigMaps(cm.Namespace).Update(ctx, cm, metav1.UpdateOptions{})
	duration := time.Since(start)
	c.RecordKubeAPIMetrics(err, metrics.ConfigMapResourceType, metrics.UpdateOpType, metrics.NodeUnstageOpSource, duration)
	if err != nil {
		return err
	}
	klog.Infof("NodeUnstageVolume successfully removed key %s from configmap %s/%s, remaning data: %v", key, updatedCM.Namespace, updatedCM.Name, updatedCM.Data)
	return nil
}

// RemoveKeyFromConfigMapWithRetry gets the latest configmap from the api server,
// removes the key from configmap.data, and update the configmap.
// Keeps retrying until configmap successfully update or timeout.
// No-op if the key does not exist.
// RemoveKeyFromConfigMapWithRetry is only called in lock release reconciler.
func (c *LockReleaseController) RemoveKeyFromConfigMapWithRetry(ctx context.Context, cm *corev1.ConfigMap, key string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		start := time.Now()
		latestCM, err := c.client.CoreV1().ConfigMaps(cm.Namespace).Get(ctx, cm.Name, metav1.GetOptions{})
		duration := time.Since(start)
		c.RecordKubeAPIMetrics(err, metrics.ConfigMapResourceType, metrics.GetOpType, metrics.ReconcilerOpSource, duration)
		if err != nil {
			return err
		}
		if _, keyExists := cm.Data[key]; !keyExists {
			klog.Infof("Skip updating configmap %s/%s: key %s not found in configmap.data", cm.Namespace, cm.Name, key)
			return nil
		}
		delete(latestCM.Data, key)
		start = time.Now()
		_, updateErr := c.client.CoreV1().ConfigMaps(latestCM.Namespace).Update(ctx, latestCM, metav1.UpdateOptions{})
		duration = time.Since(start)
		c.RecordKubeAPIMetrics(updateErr, metrics.ConfigMapResourceType, metrics.UpdateOpType, metrics.ReconcilerOpSource, duration)
		return updateErr
	})
}
