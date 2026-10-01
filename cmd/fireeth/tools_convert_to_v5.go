package main

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/streamingfast/bstream"
	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	"github.com/streamingfast/cli"
	"github.com/streamingfast/dstore"
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

			Fixed on blocks of version 2:

			  - the caller of a DELEGATE call is set to the address of its closest ancestor that is
			    not a DELEGATE call.

			Fixed on blocks of version 3 and below:

			  - a root call recorded with a 'begin_ordinal' of 0 gets the ordinal the tracer left
			    unused when the call started, or a new one right after the transaction's
			    'begin_ordinal' when there is none;
			  - any other call recorded with a 'begin_ordinal' of 0 gets a new ordinal right before
			    the first thing it recorded;
			  - the genesis root call recorded with an 'end_ordinal' of 0 gets a new ordinal right
			    before the transaction's 'end_ordinal';
			  - the ordinals of the transactions are moved above those of the system calls when they
			    overlap (blocks before Prague only);
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

			Not fixed, the block alone does not hold what is needed:

			  - the input of internal CREATE calls is empty (version 3 and below);
			  - 'Call.executed_code' can be wrong (version 3 and below);
			  - the balance changes of a self-destruct are in the old order and miss the burn
			    (version 3 and below);
			  - 'Block.withdrawals' is empty, use 'fix-withdrawals' for that.
		`),
		Args: cobra.ExactArgs(4),
		RunE: createConvertToV5E(logger),
	}
	return cmd
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

				convertEthereumBlockToV5(ethBlock)

				block, err = blockEncoder.Encode(firecore.BlockEnveloppe{Block: ethBlock, LIBNum: block.LibNum})
				if err != nil {
					return fmt.Errorf("re-packing the block: %w", err)
				}
				blocks = append(blocks, block)
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
