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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/stretchr/testify/mock"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/file"
)

var (
	fakeResourceTags = map[string]string{
		"openshift/test1/test1": "tagValues/281478395625645",
		"openshift/test2/test2": "tagValues/281481390040765",
		"openshift/test3/test3": "tagValues/281476018424673",
		"openshift/test4/test4": "tagValues/281476661334958",
		"openshift/test5/test5": "tagValues/281475302386112",
	}
)

type FakeTagServiceManager struct {
	mock.Mock
}

func NewFakeCloud() (*Cloud, error) {
	file, err := file.NewFakeService()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Filestore service: %w", err)
	}

	return &Cloud{
		File:    file,
		Project: "test-project",
		Zone:    "us-central1-c",
	}, nil
}

func NewFakeCloudWithFiler(filer file.Service, project, location string) (*Cloud, error) {
	return &Cloud{
		File:    filer,
		Project: project,
		Zone:    location,
	}, nil
}

func NewFakeTagManager() *FakeTagServiceManager { return &FakeTagServiceManager{} }

func NewFakeTagManagerForSanityTests() *FakeTagServiceManager {
	tagMgr := &FakeTagServiceManager{}
	tagMgr.On("AttachResourceTags",
		mock.MatchedBy(func(ctx context.Context) bool { return true }),
		mock.MatchedBy(func(rscType resourceType) bool { return true }),
		mock.MatchedBy(func(rscName string) bool { return true }),
		mock.MatchedBy(func(rscLocation string) bool { return true }),
		mock.MatchedBy(func(reqName string) bool { return true }),
		mock.MatchedBy(func(reqParams map[string]string) bool { return true })).Return(nil)
	return tagMgr
}

func (f *FakeTagServiceManager) SetResourceTags(tag resourceTags) { return }

func (f *FakeTagServiceManager) ValidateResourceTags(ctx context.Context, tagsSource, tags string) (resourceTags, error) {
	rets := f.Called(ctx, tags)
	t, ok := rets[0].(resourceTags)
	if !ok {
		t = resourceTags{}
	}
	e, ok := rets[1].(error)
	if !ok {
		e = nil
	}
	return t, e
}

func (f *FakeTagServiceManager) AttachResourceTags(ctx context.Context, rscType resourceType, rscName, rscLocation, reqName string, reqParameters map[string]string) error {
	rets := f.Called(ctx, rscType, rscName, rscLocation, reqName, reqParameters)
	e, ok := rets[0].(error)
	if !ok {
		return nil
	}
	return e
}

func getFakeGetNamespacedTagValueResp() []byte {
	return []byte(`{"createTime":"2023-06-30T09:42:13.159502Z","etag":"qu7uMWWfyegLF6NDZwXrwQ==","name":"tagValues/281478395625645","namespacedName":"openshift/test1/test1","parent":"tagKeys/281483145989543","shortName":"test1","updateTime":"2023-06-30T09:42:13.159502Z"}`)
}

func getFakeGetNamespacedTagValueNotFoundErrorResp(tagValueNamespacedName string) []byte {
	return []byte(fmt.Sprintf(`{"error":{"code":404,"message":"%s tag does not exist.","status":"NOT_FOUND"}}`, tagValueNamespacedName))
}

func getFakeGetNamespacedTagValueForbiddenErrorResp(tagValueNamespacedName string) []byte {
	return []byte(fmt.Sprintf(`{"error":{"code":403,"message":"Permission denied on resource '%s' (or it may not exist).","status":"PERMISSION_DENIED"}}`, tagValueNamespacedName))
}

func getFakeListEffectiveTagsResp() []byte {
	return []byte(`{"effectiveTags":[{"tagValue":"tagValues/281483998077332","namespacedTagValue":"openshift/test3/test3","tagKey":"tagKeys/281482830535601","namespacedTagKey":"openshift/test3","inherited":true,"tagKeyParentName":"projects/test-project"},
{"tagValue":"tagValues/281478395625645","namespacedTagValue":"openshift/test1/test1","tagKey":"tagKeys/281478395625645","namespacedTagKey":"openshift/test1","inherited":true,"tagKeyParentName":"projects/test-project"}]}`)
}

func getFakeListEffectiveTagsEmptyResp() []byte {
	return []byte(`{"effectiveTags":[]}`)
}

func getFakeListEffectiveTagsForbiddenErrorResp(resource string) []byte {
	return []byte(fmt.Sprintf(`{"error":{"code":403,"message":"The caller does not have permission","status":"PERMISSION_DENIED","details":[{"@type":"type.googleapis.com/google.rpc.ResourceInfo","resourceName":"%s","description":"permission [resourcemanager.hierarchyNodes.listEffectiveTags] required (or the resource may not exist in this location)"}]}}`, resource))
}

func getFakeCreateTagBindingResp(parent, tagValue, tagValueNamespacedName string) []byte {
	name := fmt.Sprintf("tagBindings/%s/%s", url.PathEscape(parent), tagValue)
	return []byte(fmt.Sprintf(`{"done":true,"response":{"@type":"type.googleapis.com/google.cloud.resourcemanager.v3.TagBinding","name":"%s","parent":"%s","tagValue":"%s","tagValueNamespacedName":"%s"}}`, name, parent, tagValue, tagValueNamespacedName))
}

func getFakeCreateTagBindingOngoingResp(parent, tagValue, tagValueNamespacedName string) []byte {
	name := fmt.Sprintf("tagBindings/%s/%s", url.PathEscape(parent), tagValue)
	return []byte(fmt.Sprintf(`{"done":false,"response":{"@type":"type.googleapis.com/google.cloud.resourcemanager.v3.TagBinding","name":"%s","parent":"%s","tagValue":"%s","tagValueNamespacedName":"%s"}}`, name, parent, tagValue, tagValueNamespacedName))
}

func getFakeCreateTagBindingConflictErrorResp(tagValue string) []byte {
	return []byte(fmt.Sprintf(`{"error":{"code":409,"message":"A binding already exists between the given resource and TagValue.","status":"ALREADY_EXISTS","details":[{"@type":"type.googleapis.com/google.rpc.PreconditionFailure","violations":[{"type":"EXISTING_BINDING","subject":"//cloudresourcemanager.googleapis.com/%s","description":"Conflicting TagValue."}]}]}}`, tagValue))
}

func getFakeCreateTagBindingForbiddenErrorResp(tagValue, tagValueNamespacedName string) []byte {
	return []byte(fmt.Sprintf(`{"error":{"code":403,"message":"Permission denied on resource '%s' (or it may not exist)","status":"PERMISSION_DENIED","details":[{"@type":"type.googleapis.com/google.rpc.PreconditionFailure","violations":[{"type":"PERMISSION_DENIED","subject":"//cloudresourcemanager.googleapis.com/%s","description":"Permission Denied"}]}]}}`, tagValueNamespacedName, tagValue))
}

func getFakeTagValue(tagValueNamespacedName string) string {
	return fakeResourceTags[tagValueNamespacedName]
}

func fakeGetNamespacedTagValueHandler(retFailureFor interface{}, w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	if name, ok := query["name"]; ok {
		if len(name) != 0 {
			scenarios := retFailureFor.(map[string]int)
			failure := scenarios[name[0]]
			switch failure {
			case http.StatusNotFound:
				w.WriteHeader(http.StatusNotFound)
				w.Write(getFakeGetNamespacedTagValueNotFoundErrorResp(name[0]))
				return
			case http.StatusUnauthorized:
				w.WriteHeader(http.StatusUnauthorized)
				w.Write(getFakeGetNamespacedTagValueForbiddenErrorResp(name[0]))
				return
			}
		}
	}
	w.WriteHeader(http.StatusOK)
	w.Write(getFakeGetNamespacedTagValueResp())
	return
}

func fakeListEffectiveTagsHandler(retFailureFor interface{}, w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	if parent, ok := query["parent"]; ok {
		if len(parent) != 0 {
			scenarios := retFailureFor.(map[string]int)
			failure := scenarios[parent[0]]
			switch failure {
			case http.StatusOK:
				w.WriteHeader(http.StatusNoContent)
				w.Write(getFakeListEffectiveTagsEmptyResp())
			case http.StatusUnauthorized:
				w.WriteHeader(http.StatusUnauthorized)
				w.Write(getFakeListEffectiveTagsForbiddenErrorResp(parent[0]))
				return
			}
		}
	}
	w.WriteHeader(http.StatusOK)
	w.Write(getFakeListEffectiveTagsResp())
	return
}

func fakeCreateTagBindingHandler(retFailureFor interface{}, w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	req := new(struct {
		Parent                 string `json:"parent,omitempty"`
		TagValue               string `json:"tagValue,omitempty"`
		TagValueNamespacedName string `json:"tagValueNamespacedName,omitempty"`
	})
	if err := json.Unmarshal(body, req); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	req.TagValue = getFakeTagValue(req.TagValueNamespacedName)
	scenarios := retFailureFor.(map[string]int)
	failure := scenarios[req.TagValueNamespacedName]
	switch failure {
	case http.StatusConflict:
		w.WriteHeader(http.StatusConflict)
		w.Write(getFakeCreateTagBindingConflictErrorResp(req.TagValue))
		return
	case http.StatusForbidden:
		w.WriteHeader(http.StatusForbidden)
		w.Write(getFakeCreateTagBindingForbiddenErrorResp(req.TagValue, req.TagValueNamespacedName))
		return
	case http.StatusAccepted:
		w.WriteHeader(http.StatusAccepted)
		w.Write(getFakeCreateTagBindingOngoingResp(req.Parent, req.TagValue, req.TagValueNamespacedName))
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(getFakeCreateTagBindingResp(req.Parent, req.TagValue, req.TagValueNamespacedName))
	return
}

func fakeAPIServerHandler(retFailureFor interface{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uri := r.URL.RequestURI()
		switch {
		case strings.HasPrefix(uri, "/v3/tagValues/namespaced?"):
			fakeGetNamespacedTagValueHandler(retFailureFor, w, r)
		case strings.HasPrefix(uri, "/v3/effectiveTags?"):
			fakeListEffectiveTagsHandler(retFailureFor, w, r)
		case strings.HasPrefix(uri, "/v3/tagBindings?"):
			fakeCreateTagBindingHandler(retFailureFor, w, r)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func NewFakeAPIServer(retFailureFor interface{}) *httptest.Server {
	return httptest.NewTLSServer(fakeAPIServerHandler(retFailureFor))
}
