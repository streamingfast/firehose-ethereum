package main

import (
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"
	"sync"

	"github.com/streamingfast/eth-go"

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

	cmd.PersistentFlags().StringSliceP("headers", "H", nil, "Headers to send with each RPC request (ex: '-H \"key1: value1\" -H \"key2: value2\"')")
	cmd.PersistentFlags().Int("max-concurrency", 0, "Maximum number of concurrent block processing jobs within a file (0 = auto-detect: GOMAXPROCS)")
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
		maxConcurrency := sflags.MustGetInt(cmd, "max-concurrency")

		if maxConcurrency == 0 {
			maxConcurrency = runtime.GOMAXPROCS(0)
			if maxConcurrency < 1 {
				maxConcurrency = 1
			}
		}

		if stop <= start {
			return fmt.Errorf("stop block must be greater than start block")
		}

		logger.Info("starting fix withdrawals processing",
			zap.String("src_store", args[0]),
			zap.String("dest_store", args[1]),
			zap.String("rpc_endpoint", rpcEndpoint),
			zap.Uint64("start_block", start),
			zap.Uint64("stop_block", stop),
			zap.Int("max_concurrency", maxConcurrency),
		)

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

			// Read all blocks first
			var rawBlocks []*pbbstream.Block
			for {
				block, err := br.Read()
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					return fmt.Errorf("reading block from bundle %s: %w", filename, err)
				}
				rawBlocks = append(rawBlocks, block)
			}

			if len(rawBlocks) != 100 {
				fmt.Printf("ERROR: incorrect block count in merged file %s: read %d blocks, expected 100 (start_block=%d)\n", filename, len(rawBlocks), startBlock)
				return fmt.Errorf("expected to have read 100 blocks, we have read %d. Bailing out.", len(rawBlocks))
			}

			// Process blocks in parallel
			blocks := make([]*pbbstream.Block, 100)
			semaphore := make(chan struct{}, maxConcurrency)
			var wg sync.WaitGroup
			errorCh := make(chan error, len(rawBlocks))

			for i, rawBlock := range rawBlocks {
				wg.Add(1)
				go func(index int, block *pbbstream.Block) {
					defer wg.Done()
					semaphore <- struct{}{}        // Acquire semaphore
					defer func() { <-semaphore }() // Release semaphore

					ethBlock := &pbeth.Block{}
					if err := block.Payload.UnmarshalTo(ethBlock); err != nil {
						errorCh <- fmt.Errorf("unmarshaling eth block: %w", err)
						return
					}

					// Fetch withdrawals from RPC and populate the block
					rpcBlock, err := rpcClient.GetBlockByNumber(ctx, rpc.BlockNumber(ethBlock.Number))
					if err != nil {
						errorCh <- fmt.Errorf("fetching rpc block %d: %w", ethBlock.Number, err)
						return
					}

					if rpcBlock.Hash.String() != eth.Hash(ethBlock.Hash).String() {
						errorCh <- fmt.Errorf("rpc block hash mismatch for block %d: expected %s, got %s", ethBlock.Number, ethBlock.Hash, rpcBlock.Hash.String())
						return
					}

					if rpcBlock.Withdrawals != nil {
						ethBlock.Withdrawals = convertRPCWithdrawalsToPB(rpcBlock.Withdrawals)
					} else {
						ethBlock.Withdrawals = nil
					}

					balanceChangeWithdrawalCount := countBalanceChangeWithdrawal(ethBlock)
					if balanceChangeWithdrawalCount > 0 && len(ethBlock.Withdrawals) != balanceChangeWithdrawalCount {
						errorCh <- fmt.Errorf("sanity check, mismatch between RPC withdrawals and balance changes for block %d", ethBlock.Number)
						return
					}

					processedBlock, err := blockEncoder.Encode(firecore.BlockEnveloppe{Block: ethBlock, LIBNum: block.LibNum})
					if err != nil {
						errorCh <- fmt.Errorf("re-packing the block: %w", err)
						return
					}

					blocks[index] = processedBlock
				}(i, rawBlock)
			}

			// Wait for all jobs to complete
			wg.Wait()
			close(errorCh)

			// Check for errors
			var allErrs error
			for err := range errorCh {
				allErrs = errors.Join(allErrs, err)
			}
			if allErrs != nil {
				return fmt.Errorf("errors encountered while processing blocks: %w", allErrs)
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
func convertRPCWithdrawalsToPB(ws []rpc.Withdrawal) []*pbeth.Withdrawal {
	if ws == nil {
		return nil
	}
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

func countBalanceChangeWithdrawal(block *pbeth.Block) int {
	count := 0
	for _, bc := range block.BalanceChanges {
		if bc.Reason == pbeth.BalanceChange_REASON_WITHDRAWAL {
			count += 1
		}
	}
	return count
}
