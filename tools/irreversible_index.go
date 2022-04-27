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

	"github.com/spf13/cobra"
	"github.com/streamingfast/bstream"
	bstransform "github.com/streamingfast/bstream/transform"
	"github.com/streamingfast/dstore"
	"github.com/streamingfast/firehose"
	pbfirehose "github.com/streamingfast/pbgo/sf/firehose/v1"
)

var generateIrrIdxCmd = &cobra.Command{
	Use:   "generate-irreversible-index {source-blocks-url} {dest-index-url} {start-block-num} [stop-block-num]",
	Short: "Prints a block from a one-block file",
	Args:  cobra.RangeArgs(3, 4),
	RunE:  generateIrrIdxE,
}

func init() {
	Cmd.AddCommand(generateIrrIdxCmd)

	generateIrrIdxCmd.Flags().IntSlice("bundle-sizes", []int{100000, 10000, 1000, 100}, "list of sizes for irreversible block indices")
}

// TODO: add flag for index size(s)

func generateIrrIdxE(cmd *cobra.Command, args []string) error {

	sizes, err := cmd.Flags().GetIntSlice("bundle-sizes")
	if err != nil {
		return err
	}
	var bundleSizes []uint64
	for _, size := range sizes {
		if size < 0 {
			return fmt.Errorf("invalid negative size for bundle-sizes: %d", size)
		}
		bundleSizes = append(bundleSizes, uint64(size))
	}

	blocksStoreURL := args[0]
	indexStoreURL := args[1]
	startBlockNum, err := strconv.ParseInt(args[2], 10, 64)
	if err != nil {
		return fmt.Errorf("unable to parse block number %q: %w", args[2], err)
	}
	var stopBlockNum uint64
	if len(args) == 4 {
		stopBlockNum, err = strconv.ParseUint(args[3], 10, 64)
		if err != nil {
			return fmt.Errorf("unable to parse block number %q: %w", args[3], err)
		}
	}

	blocksStore, err := dstore.NewDBinStore(blocksStoreURL)
	if err != nil {
		return fmt.Errorf("failed setting up block store from url %q: %w", blocksStoreURL, err)
	}

	indexStore, err := dstore.NewStore(indexStoreURL, "", "", false)
	if err != nil {
		return fmt.Errorf("failed setting up irreversible blocks index store from url %q: %w", indexStoreURL, err)
	}

	streamFactory := firehose.NewStreamFactory(
		[]dstore.Store{blocksStore},
		indexStore,
		bundleSizes,
		nil,
		nil,
		nil,
		nil,
	)
	cmd.SilenceUsage = true

	ctx := context.Background()

	var opts []bstransform.IrreversibleIndexerOption

	if startBlockNum > 0 {
		opts = append(opts, bstransform.IrrWithDefinedStartBlock(uint64(startBlockNum)))
	}
	irreversibleIndexer := bstransform.NewIrreversibleBlocksIndexer(indexStore, bundleSizes, opts...)

	handler := bstream.HandlerFunc(func(blk *bstream.Block, obj interface{}) error {
		irreversibleIndexer.Add(blk)
		return nil
	})

	stream, err := streamFactory.New(
		ctx,
		handler,
		&pbfirehose.Request{StartBlockNum: startBlockNum, StopBlockNum: stopBlockNum, ForkSteps: []pbfirehose.ForkStep{pbfirehose.ForkStep_STEP_IRREVERSIBLE}},
		zlog,
	)
	if err != nil {
		return fmt.Errorf("getting firehose stream: %w", err)
	}

	return stream.Run(ctx)
}
