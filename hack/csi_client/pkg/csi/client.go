package csi

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc"
)

const (
	reqNodePublish   = "nodepublish"
	reqNodeUnpublish = "nodeunpublish"
)

// Client is used to send CSI requests to the driver.
type Client struct {
	node   csi.NodeClient
	closer io.Closer
}

type Request struct {
	RequestType, TargetPath string
	VolumeAttr, Secrets     map[string]string
}

// NewClient constructor for Client.
func NewClient(endpoint string) (*Client, error) {
	var conn *grpc.ClientConn
	conn, err := grpc.Dial(
		endpoint,
		grpc.WithInsecure(),
		grpc.WithDialer(func(target string, timeout time.Duration) (net.Conn, error) {
			return net.Dial("unix", target)
		}),
	)

	if err != nil {
		return nil, err
	}

	return &Client{
		node:   csi.NewNodeClient(conn),
		closer: conn,
	}, nil
}

func (c *Client) NewRequest(req *Request) (err error) {
	switch strings.ToLower(req.RequestType) {
	case reqNodePublish:
		csiReq := &csi.NodePublishVolumeRequest{
			TargetPath: req.TargetPath,
			VolumeCapability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{},
				},
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
				},
			},
			VolumeContext: req.VolumeAttr,
			Secrets:       req.Secrets,
		}

		_, err := c.node.NodePublishVolume(context.Background(), csiReq)

		if err != nil {
			return err
		}
	case reqNodeUnpublish:
		_, err := c.node.NodeUnpublishVolume(context.Background(), &csi.NodeUnpublishVolumeRequest{
			TargetPath: req.TargetPath,
		})

		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("request type %s not supported", req.RequestType)
	}

	return nil
}
