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

package main

import (
	"github.com/streamingfast/sf-ethereum/cmd/sfeth/cli"
)

// Commit sha1 value, injected via go build `ldflags` at build time
var Commit = ""

// Version value, injected via go build `ldflags` at build time
var Version = "dev"

// IsDirty value, injected via go build `ldflags` at build time
var IsDirty = ""

func init() {
	cli.RootCmd.Version = version()
}

func main() {
	cli.Main(cli.RegisterCommonFlags, nil)
}

func version() string {
	shortCommit := Commit
	if len(shortCommit) >= 7 {
		shortCommit = shortCommit[0:7]
	}

	if len(shortCommit) == 0 {
		shortCommit = "adhoc"
	}

	out := Version + "-" + shortCommit
	if IsDirty != "" {
		out += "-dirty"
	}

	return out
}
