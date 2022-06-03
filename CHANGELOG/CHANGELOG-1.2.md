
# v1.2.3 - Changelog since v1.2.2

### Bug or Regression

- Shared VPC ipAddress fix and ListShare region limit ([#291](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/291), [@leiyiz](https://github.com/leiyiz))

- Fail instance eligibility check if any error encountered during checks ([#292](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/291), [@saikat-royc](https://github.com/saikat-royc))

# v1.2.2 - Changelog since v1.2.1

## Changes by Kind

### Feature
- Support IP Reservation for Multishare Filestore instances([#275](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/275), [@leiyiz](https://github.com/leiyiz))


# v1.2.1 - Changelog since v1.2.0


## Changes by Kind

### Feature
- Multishare volume expansion ([#270](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/270), [@leiyiz](https://github.com/leiyiz))
- List instances to list shares for multishare instance ([#271](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/271), [@leiyiz](https://github.com/leiyiz))

### Bug or Regression

- Align instance resize target bytes ([#269](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/269), [@saikat-royc](https://github.com/saikat-royc))

### Uncategorized

- Add test for NodeGetVolumeStats in resize e2e test. ([#268](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/268), [@tyuchn](https://github.com/tyuchn))
- Expose volume metrics by implementing NodeGetVolumeStats for NodeServer. ([#266](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/266), [@tyuchn](https://github.com/tyuchn))

## Dependencies

### Added
_Nothing has changed._

### Changed
_Nothing has changed._

### Removed
_Nothing has changed._

# v1.2.0 - Changelog since v1.1.4

## Changes by Kind

### Feature

- Metric emission for multishare ([#239](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/239), [@leiyiz](https://github.com/leiyiz))
- Cap the max retry interval time for provisioner sidecar to 1 min instead of 5mins. ([#258](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/258), [@saikat-royc](https://github.com/saikat-royc))
- Multishare create/delete volume based on APIs ([#254](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/254), [@saikat-royc](https://github.com/saikat-royc))
- New 'multishare' overlay ([#255](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/255), [@leiyiz](https://github.com/leiyiz))
- Add storageclass webhook into cloudbuild ([#233](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/233), [@leiyiz](https://github.com/leiyiz))

### Documentation

### Bug or Regression

- Handle uninitialized multishare controller object ([#252](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/252), [@saikat-royc](https://github.com/saikat-royc))

### Other (Cleanup or Flake)

- Add share cloud provider implementation ([#253](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/253), [@saikat-royc](https://github.com/saikat-royc))
- Multishare create/delete volume based on APIs ([#254](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/254), [@saikat-royc](https://github.com/saikat-royc))
- Fix error code mappings ([#249](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/249), [@amacaskill](https://github.com/amacaskill))
- Dont consider multishare instances in error state as non-ready instances ([#257](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/257), [@saikat-royc](https://github.com/saikat-royc))
- Update logging for multishares volume provisioning ([#259](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/259), [@saikat-royc](https://github.com/saikat-royc))

## Dependencies

### Added
_Nothing has changed._

### Changed
_Nothing has changed._

### Removed
_Nothing has changed._
