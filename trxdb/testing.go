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

	pbcodec "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/codec/v1"
)

type TestTransactionsReader struct {
	content map[string][]*pbcodec.TransactionTraceWithBlockRef
}

func NewTestTransactionsReader(content map[string][]*pbcodec.TransactionTraceWithBlockRef) *TestTransactionsReader {
	return &TestTransactionsReader{content: content}
}

func (r *TestTransactionsReader) GetTransaction(ctx context.Context, hash string) ([]*pbcodec.TransactionTraceWithBlockRef, error) {
	return r.content[hash], nil
}

func (r *TestTransactionsReader) GetTransactionsForBlock(ctx context.Context, hashes []string, blockHash string, blockNumber uint64) (out [][]*pbcodec.TransactionTraceWithBlockRef, err error) {
	panic("not implemented")
}
