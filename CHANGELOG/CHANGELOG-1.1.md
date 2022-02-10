# v1.1.2 - Changelog since v1.1.1

## Changes by Kind

### Uncategorized

- Users will now see the InvalidArgument error code for the 400 googleapi errors caused by invalid arguments. ([#206](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/206), [@amacaskill](https://github.com/amacaskill))
- Users will now see the InvalidArgument error code for the 403 and 429 googleapi errors. ([#203](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/203), [@amacaskill](https://github.com/amacaskill))

## Dependencies

### Added
- github.com/benbjohnson/clock: [v1.1.0](https://github.com/benbjohnson/clock/tree/v1.1.0)
- github.com/go-kit/log: [v0.1.0](https://github.com/go-kit/log/tree/v0.1.0)
- github.com/go-task/slim-sprig: [348f09d](https://github.com/go-task/slim-sprig/tree/348f09d)
- go.uber.org/goleak: v1.1.10

### Changed
- github.com/evanphx/json-patch: [v4.5.0+incompatible → v4.11.0+incompatible](https://github.com/evanphx/json-patch/compare/v4.5.0...v4.11.0)
- github.com/go-logfmt/logfmt: [v0.4.0 → v0.5.0](https://github.com/go-logfmt/logfmt/compare/v0.4.0...v0.5.0)
- github.com/go-logr/zapr: [v0.1.1 → v0.4.0](https://github.com/go-logr/zapr/compare/v0.1.1...v0.4.0)
- github.com/imdario/mergo: [v0.3.9 → v0.3.12](https://github.com/imdario/mergo/compare/v0.3.9...v0.3.12)
- github.com/jpillora/backoff: [3050d21 → v1.0.0](https://github.com/jpillora/backoff/compare/3050d21...v1.0.0)
- github.com/json-iterator/go: [v1.1.10 → v1.1.11](https://github.com/json-iterator/go/compare/v1.1.10...v1.1.11)
- github.com/julienschmidt/httprouter: [v1.2.0 → v1.3.0](https://github.com/julienschmidt/httprouter/compare/v1.2.0...v1.3.0)
- github.com/nxadm/tail: [v1.4.5 → v1.4.8](https://github.com/nxadm/tail/compare/v1.4.5...v1.4.8)
- github.com/onsi/ginkgo: [v1.14.1 → v1.16.4](https://github.com/onsi/ginkgo/compare/v1.14.1...v1.16.4)
- github.com/onsi/gomega: [v1.10.2 → v1.15.0](https://github.com/onsi/gomega/compare/v1.10.2...v1.15.0)
- github.com/prometheus/client_golang: [v1.6.0 → v1.11.0](https://github.com/prometheus/client_golang/compare/v1.6.0...v1.11.0)
- github.com/prometheus/common: [v0.9.1 → v0.26.0](https://github.com/prometheus/common/compare/v0.9.1...v0.26.0)
- github.com/prometheus/procfs: [v0.0.11 → v0.6.0](https://github.com/prometheus/procfs/compare/v0.0.11...v0.6.0)
- go.uber.org/atomic: v1.6.0 → v1.7.0
- go.uber.org/multierr: v1.5.0 → v1.6.0
- go.uber.org/zap: v1.15.0 → v1.19.0
- golang.org/x/time: 89c76fb → 1f47c86
- gomodules.xyz/jsonpatch/v2: v2.1.0 → v2.2.0
- gopkg.in/yaml.v2: v2.3.0 → v2.4.0
- sigs.k8s.io/controller-runtime: v0.6.1 → v0.10.0

### Removed
_Nothing has changed._

# v1.1.1 - Changelog since v1.1.0

## Changes by Kind

### Feature

- enable dynamic provision with shared-vpc from service project ([#192](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/192), [@leiyiz](https://github.com/leiyiz))

### Other (Cleanup or Flake)

- document the new connect-mode and reserved-ip-range parameters ([#193](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/193), [@leiyiz](https://github.com/leiyiz))

# v1.1.0 - Changelog since v1.0.0

## Changes by Kind

### Feature

- Update golang and k8s mount utils ([#185](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/185), [@saikat-royc](https://github.com/saikat-royc))
- Use launcher.gcr.io/google/debian10 base image ([#183](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/183), [@saikat-royc](https://github.com/saikat-royc))

## Bug or Regression

- add pageToken support for ListInstances ([#182](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/182), [@leiyiz](https://github.com/leiyiz))

### Other (Cleanup or Flake)

- Update the snapshotter sidecar rbac ([#184](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/184), [@saikat-royc](https://github.com/saikat-royc))


## Dependencies

### Added
- github.com/cncf/xds/go: [fbca930](https://github.com/cncf/xds/go/tree/fbca930)
- go.opentelemetry.io/proto/otlp: v0.7.0
- golang.org/x/term: 7de9c90
- google.golang.org/grpc/cmd/protoc-gen-go-grpc: v1.1.0
- k8s.io/mount-utils: v0.22.2

### Changed
- cloud.google.com/go: v0.69.1 → v0.97.0
- github.com/antihax/optional: [ca02139 → v1.0.0](https://github.com/antihax/optional/compare/ca02139...v1.0.0)
- github.com/cncf/udpa/go: [269d4d4 → 5459f2c](https://github.com/cncf/udpa/go/compare/269d4d4...5459f2c)
- github.com/envoyproxy/go-control-plane: [v0.9.4 → 63b5d3c](https://github.com/envoyproxy/go-control-plane/compare/v0.9.4...63b5d3c)
- github.com/go-logr/logr: [v0.2.1 → v0.4.0](https://github.com/go-logr/logr/compare/v0.2.1...v0.4.0)
- github.com/golang/mock: [v1.4.4 → v1.6.0](https://github.com/golang/mock/compare/v1.4.4...v1.6.0)
- github.com/golang/protobuf: [v1.4.2 → v1.5.2](https://github.com/golang/protobuf/compare/v1.4.2...v1.5.2)
- github.com/golang/snappy: [v0.0.1 → v0.0.3](https://github.com/golang/snappy/compare/v0.0.1...v0.0.3)
- github.com/google/go-cmp: [v0.5.2 → v0.5.6](https://github.com/google/go-cmp/compare/v0.5.2...v0.5.6)
- github.com/google/martian/v3: [v3.0.0 → v3.2.1](https://github.com/google/martian/v3/compare/v3.0.0...v3.2.1)
- github.com/google/pprof: [67992a1 → 4bb14d4](https://github.com/google/pprof/compare/67992a1...4bb14d4)
- github.com/googleapis/gax-go/v2: [v2.0.5 → v2.1.1](https://github.com/googleapis/gax-go/v2/compare/v2.0.5...v2.1.1)
- github.com/grpc-ecosystem/grpc-gateway: [v1.12.2 → v1.16.0](https://github.com/grpc-ecosystem/grpc-gateway/compare/v1.12.2...v1.16.0)
- github.com/stretchr/testify: [v1.6.1 → v1.7.0](https://github.com/stretchr/testify/compare/v1.6.1...v1.7.0)
- github.com/yuin/goldmark: [v1.2.1 → v1.3.5](https://github.com/yuin/goldmark/compare/v1.2.1...v1.3.5)
- go.opencensus.io: v0.22.5 → v0.23.0
- golang.org/x/lint: 738671d → 6edffad
- golang.org/x/mod: v0.3.0 → v0.4.2
- golang.org/x/net: f585440 → 7fd8e65
- golang.org/x/oauth2: 5d25da1 → 6b3c2da
- golang.org/x/sync: 6e8e738 → 036812b
- golang.org/x/sys: c1f3e33 → d303952
- golang.org/x/text: v0.3.3 → v0.3.6
- golang.org/x/tools: 64a9e34 → v0.1.5
- google.golang.org/api: v0.35.0 → v0.59.0
- google.golang.org/appengine: v1.6.6 → v1.6.7
- google.golang.org/genproto: 03b6142 → 270636b
- google.golang.org/grpc: v1.32.0 → v1.40.0
- google.golang.org/protobuf: v1.25.0 → v1.27.1
- gopkg.in/yaml.v3: eeeca48 → 496545a
- k8s.io/klog/v2: v2.3.0 → v2.9.0
- k8s.io/utils: 6301aaf → bdf08cb

### Removed
_Nothing has changed._
