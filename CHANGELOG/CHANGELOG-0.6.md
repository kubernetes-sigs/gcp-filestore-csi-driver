# v0.6.0 - Changelog since v0.5.0

## Changes by Kind

### Feature

- Update v1beta1 api (which includes enterprise tier) and move filestore api calls to v1beta1 ([#145](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/145), [@leiyiz](https://github.com/leiyiz))
- Support for enterprise tier filestore instance provisioning ([#149](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/149), [@leiyiz](https://github.com/leiyiz))

### Other (Cleanup or Flake)

- Poc for deploy of GKE managed filestore csi driver for prow tests ([#131](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/131), [@leiyiz](https://github.com/leiyiz))
- Solve the image pull auth issue by adding oauthscopes in api request ([#141](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/141), [@leiyiz](https://github.com/leiyiz))
- Cleaning up the clusterUpGKE ([#143](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/143), [@leiyiz](https://github.com/leiyiz))
- Fixed issue with inability to recognize gce-region flag is set ([#144](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/144), [@leiyiz](https://github.com/leiyiz))
- Fixed issue with clusterUpGKE when release channel is set ([#145](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/145), [@leiyiz](https://github.com/leiyiz))
- Fix kube version parse ([#148](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/148), [@saikat-royc](https://github.com/saikat-royc))
- Skip topology tests for filestore csi driver ([#150](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/150), [@saikat-royc](https://github.com/saikat-royc))
- Simplify logic to determine PVC request size ([#154](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/154), [@leiyiz](https://github.com/leiyiz))

### Documentation

- Rename master to main ([#142](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/142), [@leiyiz](https://github.com/ikarldasan))
- Update documentation for preprovisioned PVC ([#124](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/124), [@mattcary](https://github.com/mattcary))
- Overlays documentation and some readme updates ([#146](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/146), [@saikat-royc](https://github.com/saikat-royc))

### Dependencies

### Added
_Nothing has changed._

### Changed
_Nothing has changed._

### Removed
_Nothing has changed._
