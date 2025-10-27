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

package cloud

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	rscmgr "cloud.google.com/go/resourcemanager/apiv3"
	rscmgrpb "cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"github.com/googleapis/gax-go/v2"
	"github.com/googleapis/gax-go/v2/apierror"
	"golang.org/x/time/rate"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"k8s.io/klog/v2"
)

const (
	// ParameterKeyResourceTags is the name of the key defined in the parameters'
	// field of the StorageClass object, which holds the user defined resource tags.
	ParameterKeyResourceTags = "resource-tags"

	// tagsDelimiter is the character for separating the tag of the form
	// <parentID>/<tagKey>/<tagValue> in to parentID, tag key and tag value.
	tagsDelimiter = "/"

	// maxTagsPerResource is the maximum number of resource tags that can
	// be attached to a resource.
	// https://cloud.google.com/resource-manager/docs/limits#tag-limits
	maxTagsPerResource = 50

	// resourceManagerHostSubPath is the endpoint for tag requests.
	resourceManagerHostSubPath = "cloudresourcemanager.googleapis.com"

	// filestoreInstanceFullNameFmt is the string format for the full resource name of a
	// filestore Instance resource.
	filestoreInstanceFullNameFmt = "//file.googleapis.com/projects/%s/locations/%s/instances/%s"

	// filestoreBackupFullNameFmt is the string format for the full resource name of a
	// filestore Backup resource.
	filestoreBackupFullNameFmt = "//file.googleapis.com/projects/%s/locations/%s/backups/%s"

	// tagsRequestRateLimit is the tag request rate limit per second.
	tagsRequestRateLimit = 8

	// tagsRequestTokenBucketSize is the burst/token bucket size used.
	// for limiting API requests.
	tagsRequestTokenBucketSize = 8
)

const (
	// FilestoreInstance is for specifying GCP Filestore Instance resource.
	FilestoreInstance resourceType = "FilestoreInstance"

	// FilestoreBackUp is for specifying GCP Filestore Backup resource.
	FilestoreBackUp resourceType = "FilestoreBackUp"
)

var (
	// resourceTagsValueFiller is the filler value for resourceTags type.
	resourceTagsValueFiller = struct{}{}
)

// resourceType is for identifying the GCP resource.
type resourceType string

// resourceTags is the custom type for holding tags.
type resourceTags map[string]struct{}

// withoutAuthentication is for indicating to set WithoutAuthentication client
// option when creating GCP tag client. Setting this will result in no other
// authentication options being used.
type withoutAuthentication struct{}

// tagServiceManager handles resource tagging.
type tagServiceManager struct {
	*Cloud
	tags resourceTags

	// httpClient is currently used for mocking client for tests.
	httpClient *http.Client
	// httpEndpoint is the endpoint used for tests and can be used
	// with httpClient only.
	httpEndpoint string
	// withoutAuthentication is currently used only for tests. It can
	// also be utilized for accessing public resources.
	withoutAuthentication bool
}

// TagService is the interface that wraps methods for resource tag operations.
type TagService interface {
	SetResourceTags(resourceTags)
	ValidateResourceTags(context.Context, string, string) (resourceTags, error)
	AttachResourceTags(context.Context, resourceType, string, string, string, map[string]string) error
}

// TagServiceOptions is for specifying the optional TagService arguments.
type TagServiceOptions interface{}

// tagValuesClient handles operations on tag value resources.
type tagValuesClient struct {
	*rscmgr.TagValuesClient
}

// tagBindingsClient handles operations on tag binding resources.
type tagBindingsClient struct {
	*rscmgr.TagBindingsClient
}

// removeDuplicateTags removes duplicate tags in place which have
// same parentID and tag key but different tag value and expects
// tags are vetted.
func (src *resourceTags) removeDuplicateTags() {
	tagSlice := make([]string, 0, len(*src))
	for k := range *src {
		tagSlice = append(tagSlice, k)
	}
	sort.Strings(tagSlice)
	uniqTags := make(map[string]string)
	for _, tag := range tagSlice {
		idx := strings.LastIndex(tag, tagsDelimiter)
		key := tag[:idx]
		val := tag[idx+1:]
		v, exist := uniqTags[key]
		if !exist {
			uniqTags[key] = val
			continue
		}
		if val != v {
			delete(*src, tag)
		}
	}
}

// mergeTags merges tags from src to dst and removes
// duplicate tags in dst after merge.
func (src *resourceTags) mergeTags(dst *resourceTags) {
	if *dst == nil {
		*dst = make(resourceTags)
	}
	for k, v := range *src {
		(*dst)[k] = v
	}
	dst.removeDuplicateTags()
}

// NewTagManager creates a tagServiceManager instance.
func NewTagManager(cloud *Cloud, opts ...TagServiceOptions) TagService {
	mgr := &tagServiceManager{
		Cloud: cloud,
	}
	mgr.parseTagServiceOptions(opts)
	return mgr
}

// parseTagServiceOptions is for parsing and initializing optional
// TagService arguments.
func (t *tagServiceManager) parseTagServiceOptions(opts []TagServiceOptions) {
	for _, opt := range opts {
		switch val := opt.(type) {
		case resourceTags:
			t.tags = val
		case *http.Client:
			t.httpClient = val
		case withoutAuthentication:
			t.withoutAuthentication = true
		}
	}
	return
}

// SetResourceTags updates tags.
func (t *tagServiceManager) SetResourceTags(tags resourceTags) {
	t.tags = tags
}

// newRequestLimiter returns token bucket based request rate limiter after initializing
// the passed values for limit, burst(or token bucket) size. If opted for emptyBucket
// all initial tokens are reserved for the first burst.
func newRequestLimiter(limit, burst int, emptyBucket bool) *rate.Limiter {
	limiter := rate.NewLimiter(rate.Every(time.Second/time.Duration(limit)), burst)

	if emptyBucket {
		limiter.AllowN(time.Now(), burst)
	}

	return limiter
}

// getTagCreateCallOptions returns a list of additional call options to use for
// the create operations.
func getTagCreateCallOptions() []gax.CallOption {
	const (
		initialRetryDelay    = 90 * time.Second
		maxRetryDuration     = 5 * time.Minute
		retryDelayMultiplier = 2.0
	)

	return []gax.CallOption{
		gax.WithRetry(func() gax.Retryer {
			return gax.OnHTTPCodes(gax.Backoff{
				Initial:    initialRetryDelay,
				Max:        maxRetryDuration,
				Multiplier: retryDelayMultiplier,
			},
				http.StatusTooManyRequests)
		}),
	}
}

// getTagClientOptions returns the tag client options adding the credentials and
// the endpoint which will be used by the client.
func (t *tagServiceManager) getTagClientOptions(ctx context.Context, endpoint string) ([]option.ClientOption, error) {
	opts := make([]option.ClientOption, 0)

	if !t.withoutAuthentication {
		tokenSource, err := generateTokenSource(ctx, t.Config)
		if err != nil {
			return nil, err
		}
		opts = append(opts, option.WithTokenSource(tokenSource))
	} else {
		opts = append(opts, option.WithoutAuthentication())
	}

	if t.httpClient != nil {
		if t.httpEndpoint == "" {
			t.httpEndpoint = endpoint
		}
		opts = append(opts, option.WithHTTPClient(t.httpClient), option.WithEndpoint(t.httpEndpoint))
	} else {
		opts = append(opts, option.WithEndpoint(endpoint))
	}

	return opts, nil
}

// newTagValuesClient returns the client to be used for fetching the tag value resource.
func (t *tagServiceManager) newTagValuesClient(ctx context.Context, endpoint string) (*tagValuesClient, error) {
	opts, err := t.getTagClientOptions(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	client, err := rscmgr.NewTagValuesRESTClient(ctx, opts...)
	return &tagValuesClient{client}, err
}

// newTagBindingsClient returns the client to be used for creating and listing
// tag bindings on the resources.
func (t *tagServiceManager) newTagBindingsClient(ctx context.Context, endpoint string) (*tagBindingsClient, error) {
	opts, err := t.getTagClientOptions(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	client, err := rscmgr.NewTagBindingsRESTClient(ctx, opts...)
	return &tagBindingsClient{client}, err
}

// extractTags extracts user-defined resource tags in the request parameters.
func extractTags(ctx context.Context, tagService TagService, requestName string, parameters map[string]string) (resourceTags, error) {
	if parameters != nil {
		if tags, exist := parameters[ParameterKeyResourceTags]; exist {
			return tagService.ValidateResourceTags(ctx, fmt.Sprintf("%s create request", requestName), tags)
		}
	}
	return nil, nil
}

// ValidateResourceTags converts the tags from string to map
// example: "parentID_1/tagKey_1/tagValue_1,...,parentID_N/tagKey_N/tagValue_N"
// gets converted into {"parentID_1/tagKey_1/tagValue_1": {}, "parentID_N/tagKey_N/tagValue_N": {}}
// And also checks if the user provided tags already exist and validates the number of tags allowed.
func (t *tagServiceManager) ValidateResourceTags(ctx context.Context, tagsSource, commaSeparatedTags string) (resourceTags, error) {
	const (
		tagListDelimiter   = ","
		tagsDelimiterCount = 2
	)

	tags := make(resourceTags)
	if len(commaSeparatedTags) == 0 {
		return tags, nil
	}

	klog.V(5).Infof("configured list of resource tags provided in %s: %s", tagsSource, commaSeparatedTags)
	tagList := strings.Split(commaSeparatedTags, tagListDelimiter)

	if len(tagList) > maxTagsPerResource {
		return nil, fmt.Errorf("more than %d tags is not allowed, number of tags provided in %s: %d", maxTagsPerResource, tagsSource, len(tagList))
	}

	endpoint := fmt.Sprintf("https://%s", resourceManagerHostSubPath)
	client, err := t.newTagValuesClient(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	defer client.close()

	nonexistentTags := make([]string, 0)
	for _, tag := range tagList {
		name := strings.TrimSpace(tag)
		if c := strings.Count(name, tagsDelimiter); c != tagsDelimiterCount {
			return nil, fmt.Errorf("%s tag provided in %s not in expected format(<parentID/tagKey_name/tagValue_name>)", name, tagsSource)
		}
		if err := client.validateTagExist(ctx, name); err != nil {
			// check and return all non-existing tags at once
			// for user to fix in one go.
			var gErr *apierror.APIError
			// google API returns StatusForbidden when the tag does not exist,
			// since it could be because API unable to find tag due to permission issues
			// or genuinely tag does not exist.
			if errors.As(err, &gErr) && gErr.HTTPCode() == http.StatusForbidden {
				klog.V(5).Infof("does not have permission to access %s tag or does not exist, provided in %s", name, tagsSource)
				nonexistentTags = append(nonexistentTags, name)
				continue
			}
			return nil, err
		}
		tags[name] = resourceTagsValueFiller
	}

	if len(nonexistentTags) != 0 {
		return nil, fmt.Errorf("%v tag(s) provided in %s does not exist", nonexistentTags, tagsSource)
	}

	(&tags).removeDuplicateTags()

	klog.V(4).Infof("validated list of resource tags provided in %s: %v", tagsSource, tags)
	return tags, nil
}

// AttachResourceTags creates tag bindings on the resource by skipping the
// tag bindings already existing on the resource either inherited or partial
// success during previous operation.
func (t *tagServiceManager) AttachResourceTags(ctx context.Context, rscType resourceType, rscName, rscLocation, reqName string, reqParameters map[string]string) error {
	tags, err := extractTags(ctx, t, reqName, reqParameters)
	if err != nil {
		return err
	}

	if len(t.tags) == 0 && len(tags) == 0 {
		return nil
	}

	t.tags.mergeTags(&tags)

	endpoint := fmt.Sprintf("https://%s-%s", rscLocation, resourceManagerHostSubPath)
	client, err := t.newTagBindingsClient(ctx, endpoint)
	if err != nil {
		return err
	}
	defer client.close()

	var fullResourceName string
	switch rscType {
	case FilestoreInstance:
		fullResourceName = fmt.Sprintf(filestoreInstanceFullNameFmt, t.Project, rscLocation, rscName)
	case FilestoreBackUp:
		fullResourceName = fmt.Sprintf(filestoreBackupFullNameFmt, t.Project, rscLocation, rscName)
	default:
		return fmt.Errorf("unsupported resource type: %s:%s", rscType, rscName)
	}

	refinedTagList, err := client.getTagsToBind(ctx, fullResourceName, tags)
	if err != nil {
		return err
	}
	if len(refinedTagList) == 0 {
		return nil
	}

	if err := client.createTagBindings(ctx, fullResourceName, refinedTagList); err != nil {
		return err
	}

	effectiveTags := client.getEffectiveTagList(ctx, rscName)
	klog.V(4).Infof("list of tags attached to %s: %v", rscName, effectiveTags)

	return nil
}

// close the tag value client connection to API server.
func (c *tagValuesClient) close() {
	if err := c.Close(); err != nil {
		klog.Errorf("failed to close tag value client connection: %v", err)
	}
}

// validateTagExist checks whether the tag exist using the tag's Namespaced name.
func (c *tagValuesClient) validateTagExist(ctx context.Context, name string) error {
	if _, err := c.GetNamespacedTagValue(ctx, &rscmgrpb.GetNamespacedTagValueRequest{
		Name: name,
	}); err != nil {
		return fmt.Errorf("failed to fetch %s tag: %w", name, err)
	}
	return nil
}

// close the tag binding client connection to API server.
func (c *tagBindingsClient) close() {
	if err := c.Close(); err != nil {
		klog.Errorf("failed to close tag binding client connection: %v", err)
	}
}

// getEffectiveTagList returns the list of tags attached to the resource,
// which were inherited or attached to the resource directly.
func (c *tagBindingsClient) getEffectiveTagList(ctx context.Context, rscName string) resourceTags {
	effectiveTags := make(resourceTags)
	bindings := c.listEffectiveTags(ctx, rscName)
	// a resource can have a maximum of {gcpMaxTagsPerResource} tags attached to it.
	// Will iterate for {gcpMaxTagsPerResource} times in the worst case scenario, if
	// none of the break conditions are met. Should the {gcpMaxTagsPerResource} be
	// increased in the future, it should not create an issue, since this is an optimization
	// attempt to reduce the number of tag write calls by skipping already existing tags,
	// since tag write operation has quota restriction.
	for i := 0; i < maxTagsPerResource; i++ {
		binding, err := bindings.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil || binding == nil {
			// on encountering any error will continue adding refined tags
			// which will still have all the user provided tags except for
			// the removed already existing tags processed until this point
			// and would end up adding tags which already exist, and error
			// handling is present for the same.
			klog.V(5).Infof("failed to list effective tags on the %s resource: %v: %v", rscName, binding, err)
			break
		}
		effectiveTags[binding.NamespacedTagValue] = resourceTagsValueFiller
	}
	return effectiveTags
}

func (c *tagBindingsClient) listEffectiveTags(ctx context.Context, name string) *rscmgr.EffectiveTagIterator {
	return c.ListEffectiveTags(ctx, &rscmgrpb.ListEffectiveTagsRequest{
		Parent: name,
	})
}

// getTagsToBind returns the filtered list of tags after removing the tags which
// already exist on the resource.
func (c *tagBindingsClient) getTagsToBind(ctx context.Context, rscName string, tags resourceTags) (resourceTags, error) {
	effectiveTags := c.getEffectiveTagList(ctx, rscName)
	refinedTags := make(resourceTags)

	for tag := range tags {
		if _, exist := effectiveTags[tag]; !exist {
			refinedTags[tag] = resourceTagsValueFiller
		}
	}
	klog.V(4).Infof("refined list of resource tags to attach to %s: %v", rscName, refinedTags)

	return refinedTags, nil
}

// createTagBindings creates the tag bindings for the resource.
func (c *tagBindingsClient) createTagBindings(ctx context.Context, rscName string, tags resourceTags) error {
	if len(tags) == 0 {
		return nil
	}

	// GCP has a rate limit of 600 requests per minute, restricting
	// here to 8 requests per second.
	limiter := newRequestLimiter(tagsRequestRateLimit, tagsRequestTokenBucketSize, true)

	errFlag := false
	for tag := range tags {
		if err := limiter.Wait(ctx); err != nil {
			errFlag = true
			klog.Errorf("rate limiting request to add %s tag to %s resource failed: %v",
				tag, rscName, err)
			continue
		}

		createOp, err := c.createTagBinding(ctx, rscName, tag)
		if err != nil {
			var gErr *apierror.APIError
			if errors.As(err, &gErr) && gErr.HTTPCode() == http.StatusConflict {
				klog.V(5).Infof("tag %s already exists on %s", tag, rscName)
				continue
			}
			errFlag = true
			klog.Errorf("request to add %s tag to %s resource failed: %v", tag, rscName, err)
			continue
		}

		if err = c.wait(ctx, createOp); err != nil {
			errFlag = true
			klog.Errorf("failed to add %s tag to %s resource: %v", tag, rscName, err)
		}
		klog.Infof("successfully added %s tag to %s resource", tag, rscName)
	}
	if errFlag {
		return fmt.Errorf("failed to add tag(s) to %s resource", rscName)
	}

	return nil
}

func (c *tagBindingsClient) createTagBinding(ctx context.Context, parent, tag string) (*rscmgr.CreateTagBindingOperation, error) {
	op, err := c.CreateTagBinding(ctx, &rscmgrpb.CreateTagBindingRequest{
		TagBinding: &rscmgrpb.TagBinding{
			Parent:                 parent,
			TagValueNamespacedName: tag,
		},
	}, getTagCreateCallOptions()...)
	return op, err
}

func (c *tagBindingsClient) wait(ctx context.Context, op *rscmgr.CreateTagBindingOperation) error {
	_, err := op.Wait(ctx)
	return err
}
