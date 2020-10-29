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
