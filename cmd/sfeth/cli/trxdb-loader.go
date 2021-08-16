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
	"math"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/streamingfast/dlauncher/launcher"
	trxdbLoaderApp "github.com/streamingfast/sf-ethereum/trxdb-loader/app/trxdb-loader"
)

func init() {
	launcher.RegisterApp(&launcher.AppDef{
		ID:          "trxdb-loader",
		Title:       "DB loader",
		Description: "Main blocks and transactions database",
		MetricsID:   "trxdb-loader",
		Logger:      launcher.NewLoggingDef("github.com/streamingfast/sf-ethereum/trxdb-loader.*", nil),
		RegisterFlags: func(cmd *cobra.Command) error {
			cmd.Flags().String("trxdb-loader-processing-type", "live", "The actual processing type to perform, either `live`, `batch` or `patch`")
			cmd.Flags().Uint64("trxdb-loader-batch-size", 1, "number of blocks batched together for database write")
			cmd.Flags().Uint64("trxdb-loader-start-block-num", 0, "[BATCH] Block number where we start processing")
			cmd.Flags().Uint64("trxdb-loader-stop-block-num", math.MaxUint32, "[BATCH] Block number where we stop processing")
			cmd.Flags().String("trxdb-loader-http-listen-addr", TrxDBServingAddr, "Listen address for /healthz endpoint")
			cmd.Flags().Bool("trxdb-loader-allow-live-on-empty-table", true, "[LIVE] force pipeline creation if live request and table is empty")
			return nil
		},
		FactoryFunc: func(runtime *launcher.Runtime) (launcher.App, error) {
			sfDataDir := runtime.AbsDataDir
			return trxdbLoaderApp.New(&trxdbLoaderApp.Config{
				ProcessingType:        viper.GetString("trxdb-loader-processing-type"),
				BlockStoreURL:         MustReplaceDataDir(sfDataDir, viper.GetString("common-blocks-store-url")),
				KvdbDsn:               MustReplaceDataDir(sfDataDir, viper.GetString("common-trxdb-dsn")),
				BlockStreamAddr:       viper.GetString("common-blockstream-addr"),
				BatchSize:             viper.GetUint64("trxdb-loader-batch-size"),
				StartBlockNum:         viper.GetUint64("trxdb-loader-start-block-num"),
				StopBlockNum:          viper.GetUint64("trxdb-loader-stop-block-num"),
				AllowLiveOnEmptyTable: viper.GetBool("trxdb-loader-allow-live-on-empty-table"),
				HTTPListenAddr:        viper.GetString("trxdb-loader-http-listen-addr"),
			}), nil
		},
	})

}
