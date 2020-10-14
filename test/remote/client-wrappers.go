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

package remote

import (
	"context"
	"fmt"
	"time"

	csipb "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/glog"
	"google.golang.org/grpc"

	"k8s.io/apimachinery/pkg/util/wait"
)

var (
	stdVolCap = &csipb.VolumeCapability{
		AccessType: &csipb.VolumeCapability_Mount{
			Mount: &csipb.VolumeCapability_MountVolume{},
		},
		AccessMode: &csipb.VolumeCapability_AccessMode{
			Mode: csipb.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
		},
	}
	stdVolCaps = []*csipb.VolumeCapability{
		stdVolCap,
	}
)

type CsiClient struct {
	conn       *grpc.ClientConn
	idClient   csipb.IdentityClient
	nodeClient csipb.NodeClient
	ctrlClient csipb.ControllerClient

	endpoint string
}

func CreateCSIClient(endpoint string) *CsiClient {
	return &CsiClient{endpoint: endpoint}
}

func (c *CsiClient) AssertCSIConnection() error {
	var err error

	if err != nil {
		return err
	}
	if c.conn == nil {
		var conn *grpc.ClientConn
		err = wait.Poll(10*time.Second, 3*time.Minute, func() (bool, error) {
			conn, err = grpc.Dial(
				c.endpoint,
				grpc.WithInsecure(),
			)
			if err != nil {
				glog.Warningf("Client failed to dail endpoint %v", c.endpoint)
				return false, nil
			}
			return true, nil
		})
		if err != nil || conn == nil {
			return fmt.Errorf("Failed to get client connection: %v", err)
		}
		c.conn = conn
		c.idClient = csipb.NewIdentityClient(conn)
		c.nodeClient = csipb.NewNodeClient(conn)
		c.ctrlClient = csipb.NewControllerClient(conn)
	}
	return nil
}

func (c *CsiClient) CloseConn() error {
	return c.conn.Close()
}

func (c *CsiClient) CreateVolume(volName string) (*csipb.Volume, error) {
	cvr := &csipb.CreateVolumeRequest{
		Name:               volName,
		VolumeCapabilities: stdVolCaps,
	}
	cresp, err := c.ctrlClient.CreateVolume(context.Background(), cvr)
	if err != nil {
		return nil, err
	}
	return cresp.GetVolume(), nil
}

func (c *CsiClient) DeleteVolume(volId string) error {
	dvr := &csipb.DeleteVolumeRequest{
		VolumeId: volId,
	}
	_, err := c.ctrlClient.DeleteVolume(context.Background(), dvr)
	return err
}

func (c *CsiClient) NodeUnpublishVolume(volumeId, publishDir string) error {
	nodeUnpublishReq := &csipb.NodeUnpublishVolumeRequest{
		VolumeId:   volumeId,
		TargetPath: publishDir,
	}
	_, err := c.nodeClient.NodeUnpublishVolume(context.Background(), nodeUnpublishReq)
	return err
}

func (c *CsiClient) NodePublishVolume(volumeId, stageDir, publishDir string, volumeAttrs map[string]string) error {
	nodePublishReq := &csipb.NodePublishVolumeRequest{
		VolumeId:          volumeId,
		StagingTargetPath: stageDir,
		TargetPath:        publishDir,
		VolumeCapability:  stdVolCap,
		VolumeContext:     volumeAttrs,
		Readonly:          false,
	}
	_, err := c.nodeClient.NodePublishVolume(context.Background(), nodePublishReq)
	return err
}

func (c *CsiClient) NodeGetInfo() (*csipb.NodeGetInfoResponse, error) {
	resp, err := c.nodeClient.NodeGetInfo(context.Background(), &csipb.NodeGetInfoRequest{})
	return resp, err
}

func (c *CsiClient) NodeStageVolume(volumeId, stageDir string, volumeAttrs map[string]string) error {
	nodeStageReq := &csipb.NodeStageVolumeRequest{
		VolumeId:          volumeId,
		StagingTargetPath: stageDir,
		VolumeCapability:  stdVolCap,
		VolumeContext:     volumeAttrs,
	}
	_, err := c.nodeClient.NodeStageVolume(context.Background(), nodeStageReq)
	return err
}

func (c *CsiClient) NodeUnstageVolume(volumeId, stageDir string) error {
	nodeUnpublishReq := &csipb.NodeUnstageVolumeRequest{
		VolumeId:          volumeId,
		StagingTargetPath: stageDir,
	}
	_, err := c.nodeClient.NodeUnstageVolume(context.Background(), nodeUnpublishReq)
	return err
}
