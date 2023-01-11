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
	"github.com/google/go-cmp/cmp"
	"github.com/streamingfast/cli"
	"github.com/streamingfast/substreams/block"
	"google.golang.org/protobuf/proto"
	"io"
	"math"
	"sync"

	"github.com/spf13/cobra"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/dstore"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
)

var compareBlocksCmd = &cobra.Command{
	Use:   "compare-blocks <expected_bundle> <actual_bundle> [<block_range>]",
	Short: "Checks for any differences between merge files of two different stores. (To compare the output of two different instrumentations, for example)",
	Long: cli.Dedent(`
		If --diff is not provided, will print a summary of the differences found with specific instructions to 
		run the detailed dif for each bundle. It will print a summary every 100,000 blocks checked. Output will look like:\n\n

		Bundle 0 - 100,000 is different\n
		✓ Bundle 100,000 - 200,000 has no differences\n
		Bundle 300,000 - 400,000 is different\n\n
	
		At the end of the command output, the following instructions will be printed to assist to locate a specific difference:\n\n
		
		To see for details of the differences for the different bundles, run one of those commands:\n
		fireeth tools compare-blocks --diff <expected_bundle_from_arg> <actual_bundle_from_arg> 
		<range_for_matching_only_first_offending_bundle> \n
 		fireeth tools compare-blocks --diff <expected_bundle_from_arg> <actual_bundle_from_arg>
		<range_for_matching_only_second_offending_bundle> \n\n
		
		If --diff=true, range of blocks should match 1 bundle, so a range of 100. When a difference is found in the bundle, 
		for each block with differences, a message is printed. \n\n

		Block #<Number> <Hash> is missing in <Bundle>\n\n

		Block #<Number> <Hash> is different\n
		<Difference>\n\n
	`),
	Args: cobra.ExactArgs(3),
	RunE: compareBlocksE,
	Example: cli.Dedent(`
		# Run over full block range
		fireeth tools compare-blocks sf_bundle/ cs_bundle/ 100-200

		# Run over specific block range (inclusive/inclusive)
		fireeth tools compare-blocks --diff=true sf_bundle/ cs_bundle/ 100-200
	`),
}

func init() {
	Cmd.AddCommand(compareBlocksCmd)
	compareBlocksCmd.PersistentFlags().Bool("diff", false, "A flag to check for differences over small range")
}

func compareBlocksE(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	storeADef := args[0]
	storeBDef := args[1]
	blockRange := block.ParseRange(args[2])

	storeA, err := dstore.NewDBinStore(storeADef)
	if err != nil {
		return fmt.Errorf("unable to create store at path %q: %w", storeADef, err)
	}

	storeB, err := dstore.NewDBinStore(storeBDef)
	if err != nil {
		return fmt.Errorf("unable to create store at path %q: %w", storeBDef, err)
	}

	blocksA := make(map[uint64]*pbeth.Block)
	blocksB := make(map[uint64]*pbeth.Block)

	collectBlocks := func(store dstore.Store, blockMap map[uint64]*pbeth.Block) error {
		var files []string
		err = storeA.Walk(ctx, "", func(filename string) (err error) {
			files = append(files, filename)
			return nil
		})

		var toClose []io.ReadCloser
		defer func() {
			for i := range toClose {
				toClose[i].Close()
			}
		}()

		for _, filepath := range files {
			reader, err := store.OpenObject(ctx, filepath)
			if err != nil {
				return err
			}
			toClose = append(toClose, reader)

			blockReader, err := bstream.GetBlockReaderFactory.New(reader)
			if err != nil {
				return err
			}

			for {
				block, err := blockReader.Read()
				if err == io.EOF {
					break
				}
				if err != nil {
					return err
				}
				if blockRange.Contains(block.Number) {
					blockMap[block.Number] = block.ToNative().(*pbeth.Block)
				}
			}
		}
		return nil
	}
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		_ = collectBlocks(storeA, blocksA)
	}()

	go func() {
		defer wg.Done()
		_ = collectBlocks(storeB, blocksB)
	}()
	wg.Wait()

	rangeIsGood := true
	rangeNum := 0
	blocksCounted := 0
	differentBlocks := make(map[uint64]block.Range)

	if uint64(len(blocksA)) < blockRange.Len() || uint64(len(blocksA)) < blockRange.Len() {
		return fmt.Errorf("insufficient blocks for range")
	}

	isDiff, err := cmd.Flags().GetBool("diff")
	if err != nil {
		return fmt.Errorf("identifying --diff flag %w\n", err)
	}

	if !isDiff {
		for blockNum, blockA := range blocksA {
			blockB, exists := blocksB[blockNum]
			if !exists {
				continue
			}

			if !proto.Equal(blockA, blockB) {
				differentBlocks[blockA.Number] = block.Range{StartBlock: uint64(math.Round(float64(blockA.Number/100.0))) * 100,
					ExclusiveEndBlock: (uint64(math.Round(float64(blockA.Number/100.0))) * 100) + 100}

				if rangeIsGood {
					rangeIsGood = false
					fmt.Sprintf("bundle %d - %d is different", (rangeNum)*100000, (rangeNum+1)*100000)
				}
			}
			if blocksCounted >= 100000 || uint64(blocksCounted) >= blockRange.Len()-uint64(rangeNum*100000) {
				if rangeIsGood {
					fmt.Sprintf("✓ bundle %d - %d has no differences", (rangeNum)*100000, (rangeNum+1)*100000)
				}
				rangeIsGood = true
				rangeNum++
				blocksCounted = -1
			}

			blocksCounted++
		}
	} else {
		if blockRange.Len() != 100 {
			return fmt.Errorf("when using --diff, make size of range equal to 100")
		}
		for i := blockRange.StartBlock; i <= blockRange.ExclusiveEndBlock; i++ {
			blockB, exists := blocksB[i]
			if !exists {
				fmt.Sprintf("Block #%v is missing in %v", i, storeBDef)
			}
			blockA, exists := blocksA[i]
			if !exists {
				fmt.Sprintf("Block #%v is missing in %v", i, storeADef)
			}

			if !proto.Equal(blockA, blockB) {
				differentBlocks[blockA.Number] = block.Range{StartBlock: uint64(math.Round(float64(blockA.Number/100.0))) * 100,
					ExclusiveEndBlock: (uint64(math.Round(float64(blockA.Number/100.0))) * 100) + 100}

				fmt.Sprintf("Block #%v %v is different", blockA.Number, blockA.Hash)

				blockAJson, err := json.Marshal(blockA)
				if err != nil {
					return fmt.Errorf("marshaling block %w\n", blockAJson)
				}
				blockBJson, err := json.Marshal(blockB)
				if err != nil {
					return fmt.Errorf("marshaling block %w\n", blockAJson)
				}

				diff := cmp.Diff(blockAJson, blockBJson)
				if diff != "" {
					fmt.Sprintf("%v\n\n", diff)
				}
			}
		}
	}

	fmt.Sprintf(cli.Dedent(`
		\nTo see for details of the differences for the different bundles, run one of those commands:\n
	`))
	for _, blk := range differentBlocks {
		fmt.Sprintf("fireeth tools compare-blocks --diff %v %v %v-%v\n\n", storeADef, storeBDef, blk.StartBlock, blk.ExclusiveEndBlock)
	}
	return nil
}
