package cloud

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	ctx = context.TODO()
)

func (t *tagServiceManager) setTestHTTPEndpoint(endpoint string) {
	t.httpEndpoint = endpoint
}

func TestRemoveDuplicateTags(t *testing.T) {
	cases := []struct {
		name         string
		tags         resourceTags
		expectedTags resourceTags
		expectPanic  bool
	}{
		{
			name:         "empty tag list",
			tags:         nil,
			expectedTags: nil,
		},
		{
			name: "no duplicate tags",
			tags: resourceTags{
				"openshift/test1/test1": resourceTagsValueFiller,
				"openshift/test2/test2": resourceTagsValueFiller,
				"openshift/test3/test3": resourceTagsValueFiller,
				"openshift/test4/test4": resourceTagsValueFiller,
			},
			expectedTags: resourceTags{
				"openshift/test1/test1": resourceTagsValueFiller,
				"openshift/test2/test2": resourceTagsValueFiller,
				"openshift/test3/test3": resourceTagsValueFiller,
				"openshift/test4/test4": resourceTagsValueFiller,
			},
		},
		{
			name: "duplicate tags present",
			tags: resourceTags{
				"openshift/test1/test1":  resourceTagsValueFiller,
				"openshift/test2/test2":  resourceTagsValueFiller,
				"openshift/test1/test11": resourceTagsValueFiller,
				"openshift/test4/test4":  resourceTagsValueFiller,
				"openshift/test2/test22": resourceTagsValueFiller,
				"openshift/test3/test33": resourceTagsValueFiller,
				"openshift/test5/test5":  resourceTagsValueFiller,
			},
			expectedTags: resourceTags{
				"openshift/test1/test1":  resourceTagsValueFiller,
				"openshift/test2/test2":  resourceTagsValueFiller,
				"openshift/test3/test33": resourceTagsValueFiller,
				"openshift/test4/test4":  resourceTagsValueFiller,
				"openshift/test5/test5":  resourceTagsValueFiller,
			},
		},
		{
			name: "invalid tags does not create panic",
			tags: resourceTags{
				"openshift/test1/test1":  resourceTagsValueFiller,
				"openshift/test2/test2":  resourceTagsValueFiller,
				"openshift/test1/test11": resourceTagsValueFiller,
				"test5/test5":            resourceTagsValueFiller,
			},
			expectedTags: resourceTags{
				"openshift/test1/test1": resourceTagsValueFiller,
				"openshift/test2/test2": resourceTagsValueFiller,
				"test5/test5":           resourceTagsValueFiller,
			},
		},
		{
			name: "invalid tags panic error expected",
			tags: resourceTags{
				"openshift/test1/test1":  resourceTagsValueFiller,
				"openshift/test2/test2":  resourceTagsValueFiller,
				"openshift/test1/test11": resourceTagsValueFiller,
				"test6":                  resourceTagsValueFiller,
			},
			expectPanic: true,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if test.expectPanic {
				assert.Panics(t, func() { (&test.tags).removeDuplicateTags() }, "removeDuplicateTags(): expected to panic")
			} else {
				(&test.tags).removeDuplicateTags()
				if !reflect.DeepEqual(test.expectedTags, test.tags) {
					t.Errorf("removeDuplicateTags(): got: %v, want: %v", test.tags, test.expectedTags)
				}
			}
		})
	}
}

func TestMergeTags(t *testing.T) {
	cases := []struct {
		name         string
		srcTags      resourceTags
		dstTags      resourceTags
		expectedTags resourceTags
	}{
		{
			name:         "empty src and dst tags",
			srcTags:      nil,
			dstTags:      nil,
			expectedTags: resourceTags{},
		},
		{
			name:    "empty src tags",
			srcTags: nil,
			dstTags: resourceTags{
				"openshift/test1/test1": resourceTagsValueFiller,
				"openshift/test2/test2": resourceTagsValueFiller,
			},
			expectedTags: resourceTags{
				"openshift/test1/test1": resourceTagsValueFiller,
				"openshift/test2/test2": resourceTagsValueFiller,
			},
		},
		{
			name:    "empty dst tags",
			srcTags: nil,
			dstTags: resourceTags{
				"openshift/test1/test1": resourceTagsValueFiller,
				"openshift/test2/test2": resourceTagsValueFiller,
			},
			expectedTags: resourceTags{
				"openshift/test1/test1": resourceTagsValueFiller,
				"openshift/test2/test2": resourceTagsValueFiller,
			},
		},
		{
			name: "merge tags without duplicates",
			srcTags: resourceTags{
				"openshift/test1/test1": resourceTagsValueFiller,
				"openshift/test2/test2": resourceTagsValueFiller,
			},
			dstTags: resourceTags{
				"openshift/test3/test3": resourceTagsValueFiller,
				"openshift/test4/test4": resourceTagsValueFiller,
			},
			expectedTags: resourceTags{
				"openshift/test1/test1": resourceTagsValueFiller,
				"openshift/test2/test2": resourceTagsValueFiller,
				"openshift/test3/test3": resourceTagsValueFiller,
				"openshift/test4/test4": resourceTagsValueFiller,
			},
		},
		{
			name: "merge tags with duplicates",
			srcTags: resourceTags{
				"openshift/test1/test1": resourceTagsValueFiller,
				"openshift/test2/test2": resourceTagsValueFiller,
			},
			dstTags: resourceTags{
				"openshift/test1/test101": resourceTagsValueFiller,
				"openshift/test2/test202": resourceTagsValueFiller,
			},
			expectedTags: resourceTags{
				"openshift/test1/test1": resourceTagsValueFiller,
				"openshift/test2/test2": resourceTagsValueFiller,
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			test.srcTags.mergeTags(&test.dstTags)
			if !reflect.DeepEqual(test.expectedTags, test.dstTags) {
				t.Errorf("mergeTags(): got: %v, want: %v", test.dstTags, test.expectedTags)
			}
		})
	}
}

func TestExtractTags(t *testing.T) {
	cases := []struct {
		name         string
		parameters   map[string]string
		expectedTags resourceTags
		expectedErr  string
	}{
		{
			name:         "empty parameters in a request",
			parameters:   map[string]string{},
			expectedTags: nil,
		},
		{
			name: "tags not present in parameters of a request",
			parameters: map[string]string{
				"network": "test",
			},
			expectedTags: nil,
		},
		{
			name: "tags defined in parameters of a request",
			parameters: map[string]string{
				"network":                "test",
				ParameterKeyResourceTags: "openshift/test1/test1, openshift/test2/test2",
			},
			expectedTags: resourceTags{
				"openshift/test1/test1": resourceTagsValueFiller,
				"openshift/test2/test2": resourceTagsValueFiller,
			},
		},
		{
			name: "tags defined - incorrect parameter tag key name",
			parameters: map[string]string{
				"network":      "test",
				"resourceTags": "openshift/test1/test1, openshift/test2/test2",
			},
			expectedTags: nil,
		},
		{
			name: "invalid tags defined(without parent ID)",
			parameters: map[string]string{
				"network":                "test",
				ParameterKeyResourceTags: "test1/test1, openshift/test2/test2",
			},
			expectedTags: resourceTags{},
			expectedErr:  "invalid tags",
		},
	}

	tagMgr := NewFakeTagManager()
	tagMgr.On("ValidateResourceTags", ctx, "openshift/test1/test1, openshift/test2/test2").
		Return(resourceTags{
			"openshift/test1/test1": resourceTagsValueFiller,
			"openshift/test2/test2": resourceTagsValueFiller}, nil)
	tagMgr.On("ValidateResourceTags", ctx, "test1/test1, openshift/test2/test2").
		Return(resourceTags{}, fmt.Errorf("invalid tags"))

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			tags, err := extractTags(ctx, tagMgr, "test", test.parameters)
			if (err != nil || test.expectedErr != "") && err.Error() != test.expectedErr {
				t.Errorf("extractTags(): got: %v, wantErr: %v", err, test.expectedErr)
			}
			if !reflect.DeepEqual(test.expectedTags, tags) {
				t.Errorf("extractTags(): got: %#v, want: %#v", tags, test.expectedTags)
			}
		})
	}
}

func TestValidateResourceTags(t *testing.T) {
	cases := []struct {
		name               string
		commaSeparatedTags string
		expectedTags       resourceTags
		expectedErr        string
	}{
		{
			name:               "empty tags string",
			commaSeparatedTags: "",
			expectedTags:       resourceTags{},
			expectedErr:        "",
		},
		{
			name:               "valid tags string",
			commaSeparatedTags: "openshift/test1/test1,openshift/test2/test2",
			expectedTags: resourceTags{
				"openshift/test1/test1": resourceTagsValueFiller,
				"openshift/test2/test2": resourceTagsValueFiller,
			},
			expectedErr: "",
		},
		{
			name:               "non existent tag",
			commaSeparatedTags: "openshift/test1/test1,openshift/test3/test3",
			expectedTags:       resourceTags{},
			expectedErr:        "failed to fetch openshift/test3/test3 tag: googleapi: Error 404: openshift/test3/test3 tag does not exist.",
		},
		{
			name:               "gapi tag not exist error",
			commaSeparatedTags: "openshift/test1/test1,openshift/test4/test4",
			expectedTags:       resourceTags{},
			expectedErr:        "[openshift/test4/test4] tag(s) provided in unit test does not exist",
		},
		{
			name: "max tags limit exceeds",
			commaSeparatedTags: "openshift/test1/test1,openshift/test2/test2,openshift/test3/test3," +
				"openshift/test4/test4,openshift/test5/test5,openshift/test6/test6,openshift/test7/test7," +
				"openshift/test8/test8,openshift/test9/test9,openshift/test10/test10,openshift/test11/test11," +
				"openshift/test12/test12,openshift/test13/test13,openshift/test14/test14,openshift/test15/test15," +
				"openshift/test16/test16,openshift/test17/test17,openshift/test18/test18,openshift/test19/test19," +
				"openshift/test20/test20,openshift/test21/test21,openshift/test22/test22,openshift/test23/test23," +
				"openshift/test24/test24,openshift/test25/test25,openshift/test26/test26,openshift/test27/test27," +
				"openshift/test28/test28,openshift/test29/test29,openshift/test30/test30,openshift/test31/test31," +
				"openshift/test32/test32,openshift/test33/test33,openshift/test34/test34,openshift/test35/test35," +
				"openshift/test36/test36,openshift/test37/test37,openshift/test38/test38,openshift/test39/test39," +
				"openshift/test40/test40,openshift/test41/test41,openshift/test42/test42,openshift/test43/test43," +
				"openshift/test44/test44,openshift/test45/test45,openshift/test46/test46,openshift/test47/test47," +
				"openshift/test48/test48,openshift/test49/test49,openshift/test50/test50,openshift/test51/test51",
			expectedTags: resourceTags{},
			expectedErr:  "more than 50 tags is not allowed, number of tags provided in unit test: 51",
		},
		{
			name:               "invalid tag configured(without parent ID)",
			commaSeparatedTags: "openshift/test1/test1,test4/test4",
			expectedTags:       resourceTags{},
			expectedErr:        "test4/test4 tag provided in unit test not in expected format(<parentID/tagKey_name/tagValue_name>)",
		},
	}

	cloud, err := NewFakeCloud()
	if err != nil {
		t.Errorf("failed to create fake cloud provider object: %v", err)
	}

	server := NewFakeAPIServer(map[string]int{
		"openshift/test3/test3": http.StatusNotFound,
		"openshift/test4/test4": http.StatusUnauthorized,
	})
	defer server.Close()

	tagMgr := NewTagManager(cloud, server.Client(), withoutAuthentication{}).(*tagServiceManager)
	tagMgr.setTestHTTPEndpoint(server.URL)

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			tags, err := tagMgr.ValidateResourceTags(ctx, "unit test", test.commaSeparatedTags)
			tagMgr.SetResourceTags(tags)
			if (err != nil || test.expectedErr != "") && err.Error() != test.expectedErr {
				t.Errorf("ValidateResourceTags(): got: %v, wantErr: %v", err, test.expectedErr)
			}
			if !reflect.DeepEqual(tagMgr.tags, tags) {
				t.Errorf("ValidateResourceTags(): got: %#v, want: %#v", tags, test.expectedTags)
			}
		})
	}
}

func TestNewTagValuesClient(t *testing.T) {
	cloud, err := NewFakeCloud()
	if err != nil {
		t.Errorf("newTagValuesClient(): failed to create fake cloud provider object: %v", err)
	}

	tagMgr := NewTagManager(cloud, withoutAuthentication{}).(*tagServiceManager)
	client, err := tagMgr.newTagValuesClient(ctx, "test/endpoint")
	if err != nil {
		t.Errorf("newTagValuesClient(): failed to create tag values client: %v", err)
	}
	defer client.close()
}

func TestNewTagBindingsClient(t *testing.T) {
	cloud, err := NewFakeCloud()
	if err != nil {
		t.Errorf("newTagBindingsClient(): failed to create fake cloud provider object: %v", err)
	}

	tagMgr := NewTagManager(cloud, withoutAuthentication{}).(*tagServiceManager)
	client, err := tagMgr.newTagBindingsClient(ctx, "test/endpoint")
	if err != nil {
		t.Errorf("newTagBindingsClient(): failed to create tag bindings client: %v", err)
	}
	defer client.close()
}

func TestAttachResourceTags(t *testing.T) {
	cloud, err := NewFakeCloud()
	if err != nil {
		t.Errorf("AttachResourceTags(): failed to create fake cloud provider object: %v", err)
	}

	var (
		filestoreInstanceName     = "test-instance"
		filestoreBackupName       = "test-backup"
		rscLocation               = "asia-south1-a"
		filestoreInstanceFullPath = fmt.Sprintf(filestoreInstanceFullNameFmt, cloud.Project, rscLocation, filestoreInstanceName)
	)

	cases := []struct {
		name             string
		rscType          resourceType
		rscName          string
		rscLocation      string
		reqParameters    map[string]string
		includedUserTags bool
		expectedErr      string
	}{
		{
			name:        "empty user defined tags, valid parameters tags",
			rscType:     FilestoreInstance,
			rscName:     filestoreInstanceName,
			rscLocation: rscLocation,
			reqParameters: map[string]string{
				"network":                "test",
				ParameterKeyResourceTags: "openshift/test1/test1, openshift/test2/test2",
			},
		},
		{
			name:        "empty user defined tags, a tag provided in parameters does not exist",
			rscType:     FilestoreInstance,
			rscName:     filestoreInstanceName,
			rscLocation: rscLocation,
			reqParameters: map[string]string{
				"network":                "test",
				ParameterKeyResourceTags: "openshift/test1/test1, openshift/test3/test3",
			},
			expectedErr: "failed to fetch openshift/test3/test3 tag: googleapi: Error 404: openshift/test3/test3 tag does not exist.",
		},
		{
			name:          "empty user defined tags and parameters tags",
			rscType:       FilestoreInstance,
			rscName:       filestoreInstanceName,
			rscLocation:   rscLocation,
			reqParameters: nil,
		},
		{
			name:        "unsupported resource type argument",
			rscType:     resourceType("test"),
			rscName:     filestoreInstanceName,
			rscLocation: rscLocation,
			reqParameters: map[string]string{
				"network":                "test",
				ParameterKeyResourceTags: "openshift/test1/test1, openshift/test2/test2",
			},
			expectedErr: "unsupported resource type: test:test-instance",
		},
		{
			name:        "tags fetch fails with not found error",
			rscType:     FilestoreBackUp,
			rscName:     filestoreBackupName,
			rscLocation: rscLocation,
			reqParameters: map[string]string{
				"network":                "test",
				ParameterKeyResourceTags: "openshift/test4/test4",
			},
			expectedErr: "[openshift/test4/test4] tag(s) provided in test-backup create request does not exist",
		},
		{
			name:        "no new tags to add, tags already exist on resource",
			rscType:     FilestoreBackUp,
			rscName:     filestoreBackupName,
			rscLocation: rscLocation,
			reqParameters: map[string]string{
				"network":                "test",
				ParameterKeyResourceTags: "openshift/test1/test1",
			},
		},
		{
			name:             "create tag bindings fails(error mocked)",
			rscType:          FilestoreInstance,
			rscName:          filestoreInstanceName,
			rscLocation:      rscLocation,
			includedUserTags: true,
			reqParameters: map[string]string{
				"network":                "test",
				ParameterKeyResourceTags: "openshift/test5/test5",
			},
			expectedErr: "failed to add tag(s) to //file.googleapis.com/projects/test-project/locations/asia-south1-a/instances/test-instance resource",
		},
	}

	server := NewFakeAPIServer(map[string]int{
		"openshift/test3/test3":   http.StatusNotFound,
		"openshift/test4/test4":   http.StatusUnauthorized,
		filestoreInstanceFullPath: http.StatusNoContent,
		"openshift/test5/test5":   http.StatusAccepted,
	})
	defer server.Close()

	tagMgr := NewTagManager(cloud, server.Client(), withoutAuthentication{}).(*tagServiceManager)
	tagMgr.setTestHTTPEndpoint(server.URL)

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if test.includedUserTags {
				tagMgr.SetResourceTags(resourceTags{
					"openshift/test1/test1": resourceTagsValueFiller,
				})
			}

			err := tagMgr.AttachResourceTags(ctx, test.rscType, test.rscName, test.rscLocation, test.rscName, test.reqParameters)
			if (err != nil || test.expectedErr != "") && err.Error() != test.expectedErr {
				t.Errorf("AttachResourceTags(): got: %v, wantErr: %v", err, test.expectedErr)
			}
		})
	}
}

func TestValidateTagExist(t *testing.T) {
	cases := []struct {
		name        string
		tag         string
		expectedErr string
	}{
		{
			name: "tag fetch success",
			tag:  "openshift/test1/test1",
		},
		{
			name:        "tag fetch fails with permission error",
			tag:         "openshift/test2/test2",
			expectedErr: "failed to fetch openshift/test2/test2 tag: googleapi: Error 403: Permission denied on resource 'openshift/test2/test2' (or it may not exist).",
		},
	}

	cloud, err := NewFakeCloud()
	if err != nil {
		t.Errorf("validateTagExist(): failed to create fake cloud provider object: %v", err)
	}

	server := NewFakeAPIServer(map[string]int{
		"openshift/test2/test2": http.StatusUnauthorized,
	})
	defer server.Close()

	tagMgr := NewTagManager(cloud, server.Client(), withoutAuthentication{}).(*tagServiceManager)
	client, err := tagMgr.newTagValuesClient(ctx, server.URL)
	if err != nil {
		t.Errorf("validateTagExist(): failed to create tag bindings client: %v", err)
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			err := client.validateTagExist(ctx, test.tag)
			if (err != nil || test.expectedErr != "") && err.Error() != test.expectedErr {
				t.Errorf("validateTagExist(): got: %v, wantErr: %v", err, test.expectedErr)
			}
		})
	}
}

func TestGetTagsToBind(t *testing.T) {
	cloud, err := NewFakeCloud()
	if err != nil {
		t.Errorf("getTagsToBind(): failed to create fake cloud provider object: %v", err)
	}

	var (
		rscName1 = fmt.Sprintf(filestoreInstanceFullNameFmt, cloud.Project, cloud.Zone, "test-instance1")
		rscName2 = fmt.Sprintf(filestoreInstanceFullNameFmt, cloud.Project, cloud.Zone, "test-instance2")
		argTags  = resourceTags{"openshift/test5/test5": resourceTagsValueFiller}
	)

	cases := []struct {
		name         string
		rscName      string
		tags         resourceTags
		expectedTags resourceTags
		expectedErr  string
	}{
		{
			name:         "empty tags",
			rscName:      rscName1,
			tags:         resourceTags{},
			expectedTags: resourceTags{},
		},
		{
			name:    "duplicate tags exists",
			rscName: rscName1,
			tags: resourceTags{
				"openshift/test1/test1": resourceTagsValueFiller,
				"openshift/test2/test2": resourceTagsValueFiller,
				"openshift/test3/test3": resourceTagsValueFiller,
				"openshift/test4/test4": resourceTagsValueFiller,
			},
			expectedTags: resourceTags{
				"openshift/test2/test2": resourceTagsValueFiller,
				"openshift/test4/test4": resourceTagsValueFiller,
			},
		},
		{
			name:    "no duplicate tags",
			rscName: rscName1,
			tags: resourceTags{
				"openshift/test1/test1": resourceTagsValueFiller,
				"openshift/test2/test2": resourceTagsValueFiller,
				"openshift/test4/test4": resourceTagsValueFiller,
			},
			expectedTags: resourceTags{
				"openshift/test2/test2": resourceTagsValueFiller,
				"openshift/test4/test4": resourceTagsValueFiller,
			},
		},
		{
			name:    "listing effective tags fails with permission error",
			rscName: rscName2,
			tags: resourceTags{
				"openshift/test1/test1": resourceTagsValueFiller,
				"openshift/test2/test2": resourceTagsValueFiller,
				"openshift/test4/test4": resourceTagsValueFiller,
			},
			expectedTags: resourceTags{
				"openshift/test1/test1": resourceTagsValueFiller,
				"openshift/test2/test2": resourceTagsValueFiller,
				"openshift/test4/test4": resourceTagsValueFiller,
			},
		},
	}

	server := NewFakeAPIServer(map[string]int{
		rscName2: http.StatusUnauthorized,
	})
	defer server.Close()

	tagMgr := NewTagManager(cloud, argTags, server.Client(), withoutAuthentication{}).(*tagServiceManager)
	client, err := tagMgr.newTagBindingsClient(ctx, server.URL)
	if err != nil {
		t.Errorf("getTagsToBind(): failed to create tag bindings client: %v", err)
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			tags, err := client.getTagsToBind(ctx, test.rscName, test.tags)
			if (err != nil || test.expectedErr != "") && err.Error() != test.expectedErr {
				t.Errorf("getTagsToBind(): got: %v, wantErr: %v", err, test.expectedErr)
			}
			if !reflect.DeepEqual(test.expectedTags, tags) {
				t.Errorf("getTagsToBind(): got: %#v, want: %#v", tags, test.expectedTags)
			}
		})
	}
}

func TestCreateTagBindings(t *testing.T) {

	cloud, err := NewFakeCloud()
	if err != nil {
		t.Errorf("createTagBindings(): failed to create fake cloud provider object: %v", err)
	}

	var (
		rscName = fmt.Sprintf(filestoreInstanceFullNameFmt, cloud.Project, cloud.Zone, "test-instance")
		tags1   = resourceTags{
			"openshift/test1/test1": resourceTagsValueFiller,
			"openshift/test2/test2": resourceTagsValueFiller,
		}
		tags2 = resourceTags{
			"openshift/test1/test1": resourceTagsValueFiller,
			"openshift/test3/test3": resourceTagsValueFiller,
		}
		tags3 = resourceTags{
			"openshift/test1/test1": resourceTagsValueFiller,
			"openshift/test4/test4": resourceTagsValueFiller,
		}
		tags4 = resourceTags{
			"openshift/test1/test1": resourceTagsValueFiller,
			"openshift/test5/test5": resourceTagsValueFiller,
		}
	)

	cases := []struct {
		name        string
		rscName     string
		tags        resourceTags
		expectedErr string
	}{
		{
			name:    "empty tags",
			rscName: rscName,
			tags:    resourceTags{},
		},
		{
			name:    "tag bindings creation success",
			rscName: rscName,
			tags:    tags1,
		},
		{
			name:        "tag bindings creation wait operation failure",
			rscName:     rscName,
			tags:        tags2,
			expectedErr: fmt.Sprintf("failed to add tag(s) to %s resource", rscName),
		},
		{
			name:    "tag bindings creation success - skip existing tags",
			rscName: rscName,
			tags:    tags3,
		},
		{
			name:        "tag bindings creation fails with permission error",
			rscName:     rscName,
			tags:        tags4,
			expectedErr: fmt.Sprintf("failed to add tag(s) to %s resource", rscName),
		},
	}

	server := NewFakeAPIServer(map[string]int{
		"openshift/test3/test3": http.StatusAccepted,
		"openshift/test4/test4": http.StatusConflict,
		"openshift/test5/test5": http.StatusForbidden,
	})
	defer server.Close()

	tagMgr := NewTagManager(cloud, server.Client(), withoutAuthentication{}).(*tagServiceManager)
	client, err := tagMgr.newTagBindingsClient(ctx, server.URL)
	if err != nil {
		t.Errorf("createTagBindings(): failed to create tag bindings client: %v", err)
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			err := client.createTagBindings(ctx, test.rscName, test.tags)
			if (err != nil || test.expectedErr != "") && err.Error() != test.expectedErr {
				t.Errorf("createTagBindings(): got: %v, wantErr: %v", err, test.expectedErr)
			}
		})
	}
}
