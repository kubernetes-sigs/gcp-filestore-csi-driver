# CSI driver FsGroup User Guide

>**Attention:** 'CSIVolumeFSGroupPolicy' is a Kubernetes feature which is Beta is 1.20+, Alpha(1.19).
>**Attention:** CSIDriver object 'fsGroupPolicy' field is added in Kubernetes 1.19 and cannot be set when using an older Kubernetes release. This workaround is applicable for 1.19+ k8s versions. For 1.20+ k8s versions the feature will be enabled by default, but the workaround be needed until [issue](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/issues/77) is resolved.

K8s feature ‘CSIVolumeFSGroupPolicy’  is an alpha feature in K8s 1.19 by which CSI drivers can explicitly declare support for fsgroup.
Until the feature is beta, Kubernetes only applies fsgroup to CSI volumes that are RWO (ReadWriteOnce). Kubernetes uses fsGroup to change permissions and ownership of the volume to match user requested fsGroup in the pod's SecurityContext. As a workaround until the CSIVolumeFSGroupPolicy feature is beta, we can deploy a PV backing a filestore instance in RWO mode, apply the FSGroup and then recreate a PV with RWM (ReadWriteMany) mode, so that it can be used for multi reader writer workloads. This workaround does not require pods to run containers as the root user. To read more about CSIVolumeFSGroupPolicy read [here](https://kubernetes-csi.github.io/docs/csi-driver-object.html)


### FsGroup example

1. Create `StorageClass`

    ```console
    $ kubectl apply -f ./examples/kubernetes/fsgroup/demo-sc.yaml
    ```
    If the filestore instance is going to use a non-default network, setup the `network`

2. Create a PV with accessModes `ReadWriteOnce` and ReclaimPolicy `Retain`.

  **Note:** The `volumeHandle` should be updated
  based on the zone, Filestore instance name, and share name created. `storage` value
  should be generated based on the size of the underlying instance. VolumeAttributes `ip` must
  point to the filestore instance IP, and `volume` must point to the [fileshare](https://cloud.google.com/filestore/docs/reference/rest/v1beta1/projects.locations.instances#FileShareConfig) name.

  ```console
  $ kubectl apply -f ./examples/kubernetes/fsgroup/preprov-pv.yaml
  ```

  ```console
  $ kubectl get pvc preprov-pvc
  NAME           STATUS   VOLUME      CAPACITY   ACCESS MODES   STORAGECLASS    AGE
  preprov-pvc   Bound    my-pre-pv   1Ti        RWO            csi-filestore   9m14s
  ```

3. Verify that the pod is up and running and fsgroup ownerhsip change is applied in the volume.
  ```console
  $ kubectl exec busybox-pod -- ls -l /tmp
  total 16
  drwxrws---    2 root     4000         16384 Nov 16 23:25 lost+found
  ```

4. Now the dummy pod and the PVC can be deleted.
   ```console
   $ kubectl delete po busybox-pod
   pod "busybox-pod" deleted
   ```

  Since PVC has 'Retain' policy, the underlying PV and Filestore instance will not be deleted. Once PVC is deleted, PV enters a 'Release' phase.
   ```console
   $ kubectl delete pvc preprov-pvc
   persistentvolumeclaim "preprov-pvc" deleted
   ```

5. Edit the PV to change access mode to RWM, and remove claimRef so that the PV is 'Available' again.
   ```
   $ kubectl patch pv my-pre-pv -p '{"spec":{"accessModes":["ReadWriteMany"]}}'
   $ kubectl patch pv my-pre-pv -p '{"spec":{"claimRef":null}}'
   ```

   ```
   $ kubectl get pv my-pre-pv
   NAME        CAPACITY   ACCESS MODES   RECLAIM POLICY   STATUS      CLAIM   STORAGECLASS    REASON   AGE
   my-pre-pv   1Ti        RWX            Retain           Available           csi-filestore            9m54s
   ```

5. Re-use the same RWX PVC in a multipod deployment and ensure that the deployment is up and running.
   ```console
   $ kubectl apply -f ./examples/kubernetes/fsgroup/demo-deployment.yaml
   ```

   ```console
   $ kubectl get deployment web-server-deployment
   NAME                    READY   UP-TO-DATE   AVAILABLE   AGE
   web-server-deployment   3/3     3            3           12m
   ```

6. Check the volume ownership, by performing exec for each pod of the deployment.
   ```console
   $ kubectl exec web-server-deployment-679dc45b5b-6xdvr -- ls -l /usr/share/nginx/html
   total 16
   drwxrws--- 2 root 4000 16384 Nov 16 23:25 lost+found
   ```

   ```console
   $ kubectl exec web-server-deployment-679dc45b5b-phcxp -- ls -l /usr/share/nginx/html
   total 16
   drwxrws--- 2 root 4000 16384 Nov 16 23:25 lost+found
   ```

   ```console
   $ kubectl exec web-server-deployment-679dc45b5b-z2n8s -- ls -l /usr/share/nginx/html
   total 16
   drwxrws--- 2 root 4000 16384 Nov 16 23:25 lost+found
   ```