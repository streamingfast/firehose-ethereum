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
	Use:   "unmerge <store-url>",
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
		return err
	}

	destStore, err := dstore.NewDBinStore(args[1])
	if err != nil {
		return err
	}

	start := mustParseUint64(args[2])
	stop := mustParseUint64(args[3])

	err = srcStore.Walk(ctx, "", func(filename string) error {
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
			return err
		}
		defer rc.Close()

		reader, err := bstream.GetBlockReaderFactory.New(rc)
		if err != nil {
			return err
		}

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

			pr, pw := io.Pipe()

			go func(block *bstream.Block) {
				bw, _ := bstream.GetBlockWriterFactory.New(pw)
				err := bw.Write(block)
				if err != nil {
					pw.CloseWithError(err)
					return
				}
				pw.Close()
			}(block)

			oneblockFilename := bstream.BlockFileNameWithSuffix(block, "extracted")
			err = destStore.WriteObject(ctx, oneblockFilename, pr)
			if err != nil {
				return err
			}
			zlog.Info(fmt.Sprintf("wrote block %d to %s", block.Number, oneblockFilename))
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
