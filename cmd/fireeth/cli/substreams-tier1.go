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
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	dauthAuthenticator "github.com/streamingfast/dauth"
	discoveryservice "github.com/streamingfast/dgrpc/server/discovery-service"
	"github.com/streamingfast/dlauncher/launcher"
	ethss "github.com/streamingfast/firehose-ethereum/substreams"
	"github.com/streamingfast/logging"
	app "github.com/streamingfast/substreams/app"
	"github.com/streamingfast/substreams/pipeline"
	"github.com/streamingfast/substreams/wasm"
)

var ss1HeadBlockNumMetric = metricset.NewHeadBlockNumber("substreams-tier1")
var ss1HeadTimeDriftmetric = metricset.NewHeadTimeDrift("substreams-tier1")

var registerSSOnce sync.Once

func registerCommonSubstreamsFlags(cmd *cobra.Command) {
	registerSSOnce.Do(func() {
		cmd.Flags().Uint64("substreams-state-bundle-size", uint64(1_000), "Interval in blocks at which to save store snapshots and output caches")
		cmd.Flags().String("substreams-state-store-url", "{sf-data-dir}/localdata", "where substreams state data are stored")
		cmd.Flags().StringArray("substreams-rpc-endpoints", nil, "Remote endpoints to contact to satisfy Substreams 'eth_call's")
		cmd.Flags().String("substreams-rpc-cache-store-url", "{sf-data-dir}/rpc-cache", "where rpc cache will be store call responses")
		cmd.Flags().Uint64("substreams-rpc-cache-chunk-size", uint64(1_000), "RPC cache chunk size in block")
	})
}

func init() {
	appLogger, _ := logging.PackageLogger("substreams-tier1", "github.com/streamingfast/firehose-ethereum/substreams-tier1")

	launcher.RegisterApp(zlog, &launcher.AppDef{
		ID:          "substreams-tier1",
		Title:       "Substreams tier1 server",
		Description: "Provides a substreams grpc endpoint",
		RegisterFlags: func(cmd *cobra.Command) error {
			cmd.Flags().String("substreams-tier1-grpc-listen-addr", SubstreamsTier1GRPCServingAddr, "Address on which the substreams tier1 will listen. Default is plain-text, appending a '*' to the end to jkkkj")
			cmd.Flags().String("substreams-tier1-subrequests-endpoint", SubstreamsTier2GRPCServingAddr, "Address on which the tier1 can reach the tier2")

			// communication with tier2
			cmd.Flags().String("substreams-tier1-discovery-service-url", "", "URL to configure the grpc discovery service, used for communication with tier2") //traffic-director://xds?vpc_network=vpc-global&use_xds_reds=true
			cmd.Flags().Bool("substreams-tier1-subrequests-insecure", false, "Connect to tier2 without checking certificate validity")
			cmd.Flags().Bool("substreams-tier1-subrequests-plaintext", true, "Connect to tier2 without client in plaintext mode")
			cmd.Flags().Int("substreams-tier1-max-subrequests", 4, "number of parallel subrequests that the tier1 can make to the tier2 per request")
			cmd.Flags().Uint64("substreams-tier1-subrequests-size", 10000, "substreams subrequest block range size value for the scheduler")

			cmd.Flags().Bool("substreams-tier1-debug-request-stats", false, "Enables stats per request, like block rate. Should only be enabled in debugging instance, not in production")

			// all substreams
			registerCommonSubstreamsFlags(cmd)
			return nil
		},

		FactoryFunc: func(runtime *launcher.Runtime) (launcher.App, error) {
			blockstreamAddr := viper.GetString("common-live-blocks-addr")

			authenticator, err := dauthAuthenticator.New(viper.GetString("common-auth-plugin"))
			if err != nil {
				return nil, fmt.Errorf("unable to initialize dauth: %w", err)
			}

			mergedBlocksStoreURL, oneBlocksStoreURL, forkedBlocksStoreURL, err := getCommonStoresURLs(runtime.AbsDataDir)
			if err != nil {
				return nil, err
			}

			sfDataDir := runtime.AbsDataDir

			rawServiceDiscoveryURL := viper.GetString("substreams-tier1-discovery-service-url")
			grpcListenAddr := viper.GetString("substreams-tier1-grpc-listen-addr")

			rpcEndpoints := viper.GetStringSlice("substreams-rpc-endpoints")
			rpcCacheStoreURL := MustReplaceDataDir(sfDataDir, viper.GetString("substreams-rpc-cache-store-url"))
			rpcCacheChunkSize := viper.GetUint64("substreams-rpc-cache-chunk-size")

			stateStoreURL := MustReplaceDataDir(sfDataDir, viper.GetString("substreams-state-store-url"))
			stateBundleSize := viper.GetUint64("substreams-state-bundle-size")

			subrequestsEndpoint := viper.GetString("substreams-tier1-subrequests-endpoint")
			subrequestsInsecure := viper.GetBool("substreams-tier1-subrequests-insecure")
			subrequestsPlaintext := viper.GetBool("substreams-tier1-subrequests-plaintext")
			maxSubrequests := viper.GetUint64("substreams-tier1-max-subrequests")
			subrequestsSize := viper.GetUint64("substreams-tier1-subrequests-size")

			debugRequestsStates := viper.GetBool("substreams-tier1-debug-request-stats")

			tracing := os.Getenv("SUBSTREAMS_TRACING") == "modules_exec"

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

			for i, endpoint := range rpcEndpoints {
				rpcEndpoints[i] = os.ExpandEnv(endpoint)
			}

			rpcEngine, err := ethss.NewRPCEngine(
				rpcCacheStoreURL,
				rpcEndpoints,
				rpcCacheChunkSize,
			)
			if err != nil {
				return nil, fmt.Errorf("setting up Ethereum rpc engine and cache: %w", err)
			}

			return app.NewTier1(appLogger,
				&app.Tier1Config{
					MergedBlocksStoreURL: mergedBlocksStoreURL,
					OneBlocksStoreURL:    oneBlocksStoreURL,
					ForkedBlocksStoreURL: forkedBlocksStoreURL,
					BlockStreamAddr:      blockstreamAddr,

					StateStoreURL:        stateStoreURL,
					StateBundleSize:      stateBundleSize,
					BlockType:            "sf.ethereum.type.v2.Block",
					MaxSubrequests:       maxSubrequests,
					SubrequestsSize:      subrequestsSize,
					SubrequestsEndpoint:  subrequestsEndpoint,
					SubrequestsInsecure:  subrequestsInsecure,
					SubrequestsPlaintext: subrequestsPlaintext,

					WASMExtensions:  []wasm.WASMExtensioner{rpcEngine},
					PipelineOptions: []pipeline.PipelineOptioner{rpcEngine},

					DebugRequestsStates: debugRequestsStates,
					Tracing:             tracing,

					GRPCListenAddr:          grpcListenAddr,
					GRPCShutdownGracePeriod: time.Second,
					ServiceDiscoveryURL:     serviceDiscoveryURL,
				}, &app.Modules{
					Authenticator:         authenticator,
					HeadTimeDriftMetric:   ss1HeadTimeDriftmetric,
					HeadBlockNumberMetric: ss1HeadBlockNumMetric,
				}), nil
		},
	})
}
