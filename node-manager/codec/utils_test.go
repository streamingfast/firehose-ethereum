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
	"encoding/json"
	"fmt"
	"testing"

	"github.com/streamingfast/eth-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeHex(t *testing.T) {
	tests := []struct {
		in       string
		expected string
	}{
		{"0", "00"},
		{"00", "00"},
		{"000", "0000"},
		{"1", "01"},
		{"01", "01"},
		{"001", "0001"},
		{"0001", "0001"},
		{"00001", "000001"},

		{"0x", ""},
		{"0x0", "00"},
		{"0x00", "00"},
		{"0xff", "ff"},
		{"0xf", "0f"},
		{"0xF", "0f"},
		{"0xFF", "ff"},
		{"0xFa", "fa"},
		{"0xF0", "f0"},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			actual := SanitizeHex(test.in)
			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestUnmarshallEndBlockData(t *testing.T) {
	in := `{"finalizedBlockHash":"0x38b1c79e3acb45df1ac1fbd4d70e08c655874fc592c5c86342a29baf4f001769","finalizedBlockNum":"0x800","header":{"parentHash":"0x0000000000000000000000000000000000000000000000000000000000000000","sha3Uncles":"0x0000000000000000000000000000000000000000000000000000000000000000","miner":"0x0000000000000000000000000000000000000000","stateRoot":"0x0000000000000000000000000000000000000000000000000000000000000000","transactionsRoot":"0x0000000000000000000000000000000000000000000000000000000000000000","receiptsRoot":"0x0000000000000000000000000000000000000000000000000000000000000000","logsBloom":"0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000","difficulty":null,"number":null,"gasLimit":"0x0","gasUsed":"0x0","timestamp":"0x0","extraData":"0x","mixHash":"0x0000000000000000000000000000000000000000000000000000000000000000","nonce":"0x0000000000000000","baseFeePerGas":null,"hash":"0xc3bd2d00745c03048a5616146a96f5ff78e54efb9e5b04af208cdaff6f3830ee"},"totalDifficulty":"0x400","uncles":[]}`

	var endBlockData endBlockInfo
	require.NoError(t, json.Unmarshal([]byte(in), &endBlockData))

	assert.Equal(t, eth.MustNewHash("38b1c79e3acb45df1ac1fbd4d70e08c655874fc592c5c86342a29baf4f001769"), endBlockData.FinalizedBlockHash)
	assert.Equal(t, eth.Uint64(2048), endBlockData.FinalizedBlockNum)
}
