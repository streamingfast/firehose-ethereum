package main

import (
	"fmt"
	"path"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/streamingfast/cli/sflags"
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
	cmd.AddCommand(newArbOnePollerCmd(logger, tracer))
	cmd.AddCommand(newGenericEVMPollerCmd(logger, tracer))
	return cmd
}

func newOptimismPollerCmd(logger *zap.Logger, tracer logging.Tracer) *cobra.Command {
	// identical as generic-evm for now
	cmd := &cobra.Command{
		Use:   "optimism <rpc-endpoint> <first-streamable-block>",
		Short: "poll blocks from optimism rpc",
		Args:  cobra.ExactArgs(2),
		RunE:  pollerRunE(logger, tracer),
	}
	cmd.Flags().Duration("interval-between-fetch", 0, "interval between fetch")

	return cmd
}
func newArbOnePollerCmd(logger *zap.Logger, tracer logging.Tracer) *cobra.Command {
	// identical as generic-evm for now
	cmd := &cobra.Command{
		Use:   "arb-one <rpc-endpoint> <first-streamable-block>",
		Short: "poll blocks from arb-one rpc",
		Args:  cobra.ExactArgs(2),
		RunE:  pollerRunE(logger, tracer),
	}
	cmd.Flags().Duration("interval-between-fetch", 0, "interval between fetch")

	return cmd
}
func newGenericEVMPollerCmd(logger *zap.Logger, tracer logging.Tracer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generic-evm <rpc-endpoint> <first-streamable-block>",
		Short: "poll blocks from a generic EVM RPC endpoint",
		Args:  cobra.ExactArgs(2),
		RunE:  pollerRunE(logger, tracer),
	}
	cmd.Flags().Duration("interval-between-fetch", 0, "interval between fetch")

	return cmd
}

func pollerRunE(logger *zap.Logger, tracer logging.Tracer) firecore.CommandExecutor {
	return func(cmd *cobra.Command, args []string) (err error) {
		ctx := cmd.Context()

		rpcEndpoint := args[0]
		//dataDir := cmd.Flag("data-dir").Value.String()

		dataDir := sflags.MustGetString(cmd, "data-dir")
		stateDir := path.Join(dataDir, "poller-state")

		logger.Info("launching firehose-ethereum poller", zap.String("rpc_endpoint", rpcEndpoint), zap.String("data_dir", dataDir), zap.String("state_dir", stateDir))

		rpcClient := rpc.NewClient(rpcEndpoint)

		firstStreamableBlock, err := strconv.ParseUint(args[1], 10, 64)
		if err != nil {
			return fmt.Errorf("unable to parse first streamable block %d: %w", firstStreamableBlock, err)
		}

		fetchInterval := sflags.MustGetDuration(cmd, "interval-between-fetch")

		fetcher := blockfetcher.NewOptimismBlockFetcher(rpcClient, fetchInterval, 1*time.Second, logger)
		handler := blockpoller.NewFireBlockHandler("type.googleapis.com/sf.ethereum.type.v2.Block")
		poller := blockpoller.New(fetcher, handler, blockpoller.WithStoringState(stateDir), blockpoller.WithLogger(logger))

		err = poller.Run(ctx, firstStreamableBlock, 1)
		if err != nil {
			return fmt.Errorf("running poller: %w", err)
		}

		return nil
	}
}
