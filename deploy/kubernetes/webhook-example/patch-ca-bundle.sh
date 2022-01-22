#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

export CA_BUNDLE=$(kubectl config view --raw -o json | jq '.clusters[]' | jq "select(.name == \"$(kubectl config current-context)\")" | jq '.cluster."certificate-authority-data"')

if command -v envsubst >/dev/null 2>&1; then
    envsubst
else
    sed -e "s|\${CA_BUNDLE}|${CA_BUNDLE}|g"
fi