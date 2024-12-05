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

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	pbSanitizer "github.com/kubernetes-csi/csi-lib-utils/protosanitizer"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"k8s.io/klog/v2"
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
