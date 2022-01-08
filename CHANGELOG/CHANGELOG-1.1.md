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
