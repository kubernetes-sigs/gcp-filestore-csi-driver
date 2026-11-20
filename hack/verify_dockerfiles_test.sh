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
VERIFY_SCRIPT="${SCRIPT_DIR}/verify_dockerfiles.sh"

echo "Running verify_dockerfiles.sh unit tests"

TMP_ROOT="$(mktemp -d)"
trap 'rm -rf "${TMP_ROOT}"' EXIT

setup_fixture() {
  local dir="$1"
  mkdir -p \
    "${dir}/hack" \
    "${dir}/.github" \
    "${dir}/cmd/lockrelease" \
    "${dir}/cmd/webhook" \
    "${dir}/vendor/example.com/dep" \
    "${dir}/vendor/example.com/repl"

  cp "${VERIFY_SCRIPT}" "${dir}/hack/verify_dockerfiles.sh"
  chmod +x "${dir}/hack/verify_dockerfiles.sh"

  cat > "${dir}/Dockerfile" << 'EOF'
FROM --platform=$BUILDPLATFORM golang:1.27.4 as builder
FROM gke.gcr.io/debian-base:trixie-v1.0.1-gke.0 as deps
EOF

  cat > "${dir}/cmd/lockrelease/Dockerfile" << 'EOF'
FROM --platform=$BUILDPLATFORM golang:1.27.4 as builder
FROM gcr.io/distroless/static
EOF

  cat > "${dir}/cmd/webhook/Dockerfile" << 'EOF'
FROM --platform=$BUILDPLATFORM golang:1.27.4 as builder
FROM gcr.io/distroless/static
EOF

  cat > "${dir}/.github/dependabot.yml" << 'EOF'
version: 2
updates:
- package-ecosystem: "docker"
  directories:
    - "/"
    - "/cmd/webhook"
    - "/cmd/lockrelease"
  schedule:
    interval: weekly
  groups:
    golang:
      patterns:
        - "golang*"
    distroless:
      patterns:
        - "gcr.io/distroless*"
        - "distroless*"
EOF

  cat > "${dir}/go.mod" << 'EOF'
module example.com/testmod

go 1.26.0

require (
	example.com/dep v1.0.0
	example.com/repl v0.1.0
)

replace example.com/repl => example.com/repl v0.2.0
EOF

  cat > "${dir}/main.go" << 'EOF'
package main

import (
	_ "example.com/dep"
	_ "example.com/repl"
)

func main() {}
EOF

  cat > "${dir}/vendor/example.com/dep/dep.go" << 'EOF'
package dep
EOF

  cat > "${dir}/vendor/example.com/repl/repl.go" << 'EOF'
package repl
EOF

  cat > "${dir}/vendor/modules.txt" << 'EOF'
# example.com/dep v1.0.0
## explicit; go 1.26.0
example.com/dep
# example.com/repl v0.1.0 => example.com/repl v0.2.0
## explicit; go 1.26.0
example.com/repl
# example.com/repl => example.com/repl v0.2.0
EOF
}

expect_pass() {
  local name="$1" dir="$2"
  local out
  if ! out="$("${dir}/hack/verify_dockerfiles.sh" 2>&1)"; then
    echo "FAIL [${name}]: expected pass, got failure:" >&2
    echo "${out}" >&2
    exit 1
  fi
}

expect_fail() {
  local name="$1" dir="$2" substr="$3"
  local out
  if out="$("${dir}/hack/verify_dockerfiles.sh" 2>&1)"; then
    echo "FAIL [${name}]: expected failure containing '${substr}', but command succeeded" >&2
    exit 1
  fi
  if [[ "${out}" != *"${substr}"* ]]; then
    echo "FAIL [${name}]: expected output containing '${substr}', got:" >&2
    echo "${out}" >&2
    exit 1
  fi
}

# 1. Baseline valid fixture passes.
d="${TMP_ROOT}/base"
setup_fixture "${d}"
expect_pass "baseline" "${d}"

# 2. GoB Auto-VC digest syntax, multi-digit patch (1.27.10 >= 1.26), multi-line FROM with # comment,
#    2-part go directive, toolchain, and unused versioned replace summary all pass.
d="${TMP_ROOT}/gob_advanced"
setup_fixture "${d}"
cat > "${d}/Dockerfile" << 'EOF'
FROM --platform=$BUILDPLATFORM \
  # Builder image managed by Auto-VC
  google-go.pkg.dev/golang:1.27.10@sha256:9ca562ec61672333bbf60fdccc6af039ff9aae1eeffecfc5402954674daadc5e as builder
FROM gke.gcr.io/debian-base:forky-v1.0.0-gke.0@sha256:8ad64e1b223d9723dbcfc22aba22db08a711059dd761163c7ac0768a817d98fe as deps
EOF
for sub in cmd/lockrelease/Dockerfile cmd/webhook/Dockerfile; do
  cat > "${d}/${sub}" << 'EOF'
FROM --platform=$BUILDPLATFORM google-go.pkg.dev/golang:1.27.10@sha256:9ca562ec61672333bbf60fdccc6af039ff9aae1eeffecfc5402954674daadc5e as builder
FROM gcr.io/distroless/static
EOF
done
cat > "${d}/go.mod" << 'EOF'
module example.com/testmod

go 1.25

toolchain go1.26.0

require (
	example.com/dep v1.0.0
	example.com/repl v0.1.0
)

replace example.com/repl => example.com/repl v0.2.0
replace example.com/unused v1.0.0 => example.com/unused v1.0.1
EOF
cat >> "${d}/vendor/modules.txt" << 'EOF'
# example.com/unused v1.0.0 => example.com/unused v1.0.1
EOF
expect_pass "gob_multiline_versioned_replace" "${d}"

# 3. Builder version mismatch across Dockerfiles fails.
d="${TMP_ROOT}/mismatch"
setup_fixture "${d}"
cat > "${d}/cmd/webhook/Dockerfile" << 'EOF'
FROM --platform=$BUILDPLATFORM golang:1.27.3 as builder
FROM gcr.io/distroless/static
EOF
expect_fail "builder_mismatch" "${d}" "golang builder tag mismatch"

# 4. Builder version older than go.mod fails.
d="${TMP_ROOT}/older_go"
setup_fixture "${d}"
for df in Dockerfile cmd/lockrelease/Dockerfile cmd/webhook/Dockerfile; do
  sed -i.bak 's/1\.27\.4/1.25.9/g' "${d}/${df}" && rm -f "${d}/${df}.bak"
done
expect_fail "builder_older_than_gomod" "${d}" "older than go.mod 'go 1.26.0'"

# 5. Builder version older than toolchain directive fails.
d="${TMP_ROOT}/older_toolchain"
setup_fixture "${d}"
cat > "${d}/go.mod" << 'EOF'
module example.com/testmod

go 1.26.0

toolchain go1.28rc1

require (
	example.com/dep v1.0.0
	example.com/repl v0.1.0
)

replace example.com/repl => example.com/repl v0.2.0
EOF
expect_fail "builder_older_than_toolchain" "${d}" "older than go.mod 'toolchain go1.28rc1'"

# 6. Debian bookworm, legacy v2.0.0, and floating latest tags all fail.
for bad_tag in "bookworm-v1.0.8-gke.12" "v2.0.0" "latest"; do
  d="${TMP_ROOT}/deb_${bad_tag}"
  setup_fixture "${d}"
  sed -i.bak "s/trixie-v1\.0\.1-gke\.0/${bad_tag}/g" "${d}/Dockerfile" && rm -f "${d}/Dockerfile.bak"
  expect_fail "debian_${bad_tag}" "${d}" "disallowed or EOL Debian base image tag"
done

# 7. Missing directory in .github/dependabot.yml fails.
d="${TMP_ROOT}/dep_missing_dir"
setup_fixture "${d}"
sed -i.bak '/cmd\/lockrelease/d' "${d}/.github/dependabot.yml" && rm -f "${d}/.github/dependabot.yml.bak"
expect_fail "dependabot_missing_dir" "${d}" "missing '/cmd/lockrelease'"

# 8. Out-of-sync vendor/modules.txt fails.
d="${TMP_ROOT}/vendor_drift"
setup_fixture "${d}"
sed -i.bak 's/v1\.0\.0/v1.0.1/g' "${d}/vendor/modules.txt" && rm -f "${d}/vendor/modules.txt.bak"
expect_fail "vendor_modules_drift" "${d}" "do not match vendor/modules.txt"

# 9. Missing vendored .go source file fails (-deps -test).
d="${TMP_ROOT}/missing_vendor_src"
setup_fixture "${d}"
rm -f "${d}/vendor/example.com/dep/dep.go"
expect_fail "missing_vendor_go_file" "${d}" "example.com/dep"
