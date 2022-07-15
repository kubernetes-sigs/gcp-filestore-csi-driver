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

readonly overlay_name="${GCE_FS_OVERLAY_NAME:-stable-master}"
readonly boskos_resource_type="${GCE_FS_BOSKOS_RESOURCE_TYPE:-gce-project}"
readonly do_driver_build="${GCE_FS_DO_DRIVER_BUILD:-true}"
readonly deployment_strategy=${DEPLOYMENT_STRATEGY:-gce}
readonly kube_version=${GCE_FS_KUBE_VERSION:-master}
readonly test_version=${TEST_VERSION:-master}
readonly gce_zone=${GCE_CLUSTER_ZONE:-us-central1-b}
readonly image_type=${IMAGE_TYPE:-cos_containerd}
readonly teardown_driver=${GCE_FS_TEARDOWN_DRIVER:-true}
readonly gke_cluster_version=${GKE_CLUSTER_VERSION:-latest}
readonly gke_release_channel=${GKE_RELEASE_CHANNEL:-""}
readonly gke_node_version=${GKE_NODE_VERSION:-}
readonly gce_region=${GCE_CLUSTER_REGION:-}
readonly storageclass_files=${STORAGECLASS_FILES:-}
readonly use_gke_driver=${USE_STAGING_DRIVER:-false}
readonly parallel_run=${PARALLEL:-}

make -C "${PKGDIR}" test-k8s-integration

# Choose an older Kubetest2 commit version instead of using @latest 
# because of a regression in https://github.com/kubernetes-sigs/kubetest2/pull/183.
# Contact engprod oncall and ask about what is the version they are using for internal jobs.
go install sigs.k8s.io/kubetest2@0e09086b60c122e1084edd2368d3d27fe36f384f;
go install sigs.k8s.io/kubetest2/kubetest2-gce@0e09086b60c122e1084edd2368d3d27fe36f384f;
go install sigs.k8s.io/kubetest2/kubetest2-gke@0e09086b60c122e1084edd2368d3d27fe36f384f;
go install sigs.k8s.io/kubetest2/kubetest2-tester-ginkgo@0e09086b60c122e1084edd2368d3d27fe36f384f;

echo "make successful"
base_cmd="${PKGDIR}/bin/k8s-integration-test \
            --run-in-prow=true --service-account-file=${E2E_GOOGLE_APPLICATION_CREDENTIALS} \
            --do-driver-build=${do_driver_build} --teardown-driver=${teardown_driver} --boskos-resource-type=${boskos_resource_type} \
            --test-version=${test_version} --num-nodes=3 --deployment-strategy=${deployment_strategy}"

if [ "$use_gke_driver" = false ]; then
  base_cmd="${base_cmd} --deploy-overlay-name=${overlay_name}"
else
  base_cmd="${base_cmd} --use-gke-driver=${use_gke_driver}"
fi

if [ "$deployment_strategy" = "gke" ]; then
  if [ -n "$gke_release_channel" ]; then
    base_cmd="${base_cmd} --gke-release-channel=${gke_release_channel}"
  else
    base_cmd="${base_cmd} --gke-cluster-version=${gke_cluster_version}"
  fi

  if [ -n "$gke_node_version" ]; then
    base_cmd="${base_cmd} --gke-node-version=${gke_node_version}"
  fi
else
  base_cmd="${base_cmd} --kube-version=${kube_version}"
fi

if [ -z "$gce_region" ]; then
  base_cmd="${base_cmd} --gce-zone=${gce_zone}"
else
  base_cmd="${base_cmd} --gce-region=${gce_region}"
fi

if [ -n "$storageclass_files" ]; then
  base_cmd="${base_cmd} --storageclass-files=${storageclass_files}"
fi

if [ -n "$parallel_run" ]; then
  base_cmd="${base_cmd} --parallel=${parallel_run}"
fi

echo "$base_cmd"
eval "$base_cmd"
