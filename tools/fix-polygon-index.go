package tools

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/dstore"
	"github.com/streamingfast/firehose-ethereum/types"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"go.uber.org/zap"
)

var fixPolygonIndexCmd = &cobra.Command{
	Use:   "fix-polygon-index <src-blocks-store> <dest-blocks-store> <start-block> <stop-block>",
	Short: "look for blocks containing a single transaction with index==1 (where it should be index==0) and rewrite the affected 100-block-files to dest. it does not rewrite correct merged-files-bundles",
	Args:  cobra.ExactArgs(4),
	RunE:  fixPolygonIndexE,
}

func init() {
	Cmd.AddCommand(fixPolygonIndexCmd)
}

func fixPolygonIndexE(cmd *cobra.Command, args []string) error {
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

	startWalkFrom := fmt.Sprintf("%010d", start-(start%100))
	err = srcStore.WalkFrom(ctx, "", startWalkFrom, func(filename string) error {
		zlog.Debug("checking merged block file", zap.String("filename", filename))

		startBlock := mustParseUint64(filename)

		if startBlock > stop {
			zlog.Debug("skipping merged block file", zap.String("reason", "past stop block"), zap.String("filename", filename))
			return io.EOF
		}

		if startBlock+100 < start {
			zlog.Debug("skipping merged block file", zap.String("reason", "before start block"), zap.String("filename", filename))
			return nil
		}

		rc, err := srcStore.OpenObject(ctx, filename)
		if err != nil {
			return fmt.Errorf("failed to open %s: %w", filename, err)
		}
		defer rc.Close()

		br, err := bstream.GetBlockReaderFactory.New(rc)
		if err != nil {
			return fmt.Errorf("creating block reader: %w", err)
		}

		var mustWrite bool
		blocks := make([]*bstream.Block, 100)
		i := 0
		for {
			block, err := br.Read()
			if err == io.EOF {
				break
			}

			ethBlock := block.ToProtocol().(*pbeth.Block)
			if len(ethBlock.TransactionTraces) == 1 &&
				ethBlock.TransactionTraces[0].Index == 1 {
				fmt.Println("ERROR FOUND AT BLOCK", block.Number)
				mustWrite = true
				ethBlock.TransactionTraces[0].Index = 0
				block, err = types.BlockFromProto(ethBlock, block.LibNum)
				if err != nil {
					return fmt.Errorf("re-packing the block: %w", err)
				}
			}
			blocks[i] = block
			i++
		}
		if i != 100 {
			return fmt.Errorf("expected to have read 100 blocks, we have read %d. Bailing out.", i)
		}
		if mustWrite {
			if err := writeMergedBlocks(startBlock, destStore, blocks); err != nil {
				return fmt.Errorf("writing merged block %d: %w", startBlock, err)
			}
		}

		return nil
	})

	if err == io.EOF {
		return nil
	}

	if err != nil {
		return err
	}

	return nil
}

func writeMergedBlocks(lowBlockNum uint64, store dstore.Store, blocks []*bstream.Block) error {
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

		blockWriter, err := bstream.GetBlockWriterFactory.New(pw)
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
	out, err := strconv.ParseUint(in, 0, 64)
	if err != nil {
		panic(fmt.Errorf("unable to parse %q as uint64: %w", in, err))
	}

	return out
}
