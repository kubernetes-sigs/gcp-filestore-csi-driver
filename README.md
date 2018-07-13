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

Latest image: `gcr.io/google-containers/volume-csi/gcp-filestore-csi-driver:v0.1.0`

Also see [known issues](KNOWN_ISSUES.md) and [CHANGELOG](CHANGELOG.md).

### CSI Compatibility
This plugin is compatible with CSI version 0.2.0 and 0.3.0.

### Kubernetes Compatibility
This plugin can be used beginning with Kubernetes v1.10.5

## Plugin Features

### Supported CreateVolume parameters
This version of the driver creates a new Cloud Filestore instance per
volume. Customizable parameters for volume creation include:

| Parameter       | Values                  | Default    | Description |
| --------------- | ----------------------- |----------- | ----------- |
| tier            | "standard"<br>"premium" | "standard" | storage performance tier |
| network         | string                  | "default"  | VPC name |
| location        | string                  | zone where the plugin<br>is running in | zone |

For Kubernetes clusters, these parameters are specified in the StorageClass.

Note that non-default networks require extra [firewall setup](https://cloud.google.com/filestore/docs/configuring-firewall)

## Future Features
* Reserved IP range: To avoid IP conflict issues, add a CreateVolume parameter
  to provide an IP block that is reserved for dynamically provisioning GCFS instances.
* Non-root access: By default, GCFS instances are only writable by the root user
  and readable by all users. Provide a CreateVolume parameter to set non-root
  owners.
* Subdirectory provisioning: Given an existing Cloud Filestore instance, provision a
  subdirectory as a volume. This provisioning mode does not provide capacity
  isolation. Quota support needs investigation. For now, the
  [nfs-client](https://github.com/kubernetes-incubator/external-storage/tree/master/nfs-client)
  external provisioner can be used to provide similar functionality for
  Kubernetes clusters.
* Volume resizing: CSI does not have volume resizing support yet, but Cloud Filestore
  instances can currently be [manually resized](https://cloud.google.com/filestore/docs/editing-instances).
* Topology preferences: For better performance, it is recommended to run
  workloads in the same zone where the Cloud Filestore instance is provisioned in. In the
  future, the location where to create a Cloud Filestore instance could be automatically
  influenced by where the workload is scheduled.

## Kubernetes User Guide
1. One-time per project: Create GCP service account for the CSI driver and set the Cloud
   Filestore editor role.
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
kubectl apply -f ./examples/demo-sc.yaml
```
3. Create example PVC and Pod
```
kubectl apply -f ./examples/demo-pod.yaml
```

## Kubernetes Development
[Setup](#kubernetes-user-guide) GCP service account first and setup Kubernetes cluster
```
$ ./deploy/project_setup.sh
$ ./deploy/kubernetes/cluster_setup.sh
```

### Manual
```
$ make
$ make push
# Modify manifests under deploy/kubernetes/manifests to use development image
$ ./deploy/kubernetes/driver_start.sh
```

### Automatic using [Skaffold](http://github.com/GoogleContainerTools/skaffold) and [Kustomize](https://github.com/kubernetes-sigs/kustomize)
1. Modify deploy/skaffold/skaffold.yaml and deploy/kubernetes/manifests/dev/
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
