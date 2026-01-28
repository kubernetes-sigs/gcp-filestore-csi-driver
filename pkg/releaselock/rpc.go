/*
Copyright 2023 The Kubernetes Authors.
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

package lockrelease

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/rpc"
	"strconv"
	"time"

	"github.com/prashanthpai/sunrpc"
	"k8s.io/klog/v2"
)

const (
	inbandLockReleaseProgramNumber   = uint32(200002)
	inbandLockReleaseProgramVersion  = uint32(1)
	inbandLockReleaseProcedureNumber = uint32(1)
	inbandLockReleaseProcedureName   = "IN_BAND_PROPRIETARY_LOCK_OPS_PROG.RELEASE_ALL_LOCKS"

	pmapProgramNumber  = uint32(100021)
	pmapProgramVersion = uint32(4)
	pmapPort           = "111"

	protocol               = "tcp"
	connectionTimeout      = 5 * time.Second
	notifyCloseChannelSize = 1
)

type releaseLockResponse struct {
	status releaseLockStatus
}

type releaseLockStatus uint32

// Register rpc procedure for lock release.
// This function will be called during lock release
// controller initialization.
func RegisterLockReleaseProcedure() error {
	procedureID := sunrpc.ProcedureID{
		ProgramNumber:   inbandLockReleaseProgramNumber,
		ProgramVersion:  inbandLockReleaseProgramVersion,
		ProcedureNumber: inbandLockReleaseProcedureNumber,
	}
	procedure := sunrpc.Procedure{
		ID:   procedureID,
		Name: inbandLockReleaseProcedureName,
	}
	if err := sunrpc.RegisterProcedure(procedure, true /* validateProcName */); err != nil {
		return fmt.Errorf("failed to register procedure %+v: %w", procedure, err)
	}
	return nil
}

// ReleaseLock calls the Filestore server to remove all advisory locks for a given GKE node IP.
// hostIP is the internal IP address of the Filestore instance.
// clientIP is the internal IP address of the GKE node.
func ReleaseLock(hostIP, clientIP string) error {
	// Check for valid IPV4 address.
	if net.ParseIP(hostIP) == nil {
		return fmt.Errorf("invalid Filestore IP address %s", hostIP)
	}
	// Get port from portmapper.
	hostAddress := fmt.Sprintf("%s:%s", hostIP, pmapPort)
	klog.Infof("Pmap getting port for host %s", hostAddress)
	port, err := sunrpc.PmapGetPort(hostAddress, pmapProgramNumber, pmapProgramVersion, sunrpc.IPProtoTCP)
	if err != nil {
		return fmt.Errorf("failed to get port for host %s: %w", hostAddress, err)
	}

	// Connect to RPC server.
	serverAddress := net.JoinHostPort(hostIP, strconv.Itoa(int(port)))
	klog.Infof("Connecting to RPC server at address %s", serverAddress)
	conn, err := net.DialTimeout(protocol, serverAddress, connectionTimeout)
	if err != nil {
		return fmt.Errorf("failed to connect to Filestore at address %s: %w", serverAddress, err)
	}

	// Get notified when server closes the connection.
	notifyClose := make(chan io.ReadWriteCloser, notifyCloseChannelSize)
	go func() {
		for rwc := range notifyClose {
			conn := rwc.(net.Conn)
			klog.Infof("Server %s disconnected", conn.RemoteAddr().String())
		}
	}()

	// Create client using sunrpc codec.
	client := sunrpc.NewClientCodec(conn, notifyClose)

	klog.Infof("Calling Filestore address %s to release all locks for GKE node %s", serverAddress, clientIP)

	ip := net.ParseIP(clientIP)
	if ip == nil {
		return fmt.Errorf("invalid GKE node IP %s", clientIP)
	}
	ipByte := ip.To4()
	if ipByte == nil {
		return fmt.Errorf("invalid GKE node IPv4 address %s", clientIP)
	}
	ipBinary := binary.BigEndian.Uint32(ipByte)

	request := rpc.Request{
		ServiceMethod: inbandLockReleaseProcedureName,
		Seq:           uint64(time.Now().UnixNano()),
	}
	klog.Infof("Sending RPC request %+v from GKE node IP %s to Filestore IP %s", request, clientIP, hostIP)
	if err := client.WriteRequest(&request, ipBinary); err != nil {
		return fmt.Errorf("failed to write RPC request %+v for GKE node IP %s Filestore IP %s, err: %w", request, clientIP, hostIP, err)
	}

	response := rpc.Response{}
	klog.Infof("Reading RPC response header for GKE node IP %s Filestore IP %s", clientIP, hostIP)
	if err := client.ReadResponseHeader(&response); err != nil {
		return fmt.Errorf("failed to read RPC response header for GKE node IP %s Filestore IP %s, err: %w", clientIP, hostIP, err)
	}

	var releaseAllLocksRes releaseLockResponse
	klog.Infof("Reading RPC response body for GKE node IP %s Filestore IP %s", clientIP, hostIP)
	if err := client.ReadResponseBody(&releaseAllLocksRes); err != nil {
		return fmt.Errorf("failed to read RPC response body for GKE node IP %s Filestore IP %s, err: %w", clientIP, hostIP, err)
	}
	if releaseAllLocksRes.status != 0 {
		return fmt.Errorf("failed to release all locks for GKE node IP %s Filestore IP %s, err: permission denied", clientIP, hostIP)
	}

	klog.Infof("Locks released for GKE node IP %s Filestore IP %s", clientIP, hostIP)
	return nil
}
