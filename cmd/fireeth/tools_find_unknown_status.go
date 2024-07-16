package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/streamingfast/bstream"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"

	"github.com/spf13/cobra"
	"github.com/streamingfast/dstore"
	firecore "github.com/streamingfast/firehose-core"
	"go.uber.org/zap"
)

func newScanForUnknownStatusCmd(logger *zap.Logger) *cobra.Command {
	return &cobra.Command{
		Use:   "find-unknown-status <src-blocks-store> <dst-store> <start-block> <stop-block>",
		Short: "look for blocks with empty receipts",
		Args:  cobra.ExactArgs(4),
		RunE:  scanForUnknownStatusE(logger),
	}
}

func scanForUnknownStatusE(logger *zap.Logger) firecore.CommandExecutor {
	return func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		srcStore, err := dstore.NewDBinStore(args[0])
		if err != nil {
			return fmt.Errorf("unable to create source store: %w", err)
		}

		outputStore, err := dstore.NewDBinStore(args[1])
		if err != nil {
			return fmt.Errorf("unable to create output store: %w", err)
		}

		start := mustParseUint64(args[2])
		stop := mustParseUint64(args[3])

		if stop <= start {
			return fmt.Errorf("stop block must be greater than start block")
		}

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

			blocks := make([]uint64, 0, 100)
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

				for _, trace := range ethBlock.TransactionTraces {
					if trace.Status == pbeth.TransactionTraceStatus_UNKNOWN {
						blocks = append(blocks, block.Number)
						logger.Info("found block with empty receipt", zap.Uint64("block_num", ethBlock.Number))
						break
					}
				}
			}

			if len(blocks) > 0 {
				data, _ := json.Marshal(blocks)
				err = outputStore.WriteObject(ctx, fmt.Sprintf("%010d_%d.json", startBlock, len(blocks)), bytes.NewBuffer(data))
				if err != nil {
					return fmt.Errorf("failed to write output file: %w", err)
				}
			}

			return nil
		})
		if err != nil && err != io.EOF {
			return fmt.Errorf("failed to walk source store: %w", err)
		}
		return nil
	}
}
