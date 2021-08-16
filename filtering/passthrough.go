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
	pbcodec "github.com/streamingfast/sf-ethereum/pb/dfuse/ethereum/codec/v1"
)

type passthroughFilter struct{}

func NewPassthroughFilter() TrxFilter {
	return newPassthroughFilter()
}

func newPassthroughFilter() *passthroughFilter {
	return &passthroughFilter{}
}

func (f *passthroughFilter) Matches(transaction interface{}, cache *TrxFilterCache) (bool, []uint32) {

	var calls []*pbcodec.Call
	switch trx := transaction.(type) {
	case *pbcodec.Transaction:
		return true, nil
	case *pbcodec.TransactionTrace:
		calls = trx.Calls
	case *pbcodec.TransactionTraceWithBlockRef:
		calls = trx.Trace.Calls
	}

	var matchingCalls []uint32
	for i := range calls {
		matchingCalls = append(matchingCalls, uint32(i))
	}
	return true, matchingCalls
}

func (f *passthroughFilter) String() string {
	return "passthroughFilter:"
}
