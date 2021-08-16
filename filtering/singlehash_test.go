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
	"testing"

	"github.com/streamingfast/eth-go"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/dfuse/ethereum/codec/v1"
	"github.com/stretchr/testify/assert"
)

func TestSingleHash(t *testing.T) {

	tests := []struct {
		name        string
		hash        []byte
		trx         interface{}
		expectMatch bool
		expectCalls []uint32
	}{
		{
			"matches Trx",
			eth.MustNewHash("0xa7cade838e7bff1545167702500184147c956875332e0d440d77e48e29762a59"),
			&pbcodec.Transaction{
				Hash: toBytes(t, "a7cade838e7bff1545167702500184147c956875332e0d440d77e48e29762a59"),
				Input: toBytes(t, transferMethod+
					leftPad32b("aaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbbb")+
					leftPad32b("00")),
			},
			true,
			nil,
		},
		{
			"matches Trace",
			eth.MustNewHash("0xa7cade838e7bff1545167702500184147c956875332e0d440d77e48e29762a59"),
			&pbcodec.TransactionTrace{
				Hash: toBytes(t, "a7cade838e7bff1545167702500184147c956875332e0d440d77e48e29762a59"),
				Input: toBytes(t, transferMethod+
					leftPad32b("aaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbbb")+
					leftPad32b("00")),
				Calls: []*pbcodec.Call{
					&pbcodec.Call{
						Index: 0,
					},
					&pbcodec.Call{
						Index: 1,
					},
					&pbcodec.Call{
						Index: 2,
					},
				},
			},
			true,
			[]uint32{0, 1, 2},
		},
		{
			"no match trace",
			eth.MustNewHash("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			&pbcodec.TransactionTrace{
				Hash: toBytes(t, "a7cade838e7bff1545167702500184147c956875332e0d440d77e48e29762a59"),
				Input: toBytes(t, transferMethod+
					leftPad32b("aaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbbb")+
					leftPad32b("00")),
				Calls: []*pbcodec.Call{
					&pbcodec.Call{
						Index: 0,
					},
					&pbcodec.Call{
						Index: 1,
					},
					&pbcodec.Call{
						Index: 2,
					},
				},
			},
			false,
			nil,
		},
	}

	for _, test := range tests {

		t.Run(test.name, func(t *testing.T) {
			trxFilter := NewTrxHashFilterBytes(test.hash)

			matched, calls := trxFilter.Matches(test.trx, NewTrxFilterCache())
			if test.expectMatch {
				assert.True(t, matched, "Expected action trace to match trx filter but it did not")
			} else {
				assert.False(t, matched, "Expected action trace to not match trx filter but it did")
			}
			assert.Equal(t, test.expectCalls, calls)
		})
	}
}
