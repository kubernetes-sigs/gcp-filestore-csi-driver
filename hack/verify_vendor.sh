#!/usr/bin/env bash
# Copyright 2026 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PKG_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

die() {
  echo "ERROR: $*" >&2
  exit 1
}

echo "Verifying Go vendor directory"

[[ -f "${PKG_ROOT}/go.mod" ]] || die "go.mod not found in ${PKG_ROOT}"
[[ -d "${PKG_ROOT}/vendor" ]] || die "vendor/ directory not found in ${PKG_ROOT}"

TMP_DIR="$(mktemp -d)"
cleanup() {
  chmod -R u+w "${TMP_DIR}" 2>/dev/null || true
  rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

mkdir -p "${TMP_DIR}/repo"
tar \
  --exclude='./.git' \
  --exclude='./vendor' \
  --exclude='./bin' \
  -C "${PKG_ROOT}" -cf - . | tar -C "${TMP_DIR}/repo" -xf -

GOWORK=off go -C "${TMP_DIR}/repo" mod vendor

if ! diff_out="$(diff -Naupr "${PKG_ROOT}/vendor" "${TMP_DIR}/repo/vendor")"; then
  echo "${diff_out}" >&2
  die "vendor/ is out of sync with go.mod/go.sum. Please run 'go mod tidy && go mod vendor'."
fi
