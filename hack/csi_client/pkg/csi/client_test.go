package csi

import (
	"testing"
)

var (
	testHost     = "localhost"
	testVolume   = "test-volume"
	testPath     = "/tmp/test"
	testUsername = "foo"
	testPassword = "bar"
)

func TestValidateRequest(t *testing.T) {
	defaultOsString := goOs
	cases := []struct {
		name      string
		request   *Request
		expectErr bool
		osString  string
	}{
		{
			name:      "invalid request: no request type",
			request:   &Request{},
			expectErr: true,
		},
		{
			name: "valid publish request on Linux",
			request: &Request{
				RequestType: reqNodePublish,
				ShareAddr:   testHost,
				ShareName:   testVolume,
				TargetPath:  testPath,
			},
		},
		{
			name:     "valid publish request on Windows",
			osString: "windows",
			request: &Request{
				RequestType: reqNodePublish,
				ShareAddr:   testHost,
				ShareName:   testVolume,
				TargetPath:  testPath,
				Username:    testUsername,
				Password:    testPassword,
			},
		},
		{
			name: "invalid publish request: no host",
			request: &Request{
				RequestType: reqNodePublish,
				ShareName:   testVolume,
				TargetPath:  testPath,
			},
			expectErr: true,
		},
		{
			name: "invalid publish request: no volume",
			request: &Request{
				RequestType: reqNodePublish,
				ShareAddr:   testHost,
				TargetPath:  testPath,
			},
			expectErr: true,
		},
		{
			name: "invalid publish request: no target path",
			request: &Request{
				RequestType: reqNodePublish,
				ShareAddr:   testHost,
				ShareName:   testVolume,
			},
			expectErr: true,
		},
		{
			name:     "invalid publish request on Windows: no username",
			osString: "windows",
			request: &Request{
				RequestType: reqNodePublish,
				ShareAddr:   testHost,
				ShareName:   testVolume,
				TargetPath:  testPath,
				Password:    testPassword,
			},
			expectErr: true,
		},
		{
			name:     "invalid publish request on Windows: no password",
			osString: "windows",
			request: &Request{
				RequestType: reqNodePublish,
				ShareAddr:   testHost,
				ShareName:   testVolume,
				TargetPath:  testPath,
				Username:    testUsername,
			},
			expectErr: true,
		},
		{
			name: "valid unpublish request",
			request: &Request{
				RequestType: reqNodeUnpublish,
				TargetPath:  testPath,
			},
		},
		{
			name: "invalid unpublish request",
			request: &Request{
				RequestType: reqNodeUnpublish,
			},
			expectErr: true,
		},
	}

	for _, test := range cases {
		if test.osString != "" {
			goOs = test.osString
		} else {
			goOs = defaultOsString
		}
		err := validateRequest(test.request)
		if !test.expectErr && err != nil {
			t.Errorf("test %q failed: %v", test.name, err)
		}

		if test.expectErr && err == nil {
			t.Errorf("test %q failed: got success", test.name)
		}
	}
}
