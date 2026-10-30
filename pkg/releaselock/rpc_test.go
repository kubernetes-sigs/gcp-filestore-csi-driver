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

package lockrelease

import (
	"encoding/binary"
	"fmt"
	"net"
	"net/rpc"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prashanthpai/sunrpc"
)

const (
	// hangTimeout is used by cases where the server never (fully) replies, so
	// the test waits for the deadline to fire.
	hangTimeout = 500 * time.Millisecond
	// replyTimeout is used by cases where the server replies, so a busy CI
	// node running under -race does not hit the deadline.
	replyTimeout = 5 * time.Second
	// testReturnSlack is how long past the configured timeout ReleaseLock may
	// take to return before the test fails.
	testReturnSlack = 5 * time.Second
	testClientIP    = "10.0.0.5"
)

var testClientIPBytes = [4]byte{10, 0, 0, 5}

// listen starts a TCP listener on a random loopback port and serves each
// accepted connection with handle. The listener is closed on test cleanup.
func listen(t *testing.T, handle func(net.Conn)) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go handle(c)
		}
	}()
	_, port, _ := net.SplitHostPort(l.Addr().String())
	return port
}

// tarpitHandler accepts data but never replies.
func tarpitHandler(c net.Conn) {
	defer c.Close()
	buf := make([]byte, 4096)
	for {
		if _, err := c.Read(buf); err != nil {
			return
		}
	}
}

// partialReplyHandler reads the request, then sends only part of the
// 4-byte record marker and stalls.
func partialReplyHandler(c net.Conn) {
	defer c.Close()
	buf := make([]byte, 4096)
	if _, err := c.Read(buf); err != nil {
		return
	}
	if _, err := c.Write([]byte{0x80, 0x00}); err != nil {
		return
	}
	tarpitHandler(c)
}

// delayedHandler waits for delay before handing the connection to next.
func delayedHandler(delay time.Duration, next func(net.Conn)) func(net.Conn) {
	return func(c net.Conn) {
		time.Sleep(delay)
		next(c)
	}
}

// Pmap is a fake portmapper that answers GETPORT with a fixed port and
// records the mapping it was asked for.
type Pmap struct {
	port uint32
	mu   sync.Mutex
	got  []sunrpc.PortMapping
}

func (p *Pmap) ProcGetPort(args *sunrpc.PortMapping, reply *uint32) error {
	p.mu.Lock()
	p.got = append(p.got, *args)
	p.mu.Unlock()
	*reply = p.port
	return nil
}

func (p *Pmap) requests() []sunrpc.PortMapping {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]sunrpc.PortMapping(nil), p.got...)
}

// LockOps is a fake Filestore in-band lock release service. The argument is
// decoded as a fixed 4-byte opaque, matching the server's
// `opaque client_ip[4]`, and recorded.
type LockOps struct {
	status uint32
	mu     sync.Mutex
	got    [][4]byte
}

func (l *LockOps) RELEASE_ALL_LOCKS(args *[4]byte, reply *uint32) error {
	l.mu.Lock()
	l.got = append(l.got, *args)
	l.mu.Unlock()
	*reply = l.status
	return nil
}

func (l *LockOps) requests() [][4]byte {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([][4]byte(nil), l.got...)
}

func sunrpcHandler(t *testing.T, name string, rcvr interface{}) func(net.Conn) {
	t.Helper()
	srv := rpc.NewServer()
	if err := srv.RegisterName(name, rcvr); err != nil {
		t.Fatalf("RegisterName(%s): %v", name, err)
	}
	return func(c net.Conn) { srv.ServeCodec(sunrpc.NewServerCodec(c, nil)) }
}

func mustPort(t *testing.T, p string) uint32 {
	t.Helper()
	n, err := strconv.Atoi(p)
	if err != nil {
		t.Fatalf("bad port %q: %v", p, err)
	}
	return uint32(n)
}

// releaseLockWithBound runs ReleaseLock and fails the test if it does not
// return within the client's timeout plus testReturnSlack.
func releaseLockWithBound(t *testing.T, c *FileStoreRPCClient) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- c.ReleaseLock("127.0.0.1", testClientIP) }()
	bound := c.rpcTimeoutOrDefault() + testReturnSlack
	select {
	case err := <-done:
		return err
	case <-time.After(bound):
		t.Fatalf("ReleaseLock did not return within %v (rpcTimeout %v)", bound, c.rpcTimeout)
		return nil
	}
}

func TestReleaseLock(t *testing.T) {
	if err := RegisterLockReleaseProcedure(); err != nil {
		t.Fatalf("RegisterLockReleaseProcedure: %v", err)
	}

	tests := []struct {
		name    string
		timeout time.Duration
		// setup returns the portmapper port to use.
		setup   func(t *testing.T) string
		wantErr string // substring; empty means success
	}{
		{
			name:    "portmapper never replies",
			timeout: hangTimeout,
			setup: func(t *testing.T) string {
				return listen(t, tarpitHandler)
			},
			wantErr: "i/o timeout",
		},
		{
			name:    "lock release server never replies",
			timeout: hangTimeout,
			setup: func(t *testing.T) string {
				lockPort := listen(t, tarpitHandler)
				return listen(t, sunrpcHandler(t, "Pmap", &Pmap{port: mustPort(t, lockPort)}))
			},
			wantErr: "i/o timeout",
		},
		{
			name:    "lock release server sends partial reply then stalls",
			timeout: hangTimeout,
			setup: func(t *testing.T) string {
				lockPort := listen(t, partialReplyHandler)
				return listen(t, sunrpcHandler(t, "Pmap", &Pmap{port: mustPort(t, lockPort)}))
			},
			wantErr: "i/o timeout",
		},
		{
			name:    "program not registered with portmapper",
			timeout: replyTimeout,
			setup: func(t *testing.T) string {
				return listen(t, sunrpcHandler(t, "Pmap", &Pmap{port: 0}))
			},
			wantErr: "not registered with portmapper",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &FileStoreRPCClient{portmapperPort: tc.setup(t), rpcTimeout: tc.timeout}
			start := time.Now()
			err := releaseLockWithBound(t, c)
			t.Logf("ReleaseLock returned after %v: %v", time.Since(start), err)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("ReleaseLock() = %v, want error containing %q", err, tc.wantErr)
			}
		})
	}
}

// TestReleaseLockSuccess checks that the client asks the portmapper for the
// NLM v4 TCP port and sends the GKE node IP as 4 big-endian bytes.
func TestReleaseLockSuccess(t *testing.T) {
	if err := RegisterLockReleaseProcedure(); err != nil {
		t.Fatalf("RegisterLockReleaseProcedure: %v", err)
	}
	lockOps := &LockOps{status: 0}
	lockPort := listen(t, sunrpcHandler(t, "IN_BAND_PROPRIETARY_LOCK_OPS_PROG", lockOps))
	pmap := &Pmap{port: mustPort(t, lockPort)}
	pmapPort := listen(t, sunrpcHandler(t, "Pmap", pmap))

	c := &FileStoreRPCClient{portmapperPort: pmapPort, rpcTimeout: replyTimeout}
	if err := releaseLockWithBound(t, c); err != nil {
		t.Fatalf("ReleaseLock() = %v, want nil", err)
	}

	wantMapping := sunrpc.PortMapping{Program: 100021, Version: 4, Protocol: 6}
	if got := pmap.requests(); len(got) != 1 || got[0] != wantMapping {
		t.Errorf("portmapper requests = %+v, want [%+v]", got, wantMapping)
	}
	if got := lockOps.requests(); len(got) != 1 || got[0] != testClientIPBytes {
		t.Errorf("RELEASE_ALL_LOCKS client_ip = %v, want [%v]", got, testClientIPBytes)
	}
}

// TestReleaseLockSharedDeadline checks that one deadline covers the whole
// call: a slow portmapper reply eats into the time left for the lock release
// RPC instead of each leg getting the full timeout.
func TestReleaseLockSharedDeadline(t *testing.T) {
	if err := RegisterLockReleaseProcedure(); err != nil {
		t.Fatalf("RegisterLockReleaseProcedure: %v", err)
	}
	const (
		timeout   = 2 * time.Second
		pmapDelay = 1500 * time.Millisecond
	)
	lockPort := listen(t, tarpitHandler)
	pmapPort := listen(t, delayedHandler(pmapDelay, sunrpcHandler(t, "Pmap", &Pmap{port: mustPort(t, lockPort)})))

	c := &FileStoreRPCClient{portmapperPort: pmapPort, rpcTimeout: timeout}
	start := time.Now()
	err := releaseLockWithBound(t, c)
	elapsed := time.Since(start)
	if err == nil || !strings.Contains(err.Error(), "i/o timeout") {
		t.Fatalf("ReleaseLock() = %v, want i/o timeout", err)
	}
	// With per-leg timeouts this would take ~pmapDelay+timeout (3.5s).
	if limit := timeout + time.Second; elapsed > limit {
		t.Errorf("ReleaseLock took %v, want <= %v (one shared deadline)", elapsed, limit)
	}
}

// --- Raw wire-format checks, independent of sunrpc's procedure registry. ---

// rpcCall is the decoded header and argument bytes of an ONC RPC call.
type rpcCall struct {
	xid, msgType, rpcVers, prog, vers, proc uint32
	args                                    []byte
}

func be32(b []byte, off *int) (uint32, error) {
	if len(b) < *off+4 {
		return 0, fmt.Errorf("short record: need %d bytes, have %d", *off+4, len(b))
	}
	v := binary.BigEndian.Uint32(b[*off:])
	*off += 4
	return v, nil
}

// skipOpaqueAuth skips an opaque_auth (flavor, length, padded body).
func skipOpaqueAuth(b []byte, off *int) error {
	if _, err := be32(b, off); err != nil { // flavor
		return err
	}
	n, err := be32(b, off)
	if err != nil {
		return err
	}
	padded := int((n + 3) &^ 3)
	if len(b) < *off+padded {
		return fmt.Errorf("short opaque_auth body")
	}
	*off += padded
	return nil
}

func readCall(c net.Conn) (rpcCall, error) {
	rec, err := sunrpc.ReadFullRecord(c)
	if err != nil {
		return rpcCall{}, err
	}
	var call rpcCall
	off := 0
	for _, f := range []*uint32{&call.xid, &call.msgType, &call.rpcVers, &call.prog, &call.vers, &call.proc} {
		if *f, err = be32(rec, &off); err != nil {
			return rpcCall{}, err
		}
	}
	if err := skipOpaqueAuth(rec, &off); err != nil { // cred
		return rpcCall{}, err
	}
	if err := skipOpaqueAuth(rec, &off); err != nil { // verf
		return rpcCall{}, err
	}
	call.args = rec[off:]
	return call, nil
}

// writeAcceptedReply writes a successful ONC RPC reply carrying a single
// uint32 result.
func writeAcceptedReply(c net.Conn, xid, result uint32) error {
	words := []uint32{
		xid,
		1, // REPLY
		0, // MSG_ACCEPTED
		0, // verf flavor AUTH_NONE
		0, // verf length
		0, // SUCCESS
		result,
	}
	buf := make([]byte, 4+4*len(words))
	binary.BigEndian.PutUint32(buf, 0x80000000|uint32(4*len(words))) // last fragment
	for i, w := range words {
		binary.BigEndian.PutUint32(buf[4+4*i:], w)
	}
	_, err := c.Write(buf)
	return err
}

// rawServer reads one call, sends it on calls, and replies with result.
func rawServer(calls chan<- rpcCall, result uint32) func(net.Conn) {
	return func(c net.Conn) {
		defer c.Close()
		call, err := readCall(c)
		if err != nil {
			return
		}
		calls <- call
		_ = writeAcceptedReply(c, call.xid, result)
	}
}

func TestReleaseLockWireFormat(t *testing.T) {
	if err := RegisterLockReleaseProcedure(); err != nil {
		t.Fatalf("RegisterLockReleaseProcedure: %v", err)
	}
	lockCalls := make(chan rpcCall, 1)
	lockPort := listen(t, rawServer(lockCalls, 0 /* NLM4_GRANTED */))
	pmapCalls := make(chan rpcCall, 1)
	pmapPort := listen(t, rawServer(pmapCalls, mustPort(t, lockPort)))

	c := &FileStoreRPCClient{portmapperPort: pmapPort, rpcTimeout: replyTimeout}
	if err := releaseLockWithBound(t, c); err != nil {
		t.Fatalf("ReleaseLock() = %v, want nil", err)
	}

	pmapCall := <-pmapCalls
	if pmapCall.msgType != 0 || pmapCall.rpcVers != 2 {
		t.Errorf("GETPORT call header msg_type=%d rpcvers=%d, want CALL(0) and 2", pmapCall.msgType, pmapCall.rpcVers)
	}
	if pmapCall.prog != 100000 || pmapCall.vers != 2 || pmapCall.proc != 3 {
		t.Errorf("GETPORT call = prog %d vers %d proc %d, want PMAPPROG(100000) v2 PMAPPROC_GETPORT(3)", pmapCall.prog, pmapCall.vers, pmapCall.proc)
	}
	// struct mapping { prog, vers, prot, port }
	wantPmapArgs := make([]byte, 16)
	binary.BigEndian.PutUint32(wantPmapArgs[0:], 100021) // NLM
	binary.BigEndian.PutUint32(wantPmapArgs[4:], 4)      // NLM v4
	binary.BigEndian.PutUint32(wantPmapArgs[8:], 6)      // IPPROTO_TCP
	binary.BigEndian.PutUint32(wantPmapArgs[12:], 0)
	if string(pmapCall.args) != string(wantPmapArgs) {
		t.Errorf("GETPORT args = %x, want %x", pmapCall.args, wantPmapArgs)
	}

	lockCall := <-lockCalls
	if lockCall.prog != 200002 || lockCall.vers != 1 || lockCall.proc != 1 {
		t.Errorf("RELEASE_ALL_LOCKS call = prog %d vers %d proc %d, want 200002 v1 proc 1", lockCall.prog, lockCall.vers, lockCall.proc)
	}
	// struct release_all_locks_args { opaque client_ip[4]; }
	if string(lockCall.args) != string(testClientIPBytes[:]) {
		t.Errorf("RELEASE_ALL_LOCKS args = %x, want %x", lockCall.args, testClientIPBytes[:])
	}
}

func TestReleaseLockInvalidIPs(t *testing.T) {
	c := &FileStoreRPCClient{}
	if err := c.ReleaseLock("not-an-ip", testClientIP); err == nil {
		t.Errorf("ReleaseLock with invalid Filestore IP: got nil error")
	}
	if err := c.ReleaseLock("127.0.0.1", "not-an-ip"); err == nil {
		t.Errorf("ReleaseLock with invalid GKE node IP: got nil error")
	}
	if err := c.ReleaseLock("127.0.0.1", "::1"); err == nil {
		t.Errorf("ReleaseLock with IPv6 GKE node IP: got nil error")
	}
}

func TestFileStoreRPCClientDefaults(t *testing.T) {
	c := &FileStoreRPCClient{}
	if got := c.portmapperPortOrDefault(); got != pmapPort {
		t.Errorf("portmapperPortOrDefault() = %q, want %q", got, pmapPort)
	}
	if got := c.rpcTimeoutOrDefault(); got != defaultRPCTimeout {
		t.Errorf("rpcTimeoutOrDefault() = %v, want %v", got, defaultRPCTimeout)
	}
}

func TestRegisterGetPortProcedureLazily(t *testing.T) {
	if err := registerGetPortProcedure(); err != nil {
		t.Fatalf("registerGetPortProcedure: %v", err)
	}
	id, ok := sunrpc.GetProcedureID(portmapperGetPortProcedureName)
	want := sunrpc.ProcedureID{ProgramNumber: 100000, ProgramVersion: 2, ProcedureNumber: 3}
	if !ok || id != want {
		t.Errorf("GetProcedureID(%q) = %+v, %v; want %+v, true", portmapperGetPortProcedureName, id, ok, want)
	}
}
