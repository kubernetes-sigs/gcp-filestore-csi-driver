# Kubernetes Basic Windows User Guide
This guide gives an example on how to use SMB share(s) in pods running on Windows.

## Prerequisites
- Minimum K8s version: 1.16
- A non-cluster Windows 1809 VM with `Full` access to **Storage** under `Cloud API Access Scopes`. More [details](https://cloud.google.com/compute/docs/access/create-enable-service-accounts-for-instances#changeserviceaccountandscopes).
- An updated personal fork of [kubernetes](https://github.com/kubernetes/kubernetes),[node-driver-registrar](https://github.com/kubernetes-csi/node-driver-registrar) and [gcp-filestore-csi-driver](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver).

## Preparing the forks
### Kubernetes
1. Add [Almos98/kubernetes](https://github.com/kubernetes/kubernetes/compare/master...almos98:CSI-prototype) as a remote.
    ```bash
    git checkout master
    git remote add demo https://github.com/almos98/kubernetes
    ```
2. Apply changes to personal fork.
    ```bash
    git fetch demo refs/heads/CSI-prototype
    git checkout -b csi_prototype
    COMMIT=$(git ls-remote demo | grep refs/heads/CSI-prototype | cut -f 1)
    git cherry-pick $COMMIT
    ```

3. Change start up script to use your own bucket to pull the CSI driver binary. The file is at: `cluster/gce/windows/configure.ps1`.

4. Build Windows binaries.
    ```bash
    curl https://github.com/yujuhong/kubernetes/commit/27e608a050a997be5ab736a7cdeb29aa68f3b7ee.patch | git apply
    make clean
    make quick-release
    ```

### Node Driver Registrar
1. Add [Almos98/node-driver-register](https://github.com/kubernetes-csi/node-driver-registrar/compare/master...almos98:CSI-Prototype) as a remote.
    ```bash
    git checkout master
    git remote add demo https://github.com/almos98/node-driver-registrar
    ```
2. Apply changes to personal fork.
    ```bash
    git fetch demo refs/heads/CSI-Prototype
    git checkout -b csi_prototype
    COMMIT=$(git ls-remote demo | grep refs/heads/CSI-Prototype | cut -f 1)
    git cherry-pick $COMMIT
    git push # Needed for step 4
    ```
3. Build Windows binary and upload it to bucket.
    ```bash
    make
    CLOUDSDK_CORE_PROJECT=<your project>
    gsutil mb gs://${CLOUDSDK_CORE_PROJECT}-bucket
    gsutil cp bin/csi-node-driver-registrar.exe gs://${CLOUDSDK_CORE_PROJECT}-bucket/
    ```
4. Build the Windows container image.
    - Clone your personal fork on your non-cluster Windows VM.
    - Copy the binary from your bucket.
        ```Powershell
        $PROJECT='<your project>'
        cd node-driver-registrar
        git checkout CSI-Prototype
        mkdir bin
        gsutil cp gs://$PROJECT-bucket/csi-node-driver-registrar.exe bin/
        ```
    - Build and upload the image.
        ```PowerShell
        gcloud auth configure-docker
        docker build -f Dockerfile.Windows . --tag csi-node-driver-registrar:1809
        docker tag csi-node-driver-registrar:1809 gcr.io/$PROJECT/csi-node-driver-registrar:1809
        docker push gcr.io/$PROJECT/csi-node-driver-registrar:1809
        ```
### FileStore CSI Driver
0. One-time per project: Create GCP service account for the CSI driver and set the Cloud Filestore editor role.
    ```bash
    ./deploy/project_setup.sh
    ```
1. Add [Almos98/gcp-filestore-csi-driver](https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/compare/master...almos98:windows_demo) as a remote.
    ```bash
    git checkout master
    git remote add demo https://github.com/almos98/gcp-filestore-csi-driver
    ```
2. Apply changes to personal fork.
    ```bash
    git fetch demo refs/heads/windows_demo
    git checkout -b csi_prototype
    COMMIT=$(git ls-remote demo | grep refs/heads/windows_demo | cut -f 1)
    git cherry-pick $COMMIT
    ```
3. Build Windows binary and upload it to the bucket.
    ```bash
    make windows-local
    gsutil cp bin/gcfs-csi-driver.exe gs://$CLOUDSDK_CORE_PROJECT-bucket/
    ```
## Preparing the cluster
1. Go back to your k/k workspace where the patched kubernetes binaries were built and bring up a cluster using those binaries.
    
    **WARNING:** This wil tear down an existing e2e cluster in your project.
    ```bash
    PROJECT=${CLOUDSDK_CORE_PROJECT} \
    NUM_NODES=2 \
    NUM_WINDOWS_NODES=3 \
    KUBE_GCE_ENABLE_IP_ALIASES=true \
    KUBERNETES_NODE_PLATFORM=windows \
    LOGGING_STACKDRIVER_RESOURCE_TYPES=new \
    KUBERNETES_SKIP_CONFIRM=y \
    go run ./hack/e2e.go -- --up
    ```

    **NOTE:** If the above command fails, try adding the flag `-get=false`.
    ```bash
    go run ./hack/e2e.go -get=false ...
    ```

    For further reference, [see](https://github.com/kubernetes/kubernetes/blob/master/cluster/gce/windows/README-GCE-Windows-kube-up.md).

2. Untaint the Windows nodes. The easiest way to do this is running the smoke test:

    ```bash
    ./cluster/gce/windows/smoke-test.sh
    ```

3. From Filestore driver workspace, set up cluster.
    ```bash
    PROJECT=${CLOUDSDK_CORE_PROJECT} ./deploy/kubernetes/cluster_setup.sh
    ```

4. On one of the Windows nodes, create an SMB share.
    
    Remember the name of this node, will be needed next step.
    ```Powershell
    $Password = ConvertTo-SecureString "<your password>" -AsPlainText -Force
    New-LocalUser -Name smbuser -AccountNeverExpires -Password $Password
    Add-LocalGroupMember -Group "Remote Desktop Users" -Member smbuser
    New-Item -ItemType "directory" -Path "C:\SMBShare"
    New-SmbShare -Name "SMBShare" -Path "C:\SMBShare" -FullAccess "smbuser"
    ```

5. Update necessary YAML files in Filestore driver workspace.
    
    Information you need:
    - Project name
    - Name of the node that hosts the SMB share
    - Password from previous step
    - Name of the nodes you will be deploying to.

    List of files:
    - `deploy/kubernetes/manifests/node_windows.yaml`
    - `examples/kubernetes/windows/secrets.yaml`
    - `examples/kubernetes/windows/pv.yaml`
    - `examples/kubernetes/windows/nettest-pod.yaml`
    - `examples/kubernetes/windows/powershell-nettest-pod.yaml`

## Deploying
[//]: # (Use kubelet commands instead of logging in)
1. Log in to the nodes you specified in `examples/kubernetes/windows/nettest-pod.yaml` and `examples/kubernetes/windows/powershell-nettest-pod.yaml`.

2. Back from the Filestore driver workspace, run the *daemonset* for the node-driver-registrar container.
    ```bash
    ./deploy/kubernetes/driver_start_windows.sh
    ```

    Check if it was successful with the command:
    ```bash
    kubectl get pods --namespace=gcp-filestore-csi-driver
    ```
3. Finally apply all the YAML files.
    ```bash
    ./examples/kubernetes/windows/apply.sh
    ```

    This will create all the Kubernetes resources needed to run pods and then launch the pods. These resources are:
    - CSI Driver
    - Secrets
    - PersistentVolume
    - *Static* PersistentVolume Claim
    - One pod running with 1 container (nettest).
    - One pod running with 2 containers (nettest and powershell).

    To check if a csi-node was created successfully:
    ```bash
    kubectl get csinodes
    ```
    You should see your node(s) listed there.

4. Commands to help track status of pods.
    ```bash
    kubectl get pods
    kubectl describe pod nettest
    kubectl describe pod powershell-nettest
    ```

## Demo
1. Run each of the following commands in a different terminal.
    ```bash
    kubectl exec -it nettest powershell.exe
    kubectl exec -it powershell-nettest -c powershell pwsh.exe
    kubectl exec -it powershell-nettest -c nettest powershell.exe
    ```

2. From one of the containers, run:
    ```Powershell
    Set-Content -Path "C:\smbshare\Hello-Containers.txt" -Value "Hello World!" 
    ```

3. And from one of the other containers:
    ```Powershell
    Get-Content -Path "C:\smbshare\Hello-Containers.txt"
    ```

## Tearing down
1. Delete the Kubernetes pods and resources by un-applying the YAML files.
    ```bash
    ./examples/kubernetes/windows/delete.sh
    ```
    **BUG:** Kubelet crashes after requesting UnpublishVolume but successfully deletes the pod.

    **WORKAROUND:** Restart the kubelet and the driver registrar.
    ```Powershell
    sc.exe stop kubelet
    sc.exe start kubelet
    ```
2. Stop the driver
    ```bash
    ./deploy/kubernetes/driver_delete_windows.sh
    ```

[//]: # (Not sure if all those options are required to teardown the cluster)

3. From the kubernetes workspace, stop the cluster.

    ```bash
    PROJECT=${CLOUDSDK_CORE_PROJECT} \
    NUM_NODES=2 \
    NUM_WINDOWS_NODES=3 \
    KUBE_GCE_ENABLE_IP_ALIASES=true \
    KUBERNETES_NODE_PLATFORM=windows \
    LOGGING_STACKDRIVER_RESOURCE_TYPES=new \
    KUBERNETES_SKIP_CONFIRM=y \
    go run ./hack/e2e.go -- --down
    ```
