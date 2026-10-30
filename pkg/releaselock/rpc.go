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
	"net"
	"net/rpc"
	"strconv"
	"sync"
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

	// Portmapper (rpcbind) program used to look up the NLM port.
	portmapperProgramNumber          = uint32(100000)
	portmapperProgramVersion         = uint32(2)
	portmapperGetPortProcedureNumber = uint32(3)
	portmapperGetPortProcedureName   = "Pmap.ProcGetPort"

	protocol               = "tcp"
	connectionTimeout      = 5 * time.Second
	notifyCloseChannelSize = 1

	// defaultRPCTimeout bounds the whole ReleaseLock call (portmapper lookup
	// plus the lock release RPC, including dials) so that an unresponsive
	// Filestore server cannot block the caller forever. The server-side
	// RELEASE_ALL_LOCKS handler takes a fixed ~2.1s.
	defaultRPCTimeout = 30 * time.Second
	// getPortTimeout bounds the portmapper GETPORT lookup, a trivial call,
	// within the overall ReleaseLock deadline.
	getPortTimeout = 5 * time.Second
)

type FileStoreRPCClient struct {
	// portmapperPort overrides the portmapper port on the Filestore server.
	// Empty means pmapPort ("111"). Only set by tests.
	portmapperPort string
	// rpcTimeout overrides defaultRPCTimeout. Zero means the default. Only set
	// by tests.
	rpcTimeout time.Duration
}

func (c *FileStoreRPCClient) portmapperPortOrDefault() string {
	if c.portmapperPort != "" {
		return c.portmapperPort
	}
	return pmapPort
}

func (c *FileStoreRPCClient) rpcTimeoutOrDefault() time.Duration {
	if c.rpcTimeout > 0 {
		return c.rpcTimeout
	}
	return defaultRPCTimeout
}

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

var (
	registerGetPortOnce sync.Once
	registerGetPortErr  error
)

// registerGetPortProcedure registers the portmapper GETPORT procedure with
// sunrpc's registry, once per process. getPort issues the GETPORT call itself
// (instead of sunrpc.PmapGetPort, which has no timeouts), and sunrpc only
// registers its portmapper procedures lazily from inside its own Pmap*
// helpers, which we no longer call. RegisterProcedure overwrites, so this is
// safe even if sunrpc has also registered it.
func registerGetPortProcedure() error {
	registerGetPortOnce.Do(func() {
		procedure := sunrpc.Procedure{
			ID: sunrpc.ProcedureID{
				ProgramNumber:   portmapperProgramNumber,
				ProgramVersion:  portmapperProgramVersion,
				ProcedureNumber: portmapperGetPortProcedureNumber,
			},
			Name: portmapperGetPortProcedureName,
		}
		if err := sunrpc.RegisterProcedure(procedure, true /* validateProcName */); err != nil {
			registerGetPortErr = fmt.Errorf("failed to register procedure %+v: %w", procedure, err)
		}
	})
	return registerGetPortErr
}

// getPort queries the portmapper on hostIP for the NLM (program
// pmapProgramNumber, version pmapProgramVersion) TCP port.
// Unlike sunrpc.PmapGetPort, the dial and the RPC exchange are bounded by
// getPortTimeout and by the caller's deadline, whichever comes first, so an
// unresponsive server cannot block the caller forever.
func (c *FileStoreRPCClient) getPort(hostIP string, deadline time.Time) (uint32, error) {
	if err := registerGetPortProcedure(); err != nil {
		return 0, err
	}
	if d := time.Now().Add(getPortTimeout); d.Before(deadline) {
		deadline = d
	}

	hostAddress := net.JoinHostPort(hostIP, c.portmapperPortOrDefault())
	dialer := net.Dialer{Timeout: connectionTimeout, Deadline: deadline}
	conn, err := dialer.Dial(protocol, hostAddress)
	if err != nil {
		return 0, fmt.Errorf("failed to connect to portmapper at %s: %w", hostAddress, err)
	}
	// Closing the rpc.Client closes conn and stops the client's reader goroutine.
	client := rpc.NewClientWithCodec(sunrpc.NewClientCodec(conn, nil))
	defer client.Close()

	if err := conn.SetDeadline(deadline); err != nil {
		return 0, fmt.Errorf("failed to set deadline on portmapper connection %s: %w", hostAddress, err)
	}

	mapping := &sunrpc.PortMapping{
		Program:  pmapProgramNumber,
		Version:  pmapProgramVersion,
		Protocol: uint32(sunrpc.IPProtoTCP),
	}
	var port uint32
	if err := client.Call(portmapperGetPortProcedureName, mapping, &port); err != nil {
		return 0, fmt.Errorf("portmapper GETPORT call to %s failed: %w", hostAddress, err)
	}
	if port == 0 {
		return 0, fmt.Errorf("program %d version %d is not registered with portmapper at %s", pmapProgramNumber, pmapProgramVersion, hostAddress)
	}
	return port, nil
}

// ReleaseLock calls the Filestore server to remove all advisory locks for a given GKE node IP.
// hostIP is the internal IP address of the Filestore instance.
// clientIP is the internal IP address of the GKE node.
// The whole call, including the portmapper lookup, is bounded by a single
// deadline (defaultRPCTimeout unless overridden).
func (c *FileStoreRPCClient) ReleaseLock(hostIP, clientIP string) error {
	// Check for valid IPV4 address.
	if net.ParseIP(hostIP) == nil {
		return fmt.Errorf("invalid Filestore IP address %s", hostIP)
	}
	ip := net.ParseIP(clientIP)
	if ip == nil {
		return fmt.Errorf("invalid GKE node IP %s", clientIP)
	}
	ipByte := ip.To4()
	if ipByte == nil {
		return fmt.Errorf("invalid GKE node IPv4 address %s", clientIP)
	}
	ipBinary := binary.BigEndian.Uint32(ipByte)

	deadline := time.Now().Add(c.rpcTimeoutOrDefault())

	// Get port from portmapper.
	hostAddress := net.JoinHostPort(hostIP, c.portmapperPortOrDefault())
	klog.Infof("Pmap getting port for host %s", hostAddress)
	port, err := c.getPort(hostIP, deadline)
	if err != nil {
		return fmt.Errorf("failed to get port for host %s: %w", hostAddress, err)
	}

	// Connect to RPC server.
	serverAddress := net.JoinHostPort(hostIP, strconv.Itoa(int(port)))
	klog.Infof("Connecting to RPC server at address %s", serverAddress)
	dialer := net.Dialer{Timeout: connectionTimeout, Deadline: deadline}
	conn, err := dialer.Dial(protocol, serverAddress)
	if err != nil {
		return fmt.Errorf("failed to connect to Filestore at address %s: %w", serverAddress, err)
	}

	// Ensure the network connection is closed on all return paths to avoid
	// leaking OS file descriptors and goroutines inside the codec.
	defer conn.Close()

	// Bound the request/response exchange by the same overall deadline so an
	// unresponsive server cannot block the lock release controller's workers
	// indefinitely.
	if err := conn.SetDeadline(deadline); err != nil {
		return fmt.Errorf("failed to set deadline on connection to Filestore at address %s: %w", serverAddress, err)
	}

	// Create client using sunrpc codec. We don't provide a notify channel
	// because the codec is nil-safe and will not attempt to send
	// notifications if the channel is nil. Passing nil avoids creating a
	// per-call channel and eliminates any risk of leaking goroutines or
	// unread notifications.
	client := sunrpc.NewClientCodec(conn, nil)

	klog.Infof("Calling Filestore address %s to release all locks for GKE node %s", serverAddress, clientIP)

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
