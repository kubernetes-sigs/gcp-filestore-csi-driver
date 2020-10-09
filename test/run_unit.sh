#!/bin/bash

set -e
set -x

readonly PKGDIR=sigs.k8s.io/gcp-filestore-csi-driver

go test -mod=vendor -timeout 30s "${PKGDIR}/cmd/..."
go test -mod=vendor -timeout 30s "${PKGDIR}/pkg/..."
