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
	"net"
	"net/rpc"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/prashanthpai/sunrpc"
)

const (
	testRPCTimeout = 500 * time.Millisecond
	// Upper bound for ReleaseLock to return when the server is unresponsive.
	testReturnBound = 5 * time.Second
	testClientIP    = "10.0.0.5"
)

// listen starts a TCP listener on a random loopback port and serves each
// accepted connection with handle. The listener is closed on test cleanup.
func listen(t *testing.T, handle func(net.Conn)) (net.Listener, string) {
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
	return l, port
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

// Pmap is a fake portmapper that answers GETPORT with a fixed port.
type Pmap struct{ port uint32 }

func (p *Pmap) ProcGetPort(_ *sunrpc.PortMapping, reply *uint32) error {
	*reply = p.port
	return nil
}

// LockOps is a fake Filestore in-band lock release service.
type LockOps struct{ status uint32 }

func (l *LockOps) RELEASE_ALL_LOCKS(_ *uint32, reply *uint32) error {
	*reply = l.status
	return nil
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
// return within testReturnBound.
func releaseLockWithBound(t *testing.T, c *FileStoreRPCClient) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- c.ReleaseLock("127.0.0.1", testClientIP) }()
	select {
	case err := <-done:
		return err
	case <-time.After(testReturnBound):
		t.Fatalf("ReleaseLock did not return within %v (RPCTimeout %v)", testReturnBound, c.RPCTimeout)
		return nil
	}
}

func TestReleaseLock(t *testing.T) {
	if err := RegisterLockReleaseProcedure(); err != nil {
		t.Fatalf("RegisterLockReleaseProcedure: %v", err)
	}

	tests := []struct {
		name string
		// setup returns the portmapper port to use.
		setup   func(t *testing.T) string
		wantErr string // substring; empty means success
	}{
		{
			name: "portmapper never replies",
			setup: func(t *testing.T) string {
				_, p := listen(t, tarpitHandler)
				return p
			},
			wantErr: "i/o timeout",
		},
		{
			name: "lock release server never replies",
			setup: func(t *testing.T) string {
				_, lockPort := listen(t, tarpitHandler)
				_, p := listen(t, sunrpcHandler(t, "Pmap", &Pmap{port: mustPort(t, lockPort)}))
				return p
			},
			wantErr: "i/o timeout",
		},
		{
			name: "lock release server sends partial reply then stalls",
			setup: func(t *testing.T) string {
				_, lockPort := listen(t, partialReplyHandler)
				_, p := listen(t, sunrpcHandler(t, "Pmap", &Pmap{port: mustPort(t, lockPort)}))
				return p
			},
			wantErr: "i/o timeout",
		},
		{
			name: "program not registered with portmapper",
			setup: func(t *testing.T) string {
				_, p := listen(t, sunrpcHandler(t, "Pmap", &Pmap{port: 0}))
				return p
			},
			wantErr: "not registered with portmapper",
		},
		{
			name: "success",
			setup: func(t *testing.T) string {
				_, lockPort := listen(t, sunrpcHandler(t, "IN_BAND_PROPRIETARY_LOCK_OPS_PROG", &LockOps{status: 0}))
				_, p := listen(t, sunrpcHandler(t, "Pmap", &Pmap{port: mustPort(t, lockPort)}))
				return p
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &FileStoreRPCClient{PmapPort: tc.setup(t), RPCTimeout: testRPCTimeout}
			start := time.Now()
			err := releaseLockWithBound(t, c)
			t.Logf("ReleaseLock returned after %v: %v", time.Since(start), err)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("ReleaseLock() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("ReleaseLock() = %v, want error containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestReleaseLockInvalidIPs(t *testing.T) {
	c := &FileStoreRPCClient{}
	if err := c.ReleaseLock("not-an-ip", testClientIP); err == nil {
		t.Errorf("ReleaseLock with invalid Filestore IP: got nil error")
	}
}

func TestFileStoreRPCClientDefaults(t *testing.T) {
	c := &FileStoreRPCClient{}
	if got := c.pmapPort(); got != pmapPort {
		t.Errorf("pmapPort() = %q, want %q", got, pmapPort)
	}
	if got := c.rpcTimeout(); got != defaultRPCTimeout {
		t.Errorf("rpcTimeout() = %v, want %v", got, defaultRPCTimeout)
	}
}
