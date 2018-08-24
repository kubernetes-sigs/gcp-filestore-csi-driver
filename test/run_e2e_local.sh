#!/bin/bash

set -o nounset
set -o errexit

readonly PKGDIR=sigs.k8s.io/gcp-filestore-csi-driver

ginkgo -v "e2e/tests" --logtostderr -- --project ${PROJECT} --service-account ${IAM_NAME}