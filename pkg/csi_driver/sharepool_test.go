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

	tests := []struct {
		name               string
		req                *csi.CreateVolumeRequest
		featureEnabled     bool
		expectErr          bool
		expectDefaultBytes bool
	}{
		{
			name: "successful allocation",
			req: &csi.CreateVolumeRequest{
				Name: "test-sharepool-volume",
				Parameters: map[string]string{
					paramKeySharePool: sharePoolPath,
				},
				VolumeCapabilities: volumeCapabilities,
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 1 * util.Gb,
				},
			},
			featureEnabled: true,
		},
		{
			name: "omitted capacity defaulting",
			req: &csi.CreateVolumeRequest{
				Name: "test-sharepool-default-cap",
				Parameters: map[string]string{
					paramKeySharePool: sharePoolPath,
				},
				VolumeCapabilities: volumeCapabilities,
				CapacityRange:      nil,
			},
			featureEnabled:     true,
			expectDefaultBytes: true,
		},
		{
			name: "missing request name",
			req: &csi.CreateVolumeRequest{
				Name: "",
				Parameters: map[string]string{
					paramKeySharePool: sharePoolPath,
				},
				VolumeCapabilities: volumeCapabilities,
			},
			featureEnabled: true,
			expectErr:      true,
		},
		{
			name: "feature disabled",
			req: &csi.CreateVolumeRequest{
				Name: "test-sharepool-volume",
				Parameters: map[string]string{
					paramKeySharePool: sharePoolPath,
				},
				VolumeCapabilities: volumeCapabilities,
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 1 * util.Gb,
				},
			},
			featureEnabled: false,
			expectErr:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cs := initTestController(t).(*controllerServer)
			cs.config.features.FeatureSharePools = &FeatureSharePools{Enabled: tc.featureEnabled}

			resp, err := cs.CreateVolume(ctx, tc.req)
			if tc.expectErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("CreateVolume failed: %v", err)
			}
			if resp == nil || resp.Volume == nil {
				t.Fatalf("expected non-nil volume response")
			}
			if !strings.HasPrefix(resp.Volume.VolumeId, sharePoolURLPrefix) || !strings.Contains(resp.Volume.VolumeId, "/shares/") {
				t.Errorf("VolumeId = %q, want prefix %q and '/shares/'", resp.Volume.VolumeId, sharePoolURLPrefix)
			}
			if got, want := resp.Volume.VolumeContext[attrIP], "10.1.1.1"; got != want {
				t.Errorf("VolumeContext[%s] = %q, want %q", attrIP, got, want)
			}
			if got := resp.Volume.VolumeContext[attrVolume]; !strings.HasPrefix(got, "share-") {
				t.Errorf("VolumeContext[%s] = %q, want prefix 'share-'", attrVolume, got)
			}
			if tc.expectDefaultBytes {
				expectedDefaultBytes := util.GbToBytes(defaultSharePoolShareCapacityGb)
				if got, want := resp.Volume.CapacityBytes, expectedDefaultBytes; got != want {
					t.Errorf("CapacityBytes = %d, want %d", got, want)
				}
			}

			if tc.name == "successful allocation" {
				// Test idempotency/retry
				respRetry, err := cs.CreateVolume(ctx, tc.req)
				if err != nil {
					t.Fatalf("CreateVolume retry failed: %v", err)
				}
				if got, want := respRetry.Volume.VolumeId, resp.Volume.VolumeId; got != want {
					t.Errorf("VolumeId on retry = %q, want %q", got, want)
				}
				if got, want := respRetry.Volume.VolumeContext[attrVolume], resp.Volume.VolumeContext[attrVolume]; got != want {
					t.Errorf("VolumeContext[%s] on retry = %q, want %q", attrVolume, got, want)
				}
			}
		})
	}
}

func TestDeleteVolume_SharePool(t *testing.T) {
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

	tests := []struct {
		name           string
		volumeIDFunc   func(allocatedID string) string
		featureEnabled bool
		expectErr      bool
		runDouble      bool
	}{
		{
			name:           "successful deletion",
			volumeIDFunc:   func(allocatedID string) string { return allocatedID },
			featureEnabled: true,
		},
		{
			name:           "idempotent delete",
			volumeIDFunc:   func(allocatedID string) string { return allocatedID },
			featureEnabled: true,
			runDouble:      true,
		},
		{
			name:           "invalid malformed volume ID",
			volumeIDFunc:   func(allocatedID string) string { return sharePoolURLPrefix + "test-project/us-central1/my-pool/shares" },
			featureEnabled: true,
			expectErr:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cs := initTestController(t).(*controllerServer)
			cs.config.features.FeatureSharePools = &FeatureSharePools{Enabled: tc.featureEnabled}

			// Allocate a volume first
			createReq := &csi.CreateVolumeRequest{
				Name: "test-sharepool-volume",
				Parameters: map[string]string{
					paramKeySharePool: sharePoolPath,
				},
				VolumeCapabilities: volumeCapabilities,
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 1 * util.Gb,
				},
			}
			resp, err := cs.CreateVolume(ctx, createReq)
			if err != nil {
				t.Fatalf("CreateVolume failed: %v", err)
			}

			targetVolumeID := tc.volumeIDFunc(resp.Volume.VolumeId)
			delReq := &csi.DeleteVolumeRequest{
				VolumeId: targetVolumeID,
			}

			_, err = cs.DeleteVolume(ctx, delReq)
			if tc.expectErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("DeleteVolume failed: %v", err)
			}

			if tc.runDouble {
				_, err = cs.DeleteVolume(ctx, delReq)
				if err != nil {
					t.Errorf("DeleteVolume retry failed: %v", err)
				}
			}
		})
	}
}
