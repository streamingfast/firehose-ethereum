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

package main

import (
	"fmt"
	"os"

	"github.com/DataDog/zstd"
	"github.com/spf13/cobra"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/cli"
	"github.com/streamingfast/eth-go/rpc"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
)

var compareOneblockRPCCmd = &cobra.Command{
	Use:   "compare-oneblock-rpc <oneblock-path> <rpc-endpoint>",
	Short: "Checks for any differences between a firehose one-block and the same block from RPC endpoint (get_block).",
	Long: cli.Dedent(`
		The 'compare-oneblock-rpc' takes in a local path, an RPC endpoint URL and compares a single block at a time.
	`),
	Args: cobra.ExactArgs(2),
	RunE: compareOneblockRPCE,
	Example: ExamplePrefixed("fireeth tools compare-oneblock-rpc", `
		/path/to/oneblocks/0046904064-0061a308bf12bc2e-5b6ef5eed4e06d5b-46903864-default.dbin.zst http://localhost:8545
	`),
}

func compareOneblockRPCE(cmd *cobra.Command, args []string) error {

	ctx := cmd.Context()
	filepath := args[0]
	rpcEndpoint := args[1]

	fhBlock, err := getOneBlock(filepath)
	if err != nil {
		return err
	}

	cli := rpc.NewClient(rpcEndpoint)

	rpcBlock, err := cli.GetBlockByNumber(ctx, rpc.BlockNumber(fhBlock.Number), rpc.WithGetBlockFullTransaction())
	if err != nil {
		return err
	}

	logs, err := cli.Logs(ctx, rpc.LogsParams{
		FromBlock: rpc.BlockNumber(fhBlock.Number),
		ToBlock:   rpc.BlockNumber(fhBlock.Number),
	})
	if err != nil {
		return err
	}

	identical, diffs := CompareFirehoseToRPC(fhBlock, rpcBlock, logs)
	if !identical {
		fmt.Println("different", diffs)
	} else {
		fmt.Println(fhBlock.Number, "identical")
	}
	return nil
}

func getOneBlock(path string) (*pbeth.Block, error) {
	// Check if it's a file and if it exists
	if !cli.FileExists(path) {
		return nil, os.ErrNotExist
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	uncompressedReader := zstd.NewReader(file)
	defer uncompressedReader.Close()

	readerFactory, err := bstream.GetBlockReaderFactory.New(uncompressedReader)
	if err != nil {
		return nil, fmt.Errorf("new block reader: %w", err)
	}

	block, err := readerFactory.Read()
	if err != nil {
		return nil, fmt.Errorf("reading block: %w", err)
	}

	return block.ToProtocol().(*pbeth.Block), nil
}
