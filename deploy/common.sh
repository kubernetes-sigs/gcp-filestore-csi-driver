#!/bin/bash

set -o nounset
set -o errexit

GCFS_SA_DIR="${GCFS_SA_DIR:-$HOME}"
# If you override the file name, then kubernetes/controller.yaml must also be
# updated
GCFS_SA_FILE="$GCFS_SA_DIR/gcp_filestore_csi_driver_sa.json"
GCFS_SA_NAME=gcp-filestore-csi-driver-sa
GCFS_NS=gcp-filestore-csi-driver
