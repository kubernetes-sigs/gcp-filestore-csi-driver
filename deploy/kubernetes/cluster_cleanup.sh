#!/bin/bash

mydir="$(dirname $0)"
source "$mydir/../common.sh"

${KUSTOMIZE_PATH} build "${PKGDIR}/deploy/kubernetes/overlays/${DEPLOY_VERSION}" | ${KUBECTL} delete -v="${VERBOSITY}" --ignore-not-found -f -
${KUBECTL} delete secret gcp-filestore-csi-driver-sa -v="${VERBOSITY}" --ignore-not-found

if [[ "${GCFS_NS}" != "default" ]] && \
  ${KUBECTL} get namespace "${GCFS_NS}" -v="${VERBOSITY}";
then
    ${KUBECTL} delete namespace "${GCFS_NS}" -v="${VERBOSITY}"
fi
