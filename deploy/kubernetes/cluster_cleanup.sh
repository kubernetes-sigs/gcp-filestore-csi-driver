#!/bin/bash

mydir="$(dirname $0)"
kubectl delete secret gcp-filestore-csi-driver-sa --namespace=$GCFS_NS
kubectl delete -f "$mydir/manifests/setup_cluster.yaml"
