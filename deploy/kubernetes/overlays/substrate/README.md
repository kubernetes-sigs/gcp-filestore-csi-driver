# GKE Substrate Integration Overlay for GCP Filestore CSI Driver

This overlay enables the **GCP Filestore CSI Driver** to integrate natively with **GKE Substrate** and **Filestore VolumePool (FiFA Fast Availability)** architecture.

---

## 🏗️ Architecture & Component Overview

| Component | Manifest File | Rationale |
| :--- | :--- | :--- |
| **GKE Workload Identity** | `serviceaccount_patch.yaml` | Annotates `gcp-filestore-csi-controller-sa` with the GCP Service Account (`iam.gke.io/gcp-service-account`), eliminating static JSON key secrets. |
| **Controller Patch** | `controller_patch.yaml` | 1. Sets `hostNetwork: false` so Workload Identity metadata server requests (`169.254.169.254`) succeed.<br>2. Deploys a privileged `envoy` mTLS proxy bridging TCP port `10000` to Unix domain socket `/csi/csi.sock`.<br>3. Adds label `role: controller-plugin` to the pods and the Deployment selector.<br>4. Enables `--feature-volume-pools=true`.<br>5. Applies the scale tuning described in **Scale Tuning** below. |
| **Node DaemonSet Patch** | `node_patch.yaml` | 1. Adds label `role: node-plugin` to the pods and the DaemonSet selector.<br>2. Mounts `/var/lib/ateom-gvisor` with `mountPropagation: Bidirectional` so NFS mounts are accessible by Substrate `ateom-gvisor`.<br>3. Applies the scale tuning described in **Scale Tuning** below. |
| **gRPC Controller Service** | `service.yaml` | Creates the headless `Service/csi-filestore-controller` (port `10000` -> targetPort `10000` over mTLS) selecting `app=gcp-filestore-csi-driver` and `role=controller-plugin`, guaranteeing traffic reaches only the controller. Being headless, the Service name resolves to every ready controller pod, so gRPC clients can spread RPCs across replicas. |
| **Substrate Driver Config** | `csi_driver_config.yaml` | Registers the driver (`ate.dev/v1alpha1`) with Substrate pointing to the internal cluster DNS gRPC endpoint. |
| **Controller Disruption Budget** | `pdb.yaml` | `PodDisruptionBudget` allowing at most two controller pods to be evicted at a time during voluntary disruptions such as node drains and upgrades, leaving the three needed for 40 RPS. |

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

---

## ⚖️ Scale Tuning

The controller and node patches size the driver for Substrate's VolumePool workload. The sizing comes from scale tests of `CreateVolume`, `NodeStageVolume`, `NodeUnstageVolume` and `DeleteVolume` at 10 to 200 RPS.

### Controller (`controller_patch.yaml`)

- **Replicas:** 5 for 40 RPS of `CreateVolume`/`DeleteVolume`. In the scale tests, with RPCs spread across replicas, 3 replicas handled 40 RPS, 5 replicas 80 to 100 RPS, and 8 replicas 200 RPS, so 5 replicas leave two spare at 40 RPS. Each replica's limits are also sized so that a single pod can absorb all 40 RPS, in case the client does not spread RPCs across replicas.
- **Availability:** readiness probes keep a pod out of the Service's DNS until it can serve (the driver must be listening on `/csi/csi.sock`, and `envoy` must be listening on port `10000`). A soft topology spread places the replicas on different nodes and zones when possible, and `pdb.yaml` allows at most two replicas to be evicted at a time, leaving the three needed for 40 RPS.
- **Burstable QoS:** requests sit above the measured peak usage, so the pods stay within their requests (which keeps them out of the first eviction tier), while the limits leave headroom for load spikes without reserving it on the node. In the scale tests, the driver container peaked at about 35 MiB while handling 25 RPS per replica.

| Container | Requests (CPU / Memory) | Limits (CPU / Memory) |
| :--- | :--- | :--- |
| `gcp-filestore-driver` | 500m / 256Mi | 2 / 1Gi |
| `envoy` | 100m / 128Mi | 500m / 256Mi |

The `csi-external-provisioner`, `csi-external-resizer` and `csi-external-snapshotter` sidecars keep the base manifests' defaults (no requests or limits). They are not on Substrate's request path, which goes from `ate-api-server` through `envoy` to the driver, and they use only a few MiB of memory.

### Node DaemonSet (`node_patch.yaml`)

- **Burstable QoS:** requests cover steady state, and limits leave room for mount/unmount bursts. Requests are reserved on every node, so they are kept small.

| Container | Requests (CPU / Memory) | Limits (CPU / Memory) |
| :--- | :--- | :--- |
| `gcp-filestore-driver` | 1 / 1Gi | 3 / 6Gi |

The `csi-driver-registrar` and `nfs-services` sidecars keep the base manifests' defaults (no requests or limits); they use only a few MiB of memory and very little CPU.

### Common settings

- **CPU limits and threads:** the driver's Go runtime (Go 1.25 and later) sizes `GOMAXPROCS` to the container's CPU limit automatically, so no `GOMAXPROCS` setting is needed. `envoy` does not adjust to the CPU limit by itself, so it runs with `--concurrency 2` instead of one worker thread per host core.
- **Logging:** the driver runs with `--v=2`, which drops per-RPC request/response logs (`V(4)`/`V(5)`) but keeps INFO logs for debugging. The controller still logs INFO lines for each VolumePool volume it creates or deletes.
- **Priority:** both workloads use the `csi-gcp-fs-controller` and `csi-gcp-fs-node` PriorityClasses from the base manifests. No change is needed in this overlay.
- **Filestore API quota:** every `CreateVolume` and `DeleteVolume` results in one Filestore API request. Make sure the project's Filestore API request quota covers your expected rate. For example, 40 RPS of creates plus 40 RPS of deletes is about 4,800 requests per minute. Request a quota increase if needed.

---

## 🔄 Upgrading an Existing Install

`spec.selector` on the Deployment and DaemonSet, and `spec.clusterIP` on the Service, cannot be changed in place. Installs created before the `role` selectors and the headless Service were added therefore need a one-time re-create before applying this overlay. `deploy.sh` does not do this for you.

```bash
kubectl -n gcp-filestore-csi-driver delete \
    deployment/gcp-filestore-csi-controller \
    daemonset/gcp-filestore-csi-node \
    service/csi-filestore-controller \
    --ignore-not-found
# Wait for the old pods to exit: the node pods use the host network and share the
# rpcbind port and CSI socket path with their replacements.
kubectl -n gcp-filestore-csi-driver wait --for=delete pod -l app=gcp-filestore-csi-driver --timeout=180s
```

Then deploy as usual with `deploy.sh` or `kubectl apply -k`. Volumes that are already mounted keep working while the node DaemonSet is re-created, but new stage/unstage requests fail until the new node pods register. Clients that were connected through the old ClusterIP, such as `ate-api-server`, need to reconnect; if their requests keep failing, restart them.
