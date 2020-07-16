# Copyright 2018 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

IMAGE=gcr.io/$(PROJECT)/gcp-filestore-csi-driver

ifeq ($(VERSION),)
	VERSION=latest
endif

all: image

image: 
	docker build --build-arg TAG=$(VERSION) -t $(IMAGE):$(VERSION) .

local:	
	mkdir -p bin
	go build -mod=vendor -ldflags "-X main.vendorVersion=${VERSION}" -o bin/gcfs-csi-driver ./cmd/

windows: windows-local
	docker build -f test/experimental/Dockerfile --build-arg TAG=$(VERSION) -t $(IMAGE)-windows:$(VERSION) .
	
windows-local:
	mkdir -p bin
	GOOS=windows GOARCH=amd64 go build -ldflags "-X main.vendorVersion=${VERSION}" -o bin/gcfs-csi-driver.exe ./cmd/

push:
	docker push $(IMAGE):$(VERSION)

skaffold-dev:
	skaffold dev -f deploy/skaffold/skaffold.yaml

csi-client:
	mkdir -p bin
	go build -mod=vendor -ldflags "-X main.vendorVersion=${VERSION}" -o bin/csi-client ./hack/csi_client/cmd/

csi-client-windows:
	mkdir -p bin
	GOOS=windows GOARCH=amd64 go build -ldflags "-X main.vendorVersion=${VERSION}" -o bin/csi-client.exe ./hack/csi_client/cmd/
