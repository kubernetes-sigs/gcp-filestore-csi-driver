# GKE Substrate Integration Overlay for GCP Filestore CSI Driver

This overlay enables the **GCP Filestore CSI Driver** to integrate natively with **GKE Substrate** and **Filestore VolumePool (FiFA Fast Availability)** architecture.

---

## 🏗️ Architecture & Component Overview

| Component | Manifest File | Rationale |
| :--- | :--- | :--- |
| **GKE Workload Identity** | `serviceaccount_patch.yaml` | Annotates `gcp-filestore-csi-controller-sa` with the GCP Service Account (`iam.gke.io/gcp-service-account`), eliminating static JSON key secrets. |
| **Controller Patch** | `controller_patch.yaml` | 1. Sets `hostNetwork: false` so Workload Identity metadata server requests (`169.254.169.254`) succeed.<br>2. Deploys a privileged `envoy` mTLS proxy bridging TCP port `10000` to Unix domain socket `/csi/csi.sock`, with the image pinned by tag and digest.<br>3. Adds label `role: controller-plugin`.<br>4. Enables `--feature-volume-pools=true`. |
| **Node DaemonSet Patch** | `node_patch.yaml` | 1. Adds label `role: node-plugin`.<br>2. Mounts `/var/lib/ateom-gvisor` (Substrate up to release-0.2) and `/var/lib/ate` (Substrate release-0.3 and later) with `mountPropagation: Bidirectional` so NFS mounts are accessible to Substrate actors. |
| **Envoy mTLS Config** | `envoy_configmap.yaml` | Envoy bootstrap plus two file-based SDS secrets: the serving certificate (`/run/servicedns/credential-bundle.pem`, a `podCertificate` projection) and the client CA bundle (`/run/podidentity-ca/trust-bundle.pem`, a `clusterTrustBundle` projection), with the client SAN pinned to `spiffe://cluster.local/ns/ate-system/sa/ate-api-server`. Kubelet rotates both in place, and Envoy reloads them through SDS `watched_directory` without a restart (see [Certificate rotation](#-certificate-rotation)). |
| **gRPC Controller Service** | `service.yaml` | Creates `Service/csi-filestore-controller` (port `10000` -> targetPort `10000` over mTLS) selecting `app=gcp-filestore-csi-driver` and `role=controller-plugin`, guaranteeing traffic reaches only the controller. |
| **Substrate Driver Config** | `csi_driver_config.yaml` | Registers the driver (`ate.dev/v1`, Substrate v1.0.0-rc.1 and later) with Substrate pointing to the internal cluster DNS gRPC endpoint. |

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

## 🔄 Certificate rotation

The Envoy sidecar serves the controller pod's `servicedns.podcert.ate.dev/identity` certificate, which kubelet issues through a `podCertificate` projected volume. The certificate lives 24 hours by default (`maxExpirationSeconds` defaults to 86400); the Substrate signer asks kubelet to fetch a new one 30 minutes before it expires, and kubelet swaps it into `/run/servicedns` in place. The client CA bundle in `/run/podidentity-ca` is updated the same way when the cluster trust bundle changes.

Envoy reloads a certificate on a `watched_directory` change only when the certificate is delivered through SDS, so `envoy_configmap.yaml` delivers both through file-based SDS (`sds_servicedns_cert.yaml` and `sds_podidentity_validation.yaml`, mounted at `/etc/envoy`). Don't move them back inline under `tls_certificates` or `validation_context`: Envoy would then read them only at startup and keep serving the old certificate after it expires, and every controller CSI call from `ate-api-server` (such as `CreateVolume` and `DeleteVolume`) would fail until the controller restarts.

Only the certificate files in the projected volumes are reloaded live. `path_config_source` has no `watched_directory` on purpose: Envoy then reloads an SDS file only when something is renamed onto that exact path, and kubelet updates a ConfigMap volume by renaming `..data`, never the `sds_*.yaml` links. Like `envoy.yaml`, an edit to `csi-envoy-config` (including the client SAN pin) therefore takes effect only after `kubectl -n gcp-filestore-csi-driver rollout restart deployment/gcp-filestore-csi-controller`.

### Upgrading an existing installation

Installations deployed before this change keep the old configuration until the overlay is re-applied: re-run `deploy.sh`, or `kubectl apply -k deploy/kubernetes/overlays/substrate/` after rendering `serviceaccount_patch.yaml` (see [Option B](#option-b-manual-deployment-via-kustomize)). The controller pod rolls because the Envoy image reference changed, and the new pod starts with the SDS configuration. Applying only the ConfigMap doesn't roll the pod and doesn't fix a running Envoy, which read its configuration at startup. Until the overlay is re-applied, restart the controller before the served certificate's `notAfter` (more often than every 23 hours) to avoid the outage.

To confirm that a running controller has the fix, check the SDS update counter. It is 0 on the old inline configuration and at least 2 (one per SDS secret) on the new one. If it's still 0 after re-applying, run `kubectl -n gcp-filestore-csi-driver rollout restart deployment/gcp-filestore-csi-controller`. With more than one replica, check each pod (`port-forward pod/<name>`).

```bash
kubectl -n gcp-filestore-csi-driver rollout status deployment/gcp-filestore-csi-controller
kubectl -n gcp-filestore-csi-driver port-forward deployment/gcp-filestore-csi-controller 15000:15000 &
PF_PID=$!; sleep 1
curl -s localhost:15000/stats | grep ssl_context_update_by_sds
kill "${PF_PID}"
```

### What to watch

Read these from Envoy's admin API, with a port-forward as above:

```bash
curl -s localhost:15000/stats | grep -E 'ssl_context_update_by_sds|secrets_not_ready|^sds\.'
```

- `listener.0.0.0.0_10000.server_ssl_socket_factory.ssl_context_update_by_sds` goes up by one each time Envoy loads a new certificate or CA bundle: 2 at startup, then one per rotation. Use it, with `/certs`, to confirm rotations. `sds.*.update_success` and `last_updated` in `config_dump` count loads of the SDS files themselves; on v1.34.14 they keep their startup values when a certificate rotates.
- `listener.0.0.0.0_10000.server_ssl_socket_factory.downstream_context_secrets_not_ready` above 0 means Envoy rejected handshakes because an SDS secret wasn't loaded. It only counts handshake attempts, so it stays 0 if no client connected. Envoy fails closed here but still reports itself live and ready, so don't rely on pod status alone.
- `sds.servicedns_cert.update_rejected`, `sds.servicedns_cert.update_failure`, and the same two counters for `sds.podidentity_validation` count SDS file loads that failed (an unreadable or unparsable file, or an invalid secret).
- `sds.servicedns_cert.key_rotation_failed` and `sds.podidentity_validation.key_rotation_failed` count rotations Envoy noticed but couldn't load; it keeps using the previous certificate or bundle.

Known Envoy v1.34.14 behaviour, seen in local testing: if a certificate file is missing when Envoy starts and appears later, Envoy aborts (SIGABRT) and the container restarts. Kubelet writes both projected volumes before the container starts, so this isn't expected in the pod, and a normal start followed by rotations doesn't trigger it.

`envoy_config_test.go` checks this structure, that the SDS secrets name the files the projected volumes provide, the volume mounts, `kustomization.yaml`, and the image pin. With `ENVOY_BINARY` set it also runs two tests against a real Envoy: `TestEnvoyValidate` runs `envoy --mode validate`, which checks the bootstrap only (it doesn't load the SDS secrets or certificate files), and `TestEnvoyReloadsRotatedFiles` starts Envoy on loopback, checks that both SDS secrets load, then rotates the certificate and the CA bundle the way kubelet does and checks that Envoy reloads each one. The binary extracted from the image is a Linux binary for your machine's architecture, so run this on Linux:

```bash
ENVOY_IMAGE=$(sed -n 's/^ *image: \(envoyproxy\/envoy:.*\)$/\1/p' deploy/kubernetes/overlays/substrate/controller_patch.yaml)
docker create --name envoy-bin "${ENVOY_IMAGE}"
docker cp envoy-bin:/usr/local/bin/envoy /tmp/envoy && docker rm envoy-bin
ENVOY_BINARY=/tmp/envoy go test -mod=vendor -v ./deploy/kubernetes/overlays/substrate/
```

### Testing certificate rotation

Run this on a test cluster only. `rotation_test_patch.yaml` sets `maxExpirationSeconds` on the servicedns certificate to 3600, the minimum the API server accepts. The signer then issues 1-hour certificates, and kubelet rotates them about 30 minutes after each is issued.

1. Shorten the certificate lifetime. This rolls out a new controller pod. `POD` is the newest controller pod, so a pod that is still terminating isn't picked; with more than one replica, repeat the checks for each pod:
   ```bash
   kubectl -n gcp-filestore-csi-driver patch deployment gcp-filestore-csi-controller \
       --patch-file deploy/kubernetes/overlays/substrate/rotation_test_patch.yaml
   kubectl -n gcp-filestore-csi-driver rollout status deployment/gcp-filestore-csi-controller
   POD=$(kubectl -n gcp-filestore-csi-driver get pod -l app=gcp-filestore-csi-driver,role=controller-plugin \
       --sort-by=.metadata.creationTimestamp -o jsonpath='{.items[-1:].metadata.name}')
   ```
2. Record the certificate Envoy serves on port `10000`. The Envoy image ships `openssl`, and Envoy sends its certificate before it asks for the client certificate, so no client certificate is needed to see it:
   ```bash
   kubectl -n gcp-filestore-csi-driver exec "${POD}" -c envoy -- sh -c \
       'openssl s_client -connect 127.0.0.1:10000 -alpn h2 </dev/null 2>/dev/null | openssl x509 -noout -serial -dates'
   ```
   It should match the certificate kubelet has written to the volume:
   ```bash
   kubectl -n gcp-filestore-csi-driver exec "${POD}" -c envoy -- \
       openssl x509 -in /run/servicedns/credential-bundle.pem -noout -serial -dates
   ```
3. Check what Envoy has loaded through its admin API. The admin API listens on loopback only and the image has no `curl`, so port-forward. Record the serial in `/certs` and the `ssl_context_update_by_sds` value:
   ```bash
   kubectl -n gcp-filestore-csi-driver port-forward "pod/${POD}" 15000:15000 &
   PF_PID=$!; sleep 1
   curl -s localhost:15000/certs | grep -E '"serial_number"|"expiration_time"'
   curl -s localhost:15000/stats | grep -E 'ssl_context_update_by_sds|secrets_not_ready|^sds\..*(update_rejected|update_failure|key_rotation_failed)'
   kill "${PF_PID}"
   ```
4. Wait at least 1 hour (past the first certificate's `notAfter`) without restarting the pod, then repeat steps 2 and 3. The serial number must have changed, `notAfter` must be in the future, the serial Envoy serves must match the file, `ssl_context_update_by_sds` must have gone up by at least one per rotation, and the failure counters must still be 0. Don't use `last_updated` in `config_dump` for this; on v1.34.14 it doesn't change on rotation. The restart counts must not have changed:
   ```bash
   kubectl -n gcp-filestore-csi-driver get pod "${POD}" \
       -o jsonpath='{range .status.containerStatuses[*]}{.name}={.restartCount}{"\n"}{end}'
   ```
5. Confirm that `ate-api-server` can still reach the driver: create an actor with an external volume that uses your VolumePool StorageClass, check that it starts, then delete it. The controller logs should show the matching `CreateVolume` and `DeleteVolume` calls:
   ```bash
   kubectl -n gcp-filestore-csi-driver logs "${POD}" -c gcp-filestore-driver --since=10m | grep -E 'CreateVolume|DeleteVolume'
   ```
6. Restore the default lifetime when done. This patch puts back the production `podCertificate` source from `controller_patch.yaml` (a strategic merge patch replaces the whole `sources` list) and rolls the pod again:
   ```bash
   kubectl -n gcp-filestore-csi-driver patch deployment gcp-filestore-csi-controller --patch \
       '{"spec":{"template":{"spec":{"volumes":[{"name":"servicedns-certs","projected":{"sources":[{"podCertificate":{"signerName":"servicedns.podcert.ate.dev/identity","keyType":"ECDSAP256","credentialBundlePath":"credential-bundle.pem"}}]}}]}}}}'
   ```
   Re-running `deploy.sh` or `kubectl apply -k deploy/kubernetes/overlays/substrate/` (which needs a rendered `serviceaccount_patch.yaml`) should also restore it, because `sources` is replaced as a whole. Either way, check that the field is gone; this prints nothing:
   ```bash
   kubectl -n gcp-filestore-csi-driver get deployment gcp-filestore-csi-controller \
       -o jsonpath='{.spec.template.spec.volumes[?(@.name=="servicedns-certs")].projected.sources[0].podCertificate.maxExpirationSeconds}'
   ```

---

## 🗂️ StorageClass & NFS Mount Options

Agent Substrate reads only `StorageClass.provisioner` and `StorageClass.parameters`, and ignores the top-level `StorageClass.mountOptions` field. Set NFS client mount options for Substrate volumes in `parameters.mount-options`.

### Driver defaults

When a VolumePool NFSv3 volume has no mount options (neither `parameters.mount-options` nor `mountOptions`), the CSI driver mounts it with:

```
vers=3,proto=tcp,mountproto=tcp,port=2049,mountport=2050,nconnect=8,rsize=524288,wsize=524288
```

These defaults are the same for every access mode and are safe when several nodes mount a volume at the same time. They don't include `nolock` or `actimeo`. NFSv4.1 volumes get no defaults.

* The 8-option defaults apply from the first driver release that includes this change. Earlier releases set at most `vers=3,proto=tcp,mountproto=tcp` (for example `v1.14.3`, pinned in the Substrate and Volume Pool overlays; see `kustomization.yaml`), and older releases set none, so an installation keeps those earlier defaults until that image is updated. (Explicit `parameters.mount-options` in the Substrate StorageClass template still takes effect immediately on the pinned image.)
* `port=2049` and `mountport=2050` are fixed (skipping `rpcbind`). Any VolumePool endpoint or test server whose MOUNT daemon is not on port 2050 needs explicit mount options.
* `rsize` and `wsize` set read and write payloads of up to 512 KiB. They cap the transfer size that the client and server would otherwise negotiate.
* `nconnect` requires a Linux client kernel that supports it (5.3 or later; see `nfs(5)` on the node). Kernels older than 5.3 fail to mount with `nconnect=8` and need explicit options without `nconnect` (`parameters.mount-options` for Substrate, `mountOptions` for regular PVCs).
* Linux shares one NFS client per server IP, so `nconnect=8` takes effect for a server IP only once every earlier mount to that IP on the node is unmounted.
* Privileged source-port budget: Linux NFS binds each TCP connection to a privileged source port (`665`–`1023` by default, about 360 per node). With `nconnect=8`, each distinct server IP a node mounts uses 8 ports, so a node caps out at roughly 44 distinct NFS server IPs (fewer with other NFS mounts and `TIME_WAIT` sockets). If a node mounts more distinct server IPs than that, set explicit mount options (`parameters.mount-options` for Substrate, `mountOptions` for regular PVCs) with a lower `nconnect`.

### Substrate StorageClass template

`deploy.sh` and `examples/kubernetes/substrate/storageclass.yaml.tmpl` set these options for dedicated single-actor workspaces, where strictly one node mounts a volume at a time:

```yaml
parameters:
  volume-pool: "projects/<PROJECT_ID>/locations/<LOCATION>/volumePools/<VOLUMEPOOL_NAME>"
  mount-options: "vers=3,proto=tcp,mountproto=tcp,port=2049,mountport=2050,nolock,nconnect=8,rsize=524288,wsize=524288,actimeo=60"
```

`nolock` keeps file locks local to the node, and `actimeo=60` sets `acregmin=60`, `acregmax=60`, `acdirmin=60`, and `acdirmax=60` to cache file and directory attributes for 60 seconds (`/proc/mounts` prints only `acregmin=60,acdirmin=60`, because the max values equal the kernel defaults). Do not use this StorageClass for regular Kubernetes PVCs—neither `ReadWriteOnce` nor `ReadWriteMany` (the driver's `CSIDriver` sets `attachRequired: false`, so even `ReadWriteOnce` PVCs can overlap across two nodes during rolling updates)—or for volumes shared by several actors. `nolock` and `actimeo=60` are safe only when the runtime unmounts the volume on the old node before mounting it on the new one; otherwise, `nolock` gives no cross-node lock exclusion (corruption risk) and `actimeo=60` can serve up to 60 seconds of stale attributes and negative lookups. For volumes several actors share, or when you can't confirm the runtime unmounts on the old node first, use `examples/kubernetes/substrate/storageclass-shared.yaml.tmpl` (driver defaults only). For regular Kubernetes PVCs, use `examples/kubernetes/volumepool/storageclass.yaml`, or `examples/kubernetes/volumepool/storageclass-single-node.yaml` with a `ReadWriteOncePod` PVC (`examples/kubernetes/volumepool/pvc-single-node.yaml`) for volumes one Pod uses at a time.

Other VolumePool StorageClass examples (applied manually; `deploy.sh` only renders `storageclass.yaml.tmpl`):

* `examples/kubernetes/substrate/storageclass-shared.yaml.tmpl` — shared Substrate actor volumes, or when you can't confirm the runtime unmounts on the old node first (the 8 safe defaults without `nolock` and `actimeo=60`). Render and apply it manually:
  ```bash
  sed -e "s|<PROJECT_ID>|${PROJECT_ID}|g" \
      -e "s|<LOCATION>|${VOLUMEPOOL_LOCATION}|g" \
      -e "s|<VOLUMEPOOL_NAME>|${VOLUMEPOOL_NAME}|g" \
      -e "s|<STORAGECLASS_NAME>|substrate-volumepool-shared-sc|g" \
      examples/kubernetes/substrate/storageclass-shared.yaml.tmpl | kubectl apply -f -
  ```
* `examples/kubernetes/volumepool/storageclass.yaml` — regular Kubernetes PVCs (sets the 8 safe defaults in `mountOptions`).
* `examples/kubernetes/volumepool/storageclass-single-node.yaml` and `examples/kubernetes/volumepool/pvc-single-node.yaml` — `ReadWriteOncePod` single-Pod PVC example with `nolock` and `actimeo=60`.

When you write your own `parameters.mount-options`:

* Any value replaces all of the driver defaults, so list every option you want.
* Write one comma-separated string with no spaces. Don't fold it with a YAML `>` or `|` block scalar.

### Changing the StorageClass

StorageClass `provisioner`, `parameters`, `reclaimPolicy` and `volumeBindingMode` can't be changed after the StorageClass is created. To change the mount options, replace or delete and re-create the StorageClass. Existing volumes keep the `mount-options` value they were created with, and new volumes get the new value. Volumes created without mount options get the driver defaults when they're next staged on a node, so a volume still staged on the node keeps its current options until it is unstaged and staged again (and `nconnect` takes effect for a server IP once every earlier mount to that IP on the node is unmounted). While the StorageClass is deleted, creating actors or volumes that use it fails.

If a StorageClass of this driver already exists with different settings (`reclaimPolicy`, `volumeBindingMode`, `allowVolumeExpansion`, `mountOptions`, `allowedTopologies`, or `parameters`), `deploy.sh` leaves it unchanged, writes the rendered manifest to a temporary file, and prints a one-step `kubectl replace --force -f <file>` command (along with the delete + re-run alternative and `--storageclass-name` zero-downtime option). When **only** `parameters.mount-options` differs (for example, the class was created by an earlier version of `deploy.sh` without `mount-options`), `deploy.sh` prints a warning and exits 0 (the driver overlay is already applied). When anything else differs (`volume-pool` or another parameter key, `reclaimPolicy`, `volumeBindingMode`, `allowVolumeExpansion`, `mountOptions`, or `allowedTopologies`), alone or together with `parameters.mount-options`, `deploy.sh` prints an error and exits 1 after deploying the driver so the run isn't reported as a success with the wrong StorageClass. If the existing StorageClass has a different provisioner, it may belong to another driver: `deploy.sh` leaves it unchanged, doesn't suggest deleting or replacing it, and exits 1 after deploying the driver; re-run it with another `--storageclass-name`.
