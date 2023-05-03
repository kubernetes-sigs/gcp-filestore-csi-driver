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

package driver

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/signal"
	"time"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-csi/csi-lib-utils/leaderelection"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	mount "k8s.io/mount-utils"
	clientset "sigs.k8s.io/gcp-filestore-csi-driver/pkg/client/clientset/versioned"
	sharescheme "sigs.k8s.io/gcp-filestore-csi-driver/pkg/client/clientset/versioned/scheme"
	fsInformers "sigs.k8s.io/gcp-filestore-csi-driver/pkg/client/informers/externalversions"
	listers "sigs.k8s.io/gcp-filestore-csi-driver/pkg/client/listers/multishare/v1beta1"
	cloud "sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider"
	metadataservice "sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/metadata"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/metrics"
	lockrelease "sigs.k8s.io/gcp-filestore-csi-driver/pkg/releaselock"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/util"
)

const (
	// The maximum retry duration = initial duration * retry factor ^ # steps. Rearranging, this gives
	// # steps = log(maximum retry / initial duration) / log(retry factor).
	crdCheckRetryFactor       = 1.5
	crdCheckInitialDurationMs = 100
)

type GCFSDriverConfig struct {
	Name             string          // Driver name
	Version          string          // Driver version
	NodeName         string          // Node name
	RunController    bool            // Run CSI controller service
	RunNode          bool            // Run CSI node service
	Mounter          mount.Interface // Mount library
	Cloud            *cloud.Cloud    // Cloud provider
	MetadataService  metadataservice.Service
	EnableMultishare bool
	Reconciler       *MultishareReconciler
	Metrics          *metrics.MetricsManager
	EcfsDescription  string
	IsRegional       bool
	ClusterName      string
	FeatureOptions   *GCFSDriverFeatureOptions
}

type GCFSDriver struct {
	config *GCFSDriverConfig

	// CSI RPC servers
	ids csi.IdentityServer
	ns  csi.NodeServer
	cs  csi.ControllerServer

	// Stateful CSI driver
	recon         *MultishareReconciler
	factory       fsInformers.SharedInformerFactory
	coreFactory   informers.SharedInformerFactory
	driverFactory fsInformers.SharedInformerFactory

	// Plugin capabilities
	vcap  map[csi.VolumeCapability_AccessMode_Mode]*csi.VolumeCapability_AccessMode
	cscap []*csi.ControllerServiceCapability
	nscap []*csi.NodeServiceCapability
}

type GCFSDriverFeatureOptions struct {
	// FeatureLockRelease will enable the NFS lock release feature if sets to true.
	FeatureLockRelease *FeatureLockRelease
	// FeatureMaxSharesPerInstance will enable CSI driver to pack configurable number of max shares per Filestore instance (multishare)
	FeatureMaxSharesPerInstance *FeatureMaxSharesPerInstance
	FeatureStateful             *FeatureStateful
}

type FeatureStateful struct {
	Enabled      bool
	KubeAPIQPS   float64
	KubeAPIBurst int
	KubeConfig   string
	ResyncPeriod time.Duration

	LeaderElection              bool
	LeaderElectionNamespace     string
	LeaderElectionLeaseDuration time.Duration
	LeaderElectionRenewDeadline time.Duration
	LeaderElectionRetryPeriod   time.Duration

	DriverClientSet *clientset.Clientset
	ShareLister     listers.ShareInfoLister
}

type FeatureLockRelease struct {
	Enabled bool
	Config  *lockrelease.LockReleaseControllerConfig
}

type FeatureMaxSharesPerInstance struct {
	Enabled                          bool
	DescOverrideMaxSharesPerInstance string
	DescOverrideMinShareSizeGB       string
	KubeClient                       *kubernetes.Clientset
	CoreInformerResync               time.Duration
}

func NewGCFSDriver(config *GCFSDriverConfig) (*GCFSDriver, error) {
	if config.Name == "" {
		return nil, fmt.Errorf("driver name missing")
	}
	if config.Version == "" {
		return nil, fmt.Errorf("driver version missing")
	}
	if !config.RunController && !config.RunNode {
		return nil, fmt.Errorf("must run at least one controller or node service")
	}

	driver := &GCFSDriver{
		config: config,
		vcap:   map[csi.VolumeCapability_AccessMode_Mode]*csi.VolumeCapability_AccessMode{},
	}

	vcam := []csi.VolumeCapability_AccessMode_Mode{
		csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
		csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY,
		csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY,
		csi.VolumeCapability_AccessMode_MULTI_NODE_SINGLE_WRITER,
		csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
	}
	driver.addVolumeCapabilityAccessModes(vcam)

	// Setup RPC servers
	driver.ids = newIdentityServer(driver)
	if config.RunNode {
		nscap := []csi.NodeServiceCapability_RPC_Type{
			csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
			csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
		}
		ns, err := newNodeServer(driver, config.Mounter, config.MetadataService, config.FeatureOptions)
		if err != nil {
			return nil, err
		}
		driver.ns = ns
		driver.addNodeServiceCapabilities(nscap)
	}
	if config.RunController {
		csc := []csi.ControllerServiceCapability_RPC_Type{
			csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
			csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
			csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT,
		}
		driver.addControllerServiceCapabilities(csc)

		if config.FeatureOptions.FeatureStateful != nil && config.FeatureOptions.FeatureStateful.Enabled {
			driver.recon, driver.factory, driver.coreFactory, driver.driverFactory = initMultishareReconciler(config)
		}
		// Configure controller server
		driver.cs = newControllerServer(&controllerServerConfig{
			driver:           driver,
			fileService:      config.Cloud.File,
			cloud:            config.Cloud,
			volumeLocks:      util.NewVolumeLocks(),
			enableMultishare: config.EnableMultishare,
			reconciler:       config.Reconciler,
			metricsManager:   config.Metrics,
			ecfsDescription:  config.EcfsDescription,
			isRegional:       config.IsRegional,
			clusterName:      config.ClusterName,
			features:         config.FeatureOptions,
		})
	}

	return driver, nil
}

func (driver *GCFSDriver) addVolumeCapabilityAccessModes(vc []csi.VolumeCapability_AccessMode_Mode) error {
	for _, c := range vc {
		klog.Infof("Enabling volume access mode: %v", c.String())
		mode := NewVolumeCapabilityAccessMode(c)
		driver.vcap[mode.Mode] = mode
	}
	return nil
}

func (driver *GCFSDriver) validateVolumeCapabilities(caps []*csi.VolumeCapability) error {
	if len(caps) == 0 {
		return fmt.Errorf("volume capabilities must be provided")
	}

	for _, c := range caps {
		if err := driver.validateVolumeCapability(c); err != nil {
			return err
		}
	}
	return nil
}

func (driver *GCFSDriver) validateVolumeCapability(c *csi.VolumeCapability) error {
	if c == nil {
		return fmt.Errorf("volume capability must be provided")
	}

	// Validate access mode
	accessMode := c.GetAccessMode()
	if accessMode == nil {
		return fmt.Errorf("volume capability access mode not set")
	}
	if driver.vcap[accessMode.Mode] == nil {
		return fmt.Errorf("driver does not support access mode: %v", accessMode.Mode.String())
	}

	// Validate access type
	accessType := c.GetAccessType()
	if accessType == nil {
		return fmt.Errorf("volume capability access type not set")
	}
	mountType := c.GetMount()
	if mountType == nil {
		return fmt.Errorf("driver only supports mount access type volume capability")
	}
	if mountType.FsType != "" {
		// TODO: uncomment after https://github.com/kubernetes-csi/external-provisioner/issues/328 is fixed.
		// return fmt.Errorf("driver does not support fstype %v", mountType.FsType)
	}
	// TODO: check if we want to whitelist/blacklist certain mount options
	return nil
}

func (driver *GCFSDriver) addControllerServiceCapabilities(cl []csi.ControllerServiceCapability_RPC_Type) error {
	var csc []*csi.ControllerServiceCapability
	for _, c := range cl {
		klog.Infof("Enabling controller service capability: %v", c.String())
		csc = append(csc, NewControllerServiceCapability(c))
	}
	driver.cscap = csc
	return nil
}

func (driver *GCFSDriver) addNodeServiceCapabilities(nl []csi.NodeServiceCapability_RPC_Type) error {
	var nsc []*csi.NodeServiceCapability
	for _, n := range nl {
		klog.Infof("Enabling node service capability: %v", n.String())
		nsc = append(nsc, NewNodeServiceCapability(n))
	}
	driver.nscap = nsc
	return nil
}

func (driver *GCFSDriver) ValidateControllerServiceRequest(c csi.ControllerServiceCapability_RPC_Type) error {
	if c == csi.ControllerServiceCapability_RPC_UNKNOWN {
		return nil
	}

	for _, cap := range driver.cscap {
		if c == cap.GetRpc().Type {
			return nil
		}
	}

	return status.Error(codes.InvalidArgument, "Invalid controller service request")
}

func (driver *GCFSDriver) Run(endpoint string) {
	klog.Infof("Running driver: %v", driver.config.Name)

	run := func(ctx context.Context) {
		// run...
		stopCh := make(chan struct{})
		go driver.cs.(*controllerServer).Run(stopCh)

		// ...until SIGINT
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		<-c
		close(stopCh)
	}

	if driver.config.RunController {
		if driver.recon != nil {
			runMultishareReconciler(driver.config, driver.recon, driver.factory, driver.coreFactory, driver.driverFactory)
		}

		klog.Infof("runcontroller %v", driver.config.RunController)
		go run(context.TODO())
	}

	// Start the nonblocking GRPC.
	s := NewNonBlockingGRPCServer()
	s.Start(endpoint, driver.ids, driver.cs, driver.ns)
	if driver.config.RunNode && driver.config.FeatureOptions.FeatureLockRelease.Enabled {
		// Start the lock release controller on node driver.
		driver.ns.(*nodeServer).lockReleaseController.Run(context.Background())
	}
	s.Wait()
}

func initMultishareReconciler(driverConfig *GCFSDriverConfig) (*MultishareReconciler, fsInformers.SharedInformerFactory, informers.SharedInformerFactory, fsInformers.SharedInformerFactory) {
	config, err := util.BuildConfig(driverConfig.FeatureOptions.FeatureStateful.KubeConfig)
	if err != nil {
		klog.Error(err.Error())
		os.Exit(1)
	}
	config.QPS = (float32)(driverConfig.FeatureOptions.FeatureStateful.KubeAPIQPS)
	config.Burst = driverConfig.FeatureOptions.FeatureStateful.KubeAPIBurst

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
	driverfsClient, err := clientset.NewForConfig(config)
	if err != nil {
		klog.Error(err.Error())
		os.Exit(1)
	}

	resyncPeriod := driverConfig.FeatureOptions.FeatureStateful.ResyncPeriod
	factory := fsInformers.NewSharedInformerFactoryWithOptions(fsClient, resyncPeriod, fsInformers.WithNamespace(util.ManagedFilestoreCSINamespace))
	coreFactory := informers.NewSharedInformerFactory(kubeClient, resyncPeriod)
	driverFactory := fsInformers.NewSharedInformerFactoryWithOptions(driverfsClient, resyncPeriod, fsInformers.WithNamespace(util.ManagedFilestoreCSINamespace))
	sharescheme.AddToScheme(scheme.Scheme)

	recon := NewMultishareReconciler(
		fsClient,
		driverConfig,
		factory.Multishare().V1beta1().ShareInfos(),
		factory.Multishare().V1beta1().InstanceInfos(),
		coreFactory.Storage().V1().StorageClasses().Lister(),
	)
	driverConfig.Reconciler = recon
	driverConfig.FeatureOptions.FeatureStateful.DriverClientSet = driverfsClient
	driverConfig.FeatureOptions.FeatureStateful.ShareLister = driverFactory.Multishare().V1beta1().ShareInfos().Lister()

	if err := ensureCustomResourceDefinitionsExist(fsClient); err != nil {
		klog.Errorf("Exiting due to failure to ensure CRDs exist during startup: %+v", err)
		os.Exit(1)
	}

	return recon, factory, coreFactory, driverFactory
}

func runMultishareReconciler(driverConfig *GCFSDriverConfig, recon *MultishareReconciler, factory fsInformers.SharedInformerFactory, coreFactory informers.SharedInformerFactory, driverFactory fsInformers.SharedInformerFactory) {

	run := func(context.Context) {
		// run...
		stopCh := make(chan struct{})
		factory.Start(stopCh)
		coreFactory.Start(stopCh)
		driverFactory.Start(stopCh)
		go recon.Run(stopCh)

		// ...until SIGINT
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		<-c
		close(stopCh)
	}

	statefulConfig := driverConfig.FeatureOptions.FeatureStateful

	if !statefulConfig.LeaderElection {
		go run(context.TODO())
	} else {
		go func() {
			lockName := "filestore-stateful-leader"
			config, err := util.BuildConfig(driverConfig.FeatureOptions.FeatureStateful.KubeConfig)
			if err != nil {
				klog.Fatal(err.Error())
			}

			leClient, err := kubernetes.NewForConfig(config)
			if err != nil {
				klog.Fatalf("Failed to create leaderelection client: %v", err)
			}
			le := leaderelection.NewLeaderElection(leClient, lockName, run)
			if statefulConfig.LeaderElectionNamespace != "" {
				le.WithNamespace(statefulConfig.LeaderElectionNamespace)
			}
			le.WithLeaseDuration(statefulConfig.LeaderElectionLeaseDuration)
			le.WithRenewDeadline(statefulConfig.LeaderElectionRenewDeadline)
			le.WithRetryPeriod(statefulConfig.LeaderElectionRetryPeriod)
			if err := le.Run(); err != nil {
				klog.Fatalf("Failed to initialize leader election: %v", err)
			}
		}()
	}
}

// Checks that the ShareInfo v1beta1 CRDs exist.
func ensureCustomResourceDefinitionsExist(client *clientset.Clientset) error {
	condition := func() (bool, error) {
		var err error

		// scoping to an empty namespace makes `List` work across all namespaces
		_, err = client.MultishareV1beta1().ShareInfos(util.ManagedFilestoreCSINamespace).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			klog.Errorf("Failed to list v1beta1 shareinfos with error=%+v", err)
			return false, nil
		}

		return true, nil
	}

	maxMs := (5 * time.Second).Milliseconds()
	if maxMs < crdCheckInitialDurationMs {
		maxMs = crdCheckInitialDurationMs
	}
	steps := int(math.Ceil(math.Log(float64(maxMs)/crdCheckInitialDurationMs) / math.Log(crdCheckRetryFactor)))
	if steps < 1 {
		steps = 1
	}
	backoff := wait.Backoff{
		Duration: crdCheckInitialDurationMs * time.Millisecond,
		Factor:   crdCheckRetryFactor,
		Steps:    steps,
	}
	if err := wait.ExponentialBackoff(backoff, condition); err != nil {
		return err
	}

	return nil
}
