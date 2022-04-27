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

var generateAccIdxCmd = &cobra.Command{
	// TODO: make irr-index-url optional, maybe ?????
	Use:   "generate-account-index {source-blocks-url} {acct-index-url} {irr-index-url} {start-block-num} [stop-block-num]",
	Short: "Generate index files for eth accounts present in blocks",
	Args:  cobra.RangeArgs(4, 5),
	RunE:  generateAccIdxE,
}

func init() {
	generateAccIdxCmd.Flags().Uint64("account-indexes-size", 10000, "size of account index bundles that will be created")
	generateAccIdxCmd.Flags().IntSlice("lookup-account-indexes-sizes", []int{1000000, 100000, 10000, 1000}, "account index bundle sizes that we will look for on start to find first unindexed block (should include account-indexes-size)")
	generateAccIdxCmd.Flags().IntSlice("irreversible-indexes-sizes", []int{10000, 1000}, "size of irreversible indexes that will be used")
	generateAccIdxCmd.Flags().Bool("create-irreversible-indexes", false, "if true, irreversible indexes will also be created")
	Cmd.AddCommand(generateAccIdxCmd)
}

func lowBoundary(i uint64, mod uint64) uint64 {
	return i - (i % mod)
}

func toIndexFilename(bundleSize, baseBlockNum uint64, shortname string) string {
	return fmt.Sprintf("%010d.%d.%s.idx", baseBlockNum, bundleSize, shortname)
}

func generateAccIdxE(cmd *cobra.Command, args []string) error {

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

	acctIdxSize, err := cmd.Flags().GetUint64("account-indexes-size")
	if err != nil {
		return err
	}
	lais, err := cmd.Flags().GetIntSlice("lookup-account-indexes-sizes")
	if err != nil {
		return err
	}
	var lookupAccountIdxSizes []uint64
	for _, size := range lais {
		if size < 0 {
			return fmt.Errorf("invalid negative size for bundle-sizes: %d", size)
		}
		lookupAccountIdxSizes = append(lookupAccountIdxSizes, uint64(size))
	}

	blocksStoreURL := args[0]
	accountIndexStoreURL := args[1]
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

	// we are creating accountIndexStore
	accountIndexStore, err := dstore.NewStore(accountIndexStoreURL, "", "", false)
	if err != nil {
		return fmt.Errorf("failed setting up account index store from url %q: %w", accountIndexStoreURL, err)
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
	accStart := bstransform.FindNextUnindexed(ctx, uint64(startBlockNum), lookupAccountIdxSizes, transform.LogAddrIndexShortName, accountIndexStore)
	<-done

	fmt.Println("irrStart", irrStart, "accStart", accStart)
	if irrStart < accStart {
		startBlockNum = irrStart
	} else {
		startBlockNum = accStart
	}

	t := transform.NewEthLogIndexer(accountIndexStore, acctIdxSize)
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
