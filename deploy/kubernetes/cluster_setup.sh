#!/bin/bash

set -x
set -o nounset
set -o errexit

mydir="$(dirname $0)"

source "$mydir/../common.sh"

# DEPLOY_VERSION should point to the overlays name.
ensure_var DEPLOY_VERSION
ensure_kustomize

if ! ${KUBECTL} get namespace "${GCFS_NS}" -v="${VERBOSITY}";
then
  ${KUBECTL} create namespace "${GCFS_NS}" -v="${VERBOSITY}"
fi

# GKE requires this extra cluster-admin rolebinding in order to create clusterroles
if ! kubectl get clusterrolebinding cluster-admin-binding; then
  kubectl create clusterrolebinding cluster-admin-binding --clusterrole cluster-admin --user $(gcloud config get-value account)
fi

if [ "${DEPLOY_VERSION}" != dev ]; then
  if ! kubectl get secret gcp-filestore-csi-driver-sa --namespace=$GCFS_NS; then
    kubectl create secret generic gcp-filestore-csi-driver-sa --from-file="$GCFS_SA_FILE" --namespace=$GCFS_NS
  fi
fi

if [ "${DEPLOY_VERSION}" == multishare ]; then
  $mydir/webhook-example/create-cert.sh --namespace ${GCFS_NS} --service fs-validation
  webhook_config="$mydir/overlays/${DEPLOY_VERSION}/mutation-configuration.yaml"
  cat $mydir/mutation-webhook-configuration-template | $mydir/webhook-example/patch-ca-bundle.sh > $webhook_config
  kubectl apply -f $webhook_config
  rm $webhook_config
fi

# Use a securely created, unpredictable temporary file (mode 0600) instead of a
# fixed path in a world-writable directory, to prevent other local users from
# tampering with the generated specs before they are applied.
tmp_spec="$(mktemp "${TMPDIR:-/tmp}/gcp-filestore-csi-driver-specs-generated.XXXXXX")"
readonly tmp_spec
trap 'rm -f "${tmp_spec}"' EXIT
${KUSTOMIZE_PATH} build "${PKGDIR}/deploy/kubernetes/overlays/${DEPLOY_VERSION}" | tee "${tmp_spec}"
${KUBECTL} apply -v="${VERBOSITY}" -f "${tmp_spec}"
