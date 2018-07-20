# v0.1.0
* IP conflicts: By default, Cloud Filestore creation will pick an unused IP range to allocate its
  service in. This may conflict with other GCP services that do not explicitly
  reserve IP ranges, such as GKE non-IP alias clusters or GKE TPUs. To avoid
  IP conflicts, it is recommended to either:
    * Use a GKE cluster with [Alias IPs](https://cloud.google.com/kubernetes-engine/docs/how-to/alias-ips)
        * This will prevent IP conflicts with GKE Pod and Service IPs, but not TPUs.
    * Explicitly allocate IP ranges to each GCP service, and this plugin.
        * IP range reservation for this driver is a future enhancement.
        * GKE Pod and Service CIDRs can be reserved during [cluster creation](https://cloud.google.com/sdk/gcloud/reference/container/clusters/create)
          using the `--cluster-ipv4-cidr` flag.
        * GKE TPU CIDRs can be reserved during [cluster
          creation](https://cloud.google.com/sdk/gcloud/reference/beta/container/clusters/create)
          using the `--tpu-ipv4-cidr` flag.
* Locality of CSI driver and Cloud Filestore instances: If no location is specified in
  the CreateVolume parameters, then by default the driver will pick the zone
  that it is currently running in. This could result in CreateVolume failures if
  Cloud Filestore is not available in the same zone as the driver.
