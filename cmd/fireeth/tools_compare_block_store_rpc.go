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

package main

import (
	"fmt"
	"strconv"

	"github.com/streamingfast/bstream"
	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	"github.com/streamingfast/cli/sflags"
	"github.com/streamingfast/dstore"
	"github.com/streamingfast/eth-go/rpc"
	"github.com/streamingfast/firehose-ethereum/blockfetcher"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"

	"github.com/spf13/cobra"
	"github.com/streamingfast/cli"
	firecore "github.com/streamingfast/firehose-core"
	"go.uber.org/zap"
)

func newCompareBlocksStoreRPCCmd(logger *zap.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compare-block-store-rpc <merged-blocks-store> <rpc-endpoint> <start-block> <stop-block>",
		Short: "Checks for any differences between a merged blocks store and and RPC endpoint (get_block with full transactions) for a specified range.",
		Long: cli.Dedent(`
			The 'compare-blocks-rpc' takes in a merged blocks store URL (local or in the cloud), an RPC endpoint URL and inclusive start/stop block numbers.
		`),
		Args: cobra.ExactArgs(4),
		RunE: createCompareBlocksStoreRPCE(logger),
		Example: examplePrefixed("fireeth tools compare-block-store-rpc", `
			# Run over full block range
			/data/merged-blocks-store/ http://localhost:8545 1000000 1001000
		`),
	}

	cmd.PersistentFlags().Bool("save-files", false, cli.Dedent(`
		When activated, block files with difference are saved.
		Format will be fh_{block_num}.json and rpc_{block_num}.json
		diff fh_{block_num}.json and rpc_{block_num}.json
	`))

	return cmd
}

func createCompareBlocksStoreRPCE(logger *zap.Logger) firecore.CommandExecutor {
	return func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		mergedBlocksStoreURL := args[0]
		rpcEndpoint := args[1]
		rpcClient := rpc.NewClient(rpcEndpoint)

		start, err := strconv.ParseInt(args[2], 10, 64)
		if err != nil {
			return fmt.Errorf("parsing start block num: %w", err)
		}

		stop, err := strconv.ParseUint(args[3], 10, 64)
		if err != nil {
			return fmt.Errorf("parsing stop block num: %w", err)
		}

		saveFiles := sflags.MustGetBool(cmd, "save-files")

		mergedBlocksStore, err := dstore.NewDBinStore(mergedBlocksStoreURL)
		if err != nil {
			return fmt.Errorf("creating merged blocks store: %w", err)
		}

		handler := bstream.HandlerFunc(func(blk *pbbstream.Block, obj interface{}) error {
			if blk.Number > stop {
				return nil
			}

			ethBlock := &pbeth.Block{}
			err = blk.Payload.UnmarshalTo(ethBlock)
			if err != nil {
				return fmt.Errorf("unmarshalling pbeth block: %w", err)
			}

			rpcBlock, err := rpcClient.GetBlockByNumber(ctx, rpc.BlockNumber(ethBlock.Number), rpc.WithGetBlockFullTransaction())
			if err != nil {
				panic(err)
			}

			receipts, err := blockfetcher.FetchReceipts(ctx, rpcBlock, rpcClient)
			if err != nil {
				panic(err)
			}

			identical, diffs := CompareFirehoseToRPC(ethBlock, rpcBlock, receipts, saveFiles)

			fmt.Println("Comparing block", ethBlock.Number)

			if !saveFiles {
				if !identical {
					fmt.Println("different", diffs)
				} else {
					fmt.Println(ethBlock.Number, "identical")
				}
			}
			return nil
		})

		filesource := bstream.NewFileSource(mergedBlocksStore, uint64(start), handler, logger, bstream.FileSourceWithStopBlock(stop))
		filesource.Run()

		err = filesource.Err()
		if err != nil {
			if err == bstream.ErrStopBlockReached {
				return nil
			}
			return fmt.Errorf("file source error: %w", err)
		}

		return nil
	}
}
