/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"flag"
	"os"

	"github.com/golang/glog"

	"k8s.io/klog"
	mount "k8s.io/mount-utils"
	cloud "sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/metadata"
	metadataservice "sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/metadata"
	driver "sigs.k8s.io/gcp-filestore-csi-driver/pkg/csi_driver"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/metrics"
)

var (
	endpoint            = flag.String("endpoint", "unix:/tmp/csi.sock", "CSI endpoint")
	nodeID              = flag.String("nodeid", "", "node id")
	runController       = flag.Bool("controller", false, "run controller service")
	runNode             = flag.Bool("node", false, "run node service")
	cloudConfigFilePath = flag.String("cloud-config", "", "Path to GCE cloud provider config")
	httpEndpoint        = flag.String("http-endpoint", "", "The TCP network address where the prometheus metrics endpoint will listen (example: `:8080`). The default is empty string, which means metrics endpoint is disabled.")
	metricsPath         = flag.String("metrics-path", "/metrics", "The HTTP path where prometheus metrics will be exposed. Default is `/metrics`.")
	enableMultishare    = flag.Bool("enable-multishare", false, "if set to true, the driver will support multishare instance provisioning")
	// This is set at compile time
	version = "unknown"
)

const driverName = "filestore.csi.storage.gke.io"

func main() {
	flag.Set("logtostderr", "true")
	flag.Parse()

	var provider *cloud.Cloud
	var err error
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var meta metadata.Service
	if *runController {
		if *enableMultishare {
			klog.Fatalf("provisioning of multishare instances not supported")
		}
		if *httpEndpoint != "" && metrics.IsGKEComponentVersionAvailable() {
			mm := metrics.NewMetricsManager()
			mm.InitializeHttpHandler(*httpEndpoint, *metricsPath)
			mm.EmitGKEComponentVersion()
		}

		provider, err = cloud.NewCloud(ctx, version, *cloudConfigFilePath)
	} else {
		if *nodeID == "" {
			klog.Fatalf("nodeid cannot be empty for node service")
		}

		meta, err = metadataservice.NewMetadataService()
		if err != nil {
			klog.Fatalf("Failed to set up metadata service: %v", err)
		}
	}

	if err != nil {
		glog.Fatalf("Failed to initialize cloud provider: %v", err)
	}

	mounter := mount.New("")
	config := &driver.GCFSDriverConfig{
		Name:             driverName,
		Version:          version,
		NodeID:           *nodeID,
		RunController:    *runController,
		RunNode:          *runNode,
		Mounter:          mounter,
		Cloud:            provider,
		MetadataService:  meta,
		EnableMultishare: *enableMultishare,
	}

	gcfsDriver, err := driver.NewGCFSDriver(config)
	if err != nil {
		glog.Fatalf("Failed to initialize Cloud Filestore CSI Driver: %v", err)
	}

	glog.Infof("Running Google Cloud Filestore CSI driver version %v", version)
	gcfsDriver.Run(*endpoint)

	os.Exit(0)
}
