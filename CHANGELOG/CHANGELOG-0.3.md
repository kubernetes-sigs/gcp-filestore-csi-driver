# Changelog since v0.2.0

## Changes by Kind

### Feature
- Upgrade driver to use CSI spec 1.3.0 ([#45](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/45), [@saikat-royc](https://github.com/saikat-royc))
- Add support for Volume Expansion for Filestore CSI driver ([#55](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/55), [@saikat-royc](https://github.com/saikat-royc))
- Enable Filestore Backup Create/Delete using CSI CreateSnapshot/DeleteSnapshot API and CreateVolume from a backup source for CSI Filestore driver ([#61](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/61), [@saikat-royc](https://github.com/saikat-royc))
- Support NodeStageVolume for Filestore CSI driver, and NodePublish should create the target path if missing. ([#59](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/59), [@saikat-royc](https://github.com/saikat-royc))
- Support for Labels ([#58](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/58), [@saikat-royc](https://github.com/saikat-royc))
- Topology feature support for Filestore CSI driver ([#56](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/56), [@saikat-royc](https://github.com/saikat-royc))

### Tests

- Add backup and restore e2e test. ([#76](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/76), [@annapendleton](https://github.com/annapendleton))
- Add extra validation in e2e on instance via file api ([#68](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/68), [@annapendleton](https://github.com/annapendleton))
- Add resize e2e tests ([#70](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/70), [@annapendleton](https://github.com/annapendleton))
- Setup kubetest network for k8s e2e test ([#71](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/71), [@saikat-royc](https://github.com/saikat-royc))
- K8s integration test for filestore driver ([#67](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/67), [@saikat-royc](https://github.com/saikat-royc))

### Failing Test

- Fix CSI e2e test ([#66](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/66), [@saikat-royc](https://github.com/saikat-royc))

### Documentation

- FsGroup Documentation ([#78](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/78), [@saikat-royc](https://github.com/saikat-royc))
- Readme docs for filestore driver ([#72](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/72), [@saikat-royc](https://github.com/saikat-royc))

### Other Notable Changes

- Added support for WorkloadIdentity ([#64](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/64), [@xvzf](https://github.com/xvzf))
- Create overlays for filestore driver ([#65](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/65), [@saikat-royc](https://github.com/saikat-royc))

