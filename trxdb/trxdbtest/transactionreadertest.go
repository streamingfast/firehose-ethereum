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

	ct "github.com/streamingfast/sf-ethereum/codec/testing"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/dfuse/ethereum/codec/v1"
	"github.com/streamingfast/sf-ethereum/trxdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var transactionReaderTests = []testFunc{
	TestGetTransactionTraces,
}

func TestAllTransactionsReader(t *testing.T, driverName string, driverFactory DriverFactory) {
	for _, rt := range transactionReaderTests {
		t.Run(driverName+"/"+getFunctionName(rt), func(t *testing.T) {
			rt(t, driverFactory)
		})
	}
}

func TestGetTransactionTraces(t *testing.T, driverFactory DriverFactory) {
	tests := []struct {
		name               string
		trxHashes          []string
		trxHashToSearchFor string
		expectedTrxHashes  []string
		expectErr          error
	}{
		{
			name:               "sunny path",
			trxHashes:          []string{"a1bc5790ef36d5779e2a0a849a11c09c999b5dc564afce6920e20b07af1f4b6a", "a2bc5790ef36d5779e2a0a849a11c09c999b5dc564afce6920e20b07af1f4b6a", "a1bc5790ef36d5779e2a0a849a11c09c999b5dc564afce6920e20b07af1f4b6b"},
			trxHashToSearchFor: "a1",
			expectedTrxHashes:  []string{"a1bc5790ef36d5779e2a0a849a11c09c999b5dc564afce6920e20b07af1f4b6a", "a1bc5790ef36d5779e2a0a849a11c09c999b5dc564afce6920e20b07af1f4b6b"},
		},
		{
			name:               "only match prefix",
			trxHashes:          []string{"a1bc5790ef36d5779e2a0a849a11c09c999b5dc564afce6920e20b07af1f4b6a", "a2bc5790ef36d5779e2a0a849a11c09c999b5dc564afce6920e20b07af1f4b6a", "a1bc5790ef36d5779e2a0a849a11c09c999b5dc564afce6920e20b07af1f4b6b"},
			trxHashToSearchFor: "a1bc5790ef36d5779e2a0a849a11c09c999b5dc564afce6920e20b07af1f4b6a",
			expectedTrxHashes:  []string{"a1bc5790ef36d5779e2a0a849a11c09c999b5dc564afce6920e20b07af1f4b6a"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			db, clean := driverFactory()
			defer clean()

			for _, trxID := range test.trxHashes {
				putTransaction(t, db, trxID)
			}

			trxTraces, err := db.GetTransaction(ctx, ct.Hash(test.trxHashToSearchFor).Bytes(t))
			trxTraceHashes := make([]string, len(trxTraces))
			for i, trxTrace := range trxTraces {
				trxTraceHashes[i] = hex.EncodeToString(trxTrace.Trace.Hash)
			}

			if test.expectErr != nil {
				assert.Equal(t, test.expectErr, err)
			} else {
				require.NoError(t, err)
				assert.ElementsMatch(t, test.expectedTrxHashes, trxTraceHashes)
			}
		})
	}
}

func putTransaction(t *testing.T, db trxdb.DB, trxHash string) {
	blk := ct.Block(t, "06bc5790ef36d5779e2a0a849a11c09c999b5dc564afce6920e20b07af1f4b6a", ct.PreviousHash("06bc5790ef36d5779e2a0a849a11c09c999b5dc564afce6920e20b07af1f4b6a"))
	blk.TransactionTraces = append(blk.TransactionTraces, &pbcodec.TransactionTrace{
		Hash: ct.Hash(trxHash).Bytes(t),
	})

	ctx := context.Background()
	require.NoError(t, db.PutBlock(ctx, blk))
	require.NoError(t, db.UpdateNowIrreversibleBlock(ctx, blk))
	require.NoError(t, db.Flush(ctx))
}
