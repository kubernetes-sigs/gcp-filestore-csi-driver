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
VERIFY_VENDOR_SCRIPT="${SCRIPT_DIR}/verify_vendor.sh"

echo "Running verify_vendor.sh unit tests"

TMP_ROOT="$(mktemp -d)"
trap 'chmod -R u+w "${TMP_ROOT}" 2>/dev/null || true; rm -rf "${TMP_ROOT}"' EXIT

sed_inplace() {
  local expr="$1"
  local file="$2"
  sed -i.bak "${expr}" "${file}" && rm -f "${file}.bak"
}

# Build a self-contained module using a local relative replace target so
# `go mod vendor` runs completely offline during unit tests.
setup_vendor_fixture() {
  local fixture_dir="$1"
  mkdir -p \
    "${fixture_dir}/hack" \
    "${fixture_dir}/_testdeps/dep"

  cp "${VERIFY_VENDOR_SCRIPT}" "${fixture_dir}/hack/verify_vendor.sh"
  chmod +x "${fixture_dir}/hack/verify_vendor.sh"

  cat > "${fixture_dir}/_testdeps/dep/go.mod" << 'EOF'
module example.com/dep

go 1.25.0
EOF

  cat > "${fixture_dir}/_testdeps/dep/dep.go" << 'EOF'
package dep

const Name = "clean"
EOF

  cat > "${fixture_dir}/go.mod" << 'EOF'
module example.com/testmod

go 1.25.0

require example.com/dep v1.0.0

replace example.com/dep => ./_testdeps/dep
EOF

  cat > "${fixture_dir}/main.go" << 'EOF'
package main

import _ "example.com/dep"

func main() {}
EOF

  GOWORK=off go -C "${fixture_dir}" mod vendor
}

expect_pass() {
  local name="$1" fixture_dir="$2"
  local out
  if ! out="$("${fixture_dir}/hack/verify_vendor.sh" 2>&1)"; then
    echo "FAIL [${name}]: expected pass, got failure:" >&2
    echo "${out}" >&2
    exit 1
  fi
}

expect_fail() {
  local name="$1" fixture_dir="$2" substr="$3"
  local out
  if out="$("${fixture_dir}/hack/verify_vendor.sh" 2>&1)"; then
    echo "FAIL [${name}]: expected failure containing '${substr}', but command succeeded" >&2
    exit 1
  fi
  if [[ "${out}" != *"${substr}"* ]]; then
    echo "FAIL [${name}]: expected output containing '${substr}', got:" >&2
    echo "${out}" >&2
    exit 1
  fi
}

# 1. Clean vendored tree passes.
fixture_dir="${TMP_ROOT}/clean"
setup_vendor_fixture "${fixture_dir}"
expect_pass "clean_vendor" "${fixture_dir}"

# 2. Edited vendored .go file fails.
fixture_dir="${TMP_ROOT}/edited_go"
setup_vendor_fixture "${fixture_dir}"
sed_inplace 's/"clean"/"tampered"/' "${fixture_dir}/vendor/example.com/dep/dep.go"
expect_fail "edited_vendored_go_file" "${fixture_dir}" "vendor/ is out of sync"

# 3. Stale/untracked leftover file in vendor/ fails.
fixture_dir="${TMP_ROOT}/stale_file"
setup_vendor_fixture "${fixture_dir}"
echo "package dep" > "${fixture_dir}/vendor/example.com/dep/stale.go"
expect_fail "stale_vendored_file" "${fixture_dir}" "stale.go"

# 4. Out-of-sync vendor/modules.txt fails.
fixture_dir="${TMP_ROOT}/modules_txt_drift"
setup_vendor_fixture "${fixture_dir}"
sed_inplace 's/v1\.0\.0/v1.0.1/' "${fixture_dir}/vendor/modules.txt"
expect_fail "modules_txt_drift" "${fixture_dir}" "vendor/ is out of sync"

# 5. Missing vendored .go source file fails.
fixture_dir="${TMP_ROOT}/missing_src"
setup_vendor_fixture "${fixture_dir}"
rm -f "${fixture_dir}/vendor/example.com/dep/dep.go"
expect_fail "missing_vendored_go_file" "${fixture_dir}" "dep.go"
