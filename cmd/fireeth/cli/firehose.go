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
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/streamingfast/bstream/transform"
	"github.com/streamingfast/dauth"
	"github.com/streamingfast/derr"
	discoveryservice "github.com/streamingfast/dgrpc/server/discovery-service"
	"github.com/streamingfast/dlauncher/launcher"
	"github.com/streamingfast/dmetrics"
	ethtransform "github.com/streamingfast/firehose-ethereum/transform"
	firehoseApp "github.com/streamingfast/firehose/app/firehose"
	firehoseServer "github.com/streamingfast/firehose/server"
	"github.com/streamingfast/logging"
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
			return nil
		},

		FactoryFunc: func(runtime *launcher.Runtime) (launcher.App, error) {
			blockstreamAddr := viper.GetString("common-live-blocks-addr")

			authenticator, err := dauth.New(viper.GetString("common-auth-plugin"))
			if err != nil {
				return nil, fmt.Errorf("unable to initialize authenticator: %w", err)
			}
			mergedBlocksStoreURL, oneBlocksStoreURL, forkedBlocksStoreURL, err := getCommonStoresURLs(runtime.AbsDataDir)
			if err != nil {
				return nil, err
			}

			indexStore, possibleIndexSizes, err := GetIndexStore(runtime.AbsDataDir)
			if err != nil {
				return nil, fmt.Errorf("unable to initialize indexes: %w", err)
			}

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
				Authenticator:         authenticator,
				HeadTimeDriftMetric:   headTimeDriftmetric,
				HeadBlockNumberMetric: headBlockNumMetric,
				TransformRegistry:     registry,
				CheckPendingShutdown: func() bool {
					return derr.IsShuttingDown()
				},
			}), nil
		},
	})
}
