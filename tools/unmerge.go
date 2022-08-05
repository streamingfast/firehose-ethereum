package tools

import (
	"fmt"
	"io"
	"strconv"

	"go.uber.org/zap"

	"github.com/spf13/cobra"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/dstore"
)

var unmergeBlocksCmd = &cobra.Command{
	Use:   "unmerge <src-merged-blocks-store> <dest-one-blocks-store> <start-block> <stop-block>",
	Short: "unmerges merged block files into one-block-files",
	Args:  cobra.ExactArgs(4),
	RunE:  unmergeBlocksE,
}

func init() {
	Cmd.AddCommand(unmergeBlocksCmd)
}

func mustParseUint64(s string) uint64 {
	i, err := strconv.Atoi(s)
	if err != nil {
		panic(err)
	}
	return uint64(i)
}

func unmergeBlocksE(cmd *cobra.Command, args []string) error {
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

		// iterate through the blocks in the file
		for {
			block, err := br.Read()
			if err == io.EOF {
				break
			}

			if block.Number < start {
				continue
			}

			if block.Number > stop {
				break
			}

			oneblockFilename := bstream.BlockFileNameWithSuffix(block, "extracted")
			zlog.Debug("writing block", zap.Uint64("block_num", block.Number), zap.String("filename", oneblockFilename))

			pr, pw := io.Pipe()

			//write block data to pipe, and then close to signal end of data
			go func(block *bstream.Block) {
				var err error
				defer func() {
					pw.CloseWithError(err)
				}()

				var bw bstream.BlockWriter
				bw, err = bstream.GetBlockWriterFactory.New(pw)
				if err != nil {
					zlog.Error("creating block writer", zap.Error(err))
					return
				}

				err = bw.Write(block)
				if err != nil {
					zlog.Error("writing block", zap.Error(err))
					return
				}
			}(block)

			//read block data from pipe and write block data to dest store
			err = destStore.WriteObject(ctx, oneblockFilename, pr)
			if err != nil {
				return fmt.Errorf("writing block %d to %s: %w", block.Number, oneblockFilename, err)
			}

			zlog.Info("wrote block", zap.Uint64("block_num", block.Number), zap.String("filename", oneblockFilename))
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
