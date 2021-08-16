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

package tools

import (
	"strings"

	"github.com/lithammer/dedent"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{Use: "tools", Short: "Developer tools related to sfeth"}

func cobraDescription(in string) string {
	return dedent.Dedent(strings.Trim(in, "\n"))
}

func cobraExamples(in ...string) string {
	for i, line := range in {
		in[i] = "  " + line
	}

	return strings.Join(in, "\n")
}
