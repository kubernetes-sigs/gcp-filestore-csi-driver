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

import "testing"

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
