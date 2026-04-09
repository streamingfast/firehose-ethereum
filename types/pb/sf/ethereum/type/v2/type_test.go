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

package pbeth

import (
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBigInt_JSON(t *testing.T) {
	value := NewBigInt(123456)

	actualJSON, err := json.Marshal(value)
	require.NoError(t, err)

	assert.JSONEq(t, `"01e240"`, string(actualJSON))

	actual := &BigInt{}
	err = json.Unmarshal(actualJSON, actual)
	require.NoError(t, err)

	assert.Equal(t, value, actual)
}

// B is a shortcut for (must) hex.DecodeString
var B = func(s string) []byte {
	out, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}

	return out
}

// H is a shortcut for hex.EncodeToString
var H = hex.EncodeToString
