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
	"math"
	"slices"
	"strings"
	"testing"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/file"
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
		name           string
		req            *csi.CreateVolumeRequest
		featureEnabled bool
		// wantCode is the code of the error that CreateVolume must return.
		wantCode codes.Code
		// wantCapacityBytes is the capacity that the response must report.
		// Without a requested capacity, the fake gives the volume a quota of
		// 1024 MiB, as the API gives it the volume pool's default quota.
		wantCapacityBytes int64
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
			featureEnabled:    true,
			wantCapacityBytes: 1 * util.Gb,
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
			featureEnabled:    true,
			wantCapacityBytes: 1 * util.Gb,
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
			featureEnabled:    true,
			wantCapacityBytes: 1024 * util.Mb,
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
			wantCode:       codes.InvalidArgument,
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
			wantCode:       codes.FailedPrecondition,
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
			wantCode:       codes.ResourceExhausted,
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
			featureEnabled:    true,
			wantCapacityBytes: 1024 * util.Mb,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cs := initTestController(t).(*controllerServer)

			cs.config.features.FeatureVolumePools = &FeatureVolumePools{Enabled: tc.featureEnabled}

			resp, err := cs.CreateVolume(ctx, tc.req)
			if code := status.Code(err); code != tc.wantCode {
				t.Fatalf("CreateVolume() error = %v (code %v), want code %v", err, code, tc.wantCode)
			}
			if tc.wantCode != codes.OK {
				return
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

			if resp.Volume.CapacityBytes != tc.wantCapacityBytes {
				t.Errorf("CapacityBytes = %d, want %d", resp.Volume.CapacityBytes, tc.wantCapacityBytes)
			}
		})
	}
}

// volumePoolQuotaService wraps a file.Service, and records the quotas that
// volume pool volumes are created with. With overrideQuota set, it returns the
// volumes that it creates with a quota of reportedQuotaMib, as an API that
// reports another quota, or no quota (0), would.
type volumePoolQuotaService struct {
	file.Service
	overrideQuota    bool
	reportedQuotaMib int64
	quotas           []int64
}

func (s *volumePoolQuotaService) CreateVolumePoolVolume(ctx context.Context, parentPool, volumeID string, quotaMib int64) (*file.PoolVolume, error) {
	s.quotas = append(s.quotas, quotaMib)
	vol, err := s.Service.CreateVolumePoolVolume(ctx, parentPool, volumeID, quotaMib)
	if err != nil || !s.overrideQuota {
		return vol, err
	}
	reported := *vol
	reported.QuotaMib = s.reportedQuotaMib
	return &reported, nil
}

// TestCreateVolume_VolumePoolQuota checks that CreateVolume creates a volume
// pool volume with the quota that the capacity range of the request asks for,
// refuses capacity ranges that it doesn't support without calling the API, and
// reports the quota that the API returns as the volume's capacity.
func TestCreateVolume_VolumePoolQuota(t *testing.T) {
	ctx := context.Background()
	const volumePoolPath = "projects/test-project/locations/us-central1/volumePools/my-pool"
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
		name     string
		capRange *csi.CapacityRange
		// If overrideQuota is set, the API reports reportedQuotaMib as the
		// quota of the volume that it creates, instead of the quota that the
		// fake gives the volume.
		overrideQuota    bool
		reportedQuotaMib int64
		// wantQuotaMib is the quota that the volume must be created with, and
		// wantCapacityBytes the capacity that CreateVolume must report.
		wantQuotaMib      int64
		wantCapacityBytes int64
		// wantCode is the code of the error that CreateVolume must return,
		// and wantErrContains a part of its message. The driver must refuse
		// such requests without calling the API.
		wantCode        codes.Code
		wantErrContains string
	}{
		// Without a requested capacity, the fake gives the volume a quota of
		// 1024 MiB, as the API gives it the volume pool's default quota.
		{name: "no capacity range", capRange: nil, wantQuotaMib: 0, wantCapacityBytes: 1024 * util.Mb},
		{name: "empty capacity range", capRange: &csi.CapacityRange{}, wantQuotaMib: 0, wantCapacityBytes: 1024 * util.Mb},
		{name: "minimum size", capRange: &csi.CapacityRange{RequiredBytes: 100 * util.Mb}, wantQuotaMib: 100, wantCapacityBytes: 100 * util.Mb},
		{name: "whole GiB", capRange: &csi.CapacityRange{RequiredBytes: 5 * util.Gb}, wantQuotaMib: 5 * 1024, wantCapacityBytes: 5 * util.Gb},
		{name: "maximum size", capRange: &csi.CapacityRange{RequiredBytes: 100 * util.Gb}, wantQuotaMib: 100 * 1024, wantCapacityBytes: 100 * util.Gb},
		{name: "decimal size is rounded up to a whole MiB", capRange: &csi.CapacityRange{RequiredBytes: 500 * 1000 * 1000}, wantQuotaMib: 477, wantCapacityBytes: 477 * util.Mb},
		{name: "decimal 1G is rounded up to a whole MiB", capRange: &csi.CapacityRange{RequiredBytes: 1000 * 1000 * 1000}, wantQuotaMib: 954, wantCapacityBytes: 954 * util.Mb},
		{name: "a byte more than a whole MiB is rounded up", capRange: &csi.CapacityRange{RequiredBytes: 100*util.Mb + 1}, wantQuotaMib: 101, wantCapacityBytes: 101 * util.Mb},
		{name: "a size just above 99 MiB is rounded up to the minimum size", capRange: &csi.CapacityRange{RequiredBytes: 99*util.Mb + 1}, wantQuotaMib: 100, wantCapacityBytes: 100 * util.Mb},
		{name: "required capacity below the limit", capRange: &csi.CapacityRange{RequiredBytes: 5 * util.Gb, LimitBytes: 10 * util.Gb}, wantQuotaMib: 5 * 1024, wantCapacityBytes: 5 * util.Gb},
		{name: "required capacity equal to the limit", capRange: &csi.CapacityRange{RequiredBytes: 5 * util.Gb, LimitBytes: 5 * util.Gb}, wantQuotaMib: 5 * 1024, wantCapacityBytes: 5 * util.Gb},
		{name: "a size just above 99 MiB with a limit of the minimum size", capRange: &csi.CapacityRange{RequiredBytes: 99*util.Mb + 1, LimitBytes: 100 * util.Mb}, wantQuotaMib: 100, wantCapacityBytes: 100 * util.Mb},
		{name: "limit above the maximum size", capRange: &csi.CapacityRange{RequiredBytes: 5 * util.Gb, LimitBytes: util.Tb}, wantQuotaMib: 5 * 1024, wantCapacityBytes: 5 * util.Gb},
		{name: "limit only", capRange: &csi.CapacityRange{LimitBytes: 5 * util.Gb}, wantQuotaMib: 5 * 1024, wantCapacityBytes: 5 * util.Gb},
		{name: "limit only is rounded down to a whole MiB", capRange: &csi.CapacityRange{LimitBytes: 5*util.Gb + 1}, wantQuotaMib: 5 * 1024, wantCapacityBytes: 5 * util.Gb},
		{name: "limit only of the minimum size", capRange: &csi.CapacityRange{LimitBytes: 100 * util.Mb}, wantQuotaMib: 100, wantCapacityBytes: 100 * util.Mb},
		{name: "limit only of the maximum size", capRange: &csi.CapacityRange{LimitBytes: 100 * util.Gb}, wantQuotaMib: 100 * 1024, wantCapacityBytes: 100 * util.Gb},
		{name: "limit only above the maximum size gets the maximum size", capRange: &csi.CapacityRange{LimitBytes: util.Tb}, wantQuotaMib: 100 * 1024, wantCapacityBytes: 100 * util.Gb},
		{name: "limit only of the largest int64 gets the maximum size", capRange: &csi.CapacityRange{LimitBytes: math.MaxInt64}, wantQuotaMib: 100 * 1024, wantCapacityBytes: 100 * util.Gb},
		{name: "the API's quota is reported, not the requested one", capRange: &csi.CapacityRange{RequiredBytes: 1 * util.Gb}, overrideQuota: true, reportedQuotaMib: 2048, wantQuotaMib: 1024, wantCapacityBytes: 2 * util.Gb},
		{name: "API that reports no quota", capRange: &csi.CapacityRange{RequiredBytes: 5 * util.Gb}, overrideQuota: true, wantQuotaMib: 5 * 1024, wantCapacityBytes: 5 * util.Gb},
		{name: "API that reports no quota, without a requested capacity", capRange: nil, overrideQuota: true, wantQuotaMib: 0, wantCapacityBytes: 0},
		// A reported quota that isn't positive, or whose size in bytes doesn't
		// fit in an int64, is not used: the requested quota is reported.
		{name: "API that reports a negative quota", capRange: &csi.CapacityRange{RequiredBytes: 5 * util.Gb}, overrideQuota: true, reportedQuotaMib: -5, wantQuotaMib: 5 * 1024, wantCapacityBytes: 5 * util.Gb},
		{name: "API that reports a quota of 2^43 MiB", capRange: &csi.CapacityRange{RequiredBytes: 5 * util.Gb}, overrideQuota: true, reportedQuotaMib: 1 << 43, wantQuotaMib: 5 * 1024, wantCapacityBytes: 5 * util.Gb},
		{name: "API that reports the largest int64 as the quota", capRange: &csi.CapacityRange{RequiredBytes: 5 * util.Gb}, overrideQuota: true, reportedQuotaMib: math.MaxInt64, wantQuotaMib: 5 * 1024, wantCapacityBytes: 5 * util.Gb},
		{name: "API that reports the largest quota whose size in bytes fits in an int64", capRange: &csi.CapacityRange{RequiredBytes: 5 * util.Gb}, overrideQuota: true, reportedQuotaMib: math.MaxInt64 / util.Mb, wantQuotaMib: 5 * 1024, wantCapacityBytes: math.MaxInt64 / util.Mb * util.Mb},
		{name: "below the minimum size", capRange: &csi.CapacityRange{RequiredBytes: 99 * util.Mb}, wantCode: codes.OutOfRange, wantErrContains: "less than the minimum"},
		{name: "one byte", capRange: &csi.CapacityRange{RequiredBytes: 1}, wantCode: codes.OutOfRange, wantErrContains: "less than the minimum"},
		{name: "decimal 100M is below the minimum size", capRange: &csi.CapacityRange{RequiredBytes: 100 * 1000 * 1000}, wantCode: codes.OutOfRange, wantErrContains: "less than the minimum"},
		// A required capacity of 99 MiB or less is refused rather than
		// raised to the minimum size, even if the limit allows it.
		{name: "below the minimum size with a larger limit", capRange: &csi.CapacityRange{RequiredBytes: 50 * util.Mb, LimitBytes: 1 * util.Gb}, wantCode: codes.OutOfRange, wantErrContains: "less than the minimum"},
		{name: "a byte above the maximum size", capRange: &csi.CapacityRange{RequiredBytes: 100*util.Gb + 1}, wantCode: codes.OutOfRange, wantErrContains: "exceeds the maximum"},
		{name: "largest int64", capRange: &csi.CapacityRange{RequiredBytes: math.MaxInt64}, wantCode: codes.OutOfRange, wantErrContains: "exceeds the maximum"},
		{name: "limit only a byte below the minimum size", capRange: &csi.CapacityRange{LimitBytes: 100*util.Mb - 1}, wantCode: codes.OutOfRange, wantErrContains: "less than the minimum"},
		{name: "no whole MiB between the required capacity and the limit", capRange: &csi.CapacityRange{RequiredBytes: 500 * 1000 * 1000, LimitBytes: 500 * 1000 * 1000}, wantCode: codes.OutOfRange},
		{name: "no whole MiB between the required capacity and a larger limit", capRange: &csi.CapacityRange{RequiredBytes: 100*util.Mb + 1, LimitBytes: 101*util.Mb - 1}, wantCode: codes.OutOfRange},
		{name: "required capacity above the limit", capRange: &csi.CapacityRange{RequiredBytes: 10 * util.Gb, LimitBytes: 5 * util.Gb}, wantCode: codes.InvalidArgument},
		{name: "required capacity a byte above the limit", capRange: &csi.CapacityRange{RequiredBytes: 5*util.Gb + 1, LimitBytes: 5 * util.Gb}, wantCode: codes.InvalidArgument},
		{name: "required capacity above both the limit and the maximum size", capRange: &csi.CapacityRange{RequiredBytes: 200 * util.Gb, LimitBytes: 100 * util.Gb}, wantCode: codes.InvalidArgument},
		{name: "negative required capacity", capRange: &csi.CapacityRange{RequiredBytes: -1}, wantCode: codes.InvalidArgument},
		{name: "negative limit", capRange: &csi.CapacityRange{LimitBytes: -1}, wantCode: codes.InvalidArgument},
		{name: "negative limit with a required capacity", capRange: &csi.CapacityRange{RequiredBytes: 5 * util.Gb, LimitBytes: -1}, wantCode: codes.InvalidArgument},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cs := initTestController(t).(*controllerServer)
			cs.config.features.FeatureVolumePools = &FeatureVolumePools{Enabled: true}
			fs := &volumePoolQuotaService{
				Service:          cs.config.fileService,
				overrideQuota:    tc.overrideQuota,
				reportedQuotaMib: tc.reportedQuotaMib,
			}
			cs.config.fileService = fs

			resp, err := cs.CreateVolume(ctx, &csi.CreateVolumeRequest{
				Name:               "test-vol",
				Parameters:         map[string]string{paramKeyVolumePool: volumePoolPath},
				VolumeCapabilities: volumeCapabilities,
				CapacityRange:      tc.capRange,
			})
			if code := status.Code(err); code != tc.wantCode {
				t.Fatalf("CreateVolume() error = %v (code %v), want code %v", err, code, tc.wantCode)
			}
			if tc.wantCode != codes.OK {
				if !strings.Contains(err.Error(), tc.wantErrContains) {
					t.Errorf("CreateVolume() error = %v, want an error containing %q", err, tc.wantErrContains)
				}
				if len(fs.quotas) != 0 {
					t.Errorf("CreateVolumePoolVolume called with quotas %v, want no call", fs.quotas)
				}
				return
			}
			if want := []int64{tc.wantQuotaMib}; !slices.Equal(fs.quotas, want) {
				t.Errorf("CreateVolumePoolVolume called with quotas %v, want %v", fs.quotas, want)
			}
			if got := resp.GetVolume().GetCapacityBytes(); got != tc.wantCapacityBytes {
				t.Errorf("CapacityBytes = %d, want %d", got, tc.wantCapacityBytes)
			}
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
				_, err := cs.config.fileService.CreateVolumePoolVolume(ctx, "projects/test-project/locations/us-central1/volumePools/my-pool", "test-vol", 0)
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
