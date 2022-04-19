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

package ct

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGasPrice_ToBigInt(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		expected *big.Int
	}{
		{"eth, no decimal", "1 ETH", bigInt(t, "1000000000000000000")},
		{"eth, zero decimal", "1. ETH", bigInt(t, "1000000000000000000")},
		{"eth, one decimal", "0.1 ETH", bigInt(t, "100000000000000000")},
		{"eth, two decimal", "0.12 ETH", bigInt(t, "120000000000000000")},
		{"eth, eighteen decimal", "0.000000000000000001 ETH", bigInt(t, "1")},
		{"eth, nineteen decimal", "0.0000000000000000012 ETH", bigInt(t, "1")},
		{"eth, mixed decimal", "8.1234567890123456789 ETH", bigInt(t, "8123456789012345678")},

		{"as hex", "0x123", bigInt(t, "291")},

		{"as decimal", "123", bigInt(t, "123")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := GasPrice(test.in).ToBigInt(t)
			assert.Equal(t, test.expected, actual)
		})
	}
}

func bigInt(t *testing.T, in string) *big.Int {
	out, worked := new(big.Int).SetString(in, 0)
	require.True(t, worked, "Input %q is not a invalid big.Int", in)

	return out
}
