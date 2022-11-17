# gcp-filestore-csi-driver
[Google Cloud Filestore](https://cloud.google.com/filestore) CSI driver for
use in Kubernetes and other container orchestrators.

Disclaimer: Deploying this driver manually is not an officially supported Google
product. For a fully managed and supported filestore experience on kubernetes,
use [GKE with the managed filestore driver](https://cloud.google.com/kubernetes-engine/docs/how-to/persistent-volumes/filestore-csi-driver).


## Project Overview
This driver allows volumes backed by Google Cloud Filestore instances to be
dynamically created and mounted by workloads.

## Project Status
Status: GA

Latest image: `k8s.gcr.io/cloud-provider-gcp/gcp-filestore-csi-driver:v1.3.9`

Also see [known issues](KNOWN_ISSUES.md) and [CHANGELOG](CHANGELOG.md).

### CSI Compatibility
This plugin is compatible with CSI version 1.3.0.

### Kubernetes Compatibility
The following table captures the compatibility matrix of the core filestore driver binary
`k8s.gcr.io/cloud-provider-gcp/gcp-filestore-csi-driver`

| Filestore CSI Driver\Kubernetes Version | 1.16 | 1.17+ |
| --------------------------------------- | ---- | ----- |
| v0.2.0 (alpha)                          |  no  |  no   |
| v0.3.1 (beta)                           |  yes |  yes  |
| v0.4.0 (beta)                           |  yes |  yes  |
| v0.5.0 (beta)                           |  yes |  yes  |
| v0.6.0 (beta)                           |  yes |  yes  |
| v0.6.1 (beta)                           |  yes |  yes  |
| v1.0.0 (GA)                             |  yes |  yes  |
| v1.1.1 (GA)                             |  yes |  yes  |
| v1.1.2 (GA)                             |  yes |  yes  |
| v1.1.3 (GA)                             |  yes |  yes  |
| v1.1.4 (GA)                             |  yes |  yes  |
| v1.2.0 (GA)                             |  yes |  yes  |
| v1.2.1 (GA)                             |  yes |  yes  |
| v1.2.2 (GA)                             |  yes |  yes  |
| v1.2.3 (GA)                             |  yes |  yes  |
| v1.2.4 (GA)                             |  yes |  yes  |
| v1.2.5 (GA)                             |  yes |  yes  |
| v1.2.7 (GA)                             |  yes |  yes  |
| v1.3.0 (GA)                             |  yes |  yes  |
| v1.3.1 (GA)                             |  yes |  yes  |
| v1.3.2 (GA)                             |  yes |  yes  |
| v1.3.3 (GA)                             |  yes |  yes  |
| v1.3.5 (GA)                             |  yes |  yes  |
| v1.3.7 (GA)                             |  yes |  yes  |
| v1.3.8 (GA)                             |  yes |  yes  |
| v1.3.9 (GA)                             |  yes |  yes  |
| master                                  |  yes |  yes  |

The manifest bundle which captures all the driver components (driver pod which includes the containers csi-external-provisioner, csi-external-resizer, csi-external-snapshotter, gcp-filestore-driver, csi-driver-registrar, csi driver object, rbacs, pod security policies etc) can be picked up from the master branch [overlays](deploy/kubernetes/overlays) directory. We structure the overlays directory per minor version of kubernetes because not all driver components can be used with all kubernetes versions. For example volume snapshots are supported 1.17+ kubernetes versions thus [stable-1-16](deploy/kubernetes/overlays/stable-1-16) driver manifests does not contain the snapshotter sidecar. Read more about overlays [here](docs/release/overlays.md).

Example:
`stable-1-19` overlays bundle can be used to deploy all the components of the driver on kubernetes 1.19.
`stable-master` overlays bundle can be used to deploy all the components of the driver on kubernetes master.

## Plugin Features

### Supported CreateVolume parameters
This version of the driver creates a new Cloud Filestore instance per
volume. Customizable parameters for volume creation include:

| Parameter         | Values                  | Default                                | Description |
| ---------------   | ----------------------- |-----------                             | ----------- |
| tier              | "standard"<br>"premium"<br>"enterprise" | "standard"             | storage performance tier |
| network           | string                  | "default"                              | VPC name<br>When using "PRIVATE_SERVICE_ACCESS" connect-mode, network needs to be the full VPC name |
| reserved-ipv4-cidr| string		              | ""                                     | CIDR range to allocate Filestore IP Ranges from.<br>The CIDR must be large enough to accommodate multiple Filestore IP Ranges of /29 each, /24 if enterprise tier is used |
| reserved-ip-range | string		              | ""                                     | IP range to allocate Filestore IP Ranges from.<br>This flag is used instead of "reserved-ipv4-cidr" when "connect-mode" is set to "PRIVATE_SERVICE_ACCESS" and the value must be an [allocated IP address range](https://cloud.google.com/compute/docs/ip-addresses/reserve-static-internal-ip-address).<br>The IP range must be large enough to accommodate multiple Filestore IP Ranges of /29 each, /24 if enterprise tier is used |
| connect-mode      | "DIRECT_PEERING"<br>"PRIVATE_SERVICE_ACCESS" | "DIRECT_PEERING"  | The network connect mode of the Filestore instance.<br>To provision Filestore instance with shared-vpc from service project, PRIVATE_SERVICE_ACCESS mode must be used |
| instance-encryption-kms-key | string        | ""                                     | Fully qualified resource identifier for the key to use to encrypt new instances. |

For Kubernetes clusters, these parameters are specified in the StorageClass.

Note that non-default networks require extra [firewall setup](https://cloud.google.com/filestore/docs/configuring-firewall)

## Current supported Features
* Volume resizing: CSI Filestore driver supports volume expansion for all supported Filestore tiers. See user-guide [here](docs/kubernetes/resize.md). Volume expansion feature is beta in kubernetes 1.16+.
* Labels: Filestore supports labels per instance, which is a map of key value pairs. Filestore CSI driver enables user provided labels
  to be stamped on the instance. User can provide labels by using 'labels' key in StorageClass.parameters. In addition, Filestore instance can
  be labelled with information about what PVC/PV the instance was created for. To obtain the PVC/PV information, '--extra-create-metadata' flag needs to be set on the CSI external-provisioner sidecar. User provided label keys and values must comply with the naming convention as specified [here](https://cloud.google.com/resource-manager/docs/creating-managing-labels#requirements). Please see [this](examples/kubernetes/sc-labels.yaml) storage class examples to apply custom user-provided labels to the Filestore instance.
* Topology preferences: Filestore performance and network usage is affected by topology. For example, it is recommended to run
  workloads in the same zone where the Cloud Filestore instance is provisioned in. The following table describes how provisioning can be tuned by topology. The volumeBindingMode is specified in the StorageClass used for provisioning. 'strict-topology' is a flag passed to the CSI provisioner sidecar. 'allowedTopology' is also specified in the StorageClass. The Filestore driver will use the first topology in the preferred list, or if empty the first in the requisite list. If topology feature is not enabled in CSI provisioner (--feature-gates=Topology=false), CreateVolume.accessibility_requirements will be nil, and the driver simply creates the instance in the zone where the driver deployment running. See user-guide [here](docs/kubernetes/topology.md). Topology feature is GA in kubernetes 1.17+.


  | SC Bind Mode         | 'strict-topology' | SC allowedTopology  | CSI provisioner Behavior    |
  | -------------------- | ----------------- | ------------------- | --------------------------- |
  | WaitForFirstCustomer |       true        |        Present      | If the topology of the node selected by the schedule is not in allowedTopology, provisioning fails and the scheduler will continue with a different node. Otherwise, CreateVolume is called with requisite and preferred topologies set to that of the selected node |
  | WaitForFirstCustomer |       false       |        Present      | If the topology of the node selected by the schedule is not in allowedTopology, provisioning fails and the scheduler will continue with a different node. Otherwise, CreateVolume is called with requisite set to allowedTopology and preferred set to allowedTopology rearranged with the selected node topology as the first parameter |
  | WaitForFirstCustomer |       true        |        Not Present  | Call CreateVolume with requisite set to selected node topology, and preferred set to the same |
  | WaitForFirstCustomer |       false       |        Not Present  | Call CreateVolume with requisite set to aggregated topology across all nodes, which matches the topology of the selected node, and preferred is set to the sorted and shifted version of requisite, with selected node topology as the first parameter |
  | Immediate            |       N/A         |        Present      | Call CreateVolume with requisite set to allowedTopology and preferred set to the sorted and shifted version of requisite at a randomized index |
  | Immediate            |       N/A         |        Not Present  | Call CreateVolume with requisite = aggregated topology across nodes which contain the topology keys of CSINode objects, preferred = sort and shift requisite at a randomized index |

* Volume Snapshot: The CSI driver currently supports CSI VolumeSnapshots on a GCP Filestore instance using the GCP Filestore Backup feature. CSI VolumeSnapshot is a Beta feature in k8s enabled by default in 1.17+. The GCP Filestore Snapshot [alpha](https://cloud.google.com/sdk/gcloud/reference/alpha/filestore/snapshots/create) is not currently supported, but will be in the future via the type parameter in the VolumeSnapshotClass. For more details see the user-guide [here](docs/kubernetes/backup.md).
* Volume Restore: The CSI driver supports out-of-place restore of new GCP Filestore instance from a given GCP Filestore Backup. See user-guide restore steps [here](docs/kubernetes/backup.md) and GCP Filestore Backup restore documentation [here](https://cloud.google.com/filestore/docs/backup-restore). This feature needs kubernetes 1.17+.
* Pre-provisioned Filestore instance: Pre-provisioned filestore instances can be leveraged and consumed by workloads by mapping a given filestore instance to a PersistentVolume and PersistentVolumeClaim. See user-guide [here](docs/kubernetes/pre-provisioned-pv.md) and filestore documentation [here](https://cloud.google.com/filestore/docs/accessing-fileshares)
* FsGroup: [CSIVolumeFSGroupPolicy](https://kubernetes-csi.github.io/docs/support-fsgroup.html) is a Kubernetes feature in Beta is 1.20, which allows CSI drivers to opt into FSGroup policies. The stable-master [overlay](deploy/kubernetes/overlays/stable-master) of Filestore CSI driver now supports this. See the user-guide [here](docs/kubernetes/fsgroup.md) on how to apply fsgroup to volumes backed by filestore instances. For a workaround to apply fsgroup on clusters 1.19 (with CSIVolumeFSGroupPolicy feature gate disabled), and clusters <= 1.18 see user-guide [here](docs/kubernetes/fsgroup-workaround.md)

## Future Features
* Non-root access: By default, GCFS instances are only writable by the root user
  and readable by all users. Provide a CreateVolume parameter to set non-root
  owners.
* Subdirectory provisioning: Given an existing Cloud Filestore instance, provision a
  subdirectory as a volume. This provisioning mode does not provide capacity
  isolation. Quota support needs investigation. For now, the
  [nfs-client](https://github.com/kubernetes-incubator/external-storage/tree/master/nfs-client)
  external provisioner can be used to provide similar functionality for
  Kubernetes clusters.
* Windows support. The current version of the driver supports volumes mounted to Linux nodes only.

## Deploying the Driver

* Clone the repository in cloudshell using following commands

```
mkdir -p $GOPATH/src/sigs.k8s.io
cd $GOPATH/src/sigs.k8s.io
git clone https://github.com/kubernetes-sigs/gcp-filestore-csi-driver.git
```

* Set up a service account with appropriate role binds, and download a service account key. This
  service account will be used by the driver to provision Filestore instances and otherwise access
  GCP APIs. This can be done by running `./deploy/project_setup.sh` and pointing to a directory to
  store the SA key. To prevent your key from leaking do not make this directory publicly
  accessible!
  
```
$ PROJECT=<your-gcp-project> GCFS_SA_DIR=<your-directory-to-store-credentials-by-default-home-dir> ./deploy/project_setup.sh
```

* Choose a stable overlay that matches your cluster version, eg `stable-1-19`. If you are running a
  more recent cluster version than given here, use `stable-master`. The `prow-*` overlays are for
  testing, and the `dev` overlay is for driver development. `./deploy/kubernetes/cluster-setup.sh`
  will install the driver pods, as well as necessary RBAC and resources.

```
$ PROJECT=<your-gcp-project> DEPLOY_VERSION=<your-overlay-choice> GCFS_SA_DIR=<your-directory-to-store-credentials-by-default-home-dir> ./deploy/kubernetes/cluster_setup.sh
```

  After this, the driver can be used. See `./docs/kubernetes` for further instructions and
  examples.

* For cleanup of the driver run the following:

```
$ PROJECT=<your-gcp-project> DEPLOY_VERSION=<your-overlay-choice> ./deploy/kubernetes/cluster_cleanup.sh
```

## Kubernetes Development

* Set up a service account. Most development uses the `dev` [overlay](deploy/kubernetes/overlays/dev),
  where a service account key is not needed. Otherwise use `GCFS_SA_DIR` as described above.

```
$ PROJECT=<your-gcp-project> DEPLOY_VERSION=dev ./deploy/project_setup.sh
```

* To build the Filestore CSI latest driver image and push to a container registry.
```
$ PROJECT=<your-gcp-project> make build-image-and-push
```

* The base manifests like core driver manifests, rbac role bindings are listed under [here](deploy/kubernetes/base).
  The overlays (e.g prow-gke-release-staging-head, prow-gke-release-staging-rc-{k8s version}, stable-{k8s version}, dev) are listed under deploy/kubernetes/overlays
  apply transformations on top of the base manifests.

* 'dev' overlay uses default service account for communicating with GCP services. `https://www.googleapis.com/auth/cloud-platform` scope allows full access to all Google Cloud APIs and given node scope will allow any pod to reach GCP services as the provided service account, and so should only be used for testing and development, not production clusters. cluster_setup.sh installs kustomize and creates the driver manifests package and deploys to the cluster. Bring up GCE cluster with following:

```
$ NODE_SCOPES=https://www.googleapis.com/auth/cloud-platform KUBE_GCE_NODE_SERVICE_ACCOUNT=<SERVICE_ACCOUNT_NAME>@$PROJECT.iam.gserviceaccount.com kubetest --up
```

* Deploy the driver.

```
$ PROJECT=<your-gcp-project> DEPLOY_VERSION=dev ./deploy/kubernetes/cluster_setup.sh
```

## Gcloud Application Default Credentials and scopes
See [here](https://cloud.google.com/docs/authentication/production), [here](https://cloud.google.com/compute/docs/access/create-enable-service-accounts-for-instances) and [here](https://cloud.google.com/storage/docs/authentication#oauth-scopes)

## Filestore IAM roles and permissions
See [here](https://cloud.google.com/filestore/docs/access-control#iam-access)

## Driver Release [Google internal only]

* For releasing new versions of this driver, googlers should consult [go/filestore-oss-release-process](http://go/filestore-oss-release-process)
