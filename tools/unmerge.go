package tools

import (
	"bytes"
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

	start := mustParseUint64(args[1])
	stop := mustParseUint64(args[2])

	destStore, err := dstore.NewDBinStore(args[3])
	if err != nil {
		return err
	}

	err = srcStore.Walk(ctx, "", func(filename string) error {
		startBlock := mustParseUint64(filename)
		if startBlock > stop {
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

			err = destStore.WriteObject(ctx, bstream.BlockFileNameWithSuffix(block, "extracted"), buf)
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	return nil
}
