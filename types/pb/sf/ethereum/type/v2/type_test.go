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
	"google.golang.org/protobuf/proto"
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

func boolPtr(v bool) *bool { return &v }

// TestBoolOptional covers the optional bool states (true / false / nil)
// BoolOptional.State is *bool, nil means the field was never set (absent on the wire)
func TestBoolOptional(t *testing.T) {
	tests := []struct {
		name        string
		msg         *BoolOptional
		expectedGet bool
		expectNil   bool
	}{
		{name: "true", msg: &BoolOptional{State: boolPtr(true)}, expectedGet: true, expectNil: false},
		{name: "false", msg: &BoolOptional{State: boolPtr(false)}, expectedGet: false, expectNil: false},
		{name: "nil", msg: &BoolOptional{State: nil}, expectedGet: false, expectNil: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedGet, tt.msg.GetState())
			assert.Equal(t, tt.expectNil, tt.msg.State == nil)

			data, err := proto.Marshal(tt.msg)
			require.NoError(t, err)

			got := &BoolOptional{}
			require.NoError(t, proto.Unmarshal(data, got))

			assert.Equal(t, tt.msg.State, got.State, "state must survive proto round-trip")
		})
	}
}

// TestBoolRequired covers the state required bool (true / false)
// BoolRequired.State is bool, false is the zero value and is indistinguishable from absent on the wire
func TestBoolRequired(t *testing.T) {
	tests := []struct {
		name        string
		msg         *BoolRequired
		expectedGet bool
	}{
		{name: "true", msg: &BoolRequired{State: true}, expectedGet: true},
		{name: "false", msg: &BoolRequired{State: false}, expectedGet: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedGet, tt.msg.GetState())

			data, err := proto.Marshal(tt.msg)
			require.NoError(t, err)

			got := &BoolRequired{}
			require.NoError(t, proto.Unmarshal(data, got))

			assert.Equal(t, tt.msg.State, got.State, "state must survive proto round-trip")
		})
	}
}

// TestBoolOptional_NilVsFalseAreDistinct is the critical test: proves that nil (unknown) and
// false (no code ran) are distinct values that survive a proto round-trip
func TestBoolOptional_NilVsFalseAreDistinct(t *testing.T) {
	nilData, err := proto.Marshal(&BoolOptional{State: nil})
	require.NoError(t, err)

	falseData, err := proto.Marshal(&BoolOptional{State: boolPtr(false)})
	require.NoError(t, err)

	gotNil := &BoolOptional{}
	require.NoError(t, proto.Unmarshal(nilData, gotNil))

	gotFalse := &BoolOptional{}
	require.NoError(t, proto.Unmarshal(falseData, gotFalse))

	assert.Nil(t, gotNil.State, "nil must remain nil after round-trip")
	assert.NotNil(t, gotFalse.State, "false must remain non-nil after round-trip")
	assert.Equal(t, false, *gotFalse.State)
}
