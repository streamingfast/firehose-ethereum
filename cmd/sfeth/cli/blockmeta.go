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
	"github.com/spf13/viper"
	blockmetaApp "github.com/streamingfast/blockmeta/app/blockmeta"
	"github.com/streamingfast/dlauncher/launcher"
	blockmetadb "github.com/streamingfast/sf-ethereum/blockmeta"
	"github.com/streamingfast/sf-ethereum/trxdb"
)

func init() {
	// Blockmeta
	launcher.RegisterApp(&launcher.AppDef{
		ID:          "blockmeta",
		Title:       "Blockmeta",
		Description: "Serves information about blocks",
		MetricsID:   "blockmeta",
		Logger:      launcher.NewLoggingDef("github.com/streamingfast/(blockmeta|sf-ethereum/blockmeta).*", nil),
		RegisterFlags: func(cmd *cobra.Command) error {
			cmd.Flags().String("blockmeta-grpc-listen-addr", BlockmetaServingAddr, "Address to listen for incoming gRPC requests")
			cmd.Flags().Bool("blockmeta-live-source", true, "Whether we want to connect to a live block source or not.")
			cmd.Flags().Bool("blockmeta-enable-readiness-probe", true, "Enable blockmeta's app readiness probe")
			cmd.Flags().StringSlice("blockmeta-rpc-upstream-addr", []string{"http://localhost:" + MindreaderNodeRPCPort}, "Ethereum RPC API address to fetch info from running chain, must be in-sync")
			return nil
		},
		FactoryFunc: func(runtime *launcher.Runtime) (launcher.App, error) {
			sfDataDir := runtime.AbsDataDir

			trxdbClient, err := trxdb.New(MustReplaceDataDir(sfDataDir, viper.GetString("common-trxdb-dsn")))
			if err != nil {
				return nil, err
			}

			//todo: add db to a modules struct in blockmeta
			db := &blockmetadb.ETHBlockmetaDB{
				DB: trxdbClient,
			}

			blockmetadb.DB = trxdbClient

			blockmetadb.APIs = viper.GetStringSlice("blockmeta-rpc-upstream-addr")
			return blockmetaApp.New(&blockmetaApp.Config{
				Protocol:        Protocol,
				BlockStreamAddr: viper.GetString("common-blockstream-addr"),
				GRPCListenAddr:  viper.GetString("blockmeta-grpc-listen-addr"),
				BlocksStoreURL:  MustReplaceDataDir(sfDataDir, viper.GetString("common-blocks-store-url")),
				LiveSource:      viper.GetBool("blockmeta-live-source"),
			}, db), nil
		},
	})
}
