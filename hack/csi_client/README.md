# CSI Client

This is a tool used to send requests to a running CSI driver. Currently only the FileStore CSI driver is supported.
The tool must be used on the same machine that the driver is running in.

## Compiling
**LINUX:** `make csi-client`
**WINDOWS:** `make csi-client-windows`

## Usage
**NOTE:** As commas are used in delimiting map arguments, it is not recommended to use them. They will cause errors.

As of now, only two types of requests are supported:
### NodePublish
```bash
./csi-client --endpoint="/tmp/csi.sock" --request="nodepublish" volumeAttr="ip=localhost,volume=SMBShare" --targetPath="/smbsharemount"
```

If the SMB share requires a username and password, these can be provided by using the flags `smbUsername` and `smbPassword`:
```Powershell
.\csi-client.exe --endpoint="C:\tmp\csi.sock" --request="nodepublish" -- volumeAttr="ip=localhost,volume=SMBShare" --targetPath="C:\SMBShareMount" --secrets="smbUser=smbuser,smbPassword=foobar"
```

### NodeUnpublish
```bash
./csi-client --endpoint="/tmp/csi.sock" --request="nodeunpublish" --targetPath="smbsharemount"
```