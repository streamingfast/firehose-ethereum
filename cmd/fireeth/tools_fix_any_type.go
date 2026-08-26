package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/streamingfast/bstream"
	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	"github.com/streamingfast/dstore"
	firecore "github.com/streamingfast/firehose-core"
	"go.uber.org/zap"
)

func newFixAnyTypeCmd(logger *zap.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fix-any-type <src-blocks-store> <dest-blocks-store> <start-block> <stop-block>",
		Short: "look for merged blocks stored with version 0 (or missing 'type.googleapis.com/' prefix) and rewrite them.",
		Args:  cobra.ExactArgs(4),
		RunE:  createFixAnyTypeE(logger),
	}
	return cmd
}

func createFixAnyTypeE(logger *zap.Logger) firecore.CommandExecutor {
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
			needReWrite := false
			if br.Header.Version == 0 {
				needReWrite = true
			}
			i := 0
			for {
				block, err := br.Read()
				if err == io.EOF {
					break
				}
				if err != nil {
					return fmt.Errorf("reading block from %s: %w", filename, err)
				}
				if !strings.HasPrefix(block.Payload.TypeUrl, "type.googleapis.com/") {
					fmt.Println("found block with missing type url prefix", block.Payload.TypeUrl)
					block.Payload.TypeUrl = "type.googleapis.com/" + block.Payload.TypeUrl
					needReWrite = true
				}
				blocks[i] = block
				i++
			}

			if uint64(i) != bundleSize {
				return fmt.Errorf("expected to have read %d blocks, we have read %d. Bailing out.", bundleSize, i)
			}

			if needReWrite {
				if err := writeMergedBlocks(startBlock, destStore, blocks); err != nil {
					return fmt.Errorf("writing merged block %d: %w", startBlock, err)
				}
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
