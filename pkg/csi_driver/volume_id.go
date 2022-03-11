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
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/util"
)

// Ordering of elements in volume id
// ID is of form {provisioningMode}/{location}/{instanceName}/{volume}
// Adding a new element should always go at the end
const (
	idProvisioningMode = iota
	idLocation
	idInstance
	idVolume
	totalIDElements // Always last
)

// getVolumeIDFromFileInstance generates an id to uniquely identify the GCFS volume.
// This id is used for volume deletion.
func getVolumeIDFromFileInstance(obj *file.ServiceInstance, mode string) string {
	idElements := make([]string, totalIDElements)
	idElements[idProvisioningMode] = mode
	idElements[idLocation] = obj.Location
	idElements[idInstance] = obj.Name
	idElements[idVolume] = obj.Volume.Name
	return strings.Join(idElements, "/")
}

// getFileInstanceFromID generates a GCFS Instance object from the volume id
func getFileInstanceFromID(id string) (*file.ServiceInstance, string, error) {
	tokens := strings.Split(id, "/")
	if len(tokens) != totalIDElements {
		return nil, "", fmt.Errorf("volume id %q unexpected format: got %v tokens", id, len(tokens))
	}

	return &file.ServiceInstance{
		Location: tokens[idLocation],
		Name:     tokens[idInstance],
		Volume:   file.Volume{Name: tokens[idVolume]},
	}, tokens[idProvisioningMode], nil
}

func generateMultishareVolumeIdFromShare(instancePrefix string, s *file.Share) (string, error) {
	if instancePrefix == "" {
		return "", fmt.Errorf("invalid instance prefix")
	}

	if s == nil || s.Parent == nil {
		return "", fmt.Errorf("invalid share object")
	}

	return fmt.Sprintf("%s/%s/%s/%s/%s/%s", modeMultishare, instancePrefix, s.Parent.Project, s.Parent.Location, s.Parent.Name, s.Name), nil
}

func parseMultishareVolId(volId string) (string, string, string, string, string, error) {
	tokens := strings.Split(volId, "/")
	if len(tokens) != util.MultishareCSIVolIdSplitLen {
		return "", "", "", "", "", fmt.Errorf("invalid volume id %v", volId)
	}

	prefix := tokens[0]
	project := tokens[1]
	location := tokens[2]
	instanceName := tokens[3]
	shareName := tokens[4]
	if project == "" || location == "" || instanceName == "" || shareName == "" {
		return "", "", "", "", "", fmt.Errorf("invalid volume id %v", volId)
	}
	return prefix, project, location, instanceName, shareName, nil
}
