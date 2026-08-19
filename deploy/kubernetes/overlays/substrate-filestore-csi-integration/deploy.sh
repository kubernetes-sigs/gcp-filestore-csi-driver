#!/usr/bin/env bash

# Deploy GKE Substrate integration overlay for GCP Filestore CSI Driver.
#
# Environment Variables:
#   PROJECT_ID          (Required) GCP project ID where Filestore and GKE are hosted.
#   FILESTORE_ENDPOINT  (Optional) Filestore API service endpoint (default: file.googleapis.com).
#                                  Use "staging-file.sandbox.googleapis.com" for Staging.
#                                  Use "autopush-file.sandbox.googleapis.com" for Autopush.
#   GCE_REGION          (Optional) GCP region (default: us-central1). Matches Substrate dev env.
#   SHARE_POOL_NAME     (Optional) Filestore SharePool name for auto-generating storageclass.yaml.

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
  echo "     export FILESTORE_ENDPOINT=\"staging-file.sandbox.googleapis.com\" # (optional)" >&2
  echo "     ${0}" >&2
  echo "" >&2
  exit 1
fi

GCE_REGION="${GCE_REGION:-${LOCATION:-us-central1}}"
FILESTORE_ENDPOINT="${FILESTORE_ENDPOINT:-file.googleapis.com}"
SHARE_POOL_NAME="${SHARE_POOL_NAME:-}"

echo "📋 Configuration:"
echo "   - GCP Project ID     : ${PROJECT_ID}"
echo "   - Filestore Endpoint : ${FILESTORE_ENDPOINT}"
echo "   - GCE Region         : ${GCE_REGION}"
if [[ -n "${SHARE_POOL_NAME}" ]]; then
  echo "   - SharePool Name     : ${SHARE_POOL_NAME}"
else
  echo "   - SharePool Name     : (not set, skipping storageclass generation)"
fi
echo "================================================================="

# 2. Render serviceaccount_patch.yaml from template
echo "⚙️  Generating serviceaccount_patch.yaml for project '${PROJECT_ID}'..."
sed -e "s|\${PROJECT_ID}|${PROJECT_ID}|g" \
    "${SCRIPT_DIR}/serviceaccount_patch.yaml.tmpl" > "${SCRIPT_DIR}/serviceaccount_patch.yaml"

# 3. Render controller_patch.yaml from template
echo "⚙️  Generating controller_patch.yaml with endpoint '${FILESTORE_ENDPOINT}'..."
sed -e "s|\${FILESTORE_ENDPOINT}|${FILESTORE_ENDPOINT}|g" \
    "${SCRIPT_DIR}/controller_patch.yaml.tmpl" > "${SCRIPT_DIR}/controller_patch.yaml"

# 4. Render storageclass.yaml if SHARE_POOL_NAME is set
EXAMPLES_DIR="${REPO_ROOT}/examples/kubernetes/substrate-sharepool"
if [[ -n "${SHARE_POOL_NAME}" ]]; then
  if [[ -f "${EXAMPLES_DIR}/storageclass.yaml.tmpl" ]]; then
    echo "⚙️  Generating storageclass.yaml for pool '${SHARE_POOL_NAME}' in '${GCE_REGION}'..."
    sed -e "s|\${PROJECT_ID}|${PROJECT_ID}|g" \
        -e "s|\${GCE_REGION}|${GCE_REGION}|g" \
        -e "s|\${SHARE_POOL_NAME}|${SHARE_POOL_NAME}|g" \
        "${EXAMPLES_DIR}/storageclass.yaml.tmpl" > "${EXAMPLES_DIR}/storageclass.yaml"
  else
    echo "⚠️  [WARNING] Template '${EXAMPLES_DIR}/storageclass.yaml.tmpl' not found." >&2
  fi
fi

# 5. Apply Kustomize Overlay
echo "📦 Applying Substrate Filestore CSI Driver Overlay via Kustomize..."
kubectl apply -k "${SCRIPT_DIR}"

# 6. Optionally apply StorageClass
if [[ -n "${SHARE_POOL_NAME}" && -f "${EXAMPLES_DIR}/storageclass.yaml" ]]; then
  echo "💾 Applying StorageClass 'csi-filestore-sc'..."
  kubectl apply -f "${EXAMPLES_DIR}/storageclass.yaml"
fi

echo ""
echo "================================================================="
echo "✅ Deployment complete!"
echo "   - CSI Driver & Substrate CSIDriverConfig applied."
echo "   - Check controller status : kubectl get pods -n gcp-filestore-csi-driver"
echo "   - Check Substrate driver  : kubectl get csidriverconfig"
if [[ -n "${SHARE_POOL_NAME}" ]]; then
  echo "   - Check StorageClass      : kubectl get sc csi-filestore-sc"
fi
echo "================================================================="
