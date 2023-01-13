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
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/cli"
	"github.com/streamingfast/dstore"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"github.com/streamingfast/substreams/block"
	"google.golang.org/protobuf/proto"
	"io"
	"math"
	"os"
	"os/exec"
	"strconv"
)

var compareBlocksCmd = &cobra.Command{
	Use:   "compare-blocks <expected_bundle> <actual_bundle> [<block_range>]",
	Short: "Checks for any differences between two block stores between a specified range. (To compare the likeness of two block ranges, for example)",
	Long: cli.Dedent(`
		compare-blocks takes in two paths to stores of merged blocks and a range specifying the blocks you want to compare (written as: start-finish).
		It will output the status of the likeness of every million blocks, on completion, or on encountering a difference. 
		Increments that contain a difference will be communicated as well as the blocks within that contain differences.
		Increments that do not have any differences will be outputted as identical.
		
		After passing through the blocks, it will output instructions on how to locate a specific difference based on the
		blocks that were given. This is done by applying the --diff flag before your args. 

		Commands inputted with --diff will display the blocks that have differences, as well as the difference. 
	`),
	RunE: compareBlocksE,
	Example: cli.Dedent(`
		# Run over full block range
		fireeth tools compare-blocks sf_bundle/ cs_bundle/ 100-200

		# Run over specific block range, displaying differences in blocks
		fireeth tools compare-blocks --diff sf_bundle/ cs_bundle/ 100-200
	`),
}

func init() {
	Cmd.AddCommand(compareBlocksCmd)
	compareBlocksCmd.PersistentFlags().Bool("diff", false, "When activated, difference is displayed for each block with a difference")
}

func getBundleFloor(num uint64) uint64 {
	return uint64(math.Round(float64(num/100.0))) * 100
}

func getBundleCeiling(num uint64) uint64 {
	return (uint64(math.Round(float64(num/100.0))) * 100) + 100
}

func unifiedDiff(cnt1, cnt2 []byte) (string, error) {
	file1 := "/tmp/block-difference-expected-bundle"
	file2 := "/tmp/block-difference-received-bundle"
	err := os.WriteFile(file1, cnt1, 0600)
	if err != nil {
		return "", fmt.Errorf("writing temp file: %w", err)
	}
	err = os.WriteFile(file2, cnt2, 0600)
	if err != nil {
		return "", fmt.Errorf("writing temp file: %w", err)
	}

	cmd := exec.Command("diff", "-u", file1, file2)
	buffer, _ := cmd.Output()

	out := string(buffer)

	return out, nil
}

func compareBlocksE(cmd *cobra.Command, args []string) error {
	if len(args) < 3 {
		fmt.Println("improper args. should look like: <store_one> <store_two> <starting_block-ending_block>")
		return nil
	}
	isDiff, err := cmd.Flags().GetBool("diff")
	if err != nil {
		return fmt.Errorf("identifying --diff flag: %w", err)
	}
	differentBlocks := make(map[string]block.Range)

	ctx := cmd.Context()
	blockRange := block.ParseRange(args[2])
	fmt.Printf("Starting comparison between blockrange %d - %d\n", blockRange.StartBlock, blockRange.ExclusiveEndBlock)

	blocksCountedInRange := 0
	rangeIsGood := true
	rangeNum := 0
	newBundle := true

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
	err = storeExpected.Walk(ctx, "00", func(filename string) (err error) {
		fileRange, err := strconv.Atoi(filename)
		if err != nil {
			return fmt.Errorf("parsing filename: %w\n", err)
		}
		fmt.Printf("walking: %s\n", fileRange)
		// If bundleExpected is valid
		if blockRange.Contains(uint64(fileRange)) {

			// Create a fileReader
			fileReader, err := storeExpected.OpenObject(ctx, filename)
			if err != nil {
				return fmt.Errorf("creating reader: %w", err)
			}
			defer fileReader.Close()

			// Create a blockReader
			blockReader, err := bstream.GetBlockReaderFactory.New(fileReader)
			if err != nil {
				return fmt.Errorf("creating block reader: %w", err)
			}

			doesExist, err := storeReceived.FileExists(ctx, filename)
			if err != nil {
				return fmt.Errorf("checking match: %w", err)
			}

			// If/else a file that exists
			if !doesExist {
				fmt.Printf("missing file: %s", filename)
				return nil
			} else {
				compBlocks := make(map[string]*pbeth.Block)

				// Create comparing fileReader
				comparingFileReader, err := storeReceived.OpenObject(ctx, filename)
				if err != nil {
					return fmt.Errorf("creating received store reader: %w", err)
				}

				// Create comparing blockReader
				comparingBlockReader, err := bstream.GetBlockReaderFactory.New(comparingFileReader)
				if err != nil {
					return fmt.Errorf("creating block reader: %w", err)
				}

				// Go over blocks in relevant file
				for {
					curBlock, err := blockReader.Read()
					if err == io.EOF {
						break
					}
					if err != nil {
						return err
					}

					// If/else contains a relevant block: break out of file
					if !blockRange.Contains(curBlock.Number) {
						break
					} else {
						hasDif := false

						// Once in valid file within range
						//get comparing blocks once
						if newBundle {
							for {
								compBlock, err := comparingBlockReader.Read()
								if err == io.EOF {
									break
								}
								if err != nil {
									return err
								}
								compBlocks[string(compBlock.ToProtocol().(*pbeth.Block).Hash)] = compBlock.ToProtocol().(*pbeth.Block)
							}
							newBundle = false
						}

						// Check if 'million' is good
						// Print message and reset 'million'
						if blocksCountedInRange >= 1000000 || uint64(blocksCountedInRange) >= blockRange.Len()-uint64(rangeNum*1000000) || uint64(blocksCountedInRange) == blockRange.Len() {
							if rangeIsGood {
								fmt.Printf("✓ bundle %d - %d has no differences\n", (rangeNum)*100000, (rangeNum+1)*100000)
							}
							rangeIsGood = true
							rangeNum++
							blocksCountedInRange = -1
						}

						// if block exists
						_, exists := compBlocks[string(curBlock.ToProtocol().(*pbeth.Block).Hash)]

						// false && first error in range
						if !exists && rangeIsGood {
							rangeIsGood = false
							fmt.Printf("bundle %d - %d is different\n", (rangeNum)*1000000, (rangeNum+1)*1000000)
							hasDif = true
						}

						// Check if diff is enabled
						if !isDiff {

							// And doesn't exist
							if !exists {
								fmt.Printf("Block #%d (%s) is different\n", curBlock.Number, curBlock.ToProtocol().(*pbeth.Block).Hash)
								hasDif = true

								// Exists but has diff
							} else if !proto.Equal(curBlock.ToProtocol().(*pbeth.Block), compBlocks[string(curBlock.ToProtocol().(*pbeth.Block).Hash)]) {
								hasDif = true

								if rangeIsGood {
									rangeIsGood = false
									fmt.Printf("bundle %d - %d is different\n", (rangeNum)*100000, (rangeNum+1)*100000)
								}
								fmt.Printf("Block (%s) is different", curBlock.AsRef())
							}

							// If enabled
						} else {
							if !exists {
								fmt.Printf("Block (%s) is present in %s but missing in %s\n", curBlock.AsRef(), args[0], args[1])
								hasDif = true
							} else if !proto.Equal(curBlock.ToProtocol().(*pbeth.Block), compBlocks[string(curBlock.ToProtocol().(*pbeth.Block).Hash)]) {
								hasDif = true

								if rangeIsGood {
									rangeIsGood = false
									fmt.Printf("bundle %d - %d is different\n", (rangeNum)*100000, (rangeNum+1)*100000)
								}
								fmt.Printf("Block (%s) is different\n", curBlock.AsRef())

								blockExpectedJSON, err := json.Marshal(curBlock)
								if err != nil {
									return fmt.Errorf("marshaling block: %w", err)
								}
								blockReceivedJSON, err := json.Marshal(compBlocks[string(curBlock.ToProtocol().(*pbeth.Block).Hash)])
								if err != nil {
									return fmt.Errorf("marshaling block: %w", err)
								}

								diff := cmp.Diff(blockExpectedJSON, blockReceivedJSON)
								if diff != "" {
									fmt.Printf("Difference: \n%s\n\n", diff)
								}
							}
						}

							diff, err := unifiedDiff(blockExpectedJSON, blockReceivedJSON)
							if err != nil {
								return fmt.Errorf("getting diff: %w", err)
							}
							fmt.Printf("difference: \n%s\n", diff)

						}

					}
					blocksCountedInRange++
				}
				newBundle = true
			}
			return nil
		}
		return nil
	})

	fmt.Printf("\n\nTo see for details of the differences for the different bundles, run one of those commands:\n")
	for _, blk := range differentBlocks {
		fmt.Printf("- fireeth tools compare-blocks --diff %s %s %d-%d\n\n", args[0], args[1], blk.StartBlock, blk.ExclusiveEndBlock)
	}
	return nil
}
