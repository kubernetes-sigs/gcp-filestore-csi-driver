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
	"time"

	"k8s.io/klog/v2"
	mount "k8s.io/mount-utils"
	cloud "sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/metadata"
	metadataservice "sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/metadata"
	driver "sigs.k8s.io/gcp-filestore-csi-driver/pkg/csi_driver"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/metrics"
	lockrelease "sigs.k8s.io/gcp-filestore-csi-driver/pkg/releaselock"
)

var (
	endpoint                        = flag.String("endpoint", "unix:/tmp/csi.sock", "CSI endpoint")
	nodeID                          = flag.String("nodeid", "", "node id")
	runController                   = flag.Bool("controller", false, "run controller service")
	runNode                         = flag.Bool("node", false, "run node service")
	cloudConfigFilePath             = flag.String("cloud-config", "", "Path to GCE cloud provider config")
	httpEndpoint                    = flag.String("http-endpoint", "", "The TCP network address where the prometheus metrics endpoint will listen (example: `:8080`). The default is empty string, which means metrics endpoint is disabled.")
	metricsPath                     = flag.String("metrics-path", "/metrics", "The HTTP path where prometheus metrics will be exposed. Default is `/metrics`.")
	enableMultishare                = flag.Bool("enable-multishare", false, "if set to true, the driver will support multishare instance provisioning")
	testFilestoreServiceEndpoint    = flag.String("filestore-service-endpoint", "", "Endpoint for filestore service - used for testing only. Must be a well-known string.")
	primaryFilestoreServiceEndpoint = flag.String("primary-filestore-service-endpoint", "", "Primary endpoint for filestore service. This takes precedence over filestore-service-endpoint if present.")
	ecfsDescription                 = flag.String("ecfs-description", "", "Filestore multishare instance descrption. ecfs-version=<version>,image-project-id=<projectid>")
	isRegional                      = flag.Bool("is-regional", false, "cluster is regional cluster")
	gkeClusterName                  = flag.String("gke-cluster-name", "", "Cluster Name of the current GKE cluster driver is running on, required for multishare")

	// Feature lock release specific parameters, only take effect when feature-lock-release is set to true.
	featureLockRelease          = flag.Bool("feature-lock-release", false, "if set to true, the node driver will support Filestore lock release.")
	leaderElectionLeaseDuration = flag.Duration("leader-election-lease-duration", 15*time.Second, "Duration, in seconds, that non-leader candidates will wait to force acquire leadership. Defaults to 15 seconds.")
	leaderElectionRenewDeadline = flag.Duration("leader-election-renew-deadline", 10*time.Second, "Duration, in seconds, that the acting leader will retry refreshing leadership before giving up. Defaults to 10 seconds.")
	leaderElectionRetryPeriod   = flag.Duration("leader-election-retry-period", 5*time.Second, "Duration, in seconds, the LeaderElector clients should wait between tries of actions. Defaults to 5 seconds.")
	lockReleaseSyncPeriod       = flag.Duration("lock-release-sync-period", 60*time.Second, "Duration, in seconds, the sync period of the lock release controller. Defaults to 60 seconds.")

	// Feature configurable shares per Filestore instance specific parameters.
	featureMaxSharePerInstance = flag.Bool("feature-max-shares-per-instance", false, "If this feature flag is enabled, allows the user to configure max shares packed per Filestore instance")
	descOverrideMaxShareCount  = flag.String("desc-override-max-shares-per-instance", "", "If non-empty, the filestore instance description override is used to configure max share count per instance. This flag is ignored if 'feature-max-shares-per-instance' flag is false. Both 'desc-override-max-shares-per-instance' and 'desc-override-min-shares-size-gb' must be provided. 'ecfsDescription' is ignored, if this flag is provided.")
	descOverrideMinShareSizeGB = flag.String("desc-override-min-shares-size-gb", "", "If non-empty, the filestore instance description override is used to configure min share size. This flag is ignored if 'feature-max-shares-per-instance' flag is false. Both 'desc-override-max-shares-per-instance' and 'desc-override-min-shares-size-gb' must be provided. 'ecfsDescription' is ignored, if this flag is provided.")
	// This is set at compile time
	version = "unknown"
)

const driverName = "filestore.csi.storage.gke.io"

func main() {
	klog.InitFlags(nil)
	flag.Set("logtostderr", "true")
	flag.Parse()

	var provider *cloud.Cloud
	var err error
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var meta metadata.Service
	var mm *metrics.MetricsManager
	if *runController {
		if *httpEndpoint != "" && metrics.IsGKEComponentVersionAvailable() {
			mm = metrics.NewMetricsManager()
			mm.InitializeHttpHandler(*httpEndpoint, *metricsPath)
			mm.EmitGKEComponentVersion()
		}

		if *enableMultishare {
			if *gkeClusterName == "" {
				klog.Fatalf("gke-cluster-name has to be set when multishare feature is enabled")
			}
		}

		provider, err = cloud.NewCloud(ctx, version, *cloudConfigFilePath, *primaryFilestoreServiceEndpoint, *testFilestoreServiceEndpoint)
	} else {
		if *nodeID == "" {
			klog.Fatalf("nodeid cannot be empty for node service")
		}

		meta, err = metadataservice.NewMetadataService()
		if err != nil {
			klog.Fatalf("Failed to set up metadata service: %v", err)
		}
		klog.Infof("Metadata service setup: %+v", meta)
	}

	if err != nil {
		klog.Fatalf("Failed to initialize cloud provider: %v", err)
	}

	featureOptions := &driver.GCFSDriverFeatureOptions{
		FeatureLockRelease: &driver.FeatureLockRelease{
			Enabled: *featureLockRelease,
			Config: &lockrelease.LockReleaseControllerConfig{
				LeaseDuration: *leaderElectionLeaseDuration,
				RenewDeadline: *leaderElectionRenewDeadline,
				RetryPeriod:   *leaderElectionRetryPeriod,
				SyncPeriod:    *lockReleaseSyncPeriod,
			},
		},
		FeatureMaxSharesPerInstance: &driver.FeatureMaxSharesPerInstance{
			Enabled:                          *featureMaxSharePerInstance,
			DescOverrideMaxSharesPerInstance: *descOverrideMaxShareCount,
			DescOverrideMinShareSizeGB:       *descOverrideMinShareSizeGB,
		},
	}
	mounter := mount.New("")
	config := &driver.GCFSDriverConfig{
		Name:             driverName,
		Version:          version,
		NodeName:         *nodeID,
		RunController:    *runController,
		RunNode:          *runNode,
		Mounter:          mounter,
		Cloud:            provider,
		MetadataService:  meta,
		EnableMultishare: *enableMultishare,
		Metrics:          mm,
		EcfsDescription:  *ecfsDescription,
		IsRegional:       *isRegional,
		ClusterName:      *gkeClusterName,
		FeatureOptions:   featureOptions,
	}

	gcfsDriver, err := driver.NewGCFSDriver(config)
	if err != nil {
		klog.Fatalf("Failed to initialize Cloud Filestore CSI Driver: %v", err)
	}

	klog.Infof("Running Google Cloud Filestore CSI driver version %v", version)
	gcfsDriver.Run(*endpoint)

	os.Exit(0)
}
