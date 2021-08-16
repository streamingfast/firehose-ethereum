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

package trxdb_loader

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/streamingfast/kvdb/store/badger"
	_ "github.com/streamingfast/sf-ethereum/trxdb/kv"

	"github.com/golang/protobuf/ptypes"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/bstream/forkable"
	"github.com/streamingfast/jsonpb"
	"github.com/streamingfast/sf-ethereum/codec"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/dfuse/ethereum/codec/v1"
	"github.com/streamingfast/sf-ethereum/trxdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newLoader(t *testing.T, options ...interface{}) (*BigtableLoader, trxdb.DB, func()) {
	db, err := trxdb.New("badger:///tmp?cache=shared&mode=memory&createTables=true")
	require.NoError(t, err)

	l := NewBigtableLoader("", nil, 1, db, 1)
	require.NoError(t, err)

	cleanup := func() {
	}

	return l, db, cleanup
}

func TestBigtableLoader(t *testing.T) {
	loader, trxdbDriver, cleanup := newLoader(t)
	defer cleanup()

	ctx := context.Background()
	blockHash := "00000002aa"
	previousRef := bstream.NewBlockRefFromID("00000001aa")
	block := testBlock(t, 2, "00000002aa")
	block.Header.ParentHash = stringToHash(t, previousRef.ID())
	block.Ver = 1

	blk, err := codec.BlockFromProto(block)
	require.NoError(t, err)

	fkable := forkable.New(loader, forkable.WithExclusiveLIB(previousRef))
	require.NoError(t, fkable.ProcessBlock(blk, nil))
	_ = loader.UpdateIrreversibleData([]*bstream.PreprocessedBlock{{Block: blk}})
	require.NoError(t, loader.db.Flush(ctx))

	resp, err := trxdbDriver.GetBlock(ctx, trxdb.MustHexDecode(blockHash))
	require.NoError(t, err)
	assert.Equal(t, blockHash, hex.EncodeToString(resp.Block.Hash))
	assert.True(t, resp.Irreversible)
}

func TestBigtableLoader_Timeline(t *testing.T) {
	t.Skip() // not yet ready without sqlite
	loader, trxdbDriver, cleanup := newLoader(t)
	defer cleanup()

	ctx := context.Background()
	blockID := "00000002aa"
	previousRef := bstream.NewBlockRefFromID("00000001aa")
	block := testBlock(t, 2, "00000002aa")
	block.Header.ParentHash = stringToHash(t, previousRef.ID())
	block.Ver = 1
	blk, err := codec.BlockFromProto(block)
	require.NoError(t, err)

	fkable := forkable.New(loader, forkable.WithExclusiveLIB(previousRef))
	require.NoError(t, fkable.ProcessBlock(blk, nil))
	loader.UpdateIrreversibleData([]*bstream.PreprocessedBlock{{Block: blk}})
	require.NoError(t, trxdbDriver.Flush(ctx))

	respID, _, err := trxdbDriver.BlockIDBefore(ctx, blk.Time(), true) // direct timestamp
	assert.NoError(t, err)
	assert.Equal(t, blockID, respID)

	respID, _, err = trxdbDriver.BlockIDBefore(ctx, blk.Time(), true) // direct timestamp
	assert.NoError(t, err)
	assert.Equal(t, blockID, respID)

	respID, _, err = trxdbDriver.BlockIDAfter(ctx, time.Time{}, true) // first block since epoch
	assert.NoError(t, err)
	assert.Equal(t, blockID, respID)

	respID, _, err = trxdbDriver.BlockIDBefore(ctx, blk.Time().Add(-time.Second), true) // nothing before
	assert.Error(t, err)

	respID, _, err = trxdbDriver.BlockIDAfter(ctx, blk.Time().Add(time.Second), true) // nothing after
	assert.Error(t, err)
}

func testBlock(t *testing.T, num uint64, id string, trxTraceJSONs ...string) *pbcodec.Block {
	trxTraces := make([]*pbcodec.TransactionTrace, len(trxTraceJSONs))
	for i, trxTraceJSON := range trxTraceJSONs {
		trxTrace := new(pbcodec.TransactionTrace)
		require.NoError(t, jsonpb.UnmarshalString(trxTraceJSON, trxTrace))

		trxTraces[i] = trxTrace
	}

	pbblock := &pbcodec.Block{
		Hash:              stringToHash(t, id),
		Number:            num,
		TransactionTraces: trxTraces,
	}

	blockTime, err := time.Parse(time.RFC3339, "2006-01-02T15:04:05.5Z")
	require.NoError(t, err)

	blockTimestamp, err := ptypes.TimestampProto(blockTime)
	require.NoError(t, err)

	pbblock.Header = &pbcodec.BlockHeader{
		ParentHash: stringToHash(t, fmt.Sprintf("%08d%s", pbblock.Number-1, id[8:])),
		Timestamp:  blockTimestamp,
	}

	if os.Getenv("DEBUG") != "" {
		marshaler := &jsonpb.Marshaler{}
		out, err := marshaler.MarshalToString(pbblock)
		require.NoError(t, err)

		// We re-normalize to a plain map[string]interface{} so it's printed as JSON and not a proto default String implementation
		normalizedOut := map[string]interface{}{}
		require.NoError(t, json.Unmarshal([]byte(out), &normalizedOut))

		zlog.Debug("created test block", zap.Any("block", normalizedOut))
	}

	return pbblock
}

func stringToHash(t *testing.T, hash string) []byte {
	bytes, err := hex.DecodeString(hash)
	require.NoError(t, err)

	return bytes
}
