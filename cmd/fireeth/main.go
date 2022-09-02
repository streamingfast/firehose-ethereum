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
	"github.com/streamingfast/node-manager/operator"
	"github.com/streamingfast/firehose-ethereum/cmd/fireeth/cli"
	"github.com/streamingfast/snapshotter"
)

// Commit sha1 value, injected via go build `ldflags` at build time
var commit = ""

// Version value, injected via go build `ldflags` at build time
var version = "dev"

// Date value, injected via go build `ldflags` at build time
var date = ""

func init() {
	cli.RootCmd.Version = cli.Version(version, commit, date)
}

func main() {
	cli.Main(cli.RegisterCommonFlags, nil, map[string]operator.BackupModuleFactory{
		"gke-pvc-snapshot": gkeSnapshotterFactory,
	})
}

func gkeSnapshotterFactory(conf operator.BackupModuleConfig) (operator.BackupModule, error) {
	return snapshotter.NewGKEPVCSnapshotter(conf)
}
