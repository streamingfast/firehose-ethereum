package main

import (
	"fmt"
	"path"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/eth-go/rpc"
	firecore "github.com/streamingfast/firehose-core"
	"github.com/streamingfast/firehose-core/blockpoller"
	"github.com/streamingfast/firehose-ethereum/blockfetcher"
	"github.com/streamingfast/logging"
	"go.uber.org/zap"
)

func newPollerCmd(logger *zap.Logger, tracer logging.Tracer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "poller",
		Short: "poll blocks from different sources",
	}

	cmd.AddCommand(newOptimismPollerCmd(logger, tracer))
	return cmd
}

func newOptimismPollerCmd(logger *zap.Logger, tracer logging.Tracer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "optimism <rpc-endpoint> <first-streamable-block>",
		Short: "poll blocks from optimism rpc",
		Args:  cobra.ExactArgs(2),
		RunE:  optimismPollerRunE(logger, tracer),
	}

	return cmd
}

func optimismPollerRunE(logger *zap.Logger, tracer logging.Tracer) firecore.CommandExecutor {
	return func(cmd *cobra.Command, args []string) (err error) {
		ctx := cmd.Context()

		rpcEndpoint := args[0]
		dataDir := cmd.Flag("data-dir").Value.String()
		stateDir := path.Join(dataDir, "poller-state")

		logger.Info("launching firehose-ethereum poller", zap.String("rpc_endpoint", rpcEndpoint), zap.String("state_dir", stateDir))

		rpcClient := rpc.NewClient(rpcEndpoint)

		firstStreamableBlock, err := strconv.ParseUint(args[1], 10, 64)
		if err != nil {
			return fmt.Errorf("unable to parse first streamable block %d: %w", firstStreamableBlock, err)
		}

		fetcher := blockfetcher.NewOptimismBlockFetcher(rpcClient, 1*time.Second)
		handler := blockpoller.NewFireBlockHandler("type.googleapis.com/sf.ethereum.type.v2.Block")
		poller := blockpoller.New(fetcher, handler, blockpoller.WithStoringState(stateDir), blockpoller.WithLogger(logger))

		latestBlock, err := rpcClient.GetBlockByNumber(ctx, rpc.FinalizedBlock)
		if err != nil {
			return fmt.Errorf("getting latest block: %w", err)
		}

		err = poller.Run(ctx, firstStreamableBlock, bstream.NewBlockRef(latestBlock.Hash.String(), uint64(latestBlock.Number)))
		if err != nil {
			return fmt.Errorf("running poller: %w", err)
		}

		return nil
	}
}
