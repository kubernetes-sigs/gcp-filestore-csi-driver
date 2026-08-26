#!/usr/bin/env bash

# Deploy GKE Substrate integration overlay for GCP Filestore CSI Driver.
#
# Usage: ./deploy.sh [OPTIONS]
#
# Note: All options can alternatively be provided as environment variables 
#       (PROJECT_ID, GCP_SERVICE_ACCOUNT, LOCATION, VOLUMEPOOL_NAME, STORAGECLASS_NAME).
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

echo ""
echo "================================================================="
echo "✅ Deployment complete!"
echo "   - CSI Driver & Substrate CSIDriverConfig applied."
echo "   - Check controller status : kubectl get pods -n gcp-filestore-csi-driver"
echo "   - Check Substrate driver  : kubectl get csidriverconfig"
echo "================================================================="
echo ""

# 5. Optional StorageClass Generation
if [[ -n "${VOLUMEPOOL_LOCATION:-}" && -n "${VOLUMEPOOL_NAME:-}" ]]; then
  if [[ -z "${STORAGECLASS_NAME:-}" ]]; then
    # Open /dev/tty if possible, else standard input
    if [ -t 0 ]; then
       read -p "💬 No StorageClass name provided. Enter custom name or press Enter for default [substrate-volumepool-sc]: " USER_SC_NAME
    else
       USER_SC_NAME=""
    fi
    STORAGECLASS_NAME="${USER_SC_NAME:-substrate-volumepool-sc}"
  fi
  echo "📦 Creating StorageClass '${STORAGECLASS_NAME}'..."
  sed -e "s|<PROJECT_ID>|${PROJECT_ID}|g" \
      -e "s|<LOCATION>|${VOLUMEPOOL_LOCATION}|g" \
      -e "s|<VOLUMEPOOL_NAME>|${VOLUMEPOOL_NAME}|g" \
      -e "s|<STORAGECLASS_NAME>|${STORAGECLASS_NAME}|g" \
      "${REPO_ROOT}/examples/kubernetes/substrate/storageclass.yaml.tmpl" | kubectl apply -f -
fi
