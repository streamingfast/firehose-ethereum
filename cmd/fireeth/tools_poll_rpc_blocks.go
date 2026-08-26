package main

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/emmansun/base64" // benchmarked 3x faster than standard encoding/base64

	"github.com/streamingfast/firehose-ethereum/blockfetcher"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"

	"github.com/spf13/cobra"
	"github.com/streamingfast/eth-go/rpc"
	firecore "github.com/streamingfast/firehose-core"
	"github.com/streamingfast/firehose-ethereum/block"
	"go.uber.org/zap"
)

func newPollRPCBlocksCmd(logger *zap.Logger) *cobra.Command {
	return &cobra.Command{
		Use:   "poll-rpc-blocks <rpc-endpoint> <start-block>",
		Short: "Generate 'light' firehose blocks from an RPC endpoint",
		Args:  cobra.ExactArgs(2),
		RunE:  createPollRPCBlocksE(logger),
	}
}

var pollDelay = time.Millisecond * 100

var lastDelayWarning time.Time

func createPollRPCBlocksE(logger *zap.Logger) firecore.CommandExecutor {
	delay := func(err error) {
		if err != nil {
			logger.Warn("retrying...", zap.Error(err))
		}
		time.Sleep(pollDelay)
	}

	return func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		rpcEndpoint := args[0]
		startBlockNumStr := args[1]

		logger.Info("retrieving from rpc endpoint",
			zap.String("start_block_num", startBlockNumStr),
			zap.String("rpc_endpoint", rpcEndpoint),
		)
		startBlockNum, err := strconv.ParseUint(startBlockNumStr, 10, 64)
		if err != nil {
			return fmt.Errorf("unable to parse start block number %s: %w", startBlockNumStr, err)
		}
		client := rpc.NewClient(rpcEndpoint)

		fmt.Println("FIRE INIT 3.0 local v1.0.0")

		blockNum := startBlockNum
		latest := uint64(0)
		for {

			if latest <= blockNum {
				latest, err = client.LatestBlockNum(ctx)
				if err != nil {
					delay(err)
					continue
				}

				if latest <= blockNum {
					delay(nil)
					continue
				}
			}

			rpcBlock, err := client.GetBlockByNumber(ctx, rpc.BlockNumber(blockNum), rpc.WithGetBlockFullTransaction())
			if err != nil {
				delay(err)
				continue
			}

			// A `null` result comes back without an error, so retry rather than
			// dereferencing a nil block.
			if rpcBlock == nil || rpcBlock.Hash == nil {
				delay(fmt.Errorf("block %d not available on this endpoint", blockNum))
				continue
			}

			receipts, err := blockfetcher.FetchReceipts(ctx, rpcBlock, client, 20, false)
			if err != nil {
				delay(fmt.Errorf("fetching receipts for block %d %q: %w", rpcBlock.Number, rpcBlock.Hash.Pretty(), err))
				continue
			}

			ethBlock, _ := block.RpcToEthBlock(rpcBlock, receipts, nil, logger)

			lineCnt, err := fireBlockLine(ethBlock)
			if err != nil {
				return fmt.Errorf("generating FIRE BLOCK line for block %d: %w", blockNum, err)
			}
			if _, err := fmt.Println(lineCnt); err != nil {
				return fmt.Errorf("failed to write log line (char length %d): %w", len(lineCnt), err)
			}
			blockNum++
		}
	}
}

// fireBlockLine formats a Firehose protocol version 3.0 'FIRE BLOCK' line, as expected
// by codec.(*parseCtx).readBlockForProtocolVersion3:
//
//	FIRE BLOCK <num> <hash> <parent_num> <parent_hash> <lib_num> <timestamp_unix_nano> <b64_payload>
func fireBlockLine(ethBlock *pbeth.Block) (string, error) {
	payload, err := ethBlock.MarshalVT()
	if err != nil {
		return "", fmt.Errorf("failed to proto marshal pb eth block: %w", err)
	}

	parentNum := uint64(0)
	if ethBlock.Number != 0 {
		parentNum = ethBlock.Number - 1
	}
	libNum := parentNum

	return fmt.Sprintf("FIRE BLOCK %d %s %d %s %d %d %s",
		ethBlock.Number,
		hex.EncodeToString(ethBlock.Hash),
		parentNum,
		hex.EncodeToString(ethBlock.Header.ParentHash),
		libNum,
		ethBlock.Header.Timestamp.AsTime().UnixNano(),
		base64.StdEncoding.EncodeToString(payload),
	), nil
}
