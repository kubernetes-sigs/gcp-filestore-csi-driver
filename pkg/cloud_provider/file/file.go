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
	"fmt"
	"net/http"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/golang/glog"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/util"

	filev1beta1 "google.golang.org/api/file/v1beta1"
	filev1beta1multishare "google.golang.org/api/file/v1beta1multishare"
)

type PollOpts struct {
	Interval time.Duration
	Timeout  time.Duration
}

type Share struct {
	Name           string              // only the share name
	Parent         *MultishareInstance // parent captures the project, location details.
	State          string
	MountPointName string
	Labels         map[string]string
	CapacityBytes  int64
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
	Project    string
	Name       string
	Location   string
	Tier       string
	Network    Network
	Volume     Volume
	Labels     map[string]string
	State      string
	KmsKeyName string
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

type BackupInfo struct {
	Backup             *filev1beta1.Backup
	SourceVolumeHandle string
}

type Service interface {
	CreateInstance(ctx context.Context, obj *ServiceInstance) (*ServiceInstance, error)
	DeleteInstance(ctx context.Context, obj *ServiceInstance) error
	GetInstance(ctx context.Context, obj *ServiceInstance) (*ServiceInstance, error)
	ListInstances(ctx context.Context, obj *ServiceInstance) ([]*ServiceInstance, error)
	ResizeInstance(ctx context.Context, obj *ServiceInstance) (*ServiceInstance, error)
	GetBackup(ctx context.Context, backupUri string) (*BackupInfo, error)
	CreateBackup(ctx context.Context, obj *ServiceInstance, backupId, backupLocation string) (*filev1beta1.Backup, error)
	DeleteBackup(ctx context.Context, backupId string) error
	CreateInstanceFromBackupSource(ctx context.Context, obj *ServiceInstance, volumeSourceSnapshotId string) (*ServiceInstance, error)
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
	shareURIFmt     = instanceURIFmt + "/shares/%s"
	// Patch update masks
	fileShareUpdateMask          = "file_shares"
	multishareCapacityUpdateMask = "capacity_gb"
)

var _ Service = &gcfsServiceManager{}

var (
	instanceUriRegex = regexp.MustCompile(`^projects/([^/]+)/locations/([^/]+)/instances/([^/]+)$`)
	shareUriRegex    = regexp.MustCompile(`^projects/([^/]+)/locations/([^/]+)/instances/([^/]+)/shares/([^/]+)$`)
)

func NewGCFSService(version string, client *http.Client, endpoint string) (Service, error) {
	ctx := context.Background()
	fileService, err := filev1beta1.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}
	fileService.UserAgent = fmt.Sprintf("Google Cloud Filestore CSI Driver/%s (%s %s)", version, runtime.GOOS, runtime.GOARCH)

	basepath := createFilestoreEndpointUrlBasePath(endpoint)
	glog.Infof("Using endpoint %s for multshare", createFilestoreEndpointUrlBasePath(endpoint))
	fileMultishareService, err := filev1beta1multishare.NewService(ctx, basepath, option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}
	fileMultishareService.UserAgent = fmt.Sprintf("Google Cloud Filestore CSI Driver/%s (%s %s)", version, runtime.GOOS, runtime.GOARCH)

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
	betaObj := &filev1beta1.Instance{
		Tier: obj.Tier,
		FileShares: []*filev1beta1.FileShareConfig{
			{
				Name:       obj.Volume.Name,
				CapacityGb: util.RoundBytesToGb(obj.Volume.SizeBytes),
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
	}

	glog.V(4).Infof("Creating instance %q: location %q, tier %q, capacity %v, network %q, ipRange %q, connectMode %q, KmsKeyName %q, labels %v",
		obj.Name,
		obj.Location,
		betaObj.Tier,
		betaObj.FileShares[0].CapacityGb,
		betaObj.Networks[0].Network,
		betaObj.Networks[0].ReservedIpRange,
		betaObj.Networks[0].ConnectMode,
		betaObj.KmsKeyName,
		betaObj.Labels)
	op, err := manager.instancesService.Create(locationURI(obj.Project, obj.Location), betaObj).InstanceId(obj.Name).Context(ctx).Do()
	if err != nil {
		glog.Errorf("CreateInstance operation failed for instance %s: %v", obj.Name, err)
		return nil, err
	}

	glog.V(4).Infof("For instance %v, waiting for create instance op %v to complete", obj.Name, op.Name)
	err = manager.waitForOp(ctx, op)
	if err != nil {
		glog.Errorf("WaitFor CreateInstance op %s failed: %v", op.Name, err)
		return nil, err
	}
	instance, err := manager.GetInstance(ctx, obj)
	if err != nil {
		glog.Errorf("failed to get instance after creation: %v", err)
		return nil, err
	}
	return instance, nil
}

func (manager *gcfsServiceManager) CreateInstanceFromBackupSource(ctx context.Context, obj *ServiceInstance, sourceSnapshotId string) (*ServiceInstance, error) {
	instance := &filev1beta1.Instance{
		Tier: obj.Tier,
		FileShares: []*filev1beta1.FileShareConfig{
			{
				Name:         obj.Volume.Name,
				CapacityGb:   util.RoundBytesToGb(obj.Volume.SizeBytes),
				SourceBackup: sourceSnapshotId,
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
	}

	glog.V(4).Infof("Creating instance %q: location %v, tier %q, capacity %v, network %q, ipRange %q, connectMode %q, KmsKeyName %q, labels %v backup source %q",
		obj.Name,
		obj.Location,
		instance.Tier,
		instance.FileShares[0].CapacityGb,
		instance.Networks[0].Network,
		instance.Networks[0].ReservedIpRange,
		instance.Networks[0].ConnectMode,
		instance.KmsKeyName,
		instance.Labels,
		instance.FileShares[0].SourceBackup)
	op, err := manager.instancesService.Create(locationURI(obj.Project, obj.Location), instance).InstanceId(obj.Name).Context(ctx).Do()
	if err != nil {
		glog.Errorf("CreateInstance operation failed for instance %v: %v", obj.Name, err)
		return nil, err
	}

	glog.V(4).Infof("For instance %v, waiting for create instance op %v to complete", obj.Name, op.Name)
	err = manager.waitForOp(ctx, op)
	if err != nil {
		glog.Errorf("WaitFor CreateInstance op %s failed: %v", op.Name, err)
		return nil, err
	}
	serviceInstance, err := manager.GetInstance(ctx, obj)
	if err != nil {
		glog.Errorf("failed to get instance after creation: %v", err)
		return nil, err
	}
	return serviceInstance, nil
}

func (manager *gcfsServiceManager) GetInstance(ctx context.Context, obj *ServiceInstance) (*ServiceInstance, error) {
	instanceUri := instanceURI(obj.Project, obj.Location, obj.Name)
	instance, err := manager.instancesService.Get(instanceUri).Context(ctx).Do()
	if err != nil {
		glog.Errorf("Failed to get instance %v", instanceUri)
		return nil, err
	}

	if instance != nil {
		glog.V(4).Infof("GetInstance call fetched instance %+v", instance)
		return cloudInstanceToServiceInstance(instance)
	}
	return nil, fmt.Errorf("failed to get instance %v", instanceUri)
}

func cloudInstanceToServiceInstance(instance *filev1beta1.Instance) (*ServiceInstance, error) {
	project, location, name, err := getInstanceNameFromURI(instance.Name)
	if err != nil {
		return nil, err
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
			Ip:              instance.Networks[0].IpAddresses[0],
			ReservedIpRange: instance.Networks[0].ReservedIpRange,
			ConnectMode:     instance.Networks[0].ConnectMode,
		},
		KmsKeyName: instance.KmsKeyName,
		Labels:     instance.Labels,
		State:      instance.State,
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
	glog.V(4).Infof("Starting DeleteInstance cloud operation for instance %s", uri)
	op, err := manager.instancesService.Delete(uri).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("DeleteInstance operation failed: %v", err)
	}

	glog.V(4).Infof("For instance %s, waiting for delete op %v to complete", uri, op.Name)
	err = manager.waitForOp(ctx, op)
	if err != nil {
		return fmt.Errorf("WaitFor DeleteInstance op %s failed: %v", op.Name, err)
	}

	instance, err := manager.GetInstance(ctx, obj)
	if err != nil && !IsNotFoundErr(err) {
		return fmt.Errorf("failed to get instance after deletion: %v", err)
	}
	if instance != nil {
		return fmt.Errorf("instance %s still exists after delete operation in state %v", uri, instance.State)
	}

	glog.Infof("Instance %s has been deleted", uri)
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

	glog.V(4).Infof("Patching instance %q: location %q, tier %q, capacity %v, network %q, ipRange %q, connectMode %q, KmsKeyName %q",
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
		return nil, fmt.Errorf("patch operation failed: %v", err)
	}

	glog.V(4).Infof("For instance %s, waiting for patch op %v to complete", instanceuri, op.Name)
	err = manager.waitForOp(ctx, op)
	if err != nil {
		return nil, fmt.Errorf("WaitFor patch op %s failed: %v", op.Name, err)
	}

	instance, err := manager.GetInstance(ctx, obj)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance after creation: %v", err)
	}
	glog.V(4).Infof("After resize got instance %#v", instance)
	return instance, nil
}

func (manager *gcfsServiceManager) GetBackup(ctx context.Context, backupUri string) (*BackupInfo, error) {
	backup, err := manager.backupService.Get(backupUri).Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	return &BackupInfo{
		Backup:             backup,
		SourceVolumeHandle: backup.SourceInstance,
	}, nil
}

func (manager *gcfsServiceManager) CreateBackup(ctx context.Context, obj *ServiceInstance, backupName string, backupLocation string) (*filev1beta1.Backup, error) {
	backupUri, region, err := CreateBackpURI(obj, backupName, backupLocation)
	if err != nil {
		return nil, err
	}

	backupSource := fmt.Sprintf("projects/%s/locations/%s/instances/%s", obj.Project, obj.Location, obj.Name)
	backupobj := &filev1beta1.Backup{
		SourceInstance:  backupSource,
		SourceFileShare: obj.Volume.Name,
	}
	glog.V(4).Infof("Creating backup object %+v for the URI %v", *backupobj, backupUri)
	opbackup, err := manager.backupService.Create(locationURI(obj.Project, region), backupobj).BackupId(backupName).Context(ctx).Do()
	if err != nil {
		glog.Errorf("Create Backup operation failed: %v", err)
		return nil, err
	}

	glog.V(4).Infof("For backup uri %s, waiting for backup op %v to complete", backupUri, opbackup.Name)
	err = manager.waitForOp(ctx, opbackup)
	if err != nil {
		return nil, fmt.Errorf("WaitFor CreateBackup op %s for source instance %v, backup uri: %v, operation failed: %v", opbackup.Name, backupSource, backupUri, err)
	}

	backupObj, err := manager.backupService.Get(backupUri).Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	if backupObj.State != "READY" {
		return nil, fmt.Errorf("backup %v for source %v is not ready, current state: %v", backupUri, backupSource, backupObj.State)
	}
	glog.Infof("Successfully created backup %+v for source instance %v", backupObj, backupSource)
	return backupObj, nil
}

func (manager *gcfsServiceManager) DeleteBackup(ctx context.Context, backupId string) error {
	opbackup, err := manager.backupService.Delete(backupId).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("for backup Id %s, delete backup operation %s failed: %v", backupId, opbackup.Name, err)
	}

	glog.V(4).Infof("For backup Id %s, waiting for backup op %v to complete", backupId, opbackup.Name)
	err = manager.waitForOp(ctx, opbackup)
	if err != nil {
		return fmt.Errorf("delete backup: %v, op %s failed: %v", backupId, opbackup.Name, err)
	}

	glog.Infof("Backup %v successfully deleted", backupId)
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

func operationURI(project, location, name string) string {
	return fmt.Sprintf(operationURIFmt, project, location, name)
}

func backupURI(project, location, name string) string {
	return fmt.Sprintf(backupURIFmt, project, location, name)
}

func getInstanceNameFromURI(uri string) (project, location, name string, err error) {
	var uriRegex = regexp.MustCompile(`^projects/([^/]+)/locations/([^/]+)/instances/([^/]+)$`)

	substrings := uriRegex.FindStringSubmatch(uri)
	if substrings == nil {
		err = fmt.Errorf("failed to parse uri %v", uri)
		return
	}
	return substrings[1], substrings[2], substrings[3], nil
}

func IsNotFoundErr(err error) bool {
	apiErr, ok := err.(*googleapi.Error)
	if !ok {
		return false
	}

	for _, e := range apiErr.Errors {
		if e.Reason == "notFound" {
			return true
		}
	}
	return false
}

// IsUserError returns true if the error is a googleapi.Error with
// Error 403: Permission Denied, Error 400: Bad Request
func IsUserError(err error) bool {
	userErrors := map[int]bool{
		http.StatusForbidden:  true,
		http.StatusBadRequest: true,
	}
	if apiErr, ok := err.(*googleapi.Error); ok && userErrors[apiErr.Code] {
		return true
	}
	return false
}

// IsTooManyRequestError returns true if the error is a googleapi.Error with
// Error 429: Status Too Many Requests.
func IsTooManyRequestError(err error) bool {
	if apierr, ok := err.(*googleapi.Error); ok && apierr.Code == http.StatusTooManyRequests {
		return true
	}
	return false
}

// This function returns the backup URI, the region that was picked to be the backup resource location and error.
func CreateBackpURI(obj *ServiceInstance, backupName, backupLocation string) (string, string, error) {
	region := backupLocation
	if region == "" {
		var err error
		region, err = util.GetRegionFromZone(obj.Location)
		if err != nil {
			return "", "", err
		}
	}

	return backupURI(obj.Project, region, backupName), region, nil
}

func (manager *gcfsServiceManager) HasOperations(ctx context.Context, obj *ServiceInstance, operationType string, done bool) (bool, error) {
	uri := instanceURI(obj.Project, obj.Location, obj.Name)
	var totalFilteredOps []*filev1beta1.Operation
	var nextToken string
	for {
		resp, err := manager.operationsService.List(locationURI(obj.Project, obj.Location)).PageToken(nextToken).Context(ctx).Do()
		if err != nil {
			return false, fmt.Errorf("list operations for instance %q, token %q failed: %v", uri, nextToken, err)
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
			glog.V(4).Infof("Operation %q match filter for target %q", op.Name, meta.Target)
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
		glog.Errorf("Failed to get instance %v", instanceUri)
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
		CapacityGb:  util.BytesToGb(instance.CapacityBytes),
		KmsKeyName:  instance.KmsKeyName,
		Labels:      instance.Labels,
		Description: instance.Description,
	}

	op, err := manager.multishareInstancesService.Create(locationURI(instance.Project, instance.Location), targetinstance).InstanceId(instance.Name).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("CreateInstance operation failed: %v", err)
	}
	klog.Infof("Started create instance op %s, for instance %q project %q, location %q, tier %q, capacity %v, network %q, ipRange %q, connectMode %q, KmsKeyName %q, labels %v, description %s", op.Name, instance.Name, instance.Project, instance.Location, targetinstance.Tier, targetinstance.CapacityGb, targetinstance.Networks[0].Network, targetinstance.Networks[0].ReservedIpRange, targetinstance.Networks[0].ConnectMode, targetinstance.KmsKeyName, targetinstance.Labels, targetinstance.Description)
	return op, nil
}

func (manager *gcfsServiceManager) StartDeleteMultishareInstanceOp(ctx context.Context, instance *MultishareInstance) (*filev1beta1multishare.Operation, error) {
	uri := instanceURI(instance.Project, instance.Location, instance.Name)
	op, err := manager.multishareInstancesService.Delete(uri).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("DeleteInstance operation failed: %v", err)
	}
	klog.Infof("Started Delete Instance op %s for instance uri %s", op.Name, uri)
	return op, nil
}

func (manager *gcfsServiceManager) StartResizeMultishareInstanceOp(ctx context.Context, obj *MultishareInstance) (*filev1beta1multishare.Operation, error) {
	instanceuri := instanceURI(obj.Project, obj.Location, obj.Name)
	targetinstance := &filev1beta1multishare.Instance{
		MultiShareEnabled: true,
		Tier:              obj.Tier,
		Networks: []*filev1beta1multishare.NetworkConfig{
			{
				Network:         obj.Network.Name,
				Modes:           []string{"MODE_IPV4"},
				ReservedIpRange: obj.Network.ReservedIpRange,
				ConnectMode:     obj.Network.ConnectMode,
			},
		},
		CapacityGb:  util.BytesToGb(obj.CapacityBytes),
		KmsKeyName:  obj.KmsKeyName,
		Labels:      obj.Labels,
		Description: obj.Description,
	}
	op, err := manager.multishareInstancesService.Patch(instanceuri, targetinstance).UpdateMask(multishareCapacityUpdateMask).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("patch operation failed: %v for instance %+v", err, targetinstance)
	}

	klog.Infof("Started instance update operation %s for instance %+v", op.Name, targetinstance)
	return op, nil
}

func (manager *gcfsServiceManager) StartCreateShareOp(ctx context.Context, share *Share) (*filev1beta1multishare.Operation, error) {
	instanceuri := instanceURI(share.Parent.Project, share.Parent.Location, share.Parent.Name)
	targetshare := &filev1beta1multishare.Share{
		CapacityGb: util.BytesToGb(share.CapacityBytes),
		Labels:     share.Labels,
		MountName:  share.MountPointName,
	}

	op, err := manager.multishareInstancesSharesService.Create(instanceuri, targetshare).ShareId(share.Name).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("CreateShare operation failed: %v", err)
	}
	klog.Infof("Started Create Share op %s for share %q instance uri %q, with capacity(GB) %v, Labels %v", op.Name, share.Name, instanceuri, targetshare.CapacityGb, targetshare.Labels)
	return op, nil
}

func (manager *gcfsServiceManager) StartDeleteShareOp(ctx context.Context, share *Share) (*filev1beta1multishare.Operation, error) {
	uri := shareURI(share.Parent.Project, share.Parent.Location, share.Parent.Name, share.Name)
	op, err := manager.multishareInstancesSharesService.Delete(uri).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("DeleteShare operation failed: %v", err)
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
		return nil, fmt.Errorf("ResizeShare operation failed: %v", err)
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
		return nil, err
	}

	_, _, _, shareName, err := util.ParseShareURI(sobj.Name)
	if err != nil {
		return nil, err
	}
	instance, err := manager.GetMultishareInstance(ctx, obj.Parent)
	if err != nil {
		return nil, err
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
	instances := make([]*MultishareInstance, 0, 1)
	if filter.InstanceName != "-" {
		instances = append(instances, &MultishareInstance{Name: filter.InstanceName, Location: filter.Location})
	} else {
		existingInstances, err := manager.ListMultishareInstances(ctx, filter)
		if err != nil {
			return nil, err
		}
		instances = append(instances, existingInstances...)
	}

	var shares []*Share

	for _, instance := range instances {
		uri := instanceURI(filter.Project, instance.Location, instance.Name)
		lCall := manager.multishareInstancesSharesService.List(uri).Context(ctx)
		nextPageToken := "pageToken"

		for nextPageToken != "" {
			resp, err := lCall.Do()
			if err != nil {
				klog.Errorf("list share error: %v", err)
				return nil, err
			}

			for _, sobj := range resp.Shares {
				project, location, instanceName, shareName, err := util.ParseShareURI(sobj.Name)
				if err != nil {
					klog.Errorf("Failed to parse share url :%s", sobj.Name)
					continue
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
	project, location, name, err := getInstanceNameFromURI(instance.Name)
	if err != nil {
		return nil, err
	}
	return &MultishareInstance{
		Project:  project,
		Location: location,
		Name:     name,
		Tier:     instance.Tier,
		Network: Network{
			Name:            instance.Networks[0].Network,
			Ip:              instance.Networks[0].IpAddresses[0],
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
	}, nil
}

func shareURI(project, location, instanceName, shareName string) string {
	return fmt.Sprintf(shareURIFmt, project, location, instanceName, shareName)
}

func createFilestoreEndpointUrlBasePath(endpoint string) string {
	return "https://" + endpoint + "/"
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
		return "", fmt.Errorf("missing parent, project or location in instance")
	}

	return fmt.Sprintf(instanceURIFmt, m.Project, m.Location, m.Name), nil
}

func GenerateShareURI(s *Share) (string, error) {
	if s == nil || s.Parent == nil {
		return "", fmt.Errorf("missing share parent instance")
	}

	if s.Parent.Project == "" || s.Parent.Location == "" || s.Parent.Name == "" || s.Name == "" {
		return "", fmt.Errorf("missing parent, project or location in share parent")
	}

	return fmt.Sprintf(shareURIFmt, s.Parent.Project, s.Parent.Location, s.Parent.Name, s.Name), nil
}
