#!/bin/bash

set -e
set -x

readonly PKGDIR=sigs.k8s.io/gcp-filestore-csi-driver

go test -v -mod=vendor -timeout 30s "${PKGDIR}/test/sanity/" -run ^TestSanity$
