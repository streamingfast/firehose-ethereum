package main

import (
	"context"
	"fmt"
	"io"
	"strconv"

	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"

	"github.com/spf13/cobra"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/dstore"
	firecore "github.com/streamingfast/firehose-core"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"go.uber.org/zap"
)

func newFixOrdinalsCmd(logger *zap.Logger) *cobra.Command {
	return &cobra.Command{
		Use:   "fix-ordinals <src-blocks-store> <dest-blocks-store> <start-block> <stop-block>",
		Short: "look for blocks containing a single transaction with index==1 (where it should be index==0) and rewrite the affected 100-block-files to dest. it does not rewrite correct merged-files-bundles",
		Args:  cobra.ExactArgs(4),
		RunE:  createFixOrdinalsE(logger),
	}
}

func createFixOrdinalsE(logger *zap.Logger) firecore.CommandExecutor {
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
			i := 0
			for {
				block, err := br.Read()
				if err == io.EOF {
					break
				}

				ethBlock := &pbeth.Block{}
				err = block.Payload.UnmarshalTo(ethBlock)
				if err != nil {
					return fmt.Errorf("unmarshaling eth block: %w", err)
				}

				ordinal := uint64(0)
				for _, trace := range ethBlock.TransactionTraces {
					trace.BeginOrdinal = ordinal
					ordinal++
					for _, log := range trace.Receipt.Logs {
						log.Ordinal = ordinal
						ordinal++
					}
					trace.EndOrdinal = ordinal
					ordinal++
				}

				block, err = blockEncoder.Encode(firecore.BlockEnveloppe{Block: ethBlock, LIBNum: block.LibNum})
				if err != nil {
					return fmt.Errorf("re-packing the block: %w", err)
				}
				blocks[i] = block
				i++
			}
			if i != 100 {
				return fmt.Errorf("expected to have read 100 blocks, we have read %d. Bailing out.", i)
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

func writeMergedBlocks(lowBlockNum uint64, store dstore.Store, blocks []*pbbstream.Block) error {
	file := filename(lowBlockNum)
	fmt.Printf("writing merged file %s.dbin.zst\n", file)

	if len(blocks) == 0 {
		return fmt.Errorf("no blocks to write to bundle")
	}

	pr, pw := io.Pipe()

	go func() {
		var err error
		defer func() {
			pw.CloseWithError(err)
		}()

		blockWriter, err := bstream.NewDBinBlockWriter(pw)
		if err != nil {
			return
		}

		for _, blk := range blocks {
			err = blockWriter.Write(blk)
			if err != nil {
				return
			}
		}
	}()

	return store.WriteObject(context.Background(), file, pr)
}

func filename(num uint64) string {
	return fmt.Sprintf("%010d", num)
}

func mustParseUint64(in string) uint64 {
	out, err := strconv.ParseUint(in, 10, 64)
	if err != nil {
		panic(fmt.Errorf("unable to parse %q as uint64: %w", in, err))
	}

	return out
}
