**Attention:**
1.3.3 is not a recommended version to use because of known issues which can cause failures in volume provisioning with ip reservation. Users are recommended to skip 1.3.3 and directly use 1.3.4

# v1.3.10 - Changelog since v1.3.9

- Update golang version ([#400](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/400), [@saikat-royc](https://github.com/saikat-royc))


# v1.3.9 - Changelog since v1.3.5

### Bug or Regression

- Strict check for filestore service endpoints ([#383](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/383), [@saikat-royc](https://github.com/saikat-royc))

# v1.3.5 - Changelog since v1.3.4

### Feature
- If multishare is enabled, the container now requires "--gke-cluster-name" flag to be set ([#372](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/372), [@leiyiz](https://github.com/leiyiz))

# v1.3.4 - Changelog since v1.3.3

### Bug or Regression

- fix basePath set to empty ([#366](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/366), [@leiyiz](https://github.com/leiyiz))

# v1.3.3 - Changelog since v1.3.2 (Bad version)

## Changes by Kind

### Bug or Regression

- Instance validation improvement when choosing eligible instances ([#337](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/337), [@tyuchn](https://github.com/tyuchn))
- Consume official v1beta1 go client for file ([#340](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/340), [@leiyiz](https://github.com/leiyiz))
- Add reserved ip range check and fail earlier if invalid ([#347](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/347), [@tyuchn](https://github.com/tyuchn))
## Dependencies

### Added
- cloud.google.com/go/compute: v1.7.0
- cloud.google.com/go/iam: v0.3.0
- github.com/googleapis/enterprise-certificate-proxy: [v0.1.0](https://github.com/googleapis/enterprise-certificate-proxy/tree/v0.1.0)
- github.com/googleapis/go-type-adapters: [v1.0.0](https://github.com/googleapis/go-type-adapters/tree/v1.0.0)

### Changed
- cloud.google.com/go/storage: v1.10.0 → v1.22.1
- cloud.google.com/go: v0.97.0 → v0.102.0
- github.com/cncf/udpa/go: [5459f2c → 04548b0](https://github.com/cncf/udpa/go/compare/5459f2c...04548b0)
- github.com/cncf/xds/go: [fbca930 → cb28da3](https://github.com/cncf/xds/go/compare/fbca930...cb28da3)
- github.com/envoyproxy/go-control-plane: [63b5d3c → 49ff273](https://github.com/envoyproxy/go-control-plane/compare/63b5d3c...49ff273)
- github.com/googleapis/gax-go/v2: [v2.1.1 → v2.4.0](https://github.com/googleapis/gax-go/v2/compare/v2.1.1...v2.4.0)
- golang.org/x/net: c690dde → c7608f3
- golang.org/x/oauth2: 622c5d5 → 128564f
- golang.org/x/sync: 036812b → 0de741c
- golang.org/x/sys: bc2c85a → 3c1f352
- golang.org/x/xerrors: 5ec99f8 → 65e6541
- google.golang.org/api: v0.59.0 → v0.90.0
- google.golang.org/genproto: 42d7afd → dd149ef
- google.golang.org/grpc: v1.40.0 → v1.48.0
- google.golang.org/protobuf: v1.28.0 → v1.28.1

# v1.3.2 - Changelog since v1.3.1

## Changes by Kind

### Bug or Regression

- Multishare ip reservation bug fix ([#341](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/341), [@tyuchn](https://github.com/tyuchn))

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
