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

	pbcodec "github.com/streamingfast/sf-ethereum/pb/dfuse/ethereum/codec/v1"
	"github.com/stretchr/testify/assert"
)

func TestPassthrough(t *testing.T) {

	tests := []struct {
		name string
		trx  interface{}
	}{
		{
			"erc20",
			&pbcodec.Transaction{
				Input: toBytes(t, transferMethod+
					leftPad32b("aaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbbb")+
					leftPad32b("00")),
			},
		},
	}

	for _, test := range tests {

		t.Run(test.name, func(t *testing.T) {
			trxFilter := newPassthroughFilter()

			fc := NewTrxFilterCache()
			matched, _ := trxFilter.Matches(test.trx, fc)
			assert.True(t, matched, "Expected action trace to match passthrough filter but it did not")
		})
	}
}
