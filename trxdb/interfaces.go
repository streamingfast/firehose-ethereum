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

package trxdb

import (
	"context"
	"time"

	"github.com/streamingfast/bstream"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/dfuse/ethereum/codec/v1"
)

type ChainDiscriminator func(blockRef bstream.BlockRef) bool

type DB interface {
	DBReader
	DBWriter
}

type DBReader interface {
	BlocksReader
	TimelineExplorer
	TransactionsReader
}

type BlocksReader interface {
	GetBlock(ctx context.Context, blockHash []byte) (*pbcodec.BlockWithRefs, error)
	GetBlockByNum(ctx context.Context, blockNum uint64) ([]*pbcodec.BlockWithRefs, error)
	GetLastWrittenBlock(ctx context.Context) (bstream.BlockRef, error)
	GetLIBBeforeBlockNum(ctx context.Context, blockNum uint64) (bstream.BlockRef, error)
	GetLIBBeforeBlockID(ctx context.Context, blockHash []byte) (bstream.BlockRef, error)
	GetIrreversibleIDAtBlockID(ctx context.Context, blockHash []byte) (ref bstream.BlockRef, err error)
	GetClosestIrreversibleIDAtBlockNum(ctx context.Context, blockNum uint64) (ref bstream.BlockRef, err error)
}

type TimelineExplorer interface {
	BlockIDAt(ctx context.Context, start time.Time) (id string, err error)
	BlockIDAfter(ctx context.Context, start time.Time, inclusive bool) (id string, foundTime time.Time, err error)
	BlockIDBefore(ctx context.Context, start time.Time, inclusive bool) (id string, foundTime time.Time, err error)
}

type TransactionsReader interface {
	GetTransaction(ctx context.Context, hash []byte) ([]*pbcodec.TransactionTraceWithBlockRef, error)
	GetTransactionsForBlock(ctx context.Context, blockNum uint64, blockHash []byte, hashes [][]byte) ([]*pbcodec.TransactionTraceWithBlockRef, error)
}

type DBWriter interface {
	// Where did I leave off last time I wrote?
	GetLastWrittenIrreversibleBlockRef(ctx context.Context) (ref bstream.BlockRef, err error)
	PutBlock(ctx context.Context, blk *pbcodec.Block) error
	UpdateNowIrreversibleBlock(ctx context.Context, blk *pbcodec.Block) error

	RevertNowIrreversibleBlock(ctx context.Context, blk *pbcodec.Block) error

	// Flush MUST be called or you WILL lose data
	Flush(context.Context) error
}
