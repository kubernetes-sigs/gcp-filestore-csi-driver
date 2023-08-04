# Copyright 2022 The Kubernetes Authors.
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

ARG BUILDPLATFORM
FROM --platform=$BUILDPLATFORM golang:1.20.6 as builder

ARG TARGETPLATFORM

WORKDIR /go/src/sigs.k8s.io/gcp-filestore-csi-driver
ADD . .
RUN GOARCH=$(echo $TARGETPLATFORM | cut -f2 -d '/') make webhook BINDIR=/bin GCP_FS_CSI_STAGING_VERSION=${STAGINGVERSION}

FROM gcr.io/distroless/static
ARG WEBHOOKBINARY=gcp-filestore-csi-driver-webhook
COPY --from=builder /bin/${WEBHOOKBINARY} /${WEBHOOKBINARY}
ENTRYPOINT ["/gcp-filestore-csi-driver-webhook"]
