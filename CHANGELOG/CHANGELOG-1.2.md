# v1.2.0 - Changelog since v1.1.4

## Changes by Kind

### Feature

- Metric emission for multishare ([#239](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/239), [@leiyiz](https://github.com/leiyiz))
- Cap the max retry interval time for provisioner sidecar to 1 min instead of 5mins. ([#258](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/258), [@saikat-royc](https://github.com/saikat-royc))
- Multishare create/delete volume based on APIs ([#254](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/254), [@saikat-royc](https://github.com/saikat-royc))
- New 'multishare' overlay ([#255](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/255), [@leiyiz](https://github.com/leiyiz))
- Add storageclass webhook into cloudbuild ([#233](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/233), [@leiyiz](https://github.com/leiyiz))
- Implement cloudprovider code for resize multishare instance ([#263](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/263), [@saikat-royc](https://github.com/saikat-royc))
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
