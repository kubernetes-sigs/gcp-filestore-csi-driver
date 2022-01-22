/*
Copyright 2022 The Kubernetes Authors.

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
	"flag"

	"k8s.io/klog"
	webhook "sigs.k8s.io/gcp-filestore-csi-driver/pkg/webhook"
)

func main() {
	rootCmd := webhook.CmdWebhook
	loggingFlags := &flag.FlagSet{}
	klog.InitFlags(loggingFlags)
	klog.Infof("Starting filestorecsi webhook container")
	rootCmd.PersistentFlags().AddGoFlagSet(loggingFlags)
	rootCmd.Execute()
}
