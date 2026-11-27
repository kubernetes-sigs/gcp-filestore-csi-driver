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

package driver

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apiError "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	mount "k8s.io/mount-utils"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/cloud_provider/metadata"
	lockrelease "sigs.k8s.io/gcp-filestore-csi-driver/pkg/releaselock"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/util"
	"sigs.k8s.io/yaml"
)

var (
	testVolumeCapability = &csi.VolumeCapability{
		AccessType: &csi.VolumeCapability_Mount{
			Mount: &csi.VolumeCapability_MountVolume{},
		},
		AccessMode: &csi.VolumeCapability_AccessMode{
			Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
		},
	}
	testVolumeAttributes = map[string]string{
		attrIP:     "1.1.1.1",
		attrVolume: "test-volume",
	}
	testMultishareVolumeAttributes = map[string]string{
		attrIP: "1.1.1.1",
	}
	testLockReleaseVolumeAttributes = map[string]string{
		attrIP:                 "1.1.1.1",
		attrVolume:             "vol1",
		attrSupportLockRelease: "true",
	}
	testLockReleaseMultishareVolumeAttributes = map[string]string{
		attrIP:                 "1.1.1.1",
		attrSupportLockRelease: "true",
	}
	testDevice = "1.1.1.1:/test-volume"

	testWindowsValidPath = "C:\\test"
	testWindowsSecrets   = map[string]string{
		optionSmbUser:     "foo",
		optionSmbPassword: "bar",
	}
	testWindowsDevice = "\\\\1.1.1.1\\test-volume"
)

type nodeServerTestEnv struct {
	ns csi.NodeServer
	fm *mount.FakeMounter
}

func initTestNodeServer(t *testing.T) *nodeServerTestEnv {
	// TODO: make a constructor in FakeMmounter library
	mounter := &mount.FakeMounter{MountPoints: []mount.MountPoint{}}
	metaserice, err := metadata.NewFakeService()
	if err != nil {
		t.Fatalf("Failed to init metadata service")
	}
	ns, err := newNodeServer(initTestDriver(t), mounter, metaserice, &GCFSDriverFeatureOptions{FeatureLockRelease: &FeatureLockRelease{}})
	if err != nil {
		t.Fatalf("Failed to create node server: %v", err)
	}
	return &nodeServerTestEnv{
		ns: ns,
		fm: mounter,
	}
}

func initTestNodeServerWithKubeClient(t *testing.T, client kubernetes.Interface) *nodeServer {
	mounter := &mount.FakeMounter{MountPoints: []mount.MountPoint{}}
	metaserice, err := metadata.NewFakeService()
	if err != nil {
		t.Fatalf("Failed to init metadata service")
	}
	return &nodeServer{
		driver:                initTestDriver(t),
		mounter:               mounter,
		metaService:           metaserice,
		volumeLocks:           util.NewVolumeLocks(),
		lockReleaseController: lockrelease.NewControllerBuilder().WithClient(client).Build(),
		features:              &GCFSDriverFeatureOptions{FeatureLockRelease: &FeatureLockRelease{Enabled: true}},
	}
}

func TestNodePublishVolume(t *testing.T) {
	defaultPerm := os.FileMode(0750) + os.ModeDir

	// Setup mount target path
	base, err := ioutil.TempDir("", "node-publish-")
	if err != nil {
		t.Fatalf("failed to setup testdir: %v", err)
	}
	testTargetPath := filepath.Join(base, "mount")
	if err = os.MkdirAll(testTargetPath, defaultPerm); err != nil {
		t.Fatalf("failed to setup target path: %v", err)
	}
	stagingTargetPath := filepath.Join(base, "staging")
	defer os.RemoveAll(base)
	cases := []struct {
		name          string
		mounts        []mount.MountPoint // already existing mounts
		req           *csi.NodePublishVolumeRequest
		actions       []mount.FakeAction
		expectedMount *mount.MountPoint
		expectErr     bool
	}{
		{
			name:      "empty request",
			req:       &csi.NodePublishVolumeRequest{},
			expectErr: true,
		},
		{
			name: "volid request not already mounted",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:          testVolumeID,
				StagingTargetPath: stagingTargetPath,
				TargetPath:        testTargetPath,
				VolumeCapability:  testVolumeCapability,
				VolumeContext:     testVolumeAttributes,
			},
			actions:       []mount.FakeAction{{Action: mount.FakeActionMount}},
			expectedMount: &mount.MountPoint{Device: stagingTargetPath, Path: testTargetPath, Type: "nfs", Opts: []string{"bind"}},
		},
		{
			name:   "volid request already mounted",
			mounts: []mount.MountPoint{{Device: "/test-device", Path: testTargetPath}},
			req: &csi.NodePublishVolumeRequest{
				VolumeId:          testVolumeID,
				StagingTargetPath: stagingTargetPath,
				TargetPath:        testTargetPath,
				VolumeCapability:  testVolumeCapability,
				VolumeContext:     testVolumeAttributes,
			},
			expectedMount: &mount.MountPoint{Device: "/test-device", Path: testTargetPath},
		},
		{
			name: "valid request with user mount options",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:          testVolumeID,
				StagingTargetPath: stagingTargetPath,
				TargetPath:        testTargetPath,
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							MountFlags: []string{"foo", "bar"},
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				VolumeContext: testVolumeAttributes,
			},
			actions: []mount.FakeAction{{Action: mount.FakeActionMount}},

			expectedMount: &mount.MountPoint{Device: stagingTargetPath, Path: testTargetPath, Type: "nfs", Opts: []string{"bind", "foo", "bar"}},
		},
		{
			name: "valid request read only",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:          testVolumeID,
				StagingTargetPath: stagingTargetPath,
				TargetPath:        testTargetPath,
				VolumeCapability:  testVolumeCapability,
				VolumeContext:     testVolumeAttributes,
				Readonly:          true,
			},
			actions:       []mount.FakeAction{{Action: mount.FakeActionMount}},
			expectedMount: &mount.MountPoint{Device: stagingTargetPath, Path: testTargetPath, Type: "nfs", Opts: []string{"bind", "ro"}},
		},
		{
			name: "empty target path",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:          testVolumeID,
				StagingTargetPath: stagingTargetPath,
				VolumeCapability:  testVolumeCapability,
				VolumeContext:     testVolumeAttributes,
			},
			expectErr: true,
		},
		{
			name: "empty staging target path",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:         testVolumeID,
				TargetPath:       testTargetPath,
				VolumeCapability: testVolumeCapability,
				VolumeContext:    testVolumeAttributes,
			},
			expectErr: true,
		},
		{
			name: "invalid volume capability",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:      testVolumeID,
				TargetPath:    testTargetPath,
				VolumeContext: testVolumeAttributes,
			},
			expectErr: true,
		},
		{
			name: "invalid volume attribute",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:         testVolumeID,
				TargetPath:       testTargetPath,
				VolumeCapability: testVolumeCapability,
			},
			expectErr: true,
		},
		// multishare
		{
			name: "multishare volid request not already mounted",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:          testMultishareVolumeID,
				StagingTargetPath: stagingTargetPath,
				TargetPath:        testTargetPath,
				VolumeCapability:  testVolumeCapability,
				VolumeContext:     testMultishareVolumeAttributes,
			},
			actions:       []mount.FakeAction{{Action: mount.FakeActionMount}},
			expectedMount: &mount.MountPoint{Device: stagingTargetPath, Path: testTargetPath, Type: "nfs", Opts: []string{"bind"}},
		},
		{
			name:   "multishare volid request already mounted",
			mounts: []mount.MountPoint{{Device: "/test-device", Path: testTargetPath}},
			req: &csi.NodePublishVolumeRequest{
				VolumeId:          testMultishareVolumeID,
				StagingTargetPath: stagingTargetPath,
				TargetPath:        testTargetPath,
				VolumeCapability:  testVolumeCapability,
				VolumeContext:     testMultishareVolumeAttributes,
			},
			expectedMount: &mount.MountPoint{Device: "/test-device", Path: testTargetPath},
		},
		{
			name: "multishare volid request with user mount options",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:          testMultishareVolumeID,
				StagingTargetPath: stagingTargetPath,
				TargetPath:        testTargetPath,
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							MountFlags: []string{"foo", "bar"},
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				VolumeContext: testMultishareVolumeAttributes,
			},
			actions: []mount.FakeAction{{Action: mount.FakeActionMount}},

			expectedMount: &mount.MountPoint{Device: stagingTargetPath, Path: testTargetPath, Type: "nfs", Opts: []string{"bind", "foo", "bar"}},
		},
		{
			name: "multishare volid request read only",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:          testMultishareVolumeID,
				StagingTargetPath: stagingTargetPath,
				TargetPath:        testTargetPath,
				VolumeCapability:  testVolumeCapability,
				VolumeContext:     testMultishareVolumeAttributes,
				Readonly:          true,
			},
			actions:       []mount.FakeAction{{Action: mount.FakeActionMount}},
			expectedMount: &mount.MountPoint{Device: stagingTargetPath, Path: testTargetPath, Type: "nfs", Opts: []string{"bind", "ro"}},
		},
		{
			name: "multishare volid empty target path",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:          testMultishareVolumeID,
				StagingTargetPath: stagingTargetPath,
				VolumeCapability:  testVolumeCapability,
				VolumeContext:     testMultishareVolumeAttributes,
			},
			expectErr: true,
		},
		{
			name: "multishare volid empty staging target path",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:         testMultishareVolumeID,
				TargetPath:       testTargetPath,
				VolumeCapability: testVolumeCapability,
				VolumeContext:    testMultishareVolumeAttributes,
			},
			expectErr: true,
		},
		{
			name: "multishare volid invalid volume capability",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:      testMultishareVolumeID,
				TargetPath:    testTargetPath,
				VolumeContext: testMultishareVolumeAttributes,
			},
			expectErr: true,
		},
		{
			name: "multishare volid invalid volume attribute",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:         testMultishareVolumeID,
				TargetPath:       testTargetPath,
				VolumeCapability: testVolumeCapability,
			},
			expectErr: true,
		},
		{
			name: "valid request with mountOptions in VolumeContext",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:          testVolumeID,
				StagingTargetPath: stagingTargetPath,
				TargetPath:        testTargetPath,
				VolumeCapability:  testVolumeCapability,
				VolumeContext: map[string]string{
					attrIP:           "1.1.1.1",
					attrVolume:       "test-volume",
					attrMountOptions: "noatime,nodiratime",
				},
			},
			actions:       []mount.FakeAction{{Action: mount.FakeActionMount}},
			expectedMount: &mount.MountPoint{Device: stagingTargetPath, Path: testTargetPath, Type: "nfs", Opts: []string{"bind", "noatime,nodiratime"}},
		},
		{
			name: "VolumePool volume without mount options injects no defaults",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:          testVolumePoolVolumeID,
				StagingTargetPath: stagingTargetPath,
				TargetPath:        testTargetPath,
				VolumeCapability:  testVolumeCapability,
				VolumeContext: map[string]string{
					attrIP:           "10.1.1.1",
					attrVolume:       "vol1",
					attrFileProtocol: v3FileProtocol,
				},
			},
			actions:       []mount.FakeAction{{Action: mount.FakeActionMount}},
			expectedMount: &mount.MountPoint{Device: stagingTargetPath, Path: testTargetPath, Type: "nfs", Opts: []string{"bind"}},
		},
		{
			name: "VolumePool volume with the Substrate template mount options in VolumeContext",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:          testVolumePoolVolumeID,
				StagingTargetPath: stagingTargetPath,
				TargetPath:        testTargetPath,
				VolumeCapability:  testVolumeCapability,
				VolumeContext: map[string]string{
					attrIP:           "10.1.1.1",
					attrVolume:       "vol1",
					attrFileProtocol: v3FileProtocol,
					attrMountOptions: substrateTemplateMountOptions,
				},
			},
			actions:       []mount.FakeAction{{Action: mount.FakeActionMount}},
			expectedMount: &mount.MountPoint{Device: stagingTargetPath, Path: testTargetPath, Type: "nfs", Opts: []string{"bind", substrateTemplateMountOptions}},
		},
		{
			name: "VolumePool volume read-only with the Substrate template mount options in VolumeContext",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:          testVolumePoolVolumeID,
				StagingTargetPath: stagingTargetPath,
				TargetPath:        testTargetPath,
				VolumeCapability:  testVolumeCapability,
				VolumeContext: map[string]string{
					attrIP:           "10.1.1.1",
					attrVolume:       "vol1",
					attrFileProtocol: v3FileProtocol,
					attrMountOptions: substrateTemplateMountOptions,
				},
				Readonly: true,
			},
			actions:       []mount.FakeAction{{Action: mount.FakeActionMount}},
			expectedMount: &mount.MountPoint{Device: stagingTargetPath, Path: testTargetPath, Type: "nfs", Opts: []string{"bind", "ro", substrateTemplateMountOptions}},
		},
		// TODO: Revisit this (https://github.com/kubernetes-sigs/gcp-filestore-csi-driver/issues/47).
		// {
		// 	name: "target path doesn't exist",
		// 	req: &csi.NodePublishVolumeRequest{
		// 		VolumeId:         testVolumeID,
		// 		TargetPath:       "/node-publish-test-not-exists",
		// 		VolumeCapability: testVolumeCapability,
		// 		VolumeContext:    testVolumeAttributes,
		// 	},
		// 	expectErr: true,
		// },
		// TODO add a test case for mount failure.
		// need to modify FakeMounter to be able to fail the mount call (and unmount)
	}

	for _, test := range cases {
		testEnv := initTestNodeServer(t)
		if test.mounts != nil {
			testEnv.fm.MountPoints = test.mounts
		}

		_, err = testEnv.ns.NodePublishVolume(context.TODO(), test.req)
		if !test.expectErr && err != nil {
			t.Errorf("test %q failed: %v", test.name, err)
		}
		if test.expectErr && err == nil {
			t.Errorf("test %q failed: got success", test.name)
		}

		validateMountPoint(t, test.name, testEnv.fm, test.expectedMount)
		// TODO: ValidateMountActions if possible.
	}
}

// TODO: Revisit windows tests
func testWindowsNodePublishVolume(t *testing.T) {
	defaultPerm := os.FileMode(0750) + os.ModeDir
	defaultOsString := goOs

	// Setup mount target path
	base, err := ioutil.TempDir("", "node-publish-")
	if err != nil {
		t.Fatalf("failed to setup testdir: %v", err)
	}
	testTargetPath := filepath.Join(base, "mount")
	if err = os.MkdirAll(testTargetPath, defaultPerm); err != nil {
		t.Fatalf("failed to setup target path: %v", err)
	}
	defer os.RemoveAll(base)

	goOs = "windows"

	cases := []struct {
		name          string
		mounts        []mount.MountPoint // already existing mounts
		req           *csi.NodePublishVolumeRequest
		actions       []mount.FakeAction
		expectedMount *mount.MountPoint
		expectErr     bool
	}{
		// TODO: enable this test after https://github.com/kubernetes/kubernetes/issues/81609

		// {
		// 	name:     "windows target path does exist",
		// 	req: &csi.NodePublishVolumeRequest{
		// 		VolumeId:         testVolumeID,
		// 		TargetPath:       testTargetPath,
		// 		VolumeCapability: testVolumeCapability,
		// 		VolumeAttributes: testVolumeAttributes,
		// 		NodePublishSecrets: testWindowsSecrets
		// 	},
		// 	expectErr: true,
		// },
		{
			name: "windows target path doesn't exist",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:         testVolumeID,
				TargetPath:       testWindowsValidPath,
				VolumeCapability: testVolumeCapability,
				VolumeContext:    testVolumeAttributes,
				Secrets:          testWindowsSecrets,
			},

			actions:       []mount.FakeAction{{Action: mount.FakeActionMount}},
			expectedMount: &mount.MountPoint{Device: testWindowsDevice, Path: testWindowsValidPath, Type: "cifs", Opts: []string{"foo", "bar"}},
		},
		{
			name: "windows no user",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:         testVolumeID,
				TargetPath:       testWindowsValidPath,
				VolumeCapability: testVolumeCapability,
				VolumeContext:    testVolumeAttributes,
				Secrets: map[string]string{
					optionSmbPassword: "bar",
				},
			},
			expectErr: true,
		},
		{
			name: "windows no password",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:         testVolumeID,
				TargetPath:       testWindowsValidPath,
				VolumeCapability: testVolumeCapability,
				VolumeContext:    testVolumeAttributes,
				Secrets: map[string]string{
					optionSmbUser: "foo",
				},
			},
			expectErr: true,
		},
	}

	for _, test := range cases {
		testEnv := initTestNodeServer(t)
		if test.mounts != nil {
			testEnv.fm.MountPoints = test.mounts
		}

		_, err = testEnv.ns.NodePublishVolume(context.TODO(), test.req)
		if !test.expectErr && err != nil {
			t.Errorf("test %q failed: %v", test.name, err)
		}
		if test.expectErr && err == nil {
			t.Errorf("test %q failed: got success", test.name)
		}

		validateMountPoint(t, test.name, testEnv.fm, test.expectedMount)
		// TODO: ValidateMountActions if possible.
	}
	goOs = defaultOsString
}

func TestNodeUnpublishVolume(t *testing.T) {
	defaultPerm := os.FileMode(0750) + os.ModeDir

	// Setup mount target path
	base, err := ioutil.TempDir("", "node-publish-")
	if err != nil {
		t.Fatalf("failed to setup testdir: %v", err)
	}
	defer os.RemoveAll(base)
	testTargetPath := filepath.Join(base, "mount")
	if err = os.MkdirAll(testTargetPath, defaultPerm); err != nil {
		t.Fatalf("failed to setup target path: %v", err)
	}

	cases := []struct {
		name          string
		mounts        []mount.MountPoint // already existing mounts
		req           *csi.NodeUnpublishVolumeRequest
		actions       []mount.FakeAction
		expectedMount *mount.MountPoint
		expectErr     bool
	}{
		{
			name:   "successful unmount",
			mounts: []mount.MountPoint{{Device: testDevice, Path: testTargetPath}},
			req: &csi.NodeUnpublishVolumeRequest{
				VolumeId:   testVolumeID,
				TargetPath: testTargetPath,
			},
			actions: []mount.FakeAction{{Action: mount.FakeActionUnmount}},
		},
		{
			name: "empty target path",
			req: &csi.NodeUnpublishVolumeRequest{
				VolumeId: testVolumeID,
			},
			expectErr: true,
		},
		{
			name: "dir doesn't exist",
			req: &csi.NodeUnpublishVolumeRequest{
				VolumeId:   testVolumeID,
				TargetPath: "/node-unpublish-dir-not-exists",
			},
		},
		{
			name: "dir not mounted",
			req: &csi.NodeUnpublishVolumeRequest{
				VolumeId:   testVolumeID,
				TargetPath: testTargetPath,
			},
		},
		// TODO:
		// mount check failed
		// unmount failed
	}

	for _, test := range cases {
		testEnv := initTestNodeServer(t)
		if test.mounts != nil {
			testEnv.fm.MountPoints = test.mounts
		}

		_, err = testEnv.ns.NodeUnpublishVolume(context.TODO(), test.req)
		if !test.expectErr && err != nil {
			t.Errorf("test %q failed: %v", test.name, err)
		}
		if test.expectErr && err == nil {
			t.Errorf("test %q failed: got success", test.name)
		}

		validateMountPoint(t, test.name, testEnv.fm, test.expectedMount)
		// TODO: ValidateMountActions if possible.
	}
}

func TestValidateVolumeAttributes(t *testing.T) {
	cases := []struct {
		name      string
		attrs     map[string]string
		expectErr bool
	}{
		{
			name: "valid attributes",
			attrs: map[string]string{
				attrIP:     "1.1.1.1",
				attrVolume: "vol1",
			},
		},
		{
			name: "invalid ip",
			attrs: map[string]string{
				attrVolume: "vol1",
			},
			expectErr: true,
		},
		{
			name: "invalid volume",
			attrs: map[string]string{
				attrIP: "1.1.1.1",
			},
			expectErr: true,
		},
	}

	for _, test := range cases {
		err := validateVolumeAttributes(test.attrs)
		if !test.expectErr && err != nil {
			t.Errorf("test %q failed: %v", test.name, err)
		}
		if test.expectErr && err == nil {
			t.Errorf("test %q failed: got success", test.name)
		}
	}
}

// TODO
func TestNodeGetId(t *testing.T) {
}

// TODO
func TestNodeGetInfo(t *testing.T) {
}

// TODO
func TestNodeGetCapabilities(t *testing.T) {
}

func TestNodeGetVolumeStats(t *testing.T) {
	testEnv := initTestNodeServer(t)
	ns := testEnv.ns

	// Setup mount target path
	tempDir, err := ioutil.TempDir("", "ngvs")
	if err != nil {
		t.Fatalf("failed to setup testdir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	targetPath := filepath.Join(tempDir, "mount")
	stagingPath := filepath.Join(tempDir, "staging")

	req := &csi.NodePublishVolumeRequest{
		VolumeId:          testVolumeID,
		TargetPath:        targetPath,
		StagingTargetPath: stagingPath,
		Readonly:          false,
		VolumeCapability:  testVolumeCapability,
		VolumeContext:     testVolumeAttributes,
	}
	_, err = ns.NodePublishVolume(context.Background(), req)
	if err != nil {
		t.Fatalf("Failed to set up test by publishing default vol: %v", err)
	}

	testCases := []struct {
		name       string
		volumeID   string
		volumePath string
		expectErr  bool
	}{
		{
			name:       "normal",
			volumeID:   testVolumeID,
			volumePath: targetPath,
		},
		{
			name:       "no vol id",
			volumePath: targetPath,
			expectErr:  true,
		},
		{
			name:      "no vol path",
			volumeID:  testVolumeID,
			expectErr: true,
		},
		{
			name:       "bad vol path",
			volumeID:   testVolumeID,
			volumePath: "/mnt/fake",
			expectErr:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &csi.NodeGetVolumeStatsRequest{
				VolumeId:   tc.volumeID,
				VolumePath: tc.volumePath,
			}
			_, err := ns.NodeGetVolumeStats(context.Background(), req)
			if err != nil && !tc.expectErr {
				t.Fatalf("Got unexpected err: %v", err)
			}
			if err == nil && tc.expectErr {
				t.Fatal("Did not get error but expected one")
			}
		})
	}

}

func validateMountPoint(t *testing.T, name string, fm *mount.FakeMounter, e *mount.MountPoint) {
	if e == nil {
		if len(fm.MountPoints) != 0 {
			t.Errorf("test %q failed: got mounts %+v, expected none", name, fm.MountPoints)
		}
		return
	}

	if mLen := len(fm.MountPoints); mLen != 1 {
		t.Errorf("test %q failed: got %v mounts(%+v), expected %v", name, mLen, fm.MountPoints, 1)
		return
	}

	a := &fm.MountPoints[0]
	if a.Device != e.Device {
		t.Errorf("test %q failed: got device %q, expected %q", name, a.Device, e.Device)
	}
	if a.Path != e.Path {
		t.Errorf("test %q failed: got path %q, expected %q", name, a.Path, e.Path)
	}
	if a.Type != e.Type {
		t.Errorf("test %q failed: got type %q, expected %q", name, a.Type, e.Type)
	}

	// TODO: why does DeepEqual not work???
	aLen := len(a.Opts)
	eLen := len(e.Opts)
	if aLen != eLen {
		t.Errorf("test %q failed: got opts length %v, expected %v", name, aLen, eLen)
	} else {
		for i := range a.Opts {
			aOpt := a.Opts[i]
			eOpt := e.Opts[i]
			if aOpt != eOpt {
				t.Errorf("test %q failed: got opt %q, expected %q", name, aOpt, eOpt)
			}
		}
	}
}

type FakeBlockingMounter struct {
	*mount.FakeMounter
	// 'OperationUnblocker' channel is used to block the execution of the respective function using it. This is done by sending a channel of empty struct over 'OperationUnblocker' channel, and wait until the tester gives a go-ahead to proceed further in the execution of the function.
	OperationUnblocker chan chan struct{}
}

func NewFakeBlockingMounter(operationUnblocker chan chan struct{}) *FakeBlockingMounter {
	return &FakeBlockingMounter{
		FakeMounter:        &mount.FakeMounter{MountPoints: []mount.MountPoint{}},
		OperationUnblocker: operationUnblocker,
	}
}

func (m *FakeBlockingMounter) Mount(source string, target string, fstype string, options []string) error {
	execute := make(chan struct{})
	m.OperationUnblocker <- execute
	<-execute
	return m.FakeMounter.Mount(source, target, fstype, options)
}

func initBlockingTestNodeServer(t *testing.T, operationUnblocker chan chan struct{}) *nodeServerTestEnv {
	mounter := NewFakeBlockingMounter(operationUnblocker)
	metaserice, err := metadata.NewFakeService()
	if err != nil {
		t.Fatalf("Failed to init metadata service")
	}
	featureOptions := &GCFSDriverFeatureOptions{
		FeatureLockRelease:  &FeatureLockRelease{},
		FeatureNFSv4Support: &FeatureNFSv4Support{},
	}
	ns, err := newNodeServer(initTestDriver(t), mounter, metaserice, featureOptions)
	if err != nil {
		t.Fatalf("Failed to create node server: %v", err)
	}
	return &nodeServerTestEnv{
		ns: ns,
		fm: nil,
	}
}

type NodeRequestConfig struct {
	NodePublishReq   *csi.NodePublishVolumeRequest
	NodeUnpublishReq *csi.NodeUnpublishVolumeRequest
	NodeStageReq     *csi.NodeStageVolumeRequest
	NodeUnstageReq   *csi.NodeUnstageVolumeRequest
}

func TestConcurrentMounts(t *testing.T) {
	// A channel of size 1 is sufficient, because the caller of runRequest() in below steps immediately blocks and retrieves the channel of empty struct from 'operationUnblocker' channel. The test steps are such that, atmost one function pushes items on the 'operationUnblocker' channel, to indicate that the function is blocked and waiting for a signal to proceed futher in the execution.
	operationUnblocker := make(chan chan struct{}, 1)
	ns := initBlockingTestNodeServer(t, operationUnblocker)
	basePath, err := ioutil.TempDir("", "node-publish-")
	if err != nil {
		t.Fatalf("failed to setup testdir: %v", err)
	}
	stagingTargetPath := filepath.Join(basePath, "staging")
	targetPath1 := filepath.Join(basePath, "target1")
	targetPath2 := filepath.Join(basePath, "target2")

	runRequest := func(req *NodeRequestConfig) <-chan error {
		resp := make(chan error)
		go func() {
			var err error
			if req.NodePublishReq != nil {
				_, err = ns.ns.NodePublishVolume(context.Background(), req.NodePublishReq)
			} else if req.NodeUnpublishReq != nil {
				_, err = ns.ns.NodeUnpublishVolume(context.Background(), req.NodeUnpublishReq)
			} else if req.NodeStageReq != nil {
				_, err = ns.ns.NodeStageVolume(context.Background(), req.NodeStageReq)
			} else if req.NodeUnstageReq != nil {
				_, err = ns.ns.NodeUnstageVolume(context.Background(), req.NodeUnstageReq)
			}
			resp <- err
		}()
		return resp
	}

	// Node stage blocked after lock acquire.
	resp := runRequest(&NodeRequestConfig{
		NodeStageReq: &csi.NodeStageVolumeRequest{
			VolumeId:          testVolumeID,
			StagingTargetPath: stagingTargetPath,
			VolumeCapability:  testVolumeCapability,
			VolumeContext:     testVolumeAttributes,
		},
	})
	nodestageOpUnblocker := <-operationUnblocker

	// Same volume ID node stage should fail to acquire lock and return Aborted error.
	stageResp2 := runRequest(&NodeRequestConfig{
		NodeStageReq: &csi.NodeStageVolumeRequest{
			VolumeId:          testVolumeID,
			StagingTargetPath: stagingTargetPath,
			VolumeCapability:  testVolumeCapability,
			VolumeContext:     testVolumeAttributes,
		},
	})
	ValidateExpectedError(t, stageResp2, operationUnblocker, codes.Aborted)

	// Same volume ID node unstage should fail to acquire lock and return Aborted error.
	unstageResp := runRequest(&NodeRequestConfig{
		NodeUnstageReq: &csi.NodeUnstageVolumeRequest{
			VolumeId:          testVolumeID,
			StagingTargetPath: stagingTargetPath,
		},
	})
	ValidateExpectedError(t, unstageResp, operationUnblocker, codes.Aborted)

	// Unblock first node stage. Success expected.
	nodestageOpUnblocker <- struct{}{}
	if err := <-resp; err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Node publish blocked after lock acquire on the 'targetPath1'.
	targetPath1Publishresp := runRequest(&NodeRequestConfig{
		NodePublishReq: &csi.NodePublishVolumeRequest{
			VolumeId:          testVolumeID,
			StagingTargetPath: stagingTargetPath,
			TargetPath:        targetPath1,
			VolumeCapability:  testVolumeCapability,
			VolumeContext:     testVolumeAttributes,
		},
	})
	nodepublishOpTargetPath1Unblocker := <-operationUnblocker

	// Node publish for the same target path should fail to acquire lock and return Aborted error.
	targetPath1Publishresp2 := runRequest(&NodeRequestConfig{
		NodePublishReq: &csi.NodePublishVolumeRequest{
			VolumeId:          testVolumeID,
			StagingTargetPath: stagingTargetPath,
			TargetPath:        targetPath1,
			VolumeCapability:  testVolumeCapability,
			VolumeContext:     testVolumeAttributes,
		},
	})
	ValidateExpectedError(t, targetPath1Publishresp2, operationUnblocker, codes.Aborted)

	// Node unpublish for the same target path should fail to acquire lock and return Aborted error.
	targetPath1Unpublishresp := runRequest(&NodeRequestConfig{
		NodeUnpublishReq: &csi.NodeUnpublishVolumeRequest{
			VolumeId:   testVolumeID,
			TargetPath: targetPath1,
		},
	})
	ValidateExpectedError(t, targetPath1Unpublishresp, operationUnblocker, codes.Aborted)

	// Node publish succeeds for a second target path 'targetPath2'.
	targetPath2Publishresp2 := runRequest(&NodeRequestConfig{
		NodePublishReq: &csi.NodePublishVolumeRequest{
			VolumeId:          testVolumeID,
			StagingTargetPath: stagingTargetPath,
			TargetPath:        targetPath2,
			VolumeCapability:  testVolumeCapability,
			VolumeContext:     testVolumeAttributes,
		},
	})
	nodepublishOpTargetPath2Unblocker := <-operationUnblocker
	nodepublishOpTargetPath2Unblocker <- struct{}{}
	if err := <-targetPath2Publishresp2; err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Node unpublish succeeds for second target path.
	targetPath2Unpublishresp := runRequest(&NodeRequestConfig{
		NodeUnpublishReq: &csi.NodeUnpublishVolumeRequest{
			VolumeId:   testVolumeID,
			TargetPath: targetPath2,
		},
	})
	if err := <-targetPath2Unpublishresp; err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Unblock first node publish, and success expected.
	nodepublishOpTargetPath1Unblocker <- struct{}{}
	if err := <-targetPath1Publishresp; err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestNodeStageVolumeUpdateLockInfo(t *testing.T) {
	basePath, err := ioutil.TempDir("", "node-publish-")
	if err != nil {
		t.Fatalf("failed to setup testdir: %v", err)
	}
	stagingTargetPath := filepath.Join(basePath, "staging")
	cases := []struct {
		name       string
		req        *csi.NodeStageVolumeRequest
		existingCM *corev1.ConfigMap
		expectedCM *corev1.ConfigMap
		expectErr  bool
	}{
		{
			name: "non enterprise tier filestore instance",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          testVolumeID,
				StagingTargetPath: stagingTargetPath,
				VolumeCapability:  testVolumeCapability,
				VolumeContext:     testVolumeAttributes,
			},
			existingCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "fscsi-test-node",
					Namespace:  util.ManagedFilestoreCSINamespace,
					Finalizers: []string{lockrelease.ConfigMapFinalzer},
				},
				Data: map[string]string{
					"test-project.us-central1-c.test-csi.vol1.123456.127_0_0_1": "1.1.1.1",
				},
			},
			expectedCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "fscsi-test-node",
					Namespace:  util.ManagedFilestoreCSINamespace,
					Finalizers: []string{lockrelease.ConfigMapFinalzer},
				},
				Data: map[string]string{
					"test-project.us-central1-c.test-csi.vol1.123456.127_0_0_1": "1.1.1.1",
				},
			},
		},
		{
			name: "configmap not found for the current node",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          testVolumeID, //us-central1-c/test-csi/vol1
				StagingTargetPath: stagingTargetPath,
				VolumeCapability:  testVolumeCapability,
				VolumeContext:     testLockReleaseVolumeAttributes,
			},
			existingCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "fscsi-test-node-1",
					Namespace:  util.ManagedFilestoreCSINamespace,
					Finalizers: []string{lockrelease.ConfigMapFinalzer},
				},
				Data: map[string]string{
					"test-project.us-central1-c.test-csi.vol1.1234567.127_0_0_2": "1.1.1.1",
				},
			},
			expectedCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "fscsi-test-node",
					Namespace:  util.ManagedFilestoreCSINamespace,
					Finalizers: []string{lockrelease.ConfigMapFinalzer},
				},
				Data: map[string]string{
					"test-project.us-central1-c.test-csi.vol1.123456.127_0_0_1": "1.1.1.1",
				},
			},
		},
		{
			name: "configmap for the current node exists",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          testVolumeID, //us-central1-c/test-csi/vol1
				StagingTargetPath: stagingTargetPath,
				VolumeCapability:  testVolumeCapability,
				VolumeContext:     testLockReleaseVolumeAttributes,
			},
			existingCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "fscsi-test-node",
					Namespace:  util.ManagedFilestoreCSINamespace,
					Finalizers: []string{lockrelease.ConfigMapFinalzer},
				},
				Data: map[string]string{
					"test-project.us-central1.test-filestore.test-share.123456.192_168_1_1": "192.168.92.0",
				},
			},
			expectedCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "fscsi-test-node",
					Namespace:  util.ManagedFilestoreCSINamespace,
					Finalizers: []string{lockrelease.ConfigMapFinalzer},
				},
				Data: map[string]string{
					"test-project.us-central1.test-filestore.test-share.123456.192_168_1_1": "192.168.92.0",
					"test-project.us-central1-c.test-csi.vol1.123456.127_0_0_1":             "1.1.1.1",
				},
			},
		},
		{
			name: "configmap for the current node exists, key already exists",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          testVolumeID, //us-central1-c/test-csi/vol1
				StagingTargetPath: stagingTargetPath,
				VolumeCapability:  testVolumeCapability,
				VolumeContext:     testLockReleaseVolumeAttributes,
			},
			existingCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "fscsi-test-node",
					Namespace:  util.ManagedFilestoreCSINamespace,
					Finalizers: []string{lockrelease.ConfigMapFinalzer},
				},
				Data: map[string]string{
					"test-project.us-central1-c.test-csi.vol1.123456.127_0_0_1": "1.1.1.1",
				},
			},
			expectedCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "fscsi-test-node",
					Namespace:  util.ManagedFilestoreCSINamespace,
					Finalizers: []string{lockrelease.ConfigMapFinalzer},
				},
				Data: map[string]string{
					"test-project.us-central1-c.test-csi.vol1.123456.127_0_0_1": "1.1.1.1",
				},
			},
		},
	}
	for _, test := range cases {
		client := fake.NewSimpleClientset(test.existingCM)
		server := initTestNodeServerWithKubeClient(t, client)
		ctx := context.Background()
		err := server.nodeStageVolumeUpdateLockInfo(ctx, test.req)
		if gotExpected := gotExpectedError(test.name, test.expectErr, err); gotExpected != nil {
			t.Fatal(gotExpected)
		}
		cm, err := client.CoreV1().ConfigMaps(test.expectedCM.Namespace).Get(ctx, test.expectedCM.Name, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("test %q failed: unexpected error %v", test.name, err)
		}
		if diff := cmp.Diff(test.expectedCM, cm); diff != "" {
			t.Errorf("test %q failed: unexpected diff (-want +got):\n%s", test.name, diff)
		}
	}
}

func TestNodeStageVolume_AppendsMountOptions(t *testing.T) {
	testEnv := initTestNodeServer(t)
	attr := map[string]string{
		attrIP:           "10.0.0.1",
		attrVolume:       "vol1",
		attrMountOptions: "hard,intr",
	}
	req := &csi.NodeStageVolumeRequest{
		VolumeId:          "vol-test",
		StagingTargetPath: "/tmp/stage",
		VolumeCapability: &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Mount{
				Mount: &csi.VolumeCapability_MountVolume{},
			},
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			},
		},
		VolumeContext: attr,
	}

	_, err := testEnv.ns.NodeStageVolume(context.Background(), req)
	if err != nil {
		t.Fatalf("NodeStageVolume failed: %v", err)
	}

	fm := testEnv.fm
	if len(fm.MountPoints) == 0 {
		t.Fatalf("expected a mount call")
	}
	found := false
	for _, mp := range fm.MountPoints {
		for _, opt := range mp.Opts {
			if opt == "hard,intr" {
				found = true
				break
			}
		}
	}
	if !found {
		t.Errorf("expected mount options 'hard,intr' to be present in mount call")
	}
}

const testVolumePoolVolumeID = "volumepool://test-project/us-central1/my-pool/volumes/test-vol?ip=10.1.1.1"

func TestShouldApplyVolumePoolNFSv3DefaultMountOptions(t *testing.T) {
	tests := []struct {
		name     string
		volumeID string
		attr     map[string]string
		options  []string
		want     bool
	}{
		{
			name:     "VolumePool, protocol NFS_V3, no options",
			volumeID: testVolumePoolVolumeID,
			attr:     map[string]string{attrFileProtocol: v3FileProtocol},
			want:     true,
		},
		{
			name:     "VolumePool, protocol omitted, no options",
			volumeID: testVolumePoolVolumeID,
			attr:     map[string]string{},
			want:     true,
		},
		{
			name:     "VolumePool, nil volume context, no options",
			volumeID: testVolumePoolVolumeID,
			attr:     nil,
			want:     true,
		},
		{
			name:     "VolumePool, protocol empty string, no options",
			volumeID: testVolumePoolVolumeID,
			attr:     map[string]string{attrFileProtocol: ""},
			want:     true,
		},
		{
			name:     "VolumePool, protocol lowercase nfs_v3, no options",
			volumeID: testVolumePoolVolumeID,
			attr:     map[string]string{attrFileProtocol: "nfs_v3"},
			want:     true,
		},
		{
			name:     "VolumePool, protocol mixed case Nfs_V3, no options",
			volumeID: testVolumePoolVolumeID,
			attr:     map[string]string{attrFileProtocol: "Nfs_V3"},
			want:     true,
		},
		{
			name:     "VolumePool, protocol NFS_V4_1, no options",
			volumeID: testVolumePoolVolumeID,
			attr:     map[string]string{attrFileProtocol: v4_1FileProtocol},
			want:     false,
		},
		{
			name:     "VolumePool, protocol lowercase nfs_v4_1, no options",
			volumeID: testVolumePoolVolumeID,
			attr:     map[string]string{attrFileProtocol: "nfs_v4_1"},
			want:     false,
		},
		{
			name:     "VolumePool, unknown protocol, no options",
			volumeID: testVolumePoolVolumeID,
			attr:     map[string]string{attrFileProtocol: "SMB"},
			want:     false,
		},
		{
			name:     "VolumePool, protocol NFS_V3, user options present",
			volumeID: testVolumePoolVolumeID,
			attr:     map[string]string{attrFileProtocol: v3FileProtocol},
			options:  []string{"nconnect=8"},
			want:     false,
		},
		{
			name:     "VolumePool, protocol omitted, user options present",
			volumeID: testVolumePoolVolumeID,
			attr:     map[string]string{},
			options:  []string{"vers=4.1"},
			want:     false,
		},
		{
			name:     "VolumePool, empty (non-nil) options slice",
			volumeID: testVolumePoolVolumeID,
			attr:     map[string]string{attrFileProtocol: v3FileProtocol},
			options:  []string{},
			want:     true,
		},
		{
			name:     "Instance volume, protocol NFS_V3, no options",
			volumeID: testVolumeID,
			attr:     map[string]string{attrFileProtocol: v3FileProtocol},
			want:     false,
		},
		{
			name:     "Instance volume, protocol omitted, no options",
			volumeID: testVolumeID,
			attr:     map[string]string{},
			want:     false,
		},
		{
			name:     "Multishare volume, protocol NFS_V3, no options",
			volumeID: "modeMultishare/test-prefix/test-project/us-central1/test-instance/test-share",
			attr:     map[string]string{attrFileProtocol: v3FileProtocol},
			want:     false,
		},
		{
			name:     "Empty volume ID",
			volumeID: "",
			attr:     map[string]string{attrFileProtocol: v3FileProtocol},
			want:     false,
		},
		{
			name:     "volumepool substring not at prefix",
			volumeID: "modeInstance/volumepool://x",
			attr:     map[string]string{attrFileProtocol: v3FileProtocol},
			want:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldApplyVolumePoolNFSv3DefaultMountOptions(tc.volumeID, tc.attr, tc.options); got != tc.want {
				t.Errorf("shouldApplyVolumePoolNFSv3DefaultMountOptions(%q, %v, %v) = %v, want %v", tc.volumeID, tc.attr, tc.options, got, tc.want)
			}
		})
	}
}

// wantVolumePoolNFSv3DefaultMountOptions is the driver default for VolumePool
// NFSv3 volumes without mount options, for every access mode.
var wantVolumePoolNFSv3DefaultMountOptions = []string{
	"vers=3",
	"proto=tcp",
	"mountproto=tcp",
	"port=2049",
	"mountport=2050",
	"nconnect=8",
	"rsize=524288",
	"wsize=524288",
}

// substrateTemplateMountOptions is the parameters.mount-options value of the
// Substrate StorageClass template: the driver defaults plus nolock and
// actimeo=60 for dedicated single-actor workspaces.
const substrateTemplateMountOptions = "vers=3,proto=tcp,mountproto=tcp,port=2049,mountport=2050,nolock,nconnect=8,rsize=524288,wsize=524288,actimeo=60"

func TestVolumePoolNFSv3DefaultMountOptions(t *testing.T) {
	got := volumePoolNFSv3DefaultMountOptions()
	if diff := cmp.Diff(wantVolumePoolNFSv3DefaultMountOptions, got); diff != "" {
		t.Fatalf("volumePoolNFSv3DefaultMountOptions() mismatch (-want +got):\n%s", diff)
	}
	// nolock and actimeo are only safe when one node mounts the volume, which
	// the access mode doesn't guarantee (attachRequired: false).
	for _, opt := range got {
		if opt == "nolock" || opt == "noac" || strings.HasPrefix(opt, "actimeo=") || strings.HasPrefix(opt, "acreg") || strings.HasPrefix(opt, "acdir") {
			t.Errorf("default mount options contain %q, which is unsafe for volumes mounted on several nodes", opt)
		}
	}

	// Mutating a returned slice must not affect later calls.
	got[0] = "vers=4.1"
	got = append(got, "nocto")
	if diff := cmp.Diff(wantVolumePoolNFSv3DefaultMountOptions, volumePoolNFSv3DefaultMountOptions()); diff != "" {
		t.Errorf("volumePoolNFSv3DefaultMountOptions() changed after caller mutation (-want +got):\n%s", diff)
	}
}

func TestVolumePoolExampleStorageClasses(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	readFile := func(t *testing.T, rel ...string) string {
		t.Helper()
		path := filepath.Join(append([]string{repoRoot}, rel...)...)
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read %s: %v", path, err)
		}
		return string(raw)
	}
	// The template is the driver defaults with nolock after mountport=2050 and
	// actimeo=60 at the end.
	var templateOpts []string
	for _, opt := range wantVolumePoolNFSv3DefaultMountOptions {
		templateOpts = append(templateOpts, opt)
		if opt == "mountport=2050" {
			templateOpts = append(templateOpts, "nolock")
		}
	}
	templateOpts = append(templateOpts, "actimeo=60")
	if wantTemplateOpts := strings.Join(templateOpts, ","); wantTemplateOpts != substrateTemplateMountOptions {
		t.Fatalf("substrateTemplateMountOptions %q is not the driver defaults plus nolock and actimeo=60 (%q)", substrateTemplateMountOptions, strings.Join(templateOpts, ","))
	}

	t.Run("Substrate StorageClass template sets the single-actor parameters.mount-options", func(t *testing.T) {
		raw := readFile(t, "examples", "kubernetes", "substrate", "storageclass.yaml.tmpl")
		rendered := strings.NewReplacer(
			"<PROJECT_ID>", "demo-project",
			"<LOCATION>", "us-central1",
			"<VOLUMEPOOL_NAME>", "demo-pool",
			"<STORAGECLASS_NAME>", "substrate-volumepool-sc",
		).Replace(raw)

		var sc storagev1.StorageClass
		if err := yaml.Unmarshal([]byte(rendered), &sc); err != nil {
			t.Fatalf("failed to unmarshal the rendered template: %v", err)
		}
		if len(sc.MountOptions) != 0 {
			t.Errorf("Substrate StorageClass must not set top-level mountOptions (got %v); Substrate ignores mountOptions", sc.MountOptions)
		}
		if diff := cmp.Diff(substrateTemplateMountOptions, sc.Parameters[paramMountOptions]); diff != "" {
			t.Errorf("parameters[%q] mismatch (-want +got):\n%s", paramMountOptions, diff)
		}
	})

	t.Run("deploy.sh replaces every placeholder in the template", func(t *testing.T) {
		script := readFile(t, "deploy", "kubernetes", "overlays", "substrate", "deploy.sh")
		raw := readFile(t, "examples", "kubernetes", "substrate", "storageclass.yaml.tmpl")
		scSedBlock := regexp.MustCompile(`(?s)SC_MANIFEST="\$\(sed.*?storageclass\.yaml\.tmpl"\)"`).FindString(script)
		if scSedBlock == "" {
			t.Fatal("found no SC_MANIFEST sed block in deploy.sh")
		}
		// Apply deploy.sh's sed replacements, s<delim><NAME><delim>...<delim>g, with dummy values.
		subs := regexp.MustCompile(`s[^A-Za-z0-9\s](<[A-Z0-9_]+>)`).FindAllStringSubmatch(scSedBlock, -1)
		if len(subs) == 0 {
			t.Fatal("found no placeholder replacements in deploy.sh")
		}
		rendered := raw
		for _, m := range subs {
			rendered = strings.ReplaceAll(rendered, m[1], "x")
		}
		// Only the manifest matters; the usage comments mention placeholders.
		var manifest []string
		for _, line := range strings.Split(rendered, "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "#") {
				manifest = append(manifest, line)
			}
		}
		if left := regexp.MustCompile(`<[A-Za-z0-9_]+>`).FindAllString(strings.Join(manifest, "\n"), -1); len(left) > 0 {
			t.Errorf("deploy.sh leaves placeholders %v in the template", left)
		}
	})

	t.Run("Substrate shared StorageClass template sets safe driver defaults", func(t *testing.T) {
		mainRaw := readFile(t, "examples", "kubernetes", "substrate", "storageclass.yaml.tmpl")
		sharedRaw := readFile(t, "examples", "kubernetes", "substrate", "storageclass-shared.yaml.tmpl")

		// Check both templates use the exact same placeholder set in manifest lines.
		manifestPlaceholders := func(raw string) map[string]bool {
			out := map[string]bool{}
			for _, line := range strings.Split(raw, "\n") {
				if strings.HasPrefix(strings.TrimSpace(line), "#") {
					continue
				}
				for _, m := range regexp.MustCompile(`<[A-Za-z0-9_]+>`).FindAllString(line, -1) {
					out[m] = true
				}
			}
			return out
		}
		if diff := cmp.Diff(manifestPlaceholders(mainRaw), manifestPlaceholders(sharedRaw)); diff != "" {
			t.Errorf("placeholder set mismatch between main and shared templates (-main +shared):\n%s", diff)
		}

		replacer := strings.NewReplacer(
			"<PROJECT_ID>", "demo-project",
			"<LOCATION>", "us-central1",
			"<VOLUMEPOOL_NAME>", "demo-pool",
			"<STORAGECLASS_NAME>", "substrate-volumepool-shared-sc",
		)
		var mainSC, sharedSC storagev1.StorageClass
		if err := yaml.Unmarshal([]byte(replacer.Replace(mainRaw)), &mainSC); err != nil {
			t.Fatalf("failed to unmarshal main template: %v", err)
		}
		if err := yaml.Unmarshal([]byte(replacer.Replace(sharedRaw)), &sharedSC); err != nil {
			t.Fatalf("failed to unmarshal shared template: %v", err)
		}
		if len(sharedSC.MountOptions) != 0 {
			t.Errorf("Substrate shared StorageClass must not set top-level mountOptions (got %v); Substrate ignores mountOptions", sharedSC.MountOptions)
		}
		wantDefaults := strings.Join(wantVolumePoolNFSv3DefaultMountOptions, ",")
		if diff := cmp.Diff(wantDefaults, sharedSC.Parameters[paramMountOptions]); diff != "" {
			t.Errorf("parameters[%q] mismatch (-want +got):\n%s", paramMountOptions, diff)
		}
		if sharedSC.Provisioner != mainSC.Provisioner {
			t.Errorf("provisioner = %q, want %q", sharedSC.Provisioner, mainSC.Provisioner)
		}
		if diff := cmp.Diff(mainSC.ReclaimPolicy, sharedSC.ReclaimPolicy); diff != "" {
			t.Errorf("reclaimPolicy mismatch (-main +shared):\n%s", diff)
		}
		if diff := cmp.Diff(mainSC.VolumeBindingMode, sharedSC.VolumeBindingMode); diff != "" {
			t.Errorf("volumeBindingMode mismatch (-main +shared):\n%s", diff)
		}
		if diff := cmp.Diff(mainSC.AllowVolumeExpansion, sharedSC.AllowVolumeExpansion); diff != "" {
			t.Errorf("allowVolumeExpansion mismatch (-main +shared):\n%s", diff)
		}
	})

	t.Run("README carries the template and default mount options", func(t *testing.T) {
		readme := readFile(t, "deploy", "kubernetes", "overlays", "substrate", "README.md")
		for _, want := range []string{
			substrateTemplateMountOptions,
			strings.Join(wantVolumePoolNFSv3DefaultMountOptions, ","),
			"storageclass-shared.yaml.tmpl",
			"storageclass-single-node.yaml",
			"pvc-single-node.yaml",
			"examples/kubernetes/volumepool/storageclass.yaml",
		} {
			if !strings.Contains(readme, want) {
				t.Errorf("README.md doesn't contain %q", want)
			}
		}
	})

	t.Run("VolumePool StorageClass example sets the default mountOptions", func(t *testing.T) {
		var sc storagev1.StorageClass
		if err := yaml.Unmarshal([]byte(readFile(t, "examples", "kubernetes", "volumepool", "storageclass.yaml")), &sc); err != nil {
			t.Fatalf("failed to unmarshal the example: %v", err)
		}
		if diff := cmp.Diff(wantVolumePoolNFSv3DefaultMountOptions, sc.MountOptions); diff != "" {
			t.Errorf("mountOptions mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("VolumePool single-node StorageClass and PVC example match ReadWriteOncePod", func(t *testing.T) {
		var baseSC, singleSC storagev1.StorageClass
		if err := yaml.Unmarshal([]byte(readFile(t, "examples", "kubernetes", "volumepool", "storageclass.yaml")), &baseSC); err != nil {
			t.Fatalf("failed to unmarshal base example: %v", err)
		}
		if err := yaml.Unmarshal([]byte(readFile(t, "examples", "kubernetes", "volumepool", "storageclass-single-node.yaml")), &singleSC); err != nil {
			t.Fatalf("failed to unmarshal single-node example: %v", err)
		}
		if singleSC.Name != "volume-pool-single-node-sc" {
			t.Errorf("single-node StorageClass name = %q, want %q", singleSC.Name, "volume-pool-single-node-sc")
		}
		if singleSC.Provisioner != baseSC.Provisioner {
			t.Errorf("provisioner = %q, want %q", singleSC.Provisioner, baseSC.Provisioner)
		}
		if diff := cmp.Diff(baseSC.ReclaimPolicy, singleSC.ReclaimPolicy); diff != "" {
			t.Errorf("reclaimPolicy mismatch (-base +single):\n%s", diff)
		}
		if diff := cmp.Diff(baseSC.VolumeBindingMode, singleSC.VolumeBindingMode); diff != "" {
			t.Errorf("volumeBindingMode mismatch (-base +single):\n%s", diff)
		}
		if diff := cmp.Diff(templateOpts, singleSC.MountOptions); diff != "" {
			t.Errorf("single-node mountOptions mismatch (-want +got):\n%s", diff)
		}

		var pvc corev1.PersistentVolumeClaim
		if err := yaml.Unmarshal([]byte(readFile(t, "examples", "kubernetes", "volumepool", "pvc-single-node.yaml")), &pvc); err != nil {
			t.Fatalf("failed to unmarshal single-node PVC: %v", err)
		}
		if pvc.Name != "volumepool-single-node-pvc" {
			t.Errorf("single-node PVC name = %q, want %q", pvc.Name, "volumepool-single-node-pvc")
		}
		if diff := cmp.Diff([]corev1.PersistentVolumeAccessMode{corev1.ReadWriteOncePod}, pvc.Spec.AccessModes); diff != "" {
			t.Errorf("single-node PVC accessModes mismatch (-want +got):\n%s", diff)
		}
		if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName != singleSC.Name {
			t.Errorf("single-node PVC storageClassName = %v, want %q", pvc.Spec.StorageClassName, singleSC.Name)
		}
	})
}

func TestNodeStageVolume_VolumePoolMountOptions(t *testing.T) {
	expectedDevice := "10.1.1.1:/vol1"
	defaultOpts := wantVolumePoolNFSv3DefaultMountOptions

	tests := []struct {
		name              string
		volumeID          string
		accessMode        csi.VolumeCapability_AccessMode_Mode
		mountFlags        []string
		volumeContext     map[string]string
		enableNFSv4       bool
		expectedMountOpts []string
	}{
		// Defaults are applied.
		{
			name:              "VolumePool NFS_V3 single-node writer, NFSv4 feature disabled: defaults applied",
			volumeID:          testVolumePoolVolumeID,
			accessMode:        csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			volumeContext:     map[string]string{attrIP: "10.1.1.1", attrVolume: "vol1", attrFileProtocol: v3FileProtocol},
			expectedMountOpts: defaultOpts,
		},
		{
			name:              "VolumePool NFS_V3 single-node writer, NFSv4 feature enabled: defaults applied",
			volumeID:          testVolumePoolVolumeID,
			accessMode:        csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			volumeContext:     map[string]string{attrIP: "10.1.1.1", attrVolume: "vol1", attrFileProtocol: v3FileProtocol},
			enableNFSv4:       true,
			expectedMountOpts: defaultOpts,
		},
		{
			name:              "VolumePool NFS_V3 single-node reader: defaults applied",
			volumeID:          testVolumePoolVolumeID,
			accessMode:        csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY,
			volumeContext:     map[string]string{attrIP: "10.1.1.1", attrVolume: "vol1", attrFileProtocol: v3FileProtocol},
			expectedMountOpts: defaultOpts,
		},
		{
			name:              "VolumePool NFS_V3 multi-node multi-writer (RWX): same defaults",
			volumeID:          testVolumePoolVolumeID,
			accessMode:        csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
			volumeContext:     map[string]string{attrIP: "10.1.1.1", attrVolume: "vol1", attrFileProtocol: v3FileProtocol},
			expectedMountOpts: defaultOpts,
		},
		{
			name:              "VolumePool NFS_V3 multi-node reader-only (ROX): same defaults",
			volumeID:          testVolumePoolVolumeID,
			accessMode:        csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY,
			volumeContext:     map[string]string{attrIP: "10.1.1.1", attrVolume: "vol1", attrFileProtocol: v3FileProtocol},
			expectedMountOpts: defaultOpts,
		},
		{
			name:              "VolumePool NFS_V3 multi-node single-writer: same defaults",
			volumeID:          testVolumePoolVolumeID,
			accessMode:        csi.VolumeCapability_AccessMode_MULTI_NODE_SINGLE_WRITER,
			volumeContext:     map[string]string{attrIP: "10.1.1.1", attrVolume: "vol1", attrFileProtocol: v3FileProtocol},
			expectedMountOpts: defaultOpts,
		},
		{
			name:              "VolumePool NFS_V3 Substrate template options are passed through",
			volumeID:          testVolumePoolVolumeID,
			accessMode:        csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			volumeContext:     map[string]string{attrIP: "10.1.1.1", attrVolume: "vol1", attrFileProtocol: v3FileProtocol, attrMountOptions: substrateTemplateMountOptions},
			expectedMountOpts: []string{substrateTemplateMountOptions},
		},
		{
			name:              "VolumePool protocol omitted, NFSv4 feature disabled: defaults applied",
			volumeID:          testVolumePoolVolumeID,
			accessMode:        csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			volumeContext:     map[string]string{attrIP: "10.1.1.1", attrVolume: "vol1"},
			expectedMountOpts: defaultOpts,
		},
		{
			name:              "VolumePool protocol omitted, NFSv4 feature enabled: defaults applied",
			volumeID:          testVolumePoolVolumeID,
			accessMode:        csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			volumeContext:     map[string]string{attrIP: "10.1.1.1", attrVolume: "vol1"},
			enableNFSv4:       true,
			expectedMountOpts: defaultOpts,
		},
		{
			name:              "VolumePool lowercase nfs_v3: defaults applied",
			volumeID:          testVolumePoolVolumeID,
			accessMode:        csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			volumeContext:     map[string]string{attrIP: "10.1.1.1", attrVolume: "vol1", attrFileProtocol: "nfs_v3"},
			expectedMountOpts: defaultOpts,
		},
		{
			name:              "VolumePool empty mount-options parameter: defaults applied",
			volumeID:          testVolumePoolVolumeID,
			accessMode:        csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			volumeContext:     map[string]string{attrIP: "10.1.1.1", attrVolume: "vol1", attrFileProtocol: v3FileProtocol, attrMountOptions: ""},
			expectedMountOpts: defaultOpts,
		},
		// User options are respected as-is.
		{
			name:              "VolumePool respects StorageClass.mountOptions (MountFlags)",
			volumeID:          testVolumePoolVolumeID,
			accessMode:        csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			mountFlags:        []string{"vers=3", "nolock"},
			volumeContext:     map[string]string{attrIP: "10.1.1.1", attrVolume: "vol1", attrFileProtocol: v3FileProtocol},
			expectedMountOpts: []string{"vers=3", "nolock"},
		},
		{
			name:              "VolumePool respects StorageClass.parameters[mount-options] (attrMountOptions)",
			volumeID:          testVolumePoolVolumeID,
			accessMode:        csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			volumeContext:     map[string]string{attrIP: "10.1.1.1", attrVolume: "vol1", attrFileProtocol: v3FileProtocol, attrMountOptions: "nconnect=8,rsize=1048576,wsize=1048576"},
			expectedMountOpts: []string{"nconnect=8,rsize=1048576,wsize=1048576"},
		},
		{
			name:              "VolumePool respects both MountFlags and attrMountOptions",
			volumeID:          testVolumePoolVolumeID,
			accessMode:        csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			mountFlags:        []string{"nolock"},
			volumeContext:     map[string]string{attrIP: "10.1.1.1", attrVolume: "vol1", attrFileProtocol: v3FileProtocol, attrMountOptions: "nconnect=8"},
			expectedMountOpts: []string{"nolock", "nconnect=8"},
		},
		{
			name:              "VolumePool protocol omitted respects MountFlags",
			volumeID:          testVolumePoolVolumeID,
			accessMode:        csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			mountFlags:        []string{"hard"},
			volumeContext:     map[string]string{attrIP: "10.1.1.1", attrVolume: "vol1"},
			expectedMountOpts: []string{"hard"},
		},
		{
			name:              "VolumePool NFS_V3 with NFSv4 feature enabled respects MountFlags",
			volumeID:          testVolumePoolVolumeID,
			accessMode:        csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			mountFlags:        []string{"hard"},
			volumeContext:     map[string]string{attrIP: "10.1.1.1", attrVolume: "vol1", attrFileProtocol: v3FileProtocol},
			enableNFSv4:       true,
			expectedMountOpts: []string{"hard"},
		},
		// NFSv4.1 never gets NFSv3 defaults.
		{
			name:              "VolumePool NFS_V4_1, NFSv4 feature enabled: no defaults",
			volumeID:          testVolumePoolVolumeID,
			accessMode:        csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			volumeContext:     map[string]string{attrIP: "10.1.1.1", attrVolume: "vol1", attrFileProtocol: v4_1FileProtocol},
			enableNFSv4:       true,
			expectedMountOpts: []string{},
		},
		{
			name:              "VolumePool NFS_V4_1, NFSv4 feature disabled: no defaults",
			volumeID:          testVolumePoolVolumeID,
			accessMode:        csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			volumeContext:     map[string]string{attrIP: "10.1.1.1", attrVolume: "vol1", attrFileProtocol: v4_1FileProtocol},
			expectedMountOpts: []string{},
		},
		{
			name:              "VolumePool NFS_V4_1 respects MountFlags",
			volumeID:          testVolumePoolVolumeID,
			accessMode:        csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			mountFlags:        []string{"vers=4.1"},
			volumeContext:     map[string]string{attrIP: "10.1.1.1", attrVolume: "vol1", attrFileProtocol: v4_1FileProtocol},
			enableNFSv4:       true,
			expectedMountOpts: []string{"vers=4.1"},
		},
		// Non-VolumePool volumes are unchanged.
		{
			name:              "Instance volume, protocol omitted: no defaults",
			volumeID:          testVolumeID,
			accessMode:        csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			volumeContext:     map[string]string{attrIP: "10.1.1.1", attrVolume: "vol1"},
			expectedMountOpts: []string{},
		},
		{
			name:              "Instance volume, explicit NFS_V3: no defaults",
			volumeID:          testVolumeID,
			accessMode:        csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			volumeContext:     map[string]string{attrIP: "10.1.1.1", attrVolume: "vol1", attrFileProtocol: v3FileProtocol},
			expectedMountOpts: []string{},
		},
		{
			name:              "Instance volume, explicit NFS_V3, NFSv4 feature enabled: no defaults",
			volumeID:          testVolumeID,
			accessMode:        csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			volumeContext:     map[string]string{attrIP: "10.1.1.1", attrVolume: "vol1", attrFileProtocol: v3FileProtocol},
			enableNFSv4:       true,
			expectedMountOpts: []string{},
		},
		{
			name:              "Instance volume respects MountFlags",
			volumeID:          testVolumeID,
			accessMode:        csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			mountFlags:        []string{"nolock"},
			volumeContext:     map[string]string{attrIP: "10.1.1.1", attrVolume: "vol1", attrFileProtocol: v3FileProtocol},
			expectedMountOpts: []string{"nolock"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testEnv := initTestNodeServer(t)
			ns := testEnv.ns.(*nodeServer)
			ns.features.FeatureNFSv4Support = &FeatureNFSv4Support{Enabled: tc.enableNFSv4}

			stagingTargetPath := filepath.Join(t.TempDir(), "staging")
			stageReq := &csi.NodeStageVolumeRequest{
				VolumeId:          tc.volumeID,
				StagingTargetPath: stagingTargetPath,
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							MountFlags: tc.mountFlags,
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: tc.accessMode,
					},
				},
				VolumeContext: tc.volumeContext,
			}

			if _, err := testEnv.ns.NodeStageVolume(context.Background(), stageReq); err != nil {
				t.Fatalf("NodeStageVolume failed: %v", err)
			}

			validateMountPoint(t, tc.name, testEnv.fm, &mount.MountPoint{
				Device: expectedDevice,
				Path:   stagingTargetPath,
				Type:   "nfs",
				Opts:   tc.expectedMountOpts,
			})

			// NodeUnstageVolume does not depend on mount options; verify the staged
			// mount is still cleaned up normally.
			unstageReq := &csi.NodeUnstageVolumeRequest{
				VolumeId:          tc.volumeID,
				StagingTargetPath: stagingTargetPath,
			}
			if _, err := testEnv.ns.NodeUnstageVolume(context.Background(), unstageReq); err != nil {
				t.Fatalf("NodeUnstageVolume failed: %v", err)
			}
			validateMountPoint(t, tc.name+" (after unstage)", testEnv.fm, nil)
		})
	}
}

func TestNodeStageVolume_VolumePoolAlreadyMounted(t *testing.T) {
	testEnv := initTestNodeServer(t)
	ns := testEnv.ns.(*nodeServer)
	ns.features.FeatureNFSv4Support = &FeatureNFSv4Support{Enabled: false}

	stagingTargetPath := filepath.Join(t.TempDir(), "staging")
	if err := os.MkdirAll(stagingTargetPath, 0750); err != nil {
		t.Fatalf("failed to create staging path: %v", err)
	}
	existing := mount.MountPoint{
		Device: "10.1.1.1:/vol1",
		Path:   stagingTargetPath,
		Type:   "nfs",
		Opts:   []string{"existing-opt"},
	}
	testEnv.fm.MountPoints = []mount.MountPoint{existing}

	stageReq := &csi.NodeStageVolumeRequest{
		VolumeId:          testVolumePoolVolumeID,
		StagingTargetPath: stagingTargetPath,
		VolumeCapability: &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Mount{
				Mount: &csi.VolumeCapability_MountVolume{},
			},
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			},
		},
		VolumeContext: map[string]string{attrIP: "10.1.1.1", attrVolume: "vol1", attrFileProtocol: v3FileProtocol},
	}

	if _, err := testEnv.ns.NodeStageVolume(context.Background(), stageReq); err != nil {
		t.Fatalf("NodeStageVolume failed: %v", err)
	}

	// The existing mount must be left untouched: no remount and no default options injected.
	validateMountPoint(t, "already mounted", testEnv.fm, &existing)
}

func TestNodeUnstageVolumeUpdateLockInfo(t *testing.T) {
	basePath, err := ioutil.TempDir("", "node-publish-")
	if err != nil {
		t.Fatalf("failed to setup testdir: %v", err)
	}
	stagingTargetPath := filepath.Join(basePath, "staging")
	cases := []struct {
		name       string
		req        *csi.NodeUnstageVolumeRequest
		existingCM *corev1.ConfigMap
		expectedCM *corev1.ConfigMap
		expectErr  bool
	}{
		{
			name: "configmap for the current node exists, key exists in configmap",
			req: &csi.NodeUnstageVolumeRequest{
				VolumeId:          testVolumeID,
				StagingTargetPath: stagingTargetPath,
			},
			existingCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "fscsi-test-node",
					Namespace:  util.ManagedFilestoreCSINamespace,
					Finalizers: []string{lockrelease.ConfigMapFinalzer},
				},
				Data: map[string]string{
					"test-project.us-central1-c.test-filestore.vol1.123456.127_0_0_1": "1.1.1.2",
					"test-project.us-central1-c.test-csi.vol1.123456.127_0_0_1":       "1.1.1.1",
				},
			},
			expectedCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "fscsi-test-node",
					Namespace:  util.ManagedFilestoreCSINamespace,
					Finalizers: []string{lockrelease.ConfigMapFinalzer},
				},
				Data: map[string]string{
					"test-project.us-central1-c.test-filestore.vol1.123456.127_0_0_1": "1.1.1.2",
				},
			},
		},
		{
			name: "configmap for the current node exists, key not exists in configmap",
			req: &csi.NodeUnstageVolumeRequest{
				VolumeId:          testVolumeID,
				StagingTargetPath: stagingTargetPath,
			},
			existingCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "fscsi-test-node",
					Namespace:  util.ManagedFilestoreCSINamespace,
					Finalizers: []string{lockrelease.ConfigMapFinalzer},
				},
				Data: map[string]string{
					"test-project.us-central1-c.test-filestore.vol1.123456.127_0_0_1": "1.1.1.1",
				},
			},
			expectedCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "fscsi-test-node",
					Namespace:  util.ManagedFilestoreCSINamespace,
					Finalizers: []string{lockrelease.ConfigMapFinalzer},
				},
				Data: map[string]string{
					"test-project.us-central1-c.test-filestore.vol1.123456.127_0_0_1": "1.1.1.1",
				},
			},
		},
		{
			name: "configmap exists, key exists, configmap empty after removing the key",
			req: &csi.NodeUnstageVolumeRequest{
				VolumeId:          testVolumeID,
				StagingTargetPath: stagingTargetPath,
			},
			existingCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "fscsi-test-node",
					Namespace:  util.ManagedFilestoreCSINamespace,
					Finalizers: []string{lockrelease.ConfigMapFinalzer},
				},
				Data: map[string]string{
					"test-project.us-central1-c.test-csi.vol1.123456.127_0_0_1": "1.1.1.1",
				},
			},
			expectedCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "fscsi-test-node",
					Namespace:  util.ManagedFilestoreCSINamespace,
					Finalizers: []string{lockrelease.ConfigMapFinalzer},
				},
				Data: map[string]string{},
			},
		},
	}
	for _, test := range cases {
		client := fake.NewSimpleClientset(test.existingCM)
		server := initTestNodeServerWithKubeClient(t, client)
		ctx := context.Background()
		err := server.nodeUnstageVolumeUpdateLockInfo(ctx, test.req)
		if gotExpected := gotExpectedError(test.name, test.expectErr, err); gotExpected != nil {
			t.Fatal(gotExpected)
		}
		cm, err := client.CoreV1().ConfigMaps(test.existingCM.Namespace).Get(ctx, test.existingCM.Name, metav1.GetOptions{})
		if test.expectedCM == nil {
			if !apiError.IsNotFound(err) {
				t.Fatalf("test %q failed: unexpected error %v", test.name, err)
			}
			if cm != nil {
				t.Fatalf("test %q failed: expecting %v to be deleted", test.name, *cm)
			}
			return
		}
		if err != nil {
			t.Fatalf("test %q failed: unexpected error %v", test.name, err)
		}
		if test.expectedCM == nil {
			t.Fatalf("test %q failed: expecting %v to be deleted", test.name, *cm)
		}
		if diff := cmp.Diff(test.expectedCM, cm); diff != "" {
			t.Errorf("test %q failed: unexpected diff (-want +got):\n%s", test.name, diff)
		}
	}
}

func gotExpectedError(testFunc string, wantErr bool, err error) error {
	if err != nil && !wantErr {
		return fmt.Errorf("%s got error %v, want nil", testFunc, err)
	}
	if err == nil && wantErr {
		return fmt.Errorf("%s got nil, want error", testFunc)
	}
	return nil
}
