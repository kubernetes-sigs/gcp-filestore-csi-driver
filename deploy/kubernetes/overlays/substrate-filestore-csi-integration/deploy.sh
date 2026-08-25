#!/usr/bin/env bash

# Deploy GKE Substrate integration overlay for GCP Filestore CSI Driver.
#
# Environment Variables:
#   PROJECT_ID          (Required) GCP project ID where Filestore and GKE are hosted.
#   GCP_SERVICE_ACCOUNT (Optional) Google Service Account (GSA) email with roles/file.editor role.
#                                  Defaults to: substrate-filestore-csi@${PROJECT_ID}.iam.gserviceaccount.com

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../../.." && pwd)"

echo "================================================================="
echo "  🚀 GKE Substrate Filestore CSI Driver Overlay Deployment"
echo "================================================================="

# 1. Validate Required Environment Variables
if [[ -z "${PROJECT_ID:-}" ]]; then
  echo "" >&2
  echo "❌ [ERROR] PROJECT_ID environment variable is required but not set." >&2
  echo "   Please set your GCP Project ID before running this script:" >&2
  echo "     export PROJECT_ID=\"<your-gcp-project-id>\"" >&2
  echo "   Example:" >&2
  echo "     export PROJECT_ID=\"my-gcp-project\"" >&2
  echo "     ${0}" >&2
  echo "" >&2
  exit 1
fi

GCP_SERVICE_ACCOUNT="${GCP_SERVICE_ACCOUNT:-substrate-filestore-csi@${PROJECT_ID}.iam.gserviceaccount.com}"

echo "📋 Configuration:"
echo "   - GCP Project ID     : ${PROJECT_ID}"
echo "   - GCP Service Account: ${GCP_SERVICE_ACCOUNT}"
echo "================================================================="

# 2. Render serviceaccount_patch.yaml from template
echo "⚙️  Generating serviceaccount_patch.yaml with service account '${GCP_SERVICE_ACCOUNT}'..."
sed -e "s|\${GCP_SERVICE_ACCOUNT}|${GCP_SERVICE_ACCOUNT}|g" \
    "${SCRIPT_DIR}/serviceaccount_patch.yaml.tmpl" > "${SCRIPT_DIR}/serviceaccount_patch.yaml"

# 3. Apply Kustomize Overlay
echo "📦 Applying Substrate Filestore CSI Driver Overlay via Kustomize..."
kubectl apply -k "${SCRIPT_DIR}"

echo ""
echo "================================================================="
echo "✅ Deployment complete!"
echo "   - CSI Driver & Substrate CSIDriverConfig applied."
echo "   - Check controller status : kubectl get pods -n gcp-filestore-csi-driver"
echo "   - Check Substrate driver  : kubectl get csidriverconfig"
echo "================================================================="
