package main

import (
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/streamingfast/cli"
	"github.com/streamingfast/cli/sflags"
	"github.com/streamingfast/eth-go/rpc"
	firecore "github.com/streamingfast/firehose-core"
	"github.com/streamingfast/firehose-core/blockpoller"
	firecorerpc "github.com/streamingfast/firehose-core/rpc"
	"github.com/streamingfast/firehose-ethereum/blockfetcher"
	"github.com/streamingfast/logging"
	"go.uber.org/zap"
)

func newPollerCmd(logger *zap.Logger, tracer logging.Tracer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "poller",
		Short: "poll blocks from different sources",
	}

	cmd.PersistentFlags().Uint("parallel-workers", 20, "number of parallel workers to fetch transaction receipts")
	cmd.PersistentFlags().Uint("block-fetch-batch-size", 1, "number of blocks to fetch (and serialize) in parallel")
	cmd.PersistentFlags().StringSliceP("headers", "H", nil, "headers to send with each request (ex: '-H \"key1: value1\" -H \"key2: value2\"')")
	cmd.PersistentFlags().Bool("allow-empty-receipts-on-block-0", false, "whether to accept empty receipts on block 0, filling them with 'empty' transactions (useful for TRON-EVM)")

	cmd.AddCommand(newOptimismPollerCmd(logger, tracer))
	cmd.AddCommand(newArbOnePollerCmd(logger, tracer))
	cmd.AddCommand(newGenericEVMPollerCmd(logger, tracer))
	cmd.AddCommand(newFirehoseTracerPollerCmd(logger, tracer))
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
	cmd.Flags().Duration("max-block-fetch-duration", 5*time.Second, "maximum delay before retrying a block fetch")

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
	cmd.Flags().Duration("max-block-fetch-duration", 5*time.Second, "maximum delay before retrying a block fetch")

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
	cmd.Flags().Duration("max-block-fetch-duration", 5*time.Second, "maximum delay before retrying a block fetch")

	return cmd
}

func newFirehoseTracerPollerCmd(logger *zap.Logger, tracer logging.Tracer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "firehose-tracer-api <rpc-endpoint> <first-streamable-block>",
		Short: "poll blocks using debug_traceFirehoseBlockByNumber",
		Long: cli.Dedent(`
			This poller connects to a Firehose-enabled RPC endpoint and fetches
			blocks using the "debug_traceFirehoseBlockByNumber" method. It retrieves the
			full Firehose block data along with execution traces, enabling advanced debugging
			and block-level analysis.

			*Experimental*: This tool is not production-ready. Intended for development
			and debugging purposes only.
		`),
		Args: cobra.ExactArgs(2),
		RunE: pollerRunEForTracer(logger),
	}
	cmd.Flags().Duration("interval-between-fetch", 0, "interval between fetch")
	cmd.Flags().Duration("max-block-fetch-duration", 5*time.Second, "maximum delay before retrying a block fetch")

	return cmd
}

func pollerRunE(logger *zap.Logger, tracer logging.Tracer) firecore.CommandExecutor {
	return pollerRunEInternal(logger, tracer, false)
}

func pollerRunEForTracer(logger *zap.Logger) func(cmd *cobra.Command, args []string) error {
	return pollerRunEInternal(logger, nil, true)
}

func pollerRunEInternal(logger *zap.Logger, tracer logging.Tracer, useTracer bool) firecore.CommandExecutor {
	return func(cmd *cobra.Command, args []string) (err error) {
		rpcEndpoint := args[0]
		//dataDir := cmd.Flag("data-dir").Value.String()
		fetchInterval := sflags.MustGetDuration(cmd, "interval-between-fetch")
		parallelWorkers := sflags.MustGetInt(cmd, "parallel-workers")
		blockFetchBatchSize := sflags.MustGetInt(cmd, "block-fetch-batch-size")
		maxBlockFetchDuration := sflags.MustGetDuration(cmd, "max-block-fetch-duration")

		dataDir := sflags.MustGetString(cmd, "data-dir")
		stateDir := path.Join(dataDir, "poller-state")

		firstStreamableBlock, err := strconv.ParseUint(args[1], 10, 64)
		if err != nil {
			return fmt.Errorf("unable to parse first streamable block %d: %w", firstStreamableBlock, err)
		}

		logger.Info("launching firehose-ethereum poller",
			zap.String("rpc_endpoint", rpcEndpoint),
			zap.String("data_dir", dataDir),
			zap.String("state_dir", stateDir),
			zap.Duration("fetch_interval", fetchInterval),
			zap.Duration("max_block_fetch_duration", maxBlockFetchDuration),
			zap.Uint64("first_streamable_block", firstStreamableBlock),
			zap.Int("block_fetch_batch_size", blockFetchBatchSize),
		)
		rpcClients := firecorerpc.NewClients[*rpc.Client](maxBlockFetchDuration, firecorerpc.NewStickyRollingStrategy[*rpc.Client](), logger)

		var opts []rpc.Option
		for _, headerStr := range sflags.MustGetStringSlice(cmd, "headers") {
			parts := strings.SplitN(headerStr, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				opts = append(opts, rpc.WithHttpHeader(key, value))
			}
		}

		rpcClients.Add(rpc.NewClient(rpcEndpoint, opts...))

		var fetcher blockpoller.BlockFetcher[*rpc.Client]
		if useTracer {
			fetcher = blockfetcher.NewTracerBlockFetcher(fetchInterval, 1*time.Second, parallelWorkers, sflags.MustGetBool(cmd, "allow-empty-receipts-on-block-0"), logger)
		} else {
			fetcher = blockfetcher.NewGenericBlockFetcher(fetchInterval, 1*time.Second, parallelWorkers, sflags.MustGetBool(cmd, "allow-empty-receipts-on-block-0"), logger)
		}
		handler := blockpoller.NewFireBlockHandler("type.googleapis.com/sf.ethereum.type.v2.Block")
		poller := blockpoller.New[*rpc.Client](fetcher, handler, rpcClients, blockpoller.WithStoringState[*rpc.Client](stateDir), blockpoller.WithLogger[*rpc.Client](logger))

		err = poller.Run(firstStreamableBlock, nil, blockFetchBatchSize)
		if err != nil {
			return fmt.Errorf("running poller: %w", err)
		}

		return nil
	}
}
