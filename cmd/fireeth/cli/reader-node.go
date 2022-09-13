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
	"github.com/spf13/cobra"
	"github.com/streamingfast/dlauncher/launcher"
	"github.com/streamingfast/node-manager/operator"
)

func registerReaderNodeApp(backupModuleFactories map[string]operator.BackupModuleFactory) {
	launcher.RegisterApp(zlog, &launcher.AppDef{
		ID:            "reader-node",
		Title:         "Ethereum reader Node",
		Description:   "Ethereum node with built-in operational manager and reader plugin to extract block data",
		RegisterFlags: registerReaderNodeFlags,
		InitFunc: func(runtime *launcher.Runtime) error {
			return nil
		},
		FactoryFunc: nodeFactoryFunc(true, backupModuleFactories),
	})
}

func registerReaderNodeFlags(cmd *cobra.Command) error {
	registerCommonNodeFlags(cmd, true)

	cmd.Flags().String("reader-node-grpc-listen-addr", ReaderGRPCAddr, "Address to listen for incoming gRPC requests")
	cmd.Flags().String("reader-node-working-dir", "{sf-data-dir}/reader/work", "Path where reader will stores its files")
	cmd.Flags().Uint("reader-node-stop-block-num", 0, "Shutdown reader node when we the following 'stop-block-num' has been reached, inclusively.")
	cmd.Flags().Int("reader-node-blocks-chan-capacity", 100, "Capacity of the channel holding blocks read by the reader node. Process will shutdown superviser/geth if the channel gets over 90% of that capacity to prevent horrible consequences. Raise this number when processing tiny blocks very quickly")
	cmd.Flags().String("reader-node-oneblock-suffix", "default", "Unique identifier for that reader, so that it can produce 'oneblock files' in the same store as another instance without competing for writes.")

	return nil
}
