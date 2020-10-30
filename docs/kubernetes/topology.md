# Kubernetes Topology User Guide

>**Attention:** Topology is a Kubernetes feature enabled by default in 1.14-1.16(Beta) and 1.17+(GA).
>**Attention:** Topology is only available in the driver master version

To access Filestore instances, the Compute engine VM instances (or the kubernetes cluster composed of those instances) must be in the same Google Cloud project and VPC network as the Filestore instance, unless the Filestore instance is on a shared VPC network. Once an instance is created, its authorized network cannot be changed. Filestore instances are zonal resources available to all instances in a given VPC network. CSI topology feature can be leveraged to hand pick a zone (or a candidate set of zones) where an instance can be deployed dynamically. For more details into the CSI topology feature see [here](https://kubernetes-csi.github.io/docs/topology.html)

### CSI Topology with Immediate binding mode Example

This example dynamically provisions a filestore instance and uses storage class `allowedTopologies` parameter to pick the zone where a filestore instance is deployed.

1. Create `StorageClass`

    ```console
    $ kubectl apply -f ./examples/kubernetes/topology/immediate-binding/sc-immediate-allowedtopo.yaml
    ```
    If the filestore instance is going to use a non-default network, setup the `network`
    
    ```yaml
    apiVersion: storage.k8s.io/v1
    kind: StorageClass
    metadata:
    name: csi-filestore-immediate-binding-allowedtopo
    provisioner: filestore.csi.storage.gke.io
    volumeBindingMode: Immediate
    allowVolumeExpansion: true
    parameters:
      network: <network name> # Change this network as per the deployment
    allowedTopologies:
    - matchLabelExpressions:
      - key: topology.gke.io/zone
        values:
        # Change this to the intended zone (or set of zones).
        - us-central1-a
        - us-central1-b
    ```

2. Wait for PVC to reach 'Bound' status.
   ```console
   $ kubectl get pvc test-pvc-fs-immediate-binding-allowedtopo
  NAME                                        STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS                                  AGE
  test-pvc-fs-immediate-binding-allowedtopo   Bound    pvc-64e6ce36-523d-4172-b3b3-3c1080ab0b9e   1Ti        RWX            csi-filestore-immediate-binding-allowedtopo   5m7s
   ```
3. Verify that the `volumeHandle` captured in the PersistentVolume object specifies the intended zone.
   ```yaml
    kubectl get pv pvc-64e6ce36-523d-4172-b3b3-3c1080ab0b9e -o yaml
    apiVersion: v1
    kind: PersistentVolume
    metadata:
      annotations:
        pv.kubernetes.io/provisioned-by: filestore.csi.storage.gke.io
      creationTimestamp: "2020-11-13T03:48:48Z"
      finalizers:
      - kubernetes.io/pv-protection
    ...
    spec:
      accessModes:
      - ReadWriteMany
      capacity:
        storage: 1Ti
      claimRef:
        apiVersion: v1
        kind: PersistentVolumeClaim
        name: test-pvc-fs-immediate-binding-allowedtopo
        namespace: default
        resourceVersion: "8013"
        uid: 64e6ce36-523d-4172-b3b3-3c1080ab0b9e
      csi:
        driver: filestore.csi.storage.gke.io
        fsType: ext4
        volumeAttributes:
          ip: <Filestore instance IP>
          storage.kubernetes.io/csiProvisionerIdentity: 1605236375597-8081-filestore.csi.storage.gke.io
          volume: vol1
        volumeHandle: modeInstance/us-central1-a/pvc-64e6ce36-523d-4172-b3b3-3c1080ab0b9e/vol1
      persistentVolumeReclaimPolicy: Delete
      storageClassName: csi-filestore-immediate-binding-allowedtopo
      volumeMode: Filesystem
    status:
      phase: Bound
    ```
4. Verify the filestore instance properties

    ```console
    $ gcloud beta filestore instances describe pvc-64e6ce36-523d-4172-b3b3-3c1080ab0b9e --zone us-central1-a
    ```
    ```yaml
      createTime: '2020-11-13T03:46:18.870400740Z'
      fileShares:
      - capacityGb: '1024'
      name: vol1
      nfsExportOptions:
      - accessMode: READ_WRITE
      ipRanges:
      - 10.0.0.0/8
      - 172.16.0.0/12
      - 192.168.0.0/16
      squashMode: NO_ROOT_SQUASH
      labels:
        kubernetes_io_created-for_pv_name: pvc-64e6ce36-523d-4172-b3b3-3c1080ab0b9e
        kubernetes_io_created-for_pvc_name: test-pvc-fs-immediate-binding-allowedtopo
        kubernetes_io_created-for_pvc_namespace: default
        storage_gke_io_created-by: filestore_csi_storage_gke_io
        name: projects/<your-gcp-project>/locations/us-central1-a/instances/pvc-64e6ce36-523d-4172-b3b3-3c1080ab0b9e
      networks:
      - ipAddresses:
        - <Filestore instance IP>
      modes:
      - MODE_IPV4
      network: default
      reservedIpRange: <IP CIDR>
      state: READY
      tier: STANDARD
    ```

5. Ensure that the deployment is up and running.
   ```console
   $ kubectl get deployment
   NAME                               READY   UP-TO-DATE   AVAILABLE   AGE
   web-server-immediate-allowedtopo   5/5     5            5           6m10s
   ```

### CSI Topology with WaitForFirstCustomer binding mode Example

The steps are same as Immediate mode binding. Use the following yamls `./examples/kubernetes/topology/delayed-binding/sc-delayed-allowedtopo.yaml` and `./examples/kubernetes/topology/delayed-binding/demo-deployment-delayed-allowedtopo.yaml`.
If the topology of the node selected by the scheduler is not in `allowedTopology` parameter of StorageClass, provisioning fails
and the scheduler will continue with a different node.