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
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/streamingfast/bstream"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/dfuse/ethereum/codec/v1"
	sftools "github.com/streamingfast/sf-tools"
)

// CmdCheck is used in sf-ethereum-priv where additional checks are added.
var CheckCmd = &cobra.Command{Use: "check", Short: "Various checks for deployment, data integrity & debugging"}

var checkMergedBlocksCmd = &cobra.Command{
	// TODO: Not sure, it's now a required thing, but we could probably use the same logic as `start`
	//       and avoid altogether passing the args. If this would also load the config and everything else,
	//       that would be much more seamless!
	Use:   "merged-blocks {store-url}",
	Short: "Checks for any holes in merged blocks as well as ensuring merged blocks integrity",
	Args:  cobra.ExactArgs(1),
	RunE:  checkMergedBlocksE,
	Example: ExamplePrefixed("sfeth tools check merged-blocks", `
		"./sf-data/storage/merged-blocks"
		"gs://<project>/<bucket>/<path> -s"
		"s3://<project>/<bucket>/<path> -f"
		"az://<project>/<bucket>/<path> -r \"10 000 - 1 000 000"
	`),
}

func init() {
	Cmd.AddCommand(CheckCmd)
	CheckCmd.AddCommand(checkMergedBlocksCmd)

	CheckCmd.PersistentFlags().StringP("range", "r", "", "Block range to use for the check")

	checkMergedBlocksCmd.Flags().BoolP("print-stats", "s", false, "Natively decode each block in the segment and print statistics about it, ensuring it contains the required blocks")
	checkMergedBlocksCmd.Flags().BoolP("print-full", "f", false, "Natively decode each block and print the full JSON representation of the block, should be used with a small range only if you don't want to be overwhelmed")
}

func checkMergedBlocksE(cmd *cobra.Command, args []string) error {
	storeURL := args[0]
	fileBlockSize := uint32(100)

	blockRange, err := sftools.Flags.GetBlockRange("range")
	if err != nil {
		return err
	}

	printDetails := sftools.PrintNothing
	if viper.GetBool("print-stats") {
		printDetails = sftools.PrintStats
	}

	if viper.GetBool("print-full") {
		printDetails = sftools.PrintFull
	}

	return sftools.CheckMergedBlocks(cmd.Context(), zlog, storeURL, fileBlockSize, blockRange, blockPrinter, printDetails)

	// 	number := regexp.MustCompile(`(\d{10})`)

	// 	var expected uint32
	// 	var count int
	// 	var baseNum32 uint32
	// 	holeFound := false

	// 	blockRange, err := getBlockRangeFromFlag()
	// 	if err != nil {
	// 		return err
	// 	}

	// 	expected = roundToBundleStartBlock(uint32(blockRange.Start), fileBlockSize)
	// 	currentStartBlk := uint32(blockRange.Start)
	// 	seenFilters := map[string]FilteringFilters{}

	// 	blocksStore, err := dstore.NewDBinStore(storeURL)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	ctx := context.Background()
	// 	walkPrefix := walkBlockPrefix(blockRange, fileBlockSize)

	// 	zlog.Debug("walking merged blocks", zap.Stringer("block_range", blockRange), zap.String("walk_prefix", walkPrefix))
	// 	err = blocksStore.Walk(ctx, walkPrefix, ".tmp", func(filename string) error {
	// 		match := number.FindStringSubmatch(filename)
	// 		if match == nil {
	// 			return nil
	// 		}

	// 		zlog.Debug("received merged blocks", zap.String("filename", filename))

	// 		count++
	// 		baseNum, _ := strconv.ParseUint(match[1], 10, 32)
	// 		if baseNum+uint64(fileBlockSize)-1 < blockRange.Start {
	// 			zlog.Debug("base num lower then block range start, quitting", zap.Uint64("base_num", baseNum), zap.Uint64("starting_at", blockRange.Start))
	// 			return nil
	// 		}

	// 		baseNum32 = uint32(baseNum)

	// 		if printIndividualSegmentStats || printFullBlock {
	// 			newSeenFilters := validateBlockSegment(blocksStore, filename, fileBlockSize, blockRange, printIndividualSegmentStats, printFullBlock)
	// 			for key, filters := range newSeenFilters {
	// 				seenFilters[key] = filters
	// 			}
	// 		}

	// 		if baseNum32 != expected {
	// 			// There is no previous valid block range if we are at the ever first seen file
	// 			if count > 1 {
	// 				fmt.Printf("✅ Range %s\n", BlockRange{uint64(currentStartBlk), uint64(roundToBundleEndBlock(expected-fileBlockSize, fileBlockSize))})
	// 			}

	// 			// Otherwise, we do not follow last seen element (previous is `100 - 199` but we are `299 - 300`)
	// 			missingRange := BlockRange{uint64(expected), uint64(roundToBundleEndBlock(baseNum32-fileBlockSize, fileBlockSize))}
	// 			fmt.Printf("❌ Range %s! (Missing, [%s])\n", missingRange, missingRange.ReprocRange())
	// 			currentStartBlk = baseNum32

	// 			holeFound = true
	// 		}
	// 		expected = baseNum32 + fileBlockSize

	// 		if count%10000 == 0 {
	// 			fmt.Printf("✅ Range %s\n", BlockRange{uint64(currentStartBlk), uint64(roundToBundleEndBlock(baseNum32, fileBlockSize))})
	// 			currentStartBlk = baseNum32 + fileBlockSize
	// 		}

	// 		if !blockRange.Unbounded() && roundToBundleEndBlock(baseNum32, fileBlockSize) >= uint32(blockRange.Stop-1) {
	// 			return errStopWalk
	// 		}

	// 		return nil
	// 	})
	// 	if err != nil && err != errStopWalk {
	// 		return err
	// 	}

	// 	actualEndBlock := roundToBundleEndBlock(baseNum32, fileBlockSize)
	// 	if !blockRange.Unbounded() {
	// 		actualEndBlock = uint32(blockRange.Stop)
	// 	}

	// 	fmt.Printf("✅ Range %s\n", BlockRange{uint64(currentStartBlk), uint64(actualEndBlock)})

	// 	if len(seenFilters) > 0 {
	// 		fmt.Println()
	// 		fmt.Println("Seen filters")
	// 		for _, filters := range seenFilters {
	// 			fmt.Printf("- [Include %q, Exclude %q, System %q]\n", filters.Include, filters.Exclude, filters.System)
	// 		}
	// 		fmt.Println()
	// 	}

	// 	if holeFound {
	// 		fmt.Printf("🆘 Holes found!\n")
	// 	} else {
	// 		fmt.Printf("🆗 No hole found\n")
	// 	}

	// return nil
}

func blockPrinter(block *bstream.Block) {
	payloadSize := len(block.PayloadBuffer)
	ethBlock := block.ToNative().(*pbcodec.Block)

	callCount := 0
	for _, trxTrace := range ethBlock.TransactionTraces {
		callCount += len(trxTrace.Calls)
	}

	fmt.Printf("Block %s (%d bytes): %d transactions, %d calls\n",
		block,
		payloadSize,
		len(ethBlock.TransactionTraces),
		callCount,
	)
}
