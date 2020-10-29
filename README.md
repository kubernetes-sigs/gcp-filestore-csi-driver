# gcp-filestore-csi-driver
[Google Cloud Filestore](https://cloud.google.com/filestore) CSI driver for
use in Kubernetes and other container orchestrators.

Disclaimer: This is not an officially supported Google product.

## Project Overview
This driver allows volumes backed by Google Cloud Filestore instances to be
dynamically created and mounted by workloads.

If multiple volumes are not needed, then Google Cloud Filestore instances can be
manually created without this CSI driver and mounted using existing NFS volume
plugins. Please see the Cloud Filestore
[documentation](https://cloud.google.com/filestore/docs/accessing-fileshares)
for more details.

## Project Status
Status: Alpha

Latest image: `gke.gcr.io/gcp-filestore-csi-driver:v0.2.0`

Also see [known issues](KNOWN_ISSUES.md) and [CHANGELOG](CHANGELOG.md).

### CSI Compatibility
This plugin is compatible with CSI version 1.3.0.

### Kubernetes Compatibility

| Filestore CSI Driver\Kubernetes Version | 1.12 | 1.13 | 1.14 | 1.15 | 1.16+ |
| --------------------------------------- | ---- | ---- | ---- | ---- | ----  |
| v0.2.0 (alpha)                          | yes  | yes  | yes  |  no  |  no   |
| master                                  | no   | no   | yes  |  yes |  yes  |

## Plugin Features

### Supported CreateVolume parameters
This version of the driver creates a new Cloud Filestore instance per
volume. Customizable parameters for volume creation include:

| Parameter         | Values                  | Default                                | Description |
| ---------------   | ----------------------- |-----------                             | ----------- |
| tier              | "standard"<br>"premium" | "standard"                             | storage performance tier |
| network           | string                  | "default"                              | VPC name |
| reserved-ipv4-cidr| string		              | ""                                     | CIDR range to allocate Filestore IP Ranges from.<br>The CIDR must be large enough to accommodate multiple Filestore IP Ranges of /29 each |

For Kubernetes clusters, these parameters are specified in the StorageClass.

Note that non-default networks require extra [firewall setup](https://cloud.google.com/filestore/docs/configuring-firewall)

## Current supported Features
* Volume resizing: CSI Filestore driver supports volume expansion for all supported Filestore tiers.
* Labels: Filestore supports labels per instance, which is a map of key value pairs. Filestore CSI driver enables user provided labels
  to be stamped on the instance. User can provide labels by using 'labels' key in StorageClass.parameters. In addition, Filestore instance can
  be labelled with information about what PVC/PV the instance was created for. To obtain the PVC/PV information, '--extra-create-metadata' flag needs to be set on the CSI external-provisioner sidecar. User provided label keys and values must comply with the naming convention as specified [here](https://cloud.google.com/resource-manager/docs/creating-managing-labels#requirements)
* Topology preferences: Filestore performance and network usage is affected by topology. For example, it is recommended to run
  workloads in the same zone where the Cloud Filestore instance is provisioned in. The following table describes how provisioning can be tuned by topology. The volumeBindingMode is specified in the StorageClass used for provisioning. 'strict-topology' is a flag passed to the CSI provisioner sidecar. 'allowedTopology' is also specified in the StorageClass. The Filestore driver will use the first topology in the preferred list, or if empty the first in the requisite list. If topology feature is not enabled in CSI provisioner (--feature-gates=Topology=false), CreateVolume.accessibility_requirements will be nil, and the driver simply creates the instance in the zone where the driver deployment running.


  | SC Bind Mode         | 'strict-topology' | SC allowedTopology  | CSI provisioner Behavior    |
  | -------------------- | ----------------- | ------------------- | --------------------------- |
  | WaitForFirstCustomer |       true        |        Present      | If the topology of the node selected by the schedule is not in allowedTopology, provisioning fails and the scheduler will continue with a different node. Otherwise, CreateVolume is called with requisite and preferred topologies set to that of the selected node |
  | WaitForFirstCustomer |       false       |        Present      | If the topology of the node selected by the schedule is not in allowedTopology, provisioning fails and the scheduler will continue with a different node. Otherwise, CreateVolume is called with requisite set to allowedTopology and preferred set to allowedTopology rearranged with the selected node topology as the first parameter |
  | WaitForFirstCustomer |       true        |        Not Present  | Call CreateVolume with requisite set to selected node topology, and preferred set to the same |
  | WaitForFirstCustomer |       false       |        Not Present  | Call CreateVolume with requisite set to aggregated topology across all nodes, which matches the topology of the selected node, and preferred is set to the sorted and shifted version of requisite, with selected node topology as the first parameter |
  | Immediate            |       N/A         |        Present      | Call CreateVolume with requisite set to allowedTopology and preferred set to the sorted and shifted version of requisite at a randomized index |
  | Immediate            |       N/A         |        Not Present  | Call CreateVolume with requisite = aggregated topology across nodes which contain the topology keys of CSINode objects, preferred = sort and shift requisite at a randomized index |

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

## Kubernetes User Guide
1. One-time per project: Create GCP service account for the CSI driver and set the Cloud
   Filestore editor role. Also enable Cloud Filestore API for this project.
```
# Optionally set a different directory to download the service account token.
# Default is $HOME.
# GCFS_SA_DIR=/another/directory
./deploy/project_setup.sh
```
2. Deploy driver to Kubernetes cluster
```
./deploy/kubernetes/cluster_setup.sh
./deploy/kubernetes/driver_start.sh
```
3. Create example StorageClass
```
kubectl apply -f ./examples/kubernetes/demo-sc.yaml
```
3. Create example PVC and Pod
```
kubectl apply -f ./examples/kubernetes/demo-pod.yaml
```

## Kubernetes Development
Setup GCP service account first and setup Kubernetes cluster
```
# The first step would be create a service account with appropriate role bindings. The following command creates gcp-filestore-csi-driver-sa@<your-gcp-project>.iam.gserviceaccount.com and grants roles/file.editor role to the service account.
For dev overlay use,
$ PROJECT=saikatroyc-gke-dev DEPLOY_VERSION=dev ./deploy/project_setup.sh
# Else, for any other overlay,
$ PROJECT=<your-gcp-project> GCFS_SA_DIR=<your-directory-to-store-credentials-by-default-home-dir> ./deploy/project_setup.sh

# Build the Filestore CSI driver image and push to a container registry.
$ PROJECT=<your-gcp-project> make build-image-and-push

# The base manifests like core driver manifests, rbac role bindings are listed under deploy/kubernetes/base.
# The overlays (e.g prow-gke-release-staging-head, prow-gke-release-staging-rc, stable, noauth) are listed under deploy/kubernetes/overlays
# apply transformations on top of the base manifests.
# For setup of the cluster with one of the overlays (other than 'dev' overlay) use:
$ PROJECT=<your-gcp-project> GCFS_SA_DIR=<path-to-credentials-file> DEPLOY_VERSION=<overlay-name> ./deploy/kubernetes/cluster_setup.sh
'path-to-credentials-file' is the path where the key file was saved (e.g. $GCFS_SA_DIR/gcp_filestore_csi_driver_sa.json), in the step of running project_setup.sh above.

# 'dev' overlay uses default service account for communicating with GCP services. https://www.googleapis.com/auth/cloud-platform scope allows full access to all Google Cloud APIs and given node scope will allow any pod to reach GCP services as the provided service account, and so should only be used for testing and development, not production clusters. Bring up GCE cluster with following:
$ NODE_SCOPES=https://www.googleapis.com/auth/cloud-platform KUBE_GCE_NODE_SERVICE_ACCOUNT=<SERVICE_ACCOUNT_NAME>@$PROJECT.iam.gserviceaccount.com kubetest --up
$ PROJECT=<your-gcp-project> DEPLOY_VERSION=dev ./deploy/kubernetes/cluster_setup.sh
```

## Gcloud Application Default Credentials and scopes
See [here](https://cloud.google.com/docs/authentication/production), [here](https://cloud.google.com/compute/docs/access/create-enable-service-accounts-for-instances) and [here](https://cloud.google.com/storage/docs/authentication#oauth-scopes)

### Automatic using [Skaffold](http://github.com/GoogleContainerTools/skaffold) and [Kustomize](https://github.com/kubernetes-sigs/kustomize)
1. Modify [Skaffold configuration](deploy/skaffold/skaffold.yaml) and [Kustomize overlays](deploy/kubernetes/manifests/dev/)
   with your image registry
2. Run skaffold
```
$ make skaffold-dev
```

### Dependency Management
Use [dep](https://github.com/golang/dep)
```
$ dep ensure
```
