# GKE Substrate Integration Overlay for GCP Filestore CSI Driver

This overlay enables the **GCP Filestore CSI Driver** to integrate natively with **GKE Substrate** and **Filestore VolumePool (FiFA Fast Availability)** architecture.

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

1. **Deploy Substrate**: The `deploy.sh` script creates a `CSIDriverConfig` custom resource. This requires the Substrate system to be deployed on your GKE cluster first to provide the `CSIDriverConfig` CRD. Please follow the installation steps here: [Substrate GKE Quickstart](https://github.com/agent-substrate/substrate/tree/main#gke-quickstart-development).
2. **Disable Existing Driver**: The managed GKE Filestore CSI driver addon must be disabled before installing this driver via the Substrate overlay to prevent deployment conflicts. You can disable it on your cluster using:
   ```bash
   gcloud container clusters update <CLUSTER_NAME> \
       --location=<CLUSTER_LOCATION> \
       --update-addons=GcpFilestoreCsiDriver=DISABLED
   ```
3. **VolumePool Setup**: Ensure your GCP Project is allowlisted for Filestore VolumePools and that you have precreated a VolumePool instance in your target location before continuing. On the backend, a reconciler provisions pre-warmed shares on this VolumePool which are then instantly claimed by the GCP Filestore CSI driver for your Substrate workloads. You can reach out to the Filestore Control Plane team for allowlisting and creation.

4. **IAM Permissions Requirement**: The user or automation executing the deployment script *must* have the permissions to create Service Accounts and assign IAM bindings in the target GCP Project (e.g. `roles/iam.serviceAccountAdmin` and `roles/resourcemanager.projectIamAdmin`, or broadly `roles/owner`).

This integration utilizes **GKE Workload Identity** to securely authenticate against Google Cloud Filestore APIs without storing static JSON credentials in Kubernetes secrets.

> [!NOTE]
> 💡 **Automated IAM Setup via `deploy.sh`**
> The included `deploy.sh` script completely automates the IAM setup. It intelligently checks for the existence of the Service Account, skips creation if explicitly provided (and validates permissions), and automatically enforces both the `roles/file.editor` and `roles/iam.workloadIdentityUser` bindings idempotently. You do not need to run manual `gcloud` IAM commands!

---

## 🚀 Deployment

### Configuration Parameters

You can pass these as command-line flags or environment variables. Command-line flags take precedence.

| Flag | Env Var | Required | Default | Description |
| :--- | :--- | :--- | :--- | :--- |
| `-h`, `--help`| — | No | — | Print the usage menu for deploy.sh. |
| `-p`, `--project-id` | `PROJECT_ID` | **Yes** | — | GCP Project ID hosting GKE and Filestore. |
| `-s`, `--service-account` | `GCP_SERVICE_ACCOUNT` | No | `substrate-filestore-csi@...` | Optional. Provide an existing Google Service Account (GSA) email that already possesses the `roles/file.editor` role to bypass default SA creation. |
| `-l`, `--volumepool-location` | `VOLUMEPOOL_LOCATION` | No | — | Optional. GCE location (e.g. `us-central1`) for automated StorageClass generation. |
| `-v`, `--volumepool-name`| `VOLUMEPOOL_NAME` | No | — | Optional. VolumePool name for automated StorageClass generation. |
| `-c`, `--storageclass-name`| `STORAGECLASS_NAME`| No | `substrate-volumepool-sc` | Optional. Custom StorageClass name. If omitted but volumepool-location and volumepool-name are provided, you will be prompted interactively. |

---

### Option A: Automated Deployment (`deploy.sh`)
The included `deploy.sh` script completely automates the installation, validates parameters, and configures IAM Workload Identity flawlessly. 

```bash
# Minimal required deployment:
./deploy/kubernetes/overlays/substrate/deploy.sh --project-id "my-gcp-project"

# Full deployment with turnkey StorageClass creation:
./deploy/kubernetes/overlays/substrate/deploy.sh \
    --project-id "my-gcp-project" \
    --volumepool-location "us-central1" \
    --volumepool-name "my-fast-pool"
```

*Note: If you provide `--volumepool-location` and `--volumepool-name`, the deployment script will finish by interactively asking if you want to use the default StorageClass name or a custom name, and then automatically apply it for you!*

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

*(Note: If you skip `deploy.sh`, you will need to manually render and apply `examples/kubernetes/substrate/storageclass.yaml.tmpl` to provision micro-volumes from a VolumePool).*
