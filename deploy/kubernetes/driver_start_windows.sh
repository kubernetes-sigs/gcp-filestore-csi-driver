#!/bin/bash

set -x
set -o nounset
set -o errexit

mydir="$(dirname $0)"

source "$mydir/../common.sh"

kubectl apply -f "$mydir/manifests/node_windows.yaml"
kubectl apply -f "$mydir/manifests/csi-driver.yaml"

# The driver has to be started manually until the CSI proxy is implemented.
# See: https://github.com/kubernetes/enhancements/blob/master/keps/sig-windows/20190714-windows-csi-support.md
echo "The sidecar container has been started but the driver has to be manually started on the Windows node(s)."