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

package util

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	Gb = 1024 * 1024 * 1024
	Tb = 1024 * Gb

	// VolumeSnapshot parameters
	VolumeSnapshotTypeKey      = "type"
	VolumeSnapshotLocationKey  = "location"
	VolumeSnapshotTypeSnapshot = "snapshot"
	VolumeSnapshotTypeBackup   = "backup"

	SnapshotHandleBackupKey = "backups"

	// number of elements in a snapshot Id.
	// For backups: projects/{project name}/locations/{region}/backups/{name}
	// For snapshot: projects/{project name}/locations/{zone}/snapshots/{name}
	snapshotTotalElements = 6

	// number of elements in backup Volume sources e.g. projects/{project name}/locations/{zone}/instances/{name}
	volumeTotalElements = 6
)

// Round up to the nearest Gb
func RoundBytesToGb(bytes int64) int64 {
	return (bytes + Gb - 1) / Gb
}

func BytesToGb(bytes int64) int64 {
	return bytes / Gb
}

func GbToBytes(gbs int64) int64 {
	return gbs * Gb
}

func Min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func Max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// ConvertLabelsStringToMap converts the labels from string to map
// example: "key1=value1,key2=value2" gets converted into {"key1": "value1", "key2": "value2"}
func ConvertLabelsStringToMap(labels string) (map[string]string, error) {
	const labelsDelimiter = ","
	const labelsKeyValueDelimiter = "="

	labelsMap := make(map[string]string)
	if labels == "" {
		return labelsMap, nil
	}

	// Following rules enforced for label keys
	// 1. Keys have a minimum length of 1 character and a maximum length of 63 characters, and cannot be empty.
	// 2. Keys and values can contain only lowercase letters, numeric characters, underscores, and dashes.
	// 3. Keys must start with a lowercase letter.
	regexKey, _ := regexp.Compile(`^\p{Ll}[\p{Ll}0-9_-]{0,62}$`)
	checkLabelKeyFn := func(key string) error {
		if !regexKey.MatchString(key) {
			return fmt.Errorf("label value %q is invalid (should start with lowercase letter / lowercase letter, digit, _ and - chars are allowed / 1-63 characters", key)
		}
		return nil
	}

	// Values can be empty, and have a maximum length of 63 characters.
	regexValue, _ := regexp.Compile(`^[\p{Ll}0-9_-]{0,63}$`)
	checkLabelValueFn := func(value string) error {
		if !regexValue.MatchString(value) {
			return fmt.Errorf("label value %q is invalid (lowercase letter, digit, _ and - chars are allowed / 0-63 characters", value)
		}

		return nil
	}

	keyValueStrings := strings.Split(labels, labelsDelimiter)
	for _, keyValue := range keyValueStrings {
		keyValue := strings.Split(keyValue, labelsKeyValueDelimiter)

		if len(keyValue) != 2 {
			return nil, fmt.Errorf("labels %q are invalid, correct format: 'key1=value1,key2=value2'", labels)
		}

		key := strings.TrimSpace(keyValue[0])
		if err := checkLabelKeyFn(key); err != nil {
			return nil, err
		}

		value := strings.TrimSpace(keyValue[1])
		if err := checkLabelValueFn(value); err != nil {
			return nil, err
		}

		labelsMap[key] = value
	}

	const maxNumberOfLabels = 64
	if len(labelsMap) > maxNumberOfLabels {
		return nil, fmt.Errorf("more than %d labels is not allowed, given: %d", maxNumberOfLabels, len(labelsMap))
	}

	return labelsMap, nil
}

func GetRegionFromZone(location string) (string, error) {
	tokens := strings.Split(location, "-")
	if len(tokens) != 3 {
		return "", fmt.Errorf("Failed to parse location %v", location)
	}

	tokens = tokens[:2]
	return strings.Join(tokens, "-"), nil
}

func ParseTimestamp(timestamp string) (*timestamppb.Timestamp, error) {
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("Failed to parse timestamp %v: %v", timestamp, err))
	}

	tp, err := ptypes.TimestampProto(t)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("Failed to covert timestamp %v: %v", timestamp, err))
	}
	return tp, err
}

func IsBackupHandle(handle string) (bool, error) {
	splitId := strings.Split(handle, "/")
	if len(splitId) != snapshotTotalElements {
		return false, fmt.Errorf("failed to get id components. Expected 'projects/{project}/location/{zone|region}/[snapshots|backups]/{name}'. Got: %s", handle)
	}
	return splitId[4] == SnapshotHandleBackupKey, nil
}

func IsSnapshotTypeSupported(params map[string]string) (bool, error) {
	if params == nil {
		return false, fmt.Errorf("Empty parameters in VolumeSnapshot")
	}
	snapType, ok := params[VolumeSnapshotTypeKey]
	if !ok {
		return false, fmt.Errorf("Volume snapshot type is missing")
	}
	if snapType != VolumeSnapshotTypeBackup {
		return false, fmt.Errorf("Volume snapshot type %q not supported", snapType)
	}
	return true, nil
}

func GetBackupLocation(params map[string]string) string {
	location := ""
	if params == nil {
		return location
	}

	location, _ = params[VolumeSnapshotLocationKey]
	return location
}

func BackupVolumeSourceToCSIVolumeHandle(backupVolumeSource string) (string, error) {
	splitId := strings.Split(backupVolumeSource, "/")
	if len(splitId) != volumeTotalElements {
		return "", fmt.Errorf("Failed to get id components. Expected 'projects/{project}/location/{zone}/instances/{name}'. Got: %s", backupVolumeSource)
	}
	return fmt.Sprintf("modeInstance/%s/%s/vol1", splitId[3], splitId[5]), nil
}

// Multishare util functions.

func ConvertVolToShareName(csiVolName string) string {
	s := strings.ToLower(csiVolName)
	return strings.ReplaceAll(s, "-", "_")
	// TODO: verify regex
}

func CheckLabelValueRegex(value string) error {
	// Values can be empty, and have a maximum length of 63 characters.
	regexValue, _ := regexp.Compile(`^[\p{Ll}0-9_-]{0,63}$`)
	if !regexValue.MatchString(value) {
		return fmt.Errorf("value %q is invalid (lowercase letter, digit, _ and - chars are allowed / 0-63 characters", value)
	}

	return nil
}

func ParseInstanceKey(instanceKey InstanceKey) (string, string, string, error) {
	// Expected instance handle <project-name>/<location-name>/<instance-name>
	splitStr := strings.Split(string(instanceKey), "/")
	if len(splitStr) != InstanceHandleSplitLen {
		return "", "", "", fmt.Errorf("Unknown instance handle format %q", instanceKey)
	}

	project := splitStr[0]
	location := splitStr[1]
	instanceName := splitStr[2]
	if project == "" || location == "" || instanceName == "" {
		return "", "", "", fmt.Errorf("Unknown instance handle format %q", instanceKey)
	}

	return project, location, instanceName, nil
}

func ParseInstanceURI(instanceURI string) (string, string, string, error) {
	// Expected instance URI projects/<project-name>/locations/<location-name>/instances/<instance-name>
	splitStr := strings.Split(instanceURI, "/")
	if len(splitStr) != InstanceURISplitLen {
		return "", "", "", fmt.Errorf("Unknown instance URI format %q", instanceURI)
	}

	project := splitStr[1]
	location := splitStr[3]
	instanceName := splitStr[5]
	if project == "" || location == "" || instanceName == "" {
		return "", "", "", fmt.Errorf("Unknown instance URI format %q", instanceURI)
	}

	return project, location, instanceName, nil
}

func ParseShareHandle(shareHandle string) (string, string, string, string, error) {
	// Expected share handle <project-name>/<location-name>/<instance-name>/<share-name>
	splitStr := strings.Split(shareHandle, "/")
	if len(splitStr) != ShareHandleSplitLen {
		return "", "", "", "", fmt.Errorf("Unknown share handle format %q", shareHandle)
	}

	project := splitStr[0]
	location := splitStr[1]
	instanceName := splitStr[2]
	shareName := splitStr[3]
	if project == "" || location == "" || instanceName == "" || shareName == "" {
		return "", "", "", "", fmt.Errorf("Unknown share handle format %q", shareHandle)
	}

	return project, location, instanceName, shareName, nil
}

func ParseShareURI(shareURI string) (string, string, string, string, error) {
	// Expected share URI projects/<project-name>/locations/<location-name>/instances/<instance-name>/shares/<share-name>
	splitStr := strings.Split(shareURI, "/")
	if len(splitStr) != ShareURISplitLen {
		return "", "", "", "", fmt.Errorf("Unknown share URI format %q", shareURI)
	}

	project := splitStr[1]
	location := splitStr[3]
	instanceName := splitStr[5]
	shareName := splitStr[7]
	if project == "" || location == "" || instanceName == "" || shareName == "" {
		return "", "", "", "", fmt.Errorf("Unknown share URI format %q", shareURI)
	}

	return project, location, instanceName, shareName, nil
}

func GetMultishareOpsTimeoutConfig(opType OperationType) (time.Duration, time.Duration, error) {
	switch opType {
	case InstanceCreate, InstanceDelete, ShareDelete:
		return 1 * time.Hour, 5 * time.Second, nil
	case InstanceUpdate, ShareCreate, ShareUpdate:
		return 10 * time.Minute, 5 * time.Second, nil
	default:
		return 0, 0, fmt.Errorf("unknown op type %v", opType)
	}
}
