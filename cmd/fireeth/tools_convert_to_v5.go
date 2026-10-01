package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/streamingfast/bstream"
	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	"github.com/streamingfast/cli"
	"github.com/streamingfast/cli/sflags"
	"github.com/streamingfast/dstore"
	"github.com/streamingfast/eth-go"
	"github.com/streamingfast/eth-go/rpc"
	firecore "github.com/streamingfast/firehose-core"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"go.uber.org/zap"
)

func newConvertToV5Cmd(logger *zap.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "convert-to-v5 <src-blocks-store> <dest-blocks-store> <start-block> <stop-block>",
		Short: "convert blocks of version 2, 3 or 4 to version 5 and write the merged-blocks files to destination",
		Long: cli.Dedent(`
			Rewrites every merged-blocks file of the range so its blocks match what a version 5
			tracer records, and sets their version to 5.

			Removed from every block:

			  - all gas changes;
			  - all account creations;
			  - balance, nonce, code and storage changes whose old and new values are equal;
			  - keccak preimages above 256 bytes, and those that do not explain a storage slot
			    written by the same transaction or system call (directly, at an array or struct
			    offset, or through up to 16 levels of nested hashing);
			  - the single empty topic of a log without topics emitted by a reverted call.

			Fixed on every block:

			  - 'Log.index' counts every log of the transaction in the order they were emitted,
			    logs of reverted calls included. A receipt log gets the index of its copy in the
			    calls. Version 3 and below counted the receipt logs only.

			Fixed on blocks of version 2:

			  - the caller of a DELEGATE call is set to the address of its closest ancestor that is
			    not a DELEGATE call.

			Fixed on blocks of version 3 and below:

			  - a root call recorded with a 'begin_ordinal' of 0 gets the ordinal the tracer left
			    unused when the call started, or a new one right after the transaction's
			    'begin_ordinal' when there is none;
			  - any other call recorded with a 'begin_ordinal' of 0 gets the closest unused ordinal
			    below the first thing it recorded, or a new one right before it when there is none;
			  - the genesis root call recorded with an 'end_ordinal' of 0 gets a new ordinal right
			    before the transaction's 'end_ordinal';
			  - the ordinals of the transactions are moved above those of the system calls when a
			    transaction and a system call use the same ordinal (blocks before Prague only).
			    System calls that ran inside a transaction (Arbitrum) keep their place;
			  - 'TransactionTrace.return_data' is copied from the root call when empty;
			  - the input of a root CREATE call is set to the transaction's input when empty;
			  - a keccak preimage recorded as "." (empty input) becomes an empty string.

			Applied last, to every block:

			  - ordinals are renumbered 1, 2, 3, ... in their existing order, a log and its copy in
			    the receipt keep sharing one;
			  - call input and return data limits of the tracers: once the internal calls of a
			    transaction hold more than 50 MiB of input, later calls keep only their 4-byte
			    selector ('input_truncated'), and once they hold more than 25 MiB of return data,
			    later calls keep none ('return_data_truncated'). Both limits are halved while the
			    block encodes to more than 1 GiB. The root call is never truncated.

			Blocks already at version 5 go through the same steps. Only those written by a tracer
			that did not filter keccak preimages or limit call data change.

			Not fixed, the block alone does not hold what is needed. On these fields the output
			differs from the blocks a version 5 tracer produces for the same chain:

			  - the input of internal CREATE calls is empty (version 3 and below);
			  - 'Call.executed_code' can be wrong (version 3 and below): it is set on calls to
			    precompiles and on the root call of Arbitrum internal transactions, and missing
			    on calls without input to an address that has code;
			  - 'Call.address_delegates_to' is empty on calls to an account that delegates its code
			    (EIP-7702), and 'Call.executed_code' is not set on them (version 3 and below);
			  - on Prague blocks of version 3 and below, transactions and system calls keep sharing
			    ordinals, so the ordinals differ from those of a version 5 tracer;
			  - the balance changes of a self-destruct are in the old order and miss the burn
			    (version 3 and below);
			  - 'Block.withdrawals' stays empty when '--rpc-endpoint' is not set.

			Withdrawals:

			When '--rpc-endpoint' is set, 'Block.withdrawals' is fetched from it
			(eth_getBlockByNumber) on the blocks without withdrawals whose header
			'withdrawals_root' is set and is not the root of an empty list. Chains and block
			ranges without withdrawals make no RPC call. OP Stack blocks, recognized by their
			first transaction being a deposit, are never fetched: they have no withdrawals, and
			from Isthmus onward their 'withdrawals_root' holds another value.

			The lookups of a file run while its blocks are read and converted. Without
			'--rpc-endpoint', the command reports how many blocks were left without their
			withdrawals.
		`),
		Args: cobra.ExactArgs(4),
		RunE: createConvertToV5E(logger),
	}

	cmd.Flags().String("rpc-endpoint", "", "RPC endpoint the missing withdrawals are fetched from, they are left out when empty")
	cmd.Flags().StringSlice("rpc-endpoint-headers", nil, "Headers to send with each RPC request (ex: '--rpc-endpoint-headers \"key1: value1\" --rpc-endpoint-headers \"key2: value2\"')")
	cmd.Flags().Int("rpc-endpoint-max-concurrency", 0, "Maximum number of concurrent RPC requests (0 = auto-detect: GOMAXPROCS)")
	return cmd
}

// emptyWithdrawalsRoot is the header `withdrawals_root` of a block without withdrawals.
var emptyWithdrawalsRoot = eth.MustNewHash("0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421")

// isMissingWithdrawals tells if the header of the block announces withdrawals that
// `Block.withdrawals` does not hold.
func isMissingWithdrawals(block *pbeth.Block) bool {
	// OP Stack blocks never list withdrawals, from Isthmus onward their `withdrawals_root` is the
	// storage root of the L2 to L1 message passer contract
	if len(block.TransactionTraces) > 0 && block.TransactionTraces[0].Type == pbeth.TransactionTrace_TRX_TYPE_OPTIMISM_DEPOSIT {
		return false
	}

	root := block.GetHeader().GetWithdrawalsRoot()
	return len(block.Withdrawals) == 0 && len(root) != 0 && !bytes.Equal(root, emptyWithdrawalsRoot)
}

// withdrawalsFetch is the RPC lookup of the withdrawals of one block. `withdrawals` and `err`
// are set once `done` is closed.
type withdrawalsFetch struct {
	done        chan struct{}
	withdrawals []*pbeth.Withdrawal
	err         error
}

// startWithdrawalsFetch starts the lookup of the withdrawals of the block and returns right
// away. Everything it needs from the block is read before it returns, the block can be modified
// while the lookup runs. `semaphore` limits how many lookups run at once.
func startWithdrawalsFetch(ctx context.Context, rpcClient *rpc.Client, semaphore chan struct{}, block *pbeth.Block) *withdrawalsFetch {
	fetch := &withdrawalsFetch{done: make(chan struct{})}
	number, hash := block.Number, eth.Hash(block.Hash)
	balanceChangeWithdrawalCount := countBalanceChangeWithdrawal(block)

	go func() {
		defer close(fetch.done)
		semaphore <- struct{}{}
		defer func() { <-semaphore }()

		fetch.withdrawals, fetch.err = fetchWithdrawalsFromRPC(ctx, rpcClient, number, hash, balanceChangeWithdrawalCount)
	}()

	return fetch
}

func createConvertToV5E(logger *zap.Logger) firecore.CommandExecutor {
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

		var rpcClient *rpc.Client
		if rpcEndpoint := sflags.MustGetString(cmd, "rpc-endpoint"); rpcEndpoint != "" {
			rpcClient = newRPCClientWithHeaders(rpcEndpoint, sflags.MustGetStringSlice(cmd, "rpc-endpoint-headers"))
		}

		rpcMaxConcurrency := sflags.MustGetInt(cmd, "rpc-endpoint-max-concurrency")
		if rpcMaxConcurrency <= 0 {
			rpcMaxConcurrency = max(runtime.GOMAXPROCS(0), 1)
		}
		rpcSemaphore := make(chan struct{}, rpcMaxConcurrency)
		blocksLeftWithoutWithdrawals := 0

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

			var blocks []*pbbstream.Block
			encode := func(index int, ethBlock *pbeth.Block, libNum uint64) error {
				blocks[index], err = blockEncoder.Encode(firecore.BlockEnveloppe{Block: ethBlock, LIBNum: libNum})
				if err != nil {
					return fmt.Errorf("re-packing the block: %w", err)
				}
				return nil
			}

			// A block waiting for its withdrawals is kept decoded until its lookup ends, the
			// others are encoded right away
			type pendingBlock struct {
				index    int
				ethBlock *pbeth.Block
				libNum   uint64
				fetch    *withdrawalsFetch
			}
			var pending []*pendingBlock
			encodePending := func(wait bool) error {
				remaining := pending[:0]
				for _, p := range pending {
					if !wait {
						select {
						case <-p.fetch.done:
						default:
							remaining = append(remaining, p)
							continue
						}
					}

					<-p.fetch.done
					if p.fetch.err != nil {
						return fmt.Errorf("adding withdrawals to block %d: %w", p.ethBlock.Number, p.fetch.err)
					}
					p.ethBlock.Withdrawals = p.fetch.withdrawals
					if err := encode(p.index, p.ethBlock, p.libNum); err != nil {
						return err
					}
				}
				clear(pending[len(remaining):])
				pending = remaining
				return nil
			}

			for {
				block, err := br.Read()
				if err == io.EOF {
					break
				}
				if err != nil {
					return fmt.Errorf("reading block from bundle %s: %w", filename, err)
				}

				ethBlock := &pbeth.Block{}
				err = block.Payload.UnmarshalTo(ethBlock)
				if err != nil {
					return fmt.Errorf("unmarshaling eth block: %w", err)
				}

				// The lookup runs while this block and the next ones are read and converted
				var fetch *withdrawalsFetch
				if isMissingWithdrawals(ethBlock) {
					if rpcClient != nil {
						fetch = startWithdrawalsFetch(ctx, rpcClient, rpcSemaphore, ethBlock)
					} else {
						blocksLeftWithoutWithdrawals++
					}
				}

				convertEthereumBlockToV5(ethBlock)

				index := len(blocks)
				blocks = append(blocks, nil)
				if fetch == nil {
					if err := encode(index, ethBlock, block.LibNum); err != nil {
						return err
					}
				} else {
					pending = append(pending, &pendingBlock{index: index, ethBlock: ethBlock, libNum: block.LibNum, fetch: fetch})
				}

				if err := encodePending(false); err != nil {
					return err
				}
			}

			if err := encodePending(true); err != nil {
				return err
			}

			if err := writeMergedBlocks(startBlock, destStore, blocks); err != nil {
				return fmt.Errorf("writing merged block %d: %w", startBlock, err)
			}

			lastFileProcessed = filename

			return nil
		})
		fmt.Printf("Last file processed: %s.dbin.zst\n", lastFileProcessed)
		if blocksLeftWithoutWithdrawals > 0 {
			fmt.Printf("WARNING: %d blocks have withdrawals that were left out, set --rpc-endpoint to fetch them\n", blocksLeftWithoutWithdrawals)
		}

		if err == io.EOF {
			return nil
		}

		if err != nil {
			return err
		}

		return nil
	}
}
