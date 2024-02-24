# v1.5.17 - Changelog since v1.5.16

## Changes by Kind

### Uncategorized

- Change debian base image from bullseye to bookworm to fix: CVE-2023-39804, CVE-2023-47038, CVE-2022-48303. ([#800](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/800), [@k8s-infra-cherrypick-robot](https://github.com/k8s-infra-cherrypick-robot))

## Dependencies

### Added
_Nothing has changed._

### Changed
_Nothing has changed._

### Removed
_Nothing has changed._


# v1.5.16 - Changelog since v1.5.15

## Changes by Kind

### Uncategorized

- Update golang.org/x/crypto to v0.17.0 to fix CVE-2023-48795 ([#757](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/757), [@sunnylovestiramisu](https://github.com/sunnylovestiramisu))

## Dependencies

### Added
_Nothing has changed._

### Changed
- cloud.google.com/go/bigquery: v1.57.1 → v1.8.0
- cloud.google.com/go/datastore: v1.15.0 → v1.1.0
- cloud.google.com/go/firestore: v1.14.0 → v1.1.0
- cloud.google.com/go/logging: v1.8.1 → v1.0.0
- cloud.google.com/go/pubsub: v1.33.0 → v1.4.0
- cloud.google.com/go: v0.110.10 → v0.110.8
- golang.org/x/crypto: v0.16.0 → v0.17.0

### Removed
- cloud.google.com/go/accessapproval: v1.7.4
- cloud.google.com/go/accesscontextmanager: v1.8.4
- cloud.google.com/go/aiplatform: v1.54.0
- cloud.google.com/go/analytics: v0.21.6
- cloud.google.com/go/apigateway: v1.6.4
- cloud.google.com/go/apigeeconnect: v1.6.4
- cloud.google.com/go/apigeeregistry: v0.8.2
- cloud.google.com/go/appengine: v1.8.4
- cloud.google.com/go/area120: v0.8.4
- cloud.google.com/go/artifactregistry: v1.14.6
- cloud.google.com/go/asset: v1.15.3
- cloud.google.com/go/assuredworkloads: v1.11.4
- cloud.google.com/go/automl: v1.13.4
- cloud.google.com/go/baremetalsolution: v1.2.3
- cloud.google.com/go/batch: v1.6.3
- cloud.google.com/go/beyondcorp: v1.0.3
- cloud.google.com/go/billing: v1.17.4
- cloud.google.com/go/binaryauthorization: v1.7.3
- cloud.google.com/go/certificatemanager: v1.7.4
- cloud.google.com/go/channel: v1.17.3
- cloud.google.com/go/cloudbuild: v1.15.0
- cloud.google.com/go/clouddms: v1.7.3
- cloud.google.com/go/cloudtasks: v1.12.4
- cloud.google.com/go/contactcenterinsights: v1.12.0
- cloud.google.com/go/container: v1.28.0
- cloud.google.com/go/containeranalysis: v0.11.3
- cloud.google.com/go/datacatalog: v1.19.0
- cloud.google.com/go/dataflow: v0.9.4
- cloud.google.com/go/dataform: v0.9.1
- cloud.google.com/go/datafusion: v1.7.4
- cloud.google.com/go/datalabeling: v0.8.4
- cloud.google.com/go/dataplex: v1.11.2
- cloud.google.com/go/dataproc/v2: v2.3.0
- cloud.google.com/go/dataqna: v0.8.4
- cloud.google.com/go/datastream: v1.10.3
- cloud.google.com/go/deploy: v1.15.0
- cloud.google.com/go/dialogflow: v1.44.3
- cloud.google.com/go/dlp: v1.11.1
- cloud.google.com/go/documentai: v1.23.5
- cloud.google.com/go/domains: v0.9.4
- cloud.google.com/go/edgecontainer: v1.1.4
- cloud.google.com/go/errorreporting: v0.3.0
- cloud.google.com/go/essentialcontacts: v1.6.5
- cloud.google.com/go/eventarc: v1.13.3
- cloud.google.com/go/filestore: v1.8.0
- cloud.google.com/go/functions: v1.15.4
- cloud.google.com/go/gkebackup: v1.3.4
- cloud.google.com/go/gkeconnect: v0.8.4
- cloud.google.com/go/gkehub: v0.14.4
- cloud.google.com/go/gkemulticloud: v1.0.3
- cloud.google.com/go/gsuiteaddons: v1.6.4
- cloud.google.com/go/iam: v1.1.5
- cloud.google.com/go/iap: v1.9.3
- cloud.google.com/go/ids: v1.4.4
- cloud.google.com/go/iot: v1.7.4
- cloud.google.com/go/kms: v1.15.5
- cloud.google.com/go/language: v1.12.2
- cloud.google.com/go/lifesciences: v0.9.4
- cloud.google.com/go/longrunning: v0.5.4
- cloud.google.com/go/managedidentities: v1.6.4
- cloud.google.com/go/maps: v1.6.1
- cloud.google.com/go/mediatranslation: v0.8.4
- cloud.google.com/go/memcache: v1.10.4
- cloud.google.com/go/metastore: v1.13.3
- cloud.google.com/go/monitoring: v1.16.3
- cloud.google.com/go/networkconnectivity: v1.14.3
- cloud.google.com/go/networkmanagement: v1.9.3
- cloud.google.com/go/networksecurity: v0.9.4
- cloud.google.com/go/notebooks: v1.11.2
- cloud.google.com/go/optimization: v1.6.2
- cloud.google.com/go/orchestration: v1.8.4
- cloud.google.com/go/orgpolicy: v1.11.4
- cloud.google.com/go/osconfig: v1.12.4
- cloud.google.com/go/oslogin: v1.12.2
- cloud.google.com/go/phishingprotection: v0.8.4
- cloud.google.com/go/policytroubleshooter: v1.10.2
- cloud.google.com/go/privatecatalog: v0.9.4
- cloud.google.com/go/pubsublite: v1.8.1
- cloud.google.com/go/recaptchaenterprise/v2: v2.8.4
- cloud.google.com/go/recommendationengine: v0.8.4
- cloud.google.com/go/recommender: v1.11.3
- cloud.google.com/go/redis: v1.14.1
- cloud.google.com/go/resourcemanager: v1.9.4
- cloud.google.com/go/resourcesettings: v1.6.4
- cloud.google.com/go/retail: v1.14.4
- cloud.google.com/go/run: v1.3.3
- cloud.google.com/go/scheduler: v1.10.5
- cloud.google.com/go/secretmanager: v1.11.4
- cloud.google.com/go/security: v1.15.4
- cloud.google.com/go/securitycenter: v1.24.2
- cloud.google.com/go/servicedirectory: v1.11.3
- cloud.google.com/go/shell: v1.7.4
- cloud.google.com/go/spanner: v1.53.0
- cloud.google.com/go/speech: v1.21.0
- cloud.google.com/go/storagetransfer: v1.10.3
- cloud.google.com/go/talent: v1.6.5
- cloud.google.com/go/texttospeech: v1.7.4
- cloud.google.com/go/tpu: v1.6.4
- cloud.google.com/go/trace: v1.10.4
- cloud.google.com/go/translate: v1.9.3
- cloud.google.com/go/video: v1.20.3
- cloud.google.com/go/videointelligence: v1.11.4
- cloud.google.com/go/vision/v2: v2.7.5
- cloud.google.com/go/vmmigration: v1.7.4
- cloud.google.com/go/vmwareengine: v1.0.3
- cloud.google.com/go/vpcaccess: v1.7.4
- cloud.google.com/go/webrisk: v1.9.4
- cloud.google.com/go/websecurityscanner: v1.6.4
- cloud.google.com/go/workflows: v1.12.3


# v1.5.15 - Changelog since v1.5.14

## Changes by Kind

### Other (Cleanup or Flake)

- Update webhook go builder to 1.20.12 ([#747](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/747), [@msau42](https://github.com/msau42))

## Dependencies

### Added
_Nothing has changed._

### Changed
_Nothing has changed._

### Removed
_Nothing has changed._

# v1.5.14 - Changelog since v1.5.13

## Changes by Kind

### Uncategorized

- Add labels to backups created through the driver ([#731](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/731), [@k8s-infra-cherrypick-robot](https://github.com/k8s-infra-cherrypick-robot))
- Multishare Backups (disabled) and Backup Labels ([#735](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/735), [@hsadoyan](https://github.com/hsadoyan))
- Update golang builder to 1.20.12 ([#742](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/742), [@k8s-infra-cherrypick-robot](https://github.com/k8s-infra-cherrypick-robot))

## Dependencies

### Added
- cloud.google.com/go/dataproc/v2: v2.3.0
- github.com/google/go-pkcs11: [c6f7932](https://github.com/google/go-pkcs11/tree/c6f7932)
- github.com/google/s2a-go: [v0.1.7](https://github.com/google/s2a-go/tree/v0.1.7)
- google.golang.org/genproto/googleapis/api: bbf56f3
- google.golang.org/genproto/googleapis/bytestream: 83a465c
- google.golang.org/genproto/googleapis/rpc: 3a041ad

### Changed
- cloud.google.com/go/accessapproval: v1.6.0 → v1.7.4
- cloud.google.com/go/accesscontextmanager: v1.7.0 → v1.8.4
- cloud.google.com/go/aiplatform: v1.37.0 → v1.54.0
- cloud.google.com/go/analytics: v0.19.0 → v0.21.6
- cloud.google.com/go/apigateway: v1.5.0 → v1.6.4
- cloud.google.com/go/apigeeconnect: v1.5.0 → v1.6.4
- cloud.google.com/go/apigeeregistry: v0.6.0 → v0.8.2
- cloud.google.com/go/appengine: v1.7.1 → v1.8.4
- cloud.google.com/go/area120: v0.7.1 → v0.8.4
- cloud.google.com/go/artifactregistry: v1.13.0 → v1.14.6
- cloud.google.com/go/asset: v1.13.0 → v1.15.3
- cloud.google.com/go/assuredworkloads: v1.10.0 → v1.11.4
- cloud.google.com/go/automl: v1.12.0 → v1.13.4
- cloud.google.com/go/baremetalsolution: v0.5.0 → v1.2.3
- cloud.google.com/go/batch: v0.7.0 → v1.6.3
- cloud.google.com/go/beyondcorp: v0.5.0 → v1.0.3
- cloud.google.com/go/bigquery: v1.50.0 → v1.57.1
- cloud.google.com/go/billing: v1.13.0 → v1.17.4
- cloud.google.com/go/binaryauthorization: v1.5.0 → v1.7.3
- cloud.google.com/go/certificatemanager: v1.6.0 → v1.7.4
- cloud.google.com/go/channel: v1.12.0 → v1.17.3
- cloud.google.com/go/cloudbuild: v1.9.0 → v1.15.0
- cloud.google.com/go/clouddms: v1.5.0 → v1.7.3
- cloud.google.com/go/cloudtasks: v1.10.0 → v1.12.4
- cloud.google.com/go/compute: v1.19.1 → v1.23.3
- cloud.google.com/go/contactcenterinsights: v1.6.0 → v1.12.0
- cloud.google.com/go/container: v1.15.0 → v1.28.0
- cloud.google.com/go/containeranalysis: v0.9.0 → v0.11.3
- cloud.google.com/go/datacatalog: v1.13.0 → v1.19.0
- cloud.google.com/go/dataflow: v0.8.0 → v0.9.4
- cloud.google.com/go/dataform: v0.7.0 → v0.9.1
- cloud.google.com/go/datafusion: v1.6.0 → v1.7.4
- cloud.google.com/go/datalabeling: v0.7.0 → v0.8.4
- cloud.google.com/go/dataplex: v1.6.0 → v1.11.2
- cloud.google.com/go/dataqna: v0.7.0 → v0.8.4
- cloud.google.com/go/datastore: v1.11.0 → v1.15.0
- cloud.google.com/go/datastream: v1.7.0 → v1.10.3
- cloud.google.com/go/deploy: v1.8.0 → v1.15.0
- cloud.google.com/go/dialogflow: v1.32.0 → v1.44.3
- cloud.google.com/go/dlp: v1.9.0 → v1.11.1
- cloud.google.com/go/documentai: v1.18.0 → v1.23.5
- cloud.google.com/go/domains: v0.8.0 → v0.9.4
- cloud.google.com/go/edgecontainer: v1.0.0 → v1.1.4
- cloud.google.com/go/essentialcontacts: v1.5.0 → v1.6.5
- cloud.google.com/go/eventarc: v1.11.0 → v1.13.3
- cloud.google.com/go/filestore: v1.6.0 → v1.8.0
- cloud.google.com/go/firestore: v1.9.0 → v1.14.0
- cloud.google.com/go/functions: v1.13.0 → v1.15.4
- cloud.google.com/go/gkebackup: v0.4.0 → v1.3.4
- cloud.google.com/go/gkeconnect: v0.7.0 → v0.8.4
- cloud.google.com/go/gkehub: v0.12.0 → v0.14.4
- cloud.google.com/go/gkemulticloud: v0.5.0 → v1.0.3
- cloud.google.com/go/gsuiteaddons: v1.5.0 → v1.6.4
- cloud.google.com/go/iam: v0.13.0 → v1.1.5
- cloud.google.com/go/iap: v1.7.1 → v1.9.3
- cloud.google.com/go/ids: v1.3.0 → v1.4.4
- cloud.google.com/go/iot: v1.6.0 → v1.7.4
- cloud.google.com/go/kms: v1.10.1 → v1.15.5
- cloud.google.com/go/language: v1.9.0 → v1.12.2
- cloud.google.com/go/lifesciences: v0.8.0 → v0.9.4
- cloud.google.com/go/logging: v1.7.0 → v1.8.1
- cloud.google.com/go/longrunning: v0.4.1 → v0.5.4
- cloud.google.com/go/managedidentities: v1.5.0 → v1.6.4
- cloud.google.com/go/maps: v0.7.0 → v1.6.1
- cloud.google.com/go/mediatranslation: v0.7.0 → v0.8.4
- cloud.google.com/go/memcache: v1.9.0 → v1.10.4
- cloud.google.com/go/metastore: v1.10.0 → v1.13.3
- cloud.google.com/go/monitoring: v1.13.0 → v1.16.3
- cloud.google.com/go/networkconnectivity: v1.11.0 → v1.14.3
- cloud.google.com/go/networkmanagement: v1.6.0 → v1.9.3
- cloud.google.com/go/networksecurity: v0.8.0 → v0.9.4
- cloud.google.com/go/notebooks: v1.8.0 → v1.11.2
- cloud.google.com/go/optimization: v1.3.1 → v1.6.2
- cloud.google.com/go/orchestration: v1.6.0 → v1.8.4
- cloud.google.com/go/orgpolicy: v1.10.0 → v1.11.4
- cloud.google.com/go/osconfig: v1.11.0 → v1.12.4
- cloud.google.com/go/oslogin: v1.9.0 → v1.12.2
- cloud.google.com/go/phishingprotection: v0.7.0 → v0.8.4
- cloud.google.com/go/policytroubleshooter: v1.6.0 → v1.10.2
- cloud.google.com/go/privatecatalog: v0.8.0 → v0.9.4
- cloud.google.com/go/pubsub: v1.30.0 → v1.33.0
- cloud.google.com/go/pubsublite: v1.7.0 → v1.8.1
- cloud.google.com/go/recaptchaenterprise/v2: v2.7.0 → v2.8.4
- cloud.google.com/go/recommendationengine: v0.7.0 → v0.8.4
- cloud.google.com/go/recommender: v1.9.0 → v1.11.3
- cloud.google.com/go/redis: v1.11.0 → v1.14.1
- cloud.google.com/go/resourcemanager: v1.7.0 → v1.9.4
- cloud.google.com/go/resourcesettings: v1.5.0 → v1.6.4
- cloud.google.com/go/retail: v1.12.0 → v1.14.4
- cloud.google.com/go/run: v0.9.0 → v1.3.3
- cloud.google.com/go/scheduler: v1.9.0 → v1.10.5
- cloud.google.com/go/secretmanager: v1.10.0 → v1.11.4
- cloud.google.com/go/security: v1.13.0 → v1.15.4
- cloud.google.com/go/securitycenter: v1.19.0 → v1.24.2
- cloud.google.com/go/servicedirectory: v1.9.0 → v1.11.3
- cloud.google.com/go/shell: v1.6.0 → v1.7.4
- cloud.google.com/go/spanner: v1.45.0 → v1.53.0
- cloud.google.com/go/speech: v1.15.0 → v1.21.0
- cloud.google.com/go/storagetransfer: v1.8.0 → v1.10.3
- cloud.google.com/go/talent: v1.5.0 → v1.6.5
- cloud.google.com/go/texttospeech: v1.6.0 → v1.7.4
- cloud.google.com/go/tpu: v1.5.0 → v1.6.4
- cloud.google.com/go/trace: v1.9.0 → v1.10.4
- cloud.google.com/go/translate: v1.7.0 → v1.9.3
- cloud.google.com/go/video: v1.15.0 → v1.20.3
- cloud.google.com/go/videointelligence: v1.10.0 → v1.11.4
- cloud.google.com/go/vision/v2: v2.7.0 → v2.7.5
- cloud.google.com/go/vmmigration: v1.6.0 → v1.7.4
- cloud.google.com/go/vmwareengine: v0.3.0 → v1.0.3
- cloud.google.com/go/vpcaccess: v1.6.0 → v1.7.4
- cloud.google.com/go/webrisk: v1.8.0 → v1.9.4
- cloud.google.com/go/websecurityscanner: v1.5.0 → v1.6.4
- cloud.google.com/go/workflows: v1.10.0 → v1.12.3
- cloud.google.com/go: v0.110.0 → v0.110.10
- github.com/envoyproxy/go-control-plane: [9239064 → v0.11.1](https://github.com/envoyproxy/go-control-plane/compare/9239064...v0.11.1)
- github.com/envoyproxy/protoc-gen-validate: [v0.10.1 → v1.0.2](https://github.com/envoyproxy/protoc-gen-validate/compare/v0.10.1...v1.0.2)
- github.com/golang/glog: [v1.1.0 → v1.1.2](https://github.com/golang/glog/compare/v1.1.0...v1.1.2)
- github.com/google/go-cmp: [v0.5.9 → v0.6.0](https://github.com/google/go-cmp/compare/v0.5.9...v0.6.0)
- github.com/google/uuid: [v1.3.0 → v1.4.0](https://github.com/google/uuid/compare/v1.3.0...v1.4.0)
- github.com/googleapis/enterprise-certificate-proxy: [v0.2.3 → v0.3.2](https://github.com/googleapis/enterprise-certificate-proxy/compare/v0.2.3...v0.3.2)
- github.com/googleapis/gax-go/v2: [v2.7.1 → v2.12.0](https://github.com/googleapis/gax-go/v2/compare/v2.7.1...v2.12.0)
- golang.org/x/crypto: v0.14.0 → v0.16.0
- golang.org/x/net: v0.17.0 → v0.19.0
- golang.org/x/oauth2: v0.7.0 → v0.15.0
- golang.org/x/sync: v0.1.0 → v0.5.0
- golang.org/x/sys: v0.13.0 → v0.15.0
- golang.org/x/term: v0.13.0 → v0.15.0
- golang.org/x/text: v0.13.0 → v0.14.0
- golang.org/x/time: 583f2d6 → v0.5.0
- google.golang.org/api: v0.114.0 → v0.152.0
- google.golang.org/appengine: v1.6.7 → v1.6.8
- google.golang.org/genproto: daa745c → 83a465c
- google.golang.org/grpc: v1.56.3 → v1.59.0
- google.golang.org/protobuf: v1.30.0 → v1.31.0

### Removed
- cloud.google.com/go/apikeys: v0.6.0
- cloud.google.com/go/dataproc: v1.12.0
- cloud.google.com/go/gaming: v1.9.0
- cloud.google.com/go/servicecontrol: v1.11.1
- cloud.google.com/go/servicemanagement: v1.8.0
- cloud.google.com/go/serviceusage: v1.6.0


# v1.5.13 - Changelog since v1.5.11

## Changes by Kind

### Bug or Regression

- Bump Golang Builder version to 1.20.11 ([#708](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/708), [@uriel-guzman](https://github.com/uriel-guzman))
- Bump google.golang.org/grpc from v1.51.0 to v1.56.3 to fix CVE-2023-44487. ([#702](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/702), [@uriel-guzman](https://github.com/uriel-guzman))
- CVE fixes: CVE-2023-44487, CVE-2023-39323, CVE-2023-3978 ([#657](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/657), [@dannawang0221](https://github.com/dannawang0221))

## Dependencies

### Added
- cloud.google.com/go/apigeeregistry: v0.6.0
- cloud.google.com/go/apikeys: v0.6.0
- cloud.google.com/go/maps: v0.7.0
- cloud.google.com/go/vmwareengine: v0.3.0

### Changed
- cloud.google.com/go/accessapproval: v1.5.0 → v1.6.0
- cloud.google.com/go/accesscontextmanager: v1.4.0 → v1.7.0
- cloud.google.com/go/aiplatform: v1.27.0 → v1.37.0
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

# v1.5.11 - Changelog since v1.5.8
## Changes by Kind

### Uncategorized
- Bump go version to 1.20.8 ([#607](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/607))
- Remove ARG BUILDPLATFORM from Dockerfile ([#615](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/615))
- Make pkgdir match k8s_e2e dir ([#622](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/622))
- Bump webhook go version to 1.20.8 ([#634](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/634))

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
