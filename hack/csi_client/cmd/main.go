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
		- volumeAttr, targetPath, secrets

	NodeUnpublish
		- targetPath

Supported flags:
`

var (
	endpoint   = flag.String("endpoint", "/tmp/csi.sock", "CSI endpoint")
	request    = flag.String("request", "", "Type of request to make")
	volumeAttr = flag.StringToString("volumeAttr", map[string]string{}, "Attributes of the volume to mount.")
	targetPath = flag.String("targetPath", "", "Path to mount volume at")
	secrets    = flag.StringToString("secrets", map[string]string{}, "Secrets")
)

func main() {
	flag.Usage = func() {
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
		VolumeAttr:  *volumeAttr,
		TargetPath:  *targetPath,
		Secrets:     *secrets,
	})

	if err != nil {
		fmt.Printf("Error during request %s: %s\n", *request, err)
	}
}
