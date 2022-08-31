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
	relayerApp "github.com/streamingfast/relayer/app/relayer"
)

func init() {
	// Relayer
	launcher.RegisterApp(zlog, &launcher.AppDef{
		ID:          "relayer",
		Title:       "Relayer",
		Description: "Serves blocks as a stream, with a buffer",
		RegisterFlags: func(cmd *cobra.Command) error {
			cmd.Flags().String("relayer-grpc-listen-addr", RelayerServingAddr, "Address to listen for incoming gRPC requests")
			cmd.Flags().StringSlice("relayer-source", []string{ReaderGRPCAddr}, "List of Blockstream sources (mindreaders) to connect to for live block feeds (repeat flag as needed)")
			cmd.Flags().Duration("relayer-max-source-latency", 999999*time.Hour, "Max latency tolerated to connect to a source. A performance optimization for when you have redundant sources and some may not have caught up")
			return nil
		},
		FactoryFunc: func(runtime *launcher.Runtime) (launcher.App, error) {
			_, oneBlocksStoreURL, _, err := GetCommonStoresURLs(runtime.AbsDataDir)
			if err != nil {
				return nil, err
			}
			return relayerApp.New(&relayerApp.Config{
				SourcesAddr:      viper.GetStringSlice("relayer-source"),
				OneBlocksURL:     oneBlocksStoreURL,
				GRPCListenAddr:   viper.GetString("relayer-grpc-listen-addr"),
				MaxSourceLatency: viper.GetDuration("relayer-max-source-latency"),
			}), nil
		},
	})
}
