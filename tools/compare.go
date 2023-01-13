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
	compareBlocksCmd.PersistentFlags().Bool("diff", false, "A flag to check for differences over small range")
}

func compareBlocksE(cmd *cobra.Command, args []string) error {
	fmt.Printf("\n-----starting comparison-----\n")
	ctx := cmd.Context()

	storeExpectedDef := args[0]
	storeReceivedDef := args[1]
	blockRange := block.ParseRange(args[2])

	storeExpected, err := dstore.NewDBinStore(storeExpectedDef)
	if err != nil {
		return fmt.Errorf("unable to create store at path %q: %w", storeExpectedDef, err)
	}

	storeReceived, err := dstore.NewDBinStore(storeReceivedDef)
	if err != nil {
		return fmt.Errorf("unable to create store at path %q: %w", storeReceivedDef, err)
	}

	blocksExpected := make(map[uint64]*pbeth.Block)
	blocksReceived := make(map[uint64]*pbeth.Block)

	collectBlocksExpected := func(store dstore.Store, blockMap map[uint64]*pbeth.Block) error {
		fmt.Printf("collecting expected blocks\n")
		var files []string
		err = storeExpected.Walk(ctx, "", func(filename string) (err error) {
			files = append(files, filename)
			return nil
		})

		var toClose []io.ReadCloser
		defer func() {
			for i := range toClose {
				err := toClose[i].Close()
				if err != nil {
					return
				}
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
				curBlock, err := blockReader.Read()
				if err == io.EOF {
					break
				}
				if err != nil {
					return err
				}
				if blockRange.Contains(curBlock.Number) {
					blockMap[curBlock.Number] = curBlock.ToProtocol().(*pbeth.Block)
					if uint64(len(blockMap)) >= blockRange.Len() {
						break
					}
				}
			}
		}

		fmt.Printf("finished collected expected blocks\n")
		return nil
	}

	collectBlocksReceived := func(store dstore.Store, blockMap map[uint64]*pbeth.Block) error {
		fmt.Printf("collecting Received blocks\n")
		var files []string
		err = storeReceived.Walk(ctx, "", func(filename string) (err error) {
			files = append(files, filename)
			return nil
		})

		var toClose []io.ReadCloser
		defer func() {
			for i := range toClose {
				err := toClose[i].Close()
				if err != nil {
					return
				}
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
				curBlock, err := blockReader.Read()
				if err == io.EOF {
					break
				}
				if err != nil {
					return err
				}
				if blockRange.Contains(curBlock.Number) {
					blockMap[curBlock.Number] = curBlock.ToProtocol().(*pbeth.Block)
					if uint64(len(blockMap)) >= blockRange.Len() {
						break
					}
				}
			}
		}
		fmt.Printf("finished collected Received blocks\n")

		return nil
	}

	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		_ = collectBlocksExpected(storeExpected, blocksExpected)
	}()
	go func() {
		defer wg.Done()
		_ = collectBlocksReceived(storeReceived, blocksReceived)
	}()
	wg.Wait()

	rangeIsGood := true
	rangeNum := 0
	blocksCounted := 0
	differentBlocks := make(map[uint64]block.Range)

	if uint64(len(blocksExpected)) < blockRange.Len() || uint64(len(blocksReceived)) < blockRange.Len() {
		return fmt.Errorf("insufficient blocks for range")
	}

	isDiff, err := cmd.Flags().GetBool("diff")
	if err != nil {
		return fmt.Errorf("identifying --diff flag: %w", err)
	}

	if !isDiff {
		for blockNum, blockExpected := range blocksExpected {
			blocksCounted++
			blockReceived, exists := blocksReceived[blockNum]
			if !exists {
				continue
			}

			if !proto.Equal(blockExpected, blockReceived) {
				differentBlocks[blockExpected.Number] = block.Range{StartBlock: uint64(math.Round(float64(blockExpected.Number/100.0))) * 100,
					ExclusiveEndBlock: (uint64(math.Round(float64(blockExpected.Number/100.0))) * 100) + 100}

				if rangeIsGood {
					rangeIsGood = false
					fmt.Printf("bundle %d - %d is different\n", (rangeNum)*100000, (rangeNum+1)*100000)
				}
				fmt.Printf("Block #%v is different", blockExpected.Number)
			}
			if blocksCounted >= 100000 || uint64(blocksCounted) >= blockRange.Len()-uint64(rangeNum*100000) || blocksCounted == len(blocksExpected) {
				if rangeIsGood {
					fmt.Printf("✓ bundle %d - %d has no differences\n", (rangeNum)*100000, (rangeNum+1)*100000)
				}
				rangeIsGood = true
				rangeNum++
				blocksCounted = -1
			}
		}
	} else {
		if blockRange.Len() != 100 {
			return fmt.Errorf("when using --diff, make size of range equal to 100")
		}
		for i := blockRange.StartBlock; i < blockRange.ExclusiveEndBlock; i++ {
			blockReceived, exists := blocksReceived[i]
			if !exists {
				fmt.Printf("Block #%v is missing in %v\n", i, storeReceivedDef)
			}
			blockExpected, exists := blocksExpected[i]
			if !exists {
				fmt.Printf("Block #%v is missing in %v\n", i, storeExpectedDef)
			}

			if !proto.Equal(blockExpected, blockReceived) {
				differentBlocks[blockExpected.Number] = block.Range{StartBlock: uint64(math.Round(float64(blockExpected.Number/100.0))) * 100,
					ExclusiveEndBlock: (uint64(math.Round(float64(blockExpected.Number/100.0))) * 100) + 100}

				fmt.Printf("Block #%v is different\n", blockExpected.Number)

				blockExpectedJson, err := json.Marshal(blockExpected)
				if err != nil {
					return fmt.Errorf("marshaling block: %v", err)
				}
				blockReceivedJson, err := json.Marshal(blockReceived)
				if err != nil {
					return fmt.Errorf("marshaling block: %v", err)
				}

				diff := cmp.Diff(blockExpectedJson, blockReceivedJson)
				if diff != "" {
					fmt.Printf("%v\n\n", diff)
				}
			}
		}
	}

	fmt.Printf("\n\nTo see for details of the differences for the different bundles, run one of those commands:\n")
	for _, blk := range differentBlocks {
		fmt.Printf("- fireeth tools compare-blocks --diff=true %v %v %v-%v\n\n", storeExpectedDef, storeReceivedDef, blk.StartBlock, blk.ExclusiveEndBlock)
	}
	return nil
}
