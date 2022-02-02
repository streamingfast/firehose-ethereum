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
	"strconv"

	"github.com/golang/protobuf/proto"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/streamingfast/dstore"
	firehose "github.com/streamingfast/firehose"
	pbfirehose "github.com/streamingfast/pbgo/sf/firehose/v1"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/codec/v1"
	"github.com/streamingfast/sf-ethereum/transform"
)

var generateAccIdxCmd = &cobra.Command{
	Use:   "generate-account-index {acct-index-url} {irr-index-url} {source-blocks-url} {start-block-num} {stop-block-num}",
	Short: "Generate index files for eth accounts present in blocks",
	Args:  cobra.RangeArgs(4, 5),
	RunE:  generateAccIdxE,
}

func init() {
	Cmd.AddCommand(generateAccIdxCmd)

	generateAccIdxCmd.Flags().IntSlice("bundle-sizes", []int{100000, 10000, 1000, 100}, "list of sizes for irreversible block indices")
}

func generateAccIdxE(cmd *cobra.Command, args []string) error {

	var bundleSizes []uint64
	for _, size := range viper.GetIntSlice("bundle-sizes") {
		if size < 0 {
			return fmt.Errorf("invalid negative size for bundle-sizes: %d", size)
		}
		bundleSizes = append(bundleSizes, uint64(size))
	}

	accountIndexStoreURL := args[0]
	irrIndexStoreURL := args[1]
	blocksStoreURL := args[2]
	startBlockNum, err := strconv.ParseInt(args[3], 10, 64)
	if err != nil {
		return fmt.Errorf("unable to parse block number %q: %w", args[0], err)
	}
	var stopBlockNum uint64
	if len(args) == 5 {
		stopBlockNum, err = strconv.ParseUint(args[4], 10, 64)
		if err != nil {
			return fmt.Errorf("unable to parse block number %q: %w", args[0], err)
		}
	}

	blocksStore, err := dstore.NewDBinStore(blocksStoreURL)
	if err != nil {
		return fmt.Errorf("failed setting up block store from url %q: %w", blocksStoreURL, err)
	}

	irrIndexStore, err := dstore.NewStore(irrIndexStoreURL, "", "", false)
	if err != nil {
		return fmt.Errorf("failed setting up irreversible blocks index store from url %q: %w", irrIndexStoreURL, err)
	}

	accountIndexStore, err := dstore.NewStore(accountIndexStoreURL, "", "", false)
	if err != nil {
		return fmt.Errorf("failed setting up bccount index store from url %q: %w", accountIndexStoreURL, err)
	}

	firehoseServer := firehose.NewServer(
		zlog,
		[]dstore.Store{blocksStore},
		irrIndexStore,
		false,
		bundleSizes,
		nil,
		nil,
		nil,
		nil,
	)

	ctx := context.Background()
	cli := firehoseServer.BlocksFromLocal(ctx, &pbfirehose.Request{
		StartBlockNum: startBlockNum,
		StopBlockNum:  stopBlockNum,
		ForkSteps:     []pbfirehose.ForkStep{pbfirehose.ForkStep_STEP_IRREVERSIBLE},
	})

	cmd.SilenceUsage = true

	t := transform.NewLogAddressIndexer(accountIndexStore)

	for {
		resp, err := cli.Recv()
		if err != nil {
			return fmt.Errorf("receiving firehose message: %w", err)
		}
		if resp == nil {
			return nil
		}
		b := &pbcodec.Block{}
		err = proto.Unmarshal(resp.Block.Value, b)
		if err != nil {
			return fmt.Errorf("unmarshalling firehose message: %w", err)
		}
		t.ProcessEthBlock(b)

		fmt.Println(t)
	}
}
