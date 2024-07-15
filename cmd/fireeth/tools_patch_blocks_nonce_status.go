package main

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/streamingfast/bstream"
	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	"github.com/streamingfast/cli"
	"github.com/streamingfast/dstore"
	"github.com/streamingfast/eth-go/rpc"
	firecore "github.com/streamingfast/firehose-core"
	"github.com/streamingfast/firehose-ethereum/block"
	"github.com/streamingfast/firehose-ethereum/blockfetcher"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/anypb"
)

func newPatchBlocksNonceStatusCmd(logger *zap.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "patch-blocks-nonce-status",
		Short: "Patch nonce status of blocks",
		Long: cli.Dedent(`
			The 'patch-blocks-nonce-status' command patches the missing nonce and the status of the transactions in the blocks.
			In the optimism and base instrumentation, the status values can only be SUCCESS or FAILED. Some blocks
			were produced with the REVERTED status. This will fix the status of the transactions in the blocks and set the
			REVERTED to FAILED, as they should be. There is also a missing nonce at the transaction level, this tool will fix it.
		`),
		Args: cobra.ExactArgs(4),
		RunE: createPatchBlocksNonceStatusE(logger),
		Example: examplePrefixed("fireeth tools patch-blocks-nonce-status", `
			# Run over full block range
			/data/merged-blocks-store/ http://localhost:8545 1000000 1001000
		`),
	}

	return cmd
}

func createPatchBlocksNonceStatusE(logger *zap.Logger) firecore.CommandExecutor {
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

		mergedBlocksStore, err := dstore.NewDBinStore(mergedBlocksStoreURL)
		if err != nil {
			return fmt.Errorf("creating merged blocks store: %w", err)
		}

		fixedMergedBlocksStore, err := dstore.NewDBinStore(fmt.Sprintf("%s-fixed", mergedBlocksStoreURL))
		if err != nil {
			return fmt.Errorf("creating fixed merged blocks store: %w", err)
		}

		mergeWriter := &firecore.MergedBlocksWriter{
			Store: fixedMergedBlocksStore,
			TweakBlock: func(blk *pbbstream.Block) (*pbbstream.Block, error) {
				ethBlock := &pbeth.Block{}
				err = blk.Payload.UnmarshalTo(ethBlock)
				if err != nil {
					return nil, fmt.Errorf("unmarshalling pbeth block: %w", err)
				}

				rpcBlock, err := rpcClient.GetBlockByNumber(ctx, rpc.BlockNumber(ethBlock.Number), rpc.WithGetBlockFullTransaction())
				if err != nil {
					panic(err)
				}

				receipts, err := blockfetcher.FetchReceipts(ctx, rpcBlock, rpcClient)
				if err != nil {
					panic(err)
				}

				fixedBlock := fixNonceAndStatus(ethBlock, rpcBlock, receipts)

				b, err := anypb.New(fixedBlock)
				if err != nil {
					return nil, fmt.Errorf("creating any block: %w", err)
				}
				blk.Payload = b

				return blk, nil
			},
			Logger: logger,
		}

		filesource := bstream.NewFileSource(mergedBlocksStore, uint64(start), mergeWriter, logger, bstream.FileSourceWithStopBlock(stop))
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

func fixNonceAndStatus(fhBlock *pbeth.Block, rpcBlock *rpc.Block, receipts map[string]*rpc.TransactionReceipt) *pbeth.Block {
	rpcAsPBEth, _ := block.RpcToEthBlock(rpcBlock, receipts, zap.NewNop())

	for _, rpcTrx := range rpcAsPBEth.TransactionTraces {
		for _, fhTrx := range fhBlock.TransactionTraces {
			if !bytes.Equal(rpcTrx.Hash, fhTrx.Hash) {
				continue
			}

			if rpcTrx.Nonce != fhTrx.Nonce {
				fhTrx.Nonce = rpcTrx.Nonce
			}

			if fhTrx.Status == pbeth.TransactionTraceStatus_REVERTED {
				fhTrx.Status = pbeth.TransactionTraceStatus_FAILED
			}
		}
	}

	return fhBlock
}
