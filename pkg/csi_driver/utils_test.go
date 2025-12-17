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
	"testing"
)

func TestValidateAndBuildPerformanceConfig_NoParams(t *testing.T) {
	tests := []struct {
		name         string
		params       map[string]string
		capacityByte int64
		tier         string
		expectNil    bool
	}{
		{
			name:         "empty params returns nil",
			params:       map[string]string{},
			capacityByte: 1024 * 1024 * 1024 * 1024, // 1 TiB
			tier:         zonalTier,
			expectNil:    true,
		},
		{
			name:         "nil params returns nil",
			params:       nil,
			capacityByte: 1024 * 1024 * 1024 * 1024,
			tier:         zonalTier,
			expectNil:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := validateAndBuildPerformanceConfig(tc.params, tc.capacityByte, tc.tier)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.expectNil && result != nil {
				t.Fatalf("expected nil result, got %+v", result)
			}
		})
	}
}

func TestValidateAndBuildPerformanceConfig_FixedIOPS(t *testing.T) {
	tests := []struct {
		name         string
		params       map[string]string
		capacityByte int64
		tier         string
		expectError  bool
		expectIOPS   int64
	}{
		{
			name: "valid fixed IOPS for zonal tier",
			params: map[string]string{
				ParamMaxIOPS: "2000",
			},
			capacityByte: 1024 * 1024 * 1024 * 1024,
			tier:         zonalTier,
			expectError:  false,
			expectIOPS:   2000,
		},
		{
			name: "valid fixed IOPS for regional tier",
			params: map[string]string{
				ParamMaxIOPS: "5000",
			},
			capacityByte: 1024 * 1024 * 1024 * 1024,
			tier:         regionalTier,
			expectError:  false,
			expectIOPS:   5000,
		},
		{
			name: "fixed IOPS below minimum",
			params: map[string]string{
				ParamMaxIOPS: "1999",
			},
			capacityByte: 1024 * 1024 * 1024 * 1024,
			tier:         zonalTier,
			expectError:  true,
		},
		{
			name: "fixed IOPS exactly at minimum",
			params: map[string]string{
				ParamMaxIOPS: "2000",
			},
			capacityByte: 1024 * 1024 * 1024 * 1024,
			tier:         zonalTier,
			expectError:  false,
			expectIOPS:   2000,
		},
		{
			name: "invalid fixed IOPS string",
			params: map[string]string{
				ParamMaxIOPS: "invalid",
			},
			capacityByte: 1024 * 1024 * 1024 * 1024,
			tier:         zonalTier,
			expectError:  true,
		},
		{
			name: "fixed IOPS with unsupported tier",
			params: map[string]string{
				ParamMaxIOPS: "3000",
			},
			capacityByte: 1024 * 1024 * 1024 * 1024,
			tier:         "enterprise",
			expectError:  true,
		},
		{
			name: "fixed IOPS case-insensitive tier",
			params: map[string]string{
				ParamMaxIOPS: "3000",
			},
			capacityByte: 1024 * 1024 * 1024 * 1024,
			tier:         "ZONAL",
			expectError:  false,
			expectIOPS:   3000,
		},
		{
			name: "fixed IOPS exceeds maximum limit for zonal tier",
			params: map[string]string{
				ParamMaxIOPS: "166001",
			},
			capacityByte: 1024 * 1024 * 1024 * 1024,
			tier:         zonalTier,
			expectError:  true,
		},
		{
			name: "fixed IOPS exactly at maximum limit for zonal tier",
			params: map[string]string{
				ParamMaxIOPS: "166000",
			},
			capacityByte: 1024 * 1024 * 1024 * 1024,
			tier:         zonalTier,
			expectError:  false,
			expectIOPS:   166000,
		},
		{
			name: "fixed IOPS exceeds maximum limit for regional tier",
			params: map[string]string{
				ParamMaxIOPS: "750001",
			},
			capacityByte: 1024 * 1024 * 1024 * 1024,
			tier:         regionalTier,
			expectError:  true,
		},
		{
			name: "fixed IOPS exactly at maximum limit for regional tier",
			params: map[string]string{
				ParamMaxIOPS: "750000",
			},
			capacityByte: 1024 * 1024 * 1024 * 1024,
			tier:         regionalTier,
			expectError:  false,
			expectIOPS:   750000,
		},
		{
			name: "fixed IOPS not multiple of step size for small instance",
			params: map[string]string{
				ParamMaxIOPS: "2050",
			},
			capacityByte: 5 * 1024 * 1024 * 1024 * 1024, // 5 TiB
			tier:         zonalTier,
			expectError:  true,
		},
		{
			name: "fixed IOPS multiple of 100 for small instance (< 10TiB)",
			params: map[string]string{
				ParamMaxIOPS: "2100",
			},
			capacityByte: 5 * 1024 * 1024 * 1024 * 1024, // 5 TiB
			tier:         zonalTier,
			expectError:  false,
			expectIOPS:   2100,
		},
		{
			name: "fixed IOPS not multiple of 1000 for large instance",
			params: map[string]string{
				ParamMaxIOPS: "5500",
			},
			capacityByte: 10 * 1024 * 1024 * 1024 * 1024, // 10 TiB
			tier:         zonalTier,
			expectError:  true,
		},
		{
			name: "fixed IOPS multiple of 1000 for large instance (>= 10TiB)",
			params: map[string]string{
				ParamMaxIOPS: "5000",
			},
			capacityByte: 10 * 1024 * 1024 * 1024 * 1024, // 10 TiB
			tier:         zonalTier,
			expectError:  false,
			expectIOPS:   5000,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := validateAndBuildPerformanceConfig(tc.params, tc.capacityByte, tc.tier)
			if (err != nil) != tc.expectError {
				t.Fatalf("expectError: %v, got error: %v", tc.expectError, err)
			}
			if !tc.expectError && result != nil {
				if result.FixedIOPS != tc.expectIOPS {
					t.Fatalf("expected FixedIOPS %d, got %d", tc.expectIOPS, result.FixedIOPS)
				}
				if result.IOPSPerTB != 0 {
					t.Fatalf("expected IOPSPerTB to be 0, got %d", result.IOPSPerTB)
				}
			}
		})
	}
}

func TestValidateAndBuildPerformanceConfig_IOPSPerTB(t *testing.T) {
	// 1 TiB = 1024 * 1024 * 1024 * 1024 bytes
	tiBByte := int64(1024 * 1024 * 1024 * 1024)

	tests := []struct {
		name          string
		params        map[string]string
		capacityByte  int64
		tier          string
		expectError   bool
		expectDensity int64
	}{
		{
			name: "valid IOPSPerTB for capacity < 10TiB (minimum)",
			params: map[string]string{
				ParamMaxIOPSPerTB: "4000",
			},
			capacityByte:  5 * tiBByte,
			tier:          zonalTier,
			expectError:   false,
			expectDensity: 4000,
		},
		{
			name: "valid IOPSPerTB for capacity < 10TiB (maximum)",
			params: map[string]string{
				ParamMaxIOPSPerTB: "17000",
			},
			capacityByte:  5 * tiBByte,
			tier:          zonalTier,
			expectError:   false,
			expectDensity: 17000,
		},
		{
			name: "valid IOPSPerTB for capacity >= 10TiB (minimum)",
			params: map[string]string{
				ParamMaxIOPSPerTB: "3000",
			},
			capacityByte:  10 * tiBByte,
			tier:          regionalTier,
			expectError:   false,
			expectDensity: 3000,
		},
		{
			name: "valid IOPSPerTB for capacity >= 10TiB (maximum)",
			params: map[string]string{
				ParamMaxIOPSPerTB: "7000",
			},
			capacityByte:  11 * tiBByte,
			tier:          regionalTier,
			expectError:   false,
			expectDensity: 7000,
		},
		{
			name: "IOPSPerTB below minimum for capacity < 10TiB",
			params: map[string]string{
				ParamMaxIOPSPerTB: "3999",
			},
			capacityByte: 5 * tiBByte,
			tier:         zonalTier,
			expectError:  true,
		},
		{
			name: "IOPSPerTB above maximum for capacity < 10TiB",
			params: map[string]string{
				ParamMaxIOPSPerTB: "17001",
			},
			capacityByte: 5 * tiBByte,
			tier:         zonalTier,
			expectError:  true,
		},
		{
			name: "IOPSPerTB below minimum for capacity >= 10TiB",
			params: map[string]string{
				ParamMaxIOPSPerTB: "2999",
			},
			capacityByte: 10 * tiBByte,
			tier:         zonalTier,
			expectError:  true,
		},
		{
			name: "IOPSPerTB above maximum for capacity >= 10TiB",
			params: map[string]string{
				ParamMaxIOPSPerTB: "7501",
			},
			capacityByte: 10 * tiBByte,
			tier:         zonalTier,
			expectError:  true,
		},
		{
			name: "invalid IOPSPerTB string",
			params: map[string]string{
				ParamMaxIOPSPerTB: "invalid",
			},
			capacityByte: 5 * tiBByte,
			tier:         zonalTier,
			expectError:  true,
		},
		{
			name: "capacity < 1TiB (100GB) with valid density",
			params: map[string]string{
				ParamMaxIOPSPerTB: "4000",
			},
			capacityByte:  100 * 1024 * 1024 * 1024, // 100 GiB
			tier:          zonalTier,
			expectError:   false,
			expectDensity: 4000,
		},
		{
			name: "IOPSPerTB with unsupported tier",
			params: map[string]string{
				ParamMaxIOPSPerTB: "5000",
			},
			capacityByte: 5 * tiBByte,
			tier:         "enterprise",
			expectError:  true,
		},
		{
			name: "IOPSPerTB not multiple of 100 for small instance",
			params: map[string]string{
				ParamMaxIOPSPerTB: "4050",
			},
			capacityByte:  5 * tiBByte,
			tier:          zonalTier,
			expectError:   true,
			expectDensity: 0,
		},
		{
			name: "IOPSPerTB multiple of 100 for small instance (< 10TiB)",
			params: map[string]string{
				ParamMaxIOPSPerTB: "4100",
			},
			capacityByte:  5 * tiBByte,
			tier:          zonalTier,
			expectError:   false,
			expectDensity: 4100,
		},
		{
			name: "IOPSPerTB not multiple of 1000 for large instance",
			params: map[string]string{
				ParamMaxIOPSPerTB: "3500",
			},
			capacityByte:  10 * tiBByte,
			tier:          zonalTier,
			expectError:   true,
			expectDensity: 0,
		},
		{
			name: "IOPSPerTB multiple of 1000 for large instance (>= 10TiB)",
			params: map[string]string{
				ParamMaxIOPSPerTB: "4000",
			},
			capacityByte:  10 * tiBByte,
			tier:          zonalTier,
			expectError:   false,
			expectDensity: 4000,
		},
		{
			name: "total IOPS exceeds zonal tier limit (166000)",
			params: map[string]string{
				ParamMaxIOPSPerTB: "17000",
			},
			capacityByte:  10 * tiBByte, // 10 * 17000 = 170000 > 166000
			tier:          zonalTier,
			expectError:   true,
			expectDensity: 0,
		},
		{
			name: "total IOPS within zonal tier limit (166000)",
			params: map[string]string{
				ParamMaxIOPSPerTB: "5000",
			},
			capacityByte:  30 * tiBByte, // 30 * 5000 = 150000 < 166000
			tier:          zonalTier,
			expectError:   false,
			expectDensity: 5000,
		},
		{
			name: "total IOPS exceeds regional tier limit (750000)",
			params: map[string]string{
				ParamMaxIOPSPerTB: "7000",
			},
			capacityByte:  120 * tiBByte, // 120 * 7000 = 840000 > 750000
			tier:          regionalTier,
			expectError:   true,
			expectDensity: 0,
		},
		{
			name: "total IOPS within regional tier limit (750000)",
			params: map[string]string{
				ParamMaxIOPSPerTB: "7000",
			},
			capacityByte:  100 * tiBByte, // 100 * 7000 = 700000 < 750000
			tier:          regionalTier,
			expectError:   false,
			expectDensity: 7000,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := validateAndBuildPerformanceConfig(tc.params, tc.capacityByte, tc.tier)
			if (err != nil) != tc.expectError {
				t.Fatalf("expectError: %v, got error: %v", tc.expectError, err)
			}
			if !tc.expectError && result != nil {
				if result.IOPSPerTB != tc.expectDensity {
					t.Fatalf("expected IOPSPerTB %d, got %d", tc.expectDensity, result.IOPSPerTB)
				}
				if result.FixedIOPS != 0 {
					t.Fatalf("expected FixedIOPS to be 0, got %d", result.FixedIOPS)
				}
			}
		})
	}
}

func TestValidateAndBuildPerformanceConfig_BothParamsError(t *testing.T) {
	tests := []struct {
		name        string
		params      map[string]string
		tier        string
		expectError bool
	}{
		{
			name: "both max_iops and max_iops_per_tb specified",
			params: map[string]string{
				ParamMaxIOPS:      "2000",
				ParamMaxIOPSPerTB: "5000",
			},
			tier:        zonalTier,
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := validateAndBuildPerformanceConfig(tc.params, 1024*1024*1024*1024, tc.tier)
			if err == nil {
				t.Fatalf("expected error for both parameters, got result: %+v", result)
			}
		})
	}
}
