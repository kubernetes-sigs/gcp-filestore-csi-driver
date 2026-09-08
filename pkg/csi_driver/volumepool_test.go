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

func TestCreateVolume_VolumePool(t *testing.T) {
	ctx := context.Background()
	volumePoolPath := "projects/test-project/locations/us-central1/volumePools/my-pool"
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
			name: "successful allocation with volume-pool",
			req: &csi.CreateVolumeRequest{
				Name: "test-volumepool-volume",
				Parameters: map[string]string{
					paramKeyVolumePool: volumePoolPath,
				},
				VolumeCapabilities: volumeCapabilities,
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 1 * util.Gb,
				},
			},
			featureEnabled: true,
		},
		{
			name: "successful allocation with user-requested NFSv4.1",
			req: &csi.CreateVolumeRequest{
				Name: "test-volumepool-nfsv41",
				Parameters: map[string]string{
					paramKeyVolumePool: volumePoolPath,
					paramFileProtocol:  v4_1FileProtocol,
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
				Name: "test-volumepool-default-cap",
				Parameters: map[string]string{
					paramKeyVolumePool: volumePoolPath,
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
					paramKeyVolumePool: volumePoolPath,
				},
				VolumeCapabilities: volumeCapabilities,
			},
			featureEnabled: true,
			expectErr:      true,
		},
		{
			name: "feature disabled",
			req: &csi.CreateVolumeRequest{
				Name: "test-volumepool-volume",
				Parameters: map[string]string{
					paramKeyVolumePool: volumePoolPath,
				},
				VolumeCapabilities: volumeCapabilities,
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 1 * util.Gb,
				},
			},
			featureEnabled: false,
			expectErr:      true,
		},
		{
			name: "pool exhausted",
			req: &csi.CreateVolumeRequest{
				Name: "test-volumepool-exhausted",
				Parameters: map[string]string{
					paramKeyVolumePool: "projects/test-project/locations/us-central1/volumePools/exhausted-pool",
				},
				VolumeCapabilities: volumeCapabilities,
			},
			featureEnabled: true,
			expectErr:      true,
		},
		{
			name: "with mount options",
			req: &csi.CreateVolumeRequest{
				Name: "test-volumepool-mount-opts",
				Parameters: map[string]string{
					paramKeyVolumePool: volumePoolPath,
					paramMountOptions:  "nconnect=8,rsize=1048576,wsize=1048576",
				},
				VolumeCapabilities: volumeCapabilities,
			},
			featureEnabled: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cs := initTestController(t).(*controllerServer)

			cs.config.features.FeatureVolumePools = &FeatureVolumePools{Enabled: tc.featureEnabled}

			resp, err := cs.CreateVolume(ctx, tc.req)
			if tc.expectErr {
				if err == nil {
					t.Errorf("expected error, got nil response: %v", resp)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resp == nil || resp.Volume == nil {
				t.Fatalf("expected non-nil Volume in response")
			}

			if !isVolumePoolVolumeID(resp.Volume.VolumeId) {
				t.Errorf("expected volumepool:// composite volume ID, got %q", resp.Volume.VolumeId)
			}

			if resp.Volume.VolumeContext[attrIP] == "" {
				t.Errorf("expected ip attribute in VolumeContext")
			}

			if resp.Volume.VolumeContext[attrVolume] == "" {
				t.Errorf("expected volume attribute in VolumeContext")
			}

			expectedProtocol := v3FileProtocol
			if protocolParam, ok := tc.req.GetParameters()[paramFileProtocol]; ok && strings.ToUpper(protocolParam) == v4_1FileProtocol {
				expectedProtocol = v4_1FileProtocol
			}

			if resp.Volume.VolumeContext[attrFileProtocol] != expectedProtocol {
				t.Errorf("expected fileProtocol %q, got %q", expectedProtocol, resp.Volume.VolumeContext[attrFileProtocol])
			}

			if expectedMountOpts, ok := tc.req.GetParameters()[paramMountOptions]; ok {
				if resp.Volume.VolumeContext[attrMountOptions] != expectedMountOpts {
					t.Errorf("expected mountOptions %q, got %q", expectedMountOpts, resp.Volume.VolumeContext[attrMountOptions])
				}
			}

			// Capacity validation is intentionally skipped as we bypass K8s standard PVC capacity tests for backend pool agent operations.
		})
	}
}

func TestDeleteVolume_VolumePool(t *testing.T) {
	ctx := context.Background()
	validVolumePoolID := "volumepool://test-project/us-central1/my-pool/volumes/test-vol?ip=10.1.1.1"

	tests := []struct {
		name           string
		volumeID       string
		preAllocate    bool
		featureEnabled bool
		expectErr      bool
	}{
		{
			name:           "successful deletion of volume pool volume",
			volumeID:       validVolumePoolID,
			preAllocate:    true,
			featureEnabled: true,
		},
		{
			name:           "idempotent deletion of non-existing volume",
			volumeID:       validVolumePoolID,
			preAllocate:    false,
			featureEnabled: true,
		},
		{
			name:           "feature disabled",
			volumeID:       validVolumePoolID,
			preAllocate:    true,
			featureEnabled: false,
			expectErr:      true,
		},
		{
			name:           "invalid composite volume id",
			volumeID:       "volumepool://invalid-id-format",
			featureEnabled: true,
			expectErr:      true,
		},
		{
			name:           "empty volume id",
			volumeID:       "",
			featureEnabled: true,
			expectErr:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cs := initTestController(t).(*controllerServer)

			cs.config.features.FeatureVolumePools = &FeatureVolumePools{Enabled: tc.featureEnabled}

			if tc.preAllocate {
				_, err := cs.config.fileService.CreateVolumePoolVolume(ctx, "projects/test-project/locations/us-central1/volumePools/my-pool", "test-vol")
				if err != nil {
					t.Fatalf("failed to pre-allocate volume: %v", err)
				}
			}

			req := &csi.DeleteVolumeRequest{
				VolumeId: tc.volumeID,
			}

			resp, err := cs.DeleteVolume(ctx, req)
			if tc.expectErr {
				if err == nil {
					t.Errorf("expected error, got nil response: %v", resp)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resp == nil {
				t.Fatalf("expected non-nil response")
			}
		})
	}
}

func TestVolumePoolID_ParseAndBuild(t *testing.T) {
	parentPool := "projects/my-project/locations/us-central1/volumePools/fifa-pool"
	volID := "pvc-12345-6789"
	ip := "10.10.10.10"

	builtID := buildVolumePoolVolumeID(parentPool, volID, ip)
	expectedID := "volumepool://my-project/us-central1/fifa-pool/volumes/pvc-12345-6789?ip=10.10.10.10"

	if builtID != expectedID {
		t.Errorf("buildVolumePoolVolumeID mismatch: expected %q, got %q", expectedID, builtID)
	}

	parsedParent, parsedVolID, parsedIP, err := parseVolumePoolVolumeID(builtID)
	if err != nil {
		t.Fatalf("unexpected error parsing volume ID: %v", err)
	}

	if parsedParent != parentPool {
		t.Errorf("parent mismatch: expected %q, got %q", parentPool, parsedParent)
	}
	if parsedVolID != volID {
		t.Errorf("volID mismatch: expected %q, got %q", volID, parsedVolID)
	}
	if parsedIP != ip {
		t.Errorf("IP mismatch: expected %q, got %q", ip, parsedIP)
	}
}
