# Production FiFA VolumePool Pre-Provisioning & Reconciliation Guide

This runbook details the production-tested, end-to-end operational procedure to pre-provision **25,000 micro-volumes** across **25 Filestore Regional instances (1,000 shares/instance)** in Google Cloud **Production** (`file.googleapis.com` / `us-central1`).

---

## Document Index

- [1. Production Architectural Invariants & Prerequisites](#1-production-architectural-invariants--prerequisites)
  - [1.1 Mendel Experiment Allowlist (`ELFSParamsOverrideForSubstrate`)](#11-mendel-experiment-allowlist-elfsparamsoverrideforsubstrate)
  - [1.2 Mandatory Tier: `REGIONAL` (Kernel Data Container Limit)](#12-mandatory-tier-regional-kernel-data-container-limit)
  - [1.3 Regional Storage Quota Budget (25,600 GiB)](#13-regional-storage-quota-budget-25600-gib)
  - [1.4 Public API Rate Limit Quota Elevation (20,000 req/min)](#14-public-api-rate-limit-quota-elevation-20000-reqmin)
  - [1.5 ReconcileDispatcher Status in Production](#15-reconciledispatcher-status-in-production)
- [2. Parameterized Configuration Variables](#2-parameterized-configuration-variables)
- [3. Step-by-Step Production Deployment](#3-step-by-step-production-deployment)
  - [Step 3.1: Apply Production Rate Quota Override](#step-31-apply-production-rate-quota-override)
  - [Step 3.2: Create VolumePool Resource in Production](#step-32-create-volumepool-resource-in-production)
  - [Step 3.3: Patch Template Overrides & Acceleration Knobs](#step-33-patch-template-overrides--acceleration-knobs)
  - [Step 3.4: Verify VolumePool Configuration](#step-34-verify-volumepool-configuration)
  - [Step 3.5: Launch the Accelerated Reconciler Loop](#step-35-launch-the-accelerated-reconciler-loop)
- [4. All-in-One Production Setup Script](#4-all-in-one-production-setup-script)
- [5. Monitoring, Verification & Diagnostics](#5-monitoring-verification--diagnostics)
  - [5.1 Inspect Reconciler LRO Status](#51-inspect-reconciler-lro-status)
  - [5.2 Monitor Parallel Backing Instance Creation](#52-monitor-parallel-backing-instance-creation)
  - [5.3 Verify Substrate Parameters on Provisioned Instances](#53-verify-substrate-parameters-on-provisioned-instances)
  - [5.4 Audit Exact Share Counts per Instance](#54-audit-exact-share-counts-per-instance)
- [6. Teardown & Quota Release](#6-teardown--quota-release)

---

## 1. Production Architectural Invariants & Prerequisites

Before provisioning in Production, verify that these mandatory prerequisites are satisfied:

### 1.1 Mendel Experiment Allowlist (`ELFSParamsOverrideForSubstrate`)
Substrate kernel configuration requires project allowlisting in Mendel:
- **Project ID**: `arokade-consumer` is submitted in [CL 983759989](http://cl/983759989) (`ELFSParamsOverrideForSubstrate_FeatureSettings.gcl`).
- **Project Number**: `6118277613` is submitted in [CL 982462285](http://cl/982462285) (`fifa_projects_lists.gcl` under `is_prod_fifa_project`).
- **Rollout**: `ELFSParamsOverrideForSubstrate_EarlyAdopters_Launch` is already deployed to 100% across all Prod rings (`ProdGlobal` and `ProdCatchAll`).

### 1.2 Mandatory Tier: `REGIONAL` (Kernel Data Container Limit)
- **`ENTERPRISE`**: Bypasses Substrate logic and unconditionally binds to `elfs.params.enterprise` (hardcoded to **100 data containers**).
- **`REGIONAL`**: Evaluates `isSubstrateEnabled` and binds to `elfs.params.regional_small_substrate` (sets **`efs.max_number_of_dcs = 2001`**).
- You **must** set `"tier": "REGIONAL"` in the instance template. Specifying `ENTERPRISE` will cause all `CreateShare` calls beyond #100 to fail with `ApplyCreateShare rollback` and internal errors.

### 1.3 Regional Storage Quota Budget (25,600 GiB)
Filestore Regional storage in `us-central1` has a default quota limit of **25,600 GiB (25.6 TB)**:
- $25\text{ instances} \times 1{,}024\text{ GiB} = \mathbf{25{,}600\text{ GiB}}$ (exactly 100% quota consumption).
- All instances must be sized at exactly **1,024 GiB (1 TB)**. Any existing instances in the project must be torn down first.
- If storage quota is exceeded, instance creation fails with `generic::resource_exhausted`, causing the reconciler to abort.

### 1.4 Public API Rate Limit Quota Elevation (20,000 req/min)
Standard Cloud Filestore API public quota is capped at **600 requests per minute per user/project** (`PublicAPIRequestRequestsPerMinutePerUser`).
- Creating 25,000 shares and scale-testing at 10–200 RPS generates between $600$ and $12{,}000$ requests/minute.
- You must elevate the rate quota to **20,000 req/min (20k)** in Production using the Tenant Manager (`tm`) CLI.

### 1.5 ReconcileDispatcher Status in Production
- The automatic background dispatcher (`ReconcileDispatcher`) is currently disabled in Prod by policy (`final denylist_condition = @environment.IsProd` in [CL 983044415](http://cl/983044415)) pending Stubby RateACL rollout.
- While [CL 989421232](http://cl/989421232) has been prepared to allowlist `arokade-consumer`, **you must trigger `:reconcile` manually via `producerpost prod`** to initiate instance and share creation immediately without waiting for the dispatcher.

---

## 2. Parameterized Configuration Variables

Set these environment variables in your terminal before running commands:

```bash
# ------------------------------------------------------------------------------
# Project, Region & Network Configuration
# ------------------------------------------------------------------------------
export PROJECT_ID="arokade-consumer"
export LOCATION="us-central1"
export NETWORK="projects/${PROJECT_ID}/global/networks/default"
export CSI_GSA="substrate-filestore-csi@${PROJECT_ID}.iam.gserviceaccount.com"

# ------------------------------------------------------------------------------
# Target VolumePool & Backing Instance Topology
# ------------------------------------------------------------------------------
export VOLUME_POOL="test"
export INSTANCE_PREFIX="vp-test-inst"
export TIER="REGIONAL"                        # MANDATORY: Enables Substrate 2001 DC config
export INSTANCE_CAPACITY_GB=1024              # MANDATORY: 1,024 GiB * 25 = 25,600 GiB (Quota limit)
export VOLUME_SIZE_MB=1024                   # 1 GiB micro-volume per share

# Sizing: 25 instances * 1,000 shares/instance = 25,000 shares
export TARGET_TOTAL_SHARES=25000
export TARGET_INSTANCES=25
export MAX_SHARES_PER_INSTANCE=1000

# ------------------------------------------------------------------------------
# Reconciler Acceleration Knobs
# ------------------------------------------------------------------------------
export MAX_PENDING_INSTANCE_CREATIONS=25      # Provision all 25 instances concurrently
export MAX_PENDING_VOLUMES_PER_INSTANCE=100  # In-flight CreateShare LRO throttle per instance
export OPERATION_POLL_LIMIT=5000             # Max operations evaluated per cycle
export RECONCILE_CYCLE_DELAY="15s"           # Polling loop cycle frequency
export RECONCILE_DURATION="28800s"           # 8 hours runtime

# ------------------------------------------------------------------------------
# Production Producer API Justification (Mandatory for producer* in Prod)
# ------------------------------------------------------------------------------
export CLOUDSDK_CORE_REQUEST_REASON="b/358172900 -- Substrate scale testing volumepool creation"
```

---

## 3. Step-by-Step Production Deployment

### Step 3.1: Apply Production Rate Quota Override

Apply the producer override using Tenant Manager (`tm`) against the production endpoint `file.googleapis.com`:

```bash
# Set rate quota to 20,000 req/min for Production
/google/data/ro/teams/tenantmanager/tools/tm consumers quota upsert-producer-override \
  --loas \
  --env=prod \
  file.googleapis.com \
  file.googleapis.com/public_a_p_i_request_requests \
  '1/min/{project}/{user}' \
  20000 \
  ${PROJECT_ID} \
  --force

# Verify override is effective:
/google/data/ro/teams/tenantmanager/tools/tm consumers quota get-quota-metric \
  --loas \
  --env=prod \
  file.googleapis.com \
  file.googleapis.com/public_a_p_i_request_requests \
  ${PROJECT_ID}
```

---

### Step 3.2: Create VolumePool Resource in Production

Create the VolumePool through the public OnePlatform endpoint (`https://file.googleapis.com/v1beta1/...`):

```bash
TOKEN=$(gcloud auth print-access-token --impersonate-service-account="${CSI_GSA}")

curl -s -X POST \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "description": "GoogleReservedOverrides={\"CustomVolumePoolConfig\":{\"MinInstances\":1,\"MaxInstances\":'"${TARGET_INSTANCES}"',\"MinAvailableVolumes\":'"${TARGET_TOTAL_SHARES}"',\"MaxVolumesPerInstance\":'"${MAX_SHARES_PER_INSTANCE}"',\"InstanceCapacityGb\":'"${INSTANCE_CAPACITY_GB}"',\"InstanceNamePrefix\":\"'"${INSTANCE_PREFIX}"'\"},\"CustomExperimentOverrides\":[\"ELFSParamsOverrideForSubstrate::Experiment\"]}",
    "network": "'"${NETWORK}"'",
    "defaultVolumeQuotaMib": '"${VOLUME_SIZE_MB}"'
  }' \
  "https://file.googleapis.com/v1beta1/projects/${PROJECT_ID}/locations/${LOCATION}/volumePools?volumePoolId=${VOLUME_POOL}" | jq .
```

---

### Step 3.3: Patch Template Overrides & Acceleration Knobs

Apply the internal configuration using `producerpatch prod`:
1. `tier: "REGIONAL"`: Unlocks `elfs.params.regional_small_substrate` (2,001 DC limit).
2. `capacityGb: 1024`: Fits within regional quota ($25 \times 1{,}024 = 25{,}600\text{ GiB}$).
3. `CustomExperimentOverrides`: Injects `ELFSParamsOverrideForSubstrate` into backing instances.
4. `maxPendingInstanceCreations: 25`: Provisions all 25 instances concurrently.
5. `maxPendingVolumeCreationsPerInstance: 100`: Maintains 100 in-flight volume LROs per instance ($25 \times 100 = 2,500$ parallel creations across the pool).
6. `operationPollLimit: 5000`: Evaluates up to 5,000 LROs per cycle.

> [!IMPORTANT]
> **Production Justification (`admin_session`)**: Internal producer RPCs against `prod` require an audit justification. Either run these commands inside an admin session (`admin_session --reason="b/358172900 -- Substrate scale testing volumepool creation"`) or set `export CLOUDSDK_CORE_REQUEST_REASON="b/358172900 -- Substrate scale testing volumepool creation"` in your environment.

```bash
source /google/src/head/depot/google3/cloud/filer/scripts/filer_util.sh
export CLOUDSDK_CORE_REQUEST_REASON="b/358172900 -- Substrate scale testing volumepool creation"

cat <<EOF | producerpatch prod "v1internal/projects/${PROJECT_ID}/locations/${LOCATION}/volumePools/${VOLUME_POOL}?updateMask=instanceTemplate,instanceNamePrefix,minAvailableVolumes,minInstances,maxInstances,maxVolumesPerInstance,maxPendingInstanceCreations,maxPendingVolumeCreationsPerInstance,operationPollLimit"
{
  "instanceTemplate": {
    "tier": "${TIER}",
    "networks": [{
      "network": "${NETWORK}",
      "connectMode": "PRIVATE_SERVICE_CONNECT"
    }],
    "capacityGb": ${INSTANCE_CAPACITY_GB},
    "requestOverrides": "{\"CustomExperimentOverrides\":[\"ELFSParamsOverrideForSubstrate::Experiment\"]}"
  },
  "instanceNamePrefix": "${INSTANCE_PREFIX}",
  "minAvailableVolumes": ${TARGET_TOTAL_SHARES},
  "minInstances": 1,
  "maxInstances": ${TARGET_INSTANCES},
  "maxVolumesPerInstance": ${MAX_SHARES_PER_INSTANCE},
  "maxPendingInstanceCreations": ${MAX_PENDING_INSTANCE_CREATIONS},
  "maxPendingVolumeCreationsPerInstance": ${MAX_PENDING_VOLUMES_PER_INSTANCE},
  "operationPollLimit": ${OPERATION_POLL_LIMIT}
}
EOF
```

---

### Step 3.4: Verify VolumePool Configuration

In Production, `producerget prod` requires internal `cloud-filer-pa` producer permissions. Query the VolumePool directly through the public `v1beta1` API using service account credentials:

```bash
TOKEN=$(gcloud auth print-access-token --impersonate-service-account="${CSI_GSA}")

curl -s -H "Authorization: Bearer ${TOKEN}" \
  "https://file.googleapis.com/v1beta1/projects/${PROJECT_ID}/locations/${LOCATION}/volumePools/${VOLUME_POOL}" | jq .
```

Verify that the output contains:
- `name`: `projects/arokade-consumer/locations/us-central1/volumePools/test`
- `description`: Contains `GoogleReservedOverrides` with:
  - `MinAvailableVolumes`: `25000`
  - `MaxInstances`: `25`
  - `MaxVolumesPerInstance`: `1000`
  - `InstanceCapacityGb`: `1024`
  - `InstanceNamePrefix`: `"vp-test-inst"`
  - `CustomExperimentOverrides`: `["ELFSParamsOverrideForSubstrate::Experiment"]`
- `network`: `projects/arokade-consumer/global/networks/default`
- `defaultVolumeQuotaMib`: `1024`
- `uid`: Assigned UUID from Spanner

---

### Step 3.5: Launch the Accelerated Reconciler Loop

Trigger the reconciler loop in Production with a **15-second cycle delay**:

```bash
source /google/src/head/depot/google3/cloud/filer/scripts/filer_util.sh

RECONCILE_RESP=$(printf '{"duration": "%s", "repeatDelay": "%s"}' "${RECONCILE_DURATION}" "${RECONCILE_CYCLE_DELAY}" | \
  producerpost prod "v1internal/projects/${PROJECT_ID}/locations/${LOCATION}/volumePools/${VOLUME_POOL}:reconcile" 2>/dev/null)
echo "${RECONCILE_RESP}" | jq .

export RECONCILE_OP=$(echo "${RECONCILE_RESP}" | jq -r '.name // empty')
echo "Reconciler Operation: ${RECONCILE_OP}"
```

---

## 4. All-in-One Production Setup Script

You can run this complete script to execute the setup in a single block:

```bash
(
set -e
source /google/src/head/depot/google3/cloud/filer/scripts/filer_util.sh

export PROJECT_ID="arokade-consumer"
export LOCATION="us-central1"
export NETWORK="projects/${PROJECT_ID}/global/networks/default"
export CSI_GSA="substrate-filestore-csi@${PROJECT_ID}.iam.gserviceaccount.com"

export VOLUME_POOL="test"
export INSTANCE_PREFIX="vp-test-inst"
export TIER="REGIONAL"
export INSTANCE_CAPACITY_GB=1024
export VOLUME_SIZE_MB=1024
export TARGET_TOTAL_SHARES=25000
export TARGET_INSTANCES=25
export MAX_SHARES_PER_INSTANCE=1000

export MAX_PENDING_INSTANCE_CREATIONS=25
export MAX_PENDING_VOLUMES_PER_INSTANCE=100
export OPERATION_POLL_LIMIT=5000
export RECONCILE_CYCLE_DELAY="15s"
export RECONCILE_DURATION="28800s"
export CLOUDSDK_CORE_REQUEST_REASON="b/358172900 -- Substrate scale testing volumepool creation"

echo "================================================================="
echo "[1/4] Applying Production Rate Quota Override (20k req/min)..."
echo "================================================================="
/google/data/ro/teams/tenantmanager/tools/tm consumers quota upsert-producer-override \
  --loas \
  --env=prod \
  file.googleapis.com \
  file.googleapis.com/public_a_p_i_request_requests \
  '1/min/{project}/{user}' \
  20000 \
  ${PROJECT_ID} \
  --force

echo "================================================================="
echo "[2/4] Creating VolumePool Resource in Production..."
echo "================================================================="
TOKEN=$(gcloud auth print-access-token --impersonate-service-account="${CSI_GSA}")

curl -s -X POST \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "description": "GoogleReservedOverrides={\"CustomVolumePoolConfig\":{\"MinInstances\":1,\"MaxInstances\":'"${TARGET_INSTANCES}"',\"MinAvailableVolumes\":'"${TARGET_TOTAL_SHARES}"',\"MaxVolumesPerInstance\":'"${MAX_SHARES_PER_INSTANCE}"',\"InstanceCapacityGb\":'"${INSTANCE_CAPACITY_GB}"',\"InstanceNamePrefix\":\"'"${INSTANCE_PREFIX}"'\"},\"CustomExperimentOverrides\":[\"ELFSParamsOverrideForSubstrate::Experiment\"]}",
    "network": "'"${NETWORK}"'",
    "defaultVolumeQuotaMib": '"${VOLUME_SIZE_MB}"'
  }' \
  "https://file.googleapis.com/v1beta1/projects/${PROJECT_ID}/locations/${LOCATION}/volumePools?volumePoolId=${VOLUME_POOL}" | jq .

echo "================================================================="
echo "[3/4] Patching Template Overrides & Acceleration Knobs..."
echo "================================================================="
cat <<EOF | producerpatch prod "v1internal/projects/${PROJECT_ID}/locations/${LOCATION}/volumePools/${VOLUME_POOL}?updateMask=instanceTemplate,instanceNamePrefix,minAvailableVolumes,minInstances,maxInstances,maxVolumesPerInstance,maxPendingInstanceCreations,maxPendingVolumeCreationsPerInstance,operationPollLimit"
{
  "instanceTemplate": {
    "tier": "${TIER}",
    "networks": [{
      "network": "${NETWORK}",
      "connectMode": "PRIVATE_SERVICE_CONNECT"
    }],
    "capacityGb": ${INSTANCE_CAPACITY_GB},
    "requestOverrides": "{\"CustomExperimentOverrides\":[\"ELFSParamsOverrideForSubstrate::Experiment\"]}"
  },
  "instanceNamePrefix": "${INSTANCE_PREFIX}",
  "minAvailableVolumes": ${TARGET_TOTAL_SHARES},
  "minInstances": 1,
  "maxInstances": ${TARGET_INSTANCES},
  "maxVolumesPerInstance": ${MAX_SHARES_PER_INSTANCE},
  "maxPendingInstanceCreations": ${MAX_PENDING_INSTANCE_CREATIONS},
  "maxPendingVolumeCreationsPerInstance": ${MAX_PENDING_VOLUMES_PER_INSTANCE},
  "operationPollLimit": ${OPERATION_POLL_LIMIT}
}
EOF

echo "================================================================="
echo "[4/4] Triggering Accelerated Reconciler Loop..."
echo "================================================================="
RECONCILE_RESP=$(printf '{"duration": "%s", "repeatDelay": "%s"}' "${RECONCILE_DURATION}" "${RECONCILE_CYCLE_DELAY}" | \
  producerpost prod "v1internal/projects/${PROJECT_ID}/locations/${LOCATION}/volumePools/${VOLUME_POOL}:reconcile" 2>/dev/null)
echo "${RECONCILE_RESP}" | jq .

export RECONCILE_OP=$(echo "${RECONCILE_RESP}" | jq -r '.name // empty')
echo "Reconciler Operation: ${RECONCILE_OP}"
)
```

---

## 5. Monitoring, Verification & Diagnostics

### 5.1 Inspect Reconciler LRO Status
```bash
source /google/src/head/depot/google3/cloud/filer/scripts/filer_util.sh
producerget prod "v1internal/${RECONCILE_OP}" | jq .
```

### 5.2 Monitor Parallel Backing Instance Creation
Confirm that all 25 instances are created and transition to `READY`:
```bash
gcloud filestore instances list \
  --project="${PROJECT_ID}" \
  --location="${LOCATION}" \
  --filter="name:${INSTANCE_PREFIX}"
```

### 5.3 Verify Substrate Parameters on Provisioned Instances
Confirm that instances booted with `REGIONAL` tier and Substrate overrides:
```bash
FIRST_INST=$(gcloud filestore instances list \
  --project="${PROJECT_ID}" \
  --location="${LOCATION}" \
  --filter="name:${INSTANCE_PREFIX}" \
  --format="value(name)" | head -1)

gcloud filestore instances describe "${FIRST_INST}" \
  --project="${PROJECT_ID}" \
  --location="${LOCATION}"
```

**Expected Indicators:**
- `tier`: `REGIONAL`
- `description`: Contains `"CustomExperimentOverrides":["ELFSParamsOverrideForSubstrate::Experiment"]`
- `description`: Contains `"CustomMultiShareConfig":{"MaxShareCount":1000}`

### 5.4 Audit Exact Share Counts per Instance
Query every instance to confirm that shares are provisioning across all 25 nodes:
```bash
source /google/src/head/depot/google3/cloud/filer/scripts/filer_util.sh

total=0
for inst in $(gcloud filestore instances list \
  --project="${PROJECT_ID}" \
  --location="${LOCATION}" \
  --filter="name:${INSTANCE_PREFIX}" \
  --format="value(name)"); do
  token=""
  inst_total=0
  while true; do
    url="v1internal/projects/${PROJECT_ID}/locations/${LOCATION}/instances/${inst}/shares:internal?pageSize=100"
    [[ -n "$token" ]] && url="${url}&pageToken=${token}"
    resp=$(producerget prod "$url" 2>/dev/null)
    cnt=$(echo "$resp" | jq '.shares | length // 0')
    inst_total=$((inst_total + cnt))
    token=$(echo "$resp" | jq -r '.nextPageToken // empty')
    [[ -z "$token" || "$cnt" -eq 0 ]] && break
  done
  total=$((total + inst_total))
  echo "${inst}: ${inst_total} shares"
done
echo "Total Provisioned Shares across Pool: ${total} / 25000"
```

---

## 6. Teardown & Quota Release

When testing is complete or when regional quota must be freed:

```bash
source /google/src/head/depot/google3/cloud/filer/scripts/filer_util.sh

# 1. Stop Reconciler Dispatch on the Pool
cat <<EOF | producerpatch prod "v1internal/projects/${PROJECT_ID}/locations/${LOCATION}/volumePools/${VOLUME_POOL}?updateMask=minAvailableVolumes,minInstances,maxInstances,reconcilerSettings.dispatchDisabled"
{
  "minAvailableVolumes": 0,
  "minInstances": 0,
  "maxInstances": 0,
  "reconcilerSettings": {
    "dispatchDisabled": true
  }
}
EOF

# 2. Disable Deletion Protection on all 25 instances
for inst in $(gcloud filestore instances list --project="${PROJECT_ID}" --location="${LOCATION}" --filter="name:${INSTANCE_PREFIX}" --format="value(name)"); do
  gcloud filestore instances update "${inst}" --project="${PROJECT_ID}" --location="${LOCATION}" --no-deletion-protection --quiet &
done
wait

# 3. Delete Backing Instances
for inst in $(gcloud filestore instances list --project="${PROJECT_ID}" --location="${LOCATION}" --filter="name:${INSTANCE_PREFIX}" --format="value(name)"); do
  gcloud filestore instances delete "${inst}" --project="${PROJECT_ID}" --location="${LOCATION}" --force --async --quiet &
done
wait

# 4. Delete the VolumePool Resource
TOKEN=$(gcloud auth print-access-token --impersonate-service-account="${CSI_GSA}")
curl -s -X DELETE \
  -H "Authorization: Bearer ${TOKEN}" \
  "https://file.googleapis.com/v1beta1/projects/${PROJECT_ID}/locations/${LOCATION}/volumePools/${VOLUME_POOL}?force=true" | jq .

# 5. Confirm 100% Quota is Released
gcloud filestore instances list --project="${PROJECT_ID}" --location="${LOCATION}" --filter="name:${INSTANCE_PREFIX}"
```
