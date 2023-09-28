#!/bin/bash

set -o nounset
set -o errexit

readonly PKGDIR=${GOPATH}/src/github.com/kubernetes-sigs/gcp-filestore-csi-driver

ginkgo -v -trace "${PKGDIR}/test/e2e/tests" --logtostderr -- --project ${PROJECT} --service-account ${GCFS_IAM_NAME}
