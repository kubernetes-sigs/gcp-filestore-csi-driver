#!/usr/bin/env bash
# ==============================================================================
# generate_tier_payloads.sh
# Dynamic CSI gRPC scale testing payload generator and VolumePool state inspector.
# Supports:
#   1. VolumePool capacity & inventory diagnostics (Total, Acquired, Ready, Releasing, Released)
#   2. Workload prefix breakdown (e.g. perf-scale-10rps, perf-scale-20rps, perf-scale-40rps)
#   3. Backing instance IP distribution & balance verification
#   4. Dynamic generation of NodeStageVolume, NodeUnstageVolume, DeleteVolume arrays
#   5. Upload to ghz-client test pod
#   6. Post-deletion release verification (--verify-released)
# ==============================================================================
(
set -euo pipefail

if [ $# -lt 1 ]; then
  echo "Usage:"
  echo "  $0 <RPS>                     Generate payloads and upload to ghz-client (e.g. $0 40)"
  echo "  $0 <RPS> --verify-released   Verify that tier volumes were deleted and released back to pool"
  echo "  $0 <RPS> --verify-only       Inspect pool inventory and verify counts without generating payloads"
  return 1 2>/dev/null || exit 1
fi

RPS="$1"
MODE="${2:-generate}"
EXPECTED_COUNT=$(( RPS * 60 ))

export PROJECT_ID="${PROJECT_ID:-arokade-consumer}"
export LOCATION="${LOCATION:-us-central1}"
export VOLUMEPOOL_NAME="${VOLUMEPOOL_NAME:-test}"
export CSI_GSA="${CSI_GSA:-substrate-filestore-csi@${PROJECT_ID}.iam.gserviceaccount.com}"
API_ENDPOINT="https://staging-file.sandbox.googleapis.com"
CHECKPOINT_FILE="scale_test_inputs/.tier_${RPS}rps_checkpoint.json"

# Source filer utilities for producerget if available
export RUNFILES="/google/src/head/depot"
if [[ -f /google/src/head/depot/google3/cloud/filer/scripts/filer_util.sh ]]; then
  set +u
  source /google/src/head/depot/google3/cloud/filer/scripts/filer_util.sh
  set -u
fi

mkdir -p scale_test_inputs

echo "================================================================="
echo " VolumePool Diagnostics & Payload Generator"
echo " Target Pool:   projects/${PROJECT_ID}/locations/${LOCATION}/volumePools/${VOLUMEPOOL_NAME}"
echo " Test Workload: ${RPS} RPS Tier (Expected: ${EXPECTED_COUNT} volumes)"
echo " Mode:          ${MODE}"
echo " Timestamp:     $(date -u '+%Y-%m-%d %H:%M:%S UTC')"
echo "================================================================="

# ------------------------------------------------------------------------------
# 1. Fetch VolumePool Spec & Backing Capacity
# ------------------------------------------------------------------------------
echo -e "\n[1/4] Fetching VolumePool Metadata & Instance Capacity..."
MAX_INST=25
MAX_VOL_PER_INST=1000
INST_PREFIX="vp-test-inst"
POOL_UID="963bba69-6ab9-429c-bfd8-1c23418ea3fd"

if type producerget &>/dev/null; then
  VP_RAW=$(producerget staging "v1internal/projects/${PROJECT_ID}/locations/${LOCATION}/volumePools/${VOLUMEPOOL_NAME}" 2>/dev/null || true)
  VP_JSON=$(echo "${VP_RAW}" | sed -n '/^{/,$p')
  if [[ -n "${VP_JSON}" && "${VP_JSON}" != "{}" ]]; then
    MAX_INST=$(echo "${VP_JSON}" | jq -r '.maxInstances // 25')
    MAX_VOL_PER_INST=$(echo "${VP_JSON}" | jq -r '.maxVolumesPerInstance // 1000')
    INST_PREFIX=$(echo "${VP_JSON}" | jq -r '.instanceNamePrefix // "vp-test-inst"')
    POOL_UID=$(echo "${VP_JSON}" | jq -r '.uniqueId // "963bba69-6ab9-429c-bfd8-1c23418ea3fd"')
  fi
fi

TOTAL_POOL_CAPACITY=$(( MAX_INST * MAX_VOL_PER_INST ))
echo "  Pool Unique ID:           ${POOL_UID}"
echo "  Max Allowed Instances:    ${MAX_INST}"
echo "  Max Volumes Per Instance: ${MAX_VOL_PER_INST}"
echo "  Total Provisioned Capacity: ${TOTAL_POOL_CAPACITY} shares"

# ------------------------------------------------------------------------------
# 2. Fetch All Acquired Volumes from VolumePool API
# ------------------------------------------------------------------------------
echo -e "\n[2/4] Querying Acquired Volumes from VolumePool across pages..."
TOKEN=$(gcloud auth print-access-token --impersonate-service-account="${CSI_GSA}" 2>/dev/null)
PAGE_TOKEN=""
echo "[]" > /tmp/all_vp_volumes.json

while :; do
  URL="${API_ENDPOINT}/v1beta1/projects/${PROJECT_ID}/locations/${LOCATION}/volumePools/${VOLUMEPOOL_NAME}/volumes?pageSize=100"
  if [ -n "$PAGE_TOKEN" ]; then
    URL="${URL}&pageToken=${PAGE_TOKEN}"
  fi
  RESP=$(curl -s -H "Authorization: Bearer ${TOKEN}" "${URL}")
  
  jq -s '.[0] + (.[1].volumes // [])' /tmp/all_vp_volumes.json <(echo "$RESP") > /tmp/tmp_volumes.json && mv /tmp/tmp_volumes.json /tmp/all_vp_volumes.json
  
  PAGE_TOKEN=$(echo "$RESP" | jq -r '.nextPageToken // empty')
  [ -z "$PAGE_TOKEN" ] && break
  echo -n "."
done
echo ""

TOTAL_ACQUIRED=$(jq length /tmp/all_vp_volumes.json)
READY_AVAILABLE=$(( TOTAL_POOL_CAPACITY - TOTAL_ACQUIRED ))
if [[ ${READY_AVAILABLE} -lt 0 ]]; then
  READY_AVAILABLE=0
fi

# Filter for the target tier workload
TARGET_TIER_PREFIX="perf-scale-${RPS}rps-"
jq '[ .[] | select(.name | contains("'"${TARGET_TIER_PREFIX}"'")) ]' /tmp/all_vp_volumes.json > /tmp/filtered_tier_volumes.json
TIER_ACQUIRED=$(jq length /tmp/filtered_tier_volumes.json)

# Tally breakdown of all prefixes in the pool
PREFIX_SUMMARY=$(jq -r '
  [ .[].name | split("/") | last | sub("-[0-9]+$"; "") ] 
  | group_by(.) 
  | map({prefix: .[0], count: length}) 
  | .[] 
  | "  - " + (.prefix) + " : " + (.count | tostring) + " volumes"
' /tmp/all_vp_volumes.json)

# Verify backing IP balance for target tier
UNIQUE_IPS=$(jq -r '[ .[].mountPoint.ipAddress ] | unique | length' /tmp/filtered_tier_volumes.json)

# Check Spanner for live Releasing/Scrubbing shares if accessible
RELEASING_COUNT="N/A (Requires AoD)"
SPAWNER_QUERY="SELECT state, COUNT(*) as count FROM Shares WHERE pool_unique_id = '${POOL_UID}' GROUP BY state;"
SPAN_OUT=$(span sql /span/global/cloud-control2-fs-sharepool-db:qual-staging-us-central1 "${SPAWNER_QUERY}" 2>/dev/null || true)
if [[ -n "${SPAN_OUT}" && "${SPAN_OUT}" != *"denied"* ]]; then
  RELEASING_COUNT=$(echo "${SPAN_OUT}" | awk '/released/ {print $2}')
  if [[ -z "${RELEASING_COUNT}" ]]; then
    RELEASING_COUNT=0
  fi
fi

# Calculate released count if verifying release
TIER_PREV_ACQUIRED=0
TIER_RELEASED=0
if [[ -f "${CHECKPOINT_FILE}" ]]; then
  TIER_PREV_ACQUIRED=$(jq -r '.tier_acquired // 0' "${CHECKPOINT_FILE}")
  TIER_RELEASED=$(( TIER_PREV_ACQUIRED - TIER_ACQUIRED ))
  if [[ ${TIER_RELEASED} -lt 0 ]]; then
    TIER_RELEASED=0
  fi
fi

# ------------------------------------------------------------------------------
# 3. Display VolumePool Inventory & Diagnostics Table
# ------------------------------------------------------------------------------
echo -e "\n[3/4] VolumePool Inventory & Health Diagnostics"
echo "================================================================="
printf "%-35s : %d volumes\n" "Total Pool Provisioned Capacity" "${TOTAL_POOL_CAPACITY}"
printf "%-35s : %d volumes\n" "Total Acquired (In-Use in Pool)" "${TOTAL_ACQUIRED}"
printf "%-35s : %d volumes\n" "Total Ready / Available in Pool" "${READY_AVAILABLE}"
printf "%-35s : %s\n"         "Total Releasing (In-Scrubbing)"  "${RELEASING_COUNT}"
echo "-----------------------------------------------------------------"
echo "Acquired Volumes by Workload Prefix:"
if [ -n "${PREFIX_SUMMARY}" ]; then
  echo "${PREFIX_SUMMARY}"
else
  echo "  (No volumes currently acquired)"
fi
echo "-----------------------------------------------------------------"
printf "%-35s : %d volumes\n" "Target Tier (${RPS} RPS) Expected" "${EXPECTED_COUNT}"
printf "%-35s : %d volumes\n" "Target Tier (${RPS} RPS) Acquired" "${TIER_ACQUIRED}"
if [[ -f "${CHECKPOINT_FILE}" ]]; then
  printf "%-35s : %d volumes\n" "Target Tier (${RPS} RPS) Released" "${TIER_RELEASED}"
fi
printf "%-35s : %d instances\n" "Target Tier Backing IPs Active" "${UNIQUE_IPS}"
echo "================================================================="

# ------------------------------------------------------------------------------
# 4. Action based on MODE
# ------------------------------------------------------------------------------
if [ "${MODE}" == "--verify-released" ]; then
  echo -e "\n[4/4] Post-Deletion Release Verification..."
  if [ "${TIER_ACQUIRED}" -eq 0 ]; then
    echo "✅ SUCCESS: 100% of ${RPS} RPS volumes (${EXPECTED_COUNT}) have been deleted and released from ${VOLUMEPOOL_NAME}!"
    echo "   Available capacity in pool is now ${READY_AVAILABLE} / ${TOTAL_POOL_CAPACITY} volumes."
    if [ "${RELEASING_COUNT}" != "N/A (Requires AoD)" ]; then
      echo "   Shares in Releasing/Scrubbing queue: ${RELEASING_COUNT}"
    fi
    rm -f /tmp/all_vp_volumes.json /tmp/filtered_tier_volumes.json
    return 0 2>/dev/null || exit 0
  else
    echo "⚠️  WARNING: ${TIER_ACQUIRED} volumes with prefix '${TARGET_TIER_PREFIX}' are still present in ${VOLUMEPOOL_NAME}!"
    echo "   DeleteVolume may still be in progress or some volumes failed deletion."
    rm -f /tmp/all_vp_volumes.json /tmp/filtered_tier_volumes.json
    return 1 2>/dev/null || exit 1
  fi
fi

if [ "${MODE}" == "--verify-only" ]; then
  echo -e "\n[4/4] Verification-only mode completed."
  rm -f /tmp/all_vp_volumes.json /tmp/filtered_tier_volumes.json
  return 0 2>/dev/null || exit 0
fi

# Standard Generate Mode: Check volume count before building payloads
if [ "${TIER_ACQUIRED}" -eq 0 ]; then
  echo "❌ ERROR: No volumes found with prefix '${TARGET_TIER_PREFIX}' in ${VOLUMEPOOL_NAME}."
  echo "   Please verify CreateVolume ran before generating payloads."
  rm -f /tmp/all_vp_volumes.json /tmp/filtered_tier_volumes.json
  return 1 2>/dev/null || exit 1
elif [ "${TIER_ACQUIRED}" -ne "${EXPECTED_COUNT}" ]; then
  echo "⚠️  WARNING: Volume count delta detected! Found ${TIER_ACQUIRED} volumes for tier '${TARGET_TIER_PREFIX}' (expected ${EXPECTED_COUNT})."
  echo "   Proceeding to generate payloads for all ${TIER_ACQUIRED} acquired volumes..."
fi

# Save state checkpoint for release tracking (using jq instead of heredoc)
jq -n \
  --arg ts "$(date -u '+%Y-%m-%d %H:%M:%S UTC')" \
  --argjson rps "${RPS}" \
  --argjson expected "${EXPECTED_COUNT}" \
  --argjson tier_acquired "${TIER_ACQUIRED}" \
  --argjson total_acquired "${TOTAL_ACQUIRED}" \
  --argjson ready_avail "${READY_AVAILABLE}" \
  '{timestamp: $ts, rps: $rps, expected_count: $expected, tier_acquired: $tier_acquired, total_acquired: $total_acquired, ready_available: $ready_avail}' > "${CHECKPOINT_FILE}"

echo -e "\n[4/4] Generating CSI Payloads & Uploading to ghz-client..."

# 1. NodeStageVolume JSON array
jq '[
  .[] | {
    "volume_id": ("volumepool://'"${PROJECT_ID}"'/'"${LOCATION}"'/'"${VOLUMEPOOL_NAME}"'/volumes/" + ((.name | split("/") | last) | gsub(":"; "%253A")) + (if (.mountPoint.ipAddress // .mountPoints[0].ipAddress) then "?ip=" + (.mountPoint.ipAddress // .mountPoints[0].ipAddress) else "" end)),
    "staging_target_path": ("/var/lib/kubelet/plugins/kubernetes.io/csi/pv/perf-stage-" + ((.name | split("/") | last) | gsub(":"; "%253A")) + "/globalmount"),
    "volume_capability": {"mount": {"mount_flags": ["vers=3","proto=tcp","mountproto=tcp","nolock","noatime","hard"]},"access_mode": {"mode": 1}},
    "volume_context": {"ip": (.mountPoint.ipAddress // .mountPoints[0].ipAddress),"volume": (.mountPoint.mountName // .mountPoints[0].mountName // "")}
  }
]' /tmp/filtered_tier_volumes.json > "scale_test_inputs/node_stage_volume_${RPS}rps.json"

# 2. NodeUnstageVolume JSON array
jq '[
  .[] | {
    "volume_id": ("volumepool://'"${PROJECT_ID}"'/'"${LOCATION}"'/'"${VOLUMEPOOL_NAME}"'/volumes/" + ((.name | split("/") | last) | gsub(":"; "%253A")) + (if (.mountPoint.ipAddress // .mountPoints[0].ipAddress) then "?ip=" + (.mountPoint.ipAddress // .mountPoints[0].ipAddress) else "" end)),
    "staging_target_path": ("/var/lib/kubelet/plugins/kubernetes.io/csi/pv/perf-stage-" + ((.name | split("/") | last) | gsub(":"; "%253A")) + "/globalmount")
  }
]' /tmp/filtered_tier_volumes.json > "scale_test_inputs/node_unstage_volume_${RPS}rps.json"

# 3. DeleteVolume JSON array
jq '[
  .[] | {
    "volume_id": ("volumepool://'"${PROJECT_ID}"'/'"${LOCATION}"'/'"${VOLUMEPOOL_NAME}"'/volumes/" + ((.name | split("/") | last) | gsub(":"; "%253A")) + (if (.mountPoint.ipAddress // .mountPoints[0].ipAddress) then "?ip=" + (.mountPoint.ipAddress // .mountPoints[0].ipAddress) else "" end))
  }
]' /tmp/filtered_tier_volumes.json > "scale_test_inputs/delete_volume_${RPS}rps.json"

rm -f /tmp/all_vp_volumes.json /tmp/filtered_tier_volumes.json

# Copy to ghz-client
for file in "node_stage_volume_${RPS}rps.json" "node_unstage_volume_${RPS}rps.json" "delete_volume_${RPS}rps.json"; do
  kubectl cp "scale_test_inputs/${file}" "gcp-filestore-csi-driver/ghz-client:/${file}"
  echo "  Uploaded scale_test_inputs/${file} -> ghz-client:/${file}"
done

echo "✅ Successfully generated and copied all ${RPS} RPS payloads to ghz-client!"
)
