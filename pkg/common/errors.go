/*
Copyright 2024 The Kubernetes Authors.

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

package common

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TemporaryError wraps an error with the a temporary error code required by
// other services like `csi-provisioner`.
// It implements the error interface. Do not return `TemporaryError` directly
// from CSI Spec API calls, as CSI Spec API calls MUST return a
// standard gRPC status. If TemporaryErrors are returned from
// helper functions within a CSI Spec API method, make sure the outer CSI
// Spec API method returns a standard gRPC status.
type TemporaryError struct {
	err  error
	code codes.Code
}

// Unwrap extracts the original error.
func (t *TemporaryError) Unwrap() error {
	return t.err
}

// GRPCStatus extracts the underlying gRPC Status error.
// This method is necessary to fulfill the grpcstatus interface
// described in https://pkg.go.dev/google.golang.org/grpc/status#FromError.
// `FromError` is used in `CodeForError` to get existing error codes
// from status errors.
func (t *TemporaryError) GRPCStatus() *status.Status {
	if t.err == nil {
		return status.New(codes.OK, "")
	}
	return status.New(t.code, t.err.Error())
}

// NewTemporaryError constructs a new `TemporaryError` instance.
//
// This function creates a TemporaryError by wrapping the given error (`err`)
// and assigning it a specific error code (`code`).
func NewTemporaryError(code codes.Code, err error) *TemporaryError {
	return &TemporaryError{err: err, code: code}
}

// Error returns a readable representation of the TemporaryError.
func (t *TemporaryError) Error() string {
	return t.err.Error()
}
