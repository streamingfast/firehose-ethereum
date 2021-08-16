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

	"github.com/streamingfast/kvdb"
	"github.com/streamingfast/kvdb/store"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/dfuse/ethereum/codec/v1"
	pbtrxdb "github.com/streamingfast/sf-ethereum/pb/dfuse/ethereum/trxdb/v1"
	"go.uber.org/zap"
)

func (db *DB) GetTransaction(ctx context.Context, trxHash []byte) (out []*pbcodec.TransactionTraceWithBlockRef, err error) {
	ctx, cancelScanPrefix := context.WithCancel(ctx)
	defer cancelScanPrefix()

	if traceEnabled {
		zlog.Debug("get transaction", zap.Stringer("hash", hash(trxHash)))
	}

	it := db.store.Prefix(ctx, Keys.PackTrxTracesPrefix(trxHash), store.Unlimited)
	for it.Next() {
		kv := it.Item()

		row := &pbtrxdb.TrxTraceRow{}
		db.dec.MustInto(kv.Value, row)
		out = append(out, &pbcodec.TransactionTraceWithBlockRef{
			Trace: row.TrxTrace,
			BlockRef: &pbcodec.BlockRef{
				Hash:   row.BlockHeader.Hash,
				Number: row.BlockHeader.Number,
			},
		})
	}
	if err := it.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, kvdb.ErrNotFound
	}

	return out, nil
}

func (db *DB) GetTransactionBatch(ctx context.Context, hashes [][]byte) (out []*pbcodec.TransactionTraceWithBlockRef, err error) {
	ctx, cancelScanPrefix := context.WithCancel(ctx)
	defer cancelScanPrefix()

	var keys [][]byte
	for _, hash := range hashes {
		keys = append(keys, Keys.PackTrxTracesPrefix(hash))
	}

	zlog.Debug("get transaction batch", zap.Int("num_of_hashed", len(hashes)))
	it := db.store.BatchGet(ctx, keys)
	for it.Next() {
		kv := it.Item()
		row := &pbtrxdb.TrxTraceRow{}
		db.dec.MustInto(kv.Value, row)
		out = append(out, &pbcodec.TransactionTraceWithBlockRef{
			Trace: row.TrxTrace,
			BlockRef: &pbcodec.BlockRef{
				Hash:   row.BlockHeader.Hash,
				Number: row.BlockHeader.Number,
			},
		})
	}
	if err := it.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func (db *DB) GetTransactionsForBlock(ctx context.Context, blockNum uint64, blockHash []byte, hashes [][]byte) (out []*pbcodec.TransactionTraceWithBlockRef, err error) {
	ctx, cancelBatchGet := context.WithCancel(ctx)
	defer cancelBatchGet()

	keys := make([][]byte, len(hashes))
	for i, hash := range hashes {
		keys[i] = Keys.PackTrxTracesKey(hash, blockNum, blockHash)
	}

	it := db.store.BatchGet(ctx, keys)
	for it.Next() {
		kv := it.Item()

		row := &pbtrxdb.TrxTraceRow{}
		db.dec.MustInto(kv.Value, row)
		out = append(out, &pbcodec.TransactionTraceWithBlockRef{
			Trace: row.TrxTrace,
			BlockRef: &pbcodec.BlockRef{
				Hash:   row.BlockHeader.Hash,
				Number: row.BlockHeader.Number,
			},
		})
	}
	if err := it.Err(); err != nil {
		return nil, err
	}

	return out, nil
}
