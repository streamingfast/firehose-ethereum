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
			cmd.Flags().Duration("merger-time-between-store-lookups", 5*time.Second, "delay between source store polling (should be higher for remote storage)")
			cmd.Flags().Duration("merger-time-between-store-pruning", time.Minute, "delay between source store pruning loops")
			cmd.Flags().Duration("merger-max-forked-blocks-age-before-pruning", time.Hour*72, "one-block-files that are older than this will be removed from store, preventing resolution of old cursors on forked blocks")
			cmd.Flags().String("merger-grpc-listen-addr", MergerServingAddr, "Address to listen for incoming gRPC requests")
			cmd.Flags().Duration("merger-writers-leeway", 10*time.Second, "how long we wait after seeing the upper boundary, to ensure that we get as many blocks as possible in a bundle")
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

			return nil
		},
		FactoryFunc: func(runtime *launcher.Runtime) (launcher.App, error) {
			sfDataDir := runtime.AbsDataDir
			return mergerApp.New(&mergerApp.Config{
				StorageOneBlockFilesPath:       MustReplaceDataDir(sfDataDir, viper.GetString("common-oneblock-store-url")),
				StorageMergedBlocksFilesPath:   MustReplaceDataDir(sfDataDir, viper.GetString("common-blocks-store-url")),
				GRPCListenAddr:                 viper.GetString("merger-grpc-listen-addr"),
				MaxForkedBlockAgeBeforePruning: viper.GetDuration("merger-max-forked-blocks-age-before-pruning"),
				TimeBetweenPruning:             viper.GetDuration("merger-time-between-store-pruning"),
				TimeBetweenPolling:             viper.GetDuration("merger-time-between-store-lookups"),
			}), nil
		},
	})
}
