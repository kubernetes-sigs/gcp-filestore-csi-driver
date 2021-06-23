# v0.5.0 - Changelog since v0.4.0

## Changes by Kind

### Feature

- Bump csi sidecar versions ([#123](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/123), [@saikat-royc](https://github.com/saikat-royc))
- Emit component_version metric ([#120](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/120), [@saikat-royc](https://github.com/saikat-royc))
- Enable leader election and metrics endpoint for sidecars ([#121](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/121), [@saikat-royc](https://github.com/saikat-royc))
- Skip statd service on node startup if it's already running ([#135](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/135), [@mattcary](https://github.com/mattcary))

### Other (Cleanup or Flake)

- Fix roles needed to deploy the driver, and use v1 csidriver object for 1.20+ clusters ([#128](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/128), [@saikat-royc](https://github.com/saikat-royc))
- Serialize volume operations for a given volume and return appropriate error codes ([#125](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/125), [@saikat-royc](https://github.com/saikat-royc))
- Switch to v1 Filestore backups ([#130](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/130), [@saikat-royc](https://github.com/saikat-royc))

### Documentation

- Clarify deploying the driver for non-developers ([#122](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/123), [@mattcary](https://github.com/mattcary))

### Dependencies

### Added
_Nothing has changed._

### Changed
_Nothing has changed._

### Removed
_Nothing has changed._
