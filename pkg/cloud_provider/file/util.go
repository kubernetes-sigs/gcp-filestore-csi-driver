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

package file

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/golang/glog"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	compute "google.golang.org/api/compute/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

func newOauthClient() (*http.Client, error) {
	if gac, ok := os.LookupEnv("GOOGLE_APPLICATION_CREDENTIALS"); ok {
		glog.V(10).Infof("GOOGLE_APPLICATION_CREDENTIALS env var set %v", gac)
		if _, err := os.Stat(gac); err != nil {
			return nil, fmt.Errorf("error accessing GCP service account token %q: %v", gac, err)
		}
	} else {
		glog.Warningf("GOOGLE_APPLICATION_CREDENTIALS env var not set")
	}

	tokenSource, err := google.DefaultTokenSource(oauth2.NoContext, compute.CloudPlatformScope)
	if err != nil {
		return nil, err
	}

	if err := wait.PollImmediate(5*time.Second, 30*time.Second, func() (bool, error) {
		if _, err := tokenSource.Token(); err != nil {
			glog.Errorf("error fetching initial token: %v", err)
			return false, nil
		}
		return true, nil
	}); err != nil {
		return nil, err
	}

	return oauth2.NewClient(oauth2.NoContext, tokenSource), nil
}
