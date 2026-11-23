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

sed_inplace() {
  local expr="$1"
  local file="$2"
  sed -i.bak "${expr}" "${file}" && rm -f "${file}.bak"
}

setup_fixture() {
  local fixture_dir="$1"
  mkdir -p \
    "${fixture_dir}/hack" \
    "${fixture_dir}/.github" \
    "${fixture_dir}/cmd/lockrelease" \
    "${fixture_dir}/cmd/webhook" \
    "${fixture_dir}/test/experimental"

  cp "${VERIFY_SCRIPT}" "${fixture_dir}/hack/verify_dockerfiles.sh"
  chmod +x "${fixture_dir}/hack/verify_dockerfiles.sh"

  cat > "${fixture_dir}/Dockerfile" << 'EOF'
FROM --platform=$BUILDPLATFORM golang:1.27.4 as builder
FROM gke.gcr.io/debian-base:trixie-v1.0.1-gke.0 as deps
EOF

  cat > "${fixture_dir}/cmd/lockrelease/Dockerfile" << 'EOF'
FROM --platform=$BUILDPLATFORM golang:1.27.4 as builder
FROM gcr.io/distroless/static
EOF

  cat > "${fixture_dir}/cmd/webhook/Dockerfile" << 'EOF'
FROM --platform=$BUILDPLATFORM golang:1.27.4 as builder
FROM gcr.io/distroless/static
EOF

  # Non-Go Windows Dockerfile should be ignored by golang builder discovery.
  cat > "${fixture_dir}/test/experimental/Dockerfile" << 'EOF'
FROM mcr.microsoft.com/windows/servercore:1809 as core
FROM mcr.microsoft.com/windows/nanoserver:1809
EOF

  cat > "${fixture_dir}/.github/dependabot.yml" << 'EOF'
version: 2
updates:
- package-ecosystem: "docker"
  directories:
    - "/"
    - "/cmd/webhook"
    - "/cmd/lockrelease"
  groups:
    golang:
      patterns:
        - "golang*"
      group-by: dependency-name
- package-ecosystem: gomod # not docker
  directory: "/"
EOF

  cat > "${fixture_dir}/go.mod" << 'EOF'
module example.com/testmod

go 1.26.0
EOF
}

expect_pass() {
  local name="$1" fixture_dir="$2"
  local out
  if ! out="$("${fixture_dir}/hack/verify_dockerfiles.sh" 2>&1)"; then
    echo "FAIL [${name}]: expected pass, got failure:" >&2
    echo "${out}" >&2
    exit 1
  fi
}

expect_fail() {
  local name="$1" fixture_dir="$2" substr="$3"
  local out
  if out="$("${fixture_dir}/hack/verify_dockerfiles.sh" 2>&1)"; then
    echo "FAIL [${name}]: expected failure containing '${substr}', but command succeeded" >&2
    exit 1
  fi
  if [[ "${out}" != *"${substr}"* ]]; then
    echo "FAIL [${name}]: expected output containing '${substr}', got:" >&2
    echo "${out}" >&2
    exit 1
  fi
}

# 1. Baseline valid fixture (including non-Go Windows Dockerfile and '# not docker' comment) passes.
fixture_dir="${TMP_ROOT}/base"
setup_fixture "${fixture_dir}"
expect_pass "baseline" "${fixture_dir}"

# 2. GoB Auto-VC digest syntax, multi-digit patch (1.27.10 >= 1.26), multi-line FROM with # comment,
#    2-part go directive, toolchain, single-quoted 'docker', and flow-style directories: [...] pass.
fixture_dir="${TMP_ROOT}/gob_advanced"
setup_fixture "${fixture_dir}"
cat > "${fixture_dir}/Dockerfile" << 'EOF'
FROM --platform=$BUILDPLATFORM \
  # Builder image managed by Auto-VC
  google-go.pkg.dev/golang:1.27.10@sha256:9ca562ec61672333bbf60fdccc6af039ff9aae1eeffecfc5402954674daadc5e as builder
FROM gke.gcr.io/debian-base:forky-v1.0.0-gke.0@sha256:8ad64e1b223d9723dbcfc22aba22db08a711059dd761163c7ac0768a817d98fe as deps
EOF
for sub in cmd/lockrelease/Dockerfile cmd/webhook/Dockerfile; do
  cat > "${fixture_dir}/${sub}" << 'EOF'
FROM --platform=$BUILDPLATFORM google-go.pkg.dev/golang:1.27.10@sha256:9ca562ec61672333bbf60fdccc6af039ff9aae1eeffecfc5402954674daadc5e as builder
FROM gcr.io/distroless/static
EOF
done
cat > "${fixture_dir}/.github/dependabot.yml" << 'EOF'
version: 2
updates:
- package-ecosystem: 'docker'
  directories: ["/", "/cmd/webhook", "/cmd/lockrelease"]
- package-ecosystem: gomod # comment ending in docker
  directory: "/"
EOF
cat > "${fixture_dir}/go.mod" << 'EOF'
module example.com/testmod

go 1.25

toolchain go1.26.0
EOF
expect_pass "gob_multiline_flow_dirs_single_quote" "${fixture_dir}"

# 3. Builder version mismatch across Dockerfiles fails.
fixture_dir="${TMP_ROOT}/mismatch"
setup_fixture "${fixture_dir}"
cat > "${fixture_dir}/cmd/webhook/Dockerfile" << 'EOF'
FROM --platform=$BUILDPLATFORM golang:1.27.3 as builder
FROM gcr.io/distroless/static
EOF
expect_fail "builder_mismatch" "${fixture_dir}" "golang builder tag mismatch"

# 4. Floating golang:latest and 2-part golang:1.27 builder tags fail.
fixture_dir="${TMP_ROOT}/golang_latest"
setup_fixture "${fixture_dir}"
for df in Dockerfile cmd/lockrelease/Dockerfile cmd/webhook/Dockerfile; do
  sed_inplace 's/1\.27\.4/latest/g' "${fixture_dir}/${df}"
done
expect_fail "golang_latest" "${fixture_dir}" "unpinned or unparseable golang builder tag 'latest'"

fixture_dir="${TMP_ROOT}/golang_two_part"
setup_fixture "${fixture_dir}"
for df in Dockerfile cmd/lockrelease/Dockerfile cmd/webhook/Dockerfile; do
  sed_inplace 's/1\.27\.4/1.27/g' "${fixture_dir}/${df}"
done
expect_fail "golang_two_part" "${fixture_dir}" "must pin a full major.minor.patch version"

# 5. Builder version older than go.mod fails with actionable guidance to bump all Dockerfiles.
fixture_dir="${TMP_ROOT}/older_go"
setup_fixture "${fixture_dir}"
for df in Dockerfile cmd/lockrelease/Dockerfile cmd/webhook/Dockerfile; do
  sed_inplace 's/1\.27\.4/1.25.9/g' "${fixture_dir}/${df}"
done
expect_fail "builder_older_than_gomod" "${fixture_dir}" "Bump FROM golang:<version> across all Linux Dockerfiles"

# 6. Builder version older than toolchain directive fails.
fixture_dir="${TMP_ROOT}/older_toolchain"
setup_fixture "${fixture_dir}"
cat > "${fixture_dir}/go.mod" << 'EOF'
module example.com/testmod

go 1.26.0

toolchain go1.28rc1
EOF
expect_fail "builder_older_than_toolchain" "${fixture_dir}" "older than go.mod 'toolchain go1.28rc1'"

# 7. EOL Debian (bookworm), floating suites (latest, stable, sid, testing), and legacy/bare tags fail.
fixture_dir="${TMP_ROOT}/deb_bookworm"
setup_fixture "${fixture_dir}"
sed_inplace 's/trixie-v1\.0\.1-gke\.0/bookworm-v1.0.8-gke.12/g' "${fixture_dir}/Dockerfile"
expect_fail "debian_bookworm" "${fixture_dir}" "EOL Debian base image tag"

for floating_tag in "latest" "stable" "sid" "testing"; do
  fixture_dir="${TMP_ROOT}/deb_${floating_tag}"
  setup_fixture "${fixture_dir}"
  sed_inplace "s/trixie-v1\.0\.1-gke\.0/${floating_tag}/g" "${fixture_dir}/Dockerfile"
  expect_fail "debian_${floating_tag}" "${fixture_dir}" "floating Debian base image tag"
done

for unpinned_tag in "v2.0.0" "trixie"; do
  fixture_dir="${TMP_ROOT}/deb_${unpinned_tag}"
  setup_fixture "${fixture_dir}"
  sed_inplace "s/trixie-v1\.0\.1-gke\.0/${unpinned_tag}/g" "${fixture_dir}/Dockerfile"
  expect_fail "debian_${unpinned_tag}" "${fixture_dir}" "unpinned or legacy Debian base image tag"
done

# 8. Old three-block dependabot.yml regression fails.
fixture_dir="${TMP_ROOT}/dep_three_blocks"
setup_fixture "${fixture_dir}"
cat > "${fixture_dir}/.github/dependabot.yml" << 'EOF'
version: 2
updates:
- package-ecosystem: "docker"
  directory: "/"
- package-ecosystem: "docker"
  directory: "/cmd/webhook"
- package-ecosystem: "docker"
  directory: "/cmd/lockrelease"
EOF
expect_fail "dependabot_three_blocks" "${fixture_dir}" 'must define exactly 1 package-ecosystem: "docker" block (found 3)'

# 9. Missing directory in .github/dependabot.yml fails.
fixture_dir="${TMP_ROOT}/dep_missing_dir"
setup_fixture "${fixture_dir}"
sed_inplace '/cmd\/lockrelease/d' "${fixture_dir}/.github/dependabot.yml"
expect_fail "dependabot_missing_dir" "${fixture_dir}" "missing '/cmd/lockrelease'"

# 10. Adding a fourth Dockerfile with a golang stage (tested in a git repo) flags both tag mismatch and missing dependabot dir.
fixture_dir="${TMP_ROOT}/fourth_dockerfile"
setup_fixture "${fixture_dir}"
mkdir -p "${fixture_dir}/cmd/extra"
cat > "${fixture_dir}/cmd/extra/Dockerfile" << 'EOF'
FROM golang:1.27.4 as builder
FROM gcr.io/distroless/static
EOF
git -C "${fixture_dir}" init -q
expect_fail "fourth_dockerfile_missing_dependabot" "${fixture_dir}" "missing '/cmd/extra'"

cat > "${fixture_dir}/cmd/extra/Dockerfile" << 'EOF'
FROM golang:1.27.3 as builder
FROM gcr.io/distroless/static
EOF
expect_fail "fourth_dockerfile_tag_mismatch" "${fixture_dir}" "golang builder tag mismatch"

cat > "${fixture_dir}/cmd/extra/Dockerfile" << 'EOF'
FROM golang:1.27.4 as builder
FROM gcr.io/distroless/static
EOF
sed_inplace 's|- "/cmd/lockrelease"|- "/cmd/lockrelease"\
    - "/cmd/extra"|' "${fixture_dir}/.github/dependabot.yml"
expect_pass "fourth_dockerfile_valid" "${fixture_dir}"
