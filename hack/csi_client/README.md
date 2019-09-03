# CSI Client

This is a tool used to send requests to a running CSI driver. Currently only the FileStore CSI driver is supported.
The tool must be used on the same machine that the driver is running in.

## Compiling
**LINUX:** `make csi-client`
**WINDOWS:** `make csi-client-windows`

## Usage
As of now, only two types of requests are supported:
### NodePublish
```bash
./csi-client --endpoint="/tmp/csi.sock" --request="nodepublish" --shareAddr="localhost" --shareName="SMBShare" --targetPath="/smbsharemount"
```

If the SMB share requires a username and password, these can be provided by using the flags `smbUsername` and `smbPassword`:
```Powershell
.\csi-client.exe --endpoint="C:\tmp\csi.sock" --request="nodepublish" --shareAddr="localhost"
--shareName="SMBShare" --targetPath="C:\SMBShareMount"
--smbUsername="smbuser" --smbPassword="foobar"
```

### NodeUnpublish
```bash
./csi-client --endpoint="/tmp/csi.sock" --request="nodeunpublish" --targetPath="smbsharemount"
```