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

package kv

import (
	"context"
	"fmt"
	"time"

	"github.com/streamingfast/bstream"
	"github.com/streamingfast/kvdb"
	"github.com/streamingfast/kvdb/store"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/codec/v1"
	pbtrxdb "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/trxdb/v1"
	"go.uber.org/zap"
)

func (db *DB) GetLastWrittenBlock(ctx context.Context) (bstream.BlockRef, error) {
	ctx, cancelScan := context.WithCancel(ctx)
	defer cancelScan()

	it := db.store.Scan(ctx, Keys.StartOfBlockNumsTable(), Keys.EndOfBlockNumsTable(), 1)
	found := it.Next()
	if err := it.Err(); err != nil {
		return nil, err
	}

	if !found {
		return nil, kvdb.ErrNotFound
	}

	key := it.Item().Key
	zlog.Debug("retrieved key", zap.Stringer("key", zapKey(key)))
	return Keys.UnpackBlockNumsKey(key), nil
}

func (db *DB) GetBlock(ctx context.Context, hash []byte) (blk *pbcodec.BlockWithRefs, err error) {
	blkRow, err := db.getBlockRow(ctx, Keys.PackBlocksKey(hash))
	if err == kvdb.ErrNotFound {
		return nil, err
	}

	if err != nil {
		return nil, fmt.Errorf("block row: %w", err)
	}

	zlog.Debug("feeding irreversibility for fetched block", zap.Stringer("block", blkRow.Block.AsRef()))
	blk = &pbcodec.BlockWithRefs{
		Block:                blkRow.Block,
		TransactionTraceRefs: blkRow.TraceRefs,
	}

	if err := db.feedIrreversibility(ctx, blk); err != nil {
		return nil, fmt.Errorf("feed irreversibility: %w", err)
	}

	return blk, nil
}

func (db *DB) GetBlockByNum(ctx context.Context, blockNum uint64) (out []*pbcodec.BlockWithRefs, err error) {
	ctx, cancelScan := context.WithCancel(ctx)
	defer cancelScan()

	zlog.Debug("get block by num", zap.Uint64("block_num", blockNum))

	blockIDKeys := [][]byte{}
	var it *store.Iterator
	if blockNum == 0 {
		it = db.store.Prefix(ctx, Keys.PackBlockNumsPrefix(blockNum), 0)
	} else {
		it = db.store.Scan(ctx, Keys.PackBlockNumsPrefix(blockNum), Keys.PackBlockNumsPrefix(blockNum-1), 0)
	}
	for it.Next() {
		kv := it.Item()

		hash := Keys.UnpackBlockNumsKeyHash(kv.Key)
		blockIDKeys = append(blockIDKeys, Keys.PackBlocksKey(hash))
	}
	if err := it.Err(); err != nil {
		return nil, err
	}
	if len(blockIDKeys) == 0 {
		return nil, kvdb.ErrNotFound
	}

	zlog.Debug("fetching blocks from id keys", zap.Int("block_found", len(blockIDKeys)), zap.Stringer("keys", zapKeys(blockIDKeys)))
	return db.batchGetBlocks(ctx, blockIDKeys)
}

func (db *DB) batchGetBlocks(ctx context.Context, keys [][]byte) (blks []*pbcodec.BlockWithRefs, err error) {
	ctxBatch, cancelBatch := context.WithCancel(ctx)
	defer cancelBatch()

	// This whole method performs two batch get, one to retrieve the blocks and another
	// one to retrieve irreversibility of those blocks. We could perform this in parallel
	// using a WaitGroup and launching the two calls in parallel.
	irrKeys := [][]byte{}
	it := db.store.BatchGet(ctxBatch, keys)
	for it.Next() {
		kv := it.Item()

		blockRow := &pbtrxdb.BlockRow{}
		db.dec.MustInto(kv.Value, blockRow)

		irrKeys = append(irrKeys, Keys.PackIrrBlocksKeyRef(blockRow.Block.AsRef()))
		blks = append(blks, &pbcodec.BlockWithRefs{
			Block:                blockRow.Block,
			TransactionTraceRefs: blockRow.TraceRefs,
		})
	}
	if err := it.Err(); err != nil {
		return nil, fmt.Errorf("batch get: %w", err)
	}

	if len(keys) != len(blks) {
		return nil, fmt.Errorf("fetched only %d blocks but had %d keys, write process is not in sync between block nums & block ids", len(blks), len(keys))
	}

	zlog.Debug("feeding irreversibility for fetched blocks", zap.Stringer("irr_keys", zapKeys(irrKeys)), zap.Int("block_count", len(blks)))
	if err := db.batchFeedIrreversibility(ctx, irrKeys, blks); err != nil {
		return nil, fmt.Errorf("feed irreversibility: %w", err)
	}

	return blks, nil
}

func (db *DB) batchFeedIrreversibility(ctx context.Context, irrKeys [][]byte, blks []*pbcodec.BlockWithRefs) error {
	// FIXME: We currently cannot use `BatchGet` here because `BatchGet` returns an error as soon as
	//        one key is not found. The problem here is that not all of the block references might be
	//        irreversible. This means that we could retrieve only 1 out of 3 keys, the 2 other being not
	//        set and as such, return an error.
	// ctxBatch, cancelBatch := context.WithCancel(ctx)
	// defer cancelBatch()

	// // We are not sure the order of batch get will be in same order as blks. Since we expect
	// // only a few values of `blks`, we doing a simple for-loop to find the right block to set.
	// setIrreversible := func(key []byte, isIrreversible bool) {
	// 	searchingForHash := Keys.UnpackIrrBlocksKeyHash(key)
	// 	for _, blk := range blks {
	// 		if bytes.Equal(searchingForHash, blk.Block.Hash) {
	// 			blk.Irreversible = isIrreversible
	// 		}
	// 	}
	// }

	// it := db.store.BatchGet(ctxBatch, irrKeys)
	// for it.Next() {
	// 	setIrreversible(it.Item().Key, true)
	// }

	// if err := it.Err(); err != nil {
	// 	return fmt.Errorf("batch get: %w", err)
	// }

	for _, blk := range blks {
		if err := db.feedIrreversibility(ctx, blk); err != nil {
			return fmt.Errorf("feed irreversibility of %s: %w", blk.Block.AsRef(), err)
		}
	}

	return nil
}

func (db *DB) feedIrreversibility(ctx context.Context, blk *pbcodec.BlockWithRefs) error {
	_, err := db.store.Get(ctx, Keys.PackIrrBlocksKey(blk.Block.Number, blk.Block.Hash))
	if err != nil && err != store.ErrNotFound {
		return fmt.Errorf("get: %w", err)
	}

	blk.Irreversible = err == nil
	return nil
}

func (db *DB) GetClosestIrreversibleIDAtBlockNum(ctx context.Context, num uint64) (ref bstream.BlockRef, err error) {
	zlog.Debug("get closest irr id at block num", zap.Uint64("block_num", num))

	ctx, cancelScan := context.WithCancel(ctx)
	defer cancelScan()

	it := db.store.Scan(ctx, Keys.PackIrrBlocksPrefix(num), Keys.EndOfIrrBlockTable(), 1)
	found := it.Next()
	if err := it.Err(); err != nil {
		return nil, err
	}
	if !found {
		return nil, kvdb.ErrNotFound
	}

	return Keys.UnpackIrrBlocksKey(it.Item().Key), nil
}

func (db *DB) GetIrreversibleIDAtBlockID(ctx context.Context, blockHash []byte) (ref bstream.BlockRef, err error) {
	blkRow, err := db.getBlockRow(ctx, Keys.PackBlocksKey(blockHash))
	if err == kvdb.ErrNotFound {
		return nil, err
	}

	if err != nil {
		return nil, fmt.Errorf("block row: %w", err)
	}

	irrNum := blkRow.Block.LIBNum()

	zlog.Debug("get irr block by num", zap.Uint64("block_num", irrNum))
	blks, err := db.GetBlockByNum(ctx, irrNum)
	if err == kvdb.ErrNotFound {
		return nil, err
	}

	if err != nil {
		return nil, fmt.Errorf("get block by num: %w", err)
	}

	var irrRef bstream.BlockRef
	for _, blk := range blks {
		if blk.Irreversible {
			if irrRef != nil {
				return nil, fmt.Errorf("seen two blocks being irreversible for num #%d, block %s and block %s, this is not expected", irrNum, irrRef, blk.Block.AsRef())
			}

			irrRef = blk.Block.AsRef()
		}
	}

	return irrRef, nil
}

func (db *DB) getBlockRow(ctx context.Context, key []byte) (*pbtrxdb.BlockRow, error) {
	value, err := db.store.Get(ctx, key)
	if err == store.ErrNotFound {
		return nil, kvdb.ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("get: %w", err)
	}

	blkRow := &pbtrxdb.BlockRow{}
	db.dec.MustInto(value, blkRow)

	return blkRow, nil
}

func (db *DB) GetLIBBeforeBlockNum(ctx context.Context, blockNum uint64) (bstream.BlockRef, error) {
	panic("not implemented")
}

func (db *DB) GetLIBBeforeBlockID(ctx context.Context, blockHash []byte) (bstream.BlockRef, error) {
	panic("not implemented")
}

func (db *DB) BlockIDAt(ctx context.Context, start time.Time) (id string, err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	it := db.store.Scan(ctx, Keys.PackTimelinePrefix(true, start), Keys.EndOfTimelineIndex(true), 1)
	found := it.Next()
	if err := it.Err(); err != nil {
		return "", err
	}
	if !found {
		return "", kvdb.ErrNotFound
	}

	blockTime, blockID := Keys.UnpackTimelineKey(true, it.Item().Key)
	if start.Equal(blockTime) {
		return blockID, nil
	}
	return "", kvdb.ErrNotFound
}

func (db *DB) BlockIDAfter(ctx context.Context, start time.Time, inclusive bool) (id string, foundTime time.Time, err error) {
	return db.blockIDAround(ctx, true, start, inclusive)
}

func (db *DB) BlockIDBefore(ctx context.Context, start time.Time, inclusive bool) (id string, foundTime time.Time, err error) {
	return db.blockIDAround(ctx, false, start, inclusive)
}

func (db *DB) blockIDAround(ctx context.Context, fwd bool, start time.Time, inclusive bool) (id string, foundTime time.Time, err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	it := db.store.Scan(ctx, Keys.PackTimelinePrefix(fwd, start), Keys.EndOfTimelineIndex(fwd), 4) // supports 3 blocks at the *same* timestamp, should be pretty rare..

	for it.Next() {
		foundTime, id = Keys.UnpackTimelineKey(fwd, it.Item().Key)
		if !inclusive && foundTime.Equal(start) {
			continue
		}
		return
	}
	if err = it.Err(); err != nil {
		return
	}

	err = kvdb.ErrNotFound
	return
}
