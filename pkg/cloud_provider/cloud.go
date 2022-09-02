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

package cloud

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/compute/metadata"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
	"gopkg.in/gcfg.v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/file"
)

type Cloud struct {
	File    file.Service
	Project string
	Zone    string
}

type ConfigFile struct {
	Global ConfigGlobal `gcfg:"global"`
}

type ConfigGlobal struct {
	TokenURL  string `gcfg:"token-url"`
	TokenBody string `gcfg:"token-body"`
	ProjectId string `gcfg:"project-id"`
	Zone      string `gcfg:"zone"`
}

func NewCloud(ctx context.Context, version string, configPath string, filestoreServiceEndpoint string) (*Cloud, error) {
	configFile, err := maybeReadConfig(configPath)
	if err != nil {
		return nil, err
	}

	tokenSource, err := generateTokenSource(ctx, configFile)
	if err != nil {
		return nil, err
	}

	client, err := newOauthClient(ctx, tokenSource)
	if err != nil {
		return nil, err
	}

	file, err := file.NewGCFSService(version, client, filestoreServiceEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Filestore service: %v", err)
	}

	project, zone, err := getProjectAndZone(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize project information: %v", err)
	}
	return &Cloud{
		File:    file,
		Project: project,
		Zone:    zone,
	}, nil
}

func maybeReadConfig(configPath string) (*ConfigFile, error) {
	if configPath == "" {
		return nil, nil
	}

	reader, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("couldn't open cloud provider configuration at %s: %v", configPath, err)
	}
	defer reader.Close()

	cfg := &ConfigFile{}
	if err := gcfg.FatalOnly(gcfg.ReadInto(cfg, reader)); err != nil {
		return nil, fmt.Errorf("couldn't read cloud provider configuration at %s: %v", configPath, err)
	}
	klog.Infof("Config file read %#v", cfg)
	return cfg, nil
}

func generateTokenSource(ctx context.Context, configFile *ConfigFile) (oauth2.TokenSource, error) {
	// If configFile.Global.TokenURL is defined use AltTokenSource
	if configFile != nil && configFile.Global.TokenURL != "" && configFile.Global.TokenURL != "nil" {
		tokenSource := NewAltTokenSource(configFile.Global.TokenURL, configFile.Global.TokenBody)
		klog.Infof("Using AltTokenSource %#v", tokenSource)
		return tokenSource, nil
	}

	// Use DefaultTokenSource
	tokenSource, err := google.DefaultTokenSource(
		ctx,
		compute.CloudPlatformScope)

	// DefaultTokenSource relies on GOOGLE_APPLICATION_CREDENTIALS env var being set.
	if gac, ok := os.LookupEnv("GOOGLE_APPLICATION_CREDENTIALS"); ok {
		klog.Infof("GOOGLE_APPLICATION_CREDENTIALS env var set %v", gac)
	} else {
		klog.Warningf("GOOGLE_APPLICATION_CREDENTIALS env var not set")
	}
	klog.Infof("Using DefaultTokenSource %#v", tokenSource)

	return tokenSource, err
}

func newOauthClient(ctx context.Context, tokenSource oauth2.TokenSource) (*http.Client, error) {
	if err := wait.PollImmediate(5*time.Second, 30*time.Second, func() (bool, error) {
		if _, err := tokenSource.Token(); err != nil {
			klog.Errorf("error fetching initial token: %v", err)
			return false, nil
		}
		return true, nil
	}); err != nil {
		return nil, err
	}

	return oauth2.NewClient(ctx, tokenSource), nil
}

// getProjectAndZone fetches project and zone information from either the configFile or metadata server.
// The lookup is first done in configFile contents and then metadata server.
func getProjectAndZone(config *ConfigFile) (string, string, error) {
	var err error
	var zone string
	if config == nil || config.Global.Zone == "" {
		zone, err = metadata.Zone()
		if err != nil {
			return "", "", err
		}
		klog.Infof("Using GCP zone from the Metadata server: %q", zone)
	} else {
		zone = config.Global.Zone
		klog.Infof("Using GCP zone from the local GCE cloud provider config file: %q", zone)
	}

	var projectID string
	if config == nil || config.Global.ProjectId == "" {
		// Project ID is not available from the local GCE cloud provider config file.
		// This could happen if the driver is not running in the master VM.
		// Defaulting to project ID from the Metadata server.
		projectID, err = metadata.ProjectID()
		if err != nil {
			return "", "", err
		}
		klog.Infof("Using GCP project ID %q from the Metadata server", projectID)
	} else {
		projectID = config.Global.ProjectId
		klog.Infof("Using GCP project ID %q from the local GCE cloud provider config file: %#v", config, projectID)
	}

	return projectID, zone, nil
}
