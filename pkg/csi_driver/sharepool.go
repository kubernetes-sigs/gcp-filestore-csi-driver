/*
Copyright 2026 The Kubernetes Authors.

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

package driver

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/file"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/util"
)

const (
	paramKeySharePool               = "share-pool"
	defaultSharePoolShareCapacityGb = 1
	sharePoolURLScheme              = "sharepool"
	sharePoolURLPrefix              = sharePoolURLScheme + "://"
)

func buildSharePoolVolumeID(parentPool, shareID, ipAddress string) string {
	// Scheme format: sharepool://{project}/{location}/{share_pool}/shares/{share_id}?ip={ip_address}
	// parentPool format: projects/{project}/locations/{location}/sharePools/{share_pool}

	hostPath := strings.Replace(parentPool, "projects/", sharePoolURLPrefix, 1)
	hostPath = strings.Replace(hostPath, "/locations/", "/", 1)
	hostPath = strings.Replace(hostPath, "/sharePools/", "/", 1)

	return fmt.Sprintf("%s/shares/%s?ip=%s", hostPath, shareID, ipAddress)
}

func parseSharePoolVolumeID(volumeID string) (parent, shareID, ipAddress string, err error) {
	parsedURL, err := url.Parse(volumeID)
	if err != nil {
		return "", "", "", err
	}

	if parsedURL.Scheme != sharePoolURLScheme {
		return "", "", "", fmt.Errorf("invalid volume ID scheme %q: expected %q", parsedURL.Scheme, sharePoolURLScheme)
	}

	// Extract project, location, pool name from Path segment
	parts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	if len(parts) < 4 || parts[2] != "shares" {
		return "", "", "", fmt.Errorf("invalid composite volume ID: %s", volumeID)
	}

	project := parsedURL.Host
	location := parts[0]
	poolName := parts[1]
	shareID = parts[3]

	parent = fmt.Sprintf("projects/%s/locations/%s/sharePools/%s", project, location, poolName)
	ipAddress = parsedURL.Query().Get("ip")

	return
}

func isSharePoolVolumeID(volumeID string) bool {
	return strings.HasPrefix(volumeID, sharePoolURLPrefix)
}

func (s *controllerServer) handleCreateSharePoolVolume(ctx context.Context, req *csi.CreateVolumeRequest, sharePoolPath string) (*csi.CreateVolumeResponse, error) {
	name := req.GetName()
	if len(name) == 0 {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume name must be provided")
	}

	if err := s.config.driver.validateVolumeCapabilities(req.GetVolumeCapabilities()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	capacityBytes := req.GetCapacityRange().GetRequiredBytes()
	capacityGb := util.RoundBytesToGb(capacityBytes)
	if capacityGb == 0 {
		capacityGb = defaultSharePoolShareCapacityGb // Default to defaultSharePoolShareCapacityGb if not specified (capacityGb == 0)
	}

	klog.V(4).Infof("Acquiring Share Pool volume: pool %q, capacity %d GiB, requestID %q", sharePoolPath, capacityGb, name)
	poolShare, err := s.config.fileService.AcquireShare(ctx, sharePoolPath, name, capacityGb)
	if err != nil {
		klog.Errorf("Failed to acquire share from pool %s: %v", sharePoolPath, err)
		return nil, file.StatusError(err)
	}

	shareID := poolShare.ShareId

	compositeVolumeID := buildSharePoolVolumeID(sharePoolPath, shareID, poolShare.IpAddress)

	klog.Infof("Successfully acquired Share Pool volume: %q, Composite ID: %q, IP: %q, ShareId: %q", poolShare.ShareId, compositeVolumeID, poolShare.IpAddress, poolShare.ShareId)

	protocol := req.GetParameters()[paramFileProtocol]
	if protocol == "" {
		protocol = v3FileProtocol
	}

	resp := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      compositeVolumeID,
			CapacityBytes: util.GbToBytes(capacityGb),
			VolumeContext: map[string]string{
				attrIP:           poolShare.IpAddress,
				attrVolume:       poolShare.ShareId,
				attrFileProtocol: protocol,
			},
		},
	}

	if mountOptions, ok := req.GetParameters()[paramMountOptions]; ok && mountOptions != "" {
		resp.Volume.VolumeContext[attrMountOptions] = mountOptions
	}
	return resp, nil
}

func (s *controllerServer) handleDeleteSharePoolVolume(ctx context.Context, req *csi.DeleteVolumeRequest, volumeID string) (*csi.DeleteVolumeResponse, error) {
	klog.V(4).Infof("Releasing Share Pool volume from composite ID: %q", volumeID)

	parent, shareID, ipAddress, err := parseSharePoolVolumeID(volumeID)
	if err != nil {
		klog.Errorf("Failed to parse composite volume ID %q: %v", volumeID, err)
		return nil, status.Errorf(codes.InvalidArgument, "failed to parse composite volume ID: %v", err)
	}

	klog.V(4).Infof("Parsed release parameters: parent=%q, shareID=%q, ipAddress=%q", parent, shareID, ipAddress)

	err = s.config.fileService.ReleaseShare(ctx, parent, ipAddress, shareID)
	if err != nil {
		if file.IsNotFoundErr(err) {
			klog.Warningf("Share Pool volume %q not found, returning success", volumeID)
			return &csi.DeleteVolumeResponse{}, nil
		}
		klog.Errorf("Failed to release Share Pool volume %q: %v", volumeID, err)
		return nil, file.StatusError(err)
	}

	klog.Infof("Successfully released Share Pool volume: %q", volumeID)
	return &csi.DeleteVolumeResponse{}, nil
}
