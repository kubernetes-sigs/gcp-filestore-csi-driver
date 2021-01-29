# Accessing filestore instance from workloads in multiple clusters

>**Pre-requisites:** Filestore driver must be installed on all the clusters. Follow the driver install steps [here](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/blob/master/README.md#kubernetes-development)

>**Pre-requisites:** Ensure that the clusters are created in the same [VPC](https://cloud.google.com/vpc/docs/vpc) as the filestore instance

>**Attention:** Care must be taken in deleting the PersistentVolume resources, as there may be workloads in different clusters which may still point to the given filestore instance. Failure to do so, may result in pods stuck in a `Terminating` state.

The following example demonstrates the usage of a dynamically created filestore instance by workloads deployed in two clusters in a single gcloud project (say `ClusterA` and `ClusterB`). The same example can be extended to multiple clusters. One of the cluster (here `ClusterA`) would dynamically create the filestore instance and map to the pods of a deployment in that cluster. Other clusters will re-use the filestore instance in a pre-provisioned PersitentVolume and use it in workloads.

1. Create `StorageClass` in all the clusters. In this example, ReclaimPolicy: `Retain` is used.

    If you haven't created a `StorageClass` yet, create one first:

    ```console
    $ kubectl apply -f ./examples/kubernetes/cross-cluster/sc.yaml
    ```

    If a non-default network is used for the filestore instance, provide a network parameter to the storage class.

    ```yaml
    apiVersion: storage.k8s.io/v1
    kind: StorageClass
    metadata:
      name: csi-filestore
    provisioner: filestore.csi.storage.gke.io
    reclaimPolicy: Retain
    parameters:
      network: <network name> # Change this network as per the deployment
    volumeBindingMode: WaitForFirstConsumer
    allowVolumeExpansion: true
    ```
2. In `clusterA`, create the deployment as follows. 

    ```console
    $ kubectl apply -f ./examples/kubernetes/cross-cluster/demo-deployment-clusterA.yaml
    ```
    
    As part of the workflow to bringup the deployment, the CSI filestore driver in Cluster A would dynamically create the filestore instance and map to the PersistentVolume used by the deployment.

    ```console
    $ kubectl get pvc
    NAME          STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS    AGE
    test-pvc-fs   Bound    pvc-88959393-01c7-415f-aac4-a974c45f7ec7   1Ti        RWX            csi-filestore   4m17s
    ```

    The PersistentVolume is created with a ReclaimPolicy `Retain`.
    ```console
    $ kubectl get pv pvc-88959393-01c7-415f-aac4-a974c45f7ec7
    NAME                                       CAPACITY   ACCESS MODES   RECLAIM POLICY   STATUS   CLAIM                 STORAGECLASS    REASON   AGE
    pvc-88959393-01c7-415f-aac4-a974c45f7ec7   1Ti        RWX            Retain           Bound    default/test-pvc-fs   csi-filestore            2m37s
    ```

    From `clusterA`, ensure the pods are up and running
    ```console
    $ kubectl get pod
    NAME                                                      READY   STATUS    RESTARTS   AGE
    web-server-deployment-cluster-a-6fc54d4776-7lzkb          1/1     Running   0          3m3s
    web-server-deployment-cluster-a-6fc54d4776-nspm7          1/1     Running   0          3m3s
    web-server-deployment-cluster-a-6fc54d4776-qdttl          1/1     Running   0          3m3s
    ```
3. Capture the filestore instance details i.e the filestore instance name, share name from `VolumeHandle` and the filestore instance IP from `VolumeAttributes`.

    ```console
    $ kubectl describe pv pvc-88959393-01c7-415f-aac4-a974c45f7ec7
    ```

    ```yaml
    Name:            pvc-88959393-01c7-415f-aac4-a974c45f7ec7
    Labels:          <none>
    Annotations:     pv.kubernetes.io/provisioned-by: filestore.csi.storage.gke.io
    Finalizers:      [kubernetes.io/pv-protection]
    StorageClass:    csi-filestore
    Status:          Bound
    Claim:           default/test-pvc-fs
    Reclaim Policy:  Retain
    Access Modes:    RWX
    VolumeMode:      Filesystem
    Capacity:        1Ti
    Node Affinity:   <none>
    Message:         
    Source:
        Type:              CSI (a Container Storage Interface (CSI) volume source)
        Driver:            filestore.csi.storage.gke.io
        FSType:            ext4
        VolumeHandle:      modeInstance/us-central1-c/pvc-88959393-01c7-415f-aac4-a974c45f7ec7/vol1
        ReadOnly:          false
        VolumeAttributes:      ip=10.159.102.90
                            storage.kubernetes.io/csiProvisionerIdentity=1608006582064-8081-filestore.csi.storage.gke.io
                            volume=vol1
    Events:                <none>
    ```

    The `volumeHandle` can be directly obtained as follows:
    ```console
    $ kubectl get pv pvc-88959393-01c7-415f-aac4-a974c45f7ec7 -o "jsonpath={.spec.csi.volumeHandle}"
    modeInstance/us-central1-c/pvc-88959393-01c7-415f-aac4-a974c45f7ec7/vol1
    ```
    The filestore instance IP can be obtained as follows:
    ```console
    $ kubectl get pv pvc-88959393-01c7-415f-aac4-a974c45f7ec7 -o "jsonpath={.spec.csi.volumeAttributes.ip}"
    10.159.102.90
    ```

3. Switch context to the second cluster `ClusterB` and create a pre-provisioned PersistentVolume resource pointing to the filestore instance

    Edit the volumeHandle field in examples/kubernetes/cross-cluster/preprov-pv-clusterB.yaml to point to the right filestore instance.
    ```console
    $ kubectl apply -f examples/kubernetes/cross-cluster/preprov-pv-clusterB.yaml
    ```

    ```console
    $ kubectl apply -f examples/kubernetes/cross-cluster/demo-deployment-clusterB.yaml
    deployment.apps/web-server-deployment-cluster-b created
    persistentvolumeclaim/test-pvc-fs created
    ```

    ```console
    $ kubectl get pvc
    NAME          STATUS   VOLUME   CAPACITY   ACCESS MODES   STORAGECLASS    AGE
    test-pvc-fs   Bound    pre-pv   1Ti        RWX            csi-filestore   3s
    ```

    ```console
    $ kubectl get pod
    NAME                                                        READY   STATUS    RESTARTS   AGE
    web-server-deployment-cluster-b-6fc54d4776-gl5nl            1/1     Running   0          30s
    web-server-deployment-cluster-b-6fc54d4776-jdx4f            1/1     Running   0          30s
    web-server-deployment-cluster-b-6fc54d4776-jn766            1/1     Running   0          30s
    ```

4. Ensure writes work across clusters.

    From `ClusterB`,
    ```console
    $ kubectl exec web-server-deployment-cluster-b-6fc54d4776-gl5nl -- touch /usr/share/nginx/html/testfile
    $ kubectl exec web-server-deployment-cluster-b-6fc54d4776-gl5nl -- ls /usr/share/nginx/html
    lost+found
    testfile

    $ kubectl exec web-server-deployment-cluster-b-6fc54d4776-jdx4f -- ls /usr/share/nginx/html
    lost+found
    testfile

    $ kubectl exec web-server-deployment-cluster-b-6fc54d4776-jn766 -- ls /usr/share/nginx/html
    lost+found
    testfile
    ```

    Switch context to `ClusterA`. Check that the deployments in the `ClusterA`, pointing to the same filestore instance, can access the newly created file.
    ```console
    $ kubectl exec web-server-deployment-cluster-a-6fc54d4776-7lzkb -- ls /usr/share/nginx/html
    lost+found
    testfile

    $ kubectl exec web-server-deployment-cluster-a-6fc54d4776-nspm7 -- ls /usr/share/nginx/html
    lost+found
    testfile

    $ kubectl exec web-server-deployment-cluster-a-6fc54d4776-qdttl -- ls /usr/share/nginx/html
    lost+found
    testfile
    ```

5. Deletion of workloads and the volume resources.

    Since the same filestore instance is shared across clusters, and the workload in one cluster is unaware of the filestore instance being shared by a different cluster, care must be taken by the user to ensure that no running workload is pointing to the filestore instance at the time of deletion.

    Delete the pods in all the clusters
    ```console
    $ kubectl delete deployment web-server-deployment-cluster-a
    deployment.apps "web-server-deployment-cluster-a" deleted
    ```

    Ensure the PersistentVolumeClaim (PVC) still exists in `ClusterA`
    ```console
    $ kubectl get pvc test-pvc-fs
    NAME          STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS    AGE
    test-pvc-fs   Bound    pvc-88959393-01c7-415f-aac4-a974c45f7ec7   1Ti        RWX            csi-filestore   27m
    ```

    Switch to `clusterB` and delete pods.
    ```console
    $ kubectl delete deployment web-server-deployment-cluster-b
    deployment.apps "web-server-deployment-cluster-b" deleted
    ```

    Ensure PVC still exists in `ClusterB`
    ```console
    $ kubectl get pvc test-pvc-fs
    NAME          STATUS   VOLUME   CAPACITY   ACCESS MODES   STORAGECLASS    AGE
    test-pvc-fs   Bound    pre-pv   1Ti        RWX            csi-filestore   7m57s
    ```

    At this point no running pod in any cluster is using the filestore instance. We can proceed with deletion of the PVC. In this example we choose to delete the PVC from the `clusterA` first. Since the PV has a Reclaim policy of `Retain`, deletion of PVC does not trigger the deletion of the PersistentVolume or the underlying filestore instance. The PV enters a `Released` phase when the PVC is deleted. For more details into Reclaim policy see [here](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#reclaiming).

    ```console
    $ kubectl delete pvc test-pvc-fs

    $ kubectl get pvc test-pvc-fs
    Error from server (NotFound): persistentvolumeclaims "test-pvc-fs" not found

    $ kubectl get pv pvc-88959393-01c7-415f-aac4-a974c45f7ec7
    NAME                                       CAPACITY   ACCESS MODES   RECLAIM POLICY   STATUS     CLAIM                 STORAGECLASS    REASON   AGE
    pvc-88959393-01c7-415f-aac4-a974c45f7ec7   1Ti        RWX            Retain           Released   default/test-pvc-fs   csi-filestore            29m
    ```

    Similarly, switch context to `ClusterB` and delete the PVC and check that the PV is not deleted.
    ```console
    $ kubectl get pv pre-pv
    NAME     CAPACITY   ACCESS MODES   RECLAIM POLICY   STATUS     CLAIM                 STORAGECLASS    REASON   AGE
    pre-pv   1Ti        RWX            Retain           Released   default/test-pvc-fs   csi-filestore            16m
    ```

    Delete the PersistentVolume from both the clusters.

    From `ClusterB`
    ```console
    $ kubectl delete pv pre-pv
    persistentvolume "pre-pv" deleted
    ```

    From `ClusterA`
    ```console
    $  kubectl delete pv pvc-88959393-01c7-415f-aac4-a974c45f7ec7
    persistentvolume "pvc-88959393-01c7-415f-aac4-a974c45f7ec7" deleted
    ```

6. Since the PersistentVolumes were created with ReclaimPolicy `Retain`, when the PV is deleted, the underlying filestore instance will not be deleted. If the instance is not longer needed, it can be deleted using filestore delete API.

    ```console
     gcloud filestore instances list
    INSTANCE_NAME                             ZONE           TIER      CAPACITY_GB  FILE_SHARE_NAME  IP_ADDRESS     STATE  CREATE_TIME
    pvc-88959393-01c7-415f-aac4-a974c45f7ec7  us-central1-c  STANDARD  1024         vol1             10.159.102.90  READY  2020-12-15T04:30:18
    ```

    ```console
    $ gcloud filestore instances delete pvc-88959393-01c7-415f-aac4-a974c45f7ec7 --zone us-central1-c
    ```

The above example can also be performed for a RecalimPolicy `Delete`. In that case, deletion of a PVC in just one of the clusters will trigger deletion of the PV and then the underlying filestore instance. The other cluster's PV will point to a nonexistent filestore instance, pods will start failing I/O, and if a pod delete is triggerd, pods will be stuck in a `Terminating` state due to failure of CSI NodeUnpublish calls. So, care must be taken when using a `Delete` policy.
