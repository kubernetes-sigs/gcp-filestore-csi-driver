#!/bin/bash

set -e
set -x

readonly PKGDIR=sigs.k8s.io/gcp-filestore-csi-driver

go test --timeout 20m --v=true "${PKGDIR}/test/e2e/tests" --logtostderr --run-in-prow=true --delete-instances=true
