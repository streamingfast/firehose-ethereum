package main

import (
	"fmt"
	"io"

	"github.com/streamingfast/bstream"
	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"

	"github.com/spf13/cobra"
	"github.com/streamingfast/dstore"
	firecore "github.com/streamingfast/firehose-core"
	"go.uber.org/zap"
)

func newScanForEmptyReceiptsCmd(logger *zap.Logger) *cobra.Command {
	return &cobra.Command{
		Use:   "scan-for-empty-receipts <src-blocks-store> <start-block> <stop-block>",
		Short: "look for blocks with empty receipts",
		Args:  cobra.ExactArgs(3),
		RunE:  scanForEmptyReceiptsE(logger),
	}
}

func scanForEmptyReceiptsE(logger *zap.Logger) firecore.CommandExecutor {
	return func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		srcStore, err := dstore.NewDBinStore(args[0])
		if err != nil {
			return fmt.Errorf("unable to create source store: %w", err)
		}

		start := mustParseUint64(args[1])
		stop := mustParseUint64(args[2])

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

			blocks := make([]*pbbstream.Block, 100)
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
						blocks = append(blocks, block)
						logger.Info("found block with empty receipt", zap.Uint64("block_num", ethBlock.Number))
						break
					}
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
