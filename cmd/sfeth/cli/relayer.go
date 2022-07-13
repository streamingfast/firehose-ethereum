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

//import (
//	"time"
//
//	"github.com/spf13/cobra"
//	"github.com/spf13/viper"
//	"github.com/streamingfast/dlauncher/launcher"
//	relayerApp "github.com/streamingfast/relayer/app/relayer"
//)
//
//func init() {
//	// Relayer
//	launcher.RegisterApp(zlog, &launcher.AppDef{
//		ID:          "relayer",
//		Title:       "Relayer",
//		Description: "Serves blocks as a stream, with a buffer",
//		RegisterFlags: func(cmd *cobra.Command) error {
//			cmd.Flags().String("relayer-grpc-listen-addr", RelayerServingAddr, "Address to listen for incoming gRPC requests")
//			cmd.Flags().StringSlice("relayer-source", []string{MindreaderGRPCAddr}, "List of Blockstream sources (mindreaders) to connect to for live block feeds (repeat flag as needed)")
//			cmd.Flags().String("relayer-merger-addr", MergerServingAddr, "Address for grpc merger service")
//			cmd.Flags().Int("relayer-buffer-size", 300, "number of blocks that will be kept and sent immediately on connection")
//			cmd.Flags().Uint64("relayer-min-start-offset", 120, "number of blocks before HEAD where we want to start for faster buffer filling (missing blocks come from files/merger)")
//			cmd.Flags().Duration("relayer-max-source-latency", 10*time.Minute, "max latency tolerated to connect to a source")
//			return nil
//		},
//		FactoryFunc: func(runtime *launcher.Runtime) (launcher.App, error) {
//			sfDataDir := runtime.AbsDataDir
//			return relayerApp.New(&relayerApp.Config{
//				SourcesAddr:      viper.GetStringSlice("relayer-source"),
//				GRPCListenAddr:   viper.GetString("relayer-grpc-listen-addr"),
//				MergerAddr:       viper.GetString("relayer-merger-addr"),
//				BufferSize:       viper.GetInt("relayer-buffer-size"),
//				MaxSourceLatency: viper.GetDuration("relayer-max-source-latency"),
//				MinStartOffset:   viper.GetUint64("relayer-min-start-offset"),
//				SourceStoreURL:   MustReplaceDataDir(sfDataDir, viper.GetString("common-blocks-store-url")),
//			}, &relayerApp.Modules{}), nil
//		},
//	})
//}
