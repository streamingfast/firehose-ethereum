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
	"testing"

	ct "github.com/streamingfast/sf-ethereum/codec/testing"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/dfuse/ethereum/codec/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var dbWritterTests = []testFunc{
	TestPutBlock,
	TestUpdateNowIrreversibleBlock,
}

func TestAllDbWriter(t *testing.T, driverName string, driverFactory DriverFactory) {
	for _, rt := range dbWritterTests {
		t.Run(driverName+"/db_writer/"+getFunctionName(rt), func(t *testing.T) {
			rt(t, driverFactory)
		})
	}
}

func TestPutBlock(t *testing.T, driverFactory DriverFactory) {
	tests := []struct {
		name              string
		block             *pbcodec.Block
		expectErr         bool
		expectedBlockHash string
	}{
		{
			name:              "golden path",
			block:             ct.Block(t, "00000002aa"),
			expectedBlockHash: "00000002aa",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			db, clean := driverFactory()
			defer clean()

			require.NoError(t, db.PutBlock(ctx, test.block))
			require.NoError(t, db.Flush(ctx))

			resp, err := db.GetBlock(ctx, test.block.Hash)
			require.NoError(t, err)
			assert.Equal(t, test.block.Hash, resp.Block.Hash)
		})
	}
}

func TestUpdateNowIrreversibleBlock(t *testing.T, driverFactory DriverFactory) {
	tests := []struct {
		name              string
		block             *pbcodec.Block
		expectErr         bool
		expectedBlockHash string
	}{
		{
			name:              "golden path",
			block:             ct.Block(t, "00000002aa"),
			expectedBlockHash: "00000002aa",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			db, clean := driverFactory()
			defer clean()

			require.NoError(t, db.PutBlock(ctx, test.block))
			require.NoError(t, db.UpdateNowIrreversibleBlock(ctx, test.block))
			require.NoError(t, db.Flush(ctx))

			resp, err := db.GetBlock(ctx, test.block.Hash)
			require.NoError(t, err)
			assert.True(t, resp.Irreversible)
		})
	}
}
