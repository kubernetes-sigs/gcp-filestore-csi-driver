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
WEBHOOKBINARY=gcp-filestore-csi-driver-webhook
$(info PULL_BASE_REF is $(PULL_BASE_REF))
$(info GIT_TAG is $(GIT_TAG))
$(info PWD is $(PWD))

# A space-separated list of image tags under which the current build is to be pushed.
# Note: For Cloud build jobs, build-image-and-push make rule is the entry point with PULL_BASE_REF initialized.
# PULL_BASE_REF is plumbed in to the docker build as a TAG, and this is used to setup GCP_FS_CSI_STAGING_VERSION.
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

WEBHOOK_STAGINGIMAGE=
ifdef GCP_FS_CSI_WEBHOOK_STAGING_IMAGE
	WEBHOOK_STAGINGIMAGE=$(GCP_FS_CSI_WEBHOOK_STAGING_IMAGE)
else
	WEBHOOK_STAGINGIMAGE=gcr.io/$(PROJECT)/gcp-filestore-csi-driver-webhook
endif
$(info WEBHOOK_STAGINGIMAGE is $(WEBHOOK_STAGINGIMAGE))

BINDIR?=bin

# This flag is used only for csi-client and windows.
# TODO: Unify VERSION with STAGINGIMAGE
ifeq ($(VERSION),)
	VERSION=latest
endif

all: image

# Build the go binary for the CSI driver webhook.
webhook:
	mkdir -p ${BINDIR}
	{                                                                                                                                                  \
	set -e ;                                                                                                                                           \
	for i in $(STAGINGVERSION) ; do                                                                                                                    \
		CGO_ENABLED=0 go build -mod=vendor -a -ldflags '-X main.version='"$${i}"' -extldflags "-static"' -o ${BINDIR}/${WEBHOOKBINARY} ./cmd/webhook/; \
		break;                                                                                                                                         \
	done ;                                                                                                                                             \
	}

# Build the docker image for the webhook.
webhook-image: init-build
		{                                                                                                                                                                \
		set -e ;                                                                                                                                                         \
		for i in $(STAGINGVERSION) ;                                                                                                                                     \
			do docker build --build-arg WEBHOOKBINARY=$(WEBHOOKBINARY) --build-arg TAG=$(STAGINGVERSION) -f ./cmd/webhook/Dockerfile -t $(WEBHOOK_STAGINGIMAGE):$${i} .; \
		done ;                                                                                                                                                           \
		}

push-webhook-image: webhook-image
	{                                       \
	set -e ;                                \
	for i in $(STAGINGVERSION) ;            \
		do docker push $(WEBHOOK_STAGINGIMAGE):$${i}; \
	done;                                   \
	}

# Build the docker image for the core CSI driver.
image: init-buildx
		{                                                                   \
		set -e ;                                                            \
		for i in $(STAGINGVERSION) ; do                                     \
			docker buildx build \
				--platform linux/amd64 \
				--build-arg STAGINGVERSION=$(STAGINGVERSION) \
				--build-arg BUILDPLATFORM=linux/amd64 \
				--build-arg TARGETPLATFORM=linux/amd64 \
				-t $(STAGINGIMAGE):$${i} . \
				--push ; \
		done ;                                                              \
		}

# Build the go binary for the CSI driver.
# STAGINGVERSION may contain multiple tags (e.g. canary, vX.Y.Z etc). Use one of the tags
# for setting the driver version variable. For convenience we are using the first value.
driver:
	mkdir -p ${BINDIR}
	{                                                                                                                                 \
	set -e ;                                                                                                                          \
	for i in $(STAGINGVERSION) ; do                                                                                                   \
		CGO_ENABLED=0 go build -mod=vendor -a -ldflags '-X main.version='"$${i}"' -extldflags "-static"' -o ${BINDIR}/${DRIVERBINARY} ./cmd/; \
		break;                                                                                                                          \
	done ;                                                                                                                            \
	}

windows: windows-local
	docker build -f test/experimental/Dockerfile --build-arg TAG=$(VERSION) -t $(IMAGE)-windows:$(VERSION) .

windows-local:
	mkdir -p bin
	GOOS=windows GOARCH=amd64 go build -ldflags "-X main.vendorVersion=${VERSION}" -o bin/gcfs-csi-driver.exe ./cmd/

build-image-and-push: image

skaffold-dev:
	skaffold dev -f deploy/skaffold/skaffold.yaml

csi-client:
	mkdir -p bin
	go build -mod=vendor -ldflags "-X main.vendorVersion=${VERSION}" -o bin/csi-client ./hack/csi_client/cmd/

csi-client-windows:
	mkdir -p bin
	GOOS=windows GOARCH=amd64 go build -ldflags "-X main.vendorVersion=${VERSION}" -o bin/csi-client.exe ./hack/csi_client/cmd/

test-k8s-integration:
	go build -mod=vendor -o bin/k8s-integration-test ./test/k8s-integration

init-buildx:
	# Ensure we use a builder that can leverage it (the default on linux will not)
	-docker buildx rm multiarch-multiplatform-builder
	docker buildx create --use --name=multiarch-multiplatform-builder
	docker run --rm --privileged multiarch/qemu-user-static --reset --credential yes --persistent yes
	# Register gcloud as a Docker credential helper.
	# Required for "docker buildx build --push".
	gcloud auth configure-docker --quiet
