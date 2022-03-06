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

package nodemanager

import (
	"testing"

	"github.com/streamingfast/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToZapLogPlugin_LogLevel(t *testing.T) {
	tests := []struct {
		name string
		in   string
		out  string
	}{
		// The standard `geth` output
		{
			"debug",
			"DEBUG [10-05|09:54:00.585] message ...",
			`{"level":"debug","msg":"message ..."}`,
		},
		{
			"info",
			"INFO [10-05|09:54:00.585] message ...",
			`{"level":"info","msg":"message ..."}`,
		},
		{
			"warn",
			"WARN [10-05|09:54:00.585] message ...",
			`{"level":"warn","msg":"message ..."}`,
		},
		{
			"error",
			"ERROR [10-05|09:54:00.585] message ...",
			`{"level":"error","msg":"message ..."}`,
		},
		{
			"other",
			"OTHER [10-05|09:54:00.585] message ...",
			`{"level":"info","msg":"message ..."}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wrapper := logging.NewTestLogger(t)
			plugin := NewGethToZapLogPlugin(false, wrapper.Instance())
			plugin.LogLine(test.in)

			loggedLines := wrapper.RecordedLines(t)

			if len(test.out) == 0 {
				require.Len(t, loggedLines, 0)
			} else {
				require.Len(t, loggedLines, 1)
				assert.Equal(t, test.out, loggedLines[0])
			}
		})
	}
}
