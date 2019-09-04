package main

import (
	"fmt"
	"os"

	flag "github.com/spf13/pflag"

	"sigs.k8s.io/gcp-filestore-csi-driver/hack/csi_client/pkg/csi"
)

const helpMessage = `
Usage of %s:
	The endpoint and request flags are required flags.
	Other necessary flags are determined based on the request type.

	NodePublish
		- shareAddr, shareName, targetPath, smbUsername (WINDOWS), smbPassword (WINDOWS)

	NodeUnpublish
		- targetPath

Supported flags:
`

var (
	endpoint    = flag.String("endpoint", "/tmp/csi.sock", "CSI endpoint")
	request     = flag.String("request", "", "Type of request to make")
	shareAddr   = flag.String("shareAddr", "localhost", "Address of the share")
	shareName   = flag.String("shareName", "", "Name of the share")
	targetPath  = flag.String("targetPath", "", "Path to mount volume at")
	smbUsername = flag.String("smbUsername", "", "WINDOWS ONLY: Username to login to SMB share")
	smbPassword = flag.String("smbPassword", "", "WINDOWS ONLY: Password to login to SMB share")
)

func main() {
	flag.CommandLine.Usage = func() {
		fmt.Printf(helpMessage, os.Args[0])
		flag.PrintDefaults()
	}

	flag.Set("logtostderr", "true")
	flag.Parse()
	client, err := csi.NewClient(*endpoint)
	if err != nil {
		fmt.Printf("Error creating client for endpoint %s: %s\n", *endpoint, err)
		return
	}

	err = client.NewRequest(&csi.Request{
		RequestType: *request,
		ShareAddr:   *shareAddr,
		ShareName:   *shareName,
		TargetPath:  *targetPath,
		Username:    *smbUsername,
		Password:    *smbPassword,
	})

	if err != nil {
		fmt.Printf("Error during request %s: %s\n", *request, err)
	}
}
