<<<<<<< HEAD:vendor/github.com/go-openapi/swag/file.go
// Copyright 2015 go-swagger maintainers
//
=======
// Copyright 2022 The Prometheus Authors
>>>>>>> Update k8s.io/kubernetes to fix GO-2024-2746:vendor/github.com/prometheus/procfs/cpuinfo_loong64.go
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

<<<<<<< HEAD:vendor/github.com/go-openapi/swag/file.go
package swag

import "mime/multipart"

// File represents an uploaded file.
type File struct {
	Data   multipart.File
	Header *multipart.FileHeader
}

// Read bytes from the file
func (f *File) Read(p []byte) (n int, err error) {
	return f.Data.Read(p)
}

// Close the file
func (f *File) Close() error {
	return f.Data.Close()
}
=======
//go:build linux
// +build linux

package procfs

var parseCPUInfo = parseCPUInfoLoong
>>>>>>> Update k8s.io/kubernetes to fix GO-2024-2746:vendor/github.com/prometheus/procfs/cpuinfo_loong64.go
