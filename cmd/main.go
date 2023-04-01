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
	"math"
	"os"
	"os/signal"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"

	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	mount "k8s.io/mount-utils"
	clientset "sigs.k8s.io/gcp-filestore-csi-driver/pkg/client/clientset/versioned"
	sharescheme "sigs.k8s.io/gcp-filestore-csi-driver/pkg/client/clientset/versioned/scheme"
	fsInformers "sigs.k8s.io/gcp-filestore-csi-driver/pkg/client/informers/externalversions"
	cloud "sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/metadata"
	metadataservice "sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/metadata"
	driver "sigs.k8s.io/gcp-filestore-csi-driver/pkg/csi_driver"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/metrics"
	reconciler "sigs.k8s.io/gcp-filestore-csi-driver/pkg/multishare_reconciler"
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

	// Feature stateful CSI driver specific parameters
	enableStateful = flag.Bool("enable-stateful-multishare", false, "")

	kubeconfig               = flag.String("kubeconfig", "", "Absolute path to the kubeconfig file. Required only when running out of cluster.")
	coreInformerResyncPeriod = flag.Duration("core-informer-resync-repriod", 15*time.Minute, "Core informer resync period.")

	// This is set at compile time
	version = "unknown"
)

const driverName = "filestore.csi.storage.gke.io"
const resyncPeriod = 15 * time.Minute

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

	var kubeClient *kubernetes.Clientset
	if *featureMaxSharePerInstance && *runController && *enableMultishare {
		clusterConfig, err := buildConfig(*kubeconfig)
		if err != nil {
			klog.Error(err.Error())
			os.Exit(1)
		}
		klog.Infof("cluster config created")

		kubeClient, err = kubernetes.NewForConfig(clusterConfig)
		if err != nil {
			klog.Error(err.Error())
			os.Exit(1)
		}
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
			KubeClient:                       kubeClient,
			CoreInformerResync:               *coreInformerResyncPeriod,
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

	if *runController && *enableMultishare && *enableStateful {
		runMultishareReconciler(config)
	}

	gcfsDriver, err := driver.NewGCFSDriver(config)
	if err != nil {
		klog.Fatalf("Failed to initialize Cloud Filestore CSI Driver: %v", err)
	}
	klog.Infof("Running Google Cloud Filestore CSI driver version %v", version)
	gcfsDriver.Run(*endpoint)
	os.Exit(0)
}

func runMultishareReconciler(driverConfig *driver.GCFSDriverConfig) {
	config, err := buildConfig(*kubeconfig)
	if err != nil {
		klog.Error(err.Error())
		os.Exit(1)
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Error(err.Error())
		os.Exit(1)
	}
	fsClient, err := clientset.NewForConfig(config)
	if err != nil {
		klog.Error(err.Error())
		os.Exit(1)
	}

	factory := fsInformers.NewSharedInformerFactory(fsClient, resyncPeriod)
	coreFactory := informers.NewSharedInformerFactory(kubeClient, resyncPeriod)
	sharescheme.AddToScheme(scheme.Scheme)

	recon := reconciler.NewMultishareReconciler(
		fsClient,
		driverConfig,
		factory.Multishare().V1alpha1().ShareInfos(),
		factory.Multishare().V1alpha1().InstanceInfos(),
		coreFactory.Storage().V1().StorageClasses().Lister(),
	)

	if err := ensureCustomResourceDefinitionsExist(fsClient); err != nil {
		klog.Errorf("Exiting due to failure to ensure CRDs exist during startup: %+v", err)
		os.Exit(1)
	}

	//TODO: add leader election so only 1 reconciler is spawn for regional cluster
	run := func(context.Context) {
		// run...
		stopCh := make(chan struct{})
		factory.Start(stopCh)
		coreFactory.Start(stopCh)
		go recon.Run(stopCh)

		// ...until SIGINT
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		<-c
		close(stopCh)
	}

	go run(context.TODO())
}

// Checks that the ShareInfo v1alpha1 CRDs exist.
func ensureCustomResourceDefinitionsExist(client *clientset.Clientset) error {
	condition := func() (bool, error) {
		var err error

		// Scoping to an empty namespace makes `List` work across all namespaces.
		_, err = client.MultishareV1alpha1().ShareInfos().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			klog.Errorf("Failed to list v1alpha1 shareinfos with error=%+v", err)
			return false, nil
		}

		return true, nil
	}

	// The maximum retry duration = initial duration * retry factor ^ # steps. Rearranging, this gives
	// # steps = log(maximum retry / initial duration) / log(retry factor).
	const retryFactor = 1.5
	const initialDurationMs = 100
	maxMs := (5 * time.Second).Milliseconds()
	if maxMs < initialDurationMs {
		maxMs = initialDurationMs
	}
	steps := int(math.Ceil(math.Log(float64(maxMs)/initialDurationMs) / math.Log(retryFactor)))
	if steps < 1 {
		steps = 1
	}
	backoff := wait.Backoff{
		Duration: initialDurationMs * time.Millisecond,
		Factor:   retryFactor,
		Steps:    steps,
	}
	if err := wait.ExponentialBackoff(backoff, condition); err != nil {
		return err
	}

	return nil
}

func buildConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	return rest.InClusterConfig()
}
