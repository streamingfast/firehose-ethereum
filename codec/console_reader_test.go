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

package codec

import (
	"math/big"
	"testing"

	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValueParsing(t *testing.T) {
	testValue := "deff"
	expectedValue := &pbeth.BigInt{
		Bytes: big.NewInt(int64(57087)).Bytes(),
	}
	value := pbeth.BigIntFromBytes(FromHex(testValue, "TESTING value"))
	require.Equal(t, expectedValue, value)
}

func Test_computeProofOfWorkLIBNum(t *testing.T) {
	type args struct {
		blockNum                uint64
		firstStreamableBlockNum uint64
	}

	tests := []struct {
		name string
		args args
		want uint64
	}{
		{"block is before first streamable block", args{0, 200}, 200},
		{"block is equal to first streamable block", args{200, 200}, 200},
		{"block is after first streamable block", args{201, 200}, 200},
		{"block is direct +200 blocks from first streamable block", args{400, 200}, 200},
		{"block is direct +201 blocks from first streamable block", args{401, 200}, 201},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, computeProofOfWorkLIBNum(tt.args.blockNum, tt.args.firstStreamableBlockNum))
		})
	}
}

func Test_computeProofOfStakeLIBNum(t *testing.T) {
	type args struct {
		current         uint64
		finalized       uint64
		firstStreamable uint64
	}

	tests := []struct {
		name string
		args args
		want uint64
	}{
		{"current is below first streamable, finalized block below current", args{current: 10, finalized: 0, firstStreamable: 200}, 200},
		{"current is equal to first streamable, finalized block below current", args{current: 200, finalized: 0, firstStreamable: 200}, 200},

		{"current is below first streamable, finalized block above current", args{current: 10, finalized: 400, firstStreamable: 200}, 200},
		{"current is equal to first streamable, finalized block above current", args{current: 200, finalized: 400, firstStreamable: 200}, 200},

		{"current is below first streamable, finalized block below first streamable", args{current: 10, finalized: 100, firstStreamable: 200}, 200},
		{"current is equal to first streamable, finalized block below first streamable", args{current: 200, finalized: 100, firstStreamable: 200}, 200},

		{"current is below first streamable, finalized block above first streamable", args{current: 10, finalized: 400, firstStreamable: 200}, 200},
		{"current is equal to first streamable, finalized block above first streamable", args{current: 200, finalized: 400, firstStreamable: 200}, 200},

		{"current is below finalized, above first streamable", args{current: 10, finalized: 400}, 10},
		{"current is equal to finalized, above first streamable", args{current: 400, finalized: 400}, 400},
		{"current is above finalized, above first streamable", args{current: 410, finalized: 400}, 400},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, computeProofOfStakeLIBNum(tt.args.current, tt.args.finalized, tt.args.firstStreamable))
		})
	}
}
