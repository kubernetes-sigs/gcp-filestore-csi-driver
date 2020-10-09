#!/bin/bash

set -o nounset
set -o errexit

function ensure_var(){
    if [[ -z "${!1:-}" ]];
    then
        echo "${1} is unset"
        exit 1
    else
        echo "${1} is ${!1}"
    fi
}

# Installs kustomize in ${PKGDIR}/bin
function ensure_kustomize()
{
  ensure_var PKGDIR
  "${PKGDIR}/deploy/kubernetes/install_kustomize.sh"
}

ensure_var GOPATH
ensure_var PROJECT

readonly PKGDIR="${GOPATH}/src/sigs.k8s.io/gcp-filestore-csi-driver"
readonly VERBOSITY="${GCE_FS_VERBOSITY:-2}"
readonly KUSTOMIZE_PATH="${PKGDIR}/bin/kustomize"
readonly KUBECTL="${GCP_FS_KUBECTL:-kubectl}"
readonly GCFS_SA_DIR="${GCFS_SA_DIR:-$HOME}"

# If you override the file name, then deploy/kubernetes/base/controller/controller.yaml must also be
# updated
GCFS_SA_FILE="$GCFS_SA_DIR/gcp_filestore_csi_driver_sa.json"
GCFS_SA_NAME=gcp-filestore-csi-driver-sa
GCFS_NS=gcp-filestore-csi-driver

GCFS_IAM_NAME="$GCFS_SA_NAME@$PROJECT.iam.gserviceaccount.com"
