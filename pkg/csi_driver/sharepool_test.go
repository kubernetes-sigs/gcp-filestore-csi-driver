/*
Copyright 2026 The Kubernetes Authors.

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
	"context"
	"strings"
	"testing"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/util"
)

func TestCreateVolume_SharePool(t *testing.T) {
	cs := initTestController(t).(*controllerServer)
	ctx := context.Background()

	sharePoolPath := "projects/test-project/locations/us-central1/sharePools/my-pool"
	volumeCapabilities := []*csi.VolumeCapability{
		{
			AccessType: &csi.VolumeCapability_Mount{
				Mount: &csi.VolumeCapability_MountVolume{},
			},
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			},
		},
	}

	// Case 1: Successful allocation
	req := &csi.CreateVolumeRequest{
		Name: "test-sharepool-volume",
		Parameters: map[string]string{
			paramSharePool: sharePoolPath,
		},
		VolumeCapabilities: volumeCapabilities,
		CapacityRange: &csi.CapacityRange{
			RequiredBytes: 1 * util.Gb,
		},
	}

	resp, err := cs.CreateVolume(ctx, req)
	if err != nil {
		t.Fatalf("CreateVolume failed: %v", err)
	}

	if resp == nil || resp.Volume == nil {
		t.Fatalf("expected non-nil volume response")
	}

	if !strings.HasPrefix(resp.Volume.VolumeId, "sharepool://") || !strings.Contains(resp.Volume.VolumeId, "/shares/") {
		t.Errorf("expected VolumeId to start with sharepool:// and contain '/shares/', got %s", resp.Volume.VolumeId)
	}

	if resp.Volume.VolumeContext[attrIP] != "10.1.1.1" {
		t.Errorf("expected IP 10.1.1.1, got %s", resp.Volume.VolumeContext[attrIP])
	}

	if !strings.HasPrefix(resp.Volume.VolumeContext[attrVolume], "share-") {
		t.Errorf("expected share name to start with 'share-', got %s", resp.Volume.VolumeContext[attrVolume])
	}

	// Case 2: Idempotency / retry
	respRetry, err := cs.CreateVolume(ctx, req)
	if err != nil {
		t.Fatalf("CreateVolume retry failed: %v", err)
	}

	if respRetry.Volume.VolumeId != resp.Volume.VolumeId {
		t.Errorf("expected identical VolumeId for retry, got %s, expected %s", respRetry.Volume.VolumeId, resp.Volume.VolumeId)
	}

	if respRetry.Volume.VolumeContext[attrVolume] != resp.Volume.VolumeContext[attrVolume] {
		t.Errorf("expected identical ShareName for retry, got %s, expected %s", respRetry.Volume.VolumeContext[attrVolume], resp.Volume.VolumeContext[attrVolume])
	}

	// Case 3: Omitted capacity defaulting to defaultSharePoolShareCapacityGb (1 GiB)
	reqDefaultCap := &csi.CreateVolumeRequest{
		Name: "test-sharepool-default-cap",
		Parameters: map[string]string{
			paramSharePool: sharePoolPath,
		},
		VolumeCapabilities: volumeCapabilities,
		CapacityRange:      nil, // No capacity specified
	}

	respDefaultCap, err := cs.CreateVolume(ctx, reqDefaultCap)
	if err != nil {
		t.Fatalf("CreateVolume with default capacity failed: %v", err)
	}
	expectedDefaultBytes := util.GbToBytes(defaultSharePoolShareCapacityGb)
	if respDefaultCap.Volume.CapacityBytes != expectedDefaultBytes {
		t.Errorf("expected defaulted capacity to be %d bytes, got %d", expectedDefaultBytes, respDefaultCap.Volume.CapacityBytes)
	}

	// Case 4: Missing Request Name (returns error)
	reqMissingName := &csi.CreateVolumeRequest{
		Name: "",
		Parameters: map[string]string{
			paramSharePool: sharePoolPath,
		},
		VolumeCapabilities: volumeCapabilities,
	}
	_, err = cs.CreateVolume(ctx, reqMissingName)
	if err == nil {
		t.Errorf("expected error when name is empty, got nil")
	}
}

func TestDeleteVolume_SharePool(t *testing.T) {
	cs := initTestController(t).(*controllerServer)
	ctx := context.Background()

	sharePoolPath := "projects/test-project/locations/us-central1/sharePools/my-pool"
	volumeCapabilities := []*csi.VolumeCapability{
		{
			AccessType: &csi.VolumeCapability_Mount{
				Mount: &csi.VolumeCapability_MountVolume{},
			},
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			},
		},
	}

	// Allocate first
	req := &csi.CreateVolumeRequest{
		Name: "test-sharepool-volume",
		Parameters: map[string]string{
			paramSharePool: sharePoolPath,
		},
		VolumeCapabilities: volumeCapabilities,
		CapacityRange: &csi.CapacityRange{
			RequiredBytes: 1 * util.Gb,
		},
	}

	resp, err := cs.CreateVolume(ctx, req)
	if err != nil {
		t.Fatalf("CreateVolume failed: %v", err)
	}

	// Case 1: Successful deletion
	delReq := &csi.DeleteVolumeRequest{
		VolumeId: resp.Volume.VolumeId,
	}

	_, err = cs.DeleteVolume(ctx, delReq)
	if err != nil {
		t.Fatalf("DeleteVolume failed: %v", err)
	}

	// Case 2: Idempotent delete (success even if already deleted / not found)
	_, err = cs.DeleteVolume(ctx, delReq)
	if err != nil {
		t.Errorf("DeleteVolume retry failed: %v", err)
	}

	// Case 3: Invalid/Malformed VolumeID (returns error)
	invalidDelReq := &csi.DeleteVolumeRequest{
		VolumeId: "sharepool://test-project/us-central1/my-pool/shares", // missing UUID part
	}
	_, err = cs.DeleteVolume(ctx, invalidDelReq)
	if err == nil {
		t.Errorf("expected error when deleting malformed volume ID, got nil")
	}
}
