**Attention:**
1.3.3 is not a recommended version to use because of known issues which can cause failures in volume provisioning with ip reservation. Users are recommended to skip 1.3.3 and directly use 1.3.4

# v1.3.17 - Changelog since v1.3.16

## Changes by Kind

### Bug or Regression

- Fix backup source comparison logic for single share instances ([#572](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/572), [@k8s-infra-cherrypick-robot](https://github.com/k8s-infra-cherrypick-robot))
- Update go version to 1.20.6 to fix CVE-2023-29406 ([#578](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/578), [@k8s-infra-cherrypick-robot](https://github.com/k8s-infra-cherrypick-robot))

# v1.3.16 - Changelog since v1.3.15

## Changes by Kind

### Bug or Regression

- Update go version to 1.20.5 for CVE fixes ([#549](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/549), [@saikat-royc](https://github.com/saikat-royc))

# v1.3.15 - Changelog since v1.3.14

## Changes by Kind

### Bug or Regression

- Move to bullseye base image ([#543](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/543), [@saikat-royc](https://github.com/saikat-royc))

# v1.3.14 - Changelog since v1.3.13

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

# v1.3.13 - Changelog since v1.3.12

## Changes by Kind

### Uncategorized

- update golang version to 1.19.8 ([#485](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/485), [@saikat-royc](https://github.com/saikat-royc))

# v1.3.12 - Changelog since v1.3.11

## Changes by Kind

### Other (Cleanup)

- Update golang version to 1.19.7 ([#472](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/472), [@saikat-royc](https://github.com/saikat-royc))
- Update debian base to 1.10.0 buster ([#473](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/473), [@saikat-royc](https://github.com/saikat-royc))
- Update golang.org/x/net package to 0.7.0 ([#473](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/473), [@saikat-royc](https://github.com/saikat-royc))

# v1.3.11 - Changelog since v1.3.10

## Changes by Kind

### Feature

- Update sidecar for new access mode ReadWriteOncePod in beta k8s 1.27 ([#424](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/424), [@sunnylovestiramisu](https://github.com/sunnylovestiramisu))

### Documentation

- Improve pre-provisioning documentation ([#412](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/412), [@saikat-royc](https://github.com/saikat-royc))

### Bug or Regression

- Update golang version for Filestore container and webhook container ([#433](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/433), [@saikat-royc](https://github.com/saikat-royc))

### Other (Cleanup or Flake)

- Update sidecar to match internal versions ([#432](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/432), [@sunnylovestiramisu](https://github.com/sunnylovestiramisu))

### Uncategorized

- Improve error messaging during common retries. ([#404](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/404), [@hsadoyan](https://github.com/hsadoyan))
- Return DeadlineExceeded / Canceled or respective user error code instead of Internal error code when the create/delete filestore instance/share context times out / gets canceled or encounters a user error during polling. ([#417](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/417), [@amacaskill](https://github.com/amacaskill))

## Dependencies

### Added
- cloud.google.com/go/accessapproval: v1.5.0
- cloud.google.com/go/accesscontextmanager: v1.4.0
- cloud.google.com/go/aiplatform: v1.27.0
- cloud.google.com/go/analytics: v0.12.0
- cloud.google.com/go/apigateway: v1.4.0
- cloud.google.com/go/apigeeconnect: v1.4.0
- cloud.google.com/go/appengine: v1.5.0
- cloud.google.com/go/area120: v0.6.0
- cloud.google.com/go/artifactregistry: v1.9.0
- cloud.google.com/go/asset: v1.10.0
- cloud.google.com/go/assuredworkloads: v1.9.0
- cloud.google.com/go/automl: v1.8.0
- cloud.google.com/go/baremetalsolution: v0.4.0
- cloud.google.com/go/batch: v0.4.0
- cloud.google.com/go/beyondcorp: v0.3.0
- cloud.google.com/go/billing: v1.7.0
- cloud.google.com/go/binaryauthorization: v1.4.0
- cloud.google.com/go/certificatemanager: v1.4.0
- cloud.google.com/go/channel: v1.9.0
- cloud.google.com/go/cloudbuild: v1.4.0
- cloud.google.com/go/clouddms: v1.4.0
- cloud.google.com/go/cloudtasks: v1.8.0
- cloud.google.com/go/compute/metadata: v0.2.1
- cloud.google.com/go/contactcenterinsights: v1.4.0
- cloud.google.com/go/container: v1.7.0
- cloud.google.com/go/containeranalysis: v0.6.0
- cloud.google.com/go/datacatalog: v1.8.0
- cloud.google.com/go/dataflow: v0.7.0
- cloud.google.com/go/dataform: v0.5.0
- cloud.google.com/go/datafusion: v1.5.0
- cloud.google.com/go/datalabeling: v0.6.0
- cloud.google.com/go/dataplex: v1.4.0
- cloud.google.com/go/dataproc: v1.8.0
- cloud.google.com/go/dataqna: v0.6.0
- cloud.google.com/go/datastream: v1.5.0
- cloud.google.com/go/deploy: v1.5.0
- cloud.google.com/go/dialogflow: v1.19.0
- cloud.google.com/go/dlp: v1.7.0
- cloud.google.com/go/documentai: v1.10.0
- cloud.google.com/go/domains: v0.7.0
- cloud.google.com/go/edgecontainer: v0.2.0
- cloud.google.com/go/errorreporting: v0.3.0
- cloud.google.com/go/essentialcontacts: v1.4.0
- cloud.google.com/go/eventarc: v1.8.0
- cloud.google.com/go/filestore: v1.4.0
- cloud.google.com/go/functions: v1.9.0
- cloud.google.com/go/gaming: v1.8.0
- cloud.google.com/go/gkebackup: v0.3.0
- cloud.google.com/go/gkeconnect: v0.6.0
- cloud.google.com/go/gkehub: v0.10.0
- cloud.google.com/go/gkemulticloud: v0.4.0
- cloud.google.com/go/gsuiteaddons: v1.4.0
- cloud.google.com/go/iap: v1.5.0
- cloud.google.com/go/ids: v1.2.0
- cloud.google.com/go/iot: v1.4.0
- cloud.google.com/go/kms: v1.6.0
- cloud.google.com/go/language: v1.8.0
- cloud.google.com/go/lifesciences: v0.6.0
- cloud.google.com/go/longrunning: v0.3.0
- cloud.google.com/go/managedidentities: v1.4.0
- cloud.google.com/go/mediatranslation: v0.6.0
- cloud.google.com/go/memcache: v1.7.0
- cloud.google.com/go/metastore: v1.8.0
- cloud.google.com/go/monitoring: v1.8.0
- cloud.google.com/go/networkconnectivity: v1.7.0
- cloud.google.com/go/networkmanagement: v1.5.0
- cloud.google.com/go/networksecurity: v0.6.0
- cloud.google.com/go/notebooks: v1.5.0
- cloud.google.com/go/optimization: v1.2.0
- cloud.google.com/go/orchestration: v1.4.0
- cloud.google.com/go/orgpolicy: v1.5.0
- cloud.google.com/go/osconfig: v1.10.0
- cloud.google.com/go/oslogin: v1.7.0
- cloud.google.com/go/phishingprotection: v0.6.0
- cloud.google.com/go/policytroubleshooter: v1.4.0
- cloud.google.com/go/privatecatalog: v0.6.0
- cloud.google.com/go/pubsublite: v1.5.0
- cloud.google.com/go/recaptchaenterprise/v2: v2.5.0
- cloud.google.com/go/recommendationengine: v0.6.0
- cloud.google.com/go/recommender: v1.8.0
- cloud.google.com/go/redis: v1.10.0
- cloud.google.com/go/resourcemanager: v1.4.0
- cloud.google.com/go/resourcesettings: v1.4.0
- cloud.google.com/go/retail: v1.11.0
- cloud.google.com/go/run: v0.3.0
- cloud.google.com/go/scheduler: v1.7.0
- cloud.google.com/go/secretmanager: v1.9.0
- cloud.google.com/go/security: v1.10.0
- cloud.google.com/go/securitycenter: v1.16.0
- cloud.google.com/go/servicecontrol: v1.5.0
- cloud.google.com/go/servicedirectory: v1.7.0
- cloud.google.com/go/servicemanagement: v1.5.0
- cloud.google.com/go/serviceusage: v1.4.0
- cloud.google.com/go/shell: v1.4.0
- cloud.google.com/go/spanner: v1.41.0
- cloud.google.com/go/speech: v1.9.0
- cloud.google.com/go/storagetransfer: v1.6.0
- cloud.google.com/go/talent: v1.4.0
- cloud.google.com/go/texttospeech: v1.5.0
- cloud.google.com/go/tpu: v1.4.0
- cloud.google.com/go/trace: v1.4.0
- cloud.google.com/go/translate: v1.4.0
- cloud.google.com/go/video: v1.9.0
- cloud.google.com/go/videointelligence: v1.9.0
- cloud.google.com/go/vision/v2: v2.5.0
- cloud.google.com/go/vmmigration: v1.3.0
- cloud.google.com/go/vpcaccess: v1.5.0
- cloud.google.com/go/webrisk: v1.7.0
- cloud.google.com/go/websecurityscanner: v1.4.0
- cloud.google.com/go/workflows: v1.9.0

### Changed
- cloud.google.com/go/bigquery: v1.8.0 → v1.44.0
- cloud.google.com/go/compute: v1.7.0 → v1.15.0
- cloud.google.com/go/datastore: v1.1.0 → v1.10.0
- cloud.google.com/go/firestore: v1.1.0 → v1.9.0
- cloud.google.com/go/iam: v0.3.0 → v0.7.0
- cloud.google.com/go/logging: v1.0.0 → v1.6.1
- cloud.google.com/go/pubsub: v1.4.0 → v1.27.1
- cloud.google.com/go/storage: v1.22.1 → v1.10.0
- cloud.google.com/go: v0.102.0 → v0.107.0
- github.com/golang/snappy: [v0.0.3 → v0.0.1](https://github.com/golang/snappy/compare/v0.0.3...v0.0.1)
- github.com/google/go-cmp: [v0.5.8 → v0.5.9](https://github.com/google/go-cmp/compare/v0.5.8...v0.5.9)
- github.com/google/martian/v3: [v3.2.1 → v3.1.0](https://github.com/google/martian/v3/compare/v3.2.1...v3.1.0)
- github.com/google/pprof: [4bb14d4 → 94a9f03](https://github.com/google/pprof/compare/4bb14d4...94a9f03)
- github.com/googleapis/enterprise-certificate-proxy: [v0.1.0 → v0.2.0](https://github.com/googleapis/enterprise-certificate-proxy/compare/v0.1.0...v0.2.0)
- github.com/googleapis/gax-go/v2: [v2.4.0 → v2.7.0](https://github.com/googleapis/gax-go/v2/compare/v2.4.0...v2.7.0)
- github.com/stretchr/objx: [v0.2.0 → v0.5.0](https://github.com/stretchr/objx/compare/v0.2.0...v0.5.0)
- github.com/stretchr/testify: [v1.7.0 → v1.8.1](https://github.com/stretchr/testify/compare/v1.7.0...v1.8.1)
- go.opencensus.io: v0.23.0 → v0.24.0
- golang.org/x/mod: 9b9b3d8 → 86c51ed
- golang.org/x/net: c7608f3 → v0.5.0
- golang.org/x/oauth2: 128564f → 6fdb5e3
- golang.org/x/sync: 0de741c → v0.1.0
- golang.org/x/sys: 3c1f352 → v0.4.0
- golang.org/x/term: 065cf7b → v0.4.0
- golang.org/x/text: v0.3.7 → v0.6.0
- golang.org/x/tools: 897bd77 → v0.1.12
- golang.org/x/xerrors: 65e6541 → 04be3eb
- google.golang.org/api: v0.90.0 → v0.103.0
- google.golang.org/genproto: dd149ef → 67e5cbc
- google.golang.org/grpc: v1.48.0 → v1.51.0

### Removed
- github.com/googleapis/go-type-adapters: [v1.0.0](https://github.com/googleapis/go-type-adapters/tree/v1.0.0)
- google.golang.org/grpc/cmd/protoc-gen-go-grpc: v1.1.0

# v1.3.10 - Changelog since v1.3.9

- Update golang version ([#400](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/400), [@saikat-royc](https://github.com/saikat-royc))


# v1.3.9 - Changelog since v1.3.5

### Bug or Regression

- Strict check for filestore service endpoints ([#383](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/383), [@saikat-royc](https://github.com/saikat-royc))

# v1.3.5 - Changelog since v1.3.4

### Feature
- If multishare is enabled, the container now requires "--gke-cluster-name" flag to be set ([#372](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/372), [@leiyiz](https://github.com/leiyiz))

# v1.3.4 - Changelog since v1.3.3

### Bug or Regression

- fix basePath set to empty ([#366](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/366), [@leiyiz](https://github.com/leiyiz))

# v1.3.3 - Changelog since v1.3.2 (Bad version)

## Changes by Kind

### Bug or Regression

- Instance validation improvement when choosing eligible instances ([#337](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/337), [@tyuchn](https://github.com/tyuchn))
- Consume official v1beta1 go client for file ([#340](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/340), [@leiyiz](https://github.com/leiyiz))
- Add reserved ip range check and fail earlier if invalid ([#347](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/347), [@tyuchn](https://github.com/tyuchn))
## Dependencies

### Added
- cloud.google.com/go/compute: v1.7.0
- cloud.google.com/go/iam: v0.3.0
- github.com/googleapis/enterprise-certificate-proxy: [v0.1.0](https://github.com/googleapis/enterprise-certificate-proxy/tree/v0.1.0)
- github.com/googleapis/go-type-adapters: [v1.0.0](https://github.com/googleapis/go-type-adapters/tree/v1.0.0)

### Changed
- cloud.google.com/go/storage: v1.10.0 → v1.22.1
- cloud.google.com/go: v0.97.0 → v0.102.0
- github.com/cncf/udpa/go: [5459f2c → 04548b0](https://github.com/cncf/udpa/go/compare/5459f2c...04548b0)
- github.com/cncf/xds/go: [fbca930 → cb28da3](https://github.com/cncf/xds/go/compare/fbca930...cb28da3)
- github.com/envoyproxy/go-control-plane: [63b5d3c → 49ff273](https://github.com/envoyproxy/go-control-plane/compare/63b5d3c...49ff273)
- github.com/googleapis/gax-go/v2: [v2.1.1 → v2.4.0](https://github.com/googleapis/gax-go/v2/compare/v2.1.1...v2.4.0)
- golang.org/x/net: c690dde → c7608f3
- golang.org/x/oauth2: 622c5d5 → 128564f
- golang.org/x/sync: 036812b → 0de741c
- golang.org/x/sys: bc2c85a → 3c1f352
- golang.org/x/xerrors: 5ec99f8 → 65e6541
- google.golang.org/api: v0.59.0 → v0.90.0
- google.golang.org/genproto: 42d7afd → dd149ef
- google.golang.org/grpc: v1.40.0 → v1.48.0
- google.golang.org/protobuf: v1.28.0 → v1.28.1

# v1.3.2 - Changelog since v1.3.1

## Changes by Kind

### Bug or Regression

- Multishare ip reservation bug fix ([#341](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/341), [@tyuchn](https://github.com/tyuchn))

# v1.3.1 - Changelog since v1.3.0

### Other (Cleanup)

- Add log for list multishare instance ([#332](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/332), [@tyuchn](https://github.com/tyuchn))

# v1.3.0 - Changelog since v1.2.7

## Changes by Kind

### Bug or Regression

- Register process start time metric for core filestore driver container ([#321](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/321), [@saikat-royc](https://github.com/saikat-royc))
- Support for ARM nodes ([#325](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/325), [@mattcary](https://github.com/mattcary))

### Other (Cleanup or Flake)

- Check for GiB aligned sizes for multishare volumes CSI calls ([#322](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/322), [@saikat-royc](https://github.com/saikat-royc))
- Do not count user errors 404 and 429 errors against SLO unhappiness ([#324](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/324), [@amacaskill](https://github.com/amacaskill))
- Include REPAIRING as valid non-ready state for multishare instances ([#326](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/326), [@leiyiz](https://github.com/leiyiz))
- Choose an older Kubetest2 commit version instead of using latest and manually set timeout to 24h ([#323](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/323), [@tyuchn](https://github.com/tyuchn))

## Dependencies

### Added
_Nothing has changed._

### Changed
_Nothing has changed._

### Removed
_Nothing has changed._
