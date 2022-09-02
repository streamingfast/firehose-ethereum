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
	"io"
	"strconv"
	"sync"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/cobra"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/dstore"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
)

var compareBlocksCmd = &cobra.Command{
	Use:   "compareblocks <blocks_store_a> <blocks_store_b> <prefix>",
	Short: "Checks for any differences between merge files of two different stores. (To compare the output of two different instrumentations, for example)",
	Args:  cobra.ExactArgs(3),
	RunE:  compareBlocksE,
}

func init() {
	Cmd.AddCommand(compareBlocksCmd)
}

func compareBlocksE(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	storeADef := args[0]
	storeBDef := args[1]

	blockNum, err := strconv.ParseUint(args[2], 10, 64)
	if err != nil {
		return fmt.Errorf("unable to parse block number %q: %w", args[0], err)
	}

	storeA, err := dstore.NewDBinStore(storeADef)
	if err != nil {
		return fmt.Errorf("unable to create store at path %q: %w", storeADef, err)
	}

	storeB, err := dstore.NewDBinStore(storeBDef)
	if err != nil {
		return fmt.Errorf("unable to create store at path %q: %w", storeBDef, err)
	}

	blocksA := make(map[string]*pbeth.Block)
	blocksB := make(map[string]*pbeth.Block)

	filePrefix := fmt.Sprintf("%010d", blockNum)
	collectBlocks := func(store dstore.Store, blockMap map[string]*pbeth.Block) error {
		var files []string
		err = storeA.Walk(ctx, filePrefix, func(filename string) (err error) {
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
				blockMap[block.ID()] = block.ToNative().(*pbeth.Block)
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

	for blockID, blockA := range blocksA {
		blockB, exists := blocksB[blockID]
		if !exists {
			continue
		}

		blockAJSON, _ := json.Marshal(blockA)
		blockBJSON, _ := json.Marshal(blockB)

		diff := cmp.Diff(blockAJSON, blockBJSON)
		if diff != "" {
			fmt.Printf("❌ difference found on block id %s\n", blockID)
			fmt.Println(diff)
			return nil
		}
	}

	fmt.Println("✓ no differences found!")
	return nil
}
