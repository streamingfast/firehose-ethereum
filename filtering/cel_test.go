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
	"encoding/hex"
	"strings"
	"testing"

	"github.com/streamingfast/eth-go"
	pbeth "github.com/streamingfast/sf-ethereum/types/pb/sf/ethereum/type/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var listOf100Addresses = []string{
	"0xaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbeebbb", "0xaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbbc",
	"0xaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbcb", "0xaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbb1",
	"0xaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbdb", "0xaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbb2",
	"0xaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbeb", "0xaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbb3",
	"0xaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbfb", "0xaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbb4",
	"0xaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbba", "0xaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbb5",
	"0xaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbbd", "0xaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbb6",
	"0xaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbbe", "0xaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbb7",
	"0xaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbbf", "0xaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbb8",
	"0xaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbb8bbbbb0", "0xaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbb9",
	"0xaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb10", "0xaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb1b",
	"0xaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb20", "0xaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb2b",
	"0xaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb30", "0xaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb3b",
	"0xaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb40", "0xaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb4b",
	"0xbaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbcb", "0xbaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbb1",
	"0xbaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbdb", "0xbaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbb2",
	"0xbaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbeb", "0xbaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbb3",
	"0xbaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbfb", "0xbaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbb4",
	"0xbaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbba", "0xbaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbb5",
	"0xbaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbbd", "0xbaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbb6",
	"0xbaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbbe", "0xbaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbb7",
	"0xbaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbbf", "0xbaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbb8",
	"0xbaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbb8bbbbb0", "0xbaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbb9",
	"0xbaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb10", "0xbaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb1b",
	"0xbaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb20", "0xbaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb2b",
	"0xbaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb30", "0xbaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb3b",
	"0xbaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb40", "0xbaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb40",
	"0xcaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb41", "0xdaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb41",
	"0xcaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb42", "0xdaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb42",
	"0xcaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb43", "0xdaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb43",
	"0xcaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb44", "0xdaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb44",
	"0xcaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb45", "0xdaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb45",
	"0xcaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb46", "0xdaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb46",
	"0xcaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb47", "0xdaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb47",
	"0xcaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb48", "0xdaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb48",
	"0xcaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb49", "0xdaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb49",
	"0xcaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb50", "0xdaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb50",
	"0xcaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb51", "0xdaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb51",
	"0xcaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb52", "0xdaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb52",
	"0xcaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb53", "0xdaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb53",
	"0xcaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb54", "0xdaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb54",
	"0xcaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb55", "0xdaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb55",
	"0xcaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb56", "0xdaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb56",
	"0xcaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb57", "0xdaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb57",
	"0xcaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb58", "0xdaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb58",
	"0xcaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb59", "0xdaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb59",
	"0xcaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb60", "0xdaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbb60",
	"0xaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbb0", "0xaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbbb",
}

var listOf100AddressesCEL = "[\"" + strings.Join(listOf100Addresses, "\",\"") + "\"]"

var transferMethod = "a9059cbb"

func toBytes(t *testing.T, hexString string) []byte {
	b, err := hex.DecodeString(hexString)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func leftPad32b(in string) string {
	out := eth.CanonicalHex(in)
	remaining := 64 - (len(out) % 64)

	return strings.Repeat("0", remaining) + out
}

func TestCELActivation(t *testing.T) {

	tests := []struct {
		name          string
		code          string
		activation    *CallActivation
		expectedMatch bool
	}{
		{
			"to, empty input",
			`to == "0xaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbbb"`,
			&CallActivation{},
			false,
		},
		{
			"erc20_to, empty input",
			`erc20_to == "0xaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbbb"`,
			&CallActivation{},
			false,
		},
		{
			"to match trx",
			`erc20_to == "0xaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbbb"`,
			&CallActivation{
				Trx: &pbeth.Transaction{
					Input: toBytes(t, transferMethod+
						leftPad32b("aaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbbb")+
						leftPad32b("00")),
				},
			},
			true,
		},
		{
			"erc20_from match trx",
			`erc20_from == "0xaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbbb"`,
			&CallActivation{
				Trx: &pbeth.Transaction{
					From: toBytes(t, "aaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbbb"),
					Input: toBytes(t, transferMethod+
						leftPad32b("1111111111111111111111111111111111111111")+
						leftPad32b("00")),
				},
			},
			true,
		},
		{
			"from match trace",
			`from == "0xaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbbb"`,
			&CallActivation{
				Trace: &pbeth.TransactionTrace{
					From:  toBytes(t, "aaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbbb"),
					To:    toBytes(t, "cccccccccccccccccccddddddddddddddddddddd"),
					Input: toBytes(t, "12345678"),
				},
			},
			true,
		},
		{
			"from nomatch trace",
			`from == "0xeeeeeeeeeeeeeeeeeeefffffffffffffffffffff"`,
			&CallActivation{
				Trace: &pbeth.TransactionTrace{
					From:  toBytes(t, "aaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbbb"),
					To:    toBytes(t, "cccccccccccccccccccddddddddddddddddddddd"),
					Input: toBytes(t, "12345678"),
				},
			},
			false,
		},
	}

	for _, test := range tests {

		t.Run(test.name, func(t *testing.T) {
			celFilter, err := newCELFilter("test", test.code, []string{"false", ""}, false)
			require.NoError(t, err)
			matched := celFilter.match(test.activation)

			if test.expectedMatch {
				assert.True(t, matched, "Expected action trace to match CEL filter (%s) but it did not", test.code)
			} else {
				assert.False(t, matched, "Expected action trace to NOT match filter (%s) but it did", test.code)
			}
		})
	}
}
