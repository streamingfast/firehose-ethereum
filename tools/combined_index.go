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
	"github.com/streamingfast/sf-ethereum/transform"
	pbeth "github.com/streamingfast/sf-ethereum/types/pb/sf/ethereum/type/v1"
)

var generateCombinedIdxCmd = &cobra.Command{
	Use:   "generate-combined-index {source-blocks-url} {acct-index-url} {irr-index-url} {start-block-num} [stop-block-num]",
	Short: "Generate index files for eth accounts + event signatures present in blocks (logs and/or calls)",
	Args:  cobra.RangeArgs(4, 5),
	RunE:  generateCombinedIdxE,
}

func init() {
	generateCombinedIdxCmd.Flags().Uint64("combined-indexes-size", 10000, "size of combined index bundles that will be created")
	generateCombinedIdxCmd.Flags().IntSlice("lookup-combined-indexes-sizes", []int{1000000, 100000, 10000, 1000}, "combined index bundle sizes that we will look for on start to find first unindexed block (should include combined-indexes-size)")
	generateCombinedIdxCmd.Flags().IntSlice("irreversible-indexes-sizes", []int{10000, 1000}, "size of irreversible indexes that will be used")
	generateCombinedIdxCmd.Flags().Bool("create-irreversible-indexes", false, "if true, irreversible indexes will also be created")
	Cmd.AddCommand(generateCombinedIdxCmd)
}

func generateCombinedIdxE(cmd *cobra.Command, args []string) error {

	createIrr, err := cmd.Flags().GetBool("create-irreversible-indexes")
	if err != nil {
		return err
	}
	iis, err := cmd.Flags().GetIntSlice("irreversible-indexes-sizes")
	if err != nil {
		return err
	}
	var irrIdxSizes []uint64
	for _, size := range iis {
		if size < 0 {
			return fmt.Errorf("invalid negative size for bundle-sizes: %d", size)
		}
		irrIdxSizes = append(irrIdxSizes, uint64(size))
	}

	idxSize, err := cmd.Flags().GetUint64("combined-indexes-size")
	if err != nil {
		return err
	}
	lais, err := cmd.Flags().GetIntSlice("lookup-combined-indexes-sizes")
	if err != nil {
		return err
	}
	var lookupIdxSizes []uint64
	for _, size := range lais {
		if size < 0 {
			return fmt.Errorf("invalid negative size for bundle-sizes: %d", size)
		}
		lookupIdxSizes = append(lookupIdxSizes, uint64(size))
	}

	blocksStoreURL := args[0]
	indexStoreURL := args[1]
	irrIndexStoreURL := args[2]
	startBlockNum, err := strconv.ParseUint(args[3], 10, 64)
	if err != nil {
		return fmt.Errorf("unable to parse block number %q: %w", args[3], err)
	}
	var stopBlockNum uint64
	if len(args) == 5 {
		stopBlockNum, err = strconv.ParseUint(args[4], 10, 64)
		if err != nil {
			return fmt.Errorf("unable to parse block number %q: %w", args[4], err)
		}
	}

	blocksStore, err := dstore.NewDBinStore(blocksStoreURL)
	if err != nil {
		return fmt.Errorf("failed setting up block store from url %q: %w", blocksStoreURL, err)
	}

	// we are optionally reading info from the irrIndexStore
	irrIndexStore, err := dstore.NewStore(irrIndexStoreURL, "", "", false)
	if err != nil {
		return fmt.Errorf("failed setting up irreversible blocks index store from url %q: %w", irrIndexStoreURL, err)
	}

	indexStore, err := dstore.NewStore(indexStoreURL, "", "", false)
	if err != nil {
		return fmt.Errorf("failed setting up account index store from url %q: %w", indexStoreURL, err)
	}

	streamFactory := firehose.NewStreamFactory(
		[]dstore.Store{blocksStore},
		irrIndexStore,
		irrIdxSizes,
		nil,
		nil,
		nil,
		nil,
	)
	cmd.SilenceUsage = true

	ctx := context.Background()

	var irrStart uint64
	done := make(chan struct{})
	go func() { // both checks in parallel
		irrStart = bstransform.FindNextUnindexed(ctx, uint64(startBlockNum), irrIdxSizes, "irr", irrIndexStore)
		close(done)
	}()
	idxStart := bstransform.FindNextUnindexed(ctx, uint64(startBlockNum), lookupIdxSizes, transform.CombinedIndexerShortName, indexStore)
	<-done

	if irrStart < idxStart {
		startBlockNum = irrStart
	} else {
		startBlockNum = idxStart
	}

	t := transform.NewEthCombinedIndexer(indexStore, idxSize)
	var irreversibleIndexer *bstransform.IrreversibleBlocksIndexer
	if createIrr {
		irreversibleIndexer = bstransform.NewIrreversibleBlocksIndexer(irrIndexStore, irrIdxSizes, bstransform.IrrWithDefinedStartBlock(startBlockNum))
	}

	handler := bstream.HandlerFunc(func(blk *bstream.Block, obj interface{}) error {
		if createIrr {
			irreversibleIndexer.Add(blk)
		}
		t.ProcessBlock(blk.ToNative().(*pbeth.Block))
		return nil
	})

	req := &pbfirehose.Request{
		StartBlockNum: int64(startBlockNum),
		StopBlockNum:  stopBlockNum,
		ForkSteps:     []pbfirehose.ForkStep{pbfirehose.ForkStep_STEP_IRREVERSIBLE},
	}
	stream, err := streamFactory.New(
		ctx,
		handler,
		req,
		zlog,
	)
	if err != nil {
		return fmt.Errorf("getting firehose stream: %w", err)
	}

	return stream.Run(ctx)

}
