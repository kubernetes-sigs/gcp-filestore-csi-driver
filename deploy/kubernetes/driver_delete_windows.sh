#!/bin/bash

set -x
set -o nounset
set -o errexit

mydir="$(dirname $0)"

source "$mydir/../common.sh"

kubectl delete -f "$mydir/manifests/node_windows.yaml"
kubectl delete -f "$mydir/manifests/csi-driver.yaml"
