package csi

import (
	"context"
	"fmt"
	"io"
	"net"
	"runtime"
	"strings"
	"time"

	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	"google.golang.org/grpc"
)

const (
	attrIP           = "ip"
	attrVolume       = "volume"
	smbUser          = "smbUser"
	smbPassword      = "smbPassword"
	reqNodePublish   = "nodepublish"
	reqNodeUnpublish = "nodeunpublish"
)

var (
	goOs          = runtime.GOOS
	errHost       = fmt.Errorf("host name is required")
	errVolume     = fmt.Errorf("volume name is required")
	errTargetPath = fmt.Errorf("targetPath is required")
	errNoUsername = fmt.Errorf("username is required")
	errNoPassword = fmt.Errorf("password is required")
)

// Client is used to send CSI requests to the driver.
type Client struct {
	node   csi.NodeClient
	closer io.Closer
}

type Request struct {
	RequestType, ShareAddr, ShareName, TargetPath, Username, Password string
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

func validateRequest(req *Request) (err error) {
	switch strings.ToLower(req.RequestType) {
	case reqNodePublish:
		if req.ShareAddr == "" {
			return errHost
		}

		if req.ShareName == "" {
			return errVolume
		}

		if req.TargetPath == "" {
			return errTargetPath
		}

		if goOs == "windows" {
			if req.Username == "" {
				return errNoUsername
			}

			if req.Password == "" {
				return errNoPassword
			}
		}
	case reqNodeUnpublish:
		if req.TargetPath == "" {
			return errTargetPath
		}
	default:
		return fmt.Errorf("invalid requestType: %s", req.RequestType)
	}
	return nil
}

func (c *Client) NewRequest(req *Request) (err error) {
	err = validateRequest(req)
	if err != nil {
		return err
	}

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
			VolumeAttributes: map[string]string{
				attrIP:     req.ShareAddr,
				attrVolume: req.ShareName,
			},
		}

		if goOs == "windows" {
			csiReq.NodePublishSecrets = map[string]string{
				smbUser:     fmt.Sprintf("%s\\%s", req.ShareAddr, req.Username),
				smbPassword: req.Password,
			}
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
