#!/bin/bash

set -x
set -o nounset
set -o errexit

mydir="$(dirname $0)"

source "$mydir/../common.sh"

kubectl apply -f "$mydir/manifests/csi-driver.yaml"
kubectl apply -f "$mydir/manifests/node.yaml"
kubectl apply -f "$mydir/manifests/controller.yaml"
