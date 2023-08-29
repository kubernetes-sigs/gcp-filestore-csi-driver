# v1.4.7 - Changelog since v1.4.6

## Changes by Kind

### Bug or Regression

- Improve error code logging ([#574](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/574), [@judemars](https://github.com/judemars))

# v1.4.6 - Changelog since v1.4.5

## Changes by Kind

### Bug or Regression

- Update go version to 1.20.5 for CVE fixes ([#549](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/549), [@saikat-royc](https://github.com/saikat-royc))

# v1.4.5 - Changelog since v1.4.4

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

# v1.4.4 - Changelog since v1.4.3

## Changes by Kind

### Uncategorized

- update golang version to 1.19.8 ([#484](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/484), [@saikat-royc](https://github.com/saikat-royc))

# v1.4.3 - Changelog since v1.4.2

## Changes by Kind

### Bug or Regression

- Enable parsing regional backup location ([#462](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/462), [@hsadoyan](https://github.com/hsadoyan))
- Update golang version to 1.19.7 ([#466](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/466), [@saikat-royc](https://github.com/saikat-royc))
- Migrate to bullseye debian base image ([#468](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/468), [@saikat-royc](https://github.com/saikat-royc))
- Update golang 1.19.7 for webhook ([#470](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/470), [@saikat-royc](https://github.com/saikat-royc))

# v1.4.2 - Changelog since v1.4.1

## Changes by Kind

### Bug or Regression

- Fix pointer issue in lock release controller ([#458](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/458)), [@tyuchn](https://github.com/tyuchn))

# v1.4.1 - Changelog since v1.4.0

## Changes by Kind

### Feature

- "max-volume-size" storage class webhook validation changes ([#444](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/444), [@saikat-royc](https://github.com/saikat-royc))
- Lock release controller ([#445](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/445), [@tyuchn](https://github.com/tyuchn))
- NodeStageVolume/NodeUnstageVolume with lock info update ([#423](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/423), [@tyuchn](https://github.com/tyuchn))
- Initial setup of CRD for multishare resources ([#415](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/415), [@leiyiz](https://github.com/leiyiz))

### Uncategorized

- Update golang.org/x/net to 0.7.0 for cve fix ([#448](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/448), [@saikat-royc](https://github.com/saikat-royc))
- Fix backup source comparison logic for single share instances ([#447](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/447), [@saikat-royc](https://github.com/saikat-royc))
- Bump cloudbuild and e2e test timeout ([#453](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/453), [@tyuchn](https://github.com/tyuchn))

## Dependencies

### Changed
- golang.org/x/net: v0.5.0 → v0.7.0
- golang.org/x/sys: v0.4.0 → v0.5.0
- golang.org/x/term: v0.4.0 → v0.5.0
- golang.org/x/text: v0.6.0 → v0.7.0

# v1.4.0 - Changelog since v1.3.11

## Changes by Kind

### Uncategorized

- Node driver will call the ReleaseLock function to release all locks on a GKE node during reconciliation. ([#416](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/416), [@tyuchn](https://github.com/tyuchn))

## Dependencies

### Added
- github.com/prashanthpai/sunrpc: [689a388](https://github.com/prashanthpai/sunrpc/tree/689a388)
- github.com/rasky/go-xdr: [1a41d1a](https://github.com/rasky/go-xdr/tree/1a41d1a)
