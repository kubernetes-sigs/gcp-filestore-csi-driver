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

readonly REQUIRED_GOLANG_DOCKERFILES=(
  "Dockerfile"
  "cmd/lockrelease/Dockerfile"
  "cmd/webhook/Dockerfile"
)

# Update EOL_DEBIAN_CODENAMES when the driver migrates to a newer Debian
# release (e.g., add "trixie" once Debian 13 is superseded by "forky").
readonly EOL_DEBIAN_CODENAMES="wheezy|jessie|stretch|buster|bullseye|bookworm"
readonly FLOATING_DEBIAN_SUITES="latest|stable|oldstable|oldoldstable|testing|unstable|sid|experimental"

die() {
  echo "ERROR: $*" >&2
  exit 1
}

# Returns 0 if version $1 >= version $2 (numeric major.minor[.patch] comparison;
# go.mod 'go' directives may be 2-part major.minor per Go 1.21+).
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

# Extract tags for <image> (stripping any registry prefix like google-go.pkg.dev/
# and any @sha256:... digest suffix) from FROM instructions in <dockerfile>.
# Emits "<untagged>" if the image is referenced without a ':tag'.
extract_image_tags() {
  local dockerfile="$1"
  local image="$2"
  awk -v img="${image}" '
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
        if ($i ~ /^--/) { continue }
        if (tolower($i) == "as") { break }
        ref = $i
        sub(/@.*/, "", ref)
        n = split(ref, parts, "/")
        last = parts[n]
        colon = index(last, ":")
        if (colon > 0) {
          base = substr(last, 1, colon - 1)
          tag = substr(last, colon + 1)
        } else {
          base = last
          tag = "<untagged>"
        }
        if (base == img) {
          print tag
        }
        break
      }
    }
  ' "${dockerfile}"
}

# Populate GOLANG_DOCKERFILES with REQUIRED_GOLANG_DOCKERFILES plus any other
# non-vendored Dockerfile in the repository that contains a golang FROM stage
# (skipping non-Go Dockerfiles such as test/experimental/Dockerfile).
GOLANG_DOCKERFILES=()
discover_golang_dockerfiles() {
  local rel_path candidate
  GOLANG_DOCKERFILES=("${REQUIRED_GOLANG_DOCKERFILES[@]}")

  local candidates=()
  if git -C "${PKG_ROOT}" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    while IFS= read -r candidate; do
      if [[ -n "${candidate}" ]]; then
        candidates+=("${candidate}")
      fi
    done < <(git -C "${PKG_ROOT}" ls-files --cached --others --exclude-standard 'Dockerfile' '**/Dockerfile' ':!vendor')
  else
    while IFS= read -r candidate; do
      if [[ -n "${candidate}" ]]; then
        candidates+=("${candidate}")
      fi
    done < <(cd "${PKG_ROOT}" && find . \( -path './vendor' -o -path './.git' \) -prune -o -name 'Dockerfile' -type f -print | sed 's|^\./||' | LC_ALL=C sort)
  fi

  for candidate in "${candidates[@]}"; do
    local already_listed=0
    for rel_path in "${GOLANG_DOCKERFILES[@]}"; do
      if [[ "${candidate}" == "${rel_path}" ]]; then
        already_listed=1
        break
      fi
    done
    if [[ "${already_listed}" -eq 1 || ! -f "${PKG_ROOT}/${candidate}" ]]; then
      continue
    fi
    if [[ -n "$(extract_image_tags "${PKG_ROOT}/${candidate}" "golang")" ]]; then
      GOLANG_DOCKERFILES+=("${candidate}")
    fi
  done
}

verify_golang_builders() {
  local expected_tag=""
  local expected_file=""
  local builder_go_version=""
  local rel_path full_path tag parsed_ver

  for rel_path in "${GOLANG_DOCKERFILES[@]}"; do
    full_path="${PKG_ROOT}/${rel_path}"
    [[ -f "${full_path}" ]] || die "expected Dockerfile not found: ${rel_path}"

    local tags=()
    while IFS= read -r tag; do
      if [[ -n "${tag}" ]]; then
        tags+=("${tag}")
      fi
    done < <(extract_image_tags "${full_path}" "golang")

    if [[ "${#tags[@]}" -eq 0 ]]; then
      die "could not find a golang:<tag> builder FROM stage in ${rel_path}"
    fi

    for tag in "${tags[@]}"; do
      if [[ "${tag}" =~ ^[0-9]+\.[0-9]+([-+].*)?$ ]]; then
        die "Dockerfile golang builder tag 'golang:${tag}' in ${rel_path} must pin a full major.minor.patch version (e.g., golang:1.27.4)"
      fi
      if [[ ! "${tag}" =~ ^([0-9]+\.[0-9]+\.[0-9]+)([-+].*)?$ ]]; then
        die "unpinned or unparseable golang builder tag '${tag}' in ${rel_path} (expected major.minor.patch, e.g., golang:1.27.4)"
      fi
      parsed_ver="${BASH_REMATCH[1]}"
      if [[ -z "${expected_tag}" ]]; then
        expected_tag="${tag}"
        expected_file="${rel_path}"
        builder_go_version="${parsed_ver}"
      elif [[ "${tag}" != "${expected_tag}" ]]; then
        die "golang builder tag mismatch: ${expected_file} uses 'golang:${expected_tag}', but ${rel_path} uses 'golang:${tag}'. All Go Dockerfiles (${GOLANG_DOCKERFILES[*]}) must use the same golang builder version."
      fi
    done
  done

  local gomod_path="${PKG_ROOT}/go.mod"
  [[ -f "${gomod_path}" ]] || die "go.mod not found at ${gomod_path}"

  local gomod_go_version
  gomod_go_version="$(awk '$1 == "go" && NF >= 2 { print $2; exit }' "${gomod_path}")"
  if [[ -z "${gomod_go_version}" || ! "${gomod_go_version}" =~ ^[0-9]+\.[0-9]+(\.[0-9]+)?$ ]]; then
    die "could not parse 'go <version>' directive from go.mod (got '${gomod_go_version}')"
  fi

  if ! version_ge "${builder_go_version}" "${gomod_go_version}"; then
    die "Dockerfile golang builder version (${builder_go_version}) is older than go.mod 'go ${gomod_go_version}'. Bump FROM golang:<version> across all Linux Dockerfiles (${GOLANG_DOCKERFILES[*]}) to >= ${gomod_go_version} first (or in the same PR)."
  fi

  local gomod_toolchain_raw gomod_toolchain_ver
  gomod_toolchain_raw="$(awk '$1 == "toolchain" && NF >= 2 { print $2; exit }' "${gomod_path}")"
  if [[ -n "${gomod_toolchain_raw}" && "${gomod_toolchain_raw}" != "default" ]]; then
    if [[ ! "${gomod_toolchain_raw}" =~ ^go([0-9]+\.[0-9]+(\.[0-9]+)?)([-a-zA-Z0-9.]*)?$ ]]; then
      die "could not parse 'toolchain ${gomod_toolchain_raw}' directive from go.mod"
    fi
    gomod_toolchain_ver="${BASH_REMATCH[1]}"
    if ! version_ge "${builder_go_version}" "${gomod_toolchain_ver}"; then
      die "Dockerfile golang builder version (${builder_go_version}) is older than go.mod 'toolchain ${gomod_toolchain_raw}'. Bump FROM golang:<version> across all Linux Dockerfiles (${GOLANG_DOCKERFILES[*]}) to >= ${gomod_toolchain_ver} first (or in the same PR)."
    fi
  fi
}

verify_debian_runtime_base() {
  local dockerfile="${PKG_ROOT}/Dockerfile"
  local debian_tags=()
  local deb_tag

  while IFS= read -r deb_tag; do
    if [[ -n "${deb_tag}" ]]; then
      debian_tags+=("${deb_tag}")
    fi
  done < <(extract_image_tags "${dockerfile}" "debian-base")

  if [[ "${#debian_tags[@]}" -eq 0 ]]; then
    die "could not find a debian-base:<tag> runtime FROM stage in Dockerfile"
  fi

  for deb_tag in "${debian_tags[@]}"; do
    if [[ "${deb_tag}" =~ ^(${EOL_DEBIAN_CODENAMES})([-._]|$) ]]; then
      die "Dockerfile uses EOL Debian base image tag 'debian-base:${deb_tag}'; Debian 13 (trixie) or newer is required"
    fi
    if [[ "${deb_tag}" == "<untagged>" || "${deb_tag}" =~ ^(${FLOATING_DEBIAN_SUITES})([-._]|$) ]]; then
      die "Dockerfile uses floating Debian base image tag 'debian-base:${deb_tag}'; pin a '<codename>-v<version>' tag (e.g., trixie-v1.0.1-gke.0)"
    fi
    if [[ ! "${deb_tag}" =~ ^[a-z]+-v[0-9]+\.[0-9]+\.[0-9]+([.-][a-zA-Z0-9]+)*$ ]]; then
      die "Dockerfile uses unpinned or legacy Debian base image tag 'debian-base:${deb_tag}'; expected '<codename>-v<major>.<minor>.<patch>[...]' (e.g., trixie-v1.0.1-gke.0)"
    fi
  done
}

verify_dependabot_config() {
  local dependabot_path="${PKG_ROOT}/.github/dependabot.yml"
  [[ -f "${dependabot_path}" ]] || die "${dependabot_path} not found"

  local parsed_output
  if ! parsed_output="$(
    awk '
      function is_docker_ecosystem(line,    val) {
        if (line !~ /^[[:space:]]*-[[:space:]]*package-ecosystem:[[:space:]]*/) {
          return 0
        }
        val = line
        sub(/^[[:space:]]*-[[:space:]]*package-ecosystem:[[:space:]]*/, "", val)
        sub(/[[:space:]]*(#.*)?$/, "", val)
        return (val == "docker" || val == "\"docker\"" || val == "\x27docker\x27")
      }
      function trim_item(s) {
        sub(/^[[:space:]]+/, "", s)
        sub(/[[:space:]]+$/, "", s)
        gsub(/^["\x27]|["\x27]$/, "", s)
        return s
      }
      /^[[:space:]]*-[[:space:]]*package-ecosystem:/ {
        if (is_docker_ecosystem($0)) {
          docker_count++
          in_docker = 1
        } else {
          in_docker = 0
        }
        in_dirs = 0
        next
      }
      in_docker && /^[[:space:]]*directories:[[:space:]]*/ {
        rest = $0
        sub(/^[[:space:]]*directories:[[:space:]]*/, "", rest)
        sub(/[[:space:]]*(#.*)?$/, "", rest)
        if (rest == "") {
          in_dirs = 1
          next
        }
        if (rest ~ /^\[.*\]$/) {
          sub(/^\[/, "", rest)
          sub(/\]$/, "", rest)
          n = split(rest, items, ",")
          for (i = 1; i <= n; i++) {
            item = trim_item(items[i])
            if (item != "") {
              print "DIR:" item
            }
          }
          in_dirs = 0
          next
        }
        print "ERR:unsupported directories syntax in .github/dependabot.yml (use a YAML block list or single-line [...] list)"
        exit 2
      }
      in_docker && in_dirs {
        if ($0 ~ /^[[:space:]]*-[[:space:]]*/) {
          val = $0
          sub(/^[[:space:]]*-[[:space:]]*/, "", val)
          sub(/[[:space:]]*(#.*)?$/, "", val)
          val = trim_item(val)
          if (val != "") {
            print "DIR:" val
          }
          next
        }
        if ($0 !~ /^[[:space:]]*(#.*)?$/) {
          in_dirs = 0
        }
      }
      END {
        print "COUNT:" (docker_count + 0)
      }
    ' "${dependabot_path}"
  )"; then
    local err_line
    err_line="$(grep '^ERR:' <<< "${parsed_output}" | sed 's/^ERR://' || true)"
    die "${err_line:-failed to parse ${dependabot_path}}"
  fi

  local docker_ecosystems_count dependabot_dirs
  docker_ecosystems_count="$(awk -F: '$1 == "COUNT" { print $2 }' <<< "${parsed_output}")"
  if [[ "${docker_ecosystems_count}" -ne 1 ]]; then
    die ".github/dependabot.yml must define exactly 1 package-ecosystem: \"docker\" block (found ${docker_ecosystems_count})"
  fi

  dependabot_dirs="$(awk -F: '$1 == "DIR" { sub(/^DIR:/, ""); print }' <<< "${parsed_output}")"

  local rel_path dir_part expected_dir
  for rel_path in "${GOLANG_DOCKERFILES[@]}"; do
    dir_part="$(dirname "${rel_path}")"
    if [[ "${dir_part}" == "." ]]; then
      expected_dir="/"
    else
      expected_dir="/${dir_part}"
    fi
    if ! grep -Fxq "${expected_dir}" <<< "${dependabot_dirs}"; then
      die ".github/dependabot.yml docker 'directories:' is missing '${expected_dir}' (for ${rel_path})"
    fi
  done
}

main() {
  echo "Verifying Dockerfiles and Dependabot config"
  discover_golang_dockerfiles
  verify_golang_builders
  verify_debian_runtime_base
  verify_dependabot_config
}

main "$@"
