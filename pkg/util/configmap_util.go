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

package util

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apiError "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
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
	// ConfigMapNamespace  = "gcp-filestore-csi-driver"
	ConfigMapNamespace = "gke-managed-filestorecsi"
	ConfigMapFinalizer = "filestore.csi.storage.gke.io/lock-release"

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
func GetConfigMap(ctx context.Context, cmName, cmNamespace string, client kubernetes.Interface) (*corev1.ConfigMap, error) {
	cm, err := client.CoreV1().ConfigMaps(cmNamespace).Get(ctx, cmName, metav1.GetOptions{})
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
func CreateConfigMapWithData(ctx context.Context, cmName, cmNamespace string, data map[string]string, client kubernetes.Interface) (*corev1.ConfigMap, error) {
	obj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:       cmName,
			Namespace:  cmNamespace,
			Finalizers: []string{ConfigMapFinalizer},
		},
		Data: data,
	}
	cm, err := client.CoreV1().ConfigMaps(cmNamespace).Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return cm, nil
}

// UpdateConfigMapWithKeyValue adds a key value pair into configmap.data, and updates the configmap in the api server.
// No-op if the key already exists in configmap.data.
// Returns the server's representation of the configMap, and an error, if there is any.
func UpdateConfigMapWithKeyValue(ctx context.Context, cm *corev1.ConfigMap, key, value string, client kubernetes.Interface) (*corev1.ConfigMap, error) {
	if cm.Data == nil {
		cm.Data = map[string]string{}
	}
	// No-op if key already exists.
	if _, ok := cm.Data[key]; ok {
		return cm, nil
	}
	cm.Data[key] = value
	updatedCM, err := client.CoreV1().ConfigMaps(cm.Namespace).Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	return updatedCM, nil
}

// UpdateConfigMapWithData sets configmap.data to the given map, and updates the configmap in the api server.
// No-op if no changes to configmap.data.
// Returns the server's representation of the configmap, and an error, if there is any.
func UpdateConfigMapWithData(ctx context.Context, cm *corev1.ConfigMap, data map[string]string, client kubernetes.Interface) (*corev1.ConfigMap, error) {
	// No-op if no changes to configmap.data.
	if reflect.DeepEqual(cm.Data, data) {
		klog.Infof("Skip updating configmap %s/%s since configmap.data not changed", cm.Namespace, cm.Name)
		return cm, nil
	}

	klog.Infof("Updating configmap %s/%s with data %v", cm.Namespace, cm.Name, data)
	cm.Data = data
	updatedCM, err := client.CoreV1().ConfigMaps(cm.Namespace).Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	return updatedCM, nil
}

// DeleteConfigMap removes the finalizer, and deletes the configmap from the api server.
func DeleteConfigMap(ctx context.Context, cm *corev1.ConfigMap, client kubernetes.Interface) error {
	if containsFinalizer(cm, ConfigMapFinalizer) {
		removeFinalizer(cm, ConfigMapFinalizer)
		if _, err := client.CoreV1().ConfigMaps(cm.Namespace).Update(ctx, cm, metav1.UpdateOptions{}); err != nil {
			if apiError.IsNotFound(err) {
				return nil
			}
			return err
		}
	}
	if cm.DeletionTimestamp == nil {
		if err := client.CoreV1().ConfigMaps(cm.Namespace).Delete(ctx, cm.Name, metav1.DeleteOptions{}); err != nil {
			if apiError.IsNotFound(err) {
				return nil
			}
			return err
		}
	}
	return nil
}

// RemoveKeyFromConfigMap deletes the key from configmap.data.
// No-op if the key does not exist.
// Delete the configmap from api server if configmap.data is an empty map.
// Otherwise, update the configmap.
func RemoveKeyFromConfigMap(ctx context.Context, cm *corev1.ConfigMap, key string, client kubernetes.Interface) (*corev1.ConfigMap, error) {
	_, cmNeedsUpdate := cm.Data[key]
	if !cmNeedsUpdate {
		return cm, nil
	}

	delete(cm.Data, key)
	if len(cm.Data) == 0 {
		return nil, DeleteConfigMap(ctx, cm, client)
	}
	updatedCM, err := client.CoreV1().ConfigMaps(cm.Namespace).Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	return updatedCM, nil
}

// containsFinalizer checks a configmap object that the provided finalizer is present.
func containsFinalizer(cm *corev1.ConfigMap, finalizer string) bool {
	f := cm.GetFinalizers()
	for _, e := range f {
		if e == finalizer {
			return true
		}
	}
	return false
}

// removeFinalizer accepts a configmap object and removes the provided finalizer if present.
func removeFinalizer(cm *corev1.ConfigMap, finalizer string) {
	f := cm.GetFinalizers()
	for i := 0; i < len(f); i++ {
		if f[i] == finalizer {
			f = append(f[:i], f[i+1:]...)
			i--
		}
	}
	cm.SetFinalizers(f)
}
