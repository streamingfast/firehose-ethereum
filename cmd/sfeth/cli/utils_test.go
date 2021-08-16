// Copyright 2021 dfuse Platform Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

//"/storage/megered-blocks"
//:// -> assume passit dreiclty
//NO -> "/" directly
//relative + datadir

func Test_getDirsToMake(t *testing.T) {
	tests := []struct {
		name       string
		storeURL   string
		expectDirs []string
	}{
		{
			name:       "google storage path",
			storeURL:   "gs://test-bucket/eos-local/v1",
			expectDirs: nil,
		},
		{
			name:       "relative local path",
			storeURL:   "myapp/blocks",
			expectDirs: []string{"myapp/blocks"},
		},
		{
			name:       "absolute local path",
			storeURL:   "/data/myapp/blocks",
			expectDirs: []string{"/data/myapp/blocks"},
		},
	}

	for _, test := range tests {
		t.Run(t.Name(), func(t *testing.T) {
			assert.Equal(t, test.expectDirs, getDirsToMake(test.storeURL))
		})
	}

}

//func TestGethVersion_NewFromString(t *testing.T) {
//	tests := []struct {
//		name        string
//		inVersion   string
//		inHelp      string
//		expected    gethVersion
//		expectedErr error
//	}{
//		{"standard no suffix", "Version: 2.0.5", "", gethVersion{"2.0.5", 2, 0, 5, "", false}, nil},
//		{"standard with suffix then dash", "Version: 2.0.5-beta-1", "", gethVersion{"2.0.5-beta-1", 2, 0, 5, "beta-1", false}, nil},
//		{"standard with suffix then dot", "Version: 2.0.5-rc.1", "", gethVersion{"2.0.5-rc.1", 2, 0, 5, "rc.1", false}, nil},
//		{"standard with suffix with number", "Version: 2.0.5-rc1", "", gethVersion{"2.0.5-rc1", 2, 0, 5, "rc1", false}, nil},
//		{"standard with dm suffix, dash", "Version: 2.0.5-dm-12.0", "Geth Help\n\n--deep-mind  Other", gethVersion{"2.0.5-dm-12.0", 2, 0, 5, "dm-12.0", true}, nil},
//		{"standard with dm suffix, dot", "Version: 2.0.5-dm.12.0", "Geth Help\n\n--deep-mind  Other", gethVersion{"2.0.5-dm.12.0", 2, 0, 5, "dm.12.0", true}, nil},
//	}
//
//	for _, test := range tests {
//		t.Run(test.name, func(t *testing.T) {
//			actual, err := newGethVersionFromString(test.inVersion, test.inHelp)
//			if test.expectedErr == nil {
//				assert.Equal(t, test.expected, actual)
//			} else {
//				assert.Equal(t, test.expectedErr, err)
//			}
//		})
//	}
//}
