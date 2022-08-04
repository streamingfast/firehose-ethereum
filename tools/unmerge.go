package tools

import (
	"bytes"
	"fmt"
	"io"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/dstore"
)

var unmergeBlocksCmd = &cobra.Command{
	Use:   "unmerge <store-url>",
	Short: "Checks for any holes in merged blocks as well as ensuring merged blocks integrity",
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

			buf := bytes.NewBuffer(nil)

			writer, err := bstream.GetBlockWriterFactory.New(buf)
			if err != nil {
				return err
			}
			err = writer.Write(block)
			if err != nil {
				return err
			}

			oneblockFilename := bstream.BlockFileNameWithSuffix(block, "extracted")
			err = destStore.WriteObject(ctx, oneblockFilename, buf)
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
