/*
Copyright 2024 The Kubernetes Authors.
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
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	releaselock "sigs.k8s.io/gcp-filestore-csi-driver/pkg/releaselock"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/util"
)

var (
	lockReleaseSyncPeriod = flag.Duration("lock-release-sync-period", 3600*time.Second, "Duration, in seconds, the sync period of the lock release controller. Defaults to 3600 seconds.")

	httpEndpoint = flag.String("http-endpoint", "", "The TCP network address where the HTTP server for diagnostics, including metrics and leader election health check, will listen (example: `:8080`). The default is empty string.")
	metricsPath  = flag.String("metrics-path", "/metrics", "The HTTP path where prometheus metrics will be exposed. Default is `/metrics`.")

	leaderElectionLeaseDuration = flag.Duration("leader-election-lease-duration", 15*time.Second, "Duration, in seconds, that non-leader candidates will wait to force acquire leadership. Defaults to 15 seconds.")
	leaderElectionRenewDeadline = flag.Duration("leader-election-renew-deadline", 10*time.Second, "Duration, in seconds, that the acting leader will retry refreshing leadership before giving up. Defaults to 10 seconds.")
	leaderElectionRetryPeriod   = flag.Duration("leader-election-retry-period", 5*time.Second, "Duration, in seconds, the LeaderElector clients should wait between tries of actions. Defaults to 5 seconds.")

	workQueueRateLimiterBaseDelay = flag.Duration("rate-limiter-base-delay", 5*time.Millisecond, "Base dalay of the work queue rate limiter. Default is 5ms.")
	workQueueRateLimiterMaxDelay  = flag.Duration("rate-limiter-max-delay", 1000*time.Second, "Max dalay of the work queue rate limiter. Default is 1000s.")
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	config, err := rest.InClusterConfig()
	if err != nil {
		klog.Fatalf("Failed to create an in cluster config: %v", err)
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Failed to create a new discovery client: %v", err)
	}
	lockReleaseConfig := &releaselock.LockReleaseControllerConfig{
		LeaseDuration:                 *leaderElectionLeaseDuration,
		RenewDeadline:                 *leaderElectionRenewDeadline,
		RetryPeriod:                   *leaderElectionRetryPeriod,
		SyncPeriod:                    *lockReleaseSyncPeriod,
		WorkQueueRateLimiterBaseDelay: *workQueueRateLimiterBaseDelay,
		WorkQueueRateLimiterMaxDelay:  *workQueueRateLimiterMaxDelay,
		MetricEndpoint:                *httpEndpoint,
		MetricPath:                    *metricsPath,
	}
	factory := informers.NewSharedInformerFactory(client, lockReleaseConfig.SyncPeriod)
	nodeInformer := factory.Core().V1().Nodes().Informer()

	c, err := releaselock.NewLockReleaseController(client, lockReleaseConfig, &nodeInformer)
	if err != nil {
		klog.Fatalf("Failed to create a lock release controller: %v", err)
	}

	nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			klog.Infof("Node informer received node create event. %v", obj)
			c.EnqueueCreateEventObject(obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			klog.Infof("Node informer received node update event. old %v, new %v", oldObj, newObj)
			c.EnqueueUpdateEventObject(oldObj, newObj)
		},
	})

	run := func(ctx context.Context) {
		klog.Infof("Lock release controller %s started leading on node %s", c.GetId(), c.GetHost())
		factory.Start(ctx.Done())
		c.Run(ctx)
	}

	rl, err := resourcelock.New(
		resourcelock.LeasesResourceLock,
		util.ManagedFilestoreCSINamespace,
		releaselock.LeaseName,
		nil,
		c.GetClient().CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity: c.GetId(),
		})
	if err != nil {
		klog.Fatalf("Error creating resourcelock: %v", err)
	}

	// Use leader election, so that during rolling upgrade, only one of this controller and the old version lock release controller
	// is running.
	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: lockReleaseConfig.LeaseDuration,
		RenewDeadline: lockReleaseConfig.RenewDeadline,
		RetryPeriod:   lockReleaseConfig.RetryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: run,
			OnStoppedLeading: func() {
				klog.Fatalf("%s no longer the leader", c.GetId())
			},
		},
	})
}
