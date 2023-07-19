# v1.6.0 - Changelog since v1.5.6

## Changes by Kind

### Feature

- CMEK support now won't be checked in the CSI driver, trying to create basic or premium tier instances with cmek will result in invalid argument error from the Filestore API.

  high scale tier instance creation with IP reservation is now supported ([#563](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/563), [@leiyiz](https://github.com/leiyiz))

### Uncategorized

- Add labels to backups created through the driver ([#561](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/pull/561), [@hsadoyan](https://github.com/hsadoyan))

## Dependencies

_Nothing has changed._
