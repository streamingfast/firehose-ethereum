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

package filtering

import (
	"bytes"

	"github.com/streamingfast/eth-go"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/codec/v1"
)

type trxHashFilter struct {
	hash []byte
	id   string
}

func NewTrxHashFilter(id string) (TrxFilter, error) {
	hash, err := eth.NewHash(id)
	if err != nil {
		return nil, err
	}
	return newTrxHashFilter(hash), nil
}

func NewTrxHashFilterBytes(hash []byte) TrxFilter {
	return newTrxHashFilter(eth.Hash(hash))
}

func newTrxHashFilter(hash eth.Hash) *trxHashFilter {
	return &trxHashFilter{
		id:   hash.String(),
		hash: hash.Bytes(),
	}
}

func (f *trxHashFilter) Matches(transaction interface{}, cache *TrxFilterCache) (bool, []uint32) {

	var calls []*pbcodec.Call
	switch trx := transaction.(type) {
	case *pbcodec.Transaction:
		return bytes.Equal(trx.Hash, f.hash), nil
	case *pbcodec.TransactionTrace:
		if !bytes.Equal(trx.Hash, f.hash) {
			return false, nil
		}
		calls = trx.Calls
	case *pbcodec.TransactionTraceWithBlockRef:
		calls = trx.Trace.Calls
		if !bytes.Equal(trx.Trace.Hash, f.hash) {
			return false, nil
		}
	}

	var matchingCalls []uint32
	for i := range calls {
		matchingCalls = append(matchingCalls, uint32(i))
	}
	return true, matchingCalls
}

func (f *trxHashFilter) String() string {
	return "singlehash:" + f.id
}
