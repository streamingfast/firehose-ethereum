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
	dgrpcserver "github.com/streamingfast/dgrpc/server"
	"strings"

	"github.com/spf13/cobra"
	"github.com/streamingfast/bstream/blockstream"
	"github.com/streamingfast/dlauncher/launcher"
	"github.com/streamingfast/firehose-ethereum/codec"
	"github.com/streamingfast/logging"
	nodeManager "github.com/streamingfast/node-manager"
	"github.com/streamingfast/node-manager/mindreader"
	"github.com/streamingfast/node-manager/operator"
	"go.uber.org/zap"
)

func registerReaderNodeApp(backupModuleFactories map[string]operator.BackupModuleFactory) {
	launcher.RegisterApp(zlog, &launcher.AppDef{
		ID:            "reader-node",
		Title:         "Ethereum reader Node",
		Description:   "Ethereum node with built-in operational manager and reader plugin to extract block data",
		RegisterFlags: registerReaderNodeFlags,
		InitFunc: func(runtime *launcher.Runtime) error {
			return nil
		},
		FactoryFunc: nodeFactoryFunc(true, backupModuleFactories),
	})
}

func registerReaderNodeFlags(cmd *cobra.Command) error {
	registerCommonNodeFlags(cmd, true)

	cmd.Flags().String("reader-node-grpc-listen-addr", ReaderGRPCAddr, "Address to listen for incoming gRPC requests")
	cmd.Flags().String("reader-node-working-dir", "{sf-data-dir}/mindreader/work", "Path where mindreader will stores its files")
	cmd.Flags().Uint("reader-node-stop-block-num", 0, "Shutdown mindreader when we the following 'stop-block-num' has been reached, inclusively.")
	cmd.Flags().Int("reader-node-blocks-chan-capacity", 100, "Capacity of the channel holding blocks read by the mindreader. Process will shutdown superviser/geth if the channel gets over 90% of that capacity to prevent horrible consequences. Raise this number when processing tiny blocks very quickly")
	cmd.Flags().String("reader-node-oneblock-suffix", "default", "Unique identifier for that mindreader, so that it can produce 'oneblock files' in the same store as another instance without competing for writes.")

	return nil
}

func getReaderLogPlugin(
	oneBlockStoreURL string,
	workingDir string,
	batchStartBlockNum uint64,
	batchStopBlockNum uint64,
	oneBlockFileSuffix string,
	blocksChanCapacity int,
	operatorShutdownFunc func(error),
	metricsAndReadinessManager *nodeManager.MetricsAndReadinessManager,
	gs dgrpcserver.Server,
	appLogger *zap.Logger,
	appTracer logging.Tracer) (*mindreader.MindReaderPlugin, error) {

	// It's important that this call goes prior running gRPC server since it's doing
	// some service registration. If it's call later on, the overall application exits.
	blockStreamServer := blockstream.NewServer(gs, blockstream.ServerOptionWithLogger(appLogger))

	consoleReaderFactory := func(lines chan string) (mindreader.ConsolerReader, error) {
		return codec.NewConsoleReader(appLogger, lines)
	}

	logPlugin, err := mindreader.NewMindReaderPlugin(
		oneBlockStoreURL,
		workingDir,
		consoleReaderFactory,
		batchStartBlockNum,
		batchStopBlockNum,
		blocksChanCapacity,
		metricsAndReadinessManager.UpdateHeadBlock,
		func(error) {
			operatorShutdownFunc(nil)
		},
		oneBlockFileSuffix,
		blockStreamServer,
		appLogger,
		appTracer,
	)
	if err != nil {
		return nil, err
	}

	return logPlugin, nil
}

func replaceNodeRole(nodeRole, in string) string {
	return strings.Replace(in, "{node-role}", nodeRole, -1)
}
