package main

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/streamingfast/bstream"
	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	"github.com/streamingfast/dstore"
	firecore "github.com/streamingfast/firehose-core"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"go.uber.org/zap"
)

func newRemoveGasChangesCmd(logger *zap.Logger) *cobra.Command {
	return &cobra.Command{
		Use:   "remove-gas-changes <src-blocks-store> <dest-blocks-store> <start-block> <stop-block>",
		Short: "remove call gas changes from blocks and rewrite the affected 100-block files to destination",
		Args:  cobra.ExactArgs(4),
		RunE:  createRemoveGasChangesE(logger),
	}
}

func createRemoveGasChangesE(logger *zap.Logger) firecore.CommandExecutor {
	return func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		srcStore, err := dstore.NewDBinStore(args[0])
		if err != nil {
			return fmt.Errorf("unable to create source store: %w", err)
		}

		destStore, err := dstore.NewDBinStore(args[1])
		if err != nil {
			return fmt.Errorf("unable to create destination store: %w", err)
		}

		start := mustParseUint64(args[2])
		stop := mustParseUint64(args[3])

		if stop <= start {
			return fmt.Errorf("stop block must be greater than start block")
		}

		lastFileProcessed := ""
		startWalkFrom := fmt.Sprintf("%010d", start-(start%100))
		err = srcStore.WalkFrom(ctx, "", startWalkFrom, func(filename string) error {
			logger.Debug("checking merged block file", zap.String("filename", filename))

			startBlock := mustParseUint64(filename)

			if startBlock > stop {
				logger.Debug("stopping at merged block file above stop block", zap.String("filename", filename), zap.Uint64("stop", stop))
				return io.EOF
			}

			if startBlock+100 < start {
				logger.Debug("skipping merged block file below start block", zap.String("filename", filename))
				return nil
			}

			rc, err := srcStore.OpenObject(ctx, filename)
			if err != nil {
				return fmt.Errorf("failed to open %s: %w", filename, err)
			}
			defer rc.Close()

			br, err := bstream.NewDBinBlockReader(rc)
			if err != nil {
				return fmt.Errorf("creating block reader: %w", err)
			}

			blocks := make([]*pbbstream.Block, 100)
			blocksRead := 0
			for {
				block, err := br.Read()
				if err == io.EOF {
					break
				}
				if err != nil {
					return fmt.Errorf("reading block from bundle %s: %w", filename, err)
				}

				ethBlock := &pbeth.Block{}
				err = block.Payload.UnmarshalTo(ethBlock)
				if err != nil {
					return fmt.Errorf("unmarshaling eth block: %w", err)
				}

				removeGasChangesFromEthereumBlock(ethBlock)

				block, err = blockEncoder.Encode(firecore.BlockEnveloppe{Block: ethBlock, LIBNum: block.LibNum})
				if err != nil {
					return fmt.Errorf("re-packing the block: %w", err)
				}
				blocks[blocksRead] = block
				blocksRead++
			}
			if blocksRead != 100 {
				return fmt.Errorf("block count mismatch: expected 100 blocks, got %d", blocksRead)
			}
			if err := writeMergedBlocks(startBlock, destStore, blocks); err != nil {
				return fmt.Errorf("writing merged block %d: %w", startBlock, err)
			}

			lastFileProcessed = filename

			return nil
		})
		fmt.Printf("Last file processed: %s.dbin.zst\n", lastFileProcessed)

		if err == io.EOF {
			return nil
		}

		if err != nil {
			return err
		}

		return nil
	}
}

func removeGasChangesFromEthereumBlock(block *pbeth.Block) {
	if block == nil {
		return
	}

	for _, systemCall := range block.SystemCalls {
		systemCall.GasChanges = nil
	}

	for _, trace := range block.TransactionTraces {
		for _, call := range trace.Calls {
			call.GasChanges = nil
		}
	}
}
