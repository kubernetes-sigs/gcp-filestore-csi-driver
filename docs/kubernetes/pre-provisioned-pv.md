# Kubernetes Pre-Provisioned Filestore instance User Guide

This guide gives a simple example on how to use this driver with filestore instances that have
been pre-provisioned by an administrator.

## Pre-Provision a Filestore instance

If you have not already pre-provisioned a filestore instance on GCP you can do that now.

1. Create a filestore instance following the instructions [here](https://cloud.google.com/filestore/docs/creating-instances)


## Create Persistent Volume for Filestore instance

1. Create example Storage Class

```bash
kubectl apply -f ./examples/kubernetes/sc-latebind.yaml
```

This storageclass will not bind a PVC to a PV until there is a pod created using
the PVC. If you wish to bind the PV and PVC immediately on PVC creation, change
`volumeBindingMode` to `Immediate`.

2. Create example Persistent Volume

**Note:** The `volumeHandle` should be updated
based on the zone, Filestore instance name, and share name created. `storage` value
should be generated based on the size of the underlying instance. VolumeAttributes `ip` must
point to the filestore instance IP, and `volume` must point to the [fileshare](https://cloud.google.com/filestore/docs/reference/rest/v1beta1/projects.locations.instances#FileShareConfig) name.

```bash
kubectl apply -f ./examples/kubernetes/pre-provision/preprov-pv.yaml
```

## Use Persistent Volume In Pod

1. Create example PVC and Pod

```bash
$ kubectl apply -f ./examples/kubernetes/pre-provision/preprov-pod-demo.yaml
```

2. Verify PV is created and bound to PVC

```bash
$ kubectl get pvc
NAME          STATUS   VOLUME      CAPACITY   ACCESS MODES   STORAGECLASS    AGE
preprov-pvc   Bound    my-pre-pv   1Ti        RWX            csi-filestore   76s
```

3. Verify pod is created and in `RUNNING` state (it may take a few minutes to
   get to running state)

```bash
$ kubectl get pods
NAME           READY   STATUS    RESTARTS   AGE
web-server-1   1/1     Running   0          119s
```

4. Verify contents of the mounted volume

```bash
kubectl exec web-server-1 -- ls /usr/share/nginx/html
lost+found
```
