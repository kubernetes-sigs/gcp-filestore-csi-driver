/*
Copyright 2018 The Kubernetes Authors.

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

package driver

import (
	"fmt"
	"strings"

	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/file"
)

// Ordering of elements in volume id
// ID is of form {provisioningMode}/{location}/{instanceName}/{volume}
// Adding a new element should always go at the end
const (
	idProvisioningMode = iota
	idLocation
	idInstance
	idVolume
	totalIdElements // Always last
)

// getVolumeIdFromFileInstance generates an id to uniquely identify the GCFS volume.
// This id is used for volume deletion.
func getVolumeIdFromFileInstance(obj *file.ServiceInstance, mode string) string {
	idElements := make([]string, totalIdElements)
	idElements[idProvisioningMode] = mode
	idElements[idLocation] = obj.Location
	idElements[idInstance] = obj.Name
	idElements[idVolume] = obj.Volume.Name
	return strings.Join(idElements, "/")
}

// getFileInstanceFromId generates a GCFS Instance object from the volume id
func getFileInstanceFromId(id string) (*file.ServiceInstance, string, error) {
	tokens := strings.Split(id, "/")
	if len(tokens) != totalIdElements {
		return nil, "", fmt.Errorf("volume id %q unexpected format: got %v tokens", id, len(tokens))
	}

	return &file.ServiceInstance{
		Location: tokens[idLocation],
		Name:     tokens[idInstance],
		Volume:   file.Volume{Name: tokens[idVolume]},
	}, tokens[idProvisioningMode], nil
}
