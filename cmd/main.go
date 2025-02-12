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

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	mount "k8s.io/mount-utils"
	"k8s.io/utils/exec"
	cloud "sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/metadata"
	metadataservice "sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/metadata"
	driver "sigs.k8s.io/gcp-filestore-csi-driver/pkg/csi_driver"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/metrics"
	lockrelease "sigs.k8s.io/gcp-filestore-csi-driver/pkg/releaselock"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/util"
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
	extraVolumeLabelsStr            = flag.String("extra-labels", "", "Extra labels to attach to each volume created. It is a comma separated list of key value pairs like '<key1>=<value1>,<key2>=<value2>'. See https://cloud.google.com/compute/docs/labeling-resources for details")
	resourceTagsStr                 = flag.String("resource-tags", "", "Resource tags to attach to each volume created. It is a comma separated list of tags of the form '<parentID_1>/<tagKey_1>/<tagValue_1>...<parentID_N>/<tagKey_N>/<tagValue_N>' where, parentID is the ID of Organization or Project resource where tag key and value resources exist, tagKey is the shortName of the tag key resource, tagValue is the shortName of the tag value resource. See https://cloud.google.com/resource-manager/docs/tags/tags-creating-and-managing for more details.")

	// Feature lock release specific parameters, only take effect when feature-lock-release is set to true.
	featureLockRelease = flag.Bool("feature-lock-release", false, "if set to true, the node driver will support Filestore lock release.")
	// featureLockRelease must be set as true when featureLockReleaseStandalone is true. Standalone implementation will override part of the original lock release implementation when true.
	featureLockReleaseStandalone = flag.Bool("feature-lock-release-standalone", false, "if set to true, the node driver will not support v1 Filestore lock release.")
	lockReleaseSyncPeriod        = flag.Duration("lock-release-sync-period", 60*time.Second, "Duration, in seconds, the sync period of the lock release controller. Defaults to 60 seconds.")
	// Feature configurable shares per Filestore instance specific parameters.
	featureMaxSharePerInstance = flag.Bool("feature-max-shares-per-instance", false, "If this feature flag is enabled, allows the user to configure max shares packed per Filestore instance")
	descOverrideMaxShareCount  = flag.String("desc-override-max-shares-per-instance", "", "If non-empty, the filestore instance description override is used to configure max share count per instance. This flag is ignored if 'feature-max-shares-per-instance' flag is false. Both 'desc-override-max-shares-per-instance' and 'desc-override-min-shares-size-gb' must be provided. 'ecfsDescription' is ignored, if this flag is provided.")
	descOverrideMinShareSizeGB = flag.String("desc-override-min-shares-size-gb", "", "If non-empty, the filestore instance description override is used to configure min share size. This flag is ignored if 'feature-max-shares-per-instance' flag is false. Both 'desc-override-max-shares-per-instance' and 'desc-override-min-shares-size-gb' must be provided. 'ecfsDescription' is ignored, if this flag is provided.")
	coreInformerResyncPeriod   = flag.Duration("core-informer-resync-repriod", 15*time.Minute, "Core informer resync period.")

	// Feature multishare backups enabled
	featureMultishareBackups        = flag.Bool("feature-multishare-backups", false, "if set to true, the multishare backups will be enabled. enable-multishare must be set to true as well")
	featureNFSExportOptionsOnCreate = flag.Bool("feature-nfs-export-options", false, "if set to true, the driver will accpet nfs-export-options-on-create parameter and configure IP Access rules")

	// Feature stateful CSI driver specific parameters
	featureStateful      = flag.Bool("feature-stateful-multishare", false, "if set to true, the controller will run stateful multishare controller, if set to true, enable-multishare must be set to true as well")
	statefulResyncPeriod = flag.Duration("stateful-resync-period", 15*time.Minute, "Resync interval of the stateful driver.")
	kubeAPIQPS           = flag.Float64("kube-api-qps", 5, "QPS to use while communicating with the kubernetes apiserver. Defaults to 5.0.")
	kubeAPIBurst         = flag.Int("kube-api-burst", 10, "Burst to use while communicating with the kubernetes apiserver. Defaults to 10.")
	kubeconfig           = flag.String("kubeconfig", "", "Absolute path to the kubeconfig file. Required only when running out of cluster.")

	leaderElection              = flag.Bool("leader-election", false, "Enables leader election for stateful driver.")
	leaderElectionNamespace     = flag.String("leader-election-namespace", "", "The namespace where the leader election resource exists. Defaults to the pod namespace if not set.")
	leaderElectionLeaseDuration = flag.Duration("leader-election-lease-duration", 15*time.Second, "Duration, in seconds, that non-leader candidates will wait to force acquire leadership. Defaults to 15 seconds.")
	leaderElectionRenewDeadline = flag.Duration("leader-election-renew-deadline", 10*time.Second, "Duration, in seconds, that the acting leader will retry refreshing leadership before giving up. Defaults to 10 seconds.")
	leaderElectionRetryPeriod   = flag.Duration("leader-election-retry-period", 5*time.Second, "Duration, in seconds, the LeaderElector clients should wait between tries of actions. Defaults to 5 seconds.")

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
	var extraVolumeLabels map[string]string
	var tagMgr cloud.TagService
	if *runController {
		if *httpEndpoint != "" && metrics.IsGKEComponentVersionAvailable() {
			mm = metrics.NewMetricsManager()
			mm.RegisterOperationSecondsMetric()
			mm.InitializeHttpHandler(*httpEndpoint, *metricsPath)
			mm.EmitGKEComponentVersion()
		}

		if *enableMultishare {
			if *gkeClusterName == "" {
				klog.Fatalf("gke-cluster-name has to be set when multishare feature is enabled")
			}
		}

		extraVolumeLabels, err = util.ConvertLabelsStringToMap(*extraVolumeLabelsStr)
		if err != nil {
			klog.Fatalf("Bad extra volume labels: %v", err.Error())
		}

		provider, err = cloud.NewCloud(ctx, version, *cloudConfigFilePath, *primaryFilestoreServiceEndpoint, *testFilestoreServiceEndpoint)

		tagMgr = cloud.NewTagManager(provider)
		tags, err := tagMgr.ValidateResourceTags(ctx, "command line", *resourceTagsStr)
		if err != nil {
			klog.Fatalf("failed to parse resource tags provided in command line: %v", err)
		}
		tagMgr.SetResourceTags(tags)
	} else {
		if *nodeID == "" {
			klog.Fatalf("nodeid cannot be empty for node service")
		}
		if len(*extraVolumeLabelsStr) > 0 {
			klog.Fatalf("Extra volume labels provided but not running controller")
		}
		if len(*resourceTagsStr) > 0 {
			klog.Fatalf("Resource tags provided but not running controller")
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

	var kubeClient *kubernetes.Clientset
	if *featureMaxSharePerInstance && *runController && *enableMultishare {
		clusterConfig, err := util.BuildConfig(*kubeconfig)
		if err != nil {
			klog.Error(err.Error())
			os.Exit(1)
		}
		clusterConfig.ContentType = runtime.ContentTypeProtobuf
		klog.Infof("cluster config created")

		kubeClient, err = kubernetes.NewForConfig(clusterConfig)
		if err != nil {
			klog.Error(err.Error())
			os.Exit(1)
		}
	}

	featureOptions := &driver.GCFSDriverFeatureOptions{
		FeatureLockRelease: &driver.FeatureLockRelease{
			Enabled:    *featureLockRelease,
			Standalone: *featureLockReleaseStandalone,
			Config: &lockrelease.LockReleaseControllerConfig{
				LeaseDuration:  *leaderElectionLeaseDuration,
				RenewDeadline:  *leaderElectionRenewDeadline,
				RetryPeriod:    *leaderElectionRetryPeriod,
				SyncPeriod:     *lockReleaseSyncPeriod,
				MetricEndpoint: *httpEndpoint,
				MetricPath:     *metricsPath,
			},
		},
		FeatureMaxSharesPerInstance: &driver.FeatureMaxSharesPerInstance{
			Enabled:                          *featureMaxSharePerInstance,
			DescOverrideMaxSharesPerInstance: *descOverrideMaxShareCount,
			DescOverrideMinShareSizeGB:       *descOverrideMinShareSizeGB,
			KubeClient:                       kubeClient,
			CoreInformerResync:               *coreInformerResyncPeriod,
		},
		FeatureStateful: &driver.FeatureStateful{
			Enabled:                     *featureStateful,
			KubeAPIQPS:                  *kubeAPIQPS,
			KubeAPIBurst:                *kubeAPIBurst,
			KubeConfig:                  *kubeconfig,
			ResyncPeriod:                *statefulResyncPeriod,
			LeaderElection:              *leaderElection,
			LeaderElectionNamespace:     *leaderElectionNamespace,
			LeaderElectionLeaseDuration: *leaderElectionLeaseDuration,
			LeaderElectionRenewDeadline: *leaderElectionRenewDeadline,
			LeaderElectionRetryPeriod:   *leaderElectionRetryPeriod,
		},
		FeatureMultishareBackups: &driver.FeatureMultishareBackups{
			Enabled: *featureMultishareBackups,
		},
		FeatureNFSExportOptionsOnCreate: &driver.FeatureNFSExportOptionsOnCreate{
			Enabled: *featureNFSExportOptionsOnCreate,
		},
	}

	mounter := mount.NewSafeFormatAndMount(mount.New(""), exec.New())
	config := &driver.GCFSDriverConfig{
		Name:              driverName,
		Version:           version,
		NodeName:          *nodeID,
		RunController:     *runController,
		RunNode:           *runNode,
		Mounter:           mounter,
		Cloud:             provider,
		MetadataService:   meta,
		EnableMultishare:  *enableMultishare,
		Metrics:           mm,
		EcfsDescription:   *ecfsDescription,
		IsRegional:        *isRegional,
		ClusterName:       *gkeClusterName,
		FeatureOptions:    featureOptions,
		ExtraVolumeLabels: extraVolumeLabels,
		TagManager:        tagMgr,
	}

	gcfsDriver, err := driver.NewGCFSDriver(config)
	if err != nil {
		klog.Fatalf("Failed to initialize Cloud Filestore CSI Driver: %v", err)
	}
	klog.Infof("Running Google Cloud Filestore CSI driver version %v", version)
	gcfsDriver.Run(*endpoint)
	os.Exit(0)
}
