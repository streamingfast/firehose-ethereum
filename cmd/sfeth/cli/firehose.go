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
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/bstream/transform"
	dauthAuthenticator "github.com/streamingfast/dauth/authenticator"
	"github.com/streamingfast/dlauncher/launcher"
	"github.com/streamingfast/dmetering"
	"github.com/streamingfast/dmetrics"
	"github.com/streamingfast/dstore"
	"github.com/streamingfast/firehose"
	firehoseApp "github.com/streamingfast/firehose/app/firehose"
	"github.com/streamingfast/logging"
	ethss "github.com/streamingfast/sf-ethereum/substreams"
	ethtransform "github.com/streamingfast/sf-ethereum/transform"
	substreamsService "github.com/streamingfast/substreams/service"
	"go.uber.org/zap"
)

var metricset = dmetrics.NewSet()
var headBlockNumMetric = metricset.NewHeadBlockNumber("firehose")
var headTimeDriftmetric = metricset.NewHeadTimeDrift("firehose")

func init() {
	appLogger, _ := logging.PackageLogger("firehose", "github.com/streamingfast/sf-ethereum/firehose")

	launcher.RegisterApp(zlog, &launcher.AppDef{
		ID:          "firehose",
		Title:       "Block Firehose",
		Description: "Provides on-demand filtered blocks, depends on common-blocks-store-url and common-blockstream-addr",
		RegisterFlags: func(cmd *cobra.Command) error {
			cmd.Flags().String("firehose-grpc-listen-addr", FirehoseGRPCServingAddr, "Address on which the firehose will listen, appending * to the end of the listen address will start the server over an insecure TLS connection")
			cmd.Flags().Duration("firehose-realtime-tolerance", 2*time.Minute, "Longest delay to consider this service as real-time (ready) on initialization")
			// irreversible indices
			cmd.Flags().String("firehose-irreversible-blocks-index-url", "", "If non-empty, will use this URL as a store to read irreversibility data on blocks and optimize replay")
			cmd.Flags().IntSlice("firehose-irreversible-blocks-index-bundle-sizes", []int{100000, 10000, 1000, 100}, "list of sizes for irreversible block indices")
			// block indices
			cmd.Flags().String("firehose-block-index-url", "", "If non-empty, will use this URL as a store to load index data used by some transforms")
			cmd.Flags().IntSlice("firehose-block-index-sizes", []int{100000, 10000, 1000, 100}, "list of sizes for block indices")
			cmd.Flags().Bool("substreams-enabled", false, "Whether to enable substreams")
			cmd.Flags().Bool("substreams-partial-mode-enabled", false, "Whether to enable partial stores generation support on this instance (usually for internal deployments only)")
			cmd.Flags().String("substreams-rpc-endpoint", "", "Remote endpoint to contact to satisfy Substreams 'eth_call's")
			cmd.Flags().String("substreams-rpc-cache-store-url", "./rpc-cache", "where rpc cache will be store call responses")
			cmd.Flags().String("substreams-state-store-url", "./localdata", "where substreams state data are stored")
			return nil
		},

		FactoryFunc: func(runtime *launcher.Runtime) (launcher.App, error) {
			sfDataDir := runtime.AbsDataDir
			tracker := runtime.Tracker.Clone()
			blockstreamAddr := viper.GetString("common-blockstream-addr")
			if blockstreamAddr != "" {
				tracker.AddGetter(bstream.BlockStreamLIBTarget, bstream.StreamLIBBlockRefGetter(blockstreamAddr))
			}

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

			blocksStoreURL := MustReplaceDataDir(sfDataDir, viper.GetString("common-blocks-store-url"))
			firehoseBlocksStoreURLs := []string{blocksStoreURL}

			if ll := os.Getenv("FIREHOSE_THREADS"); ll != "" {
				if llint, err := strconv.ParseInt(ll, 10, 32); err == nil {
					zlog.Info("setting blockstreamV2 parallel file downloads", zap.Int("ll", int(llint)))
					firehose.StreamBlocksParallelFiles = int(llint)
				}
			}

			indexStoreUrl := viper.GetString("firehose-block-index-url")
			var indexStore dstore.Store
			if indexStoreUrl != "" {
				s, err := dstore.NewStore(indexStoreUrl, "", "", false)
				if err != nil {
					return nil, fmt.Errorf("couldn't create indexStore: %w", err)
				}
				indexStore = s
			}

			var possibleIndexSizes []uint64
			for _, size := range viper.GetIntSlice("firehose-block-index-sizes") {
				if size < 0 {
					return nil, fmt.Errorf("invalid negative size for firehose-block-index-sizes: %d", size)
				}
				possibleIndexSizes = append(possibleIndexSizes, uint64(size))
			}

			var registerServiceExt firehoseApp.RegisterServiceExtensionFunc
			if viper.GetBool("substreams-enabled") {
				rpcEngine, err := ethss.NewRPCEngine(
					viper.GetString("substreams-rpc-cache-store-url"),
					viper.GetString("substreams-rpc-endpoint"),
				)
				if err != nil {
					return nil, fmt.Errorf("setting up Ethereum rpc engine and cache: %w", err)
				}

				stateStore, err := dstore.NewStore(viper.GetString("substreams-state-store-url"), "", "", false)
				if err != nil {
					return nil, fmt.Errorf("setting up state store for data: %w", err)
				}

				opts := []substreamsService.Option{
					substreamsService.WithWASMExtension(rpcEngine),
					substreamsService.WithPipelineOptions(rpcEngine),
				}

				if viper.GetBool("substreams-partial-mode-enabled") {
					opts = append(opts, substreamsService.WithPartialMode())
				}
				sss := substreamsService.New(
					stateStore,
					"sf.ethereum.type.v1.Block",
					opts...,
				)

				registerServiceExt = sss.Register
			}

			registry := transform.NewRegistry()
			registry.Register(ethtransform.LogFilterFactory(indexStore, possibleIndexSizes))
			registry.Register(ethtransform.MultiLogFilterFactory(indexStore, possibleIndexSizes))
			registry.Register(ethtransform.CallToFilterFactory(indexStore, possibleIndexSizes))
			registry.Register(ethtransform.MultiCallToFilterFactory(indexStore, possibleIndexSizes))
			registry.Register(ethtransform.LightBlockFilterFactory)

			var bundleSizes []uint64
			for _, size := range viper.GetIntSlice("firehose-irreversible-blocks-index-bundle-sizes") {
				if size < 0 {
					return nil, fmt.Errorf("invalid negative size for firehose-irreversible-blocks-index-bundle-sizes: %d", size)
				}
				bundleSizes = append(bundleSizes, uint64(size))
			}

			return firehoseApp.New(appLogger, &firehoseApp.Config{
				BlockStoreURLs:                  firehoseBlocksStoreURLs,
				BlockStreamAddr:                 blockstreamAddr,
				GRPCListenAddr:                  viper.GetString("firehose-grpc-listen-addr"),
				GRPCShutdownGracePeriod:         time.Second,
				RealtimeTolerance:               viper.GetDuration("firehose-realtime-tolerance"),
				IrreversibleBlocksIndexStoreURL: viper.GetString("firehose-irreversible-blocks-index-url"),
				IrreversibleBlocksBundleSizes:   bundleSizes,
			}, &firehoseApp.Modules{
				Authenticator:            authenticator,
				HeadTimeDriftMetric:      headTimeDriftmetric,
				HeadBlockNumberMetric:    headBlockNumMetric,
				Tracker:                  tracker,
				TransformRegistry:        registry,
				RegisterServiceExtension: registerServiceExt,
			}), nil
		},
	})
}
