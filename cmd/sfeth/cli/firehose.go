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
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/streamingfast/bstream"
	blockstreamv2 "github.com/streamingfast/bstream/blockstream/v2"
	dauthAuthenticator "github.com/streamingfast/dauth/authenticator"
	"github.com/streamingfast/dlauncher/launcher"
	"github.com/streamingfast/dmetering"
	"github.com/streamingfast/dmetrics"
	firehoseApp "github.com/streamingfast/firehose/app/firehose"
	"github.com/streamingfast/logging"
	pbbstream "github.com/streamingfast/pbgo/dfuse/bstream/v1"
	"github.com/streamingfast/sf-ethereum/filtering"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/dfuse/ethereum/codec/v1"
	"go.uber.org/zap"
)

var metricset = dmetrics.NewSet()
var headBlockNumMetric = metricset.NewHeadBlockNumber("firehose")
var headTimeDriftmetric = metricset.NewHeadTimeDrift("firehose")

func init() {
	appLogger := zap.NewNop()
	logging.Register("github.com/streamingfast/sf-ethereum/firehose", &appLogger)

	launcher.RegisterApp(&launcher.AppDef{
		ID:          "firehose",
		Title:       "Block Firehose",
		Description: "Provides on-demand filtered blocks, depends on common-blocks-store-url and common-blockstream-addr",
		MetricsID:   "merged-filter",
		Logger:      launcher.NewLoggingDef("github.com/streamingfast/sf-ethereum/firehose.*", nil),
		RegisterFlags: func(cmd *cobra.Command) error {
			cmd.Flags().String("firehose-grpc-listen-addr", FirehoseGRPCServingAddr, "Address on which the firehose will listen, appending * to the end of the listen address will start the server over an insecure TLS connection")
			cmd.Flags().StringSlice("firehose-blocks-store-urls", nil, "If non-empty, overrides common-blocks-store-url with a list of blocks stores")
			cmd.Flags().Duration("firehose-realtime-tolerance", 2*time.Minute, "Longest delay to consider this service as real-time (ready) on initialization")

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

			firehoseBlocksStoreURLs := viper.GetStringSlice("firehose-blocks-store-urls")
			if len(firehoseBlocksStoreURLs) == 0 {
				firehoseBlocksStoreURLs = []string{MustReplaceDataDir(sfDataDir, viper.GetString("common-blocks-store-url"))}
			} else if len(firehoseBlocksStoreURLs) == 1 && strings.Contains(firehoseBlocksStoreURLs[0], ",") {
				// Providing multiple elements from config doesn't work with `viper.GetStringSlice`, so let's also handle the case where a single element has separator
				firehoseBlocksStoreURLs = strings.Split(firehoseBlocksStoreURLs[0], ",")
			}

			for i, url := range firehoseBlocksStoreURLs {
				firehoseBlocksStoreURLs[i] = MustReplaceDataDir(sfDataDir, url)
			}

			shutdownSignalDelay := viper.GetDuration("common-system-shutdown-signal-delay")
			grcpShutdownGracePeriod := time.Duration(0)
			if shutdownSignalDelay.Seconds() > 5 {
				grcpShutdownGracePeriod = shutdownSignalDelay - (5 * time.Second)
			}

			filterPreprocessorFactory := func(includeExpr, excludeExpr string) (bstream.PreprocessFunc, error) {
				filter, err := filtering.NewBlockFilter([]string{includeExpr}, []string{excludeExpr})
				if err != nil {
					return nil, fmt.Errorf("parsing filter expressions: %w", err)
				}

				preproc := &filtering.FilteringPreprocessor{Filter: filter}
				return preproc.PreprocessBlock, nil
			}

			if ll := os.Getenv("FIREHOSE_THREADS"); ll != "" {
				if llint, err := strconv.ParseInt(ll, 10, 32); err == nil {
					zlog.Info("setting blockstreamV2 parallel file downloads", zap.Int("ll", int(llint)))
					blockstreamv2.StreamBlocksParallelFiles = int(llint)
				}
			}

			return firehoseApp.New(appLogger, &firehoseApp.Config{
				BlockStoreURLs:          firehoseBlocksStoreURLs,
				BlockStreamAddr:         blockstreamAddr,
				GRPCListenAddr:          viper.GetString("firehose-grpc-listen-addr"),
				GRPCShutdownGracePeriod: grcpShutdownGracePeriod,
				RealtimeTolerance:       viper.GetDuration("firehose-realtime-tolerance"),
			}, &firehoseApp.Modules{
				Authenticator:             authenticator,
				BlockTrimmer:              blockstreamv2.BlockTrimmerFunc(trimBlock),
				FilterPreprocessorFactory: filterPreprocessorFactory,
				HeadTimeDriftMetric:       headTimeDriftmetric,
				HeadBlockNumberMetric:     headBlockNumMetric,
				Tracker:                   tracker,
			}), nil
		},
	})
}

func trimBlock(blk interface{}, details pbbstream.BlockDetails) interface{} {
	// We analyze here to ensure that they are set correctly as they are used when computing the light version
	fullBlock := blk.(*pbcodec.Block)
	fullBlock.Analyze()

	if details == pbbstream.BlockDetails_BLOCK_DETAILS_FULL {
		return fullBlock
	}

	// FIXME: The block is actually duplicated elsewhere which means that at this point,
	//        we work on our own copy of the block. So we can re-write this code to avoid
	//        all the extra allocation and simply nillify the values that we want to hide
	//        instead
	block := &pbcodec.Block{
		Hash:   fullBlock.Hash,
		Number: fullBlock.Number,
		Header: &pbcodec.BlockHeader{
			Timestamp:  fullBlock.Header.Timestamp,
			ParentHash: fullBlock.Header.ParentHash,
		},
	}

	var newTrace func(fullTrxTrace *pbcodec.TransactionTrace) (trxTrace *pbcodec.TransactionTrace)
	newTrace = func(fullTrxTrace *pbcodec.TransactionTrace) (trxTrace *pbcodec.TransactionTrace) {
		trxTrace = &pbcodec.TransactionTrace{
			Hash:    fullTrxTrace.Hash,
			Receipt: fullTrxTrace.Receipt,
			From:    fullTrxTrace.From,
			To:      fullTrxTrace.To,
		}

		trxTrace.Calls = make([]*pbcodec.Call, len(fullTrxTrace.Calls))
		for i, fullCall := range fullTrxTrace.Calls {
			call := &pbcodec.Call{
				Index:               fullCall.Index,
				ParentIndex:         fullCall.ParentIndex,
				Depth:               fullCall.Depth,
				CallType:            fullCall.CallType,
				Caller:              fullCall.Caller,
				Address:             fullCall.Address,
				Value:               fullCall.Value,
				GasLimit:            fullCall.GasLimit,
				GasConsumed:         fullCall.GasConsumed,
				ReturnData:          fullCall.ReturnData,
				Input:               fullCall.Input,
				ExecutedCode:        fullCall.ExecutedCode,
				Suicide:             fullCall.Suicide,
				Logs:                fullCall.Logs,
				Erc20BalanceChanges: fullCall.Erc20BalanceChanges,
				Erc20TransferEvents: fullCall.Erc20TransferEvents,
			}

			trxTrace.Calls[i] = call
		}

		return trxTrace
	}

	traces := make([]*pbcodec.TransactionTrace, len(fullBlock.TransactionTraces))
	for i, fullTrxTrace := range fullBlock.TransactionTraces {
		traces[i] = newTrace(fullTrxTrace)
	}

	block.TransactionTraces = traces

	return block
}
