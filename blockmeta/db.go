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

package blockmeta

import (
	"context"
	"encoding/hex"
	"errors"
	"time"

	"github.com/streamingfast/bstream"
	"github.com/streamingfast/sf-ethereum/trxdb"
)

var ErrNotImplemented = errors.New("not implemented")

type ETHBlockmetaDB struct {
	DB trxdb.DB
}

func (_ *ETHBlockmetaDB) GetForkPreviousBlocks(ctx context.Context, forkTop bstream.BlockRef) ([]bstream.BlockRef, error) {
	return nil, ErrNotImplemented
}

func (db *ETHBlockmetaDB) BlockIDAt(ctx context.Context, start time.Time) (id string, err error) {
	return db.DB.BlockIDAt(ctx, start)
}

func (db *ETHBlockmetaDB) BlockIDAfter(ctx context.Context, start time.Time, inclusive bool) (id string, foundtime time.Time, err error) {
	return db.DB.BlockIDAfter(ctx, start, inclusive)
}

func (db *ETHBlockmetaDB) BlockIDBefore(ctx context.Context, start time.Time, inclusive bool) (id string, foundtime time.Time, err error) {
	return db.DB.BlockIDBefore(ctx, start, inclusive)
}

func (db *ETHBlockmetaDB) GetLastWrittenBlockID(ctx context.Context) (blockID string, err error) {
	block, err := db.DB.GetLastWrittenBlock(ctx)
	if err != nil {
		return "", err
	}
	return block.ID(), nil
}

func (db *ETHBlockmetaDB) GetIrreversibleIDAtBlockNum(ctx context.Context, num uint64) (ref bstream.BlockRef, err error) {
	return db.DB.GetClosestIrreversibleIDAtBlockNum(ctx, num)
}

func (db *ETHBlockmetaDB) GetIrreversibleIDAtBlockID(ctx context.Context, id string) (ref bstream.BlockRef, err error) {
	h, err := hex.DecodeString(id)
	if err != nil {
		return nil, err
	}
	return db.DB.GetIrreversibleIDAtBlockID(ctx, h)
}
