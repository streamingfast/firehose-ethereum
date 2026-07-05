package main

import (
	"fmt"
	"io"
	"slices"
	"sort"

	"github.com/spf13/cobra"
	"github.com/streamingfast/bstream"
	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	"github.com/streamingfast/cli/sflags"
	"github.com/streamingfast/dstore"
	firecore "github.com/streamingfast/firehose-core"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"go.uber.org/zap"
)

func newRemoveGasChangesCmd(logger *zap.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove-gas-changes <src-blocks-store> <dest-blocks-store> <start-block> <stop-block>",
		Short: "remove call gas changes from blocks and rewrite the affected merged-blocks files to destination. Changes block version to v5",
		Args:  cobra.ExactArgs(4),
		RunE:  createRemoveGasChangesE(logger),
	}
	cmd.Flags().Uint64("bundle-size", 100, "Number of blocks per merged-blocks file in the source store")
	return cmd
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
		bundleSize := sflags.MustGetUint64(cmd, "bundle-size")

		if stop <= start {
			return fmt.Errorf("stop block must be greater than start block")
		}

		lastFileProcessed := ""
		startWalkFrom := fmt.Sprintf("%010d", start-(start%bundleSize))
		err = srcStore.WalkFrom(ctx, "", startWalkFrom, func(filename string) error {
			logger.Debug("checking merged block file", zap.String("filename", filename))

			startBlock := mustParseUint64(filename)

			if startBlock > stop {
				logger.Debug("stopping at merged block file above stop block", zap.String("filename", filename), zap.Uint64("stop", stop))
				return io.EOF
			}

			if startBlock+bundleSize < start {
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

			blocks := make([]*pbbstream.Block, bundleSize)
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
				if ethBlock.Ver < 5 {
					ethBlock.Ver = 5
				}

				block, err = blockEncoder.Encode(firecore.BlockEnveloppe{Block: ethBlock, LIBNum: block.LibNum})
				if err != nil {
					return fmt.Errorf("re-packing the block: %w", err)
				}
				blocks[blocksRead] = block
				blocksRead++
			}
			if uint64(blocksRead) != bundleSize {
				return fmt.Errorf("block count mismatch: expected %d blocks, got %d", bundleSize, blocksRead)
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

	removedOrdinals := collectRemovedGasChangeOrdinals(block)
	shiftBlockOrdinals(block, removedOrdinals)

	for _, systemCall := range block.SystemCalls {
		systemCall.GasChanges = nil
	}

	for _, trace := range block.TransactionTraces {
		for _, call := range trace.Calls {
			call.GasChanges = nil
		}
	}
}

func collectRemovedGasChangeOrdinals(block *pbeth.Block) []uint64 {
	if block == nil {
		return nil
	}

	var out []uint64
	for _, systemCall := range block.SystemCalls {
		for _, gasChange := range systemCall.GasChanges {
			if gasChange.Ordinal > 0 {
				out = append(out, gasChange.Ordinal)
			}
		}
	}

	for _, trace := range block.TransactionTraces {
		for _, call := range trace.Calls {
			for _, gasChange := range call.GasChanges {
				if gasChange.Ordinal > 0 {
					out = append(out, gasChange.Ordinal)
				}
			}
		}
	}

	if len(out) == 0 {
		return nil
	}

	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return slices.Compact(out)
}

func shiftBlockOrdinals(block *pbeth.Block, removedGasOrdinals []uint64) {
	if len(removedGasOrdinals) == 0 {
		return
	}

	for _, balanceChange := range block.BalanceChanges {
		balanceChange.Ordinal = shiftOrdinal(balanceChange.Ordinal, removedGasOrdinals)
	}

	for _, codeChange := range block.CodeChanges {
		codeChange.Ordinal = shiftOrdinal(codeChange.Ordinal, removedGasOrdinals)
	}

	for _, systemCall := range block.SystemCalls {
		shiftCallOrdinals(systemCall, removedGasOrdinals)
	}

	for _, trace := range block.TransactionTraces {
		trace.BeginOrdinal = shiftOrdinal(trace.BeginOrdinal, removedGasOrdinals)
		trace.EndOrdinal = shiftOrdinal(trace.EndOrdinal, removedGasOrdinals)

		if trace.Receipt != nil {
			for _, log := range trace.Receipt.Logs {
				log.Ordinal = shiftOrdinal(log.Ordinal, removedGasOrdinals)
			}
		}

		for _, call := range trace.Calls {
			shiftCallOrdinals(call, removedGasOrdinals)
		}
	}
}

func shiftCallOrdinals(call *pbeth.Call, removedGasOrdinals []uint64) {
	if call == nil {
		return
	}

	call.BeginOrdinal = shiftOrdinal(call.BeginOrdinal, removedGasOrdinals)
	call.EndOrdinal = shiftOrdinal(call.EndOrdinal, removedGasOrdinals)

	for _, storageChange := range call.StorageChanges {
		storageChange.Ordinal = shiftOrdinal(storageChange.Ordinal, removedGasOrdinals)
	}

	for _, balanceChange := range call.BalanceChanges {
		balanceChange.Ordinal = shiftOrdinal(balanceChange.Ordinal, removedGasOrdinals)
	}

	for _, nonceChange := range call.NonceChanges {
		nonceChange.Ordinal = shiftOrdinal(nonceChange.Ordinal, removedGasOrdinals)
	}

	for _, log := range call.Logs {
		log.Ordinal = shiftOrdinal(log.Ordinal, removedGasOrdinals)
	}

	for _, codeChange := range call.CodeChanges {
		codeChange.Ordinal = shiftOrdinal(codeChange.Ordinal, removedGasOrdinals)
	}

	for _, accountCreation := range call.AccountCreations {
		accountCreation.Ordinal = shiftOrdinal(accountCreation.Ordinal, removedGasOrdinals)
	}
}

func shiftOrdinal(value uint64, removedOrdinals []uint64) uint64 {
	if value == 0 || len(removedOrdinals) == 0 {
		return value
	}

	shift := sort.Search(len(removedOrdinals), func(i int) bool { return removedOrdinals[i] > value })
	if shift == 0 {
		return value
	}

	return value - uint64(shift)
}
