# CSI driver FsGroup User Guide

>**Attention:** This user guide to apply fsgroup to volumes provisioned by filestore driver is applicable for `stable-master` overlay driver manifest bundle, deployed to kubernetes 1.19+ clusters. For 1.19 (CSIVolumeFSGroupPolicy feature gate needs to be manually enabled). For a workaround to apply fsgroup on clusters 1.19 (with CSIVolumeFSGroupPolicy feature gate disabled), and clusters <= 1.18 see user-guide [here](fsgroup-workaround.md)

>**Attention:** `CSIVolumeFSGroupPolicy` is a Kubernetes feature which is Beta is 1.20+, Alpha(1.19).

>**Attention:** CSIDriver object `fsGroupPolicy` field is added in Kubernetes 1.19 (alpha) and cannot be set when using an older Kubernetes release. For 1.20+ k8s versions the feature is be enabled by default.

Kubernetes uses fsGroup to change permissions and ownership of the volume to match user requested fsGroup in the pod's [SecurityContext](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-pod). Kubernetes feature `CSIVolumeFSGroupPolicy` is a beta feature in K8s 1.20+ by which CSI drivers can explicitly declare support for fsgroup. Read more about `CSIVolumeFSGroupPolicy` [here](https://kubernetes-csi.github.io/docs/csi-driver-object.html) and [here](https://kubernetes-csi.github.io/docs/support-fsgroup.html).


### FsGroup example

1. Create `StorageClass`

    If you haven't created a `StorageClass` yet, create one first:

    ```console
    $ kubectl apply -f ./examples/kubernetes/fsgroup/demo-sc.yaml
    ```

    If a non-default network is used for the filestore instance, provide a network paramter to the storage class.

    ```yaml
    apiVersion: storage.k8s.io/v1
    kind: StorageClass
    metadata:
    name: csi-filestore
    provisioner: filestore.csi.storage.gke.io
    parameters:
      network: <network name> # Change this network as per the deployment
    volumeBindingMode: WaitForFirstConsumer
    allowVolumeExpansion: true
    ```

2. Check the CSI driver object for the filestore driver. It should report `fsGroupPolicy: File`

    ```console
    $ kubectl get csidriver filestore.csi.storage.gke.io -o json
    {
        "apiVersion": "storage.k8s.io/v1",
        "kind": "CSIDriver",
        ...
        "spec": {
            "attachRequired": false,
            "fsGroupPolicy": "File",
            "podInfoOnMount": true,
            "volumeLifecycleModes": [
                "Persistent"
            ]
        }
    }
    ```

3. Create Pod with fsgroup and using a PVC with ReadWriteMany access mode, provisioned by CSI Filestore driver.
    ```console
    $ kubectl apply -f ./examples/kubernetes/fsgroup/pod-with-fsgroup.yaml
    ```

4. Verify that the pod is up and running and fsgroup ownerhsip change is applied in the volume.
  ```console
  $ kubectl exec busybox-pod -- ls -l /tmp
  total 16
  drwxrws---    2 root     4000         16384 Jan 27 04:27 lost+found
  ```
