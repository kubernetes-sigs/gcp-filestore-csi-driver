# Filestore Backups User Guide (Beta)

>**Attention:** Filestore Backup relies on CSI VolumeSnapshot which is a Beta feature in k8s enabled by default in
Kubernetes 1.17+. CSI VolumeSnapshot should not be confused with Filestore Backups. Filestore CSI driver leverages CSI VolumeSnapshot capability to support Filestore Backups by specifying a `type` parameter in the VolumeSnapshotClass object.

>**Attention:** VolumeSnapshot is only available in the driver version "master".

>**Prerequisites:**: Volume Snapshot CRDs and snapshot-controller needs to be installed for the backup example to work. Please refer to [this](https://kubernetes-csi.github.io/docs/snapshot-controller.html#deployment) for additional details. GKE clusters 1.17+ come pre-installed with the above mentioned CRDs and snapshot controller, see [here](https://cloud.google.com/kubernetes-engine/docs/how-to/persistent-volumes/volume-snapshots).

### Backup Example
A Filestore backup is a copy of a file share that includes all file data and metadata of the file share from the point in time when the backup is created. It works for [Basic HDD, Basic SSD, and Enterprise tier instances](https://cloud.google.com/filestore/docs/service-tiers). The tier and volume size of the backup must match the source volume. Once a backup of a file share is created, the original file share can be modified or deleted without affecting the backup. A file share can be completely restored from a backup as a new Filestore instance or onto an existing file share. For more details refer to this [documentation](https://cloud.google.com/filestore/docs/backups)

The [CSI Snapshot](https://github.com/container-storage-interface/spec/blob/master/spec.md#createsnapshot) feature is leveraged to create Filestore Backups. By specifying a `type: backup` field in the VolumeSnapshotClass parameters, filestore CSI driver understands how to initiate a backup for a Filestore instance backed by the Persistent Volume. In future release when Filestore snapshots will be supported, an appropriate `type` parameter will be set in the VolumeSnapshotClass to indicate Filestore snapshots.

1. Create `StorageClass`

    If you haven't created a `StorageClass` yet, create one first:

    ```console
    $ kubectl apply -f ./examples/kubernetes/backups-restore/sc.yaml
    ```

    If a non-default network is used for the filestore instance, provide a network parameter to the storage class.

    ```yaml
    apiVersion: storage.k8s.io/v1
    kind: StorageClass
    metadata:
    name: csi-filestore
    provisioner: filestore.csi.storage.gke.io
    parameters:
      tier: enterprise
      network: <network name> # Change this network as per the deployment
    volumeBindingMode: WaitForFirstConsumer
    allowVolumeExpansion: true
    ```

2. Create default `VolumeSnapshotClass`

    ```console
    $ kubectl create -f ./examples/kubernetes/backups-restore/backup-volumesnapshotclass.yaml
    ```

3. Create source PVC and Pod

    ```console
    kubectl create -f ./examples/kubernetes/backups-restore/source-pod-pvc.yaml
    ```
4. Wait for PVC to reach 'Bound' status.
   ```console
   $ kubectl get pvc source-pvc
   NAME         STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS    AGE
   source-pvc   Bound    pvc-62e3d593-8844-483b-8884-bb315678b3b7   1Ti        RWX            csi-filestore   27m
   ```

5. Generate sample data

    The source PVC is mounted into `/demo/data` directory of this pod. This pod will create a file `sample-file.txt` in `/demo/data` directory. Check if the file has been created successfully:

    ```console
    $ kubectl exec source-pod -- ls /demo/data/
    ```

    The output should be:

    ```
    lost+found
    sample-file.txt
    ```

6. Create a `VolumeSnapshot` of the source PVC (This internally generates Filestore instance backup)

    ```console
    $ kubectl create -f ./examples/kubernetes/backups-restore/backup.yaml
    ```

7. Verify that `VolumeSnapshot` has been created and it is ready to use:

    ```console
    $ kubectl get volumesnapshot backup-source-pvc -o yaml
    ```

    The output is similar to this:

    ```yaml
    apiVersion: snapshot.storage.k8s.io/v1
    kind: VolumeSnapshot
    metadata:
    creationTimestamp: "2020-11-13T03:04:03Z"
    finalizers:
    - snapshot.storage.kubernetes.io/volumesnapshot-as-source-protection
    - snapshot.storage.kubernetes.io/volumesnapshot-bound-protection
    ...
    spec:
        source:
            persistentVolumeClaimName: source-pvc
        volumeSnapshotClassName: csi-gcp-filestore-backup-snap-class
    status:
        boundVolumeSnapshotContentName: snapcontent-191fde18-5eb3-4bb1-9f64-0356765c3f9f
        creationTime: "2020-11-13T03:04:39Z"
        readyToUse: true
        restoreSize: 1Ti
    ```

8. Restore the `VolumeSnapshot` into a new PVC and create a pod to use the PVC:

    Create a new PVC. Specify `spec.dataSource` section to restore from VolumeSnapshot `backup-source-pvc`.

    ```console
    $ kubectl create -f ./examples/kubernetes/backups-restore/restored-pod-pvc.yaml
    ```
9. Wait for PVC to reach 'Bound' status.
   ```console
   $ kubectl get pvc restored-pvc
   NAME           STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS    AGE
   restored-pvc   Bound    pvc-53ced778-6a28-4960-aeb7-82b7bb093981   1Ti        RWX            csi-filestore   4m39s
   ```
   
10. Verify sample data has been restored:

    Check data has been restored in `/demo/data` directory:

    ```console
    $ kubectl exec restored-pod -- ls /demo/data/
    ```

    Verify that the output is:

    ```
    lost+found
    sample-file.txt
    ```
