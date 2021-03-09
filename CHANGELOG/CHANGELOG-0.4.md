# Release notes for v0.4.0

## Changes by Kind

### Feature

- Add support for fsGroupPolicy in CSI driver object for prow-gke-release-staging-rc-master overlay ([#102](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/102), [@saikat-royc](https://github.com/saikat-royc))
- Add support for fsGroupPolicy in CSI driver object for stable-master overlay ([#103](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/103), [@saikat-royc](https://github.com/saikat-royc))
- Enable flag to configure Cloud provider config ([#107](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/107), [@saikat-royc](https://github.com/saikat-royc))

### Tests
- Add configurable timeouts for k8s e2e tests ([#92](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/92), [@saikat-royc](https://github.com/saikat-royc))
- Support GKE deployment in integration tests ([#98](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/98), [@saikat-royc](https://github.com/saikat-royc))
- Handle non-existent cluster delete in integration test runner ([#109](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/109), [@saikat-royc](https://github.com/saikat-royc))

### Documentation

- Readme update for k8s minor version overlays ([#100](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/100), [@saikat-royc](https://github.com/saikat-royc))
- Document cross-cluster access of filestore ([#97](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/97), [@saikat-royc](https://github.com/saikat-royc))

### Other (Cleanup or Flake)

- Split the overlays into per k8s minor version ([#98](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/98), [@saikat-royc](https://github.com/saikat-royc))
- Upgrade google.golang.org/api for file/v1beta1 ([#93](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/93), [@annapendleton](https://github.com/annapendleton))
- Use PULL_BASE_REF to generate driver image tags([#94](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/94), [@saikat-royc](https://github.com/saikat-royc))
- Add workaround for intermittent docker COPY error([#95](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/95), [@saikat-royc](https://github.com/saikat-royc))
- k8s minor version overlays([#98](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/98), [@saikat-royc](https://github.com/saikat-royc))
- Remove old stable and prow-gke-release-staging-rc overlays ([#101](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/101), [@saikat-royc](https://github.com/saikat-royc))
- images: update image repositories ([#105](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/105), [@saikat-royc](https://github.com/saikat-royc))
- remove nodeid flag requirement for controller service ([#107](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/107), [@saikat-royc](https://github.com/saikat-royc))

## Dependencies

### Changed
- golang.org/x/mod: v0.3.0 → v0.4.0
- golang.org/x/net: 4f7140c → f585440
- golang.org/x/sync: 6e8e738 → 67f06af
- golang.org/x/tools: 64a9e34 → bef1c47
- google.golang.org/api: v0.33.0 → v0.35.0
