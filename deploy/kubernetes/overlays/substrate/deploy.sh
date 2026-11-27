#!/usr/bin/env bash

# Deploy GKE Substrate integration overlay for GCP Filestore CSI Driver.
#
# Usage: ./deploy.sh [OPTIONS]
#
# Note: All options can alternatively be provided as environment variables 
#       (PROJECT_ID, GCP_SERVICE_ACCOUNT, VOLUMEPOOL_LOCATION, VOLUMEPOOL_NAME, STORAGECLASS_NAME).
#       Command-line arguments will always take precedence over environment variables.
#
# Required Options:
#   -p, --project-id          GCP project ID where Filestore and GKE are hosted.
#
# Optional IAM Overrides:
#   -s, --service-account     An existing Google Service Account (GSA) email that already
#                             possesses the 'roles/file.editor' role. Use this to bypass 
#                             default Service Account creation if you manage IAM externally.
#                             Otherwise, the script will automatically create a default GSA 
#                             and bind the proper IAM roles for you.
# Optional StorageClass Automation:
#   Provide both volumepool-location and volumepool-name to automatically generate and apply a ready-to-use StorageClass.
#   -l, --volumepool-location GCP region or zone (e.g., 'us-central1') where your VolumePool resides.
#   -v, --volumepool-name     The actual name of the FiFA VolumePool to bind the StorageClass to.
#   -c, --storageclass-name   Custom name for the generated StorageClass. If you omit this 
#                             but provide volumepool-location and volumepool-name, you will be prompted 
#                             interactively for a name (or given a smart default).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../../.." && pwd)"

print_usage() {
  cat << 'EOF'
Usage: ./deploy.sh [OPTIONS]

Note: All options can alternatively be provided as environment variables 
      (PROJECT_ID, GCP_SERVICE_ACCOUNT, VOLUMEPOOL_LOCATION, VOLUMEPOOL_NAME, STORAGECLASS_NAME).
      Command-line arguments will always take precedence over environment variables.

Required Options:
  -p, --project-id          GCP project ID where Filestore and GKE are hosted.

Optional IAM Overrides:
  -s, --service-account     An existing Google Service Account (GSA) email that already
                            possesses the 'roles/file.editor' role. Use this to bypass 
                            default Service Account creation if you manage IAM externally.
                            Otherwise, the script will automatically create a default GSA 
                            and bind the proper IAM roles for you.

Optional StorageClass Automation:
  Provide both volumepool-location and volumepool-name to automatically generate and apply a ready-to-use StorageClass.
  -l, --volumepool-location GCP region or zone (e.g., 'us-central1') where your VolumePool resides.
  -v, --volumepool-name     The actual name of the FiFA VolumePool to bind the StorageClass to.
  -c, --storageclass-name   Custom name for the generated StorageClass. If you omit this 
                            but provide volumepool-location and volumepool-name, you will be prompted 
                            interactively for a name (or given a smart default).
EOF
}

# Initialize variables
PROJ=""
SA=""
LOC=""
VP=""
SC=""

# Parse arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    -h|--help)                print_usage; exit 0 ;;
    -p|--project-id)          PROJ="$2"; shift 2 ;;
    -s|--service-account)     SA="$2"; shift 2 ;;
    -l|--volumepool-location) LOC="$2"; shift 2 ;;
    -v|--volumepool-name)     VP="$2"; shift 2 ;;
    -c|--storageclass-name)   SC="$2"; shift 2 ;;
    *) echo "Unknown option: $1"; echo "Run ./deploy.sh --help for usage details."; exit 1 ;;
  esac
done

# Fallback to Environment Variables
PROJECT_ID="${PROJ:-${PROJECT_ID:-}}"
GCP_SERVICE_ACCOUNT="${SA:-${GCP_SERVICE_ACCOUNT:-}}"
VOLUMEPOOL_LOCATION="${LOC:-${VOLUMEPOOL_LOCATION:-}}"
VOLUMEPOOL_NAME="${VP:-${VOLUMEPOOL_NAME:-}}"
STORAGECLASS_NAME="${SC:-${STORAGECLASS_NAME:-}}"

echo "================================================================="
echo "  🚀 GKE Substrate Filestore CSI Driver Overlay Deployment"
echo "================================================================="

# 1. Validate Required Environment Variables
if [[ -z "${PROJECT_ID:-}" ]]; then
  echo "" >&2
  echo "❌ [ERROR] PROJECT_ID is required but not set." >&2
  echo "   Use flag --project-id or set PROJECT_ID environment variable." >&2
  echo "" >&2
  exit 1
fi

CUSTOM_GSA_PROVIDED=false
if [[ -n "${GCP_SERVICE_ACCOUNT:-}" ]]; then
  CUSTOM_GSA_PROVIDED=true
fi
GCP_SERVICE_ACCOUNT="${GCP_SERVICE_ACCOUNT:-substrate-filestore-csi@${PROJECT_ID}.iam.gserviceaccount.com}"

echo "📋 Configuration:"
echo "   - GCP Project ID     : ${PROJECT_ID}"
echo "   - GCP Service Account: ${GCP_SERVICE_ACCOUNT}"
[[ -n "$VOLUMEPOOL_LOCATION" ]] && echo "   - VolumePool Location: ${VOLUMEPOOL_LOCATION}"
[[ -n "$VOLUMEPOOL_NAME" ]] && echo "   - VolumePool Name    : ${VOLUMEPOOL_NAME}"
[[ -n "$STORAGECLASS_NAME" ]] && echo "   - StorageClass Name  : ${STORAGECLASS_NAME}"
echo "================================================================="

# 2. Configure Service Account & IAM Bindings
echo "🔐 Configuring Service Account & IAM Bindings..."

if [[ "${CUSTOM_GSA_PROVIDED}" == "false" ]]; then
  echo "   - Verifying default Service Account existence..."
  if ! gcloud iam service-accounts describe "${GCP_SERVICE_ACCOUNT}" --project="${PROJECT_ID}" >/dev/null 2>&1; then
    echo "   - Service Account not found. Creating 'substrate-filestore-csi'..."
    gcloud iam service-accounts create "substrate-filestore-csi" \
      --project="${PROJECT_ID}" \
      --display-name="GCP Filestore CSI Driver Substrate Service Account"
  else
    echo "   - Service Account already exists (Skipping creation)."
  fi

  echo "   - Granting 'roles/file.editor' to the default Service Account on project ${PROJECT_ID}..."
  gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
      --member="serviceAccount:${GCP_SERVICE_ACCOUNT}" \
      --role="roles/file.editor" \
      --condition=None >/dev/null 2>&1 || true
else
  echo "   - Custom GSA explicitly provided. Verifying 'roles/file.editor' permission..."
  if ! gcloud projects get-iam-policy "${PROJECT_ID}" \
      --flatten="bindings[].members" \
      --format="value(bindings.role)" \
      --filter="bindings.members:serviceAccount:${GCP_SERVICE_ACCOUNT}" | grep -q 'roles/file.editor'; then
    echo "" >&2
    echo "❌ [ERROR] The provided custom Service Account lacks 'roles/file.editor' on project ${PROJECT_ID}." >&2
    echo "   Please grant it to the Custom SA before running this script," >&2
    echo "   OR do not pass the --service-account (or GCP_SERVICE_ACCOUNT env var)" >&2
    echo "   to let this script automatically create and bind a default Service Account for you." >&2
    echo "" >&2
    exit 1
  fi
  echo "   - Custom GSA has required permissions. Proceeding..."
fi

echo "   - Binding Workload Identity User role to the Kubernetes ServiceAccount..."
gcloud iam service-accounts add-iam-policy-binding "${GCP_SERVICE_ACCOUNT}" \
    --project="${PROJECT_ID}" \
    --role="roles/iam.workloadIdentityUser" \
    --member="serviceAccount:${PROJECT_ID}.svc.id.goog[gcp-filestore-csi-driver/gcp-filestore-csi-controller-sa]" \
    --condition=None >/dev/null 2>&1 || true

# 3. Render serviceaccount_patch.yaml from template
echo "⚙️  Generating serviceaccount_patch.yaml with service account '${GCP_SERVICE_ACCOUNT}'..."
sed -e "s|\${GCP_SERVICE_ACCOUNT}|${GCP_SERVICE_ACCOUNT}|g" \
    "${SCRIPT_DIR}/serviceaccount_patch.yaml.tmpl" > "${SCRIPT_DIR}/serviceaccount_patch.yaml"

# 4. Apply Kustomize Overlay
echo "📦 Applying Substrate Filestore CSI Driver Overlay via Kustomize..."
kubectl apply -k "${SCRIPT_DIR}"

# 5. Optional StorageClass Generation
if [[ -n "${VOLUMEPOOL_LOCATION:-}" && -n "${VOLUMEPOOL_NAME:-}" ]]; then
  if [[ -z "${STORAGECLASS_NAME:-}" ]]; then
    # Open /dev/tty if possible, else standard input
    if [ -t 0 ]; then
       read -r -p "💬 No StorageClass name provided. Enter custom name or press Enter for default [substrate-volumepool-sc]: " USER_SC_NAME || USER_SC_NAME=""
    else
       USER_SC_NAME=""
    fi
    STORAGECLASS_NAME="${USER_SC_NAME:-substrate-volumepool-sc}"
  fi
  SC_MANIFEST="$(sed -e "s|<PROJECT_ID>|${PROJECT_ID}|g" \
      -e "s|<LOCATION>|${VOLUMEPOOL_LOCATION}|g" \
      -e "s|<VOLUMEPOOL_NAME>|${VOLUMEPOOL_NAME}|g" \
      -e "s|<STORAGECLASS_NAME>|${STORAGECLASS_NAME}|g" \
      "${REPO_ROOT}/examples/kubernetes/substrate/storageclass.yaml.tmpl")"

  # Only provisioner, parameters, reclaimPolicy and volumeBindingMode are
  # immutable; allowVolumeExpansion, mountOptions and allowedTopologies are
  # mutable, and are compared as well so deploy.sh doesn't overwrite or miss
  # custom settings. If only parameters.mount-options differs (for example, an
  # existing class from an earlier deploy.sh without mount-options), warn and
  # continue (the rest of the deployment has already been applied). If anything
  # else differs, fail so the run isn't reported as a success with the wrong
  # StorageClass.
  SC_FIELDS_TEMPLATE='provisioner={{.provisioner}}{{"\n"}}reclaimPolicy={{.reclaimPolicy}}{{"\n"}}volumeBindingMode={{.volumeBindingMode}}{{"\n"}}allowVolumeExpansion={{if .allowVolumeExpansion}}true{{else}}false{{end}}{{"\n"}}{{range .mountOptions}}mountOptions={{.}}{{"\n"}}{{end}}{{range .allowedTopologies}}allowedTopologies={{.}}{{"\n"}}{{end}}{{range $k, $v := .parameters}}parameters.{{$k}}={{$v}}{{"\n"}}{{end}}'
  # Only NotFound counts as missing; any other error stops the script.
  HAVE_SC_FIELDS="$(kubectl get storageclass --ignore-not-found -o go-template="${SC_FIELDS_TEMPLATE}" -- "${STORAGECLASS_NAME}")"
  if [[ -n "${HAVE_SC_FIELDS}" ]]; then
    WANT_SC_FIELDS="$(printf '%s\n' "${SC_MANIFEST}" | kubectl patch --local -f - --type=merge -p '{}' -o go-template="${SC_FIELDS_TEMPLATE}")"
    WANT_PROVISIONER="$(printf '%s\n' "${WANT_SC_FIELDS}" | sed -n '1s/^provisioner=//p')"
    HAVE_PROVISIONER="$(printf '%s\n' "${HAVE_SC_FIELDS}" | sed -n '1s/^provisioner=//p')"
    if [[ "${WANT_PROVISIONER}" != "${HAVE_PROVISIONER}" ]]; then
      # The class may belong to another driver: leave it alone, and fail, so
      # that the run isn't reported as a success without a StorageClass for
      # this driver.
      echo "" >&2
      echo "❌ [ERROR] StorageClass '${STORAGECLASS_NAME}' already exists with provisioner '${HAVE_PROVISIONER}', not '${WANT_PROVISIONER}'." >&2
      echo "   It may belong to another driver, so it was left unchanged. Re-run deploy.sh with another --storageclass-name." >&2
      echo "" >&2
      exit 1
    elif [[ "${WANT_SC_FIELDS}" != "${HAVE_SC_FIELDS}" ]]; then
      # Use sed rather than grep -v so an empty match under set -e doesn't fail.
      HAVE_OTHER_FIELDS="$(printf '%s\n' "${HAVE_SC_FIELDS}" | sed '/^parameters\.mount-options=/d')"
      WANT_OTHER_FIELDS="$(printf '%s\n' "${WANT_SC_FIELDS}" | sed '/^parameters\.mount-options=/d')"
      SC_DIFF_EXIT=0
      SC_DIFF_HEADER="⚠️  [WARNING] StorageClass '${STORAGECLASS_NAME}' already exists with different settings, so it was left unchanged."
      if [[ "${WANT_OTHER_FIELDS}" != "${HAVE_OTHER_FIELDS}" ]]; then
        SC_DIFF_EXIT=1
        SC_DIFF_HEADER="❌ [ERROR] StorageClass '${STORAGECLASS_NAME}' already exists with different settings, so it was left unchanged."
      fi
      TMP_BASE="${TMPDIR:-/tmp}"
      TMP_BASE="${TMP_BASE%/}"
      SC_MANIFEST_FILE="$(mktemp "${TMP_BASE:-/}/substrate-storageclass.XXXXXX")"
      printf '%s\n' "${SC_MANIFEST}" > "${SC_MANIFEST_FILE}"
      RERUN_CMD="$(printf '%q --project-id %q --volumepool-location %q --volumepool-name %q --storageclass-name %q' \
        "$0" "${PROJECT_ID}" "${VOLUMEPOOL_LOCATION}" "${VOLUMEPOOL_NAME}" "${STORAGECLASS_NAME}")"
      # Keep a custom Google service account: without it, the re-run would
      # create or bind the default one and re-annotate the controller's
      # Kubernetes service account.
      if [[ "${CUSTOM_GSA_PROVIDED}" == "true" ]]; then
        RERUN_CMD="${RERUN_CMD}$(printf ' --service-account %q' "${GCP_SERVICE_ACCOUNT}")"
      fi
      echo "" >&2
      echo "${SC_DIFF_HEADER}" >&2
      if [[ "${SC_DIFF_EXIT}" -ne 0 ]]; then
        echo "   Only a parameters.mount-options difference is a warning; other differences fail so the run" >&2
        echo "   isn't reported as a success with the wrong StorageClass." >&2
      fi
      echo "   Current:" >&2
      printf '%s\n' "${HAVE_SC_FIELDS}" | sed 's/^/     /' >&2
      echo "   Template:" >&2
      printf '%s\n' "${WANT_SC_FIELDS}" | sed 's/^/     /' >&2
      echo "   StorageClass parameters, reclaimPolicy and volumeBindingMode can't be changed in place." >&2
      echo "   To replace the StorageClass with the template's settings in one step" >&2
      echo "   (avoiding the gcloud and kubectl apply -k gap where creating actors or volumes fails):" >&2
      printf '     kubectl replace --force -f %q\n' "${SC_MANIFEST_FILE}" >&2
      echo "   Or delete it and re-run deploy.sh:" >&2
      printf '     kubectl delete storageclass -- %q\n' "${STORAGECLASS_NAME}" >&2
      printf '     %s\n' "${RERUN_CMD}" >&2
      echo "   For zero downtime, re-run with a new --storageclass-name instead." >&2
      echo "   Existing volumes keep the mount-options value they were created with; volumes created without" >&2
      echo "   one get the driver's defaults when they're next staged on a node (volumes still staged keep" >&2
      echo "   their current options)." >&2
      echo "" >&2
      if [[ "${SC_DIFF_EXIT}" -ne 0 ]]; then
        exit 1
      fi
    else
      echo "📦 StorageClass '${STORAGECLASS_NAME}' already exists with the same settings; applying it..."
      printf '%s\n' "${SC_MANIFEST}" | kubectl apply -f -
    fi
  else
    echo "📦 Creating StorageClass '${STORAGECLASS_NAME}'..."
    printf '%s\n' "${SC_MANIFEST}" | kubectl apply -f -
  fi
fi

echo ""
echo "================================================================="
echo "✅ Deployment complete!"
echo "   - CSI Driver & Substrate CSIDriverConfig applied."
echo "   - Check controller status : kubectl get pods -n gcp-filestore-csi-driver"
echo "   - Check Substrate driver  : kubectl get csidriverconfig"
echo "================================================================="
echo ""
