# GKE Substrate Integration Overlay for GCP Filestore CSI Driver

This overlay enables the **GCP Filestore CSI Driver** to integrate natively with **GKE Substrate Agent microVM runtime** and **Filestore SharePool (Multi-Share)** architecture.

---

## 🏗️ Architecture & Component Overview

| Component | Manifest File | Rationale |
| :--- | :--- | :--- |
| **GKE Workload Identity** | `serviceaccount_patch.yaml` | Annotates `gcp-filestore-csi-controller-sa` with the GCP Service Account (`iam.gke.io/gcp-service-account`), eliminating static JSON key secrets. |
| **Controller Patch** | `controller_patch.yaml` | 1. Sets `hostNetwork: false` so Workload Identity metadata server requests (`169.254.169.254`) succeed.<br>2. Deploys a privileged `socat` proxy bridging TCP port `10000` to Unix domain socket `/csi/csi.sock`.<br>3. Adds label `role: controller-plugin`.<br>4. Enables `--feature-share-pools=true`.<br>5. Supports configurable API endpoints (`--filestore-service-endpoint`). |
| **Node DaemonSet Patch** | `node_patch.yaml` | 1. Adds label `role: node-plugin`.<br>2. Mounts `/var/lib/ateom-gvisor` with `mountPropagation: Bidirectional` so NFS mounts are accessible by Substrate `ateom-gvisor`. |
| **gRPC Controller Service** | `service.yaml` | Creates `Service/csi-filestore-controller` (port `50053` -> targetPort `10000`) selecting `app=gcp-filestore-csi-driver` and `role=controller-plugin`, guaranteeing traffic reaches only the controller. |
| **Substrate Driver Config** | `csi_driver_config.yaml` | Registers the driver (`ate.dev/v1alpha1`) with Substrate pointing to the internal cluster DNS gRPC endpoint. |

---

## 🚀 Deployment

### Environment Variables

| Variable | Required | Default | Description |
| :--- | :--- | :--- | :--- |
| `PROJECT_ID` | **Yes** | — | GCP Project ID hosting GKE and Filestore. |
| `FILESTORE_ENDPOINT` | No | `file.googleapis.com` | Filestore API endpoint (`file.googleapis.com` for Prod, `staging-file.sandbox.googleapis.com` for Staging, `autopush-file.sandbox.googleapis.com` for Autopush). |
| `GCE_REGION` | No | `us-central1` | GCP region (matches Substrate `.ate-dev-env.sh` convention). |
| `SHARE_POOL_NAME` | No | — | Filestore SharePool name for auto-generating `storageclass.yaml`. |

---

### Option A: Automated Deployment with Validation (`deploy.sh`)
The included `deploy.sh` script validates required environment variables, renders templates for Workload Identity and Filestore endpoint, and applies the full overlay in one step:

```bash
# Production deployment
export PROJECT_ID="my-gcp-project"
./deploy/kubernetes/overlays/substrate-filestore-csi-integration/deploy.sh

# Or Staging qualification deployment
export PROJECT_ID="my-staging-project"
export GCE_REGION="us-central1"
export FILESTORE_ENDPOINT="staging-file.sandbox.googleapis.com"
export SHARE_POOL_NAME="my-share-pool"
./deploy/kubernetes/overlays/substrate-filestore-csi-integration/deploy.sh
```

### Option B: Manual Deployment via Kustomize
1. Render `serviceaccount_patch.yaml` and `controller_patch.yaml` from templates:
   ```bash
   sed "s|\${PROJECT_ID}|my-gcp-project|g" \
       deploy/kubernetes/overlays/substrate-filestore-csi-integration/serviceaccount_patch.yaml.tmpl \
       > deploy/kubernetes/overlays/substrate-filestore-csi-integration/serviceaccount_patch.yaml

   sed "s|\${FILESTORE_ENDPOINT}|file.googleapis.com|g" \
       deploy/kubernetes/overlays/substrate-filestore-csi-integration/controller_patch.yaml.tmpl \
       > deploy/kubernetes/overlays/substrate-filestore-csi-integration/controller_patch.yaml
   ```
2. Apply the overlay:
   ```bash
   kubectl apply -k deploy/kubernetes/overlays/substrate-filestore-csi-integration/
   ```
3. Deploy the sample StorageClass:
   ```bash
   kubectl apply -f examples/kubernetes/substrate-sharepool/storageclass.yaml
   ```
