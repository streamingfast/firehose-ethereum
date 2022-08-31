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
	"github.com/streamingfast/dlauncher/launcher"
	"github.com/streamingfast/logging"
	nodeManager "github.com/streamingfast/node-manager"
	nodeMindreaderStdinApp "github.com/streamingfast/node-manager/app/node_mindreader_stdin"
	"github.com/streamingfast/node-manager/metrics"
	"github.com/streamingfast/node-manager/mindreader"
	"github.com/streamingfast/sf-ethereum/node-manager/codec"
)

func init() {
	appLogger, appTracer := logging.PackageLogger("reader-node-stdin", "github.com/streamingfast/sf-ethereum/reader-node-stdin")

	launcher.RegisterApp(zlog, &launcher.AppDef{
		ID:            "reader-node-stdin",
		Title:         "Mindreader Node (stdin)",
		Description:   "Blocks reading node, unmanaged, reads deep mind from standard input",
		RegisterFlags: func(cmd *cobra.Command) error { return nil },
		FactoryFunc: func(runtime *launcher.Runtime) (launcher.App, error) {
			sfDataDir := runtime.AbsDataDir
			archiveStoreURL := MustReplaceDataDir(sfDataDir, viper.GetString("common-one-block-store-url"))

			consoleReaderFactory := func(lines chan string) (mindreader.ConsolerReader, error) {
				r, err := codec.NewConsoleReader(appLogger, lines)
				if err != nil {
					return nil, fmt.Errorf("initiating console reader: %w", err)
				}

				return r, nil
			}

			metricID := "mindreader-geth-node-stdin"
			headBlockTimeDrift := metrics.NewHeadBlockTimeDrift(metricID)
			headBlockNumber := metrics.NewHeadBlockNumber(metricID)
			appReadiness := metrics.NewAppReadiness(metricID)
			metricsAndReadinessManager := nodeManager.NewMetricsAndReadinessManager(headBlockTimeDrift, headBlockNumber, appReadiness, viper.GetDuration("mindreader-geth-node-readiness-max-latency"))

			return nodeMindreaderStdinApp.New(&nodeMindreaderStdinApp.Config{
				GRPCAddr:                   viper.GetString("mindreader-geth-node-grpc-listen-addr"),
				OneBlocksStoreURL:          archiveStoreURL,
				MindReadBlocksChanCapacity: viper.GetInt("mindreader-geth-node-blocks-chan-capacity"),
				StartBlockNum:              viper.GetUint64("mindreader-geth-node-start-block-num"),
				StopBlockNum:               viper.GetUint64("mindreader-geth-node-stop-block-num"),
				WorkingDir:                 MustReplaceDataDir(sfDataDir, viper.GetString("mindreader-geth-node-working-dir")),
				OneBlockSuffix:             viper.GetString("mindreader-geth-node-oneblock-suffix"),
			}, &nodeMindreaderStdinApp.Modules{
				ConsoleReaderFactory:       consoleReaderFactory,
				MetricsAndReadinessManager: metricsAndReadinessManager,
			}, appLogger, appTracer), nil
		},
	})
}
