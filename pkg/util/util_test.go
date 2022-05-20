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

package util

import (
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
)

const (
	testRegion       = "us-central1"
	testInstanceName = "testInstance"
	testShareName    = "testShare"
	testProject      = "test-project"
)

func TestRoundBytesToGb(t *testing.T) {
	cases := []struct {
		name     string
		bytes    int64
		expected int64
	}{
		{
			name:     "exact",
			bytes:    10 * Gb,
			expected: 10,
		},
		{
			name:     "below",
			bytes:    10*Gb - 1,
			expected: 10,
		},
		{
			name:     "above",
			bytes:    10*Gb + 1,
			expected: 11,
		},
	}

	for _, test := range cases {
		ret := RoundBytesToGb(test.bytes)
		if ret != test.expected {
			t.Errorf("test %q failed: got %v, expected %v", test.name, ret, test.expected)
		}
	}
}

func TestGbToBytes(t *testing.T) {
	cases := []struct {
		name     string
		gbs      int64
		expected int64
	}{
		{
			name:     "1gb",
			gbs:      1,
			expected: 1024 * 1024 * 1024,
		},
		{
			name:     "5gb",
			gbs:      5,
			expected: 5 * 1024 * 1024 * 1024,
		},
	}

	for _, test := range cases {
		ret := GbToBytes(test.gbs)
		if ret != test.expected {
			t.Errorf("test %q failed: got %v, expected %v", test.name, ret, test.expected)
		}
	}
}

func TestConvertLabelsStringToMap(t *testing.T) {
	t.Run("parsing labels string into map", func(t *testing.T) {
		testCases := []struct {
			name           string
			labels         string
			expectedOutput map[string]string
			expectedError  bool
		}{
			// Success test cases
			{
				name:           "should return empty map when labels string is empty",
				labels:         "",
				expectedOutput: map[string]string{},
				expectedError:  false,
			},
			{
				name:   "single label string",
				labels: "key=value",
				expectedOutput: map[string]string{
					"key": "value",
				},
				expectedError: false,
			},
			{
				name:   "multiple label string",
				labels: "key1=value1,key2=value2",
				expectedOutput: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
				expectedError: false,
			},
			{
				name:   "multiple labels string with whitespaces gets trimmed",
				labels: "key1=value1, key2=value2",
				expectedOutput: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
				expectedError: false,
			},
			// Failure test cases
			{
				name:           "malformed labels string (no keys and values)",
				labels:         ",,",
				expectedOutput: nil,
				expectedError:  true,
			},
			{
				name:           "malformed labels string (incorrect format)",
				labels:         "foo,bar",
				expectedOutput: nil,
				expectedError:  true,
			},
			{
				name:           "malformed labels string (missing key)",
				labels:         "key1=value1,=bar",
				expectedOutput: nil,
				expectedError:  true,
			},
			{
				name:           "malformed labels string (missing key and value)",
				labels:         "key1=value1,=bar,=",
				expectedOutput: nil,
				expectedError:  true,
			},
		}

		for _, tc := range testCases {
			t.Logf("test case: %s", tc.name)
			output, err := ConvertLabelsStringToMap(tc.labels)
			if tc.expectedError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if err != nil {
				if !tc.expectedError {
					t.Errorf("Did not expect error but got: %v", err)
				}
				continue
			}

			if !reflect.DeepEqual(output, tc.expectedOutput) {
				t.Errorf("Got labels %v, but expected %v", output, tc.expectedOutput)
			}
		}
	})

	t.Run("checking google requirements", func(t *testing.T) {
		testCases := []struct {
			name          string
			labels        string
			expectedError bool
		}{
			{
				name: "64 labels at most",
				labels: `k1=v,k2=v,k3=v,k4=v,k5=v,k6=v,k7=v,k8=v,k9=v,k10=v,k11=v,k12=v,k13=v,k14=v,k15=v,k16=v,k17=v,k18=v,k19=v,k20=v,
                         k21=v,k22=v,k23=v,k24=v,k25=v,k26=v,k27=v,k28=v,k29=v,k30=v,k31=v,k32=v,k33=v,k34=v,k35=v,k36=v,k37=v,k38=v,k39=v,k40=v,
                         k41=v,k42=v,k43=v,k44=v,k45=v,k46=v,k47=v,k48=v,k49=v,k50=v,k51=v,k52=v,k53=v,k54=v,k55=v,k56=v,k57=v,k58=v,k59=v,k60=v,
                         k61=v,k62=v,k63=v,k64=v,k65=v`,
				expectedError: true,
			},
			{
				name:          "label key must have atleast 1 char",
				labels:        "=v",
				expectedError: true,
			},
			{
				name:          "label key can only contain lowercase chars, digits, _ and -)",
				labels:        "k*=v",
				expectedError: true,
			},
			{
				name:          "label key can only contain lowercase chars)",
				labels:        "K=v",
				expectedError: true,
			},
			{
				name:          "label key may not have over 63 characters",
				labels:        "abcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghij1234=v",
				expectedError: true,
			},
			{
				name:          "label value can only contain lowercase chars, digits, _ and -)",
				labels:        "k1=###",
				expectedError: true,
			},
			{
				name:          "label value can only contain lowercase chars)",
				labels:        "k1=V",
				expectedError: true,
			},
			{
				name:          "label key cannot contain . and /",
				labels:        "kubernetes.io/created-for/pvc/namespace=v",
				expectedError: true,
			},
			{
				name:          "label value cannot contain . and /",
				labels:        "kubernetes_io_created-for_pvc_namespace=v./",
				expectedError: true,
			},
			{
				name:          "label value may not have over 63 chars",
				labels:        "v=abcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghij1234",
				expectedError: true,
			},
			{
				name:          "label key can have up to 63 chars",
				labels:        "abcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghij123=v",
				expectedError: false,
			},
			{
				name:          "label value can have up to 63 chars",
				labels:        "k=abcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghij123",
				expectedError: false,
			},
			{
				name:          "label key can contain - and _",
				labels:        "abcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghij-_=v",
				expectedError: false,
			},
			{
				name:          "label value can contain - and _",
				labels:        "k=abcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghij-_",
				expectedError: false,
			},
			{
				name:          "label value can have 0 chars",
				labels:        "kubernetes_io_created-for_pvc_namespace=",
				expectedError: false,
			},
		}

		for _, tc := range testCases {
			t.Logf("test case: %s", tc.name)
			_, err := ConvertLabelsStringToMap(tc.labels)

			if tc.expectedError && err == nil {
				t.Errorf("Expected error but got none")
			}

			if !tc.expectedError && err != nil {
				t.Errorf("Did not expect error but got: %v", err)
			}
		}
	})

}

func TestConvertVolToShareName(t *testing.T) {
	testuuid := uuid.New().String()
	tests := []struct {
		name      string
		volName   string
		shareName string
	}{
		{
			name:      "tc1 - all caps",
			volName:   "PVC",
			shareName: "pvc",
		},
		{
			name:      "tc1 - caps and UUID",
			volName:   "PVC-" + testuuid,
			shareName: "pvc_" + strings.ReplaceAll(testuuid, "-", "_"),
		},
		{
			name:      "tc1 - lower and UUID",
			volName:   "pvc-" + testuuid,
			shareName: "pvc_" + strings.ReplaceAll(testuuid, "-", "_"),
		},
		{
			name:      "tc1 - caps and number",
			volName:   "pvc-" + "123",
			shareName: "pvc_" + "123",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := ConvertVolToShareName(tc.volName)
			if s != tc.shareName {
				t.Errorf("got %v, want %v", s, tc.shareName)
			}
		})
	}

}

func TestParseInstanceURI(t *testing.T) {
	tests := []struct {
		name         string
		instanceuri  string
		expectErr    bool
		project      string
		location     string
		instancename string
	}{
		{
			name:      "empty uri",
			expectErr: true,
		},
		{
			name:        "invalid uri 1",
			instanceuri: "a/b/c/d",
			expectErr:   true,
		},
		{
			name:        "invalid uri",
			instanceuri: testProject + "/" + testRegion + "/" + testInstanceName,
			expectErr:   true,
		},
		{
			name:         "invalid uri",
			instanceuri:  "projects/" + testProject + "/locations/" + testRegion + "/instances/" + testInstanceName,
			project:      testProject,
			location:     testRegion,
			instancename: testInstanceName,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, l, n, err := ParseInstanceURI(tc.instanceuri)
			if !tc.expectErr && err != nil {
				t.Error("unexpected error")
			}
			if tc.expectErr && err == nil {
				t.Error("expected error, got none")
			}
			if p != tc.project || l != tc.location || n != tc.instancename {
				t.Errorf("mismatch")
			}
		})
	}
}

func TestParseShareURI(t *testing.T) {
	tests := []struct {
		name         string
		shareuri     string
		expectErr    bool
		project      string
		location     string
		instancename string
		sharename    string
	}{
		{
			name:      "invalid uri",
			shareuri:  "a/b/c/d/e",
			expectErr: true,
		},
		{
			name:      "empty uri",
			expectErr: true,
		},
		{
			name:         "valid uri",
			shareuri:     "projects/" + testProject + "/locations/" + testRegion + "/instances/" + testInstanceName + "/shares/" + testShareName,
			project:      testProject,
			location:     testRegion,
			instancename: testInstanceName,
			sharename:    testShareName,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, l, n, s, err := ParseShareURI(tc.shareuri)
			if !tc.expectErr && err != nil {
				t.Error("unexpected error")
			}
			if tc.expectErr && err == nil {
				t.Error("expected error, got none")
			}
			if p != tc.project || l != tc.location || n != tc.instancename || s != tc.sharename {
				t.Errorf("mismatch")
			}
		})
	}

}

func TestAlignBytes(t *testing.T) {
	tests := []struct {
		name        string
		inputBytes  int64
		stepBytes   int64
		targetBytes int64
	}{
		{
			name:        "0 step",
			inputBytes:  100 * Gb,
			targetBytes: 100 * Gb,
		},
		{
			name:        "input less than step size",
			stepBytes:   256 * Gb,
			inputBytes:  100,
			targetBytes: 256 * Gb,
		},
		{
			name:        "aligned input test case 1",
			stepBytes:   256 * Gb,
			inputBytes:  2 * 256 * Gb,
			targetBytes: 2 * 256 * Gb,
		},
		{
			name:        "aligned input test case 2",
			stepBytes:   256 * Gb,
			inputBytes:  1 * Tb,
			targetBytes: 1 * Tb,
		},
		{
			name:        "aligned input test case 3",
			stepBytes:   256 * Gb,
			inputBytes:  10 * Tb,
			targetBytes: 10 * Tb,
		},
		{
			name:        "misaligned input test case 1",
			stepBytes:   256 * Gb,
			inputBytes:  256*Gb + 1,
			targetBytes: 256 * 2 * Gb,
		},
		{
			name:        "misaligned input test case 2",
			stepBytes:   256 * Gb,
			inputBytes:  1*Tb + 1,
			targetBytes: 1*Tb + 256*Gb,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := AlignBytes(tc.inputBytes, tc.stepBytes)
			if got != tc.targetBytes {
				t.Errorf("got %d bytes, expected %d bytes", got, tc.targetBytes)
			}
		})
	}

}
