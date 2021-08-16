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
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/dstore"
	"github.com/streamingfast/jsonpb"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/dfuse/ethereum/codec/v1"
	"go.uber.org/zap"
)

// FIXME: This is exactly the same as `sf-eosio/tools/check`, in fact, it was direct copy without modification
//        expect the import paths above. We need a way to share even more code!

var errStopWalk = errors.New("stop walk")

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
}

func init() {
	Cmd.AddCommand(CheckCmd)
	CheckCmd.AddCommand(checkMergedBlocksCmd)

	CheckCmd.PersistentFlags().StringP("range", "r", "", "Block range to use for the check")

	checkMergedBlocksCmd.Flags().BoolP("print-stats", "s", false, "Natively decode each block in the segment and print statistics about it, ensuring it contains the required blocks")
	checkMergedBlocksCmd.Flags().BoolP("print-full", "f", false, "Natively decode each block and print the full JSON representation of the block, should be used with a small range only if you don't want to be overwhelmed")
}

type blockNum uint64

func (b blockNum) String() string {
	return "#" + strings.ReplaceAll(humanize.Comma(int64(b)), ",", " ")
}

func checkMergedBlocksE(cmd *cobra.Command, args []string) error {
	storeURL := args[0]
	fileBlockSize := uint32(100)

	fmt.Printf("Checking block holes on %s\n", storeURL)

	number := regexp.MustCompile(`(\d{10})`)

	var expected uint32
	var count int
	var baseNum32 uint32
	holeFound := false
	printIndividualSegmentStats := viper.GetBool("print-stats")
	printFullBlock := viper.GetBool("print-full")

	blockRange, err := getBlockRangeFromFlag()
	if err != nil {
		return err
	}

	expected = roundToBundleStartBlock(uint32(blockRange.Start), fileBlockSize)
	currentStartBlk := uint32(blockRange.Start)
	seenFilters := map[string]FilteringFilters{}

	blocksStore, err := dstore.NewDBinStore(storeURL)
	if err != nil {
		return err
	}

	ctx := context.Background()
	walkPrefix := walkBlockPrefix(blockRange, fileBlockSize)

	zlog.Debug("walking merged blocks", zap.Stringer("block_range", blockRange), zap.String("walk_prefix", walkPrefix))
	err = blocksStore.Walk(ctx, walkPrefix, ".tmp", func(filename string) error {
		match := number.FindStringSubmatch(filename)
		if match == nil {
			return nil
		}

		zlog.Debug("received merged blocks", zap.String("filename", filename))

		count++
		baseNum, _ := strconv.ParseUint(match[1], 10, 32)
		if baseNum+uint64(fileBlockSize)-1 < blockRange.Start {
			zlog.Debug("base num lower then block range start, quitting", zap.Uint64("base_num", baseNum), zap.Uint64("starting_at", blockRange.Start))
			return nil
		}

		baseNum32 = uint32(baseNum)

		if printIndividualSegmentStats || printFullBlock {
			newSeenFilters := validateBlockSegment(blocksStore, filename, fileBlockSize, blockRange, printIndividualSegmentStats, printFullBlock)
			for key, filters := range newSeenFilters {
				seenFilters[key] = filters
			}
		}

		if baseNum32 != expected {
			// There is no previous valid block range if we are at the ever first seen file
			if count > 1 {
				fmt.Printf("✅ Range %s\n", BlockRange{uint64(currentStartBlk), uint64(roundToBundleEndBlock(expected-fileBlockSize, fileBlockSize))})
			}

			// Otherwise, we do not follow last seen element (previous is `100 - 199` but we are `299 - 300`)
			missingRange := BlockRange{uint64(expected), uint64(roundToBundleEndBlock(baseNum32-fileBlockSize, fileBlockSize))}
			fmt.Printf("❌ Range %s! (Missing, [%s])\n", missingRange, missingRange.ReprocRange())
			currentStartBlk = baseNum32

			holeFound = true
		}
		expected = baseNum32 + fileBlockSize

		if count%10000 == 0 {
			fmt.Printf("✅ Range %s\n", BlockRange{uint64(currentStartBlk), uint64(roundToBundleEndBlock(baseNum32, fileBlockSize))})
			currentStartBlk = baseNum32 + fileBlockSize
		}

		if !blockRange.Unbounded() && roundToBundleEndBlock(baseNum32, fileBlockSize) >= uint32(blockRange.Stop-1) {
			return errStopWalk
		}

		return nil
	})
	if err != nil && err != errStopWalk {
		return err
	}

	actualEndBlock := roundToBundleEndBlock(baseNum32, fileBlockSize)
	if !blockRange.Unbounded() {
		actualEndBlock = uint32(blockRange.Stop)
	}

	fmt.Printf("✅ Range %s\n", BlockRange{uint64(currentStartBlk), uint64(actualEndBlock)})

	if len(seenFilters) > 0 {
		fmt.Println()
		fmt.Println("Seen filters")
		for _, filters := range seenFilters {
			fmt.Printf("- [Include %q, Exclude %q, System %q]\n", filters.Include, filters.Exclude, filters.System)
		}
		fmt.Println()
	}

	if holeFound {
		fmt.Printf("🆘 Holes found!\n")
	} else {
		fmt.Printf("🆗 No hole found\n")
	}

	return nil
}

func walkBlockPrefix(blockRange BlockRange, fileBlockSize uint32) string {
	if blockRange.Unbounded() {
		return ""
	}

	startString := fmt.Sprintf("%010d", roundToBundleStartBlock(uint32(blockRange.Start), fileBlockSize))
	endString := fmt.Sprintf("%010d", roundToBundleEndBlock(uint32(blockRange.Stop-1), fileBlockSize)+1)

	offset := 0
	for i := 0; i < len(startString); i++ {
		if startString[i] != endString[i] {
			return string(startString[0:i])
		}

		offset++
	}

	// At this point, the two strings are equal, to return the string
	return startString
}

func roundToBundleStartBlock(block, fileBlockSize uint32) uint32 {
	// From a non-rounded block `1085` and size of `100`, we remove from it the value of
	// `modulo % fileblock` (`85`) making it flush (`1000`).
	return block - (block % fileBlockSize)
}

func roundToBundleEndBlock(block, fileBlockSize uint32) uint32 {
	// From a non-rounded block `1085` and size of `100`, we remove from it the value of
	// `modulo % fileblock` (`85`) making it flush (`1000`) than adding to it the last
	// merged block num value for this size which simply `size - 1` (`99`) giving us
	// a resolved formulae of `1085 - (1085 % 100) + (100 - 1) = 1085 - (85) + (99)`.
	return block - (block % fileBlockSize) + (fileBlockSize - 1)
}

func validateBlockSegment(
	store dstore.Store,
	segment string,
	fileBlockSize uint32,
	blockRange BlockRange,
	printIndividualSegmentStats bool,
	printFullBlock bool,
) (seenFilters map[string]FilteringFilters) {
	reader, err := store.OpenObject(context.Background(), segment)
	if err != nil {
		fmt.Printf("❌ Unable to read blocks segment %s: %s\n", segment, err)
		return
	}
	defer reader.Close()

	readerFactory, err := bstream.GetBlockReaderFactory.New(reader)
	if err != nil {
		fmt.Printf("❌ Unable to read blocks segment %s: %s\n", segment, err)
		return
	}

	// FIXME: Need to track block continuity (100, 101, 102a, 102b, 103, ...) and report which one are missing
	seenBlockCount := 0
	for {
		block, err := readerFactory.Read()
		if block != nil {
			if !blockRange.Unbounded() {
				if block.Number >= blockRange.Stop {
					return
				}

				if block.Number < blockRange.Start {
					continue
				}
			}

			seenBlockCount++

			if printIndividualSegmentStats {
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

			if printFullBlock {
				ethBlock := block.ToNative().(*pbcodec.Block)
				fmt.Printf(jsonpb.MarshalIndentToString(ethBlock, "  "))
			}

			continue
		}

		if block == nil && err == io.EOF {
			if seenBlockCount < expectedBlockCount(segment, fileBlockSize) {
				fmt.Printf("❌ Segment %s contained only %d blocks, expected at least 100\n", segment, seenBlockCount)
			}

			return
		}

		if err != nil {
			fmt.Printf("❌ Unable to read all blocks from segment %s after reading %d blocks: %s\n", segment, seenBlockCount, err)
			return
		}
	}
}

func expectedBlockCount(segment string, fileBlockSize uint32) int {
	if segment == "0000000000" {
		return int(fileBlockSize) - int(bstream.GetProtocolFirstStreamableBlock)
	}

	return int(fileBlockSize)
}
