/*
Copyright 2023 The Kubernetes Authors.
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

package lockrelease

import (
	"testing"
	"time"

	runtime "k8s.io/apimachinery/pkg/runtime"
	core "k8s.io/client-go/testing"
	"k8s.io/klog/v2"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
)

func NewFakeLockReleaseController() *LockReleaseController {
	return &LockReleaseController{}
}

func NewFakeLockReleaseControllerWithClient(t *testing.T, objs []runtime.Object) *LockReleaseController {
	client := fake.NewSimpleClientset(objs...)
	informer := informers.NewSharedInformerFactory(client, time.Hour /* disable resync*/)
	configmapInformer := informer.Core().V1().ConfigMaps()

	// Fill the informers with initial objects so controller can Get() them.
	for _, obj := range objs {
		switch obj.(type) {
		case *v1.ConfigMap:
			configmapInformer.Informer().GetStore().Add(obj)
		default:
			t.Fatalf("Unknown initalObject type: %+v", obj)
		}
	}

	// This reactor makes sure that all updates that the controller does are
	// reflected in its informers so Lister.Get() finds them. This does not
	// enqueue events!
	client.Fake.PrependReactor("create", "*", func(action core.Action) (bool, runtime.Object, error) {
		if action.GetVerb() == "create" {
			switch action.GetResource().Resource {
			case "configmaps":
				klog.V(5).Infof("Test reactor: updated configmap")
				configmapInformer.Informer().GetStore().Add(action.(core.UpdateAction).GetObject())
			default:
				t.Errorf("Unknown update resource: %s", action.GetResource())
			}
		}
		return false, nil, nil
	})

	return &LockReleaseController{
		client:          client,
		configmapLister: configmapInformer.Lister(),
	}
}
