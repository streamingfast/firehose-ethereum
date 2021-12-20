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
	launcher.RegisterApp(&launcher.AppDef{
		ID:          "merger",
		Title:       "Merger",
		Description: "Produces merged block files from single-block files",
		MetricsID:   "merger",
		Logger:      launcher.NewLoggingDef("github.com/streamingfast/merger.*", nil),
		RegisterFlags: func(cmd *cobra.Command) error {
			cmd.Flags().Duration("merger-time-between-store-lookups", 5*time.Second, "delay between source store polling (should be higher for remote storage)")
			cmd.Flags().String("merger-state-file", "{sf-data-dir}/merger/merger.seen.gob", "Path to file containing last written block number, as well as a map of all 'seen blocks' in the 'max-fixable-fork' range")
			cmd.Flags().String("merger-grpc-listen-addr", MergerServingAddr, "Address to listen for incoming gRPC requests")
			cmd.Flags().Duration("merger-writers-leeway", 10*time.Second, "how long we wait after seeing the upper boundary, to ensure that we get as many blocks as possible in a bundle")
			cmd.Flags().Uint64("merger-max-fixable-fork", 10000, "Size of the 'seen blocks' cache. Anything one-block-file older than this will be deleted.")
			cmd.Flags().Int("merger-one-block-deletion-threads", 10, "number of parallel threads used to delete one-block-files (more means more stress on your storage backend)")
			cmd.Flags().Int("merger-max-one-block-operations-batch-size", 2000, "max number of 'good' (mergeable) files to look up from storage in one polling operation")
			cmd.Flags().Uint64("merger-next-exclusive-highest-block-limit", 0, "for next bundle boundary")
			return nil
		},
		// FIXME: Lots of config value construction is duplicated across InitFunc and FactoryFunc, how to streamline that
		//        and avoid the duplication? Note that this duplicate happens in many other apps, we might need to re-think our
		//        init flow and call init after the factory and giving it the instantiated app...
		InitFunc: func(runtime *launcher.Runtime) (err error) {
			sfDataDir := runtime.AbsDataDir

			if err = mkdirStorePathIfLocal(MustReplaceDataDir(sfDataDir, viper.GetString("common-blocks-store-url"))); err != nil {
				return
			}

			if err = mkdirStorePathIfLocal(MustReplaceDataDir(sfDataDir, viper.GetString("common-oneblock-store-url"))); err != nil {
				return
			}

			if err = mkdirStorePathIfLocal(MustReplaceDataDir(sfDataDir, viper.GetString("merger-state-file"))); err != nil {
				return
			}

			return nil
		},
		FactoryFunc: func(runtime *launcher.Runtime) (launcher.App, error) {
			sfDataDir := runtime.AbsDataDir
			return mergerApp.New(&mergerApp.Config{
				StorageOneBlockFilesPath:       MustReplaceDataDir(sfDataDir, viper.GetString("common-oneblock-store-url")),
				StorageMergedBlocksFilesPath:   MustReplaceDataDir(sfDataDir, viper.GetString("common-blocks-store-url")),
				GRPCListenAddr:                 viper.GetString("merger-grpc-listen-addr"),
				WritersLeewayDuration:          viper.GetDuration("merger-writers-leeway"),
				TimeBetweenStoreLookups:        viper.GetDuration("merger-time-between-store-lookups"),
				StateFile:                      MustReplaceDataDir(sfDataDir, viper.GetString("merger-state-file")),
				OneBlockDeletionThreads:        viper.GetInt("merger-one-block-deletion-threads"),
				MaxOneBlockOperationsBatchSize: viper.GetInt("merger-max-one-block-operations-batch-size"),
				NextExclusiveHighestBlockLimit: viper.GetUint64("merger-next-exclusive-highest-block-limit"),
			}), nil
		},
	})
}
