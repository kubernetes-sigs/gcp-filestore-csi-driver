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

# Core Filestore CSI driver binary
DRIVERBINARY=gcp-filestore-csi-driver

# A space-separated list of image tags under which the current build is to be pushed.
# Determined dynamically.
STAGINGVERSION=
ifdef GCP_FS_CSI_STAGING_VERSION
	STAGINGVERSION=${GCP_FS_CSI_STAGING_VERSION}
else
	STAGINGVERSION=$(shell ./build/generate_image_tags.sh)
endif
$(info STAGINGVERSION is $(STAGINGVERSION))

STAGINGIMAGE=
ifdef GCP_FS_CSI_STAGING_IMAGE
	STAGINGIMAGE=$(GCP_FS_CSI_STAGING_IMAGE)
else
	STAGINGIMAGE=gcr.io/$(PROJECT)/gcp-filestore-csi-driver
endif
$(info STAGINGIMAGE is $(STAGINGIMAGE))

# This flag is used only for csi-client and windows.
# TODO: Unify VERSION with STAGINGIMAGE
ifeq ($(VERSION),)
	VERSION=latest
endif

all: image

# Build the docker image for the core CSI driver.
image:
		{                                                                   \
		set -e ;                                                            \
		for i in $(STAGINGVERSION) ;                                        \
			do docker build --build-arg DRIVERBINARY=$${DRIVERBINARY} -t $(STAGINGIMAGE):$${i} .; \
		done ;                                                              \
		}

# Build the go binary for the CSI driver.
# STAGINGVERSION may contain multiple tags (e.g. canary, vX.Y.Z etc). Use one of the tags
# for setting the driver version variable. For convenience we are using the first value.
driver:
	mkdir -p bin
	{                                                                                                                                 \
	set -e ;                                                                                                                          \
	for i in $(STAGINGVERSION) ; do                                                                                                   \
		CGO_ENABLED=0 go build -mod=vendor -a -ldflags '-X main.version='"$${i}"' -extldflags "-static"' -o bin/${DRIVERBINARY} ./cmd/; \
		break;                                                                                                                          \
	done ;                                                                                                                            \
	}

windows: windows-local
	docker build -f test/experimental/Dockerfile --build-arg TAG=$(VERSION) -t $(IMAGE)-windows:$(VERSION) .

windows-local:
	mkdir -p bin
	GOOS=windows GOARCH=amd64 go build -ldflags "-X main.vendorVersion=${VERSION}" -o bin/gcfs-csi-driver.exe ./cmd/

build-image-and-push: image
	{                                       \
	set -e ;                                \
	for i in $(STAGINGVERSION) ;            \
		do docker push $(STAGINGIMAGE):$${i}; \
	done;                                   \
	}

skaffold-dev:
	skaffold dev -f deploy/skaffold/skaffold.yaml

csi-client:
	mkdir -p bin
	go build -mod=vendor -ldflags "-X main.vendorVersion=${VERSION}" -o bin/csi-client ./hack/csi_client/cmd/

csi-client-windows:
	mkdir -p bin
	GOOS=windows GOARCH=amd64 go build -ldflags "-X main.vendorVersion=${VERSION}" -o bin/csi-client.exe ./hack/csi_client/cmd/

test-k8s-integration:
	go build -o bin/k8s-integration-test ./test/k8s-integration
