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
	"net/url"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/streamingfast/bstream/hub"
	"github.com/streamingfast/bstream/transform"
	dauthAuthenticator "github.com/streamingfast/dauth"
	"github.com/streamingfast/derr"
	dgrpcserver "github.com/streamingfast/dgrpc/server"
	discoveryservice "github.com/streamingfast/dgrpc/server/discovery-service"
	"github.com/streamingfast/dlauncher/launcher"
	"github.com/streamingfast/dmetrics"
	"github.com/streamingfast/dstore"
	ethss "github.com/streamingfast/firehose-ethereum/substreams"
	ethtransform "github.com/streamingfast/firehose-ethereum/transform"
	firehoseApp "github.com/streamingfast/firehose/app/firehose"
	firehoseServer "github.com/streamingfast/firehose/server"
	"github.com/streamingfast/logging"
	"github.com/streamingfast/substreams/client"
	substreamsService "github.com/streamingfast/substreams/service"
	"go.uber.org/zap"
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
			cmd.Flags().String("firehose-discovery-service-url", "", "url to configure the grpc discovery service") //traffic-director://xds?vpc_network=vpc-global&use_xds_reds=true
			cmd.Flags().Int("firehose-rate-limit-bucket-size", -1, "Rate limit bucket size (default: no rate limit)")
			cmd.Flags().Duration("firehose-rate-limit-bucket-fill-rate", 10*time.Second, "Rate limit bucket refill rate (default: 10s)")

			cmd.Flags().Bool("substreams-enabled", false, "Whether to enable substreams")
			cmd.Flags().Bool("substreams-tier2", false, "Whether this endpoint is serving tier2 requests (non-public-facing)")
			cmd.Flags().Bool("substreams-request-stats-enabled", false, "Enables stats per request, like block rate. Should only be enabled in debugging instance not in production")
			cmd.Flags().String("substreams-state-store-url", "{sf-data-dir}/localdata", "where substreams state data are stored")
			cmd.Flags().Uint64("substreams-cache-save-interval", uint64(1_000), "Interval in blocks at which to save store snapshots and output caches")
			cmd.Flags().Uint64("substreams-max-fuel-per-block-module", uint64(5_000_000_000_000), "Hard limit for the number of instructions within the execution of a single wasmtime module for a single block")
			cmd.Flags().Int("substreams-parallel-subrequest-limit", 4, "number of parallel subrequests substream can make to synchronize its stores")
			cmd.Flags().String("substreams-client-endpoint", "", "firehose endpoint for substreams client. If empty, this endpoint will also serve its own internal tier2 requests")
			cmd.Flags().String("substreams-client-jwt", "", "JWT for substreams client authentication")
			cmd.Flags().Bool("substreams-client-insecure", false, "substreams client in insecure mode")
			cmd.Flags().Bool("substreams-client-plaintext", true, "substreams client in plaintext mode")
			cmd.Flags().Uint64("substreams-sub-request-parallel-jobs", 5, "substreams subrequest parallel jobs for the scheduler")
			cmd.Flags().Uint64("substreams-sub-request-block-range-size", 10000, "substreams subrequest block range size value for the scheduler")
			cmd.Flags().StringArray("substreams-rpc-endpoints", nil, "Remote endpoints to contact to satisfy Substreams 'eth_call's")
			cmd.Flags().String("substreams-rpc-cache-store-url", "{sf-data-dir}/rpc-cache", "where rpc cache will be store call responses")
			cmd.Flags().Uint64("substreams-rpc-cache-chunk-size", uint64(1_000), "RPC cache chunk size in block")
			return nil
		},

		FactoryFunc: func(runtime *launcher.Runtime) (launcher.App, error) {
			blockstreamAddr := viper.GetString("common-live-blocks-addr")

			authenticator, err := dauthAuthenticator.New(viper.GetString("common-auth-plugin"))
			if err != nil {
				return nil, fmt.Errorf("unable to initialize authenticator: %w", err)
			}
			// FIXME: we cannot call this here.. it does not last the lifetime of the dauth.
			//defer authenticator.Close()

			mergedBlocksStoreURL, oneBlocksStoreURL, forkedBlocksStoreURL, err := getCommonStoresURLs(runtime.AbsDataDir)
			if err != nil {
				return nil, err
			}

			indexStore, possibleIndexSizes, err := GetIndexStore(runtime.AbsDataDir)
			if err != nil {
				return nil, fmt.Errorf("unable to initialize indexes: %w", err)
			}

			sfDataDir := runtime.AbsDataDir
			var registerServiceExt firehoseApp.RegisterServiceExtensionFunc

			rawServiceDiscoveryURL := viper.GetString("firehose-discovery-service-url")
			var serviceDiscoveryURL *url.URL
			if rawServiceDiscoveryURL != "" {
				serviceDiscoveryURL, err = url.Parse(rawServiceDiscoveryURL)
				if err != nil {
					return nil, fmt.Errorf("unable to parse discovery service url: %w", err)
				}
				err = discoveryservice.Bootstrap(serviceDiscoveryURL)
				if err != nil {
					return nil, fmt.Errorf("unable to bootstrap discovery service: %w", err)
				}
			}

			firehoseGRPCListenAddr := viper.GetString("firehose-grpc-listen-addr")
			substreamsEnabled := viper.GetBool("substreams-enabled")

			zlog.Debug("about to configure Substreams", zap.Bool("enabled", substreamsEnabled))
			if viper.GetBool("substreams-enabled") {
				endpoints := viper.GetStringSlice("substreams-rpc-endpoints")
				for i, endpoint := range endpoints {
					endpoints[i] = os.ExpandEnv(endpoint)
				}

				rpcEngine, err := ethss.NewRPCEngine(
					MustReplaceDataDir(sfDataDir, viper.GetString("substreams-rpc-cache-store-url")),
					endpoints,
					viper.GetUint64("substreams-rpc-cache-chunk-size"),
				)
				if err != nil {
					return nil, fmt.Errorf("setting up Ethereum rpc engine and cache: %w", err)
				}

				stateStore, err := dstore.NewStore(MustReplaceDataDir(sfDataDir, viper.GetString("substreams-state-store-url")), "zst", "zstd", true)
				if err != nil {
					return nil, fmt.Errorf("setting up state store for data : %w", err)
				}

				opts := []substreamsService.Option{
					substreamsService.WithWASMExtension(rpcEngine),
					substreamsService.WithPipelineOptions(rpcEngine),
					substreamsService.WithCacheSaveInterval(viper.GetUint64("substreams-cache-save-interval")),
					substreamsService.WithMaxWasmFuelPerBlockModule(viper.GetUint64("substreams-max-fuel-per-block-module")),
				}

				if os.Getenv("SUBSTREAMS_TRACING") == "modules_exec" {
					opts = append(opts, substreamsService.WithModuleExecutionTracing())
				}

				if viper.GetBool("substreams-request-stats-enabled") {
					opts = append(opts, substreamsService.WithRequestStats())
				}

				substreamsClientEndpoint := viper.GetString("substreams-client-endpoint")

				var runTier1, runTier2 bool
				if viper.GetBool("substreams-tier2") {
					runTier2 = true
				} else {
					runTier1 = true
				}

				if substreamsClientEndpoint == "" {
					runTier2 = true // self-contained deployment: run tier2 for our own tier1
					substreamsClientEndpoint = firehoseGRPCListenAddr
				}

				substreamsClientConfig := client.NewSubstreamsClientConfig(
					substreamsClientEndpoint,
					os.ExpandEnv(viper.GetString("substreams-client-jwt")),
					viper.GetBool("substreams-client-insecure"),
					viper.GetBool("substreams-client-plaintext"),
				)

				zlog.Debug("creating Substreams config",
					zap.Bool("run_tier1", runTier1),
					zap.Bool("run_tier2", runTier2),
				)

				var tier1 *substreamsService.Tier1Service
				var tier2 *substreamsService.Tier2Service

				if runTier1 {
					var err error
					tier1, err = substreamsService.NewTier1(
						stateStore,
						"sf.ethereum.type.v2.Block",
						viper.GetUint64("substreams-sub-request-parallel-jobs"),
						viper.GetUint64("substreams-sub-request-block-range-size"),
						substreamsClientConfig,
						opts...,
					)
					if err != nil {
						return nil, fmt.Errorf("creating substreams service: %w", err)
					}
				}
				if runTier2 {
					tier2 = substreamsService.NewTier2(
						stateStore,
						"sf.ethereum.type.v2.Block",
						opts...,
					)
				}

				registerServiceExt = func(server dgrpcserver.Server,
					mergedBlocksStore dstore.Store,
					forkedBlocksStore dstore.Store, // this can be nil here
					forkableHub *hub.ForkableHub,
					logger *zap.Logger) {

					if tier1 != nil {
						tier1.Register(server, mergedBlocksStore, forkedBlocksStore, forkableHub, logger)
					}
					if tier2 != nil {
						tier2.Register(server, mergedBlocksStore, forkedBlocksStore, forkableHub, logger)
					}

				}

			}

			registry := transform.NewRegistry()
			registry.Register(ethtransform.LightBlockTransformFactory)
			registry.Register(ethtransform.HeaderOnlyTransformFactory)
			registry.Register(ethtransform.MultiLogFilterTransformFactory(indexStore, possibleIndexSizes))
			registry.Register(ethtransform.MultiCallToFilterTransformFactory(indexStore, possibleIndexSizes))
			registry.Register(ethtransform.CombinedFilterTransformFactory(indexStore, possibleIndexSizes))

			serverOptions := []firehoseServer.Option{}

			limiterSize := viper.GetInt("firehose-rate-limit-bucket-size")
			limiterRefillRate := viper.GetDuration("firehose-rate-limit-bucket-fill-rate")
			if limiterSize > 0 {
				serverOptions = append(serverOptions, firehoseServer.WithLeakyBucketLimiter(limiterSize, limiterRefillRate))
			}

			return firehoseApp.New(appLogger, &firehoseApp.Config{
				MergedBlocksStoreURL:    mergedBlocksStoreURL,
				OneBlocksStoreURL:       oneBlocksStoreURL,
				ForkedBlocksStoreURL:    forkedBlocksStoreURL,
				BlockStreamAddr:         blockstreamAddr,
				GRPCListenAddr:          firehoseGRPCListenAddr,
				GRPCShutdownGracePeriod: time.Second,
				ServiceDiscoveryURL:     serviceDiscoveryURL,
				ServerOptions:           serverOptions,
			}, &firehoseApp.Modules{
				Authenticator:            authenticator,
				HeadTimeDriftMetric:      headTimeDriftmetric,
				HeadBlockNumberMetric:    headBlockNumMetric,
				TransformRegistry:        registry,
				RegisterServiceExtension: registerServiceExt,
				CheckPendingShutdown: func() bool {
					return derr.IsShuttingDown()
				},
			}), nil
		},
	})
}
