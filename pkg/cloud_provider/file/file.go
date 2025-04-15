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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/googleapis/gax-go/v2/apierror"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/common"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/util"

	filev1beta1 "google.golang.org/api/file/v1beta1"
	filev1beta1multishare "google.golang.org/api/file/v1beta1"
)

const (
	testEndpoint    = "test-file.sandbox.googleapis.com"
	stagingEndpoint = "staging-file.sandbox.googleapis.com"
	prodEndpoint    = "file.googleapis.com"
)

type PollOpts struct {
	Interval time.Duration
	Timeout  time.Duration
}
type NfsExportOptions struct {
	AccessMode string   `json:"accessMode,omitempty"`
	AnonGid    int64    `json:"anonGid,omitempty,string"`
	AnonUid    int64    `json:"anonUid,omitempty,string"`
	IpRanges   []string `json:"ipRanges,omitempty"`
	SquashMode string   `json:"squashMode,omitempty"`
}

type Share struct {
	Name             string              // only the share name
	Parent           *MultishareInstance // parent captures the project, location details.
	State            string
	MountPointName   string
	Labels           map[string]string
	CapacityBytes    int64
	BackupId         string
	NfsExportOptions []*NfsExportOptions
}

type MultishareInstance struct {
	Project            string
	Name               string
	Location           string
	Tier               string
	Network            Network
	CapacityBytes      int64
	MaxCapacityBytes   int64
	CapacityStepSizeGb int64
	Labels             map[string]string
	State              string
	KmsKeyName         string
	Description        string
	MaxShareCount      int
	Protocol           string
}

func (i *MultishareInstance) String() string {
	return fmt.Sprintf("%s/%s/%s", i.Project, i.Location, i.Name)
}

type ListFilter struct {
	Project      string
	Location     string
	InstanceName string
}

type ServiceInstance struct {
	Project          string
	Name             string
	Location         string
	Tier             string
	Network          Network
	Volume           Volume
	Labels           map[string]string
	State            string
	KmsKeyName       string
	BackupSource     string
	NfsExportOptions []*NfsExportOptions
	Protocol         string
}

type Volume struct {
	Name      string
	SizeBytes int64
}

type Network struct {
	Name            string
	ConnectMode     string
	ReservedIpRange string
	Ip              string
}

type Backup struct {
	Backup            *filev1beta1.Backup
	SourceInstance    string
	SourceShare       string
	FileSystemProtocl string
}

type BackupInfo struct {
	Name               string
	SourceVolumeId     string
	BackupURI          string
	SourceInstance     string
	SourceInstanceName string
	SourceShare        string
	Project            string
	Location           string
	Tier               string
	Labels             map[string]string
}

func (bi *BackupInfo) SourceVolumeLocation() string {
	splitId := strings.Split(bi.SourceVolumeId, "/")
	// Format: "modeInstance/us-central1/myinstance/myshare",

	return splitId[1]
}

func (bi *BackupInfo) BackupSource() string {
	if isMultishareVolId(bi.SourceVolumeId) {
		return shareURI(bi.Project, bi.SourceVolumeLocation(), bi.SourceInstanceName, bi.SourceShare)
	} else {
		return instanceURI(bi.Project, bi.SourceVolumeLocation(), bi.SourceInstanceName)
	}
}

type Service interface {
	CreateInstance(ctx context.Context, obj *ServiceInstance) (*ServiceInstance, error)
	DeleteInstance(ctx context.Context, obj *ServiceInstance) error
	GetInstance(ctx context.Context, obj *ServiceInstance) (*ServiceInstance, error)
	ListInstances(ctx context.Context, obj *ServiceInstance) ([]*ServiceInstance, error)
	ResizeInstance(ctx context.Context, obj *ServiceInstance) (*ServiceInstance, error)
	GetBackup(ctx context.Context, backupUri string) (*Backup, error)
	CreateBackup(ctx context.Context, backupInfo *BackupInfo) (*filev1beta1.Backup, error)
	DeleteBackup(ctx context.Context, backupId string) error
	HasOperations(ctx context.Context, obj *ServiceInstance, operationType string, done bool) (bool, error)
	// Multishare ops
	GetMultishareInstance(ctx context.Context, obj *MultishareInstance) (*MultishareInstance, error)
	ListMultishareInstances(ctx context.Context, filter *ListFilter) ([]*MultishareInstance, error)
	StartCreateMultishareInstanceOp(ctx context.Context, obj *MultishareInstance) (*filev1beta1multishare.Operation, error)
	StartDeleteMultishareInstanceOp(ctx context.Context, obj *MultishareInstance) (*filev1beta1multishare.Operation, error)
	StartResizeMultishareInstanceOp(ctx context.Context, obj *MultishareInstance) (*filev1beta1multishare.Operation, error)
	ListShares(ctx context.Context, filter *ListFilter) ([]*Share, error)
	GetShare(ctx context.Context, obj *Share) (*Share, error)
	StartCreateShareOp(ctx context.Context, obj *Share) (*filev1beta1multishare.Operation, error)
	StartDeleteShareOp(ctx context.Context, obj *Share) (*filev1beta1multishare.Operation, error)
	StartResizeShareOp(ctx context.Context, obj *Share) (*filev1beta1multishare.Operation, error)
	WaitForOpWithOpts(ctx context.Context, op string, opts PollOpts) error
	GetOp(ctx context.Context, op string) (*filev1beta1multishare.Operation, error)
	IsOpDone(op *filev1beta1multishare.Operation) (bool, error)
	ListOps(ctx context.Context, resource *ListFilter) ([]*filev1beta1multishare.Operation, error)
}

type gcfsServiceManager struct {
	fileService       *filev1beta1.Service
	instancesService  *filev1beta1.ProjectsLocationsInstancesService
	operationsService *filev1beta1.ProjectsLocationsOperationsService
	backupService     *filev1beta1.ProjectsLocationsBackupsService

	// multishare definitions
	fileMultishareService            *filev1beta1multishare.Service
	multishareInstancesService       *filev1beta1multishare.ProjectsLocationsInstancesService
	multishareInstancesSharesService *filev1beta1multishare.ProjectsLocationsInstancesSharesService
	multishareOperationsServices     *filev1beta1multishare.ProjectsLocationsOperationsService
}

const (
	locationURIFmt  = "projects/%s/locations/%s"
	instanceURIFmt  = locationURIFmt + "/instances/%s"
	operationURIFmt = locationURIFmt + "/operations/%s"
	backupURIFmt    = locationURIFmt + "/backups/%s"
	shareSuffixFmt  = "/shares/%s"
	shareURIFmt     = instanceURIFmt + shareSuffixFmt
	// Patch update masks
	fileShareUpdateMask          = "file_shares"
	multishareCapacityUpdateMask = "capacity_gb"
	prodBasePath                 = "https://file.googleapis.com/"
)

var _ Service = &gcfsServiceManager{}

var (
	instanceUriRegex = regexp.MustCompile(`^projects/([^/]+)/locations/([^/]+)/instances/([^/]+)$`)
	shareUriRegex    = regexp.MustCompile(`^projects/([^/]+)/locations/([^/]+)/instances/([^/]+)/shares/([^/]+)$`)
)

// userErrorCodeMap tells how API error types are translated to error codes.
var userErrorCodeMap = map[int]codes.Code{
	http.StatusForbidden:       codes.PermissionDenied,
	http.StatusBadRequest:      codes.InvalidArgument,
	http.StatusTooManyRequests: codes.ResourceExhausted,
	http.StatusNotFound:        codes.NotFound,
}

func NewGCFSService(version string, client *http.Client, primaryFilestoreServiceEndpoint, testFilestoreServiceEndpoint string) (Service, error) {
	ctx := context.Background()

	fsOpts := []option.ClientOption{
		option.WithHTTPClient(client),
		option.WithUserAgent(fmt.Sprintf("Google Cloud Filestore CSI Driver/%s (%s %s)", version, runtime.GOOS, runtime.GOARCH)),
	}

	if primaryFilestoreServiceEndpoint != "" {
		fsOpts = append(fsOpts, option.WithEndpoint(primaryFilestoreServiceEndpoint))
	} else if testFilestoreServiceEndpoint != "" {
		endpoint, err := createFilestoreEndpointUrlBasePath(testFilestoreServiceEndpoint)
		if err != nil {
			return nil, err
		}
		fsOpts = append(fsOpts, option.WithEndpoint(endpoint))
	}

	fileService, err := filev1beta1.NewService(ctx, fsOpts...)
	if err != nil {
		return nil, err
	}

	klog.Infof("Using endpoint %q for non-multishare filestore", fileService.BasePath)

	fileMultishareService, err := filev1beta1multishare.NewService(ctx, fsOpts...)
	if err != nil {
		return nil, err
	}

	klog.Infof("Using endpoint %q for multishare filestore", fileMultishareService.BasePath)

	return &gcfsServiceManager{
		fileService:                      fileService,
		instancesService:                 filev1beta1.NewProjectsLocationsInstancesService(fileService),
		operationsService:                filev1beta1.NewProjectsLocationsOperationsService(fileService),
		backupService:                    filev1beta1.NewProjectsLocationsBackupsService(fileService),
		fileMultishareService:            fileMultishareService,
		multishareInstancesService:       filev1beta1multishare.NewProjectsLocationsInstancesService(fileMultishareService),
		multishareInstancesSharesService: filev1beta1multishare.NewProjectsLocationsInstancesSharesService(fileMultishareService),
		multishareOperationsServices:     filev1beta1multishare.NewProjectsLocationsOperationsService(fileMultishareService),
	}, nil
}

func (manager *gcfsServiceManager) CreateInstance(ctx context.Context, obj *ServiceInstance) (*ServiceInstance, error) {
	instance := &filev1beta1.Instance{
		Tier: obj.Tier,
		FileShares: []*filev1beta1.FileShareConfig{
			{
				Name:             obj.Volume.Name,
				CapacityGb:       util.RoundBytesToGb(obj.Volume.SizeBytes),
				SourceBackup:     obj.BackupSource,
				NfsExportOptions: extractNfsShareExportOptions(obj.NfsExportOptions),
			},
		},
		Networks: []*filev1beta1.NetworkConfig{
			{
				Network:         obj.Network.Name,
				Modes:           []string{"MODE_IPV4"},
				ReservedIpRange: obj.Network.ReservedIpRange,
				ConnectMode:     obj.Network.ConnectMode,
			},
		},
		KmsKeyName: obj.KmsKeyName,
		Labels:     obj.Labels,
		State:      obj.State,
		Protocol:   obj.Protocol,
	}

	klog.V(4).Infof("Creating instance %q: location %v, tier %q, capacity %v, network %q, ipRange %q, connectMode %q, KmsKeyName %q, labels %v, backup source %q, protocol %v",
		obj.Name,
		obj.Location,
		instance.Tier,
		instance.FileShares[0].CapacityGb,
		instance.Networks[0].Network,
		instance.Networks[0].ReservedIpRange,
		instance.Networks[0].ConnectMode,
		instance.KmsKeyName,
		instance.Labels,
		instance.FileShares[0].SourceBackup,
		obj.Protocol)
	op, err := manager.instancesService.Create(locationURI(obj.Project, obj.Location), instance).InstanceId(obj.Name).Context(ctx).Do()
	if err != nil {
		klog.Errorf("CreateInstance operation failed for instance %v: %v", obj.Name, err)
		return nil, err
	}

	klog.V(4).Infof("For instance %v, waiting for create instance op %v to complete", obj.Name, op.Name)
	err = manager.waitForOp(ctx, op)
	if err != nil {
		klog.Errorf("WaitFor CreateInstance op %s failed: %v", op.Name, err)
		return nil, common.NewTemporaryError(codes.Unavailable, fmt.Errorf("unknown error when polling the operation: %w", err))
	}
	serviceInstance, err := manager.GetInstance(ctx, obj)
	if err != nil {
		klog.Errorf("failed to get instance after creation: %v", err)
		return nil, err
	}
	return serviceInstance, nil
}

func (manager *gcfsServiceManager) GetInstance(ctx context.Context, obj *ServiceInstance) (*ServiceInstance, error) {
	instanceUri := instanceURI(obj.Project, obj.Location, obj.Name)
	instance, err := manager.instancesService.Get(instanceUri).Context(ctx).Do()
	if err != nil {
		klog.Errorf("Failed to get instance %v", instanceUri)
		if IsNotFoundErr(err) {
			return nil, err
		}
		return nil, common.NewTemporaryError(codes.Unavailable, err)
	}

	if instance != nil {
		klog.V(4).Infof("GetInstance call fetched instance %+v", instance)
		return cloudInstanceToServiceInstance(instance)
	}
	return nil, common.NewTemporaryError(codes.Unavailable, fmt.Errorf("failed to get instance %v", instanceUri))
}

func cloudInstanceToServiceInstance(instance *filev1beta1.Instance) (*ServiceInstance, error) {
	project, location, name, err := GetInstanceNameFromURI(instance.Name)
	if err != nil {
		return nil, err
	}
	ip := ""
	if len(instance.Networks[0].IpAddresses) > 0 {
		ip = instance.Networks[0].IpAddresses[0]
	}
	return &ServiceInstance{
		Project:  project,
		Location: location,
		Name:     name,
		Tier:     instance.Tier,
		Volume: Volume{
			Name:      instance.FileShares[0].Name,
			SizeBytes: util.GbToBytes(instance.FileShares[0].CapacityGb),
		},
		Network: Network{
			Name:            instance.Networks[0].Network,
			Ip:              ip,
			ReservedIpRange: instance.Networks[0].ReservedIpRange,
			ConnectMode:     instance.Networks[0].ConnectMode,
		},
		KmsKeyName:   instance.KmsKeyName,
		Labels:       instance.Labels,
		State:        instance.State,
		BackupSource: instance.FileShares[0].SourceBackup,
		Protocol:     instance.Protocol,
	}, nil
}

func CompareInstances(a, b *ServiceInstance) error {
	mismatches := []string{}
	if !strings.EqualFold(a.Tier, b.Tier) {
		mismatches = append(mismatches, "tier")
	}
	if a.Volume.Name != b.Volume.Name {
		mismatches = append(mismatches, "volume name")
	}
	if util.RoundBytesToGb(a.Volume.SizeBytes) != util.RoundBytesToGb(b.Volume.SizeBytes) {
		mismatches = append(mismatches, "volume size")
	}
	if a.Network.Name != b.Network.Name {
		mismatches = append(mismatches, "network name")
	}
	// Filestore API does not include key version info in the Instance object, simple string comparison will work
	if a.KmsKeyName != b.KmsKeyName {
		mismatches = append(mismatches, "kms key name")
	}

	if len(mismatches) > 0 {
		return fmt.Errorf("instance %v already exists but doesn't match expected: %+v", a.Name, mismatches)
	}
	return nil
}

func (manager *gcfsServiceManager) DeleteInstance(ctx context.Context, obj *ServiceInstance) error {
	uri := instanceURI(obj.Project, obj.Location, obj.Name)
	klog.V(4).Infof("Starting DeleteInstance cloud operation for instance %s", uri)
	op, err := manager.instancesService.Delete(uri).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("DeleteInstance operation failed: %w", err)
	}

	klog.V(4).Infof("For instance %s, waiting for delete op %v to complete", uri, op.Name)
	err = manager.waitForOp(ctx, op)
	if err != nil {
		return fmt.Errorf("WaitFor DeleteInstance op %s failed: %w", op.Name, err)
	}

	instance, err := manager.GetInstance(ctx, obj)
	if err != nil && !IsNotFoundErr(err) {
		return fmt.Errorf("failed to get instance after deletion: %w", err)
	}
	if instance != nil {
		return fmt.Errorf("instance %s still exists after delete operation in state %v", uri, instance.State)
	}

	klog.Infof("Instance %s has been deleted", uri)
	return nil
}

// ListInstances returns a list of active instances in a project at a specific location
func (manager *gcfsServiceManager) ListInstances(ctx context.Context, obj *ServiceInstance) ([]*ServiceInstance, error) {
	// Calling cloud provider service to get list of active instances. - indicates we are looking for instances in all the locations for a project
	lCall := manager.instancesService.List(locationURI(obj.Project, "-")).Context(ctx)
	nextPageToken := "pageToken"
	var activeInstances []*ServiceInstance

	for nextPageToken != "" {
		instances, err := lCall.Do()
		if err != nil {
			return nil, err
		}

		for _, activeInstance := range instances.Instances {
			if len(activeInstance.FileShares) == 0 {
				// skip multi-share instances
				continue
			}
			serviceInstance, err := cloudInstanceToServiceInstance(activeInstance)
			if err != nil {
				return nil, err
			}
			activeInstances = append(activeInstances, serviceInstance)
		}

		nextPageToken = instances.NextPageToken
		lCall.PageToken(nextPageToken)
	}
	return activeInstances, nil
}

func (manager *gcfsServiceManager) ResizeInstance(ctx context.Context, obj *ServiceInstance) (*ServiceInstance, error) {
	instanceuri := instanceURI(obj.Project, obj.Location, obj.Name)
	// Create a file instance for the Patch request.
	betaObj := &filev1beta1.Instance{
		Tier: obj.Tier,
		FileShares: []*filev1beta1.FileShareConfig{
			{
				Name: obj.Volume.Name,
				// This is the updated instance size requested.
				CapacityGb: util.BytesToGb(obj.Volume.SizeBytes),
			},
		},
		Networks: []*filev1beta1.NetworkConfig{
			{
				Network:         obj.Network.Name,
				Modes:           []string{"MODE_IPV4"},
				ReservedIpRange: obj.Network.ReservedIpRange,
				ConnectMode:     obj.Network.ConnectMode,
			},
		},
		KmsKeyName: obj.KmsKeyName,
	}

	klog.V(4).Infof("Patching instance %q: location %q, tier %q, capacity %v, network %q, ipRange %q, connectMode %q, KmsKeyName %q",
		obj.Name,
		obj.Location,
		betaObj.Tier,
		betaObj.FileShares[0].CapacityGb,
		betaObj.Networks[0].Network,
		betaObj.Networks[0].ReservedIpRange,
		betaObj.Networks[0].ConnectMode,
		betaObj.KmsKeyName,
	)
	op, err := manager.instancesService.Patch(instanceuri, betaObj).UpdateMask(fileShareUpdateMask).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("patch operation failed: %w", err)
	}

	klog.V(4).Infof("For instance %s, waiting for patch op %v to complete", instanceuri, op.Name)
	err = manager.waitForOp(ctx, op)
	if err != nil {
		return nil, fmt.Errorf("WaitFor patch op %s failed: %w", op.Name, err)
	}

	instance, err := manager.GetInstance(ctx, obj)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance after creation: %w", err)
	}
	klog.V(4).Infof("After resize got instance %#v", instance)
	return instance, nil
}

func (manager *gcfsServiceManager) GetBackup(ctx context.Context, backupUri string) (*Backup, error) {
	backup, err := manager.backupService.Get(backupUri).Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	return &Backup{
		Backup:            backup,
		SourceInstance:    backup.SourceInstance,
		SourceShare:       backup.SourceFileShare,
		FileSystemProtocl: backup.FileSystemProtocol,
	}, nil
}

func (manager *gcfsServiceManager) CreateBackup(ctx context.Context, backupInfo *BackupInfo) (*filev1beta1.Backup, error) {

	backupobj := &filev1beta1.Backup{
		SourceInstance:  backupInfo.BackupSource(),
		SourceFileShare: backupInfo.SourceShare,
		Labels:          backupInfo.Labels,
	}
	klog.V(4).Infof("Creating backup object %+v for the URI %v", *backupobj, backupInfo.BackupURI)
	opbackup, err := manager.backupService.Create(locationURI(backupInfo.Project, backupInfo.Location), backupobj).BackupId(backupInfo.Name).Context(ctx).Do()

	if err != nil {
		klog.Errorf("Create Backup operation failed: %v", err)
		return nil, err
	}

	klog.V(4).Infof("For backup uri %s, waiting for backup op %v to complete", backupInfo.BackupURI, opbackup.Name)
	err = manager.waitForOp(ctx, opbackup)
	if err != nil {
		return nil, fmt.Errorf("WaitFor CreateBackup op %s for source instance %v, backup uri: %v, operation failed: %w", opbackup.Name, backupInfo.BackupSource(), backupInfo.BackupURI, err)
	}

	backupObj, err := manager.backupService.Get(backupInfo.BackupURI).Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	if backupObj.State != "READY" {
		return nil, fmt.Errorf("backup %v for source %v is not ready, current state: %v", backupInfo.BackupURI, backupInfo.BackupSource(), backupObj.State)
	}
	klog.Infof("Successfully created backup %+v for source instance %v", backupObj, backupInfo.BackupSource())
	return backupObj, nil
}

func (manager *gcfsServiceManager) DeleteBackup(ctx context.Context, backupId string) error {
	opbackup, err := manager.backupService.Delete(backupId).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("for backup Id %s, delete backup operation %s failed: %w", backupId, opbackup.Name, err)
	}

	klog.V(4).Infof("For backup Id %s, waiting for backup op %v to complete", backupId, opbackup.Name)
	err = manager.waitForOp(ctx, opbackup)
	if err != nil {
		return fmt.Errorf("delete backup: %v, op %s failed: %w", backupId, opbackup.Name, err)
	}

	klog.Infof("Backup %v successfully deleted", backupId)
	return nil
}

func (manager *gcfsServiceManager) waitForOp(ctx context.Context, op *filev1beta1.Operation) error {
	return wait.Poll(5*time.Second, 5*time.Minute, func() (bool, error) {
		pollOp, err := manager.operationsService.Get(op.Name).Context(ctx).Do()
		if err != nil {
			return false, err
		}
		return isOpDone(pollOp)
	})
}

// TODO: unify this function behavior with IsOpDone
func isOpDone(op *filev1beta1.Operation) (bool, error) {
	if op == nil {
		return false, nil
	}
	if op.Error != nil {
		return true, fmt.Errorf("operation %v failed (%v): %v", op.Name, op.Error.Code, op.Error.Message)
	}
	return op.Done, nil
}

func (manager *gcfsServiceManager) IsOpDone(op *filev1beta1multishare.Operation) (bool, error) {
	// TODO: verify this behavior with filestore
	if op == nil {
		return true, nil
	}
	if op.Error != nil {
		return true, fmt.Errorf("operation %v failed (%v): %v", op.Name, op.Error.Code, op.Error.Message)
	}
	return op.Done, nil
}

func locationURI(project, location string) string {
	return fmt.Sprintf(locationURIFmt, project, location)
}

func instanceURI(project, location, name string) string {
	return fmt.Sprintf(instanceURIFmt, project, location, name)
}

func backupURI(project, location, name string) string {
	return fmt.Sprintf(backupURIFmt, project, location, name)
}

func GetInstanceNameFromURI(uri string) (project, location, name string, err error) {
	var uriRegex = regexp.MustCompile(`^projects/([^/]+)/locations/([^/]+)/instances/([^/]+)$`)

	substrings := uriRegex.FindStringSubmatch(uri)
	if substrings == nil {
		err = fmt.Errorf("failed to parse uri %v", uri)
		return
	}
	return substrings[1], substrings[2], substrings[3], nil
}

func IsNotFoundErr(err error) bool {
	var apiErr *googleapi.Error
	if !errors.As(err, &apiErr) {
		return false
	}

	for _, e := range apiErr.Errors {
		if e.Reason == "notFound" {
			return true
		}
	}
	return false
}

// isUserError returns a pointer to the grpc error code that maps to the http
// error code for the passed in user googleapi error. Returns nil if the
// given error is not a googleapi error caused by the user. The following
// http error codes are considered user errors:
// (1) http 400 Bad Request, returns grpc InvalidArgument,
// (2) http 403 Forbidden, returns grpc PermissionDenied,
// (3) http 404 Not Found, returns grpc NotFound
// (4) http 429 Too Many Requests, returns grpc ResourceExhausted
func isUserOperationError(err error) *codes.Code {
	// Upwrap the error
	var apiErr *googleapi.Error
	if !errors.As(err, &apiErr) {
		// Fallback to check for expected error code in the error string
		return containsUserErrStr(err)
	}

	return nil
}

func containsUserErrStr(err error) *codes.Code {
	if err == nil {
		return nil
	}

	// Error string picked up from https://cloud.google.com/apis/design/errors#handling_errors
	if strings.Contains(err.Error(), "PERMISSION_DENIED") {
		return util.ErrCodePtr(codes.PermissionDenied)
	}
	if strings.Contains(err.Error(), "RESOURCE_EXHAUSTED") {
		return util.ErrCodePtr(codes.ResourceExhausted)
	}
	if strings.Contains(err.Error(), "INVALID_ARGUMENT") {
		return util.ErrCodePtr(codes.InvalidArgument)
	}
	if strings.Contains(err.Error(), "NOT_FOUND") {
		return util.ErrCodePtr(codes.NotFound)
	}
	return nil
}

// isFilestoreLimitError returns a pointer to the grpc error code
// ResourceExhausted if the passed in error contains the
// "System limit for internal resources has been reached" string.
func isFilestoreLimitError(err error) *codes.Code {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "System limit for internal resources has been reached") {
		return util.ErrCodePtr(codes.ResourceExhausted)
	}
	return nil
}

// isContextError returns a pointer to the grpc error code DeadlineExceeded
// if the passed in error contains the "context deadline exceeded" string and returns
// the grpc error code Canceled if the error contains the "context canceled" string.
func isContextError(err error) *codes.Code {
	if err == nil {
		return nil
	}

	errStr := err.Error()
	if strings.Contains(errStr, context.DeadlineExceeded.Error()) {
		return util.ErrCodePtr(codes.DeadlineExceeded)
	}
	if strings.Contains(errStr, context.Canceled.Error()) {
		return util.ErrCodePtr(codes.Canceled)
	}
	return nil
}

// existingErrorCode returns a pointer to the grpc error code for the passed in error.
// Returns nil if the error is nil, or if the error cannot be converted to a grpc status.
// Since github.com/googleapis/gax-go/v2/apierror now wraps googleapi errors (returned from
// GCE API calls), and sets their status error code to Unknown, we now have to make sure we
// only return existing error codes from errors that do not wrap googleAPI errors. Otherwise,
// we will return Unknown for all GCE API calls that return googleapi errors.
func existingErrorCode(err error) *codes.Code {
	if err == nil {
		return nil
	}

	var te *common.TemporaryError
	// explicitly check if the error type is a `common.TemporaryError`.
	if errors.As(err, &te) {
		if status, ok := status.FromError(err); ok {
			return util.ErrCodePtr(status.Code())
		}
	}
	// We want to make sure we catch other error types that are statusable.
	// (eg. grpc-go/internal/status/status.go Error struct that wraps a status)
	var googleErr *googleapi.Error
	if !errors.As(err, &googleErr) {
		if status, ok := status.FromError(err); ok {
			return util.ErrCodePtr(status.Code())
		}
	}
	return nil
}

// isGoogleAPIError returns the gRPC status code for the given googleapi error by mapping
// the googleapi error's HTTP code to the corresponding gRPC error code. If the error is
// wrapped in an APIError (github.com/googleapis/gax-go/v2/apierror), it maps the wrapped
// googleAPI error's HTTP code to the corresponding gRPC error code. Returns an error if
// the given error is not a googleapi error.
func isGoogleAPIError(err error) *codes.Code {
	var googleErr *googleapi.Error
	if !errors.As(err, &googleErr) {
		return nil
	}
	var sourceCode int
	var apiErr *apierror.APIError
	if errors.As(err, &apiErr) {
		// When googleapi.Err is used as a wrapper, we return the error code of the wrapped contents.
		sourceCode = apiErr.HTTPCode()
	} else {
		// Rely on error code in googleapi.Err when it is our primary error.
		sourceCode = googleErr.Code
	}
	// Map API error code to user error code.
	if code, ok := userErrorCodeMap[sourceCode]; ok {
		return util.ErrCodePtr(code)
	}
	// Map API error code to user error code.
	return nil
}

// codeForError returns a pointer to the grpc error code that maps to the http
// error code for the passed in user googleapi error or context error. Returns
// codes.Internal if the given error is not a googleapi error caused by the user.
// The following http error codes are considered user errors:
// (1) http 400 Bad Request, returns grpc InvalidArgument,
// (2) http 403 Forbidden, returns grpc PermissionDenied,
// (3) http 404 Not Found, returns grpc NotFound
// (4) http 429 Too Many Requests, returns grpc ResourceExhausted
// The following errors are considered context errors:
// (1) "context deadline exceeded", returns grpc DeadlineExceeded,
// (2) "context canceled", returns grpc Canceled
func codeForError(err error) *codes.Code {
	if err == nil {
		return nil
	}
	if errCode := existingErrorCode(err); errCode != nil {
		return errCode
	}
	if errCode := isUserOperationError(err); errCode != nil {
		return errCode
	}
	if errCode := isContextError(err); errCode != nil {
		return errCode
	}
	if errCode := isFilestoreLimitError(err); errCode != nil {
		return errCode
	}
	if errCode := isGoogleAPIError(err); errCode != nil {
		return errCode
	}

	return util.ErrCodePtr(codes.Internal)
}

// Status error returns the error as a grpc status error, and
// sets the grpc error code according to CodeForError.
func StatusError(err error) error {
	if err == nil {
		return nil
	}
	return status.Error(*codeForError(err), err.Error())
}

// This function will process an existing backup
func ProcessExistingBackup(ctx context.Context, backup *Backup, volumeID string, mode string) (*csi.Snapshot, error) {
	backupSourceCSIHandle, err := util.BackupVolumeSourceToCSIVolumeHandle(mode, backup.SourceInstance, backup.SourceShare)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Cannot determine volume handle from back source instance %s, share %s", backup.SourceInstance, backup.SourceShare)
	}
	if backupSourceCSIHandle != volumeID {
		return nil, status.Errorf(codes.AlreadyExists, "Backup already exists with a different source volume %s, input source volume %s", backupSourceCSIHandle, volumeID)
	}
	// Check if backup is in the process of getting created.
	if backup.Backup.State == "CREATING" || backup.Backup.State == "FINALIZING" {
		return nil, status.Errorf(codes.DeadlineExceeded, "Backup %v not yet ready, current state %s", backup.Backup.Name, backup.Backup.State)
	}
	if backup.Backup.State != "READY" {
		return nil, status.Errorf(codes.Internal, "Backup %v not yet ready, current state %s", backup.Backup.Name, backup.Backup.State)
	}
	tp, err := util.ParseTimestamp(backup.Backup.CreateTime)
	if err != nil {
		err = fmt.Errorf("failed to parse create timestamp for backup %v: %w", backup.Backup.Name, err)
		return nil, StatusError(err)
	}
	klog.V(4).Infof("CreateSnapshot success for volume %v, Backup Id: %v", volumeID, backup.Backup.Name)
	return &csi.Snapshot{
		SizeBytes:      util.GbToBytes(backup.Backup.CapacityGb),
		SnapshotId:     backup.Backup.Name,
		SourceVolumeId: volumeID,
		CreationTime:   tp,
		ReadyToUse:     true,
	}, nil
}

func CheckBackupExists(backupInfo *Backup, err error) (bool, error) {
	if err != nil {
		if !IsNotFoundErr(err) {
			return false, StatusError(err)
		} else {
			//no backup exists
			return false, nil
		}
	}
	//process existing backup
	return true, nil
}

// This function returns the backup URI, the region that was picked to be the backup resource location and error.
func CreateBackupURI(serviceLocation, project, backupName, backupLocation string) (string, string, error) {
	region, err := deduceRegion(serviceLocation, backupLocation)
	if err != nil {
		return "", "", err
	}

	if !hasRegionPattern(region) {
		return "", "", fmt.Errorf("provided location did not match region pattern: %s", backupLocation)
	}
	return backupURI(project, region, backupName), region, nil
}

// deduceRegion will either return the provided backupLocation region or deduce
// from the ServiceInstance
func deduceRegion(serviceLocation, backupLocation string) (string, error) {
	region := backupLocation
	if region == "" {
		if hasRegionPattern(serviceLocation) {
			region = serviceLocation
		} else {
			var err error
			region, err = util.GetRegionFromZone(serviceLocation)
			if err != nil {
				return "", err
			}
		}
	}
	return region, nil
}

// hasRegionPattern returns true if the give location matches the standard
// region pattern. This expects regions to look like multiregion-regionsuffix.
// Example: us-central1
func hasRegionPattern(location string) bool {
	regionPattern := regexp.MustCompile("^[^-]+-[^-]+$")
	return regionPattern.MatchString(location)
}

func (manager *gcfsServiceManager) HasOperations(ctx context.Context, obj *ServiceInstance, operationType string, done bool) (bool, error) {
	uri := instanceURI(obj.Project, obj.Location, obj.Name)
	var totalFilteredOps []*filev1beta1.Operation
	var nextToken string
	for {
		resp, err := manager.operationsService.List(locationURI(obj.Project, obj.Location)).PageToken(nextToken).Context(ctx).Do()
		if err != nil {
			return false, fmt.Errorf("list operations for instance %q, token %q failed: %w", uri, nextToken, err)
		}

		filteredOps, err := ApplyFilter(resp.Operations, uri, operationType, done)
		if err != nil {
			return false, err
		}

		totalFilteredOps = append(totalFilteredOps, filteredOps...)
		if resp.NextPageToken == "" {
			break
		}
		nextToken = resp.NextPageToken
	}

	return len(totalFilteredOps) > 0, nil
}

func ApplyFilter(ops []*filev1beta1.Operation, uri string, opType string, done bool) ([]*filev1beta1.Operation, error) {
	var res []*filev1beta1.Operation
	for _, op := range ops {
		var meta filev1beta1.OperationMetadata
		if op.Metadata == nil {
			continue
		}
		if err := json.Unmarshal(op.Metadata, &meta); err != nil {
			return nil, err
		}
		if meta.Target == uri && meta.Verb == opType && op.Done == done {
			klog.V(4).Infof("Operation %q match filter for target %q", op.Name, meta.Target)
			res = append(res, op)
		}
	}
	return res, nil
}

// Multishare functions defined here
func (manager *gcfsServiceManager) GetMultishareInstance(ctx context.Context, obj *MultishareInstance) (*MultishareInstance, error) {
	instanceUri := instanceURI(obj.Project, obj.Location, obj.Name)
	instance, err := manager.multishareInstancesService.Get(instanceUri).Context(ctx).Do()
	if err != nil {
		klog.Errorf("Failed to get instance %v", instanceUri)
		return nil, err
	}

	return cloudInstanceToMultishareInstance(instance)
}

func (manager *gcfsServiceManager) ListMultishareInstances(ctx context.Context, filter *ListFilter) ([]*MultishareInstance, error) {
	lCall := manager.multishareInstancesService.List(locationURI(filter.Project, filter.Location)).Context(ctx)
	nextPageToken := "pageToken"
	var activeInstances []*MultishareInstance

	for nextPageToken != "" {
		instances, err := lCall.Do()
		if err != nil {
			return nil, err
		}

		for _, activeInstance := range instances.Instances {
			if !activeInstance.MultiShareEnabled {
				continue
			}

			instance, err := cloudInstanceToMultishareInstance(activeInstance)
			if err != nil {
				return nil, err
			}
			activeInstances = append(activeInstances, instance)
		}

		nextPageToken = instances.NextPageToken
		lCall.PageToken(nextPageToken)
	}
	return activeInstances, nil
}

func (manager *gcfsServiceManager) StartCreateMultishareInstanceOp(ctx context.Context, instance *MultishareInstance) (*filev1beta1multishare.Operation, error) {
	targetinstance := &filev1beta1multishare.Instance{
		MultiShareEnabled: true,
		Tier:              instance.Tier,
		Networks: []*filev1beta1multishare.NetworkConfig{
			{
				Network:         instance.Network.Name,
				Modes:           []string{"MODE_IPV4"},
				ReservedIpRange: instance.Network.ReservedIpRange,
				ConnectMode:     instance.Network.ConnectMode,
			},
		},
		CapacityGb:    util.BytesToGb(instance.CapacityBytes),
		KmsKeyName:    instance.KmsKeyName,
		Labels:        instance.Labels,
		Description:   instance.Description,
		MaxShareCount: int64(instance.MaxShareCount),
		Protocol:      instance.Protocol,
	}

	op, err := manager.multishareInstancesService.Create(locationURI(instance.Project, instance.Location), targetinstance).InstanceId(instance.Name).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("CreateInstance operation failed: %w", err)
	}
	klog.Infof("Started create instance op %s, for instance %q project %q, location %q, tier %q, capacity %v, network %q, ipRange %q, connectMode %q, KmsKeyName %q, labels %v, description %s, maxShareCount %d", op.Name, instance.Name, instance.Project, instance.Location, targetinstance.Tier, targetinstance.CapacityGb, targetinstance.Networks[0].Network, targetinstance.Networks[0].ReservedIpRange, targetinstance.Networks[0].ConnectMode, targetinstance.KmsKeyName, targetinstance.Labels, targetinstance.Description, targetinstance.MaxShareCount)
	return op, nil
}

func (manager *gcfsServiceManager) StartDeleteMultishareInstanceOp(ctx context.Context, instance *MultishareInstance) (*filev1beta1multishare.Operation, error) {
	uri := instanceURI(instance.Project, instance.Location, instance.Name)
	op, err := manager.multishareInstancesService.Delete(uri).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("DeleteInstance operation failed: %w", err)
	}
	klog.Infof("Started Delete Instance op %s for instance uri %s", op.Name, uri)
	return op, nil
}

func (manager *gcfsServiceManager) StartResizeMultishareInstanceOp(ctx context.Context, obj *MultishareInstance) (*filev1beta1multishare.Operation, error) {
	instanceuri := instanceURI(obj.Project, obj.Location, obj.Name)
	targetinstance := &filev1beta1multishare.Instance{
		MultiShareEnabled: true,
		Tier:              obj.Tier,
		CapacityGb:        util.BytesToGb(obj.CapacityBytes),
		KmsKeyName:        obj.KmsKeyName,
		Labels:            obj.Labels,
		Description:       obj.Description,
		Protocol:          obj.Protocol,
	}
	op, err := manager.multishareInstancesService.Patch(instanceuri, targetinstance).UpdateMask(multishareCapacityUpdateMask).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("patch operation failed: %w for instance %+v", err, targetinstance)
	}

	klog.Infof("Started instance update operation %s for instance %+v", op.Name, targetinstance)
	return op, nil
}

func (manager *gcfsServiceManager) StartCreateShareOp(ctx context.Context, share *Share) (*filev1beta1multishare.Operation, error) {
	instanceuri := instanceURI(share.Parent.Project, share.Parent.Location, share.Parent.Name)
	targetshare := &filev1beta1multishare.Share{
		CapacityGb:       util.BytesToGb(share.CapacityBytes),
		Labels:           share.Labels,
		MountName:        share.MountPointName,
		Backup:           share.BackupId,
		NfsExportOptions: extractNfsShareExportOptions(share.NfsExportOptions),
	}

	op, err := manager.multishareInstancesSharesService.Create(instanceuri, targetshare).ShareId(share.Name).Context(ctx).Do()
	if err != nil {
		return nil, common.NewTemporaryError(codes.Unavailable, fmt.Errorf("CreateShare operation failed: %w", err))
	}
	klog.Infof("Started Create Share op %s for share %q instance uri %q, with capacity(GB) %v, Labels %v", op.Name, share.Name, instanceuri, targetshare.CapacityGb, targetshare.Labels)
	return op, nil
}

func (manager *gcfsServiceManager) StartDeleteShareOp(ctx context.Context, share *Share) (*filev1beta1multishare.Operation, error) {
	uri := shareURI(share.Parent.Project, share.Parent.Location, share.Parent.Name, share.Name)
	op, err := manager.multishareInstancesSharesService.Delete(uri).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("DeleteShare operation failed: %w", err)
	}
	klog.Infof("Started Delete Share op %s for share uri %q ", op.Name, uri)
	return op, nil
}

func (manager *gcfsServiceManager) StartResizeShareOp(ctx context.Context, share *Share) (*filev1beta1multishare.Operation, error) {
	uri := shareURI(share.Parent.Project, share.Parent.Location, share.Parent.Name, share.Name)
	targetShare := &filev1beta1multishare.Share{
		CapacityGb: util.BytesToGb(share.CapacityBytes),
		Labels:     share.Labels,
		MountName:  share.MountPointName,
	}
	op, err := manager.multishareInstancesSharesService.Patch(uri, targetShare).UpdateMask(multishareCapacityUpdateMask).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("ResizeShare operation failed: %w", err)
	}
	klog.Infof("Started Resize Share op %s for share uri %q ", op.Name, uri)
	return op, nil
}

func (manager *gcfsServiceManager) WaitForOpWithOpts(ctx context.Context, op string, opts PollOpts) error {
	return wait.Poll(opts.Interval, opts.Timeout, func() (bool, error) {
		pollOp, err := manager.multishareOperationsServices.Get(op).Context(ctx).Do()
		if err != nil {
			return false, err
		}
		return manager.IsOpDone(pollOp)
	})
}

func (manager *gcfsServiceManager) GetOp(ctx context.Context, op string) (*filev1beta1multishare.Operation, error) {
	opInfo, err := manager.multishareOperationsServices.Get(op).Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	return opInfo, nil
}

func (manager *gcfsServiceManager) GetShare(ctx context.Context, obj *Share) (*Share, error) {
	sobj, err := manager.multishareInstancesSharesService.Get(shareURI(obj.Parent.Project, obj.Parent.Location, obj.Parent.Name, obj.Name)).Context(ctx).Do()
	if err != nil {
		return nil, common.NewTemporaryError(codes.Unavailable, err)
	}

	_, _, _, shareName, err := util.ParseShareURI(sobj.Name)
	if err != nil {
		return nil, err
	}
	instance, err := manager.GetMultishareInstance(ctx, obj.Parent)
	if err != nil {
		return nil, common.NewTemporaryError(codes.Unavailable, err)
	}

	return &Share{
		Name:           shareName,
		Parent:         instance,
		MountPointName: sobj.MountName,
		CapacityBytes:  sobj.CapacityGb * util.Gb,
		State:          sobj.State,
		Labels:         sobj.Labels,
	}, nil
}

func (manager *gcfsServiceManager) ListShares(ctx context.Context, filter *ListFilter) ([]*Share, error) {

	var shares []*Share

	instanceUri := instanceURI(filter.Project, filter.Location, filter.InstanceName)
	lCall := manager.multishareInstancesSharesService.List(instanceUri).Context(ctx)
	nextPageToken := "pageToken"

	for nextPageToken != "" {
		resp, err := lCall.Do()
		if err != nil {
			klog.Errorf("list share error: %v for parent uri %q", err, instanceUri)
			return nil, err
		}

		klog.V(6).Infof("List Share API call returned %d results in resp.Shares with unreachable %v", len(resp.Shares), resp.Unreachable)

		for _, sobj := range resp.Shares {
			project, location, instanceName, shareName, err := util.ParseShareURI(sobj.Name)
			if err != nil {
				klog.Errorf("Failed to parse share url :%s", sobj.Name)
				return nil, err
			}
			s := &Share{
				Name: shareName,
				Parent: &MultishareInstance{
					Name:     instanceName,
					Project:  project,
					Location: location,
				},
				MountPointName: sobj.MountName,
				CapacityBytes:  sobj.CapacityGb * util.Gb,
				Labels:         sobj.Labels,
				State:          sobj.State,
			}
			shares = append(shares, s)
		}
		nextPageToken = resp.NextPageToken
		lCall.PageToken(nextPageToken)
	}

	return shares, nil
}

func ParseShare(s *Share) (string, string, string, string, error) {
	if s == nil || s.Parent == nil {
		return "", "", "", "", fmt.Errorf("missing parent object for share %s", s.Name)
	}

	if s.Parent.Project == "" || s.Parent.Location == "" || s.Parent.Name == "" || s.Name == "" {
		return "", "", "", "", fmt.Errorf("missing identifier in share")
	}

	return s.Parent.Project, s.Parent.Location, s.Parent.Name, s.Name, nil
}

func GetMultishareInstanceHandle(m *MultishareInstance) (string, error) {
	if m == nil {
		return "", fmt.Errorf("empty multishare instance")
	}
	return fmt.Sprintf("%s/%s/%s", m.Project, m.Location, m.Name), nil
}

func CompareMultishareInstances(a, b *MultishareInstance) error {
	if a == nil || b == nil {
		return fmt.Errorf("empty instance object detected")
	}

	mismatches := []string{}
	if a.Project != b.Project {
		mismatches = append(mismatches, "project")
	}

	if a.Location != b.Location {
		mismatches = append(mismatches, "location")
	}

	if a.Name != b.Name {
		mismatches = append(mismatches, "name")
	}

	if !strings.EqualFold(a.Tier, b.Tier) {
		mismatches = append(mismatches, "tier")
	}

	if a.Network.Name != b.Network.Name {
		mismatches = append(mismatches, "network name")
	}

	// Filestore API does not include key version info in the Instance object, simple string comparison will work
	if a.KmsKeyName != b.KmsKeyName {
		mismatches = append(mismatches, "kms key name")
	}

	if len(mismatches) > 0 {
		return fmt.Errorf("instance %v already exists but doesn't match expected: %+v", a.Name, mismatches)
	}
	return nil
}

func CompareShares(a, b *Share) error {
	if a == nil || b == nil {
		return fmt.Errorf("empty share object detected")
	}

	mismatches := []string{}
	if a.Name != b.Name {
		mismatches = append(mismatches, "tier")
	}
	if util.RoundBytesToGb(a.CapacityBytes) != util.RoundBytesToGb(b.CapacityBytes) {
		mismatches = append(mismatches, "share size")
	}
	if len(mismatches) > 0 {
		return fmt.Errorf("share %v already exists but doesn't match expected: %+v", a.Name, mismatches)
	}
	if err := CompareMultishareInstances(a.Parent, b.Parent); err != nil {
		return err
	}
	return nil
}

func cloudInstanceToMultishareInstance(instance *filev1beta1multishare.Instance) (*MultishareInstance, error) {
	if instance == nil {
		return nil, fmt.Errorf("nil instance")
	}
	project, location, name, err := GetInstanceNameFromURI(instance.Name)
	if err != nil {
		return nil, err
	}
	ip := ""
	if len(instance.Networks[0].IpAddresses) > 0 {
		ip = instance.Networks[0].IpAddresses[0]
	}
	return &MultishareInstance{
		Project:  project,
		Location: location,
		Name:     name,
		Tier:     instance.Tier,
		Network: Network{
			Name:            instance.Networks[0].Network,
			Ip:              ip,
			ReservedIpRange: instance.Networks[0].ReservedIpRange,
			ConnectMode:     instance.Networks[0].ConnectMode,
		},
		KmsKeyName:         instance.KmsKeyName,
		Labels:             instance.Labels,
		State:              instance.State,
		CapacityBytes:      instance.CapacityGb * util.Gb,
		MaxCapacityBytes:   instance.MaxCapacityGb * util.Gb,
		CapacityStepSizeGb: instance.CapacityStepSizeGb,
		Description:        instance.Description,
		MaxShareCount:      int(instance.MaxShareCount),
		Protocol:           instance.Protocol,
	}, nil
}

func shareURI(project, location, instanceName, shareName string) string {
	return fmt.Sprintf(shareURIFmt, project, location, instanceName, shareName)
}

func createFilestoreEndpointUrlBasePath(endpoint string) (string, error) {
	if endpoint != "" {
		if !isValidEndpoint(endpoint) {
			return "", fmt.Errorf("invalid filestore endpoint %v", endpoint)
		}
		return "https://" + endpoint + "/", nil
	} else {
		return prodBasePath, nil
	}
}

func isValidEndpoint(endpoint string) bool {
	switch endpoint {
	case testEndpoint:
		return true
	case stagingEndpoint:
		return true
	case prodEndpoint:
		return true
	}

	return false
}

func (manager *gcfsServiceManager) ListOps(ctx context.Context, filter *ListFilter) ([]*filev1beta1multishare.Operation, error) {
	lCall := manager.multishareOperationsServices.List(locationURI(filter.Project, filter.Location)).Context(ctx)
	nextPageToken := "pageToken"
	var activeOperations []*filev1beta1multishare.Operation

	for nextPageToken != "" {
		operations, err := lCall.Do()
		if err != nil {
			return nil, err
		}

		activeOperations = append(activeOperations, operations.Operations...)

		nextPageToken = operations.NextPageToken
		lCall.PageToken(nextPageToken)
	}
	return activeOperations, nil
}

func IsInstanceTarget(target string) bool {
	return instanceUriRegex.MatchString(target)
}

func IsShareTarget(target string) bool {
	return shareUriRegex.MatchString(target)
}

func GenerateMultishareInstanceURI(m *MultishareInstance) (string, error) {
	if m == nil {
		return "", fmt.Errorf("nil instance")
	}

	if m.Project == "" || m.Location == "" || m.Name == "" {
		return "", fmt.Errorf("missing project, name or location in instance")
	}

	return instanceURI(m.Project, m.Location, m.Name), nil
}

func GenerateShareURI(s *Share) (string, error) {
	if s == nil || s.Parent == nil {
		return "", fmt.Errorf("missing share parent instance")
	}

	if s.Parent.Project == "" || s.Parent.Location == "" || s.Parent.Name == "" || s.Name == "" {
		return "", fmt.Errorf("missing parent, project, name or location in share parent")
	}

	return shareURI(s.Parent.Project, s.Parent.Location, s.Parent.Name, s.Name), nil
}

func isMultishareVolId(volId string) bool {
	return strings.Contains(volId, "modeMultishare")
}

func extractNfsShareExportOptions(options []*NfsExportOptions) []*filev1beta1multishare.NfsExportOptions {
	var filerOpts []*filev1beta1multishare.NfsExportOptions
	for _, opt := range options {
		filerOpts = append(filerOpts,
			&filev1beta1multishare.NfsExportOptions{
				AccessMode: opt.AccessMode,
				AnonGid:    opt.AnonGid,
				AnonUid:    opt.AnonUid,
				IpRanges:   opt.IpRanges,
				SquashMode: opt.SquashMode,
			})
	}
	return filerOpts
}
