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
	"math"
	"net"
	"sync"
)

const (
	// Total number of bits in an IPV4 address
	ipV4Bits = 32
)

// IPAllocator struct consists of shared resources that are used to keep track of the CIDR IPRanges currently reserved by service instances
type IPAllocator struct {
	// pendingIPRanges set maintains the set of  IP ranges that have been reserved by the service instances but pending reservation in the cloud instances
	// The key is a IP range currently reserved by a service instance e.g(192.168.92.0/29). Value is a bool to implement map as a set
	pendingIPRanges map[string]bool

	// pendingIPRangesMutex is used to synchronize access to the pendingIPRanges set to prevent data races
	pendingIPRangesMutex sync.Mutex
}

// NewIPAllocator is the constructor to initialize the IPAllocator object
// Argument pendingIPRanges map[string]bool is a set of  IP ranges currently reserved by service instances but pending reservation in the cloud instances
func NewIPAllocator(pendingIPRanges map[string]bool) *IPAllocator {
	// Make a copy of the pending IP ranges and set it in the IPAllocator so that the caller cannot mutate this map outside the library
	pendingIPRangesCopy := make(map[string]bool)
	for pendingIPRange := range pendingIPRanges {
		pendingIPRangesCopy[pendingIPRange] = true
	}
	return &IPAllocator{
		pendingIPRanges: pendingIPRangesCopy,
	}
}

// holdIPRange adds a particular IP range in the pendingIPRanges set
// Argument ipRange string is an IPV4 range which needs put in pendingIPRanges
func (ipAllocator *IPAllocator) holdIPRange(ipRange string) {
	ipAllocator.pendingIPRanges[ipRange] = true
}

// ReleaseIPRange releases the pending IPRange
// Argument ipRange string is an IPV4 range which needs to be released
func (ipAllocator *IPAllocator) ReleaseIPRange(ipRange string) {
	ipAllocator.pendingIPRangesMutex.Lock()
	defer ipAllocator.pendingIPRangesMutex.Unlock()
	delete(ipAllocator.pendingIPRanges, ipRange)
}

// GetUnreservedIPRange returns an unreserved IP block.
// cidr: Provided cidr address in which we need to look for an unreserved IP range with specified size
// ipRangeSize: the size of the unreserved IP range we are looking for
// cloudInstancesReservedIPRanges: All the used IP ranges in the cloud instances
// All the used IP ranges in the service instances not updated in cloud instances is extracted from the pendingIPRanges list in the IPAllocator
// Finally a final reservedIPRange list is created by merging these two lists
// Potential error cases:
// 1) No IP range in the CIDR is unreserved
// 2) Parsing the CIDR resulted in an error
func (ipAllocator *IPAllocator) GetUnreservedIPRange(cidr string, ipRangeSize int, cloudInstancesReservedIPRanges map[string]bool) (string, error) {
	ip, ipnet, err := ipAllocator.parseCIDR(cidr, ipRangeSize)
	if err != nil {
		return "", err
	}
	var reservedIPRanges = make(map[string]bool)

	// The final reserved list is obtained by combining the cloudInstancesReservedIPRanges list and the pendingIPRanges list in the ipAllocator
	for cloudInstancesReservedIPRange := range cloudInstancesReservedIPRanges {
		reservedIPRanges[cloudInstancesReservedIPRange] = true
	}

	// Lock is placed here so that the pendingIPRanges list captures all the IPs pending reservation in the cloud instances
	ipAllocator.pendingIPRangesMutex.Lock()
	defer ipAllocator.pendingIPRangesMutex.Unlock()
	for reservedIPRange := range ipAllocator.pendingIPRanges {
		reservedIPRanges[reservedIPRange] = true
	}

	incrementStepIPRange := (uint32)(math.Exp2(float64(ipV4Bits - ipRangeSize)))
	for cidrIP := cloneIP(ip.Mask(ipnet.Mask)); ipnet.Contains(cidrIP) && err == nil; cidrIP, err = incrementIP(cidrIP, incrementStepIPRange) {
		overLap := false
		for reservedIPRange := range reservedIPRanges {
			_, reservedIPNet, err := net.ParseCIDR(reservedIPRange)
			if err != nil {
				return "", err
			}
			// Creating IPnet object using IP and mask
			cidrIPNet := &net.IPNet{
				IP:   cidrIP,
				Mask: net.CIDRMask(ipRangeSize, ipV4Bits),
			}

			// Find if the current IP range in the CIDR overlaps with any of the reserved IP ranges. If not, this can be returned
			overLap, err = isOverlap(cidrIPNet, reservedIPNet)

			// Error while processing ipnet
			if err != nil {
				return "", err
			}
			if overLap {
				break
			}
		}
		if !overLap {
			ipRange := fmt.Sprint(cidrIP.String(), "/", ipRangeSize)
			ipAllocator.holdIPRange(ipRange)
			return ipRange, nil
		}
	}

	// No unreserved IP range available in the entire CIDR range since we did not return
	return "", fmt.Errorf("all of the /%d IP ranges in the cidr %s are reserved", ipRangeSize, cidr)
}

// isOverlap checks if two ipnets have any overlapping IPs
func isOverlap(ipnet1 *net.IPNet, ipnet2 *net.IPNet) (bool, error) {
	if ipnet1 == nil || ipnet2 == nil {
		return true, fmt.Errorf("invalid ipnet object provided for cidr overlap check")
	}
	return ipnet1.Contains(ipnet2.IP) || ipnet2.Contains(ipnet1.IP), nil
}

// ParseCIDR function parses the CIDR and returns the ip and ipnet object if the cidr is valid
// For a CIDR to be valid it must satisfy the following properties
// 1) Network address bits must be less than 30
// 2) The IP in the CIDR must be 'aligned' i.e we must have 8 available IPs before byte overflow occurs
func (ipAllocator *IPAllocator) parseCIDR(cidr string, ipRangeSize int) (net.IP, *net.IPNet, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, nil, err
	}
	// The reserved-ipv4-cidr network size must be at least ipRangeSize
	cidrSize, _ := ipnet.Mask.Size()
	if cidrSize > ipRangeSize {
		return nil, nil, fmt.Errorf("the reserved-ipv4-cidr network size must be at least /%d", ipRangeSize)
	}

	// The IP specified in the reserved-ipv4-cidr must be aligned on the ipRangeSize network boundary
	if ip.String() != ip.Mask(net.CIDRMask(ipRangeSize, ipV4Bits)).String() {
		return nil, nil, fmt.Errorf("the IP specified in the reserved-ipv4-cidr must be aligned on the /%d network boundary", ipRangeSize)
	}
	return ip, ipnet, nil
}

// Increment the given IP value by the provided step. The step is a uint32
func incrementIP(ip net.IP, step uint32) (net.IP, error) {
	incrementedIP := cloneIP(ip)
	incrementedIP = incrementedIP.To4()

	ipValue := uint32(incrementedIP[0])<<24 + uint32(incrementedIP[1])<<16 + uint32(incrementedIP[2])<<8 + uint32(incrementedIP[3])
	newIpValue := ipValue + step
	if newIpValue < ipValue {
		return nil, fmt.Errorf("ip range overflowed while incrementing IP %s by step %d", ip.String(), step)
	}

	v3 := byte(newIpValue & 0xFF)
	v2 := byte((newIpValue >> 8) & 0xFF)
	v1 := byte((newIpValue >> 16) & 0xFF)
	v0 := byte((newIpValue >> 24) & 0xFF)
	return net.IPv4(v0, v1, v2, v3), nil
}

// Clone the provided IP and return the copy
func cloneIP(ip net.IP) net.IP {
	clone := make(net.IP, len(ip))
	copy(clone, ip)
	return clone
}
