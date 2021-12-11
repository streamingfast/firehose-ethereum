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

package trxdbtest

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/streamingfast/kvdb"
	ct "github.com/streamingfast/sf-ethereum/codec/testing"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/codec/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var dbReaderTests = []testFunc{
	TestGetBlock,
	TestGetBlockByNum,
	TestGetClosestIrreversibleIDAtBlockNum,
	TestGetIrreversibleIDAtBlockID,
	TestGetLastWrittenBlockID,
}

func TestAllDbReader(t *testing.T, driverName string, driverFactory DriverFactory) {
	for _, rt := range dbReaderTests {
		t.Run(driverName+"/db_reader/"+getFunctionName(rt), func(t *testing.T) {
			rt(t, driverFactory)
		})
	}
}

func TestGetBlock(t *testing.T, driverFactory DriverFactory) {
	tests := []struct {
		name            string
		block           *pbcodec.Block
		blockHash       string
		expectErr       error
		expectBlockHash string
	}{
		{
			name:            "sunny path",
			block:           ct.Block(t, "00000002aa"),
			blockHash:       "00000002aa",
			expectBlockHash: "00000002aa",
		},
		{
			name:      "block does not exist",
			block:     ct.Block(t, "00000002aa"),
			blockHash: "00000003aa",
			expectErr: kvdb.ErrNotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var ctx = context.Background()
			db, clean := driverFactory()
			defer clean()

			require.NoError(t, db.PutBlock(ctx, test.block))
			require.NoError(t, db.Flush(ctx))

			resp, err := db.GetBlock(ctx, ct.Hash(test.blockHash).Bytes(t))

			if test.expectErr != nil {
				assert.Equal(t, test.expectErr, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.block.Hash, resp.Block.Hash)
			}
		})
	}
}

func TestGetBlockByNum(t *testing.T, driverFactory DriverFactory) {
	tests := []struct {
		name           string
		blocks         []*pbcodec.Block
		blockNum       uint64
		expectErr      error
		expectBlockIds []string
	}{
		{
			name: "sunny path",
			blocks: []*pbcodec.Block{
				ct.Block(t, "00000002aa"),
				ct.Block(t, "00000003aa"),
				ct.Block(t, "00000004aa"),
			},
			blockNum:       3,
			expectBlockIds: []string{"00000003aa"},
		},
		{
			name: "block does not exist",
			blocks: []*pbcodec.Block{
				ct.Block(t, "00000002aa"),
			},
			blockNum:  3,
			expectErr: kvdb.ErrNotFound,
		},
		{
			name: "return multiple blocks with same number",
			blocks: []*pbcodec.Block{
				ct.Block(t, "00000002aa"),
				ct.Block(t, "00000002dd"),
			},
			blockNum:       2,
			expectBlockIds: []string{"00000002aa", "00000002dd"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var ctx = context.Background()
			db, clean := driverFactory()
			defer clean()

			for _, blk := range test.blocks {
				require.NoError(t, db.PutBlock(ctx, blk))
			}
			require.NoError(t, db.Flush(ctx))

			resp, err := db.GetBlockByNum(ctx, test.blockNum)

			if test.expectErr != nil {
				assert.Equal(t, test.expectErr, err)
			} else {
				require.NoError(t, err)
				ids := []string{}
				for _, blk := range resp {
					ids = append(ids, hex.EncodeToString(blk.Block.Hash))
				}
				assert.ElementsMatch(t, test.expectBlockIds, ids)
			}
		})
	}
}

func TestGetClosestIrreversibleIDAtBlockNum(t *testing.T, driverFactory DriverFactory) {
	tests := []struct {
		name            string
		blocks          []*pbcodec.Block
		irrBlock        []*pbcodec.Block
		blockNum        uint64
		expectBlockHash string
		expectErr       error
	}{
		{
			name: "sunny path",
			blocks: []*pbcodec.Block{
				ct.Block(t, "00000002aa"),
				ct.Block(t, "00000003aa"),
				ct.Block(t, "00000005aa"),
				ct.Block(t, "00000006aa"),
				ct.Block(t, "00000007aa"),
				ct.Block(t, "00000008aa"),
			},
			irrBlock: []*pbcodec.Block{
				ct.Block(t, "00000002aa"),
				ct.Block(t, "00000003aa"),
				ct.Block(t, "00000005aa"),
			},
			blockNum:        8,
			expectBlockHash: "00000005aa",
		},
		{
			name: "no irr blocks",
			blocks: []*pbcodec.Block{
				ct.Block(t, "00000002aa"),
				ct.Block(t, "00000003aa"),
				ct.Block(t, "00000005aa"),
				ct.Block(t, "00000006aa"),
				ct.Block(t, "00000007aa"),
				ct.Block(t, "00000008aa"),
			},
			irrBlock:  nil,
			blockNum:  8,
			expectErr: kvdb.ErrNotFound,
		},
		{
			name: "looking for irr block",
			blocks: []*pbcodec.Block{
				ct.Block(t, "00000002aa"),
				ct.Block(t, "00000003aa"),
				ct.Block(t, "00000005aa"),
				ct.Block(t, "00000006aa"),
				ct.Block(t, "00000007aa"),
				ct.Block(t, "00000008aa"),
			},
			irrBlock: []*pbcodec.Block{
				ct.Block(t, "00000002aa"),
				ct.Block(t, "00000003aa"),
				ct.Block(t, "00000004aa"),
				ct.Block(t, "00000005aa"),
			},
			blockNum:        5,
			expectBlockHash: "00000005aa",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var ctx = context.Background()
			db, clean := driverFactory()
			defer clean()

			for _, blk := range test.blocks {
				require.NoError(t, db.PutBlock(ctx, blk))
			}

			for _, blk := range test.irrBlock {
				require.NoError(t, db.UpdateNowIrreversibleBlock(ctx, blk))
			}
			require.NoError(t, db.Flush(ctx))

			resp, err := db.GetClosestIrreversibleIDAtBlockNum(ctx, test.blockNum)

			if test.expectErr != nil {
				assert.Equal(t, test.expectErr, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.expectBlockHash, resp.ID())
			}
		})
	}
}
func TestGetLastWrittenBlockID(t *testing.T, driverFactory DriverFactory) {
	tests := []struct {
		name            string
		blocks          []*pbcodec.Block
		expectBlockHash string
		expectError     error
	}{
		{
			name: "sunny path",
			blocks: []*pbcodec.Block{
				ct.Block(t, "00000002aa"),
				ct.Block(t, "00000003aa"),
				ct.Block(t, "00000005aa"),
				ct.Block(t, "00000006aa"),
				ct.Block(t, "00000007aa"),
				ct.Block(t, "00000008aa"),
			},
			expectBlockHash: "00000008aa",
		},
		{
			name:            "not found",
			blocks:          []*pbcodec.Block{},
			expectBlockHash: "",
			expectError:     kvdb.ErrNotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var ctx = context.Background()
			db, clean := driverFactory()
			defer clean()

			for _, blk := range test.blocks {
				require.NoError(t, db.PutBlock(ctx, blk))
			}
			require.NoError(t, db.Flush(ctx))

			resp, err := db.GetLastWrittenBlock(ctx)

			if test.expectError == nil {
				require.NoError(t, err)
				assert.Equal(t, test.expectBlockHash, resp.ID())
			} else {
				require.Equal(t, test.expectError, err)
			}
		})
	}
}

func TestGetIrreversibleIDAtBlockID(t *testing.T, driverFactory DriverFactory) {
	tests := []struct {
		name            string
		blocks          []*pbcodec.Block
		irrBlock        []*pbcodec.Block
		blockHash       string
		expectBlockHash string
		expectErr       error
	}{
		{
			name: "sunny path",
			blocks: []*pbcodec.Block{
				ct.Block(t, "00000001aa"),
				ct.Block(t, "00000002aa"),
				ct.Block(t, "00000003aa"),
				ct.Block(t, "00000005aa"),
				ct.Block(t, "00000006aa"),
				ct.Block(t, "00000007aa"),
				ct.Block(t, "00000008aa"),
			},
			irrBlock: []*pbcodec.Block{
				ct.Block(t, "00000001aa"),
				ct.Block(t, "00000002aa"),
				ct.Block(t, "00000003aa"),
				ct.Block(t, "00000005aa"),
				ct.Block(t, "00000006aa"),
				ct.Block(t, "00000007aa"),
			},
			// This will try to fetch LIB block #1 (References block ##8 which has LIB num of #1 (-200))
			blockHash:       "00000008aa",
			expectBlockHash: "00000001aa",
		},
		{
			name: "no irr blocks",
			blocks: []*pbcodec.Block{
				ct.Block(t, "00000001aa"),
			},
			irrBlock: nil,
			// This will try to fetch LIB block #2 (References block #202 which has LIB num of #2 (-200))
			blockHash: "000000caaa",
			expectErr: kvdb.ErrNotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var ctx = context.Background()
			db, clean := driverFactory()
			defer clean()

			for _, blk := range test.blocks {
				require.NoError(t, db.PutBlock(ctx, blk))
			}

			for _, blk := range test.irrBlock {
				require.NoError(t, db.UpdateNowIrreversibleBlock(ctx, blk))
			}

			require.NoError(t, db.Flush(ctx))

			resp, err := db.GetIrreversibleIDAtBlockID(ctx, ct.Hash(test.blockHash).Bytes(t))

			if test.expectErr != nil {
				assert.Equal(t, test.expectErr, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.expectBlockHash, resp.ID())
			}
		})
	}
}
