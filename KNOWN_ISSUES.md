# v0.1.0
* IP conflicts: By default, Cloud Filestore creation will pick an unused IP subnet to allocate its
  service in. This may conflict with other GCP services that do not explicitly
  reserve IP subnets, such as GKE non-IP alias clusters or GKE TPUs. To avoid
  IP conflicts, it is recommended to explicitly allocate IP subnets to each GCP
  service, and this plugin.
    * IP reservation for this driver is a future enhancement.
    * GKE Pod and Service CIDRs can be reserved during cluster creation using the
      `--cluster-ipv4-cidr` and `--services-ipv4-cidr` flags.
    * GKE TPU CIDRs can be reserved during cluster creation using the
      `--tpu-ipv4-cidr` flag.
* Locality of CSI driver and Cloud Filestore instances: If no location is specified in
  the CreateVolume parameters, then by default the driver will pick the zone
  that it is currently running in. This could result in CreateVolume failures if
  Cloud Filestore is not available in the same zone as the driver.
