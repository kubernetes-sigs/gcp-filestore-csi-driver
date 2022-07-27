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
