#!/bin/bash

set -o nounset
set -o errexit

readonly PKGDIR=${GOPATH}/src/sigs.k8s.io/gcp-filestore-csi-driver

ginkgo -v -trace "${PKGDIR}/test/e2e/tests" --logtostderr -- --project ${PROJECT} --service-account ${GCFS_IAM_NAME}
