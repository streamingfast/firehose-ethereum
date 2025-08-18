package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/streamingfast/bstream"
	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	"github.com/streamingfast/cli/sflags"
	"github.com/streamingfast/dstore"
	"github.com/streamingfast/eth-go/rpc"
	firecore "github.com/streamingfast/firehose-core"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"go.uber.org/zap"
)

func newFixWithdrawalsCmd(logger *zap.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fix-withdrawals <src-blocks-store> <dest-blocks-store> <rpc-endpoint> <start-block> <stop-block>",
		Short: "populate the Block.withdrawals field by fetching blocks from RPC and rewrite the corrected 100-block files to destination",
		Args:  cobra.ExactArgs(5),
		RunE:  createFixWithdrawalsE(logger),
	}

	cmd.PersistentFlags().StringSliceP("headers", "H", nil, "headers to send with each RPC request (ex: '-H \"key1: value1\" -H \"key2: value2\"')")
	return cmd
}

func createFixWithdrawalsE(logger *zap.Logger) firecore.CommandExecutor {
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

		rpcEndpoint := args[2]
		var opts []rpc.Option
		for _, headerStr := range sflags.MustGetStringSlice(cmd, "headers") {
			parts := strings.SplitN(headerStr, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				opts = append(opts, rpc.WithHttpHeader(key, value))
			}
		}
		rpcClient := rpc.NewClient(rpcEndpoint, opts...)

		start := mustParseUint64(args[3])
		stop := mustParseUint64(args[4])

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
				if err != nil {
					return fmt.Errorf("reading block from bundle %s: %w", filename, err)
				}

				ethBlock := &pbeth.Block{}
				if err := block.Payload.UnmarshalTo(ethBlock); err != nil {
					return fmt.Errorf("unmarshaling eth block: %w", err)
				}

				// Fetch withdrawals from RPC and populate the block
				rpcBlock, err := rpcClient.GetBlockByNumber(ctx, rpc.BlockNumber(ethBlock.Number), rpc.WithGetBlockFullTransaction())
				if err != nil {
					return fmt.Errorf("fetching rpc block %d: %w", ethBlock.Number, err)
				}

				if rpcBlock.Withdrawals != nil {
					ethBlock.Withdrawals = convertRPCWithdrawalsToPB(rpcBlock.Withdrawals)
				} else {
					ethBlock.Withdrawals = nil
				}

				block, err = blockEncoder.Encode(firecore.BlockEnveloppe{Block: ethBlock, LIBNum: block.LibNum})
				if err != nil {
					return fmt.Errorf("re-packing the block: %w", err)
				}
				blocks[i] = block
				i++
			}
			if !(i == 99 || i == 100) {
				fmt.Printf("ERROR: incorrect block count in merged file %s: read %d blocks, expected 100 (start_block=%d)\n", filename, i, startBlock)
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

// convertRPCWithdrawalsToPB converts RPC block withdrawals to protobuf withdrawals
func convertRPCWithdrawalsToPB(in interface{}) []*pbeth.Withdrawal {
	// We accept interface{} here to avoid tight coupling on the exact rpc.Withdrawal type.
	// However, we know eth-go exposes it as []rpc.Withdrawal. Perform a type assertion accordingly.
	if in == nil {
		return nil
	}
	// Try common concrete types
	if ws, ok := in.([]rpc.Withdrawal); ok {
		out := make([]*pbeth.Withdrawal, len(ws))
		for i := range ws {
			out[i] = &pbeth.Withdrawal{
				Index:          uint64(ws[i].Index),
				ValidatorIndex: uint64(ws[i].Validator),
				Address:        ws[i].Address.Bytes(),
				Amount:         uint64(ws[i].Amount),
			}
		}
		return out
	}
	if wps, ok := in.(*[]rpc.Withdrawal); ok && wps != nil {
		ws := *wps
		out := make([]*pbeth.Withdrawal, len(ws))
		for i := range ws {
			out[i] = &pbeth.Withdrawal{
				Index:          uint64(ws[i].Index),
				ValidatorIndex: uint64(ws[i].Validator),
				Address:        ws[i].Address.Bytes(),
				Amount:         uint64(ws[i].Amount),
			}
		}
		return out
	}
	// Fallback no-op if unexpected type
	return nil
}
