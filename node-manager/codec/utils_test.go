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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
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
