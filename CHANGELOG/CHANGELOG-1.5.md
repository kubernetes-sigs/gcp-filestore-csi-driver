# v1.5.8 - Changelog since v1.5.7

## Changes by Kind

### Bug or Regression

- Update go version to 1.20.7 to fix CVE-2023-29409 CVE-2023-39533 ([#593](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/593), [@Sneha-at](https://github.com/Sneha-at))

### Uncategorized

- CMEK support now won't be checked in the CSI driver, trying to create basic or premium tier instances with cmek will result in invalid argument error from the Filestore API. high scale tier instance creation with IP reservation is now supported ([#589](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/589), [@k8s-infra-cherrypick-robot](https://github.com/k8s-infra-cherrypick-robot))

# v1.5.7 - Changelog since v1.5.6

## Changes by Kind

### Feature

- Promote CRD to v1 ([#542](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/542), [@leiyiz](https://github.com/leiyiz))
- 80 share support for stateful driver ([#559](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/559), [@leiyiz](https://github.com/leiyiz))

### Bug or Regression

- Update go version to 1.20.6 to fix CVE-2023-29406 ([#577](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/577), [@k8s-infra-cherrypick-robot](https://github.com/k8s-infra-cherrypick-robot))
- Improve error code classification from Filestore API ([#562](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/562), [@k8s-infra-cherrypick-robot](https://github.com/k8s-infra-cherrypick-robot))

# v1.5.6 - Changelog since v1.5.5

## Changes by Kind
_Nothing has changed._

# v1.5.5 - Changelog since v1.5.4

## Changes by Kind

### Bug or Regression

- Update go version to 1.20.5 for CVE fixes ([#549](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/549), [@saikat-royc](https://github.com/saikat-royc))

# v1.5.4 - Changelog since v1.5.3

## Changes by Kind

### Bug or Regression

- Fix lock release logs ([#524](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/524), [@tyuchn](https://github.com/tyuchn))
- Update debian base image to latest version for CVE fixes ([#528](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/528), [@amacaskill](https://github.com/amacaskill))
- Fix bug where err is passed to CodeForError instead of createErr ([#532](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/532), [@amacaskill](https://github.com/amacaskill))

### Uncategorized

- replace PollOpErrorCode and IsUserError with CodeForError ([#521](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/521), [@amacaskill](https://github.com/amacaskill))
- Handle CreateBackupURI errors as InvalidArgument ([#527](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/527), [@hsadoyan](https://github.com/hsadoyan))
- Handle user error which is not wrapped as googleapi.Error ([#535](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/535), [@hsadoyan](https://github.com/saikat-royc))

# v1.5.3 - Changelog since v1.5.2

## Changes by Kind

### Bug or Regression

- Fixed issue where the webhook doesn't recognize -next as an invalid label ([#504](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/504), [@leiyiz](https://github.com/leiyiz))
- Update go version to 1.20.4 ([#513](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/513), [@tyuchn](https://github.com/tyuchn))

## Dependencies

### Added
_Nothing has changed._

### Changed
_Nothing has changed._

### Removed
_Nothing has changed._

# v1.5.2 - Changelog since v1.5.1

## Changes by Kind

### Uncategorized

- bumping CRD to v1beta1 and move scope into namespaced ([#487](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/487), [@leiyiz](https://github.com/leiyiz))
- fix timestamp parsing ([#492](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/492), [@leiyiz](https://github.com/leiyiz))
- NFS lock release metrics ([#496](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/496), [@tyuchn](https://github.com/tyuchn))
- Configmap rbac improvement for NFS lock release ([#486](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/486), [@tyuchn](https://github.com/tyuchn))
- use namespaced Factory to enable namespaced role binding ([#499](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/499), [@leiyiz](https://github.com/leiyiz))

# v1.5.1 - Changelog since v1.5.0

## Changes by Kind

## Dependencies

### Added
_Nothing has changed._

### Changed
_Nothing has changed._

### Removed
_Nothing has changed._

# v1.5.0 - Changelog since v1.4.3

## Changes by Kind

### Uncategorized

- Stateful CSI call logic ([#478](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/478), [@leiyiz](https://github.com/leiyiz))
- Update golang version to 1.19.8 ([#480](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/480), [@saikat-royc](https://github.com/saikat-royc))
- Stateful CSI reconciler logic and leader election ([#465](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/465), [@leiyiz](https://github.com/leiyiz))
- Controller Expand implementation for configurable max shares ([#463](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/463), [@saikat-royc](https://github.com/saikat-royc))
- Implement configurable max shares CreateVolume path with feature gate ([#461](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/461), [@saikat-royc](https://github.com/saikat-royc))


## Dependencies

### Added
- github.com/cenkalti/backoff/v4: [v4.1.3](https://github.com/cenkalti/backoff/v4/tree/v4.1.3)
- github.com/go-logr/stdr: [v1.2.2](https://github.com/go-logr/stdr/tree/v1.2.2)
- github.com/golang-jwt/jwt/v4: [v4.2.0](https://github.com/golang-jwt/jwt/v4/tree/v4.2.0)
- github.com/golangplus/bytes: [v1.0.0](https://github.com/golangplus/bytes/tree/v1.0.0)
- github.com/golangplus/fmt: [v1.0.0](https://github.com/golangplus/fmt/tree/v1.0.0)
- github.com/grpc-ecosystem/grpc-gateway/v2: [v2.7.0](https://github.com/grpc-ecosystem/grpc-gateway/v2/tree/v2.7.0)
- github.com/modocache/gover: [b58185e](https://github.com/modocache/gover/tree/b58185e)
- go.opentelemetry.io/otel/exporters/otlp/internal/retry: v1.10.0
- go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc: v1.10.0
- go.opentelemetry.io/otel/exporters/otlp/otlptrace: v1.10.0
- google.golang.org/grpc/cmd/protoc-gen-go-grpc: v1.1.0
- k8s.io/kms: v0.26.0

### Changed
- github.com/Azure/go-autorest/autorest/adal: [v0.9.13 → v0.9.20](https://github.com/Azure/go-autorest/autorest/adal/compare/v0.9.13...v0.9.20)
- github.com/Azure/go-autorest/autorest/mocks: [v0.4.1 → v0.4.2](https://github.com/Azure/go-autorest/autorest/mocks/compare/v0.4.1...v0.4.2)
- github.com/Azure/go-autorest/autorest: [v0.11.18 → v0.11.27](https://github.com/Azure/go-autorest/autorest/compare/v0.11.18...v0.11.27)
- github.com/GoogleCloudPlatform/k8s-cloud-provider: [ea6160c → f118173](https://github.com/GoogleCloudPlatform/k8s-cloud-provider/compare/ea6160c...f118173)
- github.com/MakeNowJust/heredoc: [bb23615 → v1.0.0](https://github.com/MakeNowJust/heredoc/compare/bb23615...v1.0.0)
- github.com/antlr/antlr4/runtime/Go/antlr: [b48c857 → v1.4.10](https://github.com/antlr/antlr4/runtime/Go/antlr/compare/b48c857...v1.4.10)
- github.com/aws/aws-sdk-go: [v1.38.49 → v1.44.116](https://github.com/aws/aws-sdk-go/compare/v1.38.49...v1.44.116)
- github.com/bketelsen/crypt: [v0.0.4 → 5cbc8cc](https://github.com/bketelsen/crypt/compare/v0.0.4...5cbc8cc)
- github.com/chai2010/gettext-go: [c6fed77 → v1.0.2](https://github.com/chai2010/gettext-go/compare/c6fed77...v1.0.2)
- github.com/container-storage-interface/spec: [v1.5.0 → v1.7.0](https://github.com/container-storage-interface/spec/compare/v1.5.0...v1.7.0)
- github.com/cpuguy83/go-md2man/v2: [v2.0.1 → v2.0.2](https://github.com/cpuguy83/go-md2man/v2/compare/v2.0.1...v2.0.2)
- github.com/daviddengcn/go-colortext: [511bcaf → v1.0.0](https://github.com/daviddengcn/go-colortext/compare/511bcaf...v1.0.0)
- github.com/dnaeon/go-vcr: [v1.0.1 → v1.2.0](https://github.com/dnaeon/go-vcr/compare/v1.0.1...v1.2.0)
- github.com/emicklei/go-restful/v3: [v3.8.0 → v3.9.0](https://github.com/emicklei/go-restful/v3/compare/v3.8.0...v3.9.0)
- github.com/felixge/httpsnoop: [v1.0.1 → v1.0.3](https://github.com/felixge/httpsnoop/compare/v1.0.1...v1.0.3)
- github.com/fsnotify/fsnotify: [v1.5.4 → v1.6.0](https://github.com/fsnotify/fsnotify/compare/v1.5.4...v1.6.0)
- github.com/go-logr/zapr: [v1.2.0 → v1.2.3](https://github.com/go-logr/zapr/compare/v1.2.0...v1.2.3)
- github.com/golang/snappy: [v0.0.1 → v0.0.3](https://github.com/golang/snappy/compare/v0.0.1...v0.0.3)
- github.com/golangplus/testing: [af21d9c → v1.0.0](https://github.com/golangplus/testing/compare/af21d9c...v1.0.0)
- github.com/google/cel-go: [v0.10.1 → v0.12.5](https://github.com/google/cel-go/compare/v0.10.1...v0.12.5)
- github.com/google/martian/v3: [v3.1.0 → v3.2.1](https://github.com/google/martian/v3/compare/v3.1.0...v3.2.1)
- github.com/google/pprof: [94a9f03 → 4bb14d4](https://github.com/google/pprof/compare/94a9f03...4bb14d4)
- github.com/googleapis/gnostic: [v0.5.1 → v0.4.0](https://github.com/googleapis/gnostic/compare/v0.5.1...v0.4.0)
- github.com/inconshreveable/mousetrap: [v1.0.0 → v1.0.1](https://github.com/inconshreveable/mousetrap/compare/v1.0.0...v1.0.1)
- github.com/kubernetes-csi/csi-lib-utils: [v0.8.1 → v0.13.0](https://github.com/kubernetes-csi/csi-lib-utils/compare/v0.8.1...v0.13.0)
- github.com/magiconair/properties: [v1.8.5 → v1.8.1](https://github.com/magiconair/properties/compare/v1.8.5...v1.8.1)
- github.com/matttproud/golang_protobuf_extensions: [c182aff → v1.0.2](https://github.com/matttproud/golang_protobuf_extensions/compare/c182aff...v1.0.2)
- github.com/moby/sys/mountinfo: [v0.6.0 → v0.6.2](https://github.com/moby/sys/mountinfo/compare/v0.6.0...v0.6.2)
- github.com/moby/term: [3f7ff69 → 39b0c02](https://github.com/moby/term/compare/3f7ff69...39b0c02)
- github.com/onsi/ginkgo/v2: [v2.0.0 → v2.4.0](https://github.com/onsi/ginkgo/v2/compare/v2.0.0...v2.4.0)
- github.com/onsi/gomega: [v1.18.1 → v1.23.0](https://github.com/onsi/gomega/compare/v1.18.1...v1.23.0)
- github.com/pelletier/go-toml: [v1.9.3 → v1.8.0](https://github.com/pelletier/go-toml/compare/v1.9.3...v1.8.0)
- github.com/pquerna/cachecontrol: [0dec1b3 → v0.1.0](https://github.com/pquerna/cachecontrol/compare/0dec1b3...v0.1.0)
- github.com/prometheus/client_golang: [v1.12.2 → v1.14.0](https://github.com/prometheus/client_golang/compare/v1.12.2...v1.14.0)
- github.com/prometheus/client_model: [v0.2.0 → v0.3.0](https://github.com/prometheus/client_model/compare/v0.2.0...v0.3.0)
- github.com/prometheus/common: [v0.34.0 → v0.37.0](https://github.com/prometheus/common/compare/v0.34.0...v0.37.0)
- github.com/prometheus/procfs: [v0.7.3 → v0.8.0](https://github.com/prometheus/procfs/compare/v0.7.3...v0.8.0)
- github.com/spf13/afero: [v1.6.0 → v1.2.2](https://github.com/spf13/afero/compare/v1.6.0...v1.2.2)
- github.com/spf13/cobra: [v1.4.0 → v1.6.0](https://github.com/spf13/cobra/compare/v1.4.0...v1.6.0)
- github.com/spf13/viper: [v1.8.1 → v1.7.0](https://github.com/spf13/viper/compare/v1.8.1...v1.7.0)
- github.com/xlab/treeprint: [a009c39 → v1.1.0](https://github.com/xlab/treeprint/compare/a009c39...v1.1.0)
- github.com/yuin/goldmark: [v1.4.1 → v1.4.13](https://github.com/yuin/goldmark/compare/v1.4.1...v1.4.13)
- go.etcd.io/etcd/api/v3: v3.5.1 → v3.5.5
- go.etcd.io/etcd/client/pkg/v3: v3.5.1 → v3.5.5
- go.etcd.io/etcd/client/v2: v2.305.0 → v2.305.5
- go.etcd.io/etcd/client/v3: v3.5.1 → v3.5.5
- go.etcd.io/etcd/pkg/v3: v3.5.0 → v3.5.5
- go.etcd.io/etcd/raft/v3: v3.5.0 → v3.5.5
- go.etcd.io/etcd/server/v3: v3.5.0 → v3.5.5
- go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc: v0.20.0 → v0.35.0
- go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp: v0.20.0 → v0.35.0
- go.opentelemetry.io/otel/metric: v0.20.0 → v0.31.0
- go.opentelemetry.io/otel/sdk: v0.20.0 → v1.10.0
- go.opentelemetry.io/otel/trace: v0.20.0 → v1.10.0
- go.opentelemetry.io/otel: v0.20.0 → v1.10.0
- go.opentelemetry.io/proto/otlp: v0.7.0 → v0.19.0
- go.uber.org/goleak: v1.1.12 → v1.2.0
- golang.org/x/crypto: 8634188 → v0.1.0
- golang.org/x/mod: 86c51ed → v0.6.0
- golang.org/x/tools: v0.1.12 → v0.2.0
- gopkg.in/ini.v1: v1.62.0 → v1.56.0
- k8s.io/api: v0.24.1 → v0.26.0
- k8s.io/apiextensions-apiserver: v0.24.1 → v0.26.0
- k8s.io/apimachinery: v0.24.1 → v0.26.0
- k8s.io/apiserver: v0.24.1 → v0.26.0
- k8s.io/cli-runtime: v0.24.1 → v0.26.0
- k8s.io/client-go: v0.24.1 → v0.26.0
- k8s.io/cloud-provider: v0.24.1 → v0.26.0
- k8s.io/cluster-bootstrap: v0.24.1 → v0.26.0
- k8s.io/code-generator: v0.24.1 → v0.26.0
- k8s.io/component-base: v0.24.1 → v0.26.0
- k8s.io/component-helpers: v0.24.1 → v0.26.0
- k8s.io/controller-manager: v0.24.1 → v0.26.0
- k8s.io/cri-api: v0.24.1 → v0.26.0
- k8s.io/csi-translation-lib: v0.24.1 → v0.26.0
- k8s.io/gengo: c02415c → c0856e2
- k8s.io/klog/v2: v2.60.1 → v2.80.1
- k8s.io/kube-aggregator: v0.24.1 → v0.26.0
- k8s.io/kube-controller-manager: v0.24.1 → v0.26.0
- k8s.io/kube-openapi: 31174f5 → 172d655
- k8s.io/kube-proxy: v0.24.1 → v0.26.0
- k8s.io/kube-scheduler: v0.24.1 → v0.26.0
- k8s.io/kubectl: v0.24.1 → v0.26.0
- k8s.io/kubelet: v0.24.1 → v0.26.0
- k8s.io/legacy-cloud-providers: v0.24.1 → v0.26.0
- k8s.io/metrics: v0.24.1 → v0.26.0
- k8s.io/mount-utils: v0.24.1 → v0.26.0
- k8s.io/pod-security-admission: v0.24.1 → v0.26.0
- k8s.io/sample-apiserver: v0.24.1 → v0.26.0
- k8s.io/utils: 3a6ce19 → 1a15be2
- sigs.k8s.io/apiserver-network-proxy/konnectivity-client: v0.0.30 → v0.0.33
- sigs.k8s.io/json: 227cbc7 → f223a00
- sigs.k8s.io/kustomize/api: v0.11.4 → v0.12.1
- sigs.k8s.io/kustomize/cmd/config: v0.10.6 → v0.10.9
- sigs.k8s.io/kustomize/kustomize/v4: v4.5.4 → v4.5.7
- sigs.k8s.io/kustomize/kyaml: v0.13.6 → v0.13.9
- sigs.k8s.io/structured-merge-diff/v4: v4.2.1 → v4.2.3

### Removed
- github.com/google/cel-spec: [v0.6.0](https://github.com/google/cel-spec/tree/v0.6.0)
- github.com/gophercloud/gophercloud: [v0.1.0](https://github.com/gophercloud/gophercloud/tree/v0.1.0)
- github.com/kr/fs: [v0.1.0](https://github.com/kr/fs/tree/v0.1.0)
- github.com/pkg/sftp: [v1.10.1](https://github.com/pkg/sftp/tree/v1.10.1)
- go.opentelemetry.io/contrib: v0.20.0
- go.opentelemetry.io/otel/exporters/otlp: v0.20.0
- go.opentelemetry.io/otel/sdk/export/metric: v0.20.0
- go.opentelemetry.io/otel/sdk/metric: v0.20.0
