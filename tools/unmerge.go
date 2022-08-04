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
	Use:   "unmerge <src-store> <dest-store> <start> <stop>",
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

	startFrom := fmt.Sprintf("%010d", start-(start%100))
	err = srcStore.WalkFrom(ctx, "", startFrom, func(filename string) error {
		zlog.Debug("checking 100-block file", zap.String("filename", filename))
		startBlock := mustParseUint64(filename)
		if startBlock > stop {
			return io.EOF
		}

		if startBlock+100 < start {
			return nil
		}

		rc, err := srcStore.OpenObject(ctx, filename)
		if err != nil {
			return fmt.Errorf("failed to open %s: %w", filename, err)
		}
		defer rc.Close()

		reader, err := bstream.GetBlockReaderFactory.New(rc)
		if err != nil {
			return fmt.Errorf("get block reader: %w", err)
		}

		// iterate through the blocks in the file
		for {
			block, err := reader.Read()
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

			//write block data to pipe
			go func(block *bstream.Block) {
				var err error
				defer func() {
					pw.CloseWithError(err)
				}()

				var bw bstream.BlockWriter
				bw, err = bstream.GetBlockWriterFactory.New(pw)
				if err != nil {
					zlog.Error("error creating block writer", zap.Error(err))
					return
				}

				err = bw.Write(block)
				if err != nil {
					zlog.Error("error writing block", zap.Error(err))
					return
				}
			}(block)

			//read block data from pipe and write block data to dest store
			err = destStore.WriteObject(ctx, oneblockFilename, pr)
			if err != nil {
				return fmt.Errorf("error writing block %d to %s: %w", block.Number, oneblockFilename, err)
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
