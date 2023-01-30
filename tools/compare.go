// Copyright 2021 dfuse Platform Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tools

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"sync"

	"github.com/spf13/cobra"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/cli"
	"github.com/streamingfast/dstore"
	"github.com/streamingfast/eth-go/rpc"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	sftools "github.com/streamingfast/sf-tools"
	"github.com/streamingfast/substreams/block"
	"go.uber.org/multierr"
	"google.golang.org/protobuf/proto"
)

var compareBlocksCmd = &cobra.Command{
	Use:   "compare-blocks <expected_bundle> <actual_bundle> [<block_range>]",
	Short: "Checks for any differences between two block stores between a specified range. (To compare the likeness of two block ranges, for example)",
	Long: cli.Dedent(`
		The 'compare-blocks' takes in two paths to stores of merged blocks and a range specifying the blocks you
		want to compare, written as: '<start>:<finish>'. It will output the status of the likeness of every
		100,000 blocks, on completion, or on encountering a difference. Increments that contain a difference will
		be communicated as well as the blocks within that contain differences. Increments that do not have any
		differences will be outputted as identical.

		After passing through the blocks, it will output instructions on how to locate a specific difference
		based on the blocks that were given. This is done by applying the '--diff' flag before your args.

		Commands inputted with '--diff' will display the blocks that have differences, as well as the
		difference.
	`),
	Args: cobra.ExactArgs(3),
	RunE: compareBlocksE,
	Example: ExamplePrefixed("fireeth tools compare-blocks", `
		# Run over full block range
		expected_store/ actual_store/ 0:16000000

		# Run over specific block range, displaying differences in blocks
		--diff expected_store/ actual_store/ 100:200
	`),
}

func init() {
	Cmd.AddCommand(compareBlocksCmd)
	compareBlocksCmd.PersistentFlags().Bool("diff", false, "When activated, difference is displayed for each block with a difference")
}

func unifiedDiff(cnt1, cnt2 []byte) (string, error) {
	file1 := "/tmp/block-difference-expected-bundle"
	file2 := "/tmp/block-difference-received-bundle"
	err := os.WriteFile(file1, cnt1, 0600)
	if err != nil {
		return "", fmt.Errorf("writing temporary file: %w", err)
	}
	err = os.WriteFile(file2, cnt2, 0600)
	if err != nil {
		return "", fmt.Errorf("writing temporary file: %w", err)
	}

	cmd := exec.Command("diff", "-u", file1, file2)
	buffer, _ := cmd.Output()

	out := string(buffer)

	return out, nil
}

func sanitizeBlock(block *pbeth.Block) *pbeth.Block {
	for _, trxTrace := range block.TransactionTraces {
		for _, call := range trxTrace.Calls {
			if call.FailureReason != "" {
				call.FailureReason = "(varying field)"
			}
		}
	}

	return block
}

func readBundle(ctx context.Context, filename string, store dstore.Store) ([]string, map[string]*pbeth.Block, error) {

	fileReader, err := store.OpenObject(ctx, filename)
	if err != nil {
		return nil, nil, fmt.Errorf("creating reader: %w", err)
	}

	blockReader, err := bstream.GetBlockReaderFactory.New(fileReader)
	if err != nil {
		return nil, nil, fmt.Errorf("creating block reader: %w", err)
	}

	var blockHashes []string
	blocksMap := make(map[string]*pbeth.Block)
	for {
		curBlock, err := blockReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("reading blocks: %w", err)
		}

		curBlockPB := sanitizeBlock(curBlock.ToProtocol().(*pbeth.Block))
		blockHashes = append(blockHashes, string(curBlockPB.Hash))
		blocksMap[string(curBlockPB.Hash)] = curBlockPB
	}

	return blockHashes, blocksMap, nil
}

func compareBlocksE(cmd *cobra.Command, args []string) error {
	displayDiff := mustGetBool(cmd, "diff")
	chunkSize := 100000

	ctx := cmd.Context()
	blockRange, err := bstream.ParseRange(args[2])
	if err != nil {
		return fmt.Errorf("parsing range: %w", err)
	}
	blockRangeSize, err := blockRange.Size()
	if err != nil {
		return fmt.Errorf("checking for valid range: %w", err)
	}
	if blockRangeSize == 0 {
		return fmt.Errorf("invalid block range")
	}
	blockRangeAsRange := sftools.BlockRange{
		Start: blockRange.StartBlock(),
		Stop:  *blockRange.EndBlock(),
	}
	blockRangePrefix := sftools.WalkBlockPrefix(blockRangeAsRange, 100)

	// Create stores
	storeExpected, err := dstore.NewDBinStore(args[0])
	if err != nil {
		return fmt.Errorf("unable to create store at path %q: %w", args[0], err)
	}
	storeReceived, err := dstore.NewDBinStore(args[1])
	if err != nil {
		return fmt.Errorf("unable to create store at path %q: %w", args[1], err)
	}

	// Walk expected files
	differentBlocks := make(map[string]block.Range)
	blocksCountedInChunk := -1
	chunkIsGood := true
	rangeNum := 0
	err = storeExpected.Walk(ctx, blockRangePrefix, func(filename string) (err error) {
		bundleStartBlock, err := strconv.Atoi(filename)
		if err != nil {
			return fmt.Errorf("parsing filename: %w", err)
		}

		// If reached end of range
		if *blockRange.EndBlock() <= uint64(bundleStartBlock) {
			return dstore.StopIteration
		}

		// If bundle is in range
		if blockRange.Contains(uint64(bundleStartBlock)) {
			var wg sync.WaitGroup
			var bundleErrLock sync.Mutex
			var bundleReadErr error
			var expectedBlockHashes []string
			var expectedBlocks map[string]*pbeth.Block
			var receivedBlocks map[string]*pbeth.Block

			wg.Add(1)
			go func() {
				defer wg.Done()
				expectedBlockHashes, expectedBlocks, err = readBundle(ctx, filename, storeExpected)
				if err != nil {
					bundleErrLock.Lock()
					bundleReadErr = multierr.Append(bundleReadErr, err)
					bundleErrLock.Unlock()
				}
			}()

			wg.Add(1)
			go func() {
				defer wg.Done()
				_, receivedBlocks, err = readBundle(ctx, filename, storeReceived)
				if err != nil {
					bundleErrLock.Lock()
					bundleReadErr = multierr.Append(bundleReadErr, err)
					bundleErrLock.Unlock()
				}
			}()
			wg.Wait()
			if bundleReadErr != nil {
				return fmt.Errorf("reading bundles: %w", bundleReadErr)
			}

			// check if all blocks exists
			bundleHasDiff := false

			for _, expectedBlockHash := range expectedBlockHashes {
				blocksCountedInChunk++
				expectedBlock := expectedBlocks[expectedBlockHash]
				receivedBlock, existsInReceived := receivedBlocks[expectedBlockHash]

				// Reset chunk, print if good
				if blocksCountedInChunk >= chunkSize || uint64(blocksCountedInChunk) >= (blockRangeSize-uint64(rangeNum*chunkSize)-1) || uint64(blocksCountedInChunk) == blockRangeSize-1 {
					if chunkIsGood {
						fmt.Printf("✓ Bundle %d - %d has no differences\n", (rangeNum)*chunkSize, (rangeNum+1)*chunkSize)
					}
					chunkIsGood = true
					rangeNum++
					blocksCountedInChunk = -1
				}

				// false && first error in chunk
				if !existsInReceived && chunkIsGood {
					chunkIsGood = false
					fmt.Printf("✖ Bundle %d - %d is different\n", (rangeNum)*chunkSize, (rangeNum+1)*chunkSize)
					bundleHasDiff = true
				}

				if !existsInReceived {
					fmt.Printf("- Block (%s) is present in %s but missing in %s\n", expectedBlock.AsRef(), args[0], args[1])
					bundleHasDiff = true
				} else if !proto.Equal(expectedBlock, receivedBlock) {
					bundleHasDiff = true

					if chunkIsGood {
						chunkIsGood = false
						fmt.Printf("✖ Bundle %d - %d is different\n", (rangeNum)*chunkSize, (rangeNum+1)*chunkSize)
					}

					fmt.Printf("- Block (%s) is different\n", expectedBlock.AsRef())
					if displayDiff {
						expectedBlockJSON, err := rpc.MarshalJSONRPCIndent(expectedBlock, "", " ")
						if err != nil {
							return fmt.Errorf("marshaling block: %w", err)
						}
						receivedBlockJSON, err := rpc.MarshalJSONRPCIndent(receivedBlock, "", " ")
						if err != nil {
							return fmt.Errorf("marshaling block: %w", err)
						}

						diff, err := unifiedDiff(expectedBlockJSON, receivedBlockJSON)
						if err != nil {
							return fmt.Errorf("getting diff: %w", err)
						}
						fmt.Printf("difference: \n%s\n", diff)
					}
				}

				// Add to final differences to be printed
				if bundleHasDiff {
					differentBlocks[string(expectedBlock.Hash)] = block.Range{StartBlock: uint64(sftools.RoundToBundleStartBlock(uint32(expectedBlock.Number), 100)),
						ExclusiveEndBlock: uint64(sftools.RoundToBundleEndBlock(uint32(expectedBlock.Number), 100))}
				}
				bundleHasDiff = false
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("walking files: %w", err)
	}

	if !displayDiff {
		fmt.Printf("\n\nTo see for details of the differences for the different bundles, run one of those commands:\n")
		for _, blk := range differentBlocks {
			fmt.Printf("- fireeth tools compare-blocks --diff %s %s %d:%d\n\n", args[0], args[1], blk.StartBlock, blk.ExclusiveEndBlock)
		}
	}
	return nil
}
