# v1.6.13 - Changelog since v1.6.12

## Changes by Kind

### Uncategorized

- Add NfsExportOptions parsing behind a disabled flag ([#779](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/779), [@hsadoyan](https://github.com/hsadoyan))
- Change debian base image from bullseye to bookworm to fix: CVE-2023-39804, CVE-2023-47038, CVE-2022-48303. ([#801](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/801), [@k8s-infra-cherrypick-robot](https://github.com/k8s-infra-cherrypick-robot))
- Fix issue where errors were reported as Info for multishare operations ([#769](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/769), [@hsadoyan](https://github.com/hsadoyan))

## Dependencies

### Added
_Nothing has changed._

### Changed
_Nothing has changed._

### Removed
_Nothing has changed._


# v1.6.11 - Changelog since v1.6.10

## Changes by Kind

### Other (Cleanup or Flake)

- Update golang.org/x/crypto to v0.17.0 to fix CVE-2023-48795 ([#756](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/756), [@sunnylovestiramisu](https://github.com/sunnylovestiramisu))

## Dependencies

### Added
_Nothing has changed._

### Changed
- golang.org/x/crypto: v0.14.0 → v0.17.0
- golang.org/x/sys: v0.13.0 → v0.15.0
- golang.org/x/term: v0.13.0 → v0.15.0
- golang.org/x/text: v0.13.0 → v0.14.0

### Removed
_Nothing has changed._


# v1.6.10 - Changelog since v1.6.9

## Changes by Kind

### Other (Cleanup or Flake)

- Update webhook go builder to 1.20.12 ([#745](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/745), [@msau42](https://github.com/msau42))

## Dependencies

### Added
_Nothing has changed._

### Changed
_Nothing has changed._

### Removed
_Nothing has changed._

# v1.6.9 - Changelog since v1.6.8

## Changes by Kind

### Other (Cleanup or Flake)

- Update golang builder to 1.20.12 ([#741](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/741), [@msau42](https://github.com/msau42))

## Dependencies

### Added
_Nothing has changed._

### Changed
_Nothing has changed._

### Removed
_Nothing has changed._


# v1.6.8 - Changelog since v1.6.5

## Changes by Kind

### Bug or Regression

- Bump Golang Builder version to 1.20.11 ([#709](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/709), [@uriel-guzman](https://github.com/uriel-guzman))
- Bump google.golang.org/grpc from v1.57.0 to v1.57.1 to fix CVE-2023-44487. ([#710](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/710), [@songjiaxun](https://github.com/songjiaxun))
- CVE fixes: CVE-2023-44487, CVE-2023-39323, CVE-2023-3978 ([#659](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/659), [@dannawang0221](https://github.com/dannawang0221))

## Dependencies

### Added
_Nothing has changed._

### Changed
- golang.org/x/crypto: v0.12.0 → v0.14.0
- golang.org/x/net: v0.14.0 → v0.17.0
- golang.org/x/sys: v0.11.0 → v0.13.0
- golang.org/x/term: v0.11.0 → v0.13.0
- golang.org/x/text: v0.12.0 → v0.13.0
- google.golang.org/grpc: v1.57.0 → v1.57.1

### Removed
_Nothing has changed._
tform: v1.27.0 → v1.37.0
- cloud.google.com/go/analytics: v0.12.0 → v0.19.0
- cloud.google.com/go/apigateway: v1.4.0 → v1.5.0
- cloud.google.com/go/apigeeconnect: v1.4.0 → v1.5.0
- cloud.google.com/go/appengine: v1.5.0 → v1.7.1
- cloud.google.com/go/area120: v0.6.0 → v0.7.1
- cloud.google.com/go/artifactregistry: v1.9.0 → v1.13.0
- cloud.google.com/go/asset: v1.10.0 → v1.13.0
- cloud.google.com/go/assuredworkloads: v1.9.0 → v1.10.0
- cloud.google.com/go/automl: v1.8.0 → v1.12.0
- cloud.google.com/go/baremetalsolution: v0.4.0 → v0.5.0
- cloud.google.com/go/batch: v0.4.0 → v0.7.0
- cloud.google.com/go/beyondcorp: v0.3.0 → v0.5.0
- cloud.google.com/go/bigquery: v1.44.0 → v1.50.0
- cloud.google.com/go/billing: v1.7.0 → v1.13.0
- cloud.google.com/go/binaryauthorization: v1.4.0 → v1.5.0
- cloud.google.com/go/certificatemanager: v1.4.0 → v1.6.0
- cloud.google.com/go/channel: v1.9.0 → v1.12.0
- cloud.google.com/go/cloudbuild: v1.4.0 → v1.9.0
- cloud.google.com/go/clouddms: v1.4.0 → v1.5.0
- cloud.google.com/go/cloudtasks: v1.8.0 → v1.10.0
- cloud.google.com/go/compute/metadata: v0.2.1 → v0.2.3
- cloud.google.com/go/compute: v1.15.0 → v1.19.1
- cloud.google.com/go/contactcenterinsights: v1.4.0 → v1.6.0
- cloud.google.com/go/container: v1.7.0 → v1.15.0
- cloud.google.com/go/containeranalysis: v0.6.0 → v0.9.0
- cloud.google.com/go/datacatalog: v1.8.0 → v1.13.0
- cloud.google.com/go/dataflow: v0.7.0 → v0.8.0
- cloud.google.com/go/dataform: v0.5.0 → v0.7.0
- cloud.google.com/go/datafusion: v1.5.0 → v1.6.0
- cloud.google.com/go/datalabeling: v0.6.0 → v0.7.0
- cloud.google.com/go/dataplex: v1.4.0 → v1.6.0
- cloud.google.com/go/dataproc: v1.8.0 → v1.12.0
- cloud.google.com/go/dataqna: v0.6.0 → v0.7.0
- cloud.google.com/go/datastore: v1.10.0 → v1.11.0
- cloud.google.com/go/datastream: v1.5.0 → v1.7.0
- cloud.google.com/go/deploy: v1.5.0 → v1.8.0
- cloud.google.com/go/dialogflow: v1.19.0 → v1.32.0
- cloud.google.com/go/dlp: v1.7.0 → v1.9.0
- cloud.google.com/go/documentai: v1.10.0 → v1.18.0
- cloud.google.com/go/domains: v0.7.0 → v0.8.0
- cloud.google.com/go/edgecontainer: v0.2.0 → v1.0.0
- cloud.google.com/go/essentialcontacts: v1.4.0 → v1.5.0
- cloud.google.com/go/eventarc: v1.8.0 → v1.11.0
- cloud.google.com/go/filestore: v1.4.0 → v1.6.0
- cloud.google.com/go/functions: v1.9.0 → v1.13.0
- cloud.google.com/go/gaming: v1.8.0 → v1.9.0
- cloud.google.com/go/gkebackup: v0.3.0 → v0.4.0
- cloud.google.com/go/gkeconnect: v0.6.0 → v0.7.0
- cloud.google.com/go/gkehub: v0.10.0 → v0.12.0
- cloud.google.com/go/gkemulticloud: v0.4.0 → v0.5.0
- cloud.google.com/go/gsuiteaddons: v1.4.0 → v1.5.0
- cloud.google.com/go/iam: v0.7.0 → v0.13.0
- cloud.google.com/go/iap: v1.5.0 → v1.7.1
- cloud.google.com/go/ids: v1.2.0 → v1.3.0
- cloud.google.com/go/iot: v1.4.0 → v1.6.0
- cloud.google.com/go/kms: v1.6.0 → v1.10.1
- cloud.google.com/go/language: v1.8.0 → v1.9.0
- cloud.google.com/go/lifesciences: v0.6.0 → v0.8.0
- cloud.google.com/go/logging: v1.6.1 → v1.7.0
- cloud.google.com/go/longrunning: v0.3.0 → v0.4.1
- cloud.google.com/go/managedidentities: v1.4.0 → v1.5.0
- cloud.google.com/go/mediatranslation: v0.6.0 → v0.7.0
- cloud.google.com/go/memcache: v1.7.0 → v1.9.0
- cloud.google.com/go/metastore: v1.8.0 → v1.10.0
- cloud.google.com/go/monitoring: v1.8.0 → v1.13.0
- cloud.google.com/go/networkconnectivity: v1.7.0 → v1.11.0
- cloud.google.com/go/networkmanagement: v1.5.0 → v1.6.0
- cloud.google.com/go/networksecurity: v0.6.0 → v0.8.0
- cloud.google.com/go/notebooks: v1.5.0 → v1.8.0
- cloud.google.com/go/optimization: v1.2.0 → v1.3.1
- cloud.google.com/go/orchestration: v1.4.0 → v1.6.0
- cloud.google.com/go/orgpolicy: v1.5.0 → v1.10.0
- cloud.google.com/go/osconfig: v1.10.0 → v1.11.0
- cloud.google.com/go/oslogin: v1.7.0 → v1.9.0
- cloud.google.com/go/phishingprotection: v0.6.0 → v0.7.0
- cloud.google.com/go/policytroubleshooter: v1.4.0 → v1.6.0
- cloud.google.com/go/privatecatalog: v0.6.0 → v0.8.0
- cloud.google.com/go/pubsub: v1.27.1 → v1.30.0
- cloud.google.com/go/pubsublite: v1.5.0 → v1.7.0
- cloud.google.com/go/recaptchaenterprise/v2: v2.5.0 → v2.7.0
- cloud.google.com/go/recommendationengine: v0.6.0 → v0.7.0
- cloud.google.com/go/recommender: v1.8.0 → v1.9.0
- cloud.google.com/go/redis: v1.10.0 → v1.11.0
- cloud.google.com/go/resourcemanager: v1.4.0 → v1.7.0
- cloud.google.com/go/resourcesettings: v1.4.0 → v1.5.0
- cloud.google.com/go/retail: v1.11.0 → v1.12.0
- cloud.google.com/go/run: v0.3.0 → v0.9.0
- cloud.google.com/go/scheduler: v1.7.0 → v1.9.0
- cloud.google.com/go/secretmanager: v1.9.0 → v1.10.0
- cloud.google.com/go/security: v1.10.0 → v1.13.0
- cloud.google.com/go/securitycenter: v1.16.0 → v1.19.0
- cloud.google.com/go/servicecontrol: v1.5.0 → v1.11.1
- cloud.google.com/go/servicedirectory: v1.7.0 → v1.9.0
- cloud.google.com/go/servicemanagement: v1.5.0 → v1.8.0
- cloud.google.com/go/serviceusage: v1.4.0 → v1.6.0
- cloud.google.com/go/shell: v1.4.0 → v1.6.0
- cloud.google.com/go/spanner: v1.41.0 → v1.45.0
- cloud.google.com/go/speech: v1.9.0 → v1.15.0
- cloud.google.com/go/storagetransfer: v1.6.0 → v1.8.0
- cloud.google.com/go/talent: v1.4.0 → v1.5.0
- cloud.google.com/go/texttospeech: v1.5.0 → v1.6.0
- cloud.google.com/go/tpu: v1.4.0 → v1.5.0
- cloud.google.com/go/trace: v1.4.0 → v1.9.0
- cloud.google.com/go/translate: v1.4.0 → v1.7.0
- cloud.google.com/go/video: v1.9.0 → v1.15.0
- cloud.google.com/go/videointelligence: v1.9.0 → v1.10.0
- cloud.google.com/go/vision/v2: v2.5.0 → v2.7.0
- cloud.google.com/go/vmmigration: v1.3.0 → v1.6.0
- cloud.google.com/go/vpcaccess: v1.5.0 → v1.6.0
- cloud.google.com/go/webrisk: v1.7.0 → v1.8.0
- cloud.google.com/go/websecurityscanner: v1.4.0 → v1.5.0
- cloud.google.com/go/workflows: v1.9.0 → v1.10.0
- cloud.google.com/go: v0.107.0 → v0.110.0
- github.com/census-instrumentation/opencensus-proto: [v0.2.1 → v0.4.1](https://github.com/census-instrumentation/opencensus-proto/compare/v0.2.1...v0.4.1)
- github.com/cespare/xxhash/v2: [v2.1.2 → v2.2.0](https://github.com/cespare/xxhash/v2/compare/v2.1.2...v2.2.0)
- github.com/cncf/udpa/go: [04548b0 → c52dc94](https://github.com/cncf/udpa/go/compare/04548b0...c52dc94)
- github.com/cncf/xds/go: [cb28da3 → e9ce688](https://github.com/cncf/xds/go/compare/cb28da3...e9ce688)
- github.com/envoyproxy/go-control-plane: [49ff273 → 9239064](https://github.com/envoyproxy/go-control-plane/compare/49ff273...9239064)
- github.com/envoyproxy/protoc-gen-validate: [v0.1.0 → v0.10.1](https://github.com/envoyproxy/protoc-gen-validate/compare/v0.1.0...v0.10.1)
- github.com/golang/glog: [v1.0.0 → v1.1.0](https://github.com/golang/glog/compare/v1.0.0...v1.1.0)
- github.com/golang/protobuf: [v1.5.2 → v1.5.3](https://github.com/golang/protobuf/compare/v1.5.2...v1.5.3)
- github.com/googleapis/enterprise-certificate-proxy: [v0.2.0 → v0.2.3](https://github.com/googleapis/enterprise-certificate-proxy/compare/v0.2.0...v0.2.3)
- github.com/googleapis/gax-go/v2: [v2.7.0 → v2.7.1](https://github.com/googleapis/gax-go/v2/compare/v2.7.0...v2.7.1)
- golang.org/x/crypto: v0.1.0 → v0.14.0
- golang.org/x/mod: v0.6.0 → v0.8.0
- golang.org/x/net: v0.7.0 → v0.17.0
- golang.org/x/oauth2: 6fdb5e3 → v0.7.0
- golang.org/x/sys: v0.5.0 → v0.13.0
- golang.org/x/term: v0.5.0 → v0.13.0
- golang.org/x/text: v0.7.0 → v0.13.0
- golang.org/x/tools: v0.2.0 → v0.6.0
- golang.org/x/xerrors: 04be3eb → 5ec99f8
- google.golang.org/api: v0.103.0 → v0.114.0
- google.golang.org/genproto: 67e5cbc → daa745c
- google.golang.org/grpc: v1.51.0 → v1.56.3
- google.golang.org/protobuf: v1.28.1 → v1.30.0

### Removed
_Nothing has changed._
_Nothing has changed._

# v1.6.5 - Changelog since v1.6.2
## Changes by Kind

### Feature

- Disable multishare backups until hard quota enforcement issues are resolved. ([#604](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/604), [@hsadoyan](https://github.com/hsadoyan))

### Uncategorized

- Bump go version to 1.20.8 ([#607](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/607))
- Remove ARG BUILDPLATFORM from Dockerfile ([#615](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/615))
- Make pkgdir match k8s_e2e dir ([#622](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/622))
- Bump webhook go version to 1.20.8 ([#634](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/634))

## Dependencies

### Added
- github.com/google/go-pkcs11: [v0.2.0](https://github.com/google/go-pkcs11/tree/v0.2.0)
- github.com/google/s2a-go: [v0.1.5](https://github.com/google/s2a-go/tree/v0.1.5)
- google.golang.org/genproto/googleapis/api: f966b18
- google.golang.org/genproto/googleapis/bytestream: 1744710
- google.golang.org/genproto/googleapis/rpc: 6bfd019

### Changed
- cloud.google.com/go/bigquery: v1.44.0 → v1.8.0
- cloud.google.com/go/compute/metadata: v0.2.1 → v0.2.3
- cloud.google.com/go/compute: v1.15.0 → v1.23.0
- cloud.google.com/go/datastore: v1.10.0 → v1.1.0
- cloud.google.com/go/firestore: v1.9.0 → v1.1.0
- cloud.google.com/go/logging: v1.6.1 → v1.0.0
- cloud.google.com/go/pubsub: v1.27.1 → v1.4.0
- cloud.google.com/go: v0.107.0 → v0.110.2
- github.com/census-instrumentation/opencensus-proto: [v0.2.1 → v0.4.1](https://github.com/census-instrumentation/opencensus-proto/compare/v0.2.1...v0.4.1)
- github.com/cespare/xxhash/v2: [v2.1.2 → v2.2.0](https://github.com/cespare/xxhash/v2/compare/v2.1.2...v2.2.0)
- github.com/cncf/udpa/go: [04548b0 → c52dc94](https://github.com/cncf/udpa/go/compare/04548b0...c52dc94)
- github.com/cncf/xds/go: [cb28da3 → e9ce688](https://github.com/cncf/xds/go/compare/cb28da3...e9ce688)
- github.com/envoyproxy/go-control-plane: [49ff273 → 9239064](https://github.com/envoyproxy/go-control-plane/compare/49ff273...9239064)
- github.com/envoyproxy/protoc-gen-validate: [v0.1.0 → v0.10.1](https://github.com/envoyproxy/protoc-gen-validate/compare/v0.1.0...v0.10.1)
- github.com/golang/glog: [v1.0.0 → v1.1.0](https://github.com/golang/glog/compare/v1.0.0...v1.1.0)
- github.com/golang/protobuf: [v1.5.2 → v1.5.3](https://github.com/golang/protobuf/compare/v1.5.2...v1.5.3)
- github.com/googleapis/enterprise-certificate-proxy: [v0.2.0 → v0.2.5](https://github.com/googleapis/enterprise-certificate-proxy/compare/v0.2.0...v0.2.5)
- github.com/googleapis/gax-go/v2: [v2.7.0 → v2.12.0](https://github.com/googleapis/gax-go/v2/compare/v2.7.0...v2.12.0)
- golang.org/x/crypto: v0.1.0 → v0.12.0
- golang.org/x/mod: v0.6.0 → v0.8.0
- golang.org/x/net: v0.7.0 → v0.14.0
- golang.org/x/oauth2: 6fdb5e3 → v0.11.0
- golang.org/x/sync: v0.1.0 → v0.3.0
- golang.org/x/sys: v0.5.0 → v0.11.0
- golang.org/x/term: v0.5.0 → v0.11.0
- golang.org/x/text: v0.7.0 → v0.12.0
- golang.org/x/tools: v0.2.0 → v0.6.0
- golang.org/x/xerrors: 04be3eb → 5ec99f8
- google.golang.org/api: v0.103.0 → v0.138.0
- google.golang.org/genproto: 67e5cbc → f966b18
- google.golang.org/grpc: v1.51.0 → v1.57.0
- google.golang.org/protobuf: v1.28.1 → v1.31.0

### Removed
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
- cloud.google.com/go/iam: v0.7.0
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

# v1.6.2 - Changelog since v1.6.1

## Changes by Kind

### Bug or Regression

- Update go version to 1.20.7 to fix CVE-2023-29409 CVE-2023-39533 ([#592](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/592), [@Sneha-at](https://github.com/Sneha-at))

### Uncategorized

- Now supports tier "zonal" with large band. small band is actively blocked ([#588](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/588), [@k8s-infra-cherrypick-robot](https://github.com/k8s-infra-cherrypick-robot))

# v1.6.1 - Changelog since v1.6.0

## Changes by Kind

### Bug or Regression

- Update go version to 1.20.6 to fix CVE-2023-29406 ([#576](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/576), [@k8s-infra-cherrypick-robot](https://github.com/k8s-infra-cherrypick-robot))

# v1.6.0 - Changelog since v1.5.6

## Changes by Kind

### Feature

- CMEK support now won't be checked in the CSI driver, trying to create basic or premium tier instances with cmek will result in invalid argument error from the Filestore API.

  high scale tier instance creation with IP reservation is now supported ([#563](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/563), [@leiyiz](https://github.com/leiyiz))

### Uncategorized

- Add labels to backups created through the driver ([#561](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/561), [@hsadoyan](https://github.com/hsadoyan))

## Dependencies

_Nothing has changed._
