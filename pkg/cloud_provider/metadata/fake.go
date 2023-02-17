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

package metadata

type fakeServiceManager struct{}

var _ Service = &fakeServiceManager{}

func NewFakeService() (Service, error) {
	return &fakeServiceManager{}, nil
}

func (manager *fakeServiceManager) GetZone() string {
	return "us-central1-c"
}

func (manager *fakeServiceManager) GetProject() string {
	return "test-project"
}

func (manager *fakeServiceManager) GetInstanceID() string {
	return "123456"
}

func (manager *fakeServiceManager) GetInternalIP() string {
	return "127.0.0.1"
}
