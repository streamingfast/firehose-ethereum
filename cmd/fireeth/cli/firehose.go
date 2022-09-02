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
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/streamingfast/bstream/transform"
	dauthAuthenticator "github.com/streamingfast/dauth/authenticator"
	"github.com/streamingfast/dlauncher/launcher"
	"github.com/streamingfast/dmetering"
	"github.com/streamingfast/dmetrics"
	"github.com/streamingfast/dstore"
	firehoseApp "github.com/streamingfast/firehose/app/firehose"
	"github.com/streamingfast/logging"
	ethss "github.com/streamingfast/firehose-ethereum/substreams"
	ethtransform "github.com/streamingfast/firehose-ethereum/transform"
	"github.com/streamingfast/substreams/client"
	substreamsService "github.com/streamingfast/substreams/service"
	"os"
	"time"
)

var metricset = dmetrics.NewSet()
var headBlockNumMetric = metricset.NewHeadBlockNumber("firehose")
var headTimeDriftmetric = metricset.NewHeadTimeDrift("firehose")

func init() {
	appLogger, _ := logging.PackageLogger("firehose", "github.com/streamingfast/firehose-ethereum/firehose")

	launcher.RegisterApp(zlog, &launcher.AppDef{
		ID:          "firehose",
		Title:       "Block Firehose",
		Description: "Provides on-demand filtered blocks, depends on common-merged-blocks-store-url and common-live-blocks-addr",
		RegisterFlags: func(cmd *cobra.Command) error {
			cmd.Flags().String("firehose-grpc-listen-addr", FirehoseGRPCServingAddr, "Address on which the firehose will listen, appending * to the end of the listen address will start the server over an insecure TLS connection. By default firehose will start in plain-text mode.")

			cmd.Flags().Bool("substreams-enabled", false, "Whether to enable substreams")
			cmd.Flags().Bool("substreams-partial-mode-enabled", false, "Whether to enable partial stores generation support on this instance (usually for internal deployments only)")
			cmd.Flags().StringArray("substreams-rpc-endpoints", nil, "Remote endpoints to contact to satisfy Substreams 'eth_call's")
			cmd.Flags().String("substreams-rpc-cache-store-url", "{sf-data-dir}/rpc-cache", "where rpc cache will be store call responses")
			cmd.Flags().String("substreams-state-store-url", "{sf-data-dir}/localdata", "where substreams state data are stored")
			cmd.Flags().Uint64("substreams-stores-save-interval", uint64(1_000), "Interval in blocks at which to save store snapshots")     // fixme
			cmd.Flags().Uint64("substreams-output-cache-save-interval", uint64(100), "Interval in blocks at which to save store snapshots") // fixme
			cmd.Flags().Uint64("substreams-rpc-cache-chunk-size", uint64(1_000), "RPC cache chunk size in block")
			cmd.Flags().Int("substreams-parallel-subrequest-limit", 4, "number of parallel subrequests substream can make to synchronize its stores")
			cmd.Flags().String("substreams-client-endpoint", "", "firehose endpoint for substreams client.  if left empty, will default to this current local firehose.")
			cmd.Flags().String("substreams-client-jwt", "", "jwt for substreams client authentication")
			cmd.Flags().Bool("substreams-client-insecure", false, "substreams client in insecure mode")
			cmd.Flags().Bool("substreams-client-plaintext", true, "substreams client in plaintext mode")
			cmd.Flags().Int("substreams-sub-request-parallel-jobs", 5, "substreams subrequest parallel jobs for the scheduler")
			cmd.Flags().Int("substreams-sub-request-block-range-size", 1000, "substreams subrequest block range size value for the scheduler")
			return nil
		},

		FactoryFunc: func(runtime *launcher.Runtime) (launcher.App, error) {
			blockstreamAddr := viper.GetString("common-live-blocks-addr")

			// FIXME: That should be a shared dependencies across `Ethereum on StreamingFast`
			authenticator, err := dauthAuthenticator.New(viper.GetString("common-auth-plugin"))
			if err != nil {
				return nil, fmt.Errorf("unable to initialize dauth: %w", err)
			}

			// FIXME: That should be a shared dependencies across `Ethereum on StreamingFast`, it will avoid the need to call `dmetering.SetDefaultMeter`
			metering, err := dmetering.New(viper.GetString("common-metering-plugin"))
			if err != nil {
				return nil, fmt.Errorf("unable to initialize dmetering: %w", err)
			}
			dmetering.SetDefaultMeter(metering)

			mergedBlocksStoreURL, oneBlocksStoreURL, forkedBlocksStoreURL, err := GetCommonStoresURLs(runtime.AbsDataDir)
			if err != nil {
				return nil, err
			}
			indexStore, possibleIndexSizes, err := GetIndexStore(runtime.AbsDataDir)
			if err != nil {
				return nil, fmt.Errorf("unable to initialize indexes: %w", err)
			}

			endpoints := viper.GetStringSlice("substreams-rpc-endpoints")
			for i, endpoint := range endpoints {
				endpoints[i] = os.ExpandEnv(endpoint)
			}

			sfDataDir := runtime.AbsDataDir
			var registerServiceExt firehoseApp.RegisterServiceExtensionFunc
			if viper.GetBool("substreams-enabled") {
				rpcEngine, err := ethss.NewRPCEngine(
					MustReplaceDataDir(sfDataDir, viper.GetString("substreams-rpc-cache-store-url")),
					endpoints,
					viper.GetUint64("substreams-rpc-cache-chunk-size"),
				)
				if err != nil {
					return nil, fmt.Errorf("setting up Ethereum rpc engine and cache: %w", err)
				}

				stateStore, err := dstore.NewStore(MustReplaceDataDir(sfDataDir, viper.GetString("substreams-state-store-url")), "", "", true)
				if err != nil {
					return nil, fmt.Errorf("setting up state store for data: %w", err)
				}

				opts := []substreamsService.Option{
					substreamsService.WithWASMExtension(rpcEngine),
					substreamsService.WithPipelineOptions(rpcEngine),
					substreamsService.WithStoresSaveInterval(viper.GetUint64("substreams-stores-save-interval")),
					substreamsService.WithOutCacheSaveInterval(viper.GetUint64("substreams-output-cache-save-interval")),
				}

				if viper.GetBool("substreams-partial-mode-enabled") {
					opts = append(opts, substreamsService.WithPartialMode())
				}

				endpoint := viper.GetString("substreams-client-endpoint")
				if endpoint == "" {
					endpoint = viper.GetString("firehose-grpc-listen-addr")
				}

				substreamsClientConfig := client.NewSubstreamsClientConfig(
					endpoint,
					os.ExpandEnv(viper.GetString("substreams-client-jwt")),
					viper.GetBool("substreams-client-insecure"),
					viper.GetBool("substreams-client-plaintext"),
				)

				sss := substreamsService.New(
					stateStore,
					"sf.ethereum.type.v2.Block",
					viper.GetInt("substreams-sub-request-parallel-jobs"),
					viper.GetInt("substreams-sub-request-block-range-size"),
					substreamsClientConfig,
					opts...,
				)

				registerServiceExt = sss.Register
			}

			registry := transform.NewRegistry()
			registry.Register(ethtransform.LightBlockFilterFactory)
			registry.Register(ethtransform.MultiLogFilterFactory(indexStore, possibleIndexSizes))
			registry.Register(ethtransform.MultiCallToFilterFactory(indexStore, possibleIndexSizes))
			registry.Register(ethtransform.CombinedFilterFactory(indexStore, possibleIndexSizes))

			//bootStrapFilename := os.Getenv("GRPC_XDS_BOOTSTRAP")
			//zlog.Info("looked for GRPC_XDS_BOOTSTRAP", zap.String("filename", bootStrapFilename))
			//
			//if bootStrapFilename != "" {
			//	zlog.Info("generating bootstrap file", zap.String("filename", bootStrapFilename))
			//	err := dgrpcxds.GenerateBootstrapFile("trafficdirector.googleapis.com:443", bootStrapFilename)
			//	if err != nil {
			//		panic(fmt.Sprintf("failed to generate bootstrap file: %v", err))
			//	}
			//}

			return firehoseApp.New(appLogger, &firehoseApp.Config{
				MergedBlocksStoreURL:    mergedBlocksStoreURL,
				OneBlocksStoreURL:       oneBlocksStoreURL,
				ForkedBlocksStoreURL:    forkedBlocksStoreURL,
				BlockStreamAddr:         blockstreamAddr,
				GRPCListenAddr:          viper.GetString("firehose-grpc-listen-addr"),
				GRPCShutdownGracePeriod: time.Second,
			}, &firehoseApp.Modules{
				Authenticator:            authenticator,
				HeadTimeDriftMetric:      headTimeDriftmetric,
				HeadBlockNumberMetric:    headBlockNumMetric,
				TransformRegistry:        registry,
				RegisterServiceExtension: registerServiceExt,
			}), nil
		},
	})
}
