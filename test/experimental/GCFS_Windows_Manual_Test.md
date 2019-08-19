This is how I manually tested the driver:

I brought up a kubernetes cluster following the steps in [this readme](https://github.com/kubernetes/kubernetes/blob/master/cluster/gce/windows/README-GCE-Windows-kube-up.md).

On one of the Windows nodes, I created a new user and a new SMB share.

```powershell
# Create the SMB share.
New-Item -Path "C:\" -Name "share" -ItemType "directory"
New-SmbShare -Name "myshare" -Path "C:\share"

# Create local user with access to the SMB share.
$Password = Read-Host -AsSecureString # then paste password
New-LocalUser -Name smbuser -AccountNeverExpires -Password $Password
Add-LocalGroupMember -Group "Remote Desktop Users" -Member smbuser
Grant-SmbShareAccess myshare -AccessRight Full -AccountName smbuser -Force
```

I hardcoded a NodePublishRequest and a NodeUnpublishRequest into `pkg/csi_driver/gcfs_driver.go`'s Run method. They are shown below: 

```go
targetPath := "C:\\test"
windowsMachineName := "FIRST_WINDOWS_NODE_NAME"
smbShareName := "myshare"
password := "PASSWORD_USED_ABOVE"

req, err := driver.ns.NodePublishVolume(context.TODO(), &csi.NodePublishVolumeRequest{
	TargetPath: targetPath,
	VolumeCapability: &csi.VolumeCapability{
		AccessType: &csi.VolumeCapability_Mount{
			Mount: &csi.VolumeCapability_MountVolume{},
		},
		AccessMode: &csi.VolumeCapability_AccessMode{
			Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
		},
	},
	VolumeAttributes: map[string]string{
		attrIp:     windowsMachineName,
		attrVolume: smbShareName,
	},
	NodePublishSecrets: map[string]string{
		smbUser:     fmt.Sprintf("%s\\smbuser", windowsMachineName),
		smbPassword: password,
	},
})

// Commented to be able to check if it was mounted successfully.
// resp, err := driver.ns.NodeUnpublishVolume(context.TODO(), &csi.NodeUnpublishVolumeRequest{
// 	TargetPath: targetPath,
// })
```

Compile the driver with `make windows-local` and move the resulting binary to the second Windows node. After running the binary, `C:\test` should be the SMB share mounted.

To clean up the mount manually run the following PowerShell commands:
```powershell
Remove-SmbGlobalMapping
(Get-Item C:\test\).Delete()
```