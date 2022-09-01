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
