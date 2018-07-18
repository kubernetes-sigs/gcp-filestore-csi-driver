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
	"net"
	"testing"
)

func initTestIPAllocator() *IPAllocator {
	pendingIPRanges := make(map[string]bool)
	return &IPAllocator{
		pendingIPRanges: pendingIPRanges,
	}
}

func TestParseCIDR(t *testing.T) {
	cases := []struct {
		name          string
		cidr          string
		ipExpected    string
		errorExpected bool
	}{
		{
			name:          "Valid /29 CIDR format",
			cidr:          "192.168.92.192/29",
			errorExpected: false,
		},
		{
			name:          "Invalid CIDR format",
			cidr:          "192.168.92.192",
			errorExpected: true,
		},
		{
			name:          "Invalid CIDR format with network address greater than 29 bits",
			cidr:          "192.168.92.249/30",
			errorExpected: true,
		},
		{
			name:          "Misaligned CIDR with network address less than 29 bits",
			cidr:          "192.168.92.249/28",
			errorExpected: true,
		},
		{
			name:          "Valid CIDR format with network address less than 29 bits",
			cidr:          "192.168.92.248/28",
			errorExpected: false,
		},
	}

	ipAllocator := initTestIPAllocator()
	for _, test := range cases {
		ip, ipnet, err := ipAllocator.parseCIDR(test.cidr)
		if test.errorExpected && err == nil {
			t.Errorf("error while validating cidr %s, expected error while validating, got response as valid", test.cidr)
		} else if !test.errorExpected && err != nil {
			t.Errorf("error while validating cidr %s, expected valid response, got error %s", test.cidr, err.Error())
		} else if !test.errorExpected {
			ipExpected, ipnetExpected, err := net.ParseCIDR(test.cidr)
			// If parsing fails at this point, it implies test input is invalid
			if err != nil {
				t.Errorf("invalid CIDR %s provided as test input", test.cidr)
			}
			if !ipExpected.Equal(ipExpected) {
				t.Errorf("test %q failed, expected ip %s but got %s", test.name, ipExpected.String(), ip.String())
			}
			if ipnetExpected.String() != ipnet.String() {
				t.Errorf("test %q failed, expected ipnet %s but got ipnet %s", test.name, ipnetExpected.String(), ipnet.String())
			}
		}
	}
}

func TestGetUnReservedIPRange(t *testing.T) {
	// Using IPs which are not the beginning IPs of /29 CIDRs to evaluate the edge case
	ips := [8]string{"192.168.92.3/29", "192.168.92.10/29", "192.168.92.20/29", "192.168.92.28/29"}
	cases := []struct {
		name                          string
		cidr                          string
		pendingIPRanges               map[string]bool
		cloudProviderReservedIPRanges map[string]bool
		expected                      string
		errorExpected                 bool
	}{
		{
			name:                          "0 Pending, 0 Used",
			cidr:                          "192.168.92.0/27",
			pendingIPRanges:               make(map[string]bool),
			cloudProviderReservedIPRanges: make(map[string]bool),
			expected:                      "192.168.92.0/29",
			errorExpected:                 false,
		},
		{
			name:            "0 Pending, 1 Used",
			cidr:            "192.168.92.0/27",
			pendingIPRanges: make(map[string]bool),
			cloudProviderReservedIPRanges: map[string]bool{
				ips[0]: true,
			},
			expected:      "192.168.92.8/29",
			errorExpected: false,
		},
		{
			name:            "1 Pending 0 Used",
			cidr:            "192.168.92.0/27",
			pendingIPRanges: make(map[string]bool),
			cloudProviderReservedIPRanges: map[string]bool{
				ips[0]: true,
			},
			expected:      "192.168.92.8/29",
			errorExpected: false,
		},
		{
			name: "1 Pending 1 Used",
			cidr: "192.168.92.0/27",
			pendingIPRanges: map[string]bool{
				ips[0]: true,
			},
			cloudProviderReservedIPRanges: map[string]bool{
				ips[1]: true,
			},
			expected:      "192.168.92.16/29",
			errorExpected: false,
		},
		{
			name: "2 Pending 1 Used",
			cidr: "192.168.92.0/27",
			pendingIPRanges: map[string]bool{
				ips[0]: true,
				ips[2]: true,
			},
			cloudProviderReservedIPRanges: map[string]bool{
				ips[1]: true,
			},
			expected:      "192.168.92.24/29",
			errorExpected: false,
		},
		{
			name: "Pending and used IPs out of CIDR range",
			cidr: "192.168.92.0/27",
			pendingIPRanges: map[string]bool{
				"192.168.33.33/29": true,
				"192.168.44.44/29": true,
			},
			cloudProviderReservedIPRanges: map[string]bool{
				"192.255.255.0/29":   true,
				"192.168.255.255/29": true,
			},
			expected:      "192.168.92.0/29",
			errorExpected: false,
		},
		{
			name: "Unreserved IP Range obtained with carry over to significant bytes",
			cidr: "192.168.0.0/16",
			// Using a function for this case as we reserve 32 IP ranges
			pendingIPRanges: getIPRanges("192.168.0.0/16", 32, t),
			// Reserving IP ranges from 192.168.0.0/29 to 192.168.1.248
			cloudProviderReservedIPRanges: getIPRanges("192.168.1.0/16", 32, t),
			expected:                      "192.168.2.0/29",
			errorExpected:                 false,
		},
		{
			name: "2 Pending 2 Used. Unreserved IPRange unavailable",
			cidr: "192.168.92.0/27",
			pendingIPRanges: map[string]bool{
				ips[0]: true,
				ips[2]: true,
			},
			cloudProviderReservedIPRanges: map[string]bool{
				ips[1]: true,
				ips[3]: true,
			},
			errorExpected: true,
		},
	}

	for _, test := range cases {
		ipAllocator := initTestIPAllocator()
		ipAllocator.pendingIPRanges = test.pendingIPRanges
		ipRange, err := ipAllocator.GetUnreservedIPRange(test.cidr, test.cloudProviderReservedIPRanges)
		if err != nil && !test.errorExpected {
			t.Errorf("test %q failed: got error %s, expected %s", test.name, err.Error(), test.expected)
		} else if err == nil && test.errorExpected {
			t.Errorf("test %q failed: got reserved IP range %s, expected error", test.name, ipRange)
		} else if ipRange != test.expected {
			t.Errorf("test %q failed: got reserved IP range %s, expected %s", test.name, ipRange, test.expected)
		}
	}
}

func getIPRanges(cidr string, ipRangesCount int, t *testing.T) map[string]bool {
	ip, ipnet, err := net.ParseCIDR(cidr)
	ipRangeMask := net.CIDRMask(ipRangeSize, ipV4Bits)
	i := 0
	ipRanges := make(map[string]bool)
	// Break out of the loop if
	// 1) We have the required number of IP ranges in the set
	// 2) IP range overflow occurs and IP increment is not possible
	// 3) The incremented IP range is not contained in the cidr
	for cidrIP := ip.Mask(ipRangeMask); ipnet.Contains(cidrIP) && err == nil && i < ipRangesCount; cidrIP, err = incrementIP(cidrIP, incrementStep29IPRange) {
		i++
		ipRangeString := fmt.Sprint(cidrIP.String(), "/", ipRangeSize)
		ipRanges[ipRangeString] = true
	}
	if err != nil {
		t.Fatalf(err.Error())
	} else if i != ipRangesCount {
		t.Fatalf("The required number of IP ranges %d are not available in the CIDR %s", ipRangesCount, cidr)
	}
	return ipRanges
}

func TestValidateCIDROverlap(t *testing.T) {
	cases := []struct {
		name          string
		cidr1         string
		cidr2         string
		expected      bool
		errorExpected bool
	}{
		{
			name:          "Overlapping CIDRs",
			cidr1:         "192.168.92.0/29",
			cidr2:         "192.168.92.48/26",
			expected:      true,
			errorExpected: false,
		},
		{
			name:          "Non overlapping CIDRs",
			cidr1:         "192.168.92.0/29",
			cidr2:         "192.168.22.67/26",
			expected:      false,
			errorExpected: false,
		},
		{
			name:          "Non overlapping CIDRs with same cidr size",
			cidr1:         "192.168.92.247/29",
			cidr2:         "192.168.92.248/29",
			expected:      false,
			errorExpected: false,
		},
		{
			name:          "Overlapping CIDRs with same cidr size",
			cidr1:         "192.168.92.249/29",
			cidr2:         "192.168.92.255/29",
			expected:      true,
			errorExpected: false,
		},
		{
			name:          "Invalid CIDR provided",
			cidr1:         "192.168.92.0",
			cidr2:         "192.168.22.67/26",
			errorExpected: true,
		},
	}

	for _, test := range cases {
		_, ipnet1, _ := net.ParseCIDR(test.cidr1)
		_, ipnet2, _ := net.ParseCIDR(test.cidr2)
		overlap, err := isOverlap(ipnet1, ipnet2)
		if err != nil && !test.errorExpected {
			t.Errorf("test %q failed: got error %s, expected cidr overlap between %s and %s to be %t", test.name, err.Error(), test.cidr1, test.cidr2, test.expected)
		} else if err == nil && test.errorExpected {
			t.Errorf("test %q failed: got cidr overlap value %t, expected error", test.name, overlap)
		} else if !test.errorExpected && overlap != test.expected {
			t.Errorf("test %q failed: got overlap for cidr %s and %s as %t, expected %t", test.name, test.cidr1, test.cidr2, test.expected, test.expected)
		}
	}
}

func TestIncrementIP(t *testing.T) {

	cases := []struct {
		name          string
		currentIP     string
		step          byte
		expected      string
		errorExpected bool
	}{
		{
			name:          "Valid IP increment with step size 143 without carry forward to significant bytes",
			currentIP:     "192.168.92.32",
			step:          143,
			expected:      "192.168.92.175",
			errorExpected: false,
		},
		{
			name:          "Valid increment with step size 255 and carry forward to significant bytes with maximum step size",
			currentIP:     "192.255.255.253",
			step:          255,
			expected:      "193.0.0.252",
			errorExpected: false,
		},
		{
			name:          "Valid increment with step size 8 without carry forward to significant bytes",
			currentIP:     "0.255.255.106",
			step:          8,
			expected:      "0.255.255.114",
			errorExpected: false,
		},
		{
			name:          "Valid increment with step size 8 and carry forward uptil 3rd byte",
			currentIP:     "0.255.106.255",
			step:          8,
			expected:      "0.255.107.7",
			errorExpected: false,
		},
		{
			name:          "Valid increment with step size 8 and carry forward uptil 2nd byte bytes",
			currentIP:     "255.106.255.255",
			step:          8,
			expected:      "255.107.0.7",
			errorExpected: false,
		},
		{
			name:          "Valid increment with step size 8 and carry forward uptil 1st byte",
			currentIP:     "106.255.255.255",
			step:          8,
			expected:      "107.0.0.7",
			errorExpected: false,
		},
		{
			name:          "Valid increment with step size 8 and last byte expected 255",
			currentIP:     "106.255.255.247",
			step:          8,
			expected:      "106.255.255.255",
			errorExpected: false,
		},
		{
			name:          "Valid increment with step size 8 and last byte expected 0",
			currentIP:     "106.255.255.248",
			step:          8,
			expected:      "107.0.0.0",
			errorExpected: false,
		},
		{
			name:          "Valid increment with step size 8 and last byte expected 1",
			currentIP:     "106.255.255.249",
			step:          8,
			expected:      "107.0.0.1",
			errorExpected: false,
		},
		{
			name:          "Invalid increment with step size 3",
			currentIP:     "255.255.255.253",
			step:          3,
			errorExpected: true,
		},
		{
			name:          "Invalid increment with step size 8",
			currentIP:     "255.255.255.253",
			step:          8,
			errorExpected: true,
		},
	}

	for _, test := range cases {
		currentIP := net.ParseIP(test.currentIP)
		incrementedIP, err := incrementIP(currentIP, test.step)

		if err != nil && !test.errorExpected {
			t.Errorf("test %q failed: got error %s, expected %s", test.name, err.Error(), test.expected)
		} else if err == nil && test.errorExpected {
			t.Errorf("test %q failed: got reserved IP range %s, expected error", test.name, incrementedIP.String())
		} else if !test.errorExpected && incrementedIP.String() != test.expected {
			t.Errorf("test %q failed: got incremented IP %s, expected %s", test.name, incrementedIP.String(), test.expected)
		}
	}
}
func TestCloneIP(t *testing.T) {
	originalIP := net.ParseIP("192.168.92.32")
	cloneIP := cloneIP(originalIP)
	if cloneIP.String() != originalIP.String() {
		t.Errorf("error while cloning IP %s", originalIP.String())
	}
	if &originalIP == &cloneIP {
		t.Errorf("clone function returned the original object")
	}
}
