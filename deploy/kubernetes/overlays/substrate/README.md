# GKE Substrate Integration Overlay for GCP Filestore CSI Driver

This overlay enables the **GCP Filestore CSI Driver** to integrate natively with **GKE Substrate Agent microVM runtime** and **Filestore VolumePool (FiFA Fast Availability)** architecture.

---

## 🏗️ Architecture & Component Overview

| Component | Manifest File | Rationale |
| :--- | :--- | :--- |
| **GKE Workload Identity** | `serviceaccount_patch.yaml` | Annotates `gcp-filestore-csi-controller-sa` with the GCP Service Account (`iam.gke.io/gcp-service-account`), eliminating static JSON key secrets. |
| **Controller Patch** | `controller_patch.yaml` | 1. Sets `hostNetwork: false` so Workload Identity metadata server requests (`169.254.169.254`) succeed.<br>2. Deploys a privileged `socat` proxy bridging TCP port `10000` to Unix domain socket `/csi/csi.sock`.<br>3. Adds label `role: controller-plugin`.<br>4. Enables `--feature-volume-pools=true`. |
| **Node DaemonSet Patch** | `node_patch.yaml` | 1. Adds label `role: node-plugin`.<br>2. Mounts `/var/lib/ateom-gvisor` with `mountPropagation: Bidirectional` so NFS mounts are accessible by Substrate `ateom-gvisor`. |
| **gRPC Controller Service** | `service.yaml` | Creates `Service/csi-filestore-controller` (port `50053` -> targetPort `10000`) selecting `app=gcp-filestore-csi-driver` and `role=controller-plugin`, guaranteeing traffic reaches only the controller. |
| **Substrate Driver Config** | `csi_driver_config.yaml` | Registers the driver (`ate.dev/v1alpha1`) with Substrate pointing to the internal cluster DNS gRPC endpoint. |

---

## 📋 Prerequisites & IAM Setup

This integration utilizes **GKE Workload Identity** to securely authenticate against Google Cloud Filestore APIs without storing static JSON credentials in Kubernetes secrets.

The Google Service Account (GSA) used by the CSI driver must have the **`roles/file.editor`** role (which grants full permissions across Filestore instances, VolumePools, shares, and operations).

* **Default GSA (`substrate-filestore-csi@<PROJECT_ID>.iam.gserviceaccount.com`)**: If you don't have an existing GSA, create and configure the default GSA using the steps below.
* **Custom / Existing GSA**: If your organization already has a GSA with the `roles/file.editor` permission, you can reuse it by passing `GCP_SERVICE_ACCOUNT="<custom-sa>@<PROJECT_ID>.iam.gserviceaccount.com"`.

```bash
export PROJECT_ID="my-gcp-project"
export GCP_SERVICE_ACCOUNT="substrate-filestore-csi@${PROJECT_ID}.iam.gserviceaccount.com"

# 1. (Optional) Create the default Google Service Account if it does not exist
gcloud iam service-accounts create substrate-filestore-csi \
    --project=${PROJECT_ID} \
    --display-name="GCP Filestore CSI Driver Substrate Service Account"

# 2. Grant the required Filestore Editor role to the GSA
gcloud projects add-iam-policy-binding ${PROJECT_ID} \
    --member="serviceAccount:${GCP_SERVICE_ACCOUNT}" \
    --role="roles/file.editor"

# 3. Bind the GSA to the Kubernetes ServiceAccount for GKE Workload Identity
gcloud iam service-accounts add-iam-policy-binding ${GCP_SERVICE_ACCOUNT} \
    --project=${PROJECT_ID} \
    --role="roles/iam.workloadIdentityUser" \
    --member="serviceAccount:${PROJECT_ID}.svc.id.goog[gcp-filestore-csi-driver/gcp-filestore-csi-controller-sa]"
```

---

## 🚀 Deployment

### Environment Variables

| Variable | Required | Default | Description |
| :--- | :--- | :--- | :--- |
| `PROJECT_ID` | **Yes** | — | GCP Project ID hosting GKE and Filestore. |
| `GCP_SERVICE_ACCOUNT` | No | `substrate-filestore-csi@${PROJECT_ID}.iam.gserviceaccount.com` | Google Service Account (GSA) email with `roles/file.editor` role for Workload Identity. Allows reusing any existing GSA. |

---

### Option A: Automated Deployment with Validation (`deploy.sh`)
The included `deploy.sh` script validates required environment variables, renders templates for Workload Identity with your specified `GCP_SERVICE_ACCOUNT` (or default), and applies the full overlay:

```bash
export PROJECT_ID="my-gcp-project"
# Optional: override only if using a custom GSA
# export GCP_SERVICE_ACCOUNT="my-custom-sa@${PROJECT_ID}.iam.gserviceaccount.com"
./deploy/kubernetes/overlays/substrate/deploy.sh
```

### Option B: Manual Deployment via Kustomize
1. Render `serviceaccount_patch.yaml` from template:
   ```bash
   sed "s|\${GCP_SERVICE_ACCOUNT}|substrate-filestore-csi@${PROJECT_ID}.iam.gserviceaccount.com|g" \
       deploy/kubernetes/overlays/substrate/serviceaccount_patch.yaml.tmpl \
       > deploy/kubernetes/overlays/substrate/serviceaccount_patch.yaml
   ```
2. Apply the overlay:
   ```bash
   kubectl apply -k deploy/kubernetes/overlays/substrate/
   ```

---

## 📦 Creating a StorageClass for VolumePool

To dynamically provision micro-volumes from a pre-warmed FiFA VolumePool, create a `StorageClass` referencing your VolumePool resource path via `parameters.volume-pool`.

See [`examples/kubernetes/substrate/storageclass.yaml`](../../../../examples/kubernetes/substrate/storageclass.yaml) for reference:

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: csi-filestore-volumepool-sc
provisioner: filestore.csi.storage.gke.io
parameters:
  volume-pool: "projects/<PROJECT_ID>/locations/<GCE_LOCATION>/volumePools/<VOLUMEPOOL_NAME>"
allowVolumeExpansion: false
reclaimPolicy: Delete
volumeBindingMode: Immediate
```

Apply the StorageClass:
```bash
kubectl apply -f examples/kubernetes/substrate/storageclass.yaml
```
