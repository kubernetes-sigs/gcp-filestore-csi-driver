#!/bin/bash

set -o nounset
set -o errexit

readonly PKGDIR=${GOPATH}/src/sigs.k8s.io/gcp-filestore-csi-driver
readonly SCRIPTDIR="$( realpath -s "$(dirname $BASH_SOURCE[0])" )"
readonly K8S_E2E_SCRIPT_PARENT_DIR="$( realpath -s "$(dirname "$SCRIPTDIR")" )"

if [ "$K8S_E2E_SCRIPT_PARENT_DIR" != "$PKGDIR" ]; then
  echo "Mismatch in PKGDIR $PKGDIR and K8S_E2E_SCRIPT_PARENT_DIR $K8S_E2E_SCRIPT_PARENT_DIR"
  exit 1
fi

readonly overlay_name="${GCE_FS_OVERLAY_NAME:-stable}"
readonly boskos_resource_type="${GCE_FS_BOSKOS_RESOURCE_TYPE:-gce-project}"
readonly do_driver_build="${GCE_FS_DO_DRIVER_BUILD:-true}"
readonly deployment_strategy=${DEPLOYMENT_STRATEGY:-gce}
readonly kube_version=${GCE_FS_KUBE_VERSION:-master}
readonly test_version=${TEST_VERSION:-master}
readonly gce_zone=${GCE_CLUSTER_ZONE:-us-central1-b}
readonly image_type=${IMAGE_TYPE:-cos}
readonly teardown_driver=${GCE_FS_TEARDOWN_DRIVER:-true}

make -C "${PKGDIR}" test-k8s-integration
echo "make successful"
base_cmd="${PKGDIR}/bin/k8s-integration-test \
            --run-in-prow=true --service-account-file=${E2E_GOOGLE_APPLICATION_CREDENTIALS} \
            --do-driver-build=${do_driver_build} --teardown-driver=${teardown_driver} --boskos-resource-type=${boskos_resource_type} \
            --test-version=${test_version} --kube-version=${kube_version} --num-nodes=3 --image-type=${image_type} \
            --deploy-overlay-name=${overlay_name} --gce-zone=${gce_zone}"

eval "$base_cmd"
