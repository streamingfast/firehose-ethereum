package main

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/streamingfast/eth-go/rpc"
	firecore "github.com/streamingfast/firehose-core"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
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

		fmt.Println("FIRE INIT 2.3 local v1.0.0")

		blockNum := startBlockNum
		for {
			latest, err := client.LatestBlockNum(ctx)
			if err != nil {
				delay(err)
				continue
			}

			if latest < blockNum {
				delay(nil)
				continue
			}

			rpcBlock, err := client.GetBlockByNumber(ctx, rpc.BlockNumber(blockNum), rpc.WithGetBlockFullTransaction())
			if err != nil {
				delay(err)
				continue
			}

			logs, err := client.Logs(ctx, rpc.LogsParams{
				FromBlock: rpc.BlockHash(rpcBlock.Hash.String()),
				ToBlock:   rpc.BlockHash(rpcBlock.Hash.String()),
			})
			if err != nil {
				delay(err)
				continue
			}

			block, _ := toFirehoseBlock(rpcBlock, logs)
			cnt, err := proto.Marshal(block)
			if err != nil {
				return fmt.Errorf("failed to proto  marshal pb sol block: %w", err)
			}

			libNum := uint64(0)
			if blockNum != 0 {
				libNum = blockNum - 1
			}
			b64Cnt := base64.StdEncoding.EncodeToString(cnt)
			lineCnt := fmt.Sprintf("FIRE BLOCK %d %s %d %s %s", blockNum, hex.EncodeToString(block.Hash), libNum, hex.EncodeToString(block.Header.ParentHash), b64Cnt)
			if _, err := fmt.Println(lineCnt); err != nil {
				return fmt.Errorf("failed to write log line (char lenght %d): %w", len(lineCnt), err)
			}
			blockNum++
		}
	}
}
