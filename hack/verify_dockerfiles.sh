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

echo "Verifying Dockerfiles, Dependabot config, and vendor consistency"

# Returns 0 if version $1 >= version $2 (numeric major.minor[.patch] comparison).
version_ge() {
  local v1="$1" v2="$2"
  local m1 n1 p1 m2 n2 p2
  IFS='.' read -r m1 n1 p1 <<< "${v1}"
  IFS='.' read -r m2 n2 p2 <<< "${v2}"
  p1="${p1:-0}"
  p2="${p2:-0}"
  if (( 10#${m1} != 10#${m2} )); then
    if (( 10#${m1} > 10#${m2} )); then
      return 0
    fi
    return 1
  fi
  if (( 10#${n1} != 10#${n2} )); then
    if (( 10#${n1} > 10#${n2} )); then
      return 0
    fi
    return 1
  fi
  if (( 10#${p1} >= 10#${p2} )); then
    return 0
  fi
  return 1
}

# Extract the golang builder tag (stripping any registry prefix like
# google-go.pkg.dev/ and any @sha256:... digest suffix) from FROM instructions.
extract_golang_tags() {
  local dockerfile="$1"
  awk '
    /^[[:space:]]*#/ { next }
    /\\[[:space:]]*$/ {
      sub(/\\[[:space:]]*$/, " ")
      line = line $0
      next
    }
    {
      $0 = line $0
      line = ""
    }
    /^[[:space:]]*[Ff][Rr][Oo][Mm][[:space:]]/ {
      for (i = 2; i <= NF; i++) {
        if ($i ~ /^([^\/]+\/)*golang:[^\/]+$/) {
          tag = $i
          sub(/^.*golang:/, "", tag)
          sub(/@.*/, "", tag)
          print tag
        }
      }
    }
  ' "${dockerfile}"
}

DOCKERFILES=(
  "Dockerfile"
  "cmd/lockrelease/Dockerfile"
  "cmd/webhook/Dockerfile"
)

expected_tag=""
expected_file=""
builder_go_version=""
for rel_path in "${DOCKERFILES[@]}"; do
  full_path="${PKG_ROOT}/${rel_path}"
  if [[ ! -f "${full_path}" ]]; then
    echo "ERROR: expected Dockerfile not found: ${rel_path}" >&2
    exit 1
  fi

  tags=()
  while IFS= read -r tag; do
    if [[ -n "${tag}" ]]; then
      tags+=("${tag}")
    fi
  done < <(extract_golang_tags "${full_path}")

  if [[ "${#tags[@]}" -eq 0 ]]; then
    echo "ERROR: could not find a golang:<tag> builder FROM stage in ${rel_path}" >&2
    exit 1
  fi

  for tag in "${tags[@]}"; do
    if [[ ! "${tag}" =~ ^([0-9]+\.[0-9]+(\.[0-9]+)?)([-+].*)?$ ]]; then
      echo "ERROR: unparseable golang builder tag '${tag}' in ${rel_path}" >&2
      exit 1
    fi
    parsed_ver="${BASH_REMATCH[1]}"
    if [[ -z "${expected_tag}" ]]; then
      expected_tag="${tag}"
      expected_file="${rel_path}"
      builder_go_version="${parsed_ver}"
    elif [[ "${tag}" != "${expected_tag}" ]]; then
      echo "ERROR: golang builder tag mismatch: ${expected_file} uses 'golang:${expected_tag}', but ${rel_path} uses 'golang:${tag}'" >&2
      echo "All Linux Dockerfiles (${DOCKERFILES[*]}) must use the same golang builder version." >&2
      exit 1
    fi
  done
done

gomod_path="${PKG_ROOT}/go.mod"
gomod_go_version="$(awk '$1 == "go" && NF >= 2 { print $2; exit }' "${gomod_path}")"
if [[ -z "${gomod_go_version}" || ! "${gomod_go_version}" =~ ^[0-9]+\.[0-9]+(\.[0-9]+)?$ ]]; then
  echo "ERROR: could not parse 'go <version>' directive from go.mod (got '${gomod_go_version}')" >&2
  exit 1
fi

if ! version_ge "${builder_go_version}" "${gomod_go_version}"; then
  echo "ERROR: Dockerfile golang builder version (${builder_go_version}) is older than go.mod 'go ${gomod_go_version}'" >&2
  exit 1
fi

gomod_toolchain_raw="$(awk '$1 == "toolchain" && NF >= 2 { print $2; exit }' "${gomod_path}")"
if [[ -n "${gomod_toolchain_raw}" && "${gomod_toolchain_raw}" != "default" ]]; then
  if [[ ! "${gomod_toolchain_raw}" =~ ^go([0-9]+\.[0-9]+(\.[0-9]+)?)([-a-zA-Z0-9.]*)?$ ]]; then
    echo "ERROR: could not parse 'toolchain ${gomod_toolchain_raw}' directive from go.mod" >&2
    exit 1
  fi
  gomod_toolchain_ver="${BASH_REMATCH[1]}"
  if ! version_ge "${builder_go_version}" "${gomod_toolchain_ver}"; then
    echo "ERROR: Dockerfile golang builder version (${builder_go_version}) is older than go.mod 'toolchain ${gomod_toolchain_raw}'" >&2
    exit 1
  fi
fi

# Verify root Dockerfile runtime base image does not regress to EOL Debian
# codenames (wheezy..bookworm), legacy pre-codename v* tags, or floating latest.
debian_tags=()
while IFS= read -r deb_tag; do
  if [[ -n "${deb_tag}" ]]; then
    debian_tags+=("${deb_tag}")
  fi
done < <(
  awk '
    /^[[:space:]]*#/ { next }
    /\\[[:space:]]*$/ {
      sub(/\\[[:space:]]*$/, " ")
      line = line $0
      next
    }
    {
      $0 = line $0
      line = ""
    }
    /^[[:space:]]*[Ff][Rr][Oo][Mm][[:space:]]/ {
      for (i = 2; i <= NF; i++) {
        if ($i ~ /^([^\/]+\/)*debian-base:[^\/]+$/) {
          tag = $i
          sub(/^.*debian-base:/, "", tag)
          sub(/@.*/, "", tag)
          print tag
        }
      }
    }
  ' "${PKG_ROOT}/Dockerfile"
)

if [[ "${#debian_tags[@]}" -eq 0 ]]; then
  echo "ERROR: could not find a debian-base:<tag> runtime FROM stage in Dockerfile" >&2
  exit 1
fi

for deb_tag in "${debian_tags[@]}"; do
  if [[ "${deb_tag}" =~ ^(wheezy|jessie|stretch|buster|bullseye|bookworm|latest)([-._]|$) || "${deb_tag}" =~ ^v[0-9] || ! "${deb_tag}" =~ ^[a-z] ]]; then
    echo "ERROR: Dockerfile uses disallowed or EOL Debian base image tag 'debian-base:${deb_tag}'; a pinned Debian 13 (trixie) or newer codename tag is required" >&2
    exit 1
  fi
done

# Verify .github/dependabot.yml groups all Linux Dockerfile directories under a
# single package-ecosystem: "docker" entry so Dependabot updates stay in sync.
dependabot_path="${PKG_ROOT}/.github/dependabot.yml"
if [[ ! -f "${dependabot_path}" ]]; then
  echo "ERROR: ${dependabot_path} not found" >&2
  exit 1
fi

docker_ecosystems_count="$(
  awk '
    /^[[:space:]]*-[[:space:]]*package-ecosystem:/ && $0 ~ /"?docker"?([[:space:]]*(#.*)?$)/ { count++ }
    END { print count + 0 }
  ' "${dependabot_path}"
)"
if [[ "${docker_ecosystems_count}" -ne 1 ]]; then
  echo "ERROR: .github/dependabot.yml must define exactly 1 package-ecosystem: \"docker\" block (found ${docker_ecosystems_count})" >&2
  exit 1
fi

dependabot_dirs="$(
  awk '
    /^[[:space:]]*-[[:space:]]*package-ecosystem:/ {
      in_docker = ($0 ~ /"?docker"?([[:space:]]*(#.*)?$)/)
      in_dirs = 0
      next
    }
    in_docker && /^[[:space:]]*directories:[[:space:]]*(#.*)?$/ {
      in_dirs = 1
      next
    }
    in_docker && in_dirs {
      if ($0 ~ /^[[:space:]]*-[[:space:]]*/) {
        val = $0
        sub(/^[[:space:]]*-[[:space:]]*/, "", val)
        sub(/[[:space:]]*(#.*)?$/, "", val)
        gsub(/^["'\'']|["'\'']$/, "", val)
        print val
        next
      }
      if ($0 !~ /^[[:space:]]*(#.*)?$/) {
        in_dirs = 0
      }
    }
  ' "${dependabot_path}"
)"

for rel_path in "${DOCKERFILES[@]}"; do
  dir_part="$(dirname "${rel_path}")"
  if [[ "${dir_part}" == "." ]]; then
    expected_dir="/"
  else
    expected_dir="/${dir_part}"
  fi
  if ! grep -Fxq "${expected_dir}" <<< "${dependabot_dirs}"; then
    echo "ERROR: .github/dependabot.yml docker 'directories:' is missing '${expected_dir}' (for ${rel_path})" >&2
    exit 1
  fi
done

# Verify go.mod require entries and vendor/modules.txt are in sync offline
# (without contacting proxy.golang.org).
gomod_reqs="$(
  awk '
    $1 == "require" && $2 == "(" { in_req = 1; next }
    in_req && $1 == ")" { in_req = 0; next }
    $1 == "require" && $2 != "(" { print $2, $3; next }
    in_req && $1 !~ /^\/\// && NF >= 2 { print $1, $2 }
  ' "${gomod_path}" | LC_ALL=C sort
)"

vendor_modules_path="${PKG_ROOT}/vendor/modules.txt"
if [[ ! -f "${vendor_modules_path}" ]]; then
  echo "ERROR: vendor/modules.txt not found" >&2
  exit 1
fi

vendor_reqs="$(
  awk '
    /^# / {
      if ($3 == "=>" || NF < 3) {
        pending = ""
        next
      }
      if ($4 == "=>") {
        pending = $2 " " $3
        next
      }
      pending = ""
      print $2, $3
      next
    }
    pending != "" && (/^## / || /^[^#[:space:]]/) {
      print pending
      pending = ""
    }
  ' "${vendor_modules_path}" | LC_ALL=C sort
)"

if [[ "${gomod_reqs}" != "${vendor_reqs}" ]]; then
  echo "ERROR: go.mod require directives do not match vendor/modules.txt:" >&2
  diff -u <(echo "${gomod_reqs}") <(echo "${vendor_reqs}") >&2 || true
  echo "Please run 'go mod tidy && go mod vendor'." >&2
  exit 1
fi

(
  cd "${PKG_ROOT}"
  GOPROXY=off GOWORK=off go list -mod=vendor -deps -test ./... >/dev/null
)
