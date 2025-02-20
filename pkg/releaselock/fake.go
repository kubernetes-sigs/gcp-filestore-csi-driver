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

import "k8s.io/client-go/kubernetes"

type FakeLockReleaseControllerBuilder struct {
	client      kubernetes.Interface
	processor   EventProcessor
	lockService LockService
}

func NewControllerBuilder() *FakeLockReleaseControllerBuilder {
	return &FakeLockReleaseControllerBuilder{}
}

func (b *FakeLockReleaseControllerBuilder) WithClient(client kubernetes.Interface) *FakeLockReleaseControllerBuilder {
	b.client = client
	return b
}

func (b *FakeLockReleaseControllerBuilder) WithProcessor(processor EventProcessor) *FakeLockReleaseControllerBuilder {
	b.processor = processor
	return b
}

func (b *FakeLockReleaseControllerBuilder) WithLockService(lockService LockService) *FakeLockReleaseControllerBuilder {
	b.lockService = lockService
	return b
}

func (b *FakeLockReleaseControllerBuilder) Build() *LockReleaseController {
	c := &LockReleaseController{
		client:         b.client,
		eventProcessor: b.processor,
		lockService:    b.lockService,
	}
	if b.processor != nil {
		b.processor.SetController(c)
	}
	return c
}
