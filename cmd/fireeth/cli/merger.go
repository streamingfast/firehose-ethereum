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
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/streamingfast/dlauncher/launcher"
	mergerApp "github.com/streamingfast/merger/app/merger"
)

func init() {
	launcher.RegisterApp(zlog, &launcher.AppDef{
		ID:          "merger",
		Title:       "Merger",
		Description: "Produces merged block files from single-block files",
		RegisterFlags: func(cmd *cobra.Command) error {
			cmd.Flags().Duration("merger-time-between-store-lookups", 1*time.Second, "Delay between source store polling (should be higher for remote storage)")
			cmd.Flags().Duration("merger-time-between-store-pruning", time.Minute, "Delay between source store pruning loops")
			cmd.Flags().Uint64("merger-prune-forked-blocks-after", 50000, "Number of blocks that must pass before we delete old forks (one-block-files lingering)")
			cmd.Flags().String("merger-grpc-listen-addr", MergerServingAddr, "Address to listen for incoming gRPC requests")
			cmd.Flags().Uint64("merger-stop-block", 0, "if non-zero, merger will trigger shutdown when blocks have been merged up to this block")
			return nil
		},
		InitFunc: func(runtime *launcher.Runtime) (err error) {
			return nil
		},
		FactoryFunc: func(runtime *launcher.Runtime) (launcher.App, error) {
			mergedBlocksStoreURL, oneBlocksStoreURL, forkedBlocksStoreURL, err := getCommonStoresURLs(runtime.AbsDataDir)
			if err != nil {
				return nil, err
			}
			return mergerApp.New(&mergerApp.Config{
				StorageOneBlockFilesPath:     oneBlocksStoreURL,
				StorageMergedBlocksFilesPath: mergedBlocksStoreURL,
				StorageForkedBlocksFilesPath: forkedBlocksStoreURL,
				GRPCListenAddr:               viper.GetString("merger-grpc-listen-addr"),
				PruneForkedBlocksAfter:       viper.GetUint64("merger-prune-forked-blocks-after"),
				StopBlock:                    viper.GetUint64("merger-stop-block"),
				TimeBetweenPruning:           viper.GetDuration("merger-time-between-store-pruning"),
				TimeBetweenPolling:           viper.GetDuration("merger-time-between-store-lookups"),
			}), nil
		},
	})
}
