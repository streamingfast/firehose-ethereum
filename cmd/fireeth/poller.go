package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
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
		Args:  cobra.ExactArgs(2),
		RunE:  pollerRunEForTracer(logger),
	}
	cmd.Flags().Duration("interval-between-fetch", 0, "interval between fetch")
	cmd.Flags().Duration("max-block-fetch-duration", 5*time.Second, "maximum delay before retrying a block fetch")

	return cmd
}

func pollerRunE(logger *zap.Logger, tracer logging.Tracer) firecore.CommandExecutor {
	return func(cmd *cobra.Command, args []string) (err error) {
		rpcEndpoint := args[0]
		//dataDir := cmd.Flag("data-dir").Value.String()
		fetchInterval := sflags.MustGetDuration(cmd, "interval-between-fetch")
		parallelWorkers := sflags.MustGetInt(cmd, "parallel-workers")
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

		fetcher := blockfetcher.NewGenericBlockFetcher(fetchInterval, 1*time.Second, parallelWorkers, sflags.MustGetBool(cmd, "allow-empty-receipts-on-block-0"), logger)
		handler := blockpoller.NewFireBlockHandler("type.googleapis.com/sf.ethereum.type.v2.Block")
		poller := blockpoller.New[*rpc.Client](fetcher, handler, rpcClients, blockpoller.WithStoringState[*rpc.Client](stateDir), blockpoller.WithLogger[*rpc.Client](logger))

		err = poller.Run(firstStreamableBlock, nil, 1)
		if err != nil {
			return fmt.Errorf("running poller: %w", err)
		}

		return nil
	}
}

func pollerRunEForTracer(logger *zap.Logger) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		rpcEndpoint := args[0]
		firstBlock, err := strconv.ParseUint(args[1], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid block number: %w", err)
		}

		// Get flag values
		intervalBetweenFetch := sflags.MustGetDuration(cmd, "interval-between-fetch")
		maxBlockFetchDuration := sflags.MustGetDuration(cmd, "max-block-fetch-duration")

		// Connect to the RPC endpoint
		rpcClient := rpc.NewClient(rpcEndpoint)

		handler := blockpoller.NewFireBlockHandler("type.googleapis.com/sf.ethereum.type.v2.Block")

		for blockNum := firstBlock; ; blockNum++ {
			select {
			case <-cmd.Context().Done():
				return nil
			default:
			}

			// Create context with timeout for the RPC call
			ctx, cancel := context.WithTimeout(cmd.Context(), maxBlockFetchDuration)

			// Make the RPC call to trace the block
			respStr, err := rpcClient.DoRequest(ctx, "debug_traceFirehoseBlockByNumber", []interface{}{fmt.Sprintf("0x%x", blockNum), nil})
			cancel()

			if err != nil {
				logger.Warn("failed to trace block", zap.Uint64("block", blockNum), zap.Error(err))
				time.Sleep(1 * time.Second)
				continue
			}

			// The response is base64-encoded, not hex-encoded
			// First decode the base64 response
			rawBytes, err := base64.StdEncoding.DecodeString(respStr)
			if err != nil {
				logger.Warn("failed to decode base64 response", zap.Uint64("block", blockNum), zap.Error(err))
				time.Sleep(1 * time.Second)
				continue
			}

			// Remove trailing newline if present
			if len(rawBytes) > 0 && rawBytes[len(rawBytes)-1] == '\n' {
				rawBytes = rawBytes[:len(rawBytes)-1]
			}

			// Find the last space to separate the firehose line from the block data
			lastSpace := bytes.LastIndexByte(rawBytes, ' ')
			if lastSpace == -1 {
				logger.Warn("invalid firehose block line", zap.Uint64("block", blockNum))
				time.Sleep(1 * time.Second)
				continue
			}

			// The part after the last space is the base64-encoded block data
			base64Part := rawBytes[lastSpace+1:]
			blockData := make([]byte, base64.StdEncoding.DecodedLen(len(base64Part)))
			n, err := base64.StdEncoding.Decode(blockData, base64Part)
			if err != nil {
				logger.Warn("failed to base64 decode block", zap.Uint64("block", blockNum), zap.Error(err))
				time.Sleep(1 * time.Second)
				continue
			}
			blockData = blockData[:n]

			block := &pbeth.Block{}
			if err := proto.Unmarshal(blockData, block); err != nil {
				logger.Warn("failed to unmarshal traced block", zap.Uint64("block", blockNum), zap.Error(err))
				time.Sleep(1 * time.Second)
				continue
			}

			blockBytes, err := proto.Marshal(block)
			if err != nil {
				logger.Warn("failed to marshal block", zap.Uint64("block", blockNum), zap.Error(err))
				time.Sleep(1 * time.Second)
				continue
			}

			bstreamBlock := &pbbstream.Block{
				Number:    block.Number,
				Id:        hex.EncodeToString(block.Hash),
				ParentNum: block.Number - 1,
				ParentId:  hex.EncodeToString(block.Header.ParentHash),
				LibNum:    0,
				Timestamp: block.Header.Timestamp,
				Payload: &anypb.Any{
					TypeUrl: "type.googleapis.com/sf.ethereum.type.v2.Block",
					Value:   blockBytes,
				},
			}

			if err := handler.Handle(bstreamBlock); err != nil {
				return fmt.Errorf("failed to handle block: %w", err)
			}

			// Apply interval between fetches if specified
			if intervalBetweenFetch > 0 {
				time.Sleep(intervalBetweenFetch)
			}
		}
	}
}
