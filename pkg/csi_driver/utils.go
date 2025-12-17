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
	"net"
	"strconv"
	"strings"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	pbSanitizer "github.com/kubernetes-csi/csi-lib-utils/protosanitizer"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"k8s.io/klog/v2"

	file "sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/file"
)

const (
	ParamMaxIOPS            = "max_iops"
	ParamMaxIOPSPerTB       = "max_iops_per_tb"
	MaxIOPSZonal            = int64(166000)
	MaxIOPSRegional         = int64(750000)
	CapacityThresholdTiB    = 10.0
	SmallCapacityStep       = int64(100)
	LargeCapacityStep       = int64(1000)
	MinDensitySmallCapacity = int64(4000)
	MaxDensitySmallCapacity = int64(17000)
	MinDensityLargeCapacity = int64(3000)
	MaxDensityLargeCapacity = int64(7500)
	MinIOPS                 = int64(2000)
)

func NewVolumeCapabilityAccessMode(mode csi.VolumeCapability_AccessMode_Mode) *csi.VolumeCapability_AccessMode {
	return &csi.VolumeCapability_AccessMode{Mode: mode}
}

func NewControllerServiceCapability(cap csi.ControllerServiceCapability_RPC_Type) *csi.ControllerServiceCapability {
	return &csi.ControllerServiceCapability{
		Type: &csi.ControllerServiceCapability_Rpc{
			Rpc: &csi.ControllerServiceCapability_RPC{
				Type: cap,
			},
		},
	}
}

func NewNodeServiceCapability(cap csi.NodeServiceCapability_RPC_Type) *csi.NodeServiceCapability {
	return &csi.NodeServiceCapability{
		Type: &csi.NodeServiceCapability_Rpc{
			Rpc: &csi.NodeServiceCapability_RPC{
				Type: cap,
			},
		},
	}
}

func logGRPC(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	klog.V(5).Infof("GRPC call: %s, GRPC request: %+v", info.FullMethod, pbSanitizer.StripSecretsCSI03(req).String())
	resp, err := handler(ctx, req)
	if err != nil {
		klog.Errorf("GRPC call: %s, GRPC error: %v", info.FullMethod, err.Error())
	} else {
		klog.V(5).Infof("GRPC call: %s, GRPC response: %+v", info.FullMethod, resp)
	}
	return resp, err
}

// IsIpWithinRange checks if an ip address is within the given ip range.
func IsIpWithinRange(ipAddress, ipRange string) (bool, error) {
	_, ipnet, err := net.ParseCIDR(ipRange)
	if err != nil {
		return false, fmt.Errorf("failed to parse cidr range %s: %w", ipRange, err)
	}
	return ipnet.Contains(net.ParseIP(ipAddress)), nil
}

// IsCIDR verifies if the given ip range is a valid CIDR value.
func IsCIDR(ipRange string) bool {
	_, _, err := net.ParseCIDR(ipRange)
	return err == nil
}

// bytesToTiB converts bytes to TiB with floating point precision
func bytesToTiB(bytes int64) float64 {
	return float64(bytes) / (1024.0 * 1024.0 * 1024.0 * 1024.0)
}

// getStepSize returns the required step size based on capacity
func getStepSize(capacityTiB float64) int64 {
	if capacityTiB < CapacityThresholdTiB {
		return SmallCapacityStep
	}
	return LargeCapacityStep
}

// validateDensityBand validates density against tier-specific bands
func validateDensityBand(capacityTiB float64, density int64) error {
	if capacityTiB < CapacityThresholdTiB {
		if density < MinDensitySmallCapacity || density > MaxDensitySmallCapacity {
			return fmt.Errorf("for instances < %.1fTiB, density must be %d-%d", CapacityThresholdTiB, MinDensitySmallCapacity, MaxDensitySmallCapacity)
		}
	} else {
		if density < MinDensityLargeCapacity || density > MaxDensityLargeCapacity {
			return fmt.Errorf("for instances >= %.1fTiB, density must be %d-%d", CapacityThresholdTiB, MinDensityLargeCapacity, MaxDensityLargeCapacity)
		}
	}
	return nil
}

// getMaxTotalIOPS returns the maximum total IOPS based on tier
func getMaxTotalIOPS(tier string) int64 {
	if strings.ToLower(tier) == regionalTier {
		return MaxIOPSRegional
	}
	return MaxIOPSZonal
}

func validateAndBuildPerformanceConfig(params map[string]string, capacityBytes int64, tier string) (*file.PerformanceConfig, error) {
	iopsStr, hasIOPS := params[ParamMaxIOPS]
	densityStr, hasDensity := params[ParamMaxIOPSPerTB]

	// If no performance parameters specified, return early
	if !hasIOPS && !hasDensity {
		return nil, nil
	}

	// Tier validation: performance config only supported for zonal and regional tiers
	lowerTier := strings.ToLower(tier)
	supportedTiers := map[string]bool{
		zonalTier:    true,
		regionalTier: true,
	}
	if !supportedTiers[lowerTier] {
		return nil, fmt.Errorf("performance configuration is only supported for zonal and regional tier instances, got tier: %s", tier)
	}

	// Exclusivity Check
	if hasIOPS && hasDensity {
		return nil, fmt.Errorf("cannot specify both %s and %s", ParamMaxIOPS, ParamMaxIOPSPerTB)
	}

	// Fixed IOPS
	if hasIOPS {
		iops, err := strconv.ParseInt(iopsStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid %s: %v", ParamMaxIOPS, err)
		}
		if iops < MinIOPS {
			return nil, fmt.Errorf("%s must be >= %d", ParamMaxIOPS, MinIOPS)
		}
		maxIOPS := getMaxTotalIOPS(tier)
		if iops > maxIOPS {
			return nil, fmt.Errorf("%s must be <= %d for tier %s", ParamMaxIOPS, maxIOPS, tier)
		}
		// Determine step size based on capacity (small < 10TiB, large >= 10TiB)
		capacityTiB := bytesToTiB(capacityBytes)
		stepSize := getStepSize(capacityTiB)
		if iops%stepSize != 0 {
			return nil, fmt.Errorf("%s must be a multiple of %d", ParamMaxIOPS, stepSize)
		}
		return &file.PerformanceConfig{FixedIOPS: iops}, nil
	}

	// IOPS per TiB
	if hasDensity {
		density, err := strconv.ParseInt(densityStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid %s: %v", ParamMaxIOPSPerTB, err)
		}

		// Convert Bytes to TiB (float division for precision)
		capacityTiB := bytesToTiB(capacityBytes)

		// Check step size
		stepSize := getStepSize(capacityTiB)
		if density%stepSize != 0 {
			return nil, fmt.Errorf("%s must be a multiple of %d", ParamMaxIOPSPerTB, stepSize)
		}

		// Validate density band
		if err := validateDensityBand(capacityTiB, density); err != nil {
			return nil, err
		}

		// Check total IOPS limit based on tier
		totalIOPS := int64(float64(density) * capacityTiB)
		maxTotalIOPS := getMaxTotalIOPS(tier)
		if totalIOPS > maxTotalIOPS {
			return nil, fmt.Errorf("total IOPS (%d) exceeds maximum limit of %d (%s)", totalIOPS, maxTotalIOPS, tier)
		}

		return &file.PerformanceConfig{IOPSPerTB: density}, nil
	}

	return nil, nil
}
