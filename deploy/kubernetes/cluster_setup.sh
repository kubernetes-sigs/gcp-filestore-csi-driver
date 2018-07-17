#!/bin/bash

set -x
set -o nounset
set -o errexit

mydir="$(dirname $0)"

source "$mydir/../common.sh"

# GKE requires this extra cluster-admin rolebinding in order to create clusterroles
if ! kubectl get clusterrolebinding cluster-admin binding; then
  kubectl create clusterrolebinding cluster-admin-binding --clusterrole cluster-admin --user $(gcloud config get-value account)
fi

kubectl apply -f "$mydir/manifests/setup_cluster.yaml"

if ! kubectl get secret gcp-filestore-csi-driver-sa --namespace=$GCFS_NS; then
  kubectl create secret generic gcp-filestore-csi-driver-sa --from-file="$GCFS_SA_FILE" --namespace=$GCFS_NS
fi
