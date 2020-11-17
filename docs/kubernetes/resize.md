# Kubernetes Resize User Guide

>**Attention:** Volume Resize is a Kubernetes Beta feature enabled by default in 1.16+.
>**Attention:** Volume Resize is only available in the driver version master

### Resize Example

This example dynamically provisions a filestore instance and performs online resize of the instance (i.e while the volume is mounted on a Pod). For more details about CSI VolumeExpansion capability see [here](https://kubernetes-csi.github.io/docs/volume-expansion.html)

1. Ensure resize field `allowVolumeExpansion`is set to True, in the  example Zonal Storage Class
    ```yaml
    apiVersion: storage.k8s.io/v1
    kind: StorageClass
    metadata:
      name: csi-filestore
    provisioner: filestore.csi.storage.gke.io
    volumeBindingMode: WaitForFirstConsumer
    allowVolumeExpansion: true
    ```

2. Create example Zonal Storage Class with resize enabled
    ```
    $ kubectl apply -f ./examples/kubernetes/sc-latebind.yaml
    ```

3. Create example PVC and Pod
    ```
    $ kubectl apply -f ./examples/kubernetes/demo-pod.yaml
    ```

4. Verify PV is created and bound to PVC
    ```
    $ kubectl get pvc test-pvc
    NAME       STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS    AGE
    test-pvc   Bound    pvc-def880a7-b6a6-4b12-b8ad-4fa6d928c142   1Ti        RWX            csi-filestore   9m36s
    ```

5. Verify pod is created and in `RUNNING` state (it may take a few minutes to get to running state)
    ```
    $ kubectl get pods
    NAME                      READY     STATUS    RESTARTS   AGE
    web-server                1/1       Running   0          1m
    ```

6. Check current filesystem size on the running pod
    ```
    $ kubectl exec web-server -- df -h /usr/share/nginx/html
    Filesystem          Size  Used Avail Use% Mounted on
    <Instance IP>:/vol1 1007G   76M  956G   1% /usr/share/nginx/html
    ```

7. Get the zone information from the `volumeHandle` of PV spec
    ```console  
    $ kubectl get pv pvc-def880a7-b6a6-4b12-b8ad-4fa6d928c142 -o yaml
    ```
    ```yaml
    apiVersion: v1
    kind: PersistentVolume
    metadata:
      annotations:
        pv.kubernetes.io/provisioned-by: filestore.csi.storage.gke.io
      creationTimestamp: "2020-11-13T05:15:21Z"
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
        name: test-pvc
        namespace: default
        resourceVersion: "18588"
        uid: def880a7-b6a6-4b12-b8ad-4fa6d928c142
      csi:
        driver: filestore.csi.storage.gke.io
        fsType: ext4
        volumeAttributes:
          ip: <Filestore Instance IP>
          storage.kubernetes.io/csiProvisionerIdentity: 1605236375597-8081-filestore.csi.storage.gke.io
          volume: vol1
        volumeHandle: modeInstance/us-central1-b/pvc-def880a7-b6a6-4b12-b8ad-4fa6d928c142/vol1
      persistentVolumeReclaimPolicy: Delete
      storageClassName: csi-filestore
      volumeMode: Filesystem
    status:
      phase: Bound
      ```
7. Verify the filestore instance properties:

    ```console
    $ gcloud beta filestore instances describe pvc-def880a7-b6a6-4b12-b8ad-4fa6d928c142 --zone us-central1-b
    ```

    ```yaml
        createTime: '2020-11-13T05:13:10.929454142Z'
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
          kubernetes_io_created-for_pv_name: pvc-def880a7-b6a6-4b12-b8ad-4fa6d928c142
          kubernetes_io_created-for_pvc_name: test-pvc
          kubernetes_io_created-for_pvc_namespace: default
          storage_gke_io_created-by: filestore_csi_storage_gke_io
        name: projects/<your-gcp-project>/locations/us-central1-b/instances/pvc-def880a7-b6a6-4b12-b8ad-4fa6d928c142
        networks:
        - ipAddresses:
        - <Instance IP>
        modes:
        - MODE_IPV4
        network: default
        reservedIpRange: <IP CIDR>
        state: READY
        tier: STANDARD
    ```

7. Resize volume by modifying the field `spec -> resources -> requests -> storage`
    ```
    $ kubectl edit pvc test-pvc
    apiVersion: v1
    kind: PersistentVolumeClaim
    ...
    spec:
      resources:
        requests:
          storage: 2Ti
      ...
    ...
    ```
8. Verify the PVC and PV reflect the new changes
    ```
    $ kubectl get pvc test-pvc
    NAME       STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS    AGE
    test-pvc   Bound    pvc-86fb8466-68ee-48a8-bca8-d6c02538962f   2Ti        RWX            csi-filestore   11m

    $ kubectl get pv pvc-86fb8466-68ee-48a8-bca8-d6c02538962f
    NAME                                       CAPACITY   ACCESS MODES   RECLAIM POLICY   STATUS   CLAIM              STORAGECLASS    REASON   AGE
    pvc-86fb8466-68ee-48a8-bca8-d6c02538962f   2Ti        RWX            Delete           Bound    default/test-pvc   csi-filestore            9m46s
    ```

9. Verify filesystem resized on the running pod
    ```
    $  kubectl exec web-server -- df -h /usr/share/nginx/html
    Filesystem          Size  Used Avail Use% Mounted on
    <Instance IP>:/vol1  2.0T   70M  1.9T   1% /usr/share/nginx/html
    ```
10. Verify the filestore instance properties:

    ```console
    $ gcloud beta filestore instances describe pvc-def880a7-b6a6-4b12-b8ad-4fa6d928c142 --zone us-central1-b
    ```

    ```yaml
        createTime: '2020-11-13T05:13:10.929454142Z'
        fileShares:
        - capacityGb: '2048' # Size doubled
        name: vol1
        nfsExportOptions:
        - accessMode: READ_WRITE
        ipRanges:
        - 10.0.0.0/8
        - 172.16.0.0/12
        - 192.168.0.0/16
        squashMode: NO_ROOT_SQUASH
        labels:
          kubernetes_io_created-for_pv_name: pvc-def880a7-b6a6-4b12-b8ad-4fa6d928c142
          kubernetes_io_created-for_pvc_name: test-pvc
          kubernetes_io_created-for_pvc_namespace: default
          storage_gke_io_created-by: filestore_csi_storage_gke_io
        name: projects/<your-gcp-project>/locations/us-central1-b/instances/pvc-def880a7-b6a6-4b12-b8ad-4fa6d928c142
        networks:
        - ipAddresses:
        - <Instance IP>
        modes:
        - MODE_IPV4
        network: default
        reservedIpRange: <IP CIDR>
        state: READY
        tier: STANDARD
    ```