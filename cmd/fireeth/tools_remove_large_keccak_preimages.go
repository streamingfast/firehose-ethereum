package main

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/streamingfast/bstream"
	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	"github.com/streamingfast/cli"
	"github.com/streamingfast/dstore"
	firecore "github.com/streamingfast/firehose-core"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"go.uber.org/zap"
)

func newRemoveLargeKeccakPreimagesCmd(logger *zap.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove-large-keccak-preimages <src-blocks-store> <dest-blocks-store> <start-block> <stop-block>",
		Short: "remove keccak preimages above 256 bytes from blocks and rewrite the affected merged-blocks files to destination",
		Long: cli.Dedent(`
			Brings already-written blocks in line with the tracers that cap 'Call.keccak_preimages'
			at 256 bytes, so a range written before the cap compares equal to one written after it.

			An entry is dropped when its preimage exceeds 256 bytes. Map values are hex-encoded, so
			that is a value longer than 512 characters. Nothing else in the block is touched: keccak
			preimages carry no ordinal, so no ordinal is shifted and the block version is unchanged.
		`),
		Args: cobra.ExactArgs(4),
		RunE: createRemoveLargeKeccakPreimagesE(logger),
	}
	return cmd
}

func createRemoveLargeKeccakPreimagesE(logger *zap.Logger) firecore.CommandExecutor {
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
		bundleSize, err := firecore.GetMergedBlocksBundleSizeFlag(cmd)
		if err != nil {
			return err
		}

		if stop <= start {
			return fmt.Errorf("stop block must be greater than start block")
		}

		lastFileProcessed := ""
		removedTotal := 0
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
			removedInFile := 0
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

				removedInFile += removeLargeKeccakPreimagesFromEthereumBlock(ethBlock)

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

			logger.Debug("rewrote merged block file", zap.String("filename", filename), zap.Int("removed_preimages", removedInFile))
			removedTotal += removedInFile
			lastFileProcessed = filename

			return nil
		})
		fmt.Printf("Removed %d keccak preimages above %d bytes. Last file processed: %s.dbin.zst\n", removedTotal, maxComparableKeccakPreimageSize, lastFileProcessed)

		if err == io.EOF {
			return nil
		}

		if err != nil {
			return err
		}

		return nil
	}
}
