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
	"encoding/hex"
	"fmt"
	"math"

	"github.com/streamingfast/bstream"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/dfuse/ethereum/codec/v1"
	pbtrxdb "github.com/streamingfast/sf-ethereum/pb/dfuse/ethereum/trxdb/v1"
	"go.uber.org/zap"
)

func (db *DB) Flush(ctx context.Context) error {
	return db.store.FlushPuts(ctx)
}

func (db *DB) GetLastWrittenIrreversibleBlockRef(ctx context.Context) (ref bstream.BlockRef, err error) {
	return db.GetClosestIrreversibleIDAtBlockNum(ctx, math.MaxUint64)
}

func (db *DB) PutBlock(ctx context.Context, blk *pbcodec.Block) error {
	if err := db.putTransactionTraces(ctx, blk); err != nil {
		return fmt.Errorf("put block: unable to putTransactions: %w", err)
	}

	return db.putBlock(ctx, blk)
}

func (db *DB) putTransactionTraces(ctx context.Context, blk *pbcodec.Block) error {
	for _, trxTrace := range blk.TransactionTraces {
		trxTraceRow := &pbtrxdb.TrxTraceRow{
			BlockHeader: blk.Header,
			TrxTrace:    trxTrace,
		}

		zlog.Debug("put transaction trace", zap.Stringer("trx_hash", hash(trxTrace.Hash)))
		key := Keys.PackTrxTracesKey(trxTrace.Hash, blk.Number, blk.Hash)
		if err := db.store.Put(ctx, key, db.enc.MustProto(trxTraceRow)); err != nil {
			return fmt.Errorf("put trxTraceRow: write to db: %w", err)
		}
	}

	return nil
}

var oneByte = []byte{0x01}

func (db *DB) putBlock(ctx context.Context, blk *pbcodec.Block) error {
	tracesRefs := db.getRefs(blk)

	holdTransactionTraces := blk.TransactionTraces
	blk.TransactionTraces = nil

	blockRow := &pbtrxdb.BlockRow{
		Block:     blk,
		TraceRefs: tracesRefs,
	}

	zlog.Debug("put block", zap.Stringer("block", blk.AsRef()))

	key := Keys.PackBlocksKey(blk.Hash)
	if err := db.store.Put(ctx, key, db.enc.MustProto(blockRow)); err != nil {
		return fmt.Errorf("put block: write to db: %w", err)
	}

	key = Keys.PackBlockNumsKey(blk.Number, blk.Hash)
	if err := db.store.Put(ctx, key, oneByte); err != nil {
		return fmt.Errorf("put block num: write to db: %w", err)
	}

	blk.TransactionTraces = holdTransactionTraces

	return nil
}

func (db *DB) RevertNowIrreversibleBlock(ctx context.Context, blk *pbcodec.Block) error {
	blockTime := blk.MustTime()
	blkRef := blk.AsRef()

	keysToDelete := [][]byte{
		Keys.PackTimelineKey(true, blockTime, blkRef.ID()),
		Keys.PackTimelineKey(false, blockTime, blkRef.ID()),
		Keys.PackIrrBlocksKeyRef(blkRef),
	}

	zlog.Debug("reverting irreversibility", zap.Stringer("block", blkRef))
	if err := db.store.BatchDelete(ctx, keysToDelete); err != nil {
		return err
	}

	return nil
}

func (db *DB) UpdateNowIrreversibleBlock(ctx context.Context, blk *pbcodec.Block) error {
	blockTime := blk.MustTime()
	blkRef := blk.AsRef()

	if err := db.store.Put(ctx, Keys.PackTimelineKey(true, blockTime, blkRef.ID()), oneByte); err != nil {
		return err
	}
	if err := db.store.Put(ctx, Keys.PackTimelineKey(false, blockTime, blkRef.ID()), oneByte); err != nil {
		return err
	}

	zlog.Debug("adding irreversible block", zap.Stringer("block", blkRef))
	if err := db.store.Put(ctx, Keys.PackIrrBlocksKeyRef(blkRef), oneByte); err != nil {
		return err
	}

	return nil
}

func (db *DB) getRefs(blk *pbcodec.Block) (tracesRefs *pbcodec.TransactionRefs) {
	tracesRefs = &pbcodec.TransactionRefs{
		Hashes: make([][]byte, len(blk.TransactionTraces)),
	}

	for i, trxTrace := range blk.TransactionTraces {
		tracesRefs.Hashes[i] = trxTrace.Hash
	}

	return
}

type hash []byte

func (b hash) String() string {
	return hex.EncodeToString([]byte(b))
}

type hashAsString string

func (s hashAsString) MustBytes() []byte {
	out, err := hex.DecodeString(string(s))
	if err != nil {
		panic(fmt.Errorf("unable to decode %q as hex: %w", s, err))
	}

	return out
}
