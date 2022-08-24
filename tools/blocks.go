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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"

	"github.com/klauspost/compress/zstd"
	"github.com/spf13/cobra"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/cli"
	"github.com/streamingfast/dstore"
	"github.com/streamingfast/eth-go"
	"github.com/streamingfast/jsonpb"
	pbeth "github.com/streamingfast/sf-ethereum/types/pb/sf/ethereum/type/v2"
	"go.uber.org/zap"
)

var printCmd = &cobra.Command{
	Use:   "print",
	Short: "Prints of one block or merged blocks file",
}

var oneBlockCmd = &cobra.Command{
	Use:   "one-block <block_num|block_file>",
	Short: "Prints a block from a one-block file",
	Args:  cobra.ExactArgs(1),
	RunE:  printOneBlockE,
}

var blocksCmd = &cobra.Command{
	Use:   "blocks <block_num>",
	Short: "Prints the content summary of a merged block file",
	Args:  cobra.ExactArgs(1),
	RunE:  printBlocksE,
}

var blockCmd = &cobra.Command{
	Use:   "block <block_num>",
	Short: "Finds and prints one block from a merged block file",
	Args:  cobra.ExactArgs(1),
	RunE:  printBlockE,
}

func init() {
	Cmd.AddCommand(printCmd)

	printCmd.AddCommand(oneBlockCmd)

	printCmd.AddCommand(blocksCmd)
	blocksCmd.PersistentFlags().Bool("transactions", false, "Include transaction IDs in output")

	printCmd.AddCommand(blockCmd)
	blockCmd.Flags().String("transaction", "", "Filters transaction by this hash")

	printCmd.PersistentFlags().Uint64("transactions-for-block", 0, "Include transaction IDs in output")
	printCmd.PersistentFlags().Bool("transactions", false, "Include transaction IDs in output")
	printCmd.PersistentFlags().Bool("calls", false, "Include transaction's Call data in output")
	printCmd.PersistentFlags().Bool("full", false, "print the fullblock instead of just header/ID")
	printCmd.PersistentFlags().String("store", "", "block store")
}

var integerRegex = regexp.MustCompile("^[0-9]+$")

func printBlocksE(cmd *cobra.Command, args []string) error {
	var identifier string
	var reader io.Reader
	var closer func() error

	if integerRegex.MatchString(args[0]) {
		blockNum, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("unable to parse block number %q: %w", args[0], err)
		}

		str := mustGetString(cmd, "store")

		store, err := dstore.NewDBinStore(str)
		if err != nil {
			return fmt.Errorf("unable to create store at path %q: %w", store, err)
		}

		identifier = fmt.Sprintf("%010d", blockNum)
		gsReader, err := store.OpenObject(context.Background(), identifier)
		if err != nil {
			fmt.Printf("❌ Unable to read blocks filename %q: %s\n", identifier, err)
			return err
		}

		reader = gsReader
		closer = gsReader.Close
	} else {
		identifier = args[0]

		// Check if it's a file and if it exists
		if !cli.FileExists(identifier) {
			fmt.Printf("❌ File %q does not exist\n", identifier)
			return os.ErrNotExist
		}

		file, err := os.Open(identifier)
		if err != nil {
			fmt.Printf("❌ Unable to read blocks filename %q: %s\n", identifier, err)
			return err
		}
		reader = file
		closer = file.Close
	}

	defer closer()

	return printBlocks(identifier, reader, mustGetBool(cmd, "transactions"))
}

func printBlocks(inputIdentifier string, reader io.Reader, printTransactions bool) error {
	readerFactory, err := bstream.GetBlockReaderFactory.New(reader)
	if err != nil {
		fmt.Printf("❌ Unable to read blocks %s: %s\n", inputIdentifier, err)
		return err
	}

	seenBlockCount := 0
	for {
		block, err := readerFactory.Read()
		if err != nil {
			if err == io.EOF {
				fmt.Printf("Total blocks: %d\n", seenBlockCount)
				return nil
			}
			return fmt.Errorf("error receiving blocks: %w", err)
		}

		seenBlockCount++

		//payloadSize, err := len(block.Payload.Get()) //disabled after rework
		ethBlock := block.ToNative().(*pbeth.Block)

		fmt.Printf("Block #%d (%s) (prev: %s): %d transactions, %d balance changes\n",
			block.Num(),
			block.ID()[0:7],
			block.PreviousID()[0:7],
			//			payloadSize,
			len(ethBlock.TransactionTraces),
			len(ethBlock.BalanceChanges),
		)
		if printTransactions {
			fmt.Println("- Transactions: ")
			for _, t := range ethBlock.TransactionTraces {
				fmt.Println("  * ", t.Hash)
			}
			fmt.Println()
		}
	}
}

func printBlockE(cmd *cobra.Command, args []string) error {
	printTransactions := mustGetBool(cmd, "transactions")
	printCall := mustGetBool(cmd, "calls")
	printFull := mustGetBool(cmd, "full")
	transactionFilter := mustGetString(cmd, "transaction")

	zlog.Info("printing block",
		zap.Bool("print_transactions", printTransactions),
		zap.String("transaction_filter", transactionFilter),
	)

	blockNum, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("unable to parse block number %q: %w", args[0], err)
	}

	str := mustGetString(cmd, "store")

	store, err := dstore.NewDBinStore(str)
	if err != nil {
		return fmt.Errorf("unable to create store at path %q: %w", store, err)
	}

	mergedBlockNum := blockNum - (blockNum % 100)
	zlog.Info("finding merged block file",
		zap.Uint64("merged_block_num", mergedBlockNum),
		zap.Uint64("block_num", blockNum),
	)

	filename := fmt.Sprintf("%010d", mergedBlockNum)
	reader, err := store.OpenObject(context.Background(), filename)
	if err != nil {
		fmt.Printf("❌ Unable to read blocks filename %s: %s\n", filename, err)
		return err
	}
	defer reader.Close()

	readerFactory, err := bstream.GetBlockReaderFactory.New(reader)
	if err != nil {
		fmt.Printf("❌ Unable to read blocks filename %s: %s\n", filename, err)
		return err
	}

	for {
		block, err := readerFactory.Read()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("error reading blocks: %w", err)
		}

		if block.Number != blockNum {
			zlog.Debug("skipping block",
				zap.Uint64("desired_block_num", blockNum),
				zap.Uint64("block_num", block.Number),
			)
			continue
		}
		ethBlock := block.ToNative().(*pbeth.Block)

		if printFull {
			jsonPayload, _ := jsonpb.MarshalIndentToString(ethBlock, "  ")
			fmt.Println(jsonPayload)
			continue
		}

		fmt.Printf("Block #%d (%s) (prev: %s): %d transactions, %d balance changes\n",
			block.Num(),
			block.ID()[0:7],
			block.PreviousID()[0:7],
			len(ethBlock.TransactionTraces),
			len(ethBlock.BalanceChanges),
		)
		if printTransactions {
			fmt.Println("- Transactions: ")
			for _, t := range ethBlock.TransactionTraces {
				hash := eth.Hash(t.Hash)
				if transactionFilter != "" {
					if transactionFilter != hash.String() && transactionFilter != hash.Pretty() {
						continue
					}
				}
				printTrx(t, printCall)
			}

		}

		continue
	}
}

func printOneBlockE(cmd *cobra.Command, args []string) error {
	if integerRegex.MatchString(args[0]) {
		blockNum, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("unable to parse block number %q: %w", args[0], err)
		}

		return printOneBlockFromStore(cmd.Context(), blockNum, mustGetString(cmd, "store"), mustGetBool(cmd, "transactions"))
	}

	path := args[0]

	// Check if it's a file and if it exists
	if !cli.FileExists(path) {
		fmt.Printf("❌ File %q does not exist\n", path)
		return os.ErrNotExist
	}

	file, err := os.Open(path)
	if err != nil {
		fmt.Printf("❌ Unable to open file %q: %s\n", path, err)
		return err
	}

	uncompressedReader, err := zstd.NewReader(file)
	if err != nil {
		return fmt.Errorf("new zstd reader: %w", err)
	}
	defer uncompressedReader.Close()

	if err := printBlockFromReader(path, uncompressedReader); err != nil {
		if errors.Is(err, io.EOF) {
			fmt.Printf("❌ One block file is empty %q: %s\n", path, err)
			return err
		}

		fmt.Printf("❌ Unable to print one-block file %s: %s\n", path, err)
		return err
	}

	return nil
}

func printOneBlockFromStore(ctx context.Context, blockNum uint64, storeDSN string, printTransactions bool) error {
	store, err := dstore.NewDBinStore(storeDSN)
	if err != nil {
		return fmt.Errorf("unable to create store at path %q: %w", store, err)
	}

	var files []string
	filePrefix := fmt.Sprintf("%010d", blockNum)
	err = store.Walk(ctx, filePrefix, func(filename string) (err error) {
		files = append(files, filename)
		return nil
	})
	if err != nil {
		return fmt.Errorf("unable to find on block files: %w", err)
	}

	for _, filepath := range files {
		reader, err := store.OpenObject(ctx, filepath)
		if err != nil {
			fmt.Printf("❌ Unable to open one-block file %s: %s\n", filepath, err)
			return err
		}
		defer reader.Close()

		if err := printBlockFromReader(filepath, reader); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return err
		}
	}
	return nil
}

func printBlockFromReader(identifier string, reader io.Reader) error {
	readerFactory, err := bstream.GetBlockReaderFactory.New(reader)
	if err != nil {
		return fmt.Errorf("new block reader: %w", err)
	}

	block, err := readerFactory.Read()
	if err != nil {
		return fmt.Errorf("reading block: %w", err)
	}

	return printBlock(block)
}

func printBlock(block *bstream.Block) error {
	nativeBlock := block.ToNative().(*pbeth.Block)

	data, err := json.MarshalIndent(nativeBlock, "", "  ")
	if err != nil {
		return fmt.Errorf("json marshall: %w", err)
	}

	fmt.Println(string(data))

	return nil
}

func printTrx(trx *pbeth.TransactionTrace, withCall bool) {
	hash := eth.Hash(trx.Hash)

	fmt.Printf("  * %s\n", hash.Pretty())
	if withCall {
		for _, call := range trx.Calls {

			str := ""
			//if len(call.Input) > 8 {
			//	str = hex.EncodeToString(call.Input[0:8])
			//} else {
			//	str = hex.EncodeToString(call.Input)
			//}

			fmt.Printf("    -> Call: %d , input: %s (parent: %d, depth: %d, Statusfailed: %v, StatusReverted: %v, FailureReason: %s, StateReverted: %v)\n",
				call.Index,
				str,
				call.ParentIndex,
				call.Depth,
				call.StatusFailed,
				call.StatusReverted,
				call.FailureReason,
				call.StateReverted,
			)
		}
	}

}
