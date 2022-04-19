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

	"github.com/streamingfast/jsonpb"
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

func TestBigInt_JSONPB(t *testing.T) {
	value := NewBigInt(123456)

	marshaler := &jsonpb.Marshaler{}
	actualJSON, err := marshaler.MarshalToString(value)
	require.NoError(t, err)

	assert.JSONEq(t, `"01e240"`, actualJSON)

	actual := &BigInt{}
	err = jsonpb.UnmarshalString(actualJSON, actual)
	require.NoError(t, err)

	assert.Equal(t, value, actual)
}

func TestPopulateStateReverted(t *testing.T) {
	trxTrace := func(calls ...*Call) *TransactionTrace {
		return &TransactionTrace{
			Hash:  B("ff"),
			Calls: calls,
		}
	}

	call := func(index, parent uint32, status string) *Call {
		call := &Call{
			Index:       index,
			ParentIndex: parent,
		}

		if status == "failed" {
			call.StatusFailed = true
		}

		if status != "failed" && status != "succeeded" {
			require.Fail(t, "only failed or succeeded status are permitted")
		}

		return call
	}

	tests := []struct {
		name     string
		in       *TransactionTrace
		expected map[uint32]bool
	}{
		{
			"single-call-success",
			trxTrace(
				call(1, 0, "succeeded"),
			),
			map[uint32]bool{
				1: false,
			},
		},
		{
			"single-call-failed",
			trxTrace(
				call(1, 0, "failed"),
			),
			map[uint32]bool{
				1: true,
			},
		},
		{
			"single-child-success",
			trxTrace(
				call(1, 0, "succeeded"),
				call(2, 1, "succeeded"),
			),
			map[uint32]bool{
				1: false,
				2: false,
			},
		},
		{
			"single-child-success-child-failed",
			trxTrace(
				call(1, 0, "succeeded"),
				call(2, 1, "failed"),
			),
			map[uint32]bool{
				1: false,
				2: true,
			},
		},
		{
			"single-child-success-parent-failed",
			trxTrace(
				call(1, 0, "failed"),
				call(2, 1, "succeeded"),
			),
			map[uint32]bool{
				1: true,
				2: true,
			},
		},
		{
			"multi-child-all-success",
			trxTrace(
				call(1, 0, "succeeded"),
				call(2, 1, "succeeded"),
				call(3, 1, "succeeded"),
				call(4, 1, "succeeded"),
			),
			map[uint32]bool{
				1: false,
				2: false,
				3: false,
				4: false,
			},
		},
		{
			"multi-child-all-success-middle-child-failed",
			trxTrace(
				call(1, 0, "succeeded"),
				call(2, 1, "succeeded"),
				call(3, 1, "succeeded"),
				call(4, 1, "succeeded"),
			),
			map[uint32]bool{
				1: false,
				2: false,
				3: false,
				4: false,
			},
		},
		{
			"multi-child-nested-all-success",
			trxTrace(
				call(1, 0, "succeeded"),
				call(2, 1, "succeeded"),
				call(3, 2, "succeeded"),
				call(4, 3, "succeeded"),
				call(5, 2, "succeeded"),
				call(6, 1, "succeeded"),
				call(7, 1, "succeeded"),
			),
			map[uint32]bool{
				1: false,
				2: false,
				3: false,
				4: false,
				5: false,
				6: false,
				7: false,
			},
		},
		{
			"multi-child-nested-only-root-failed",
			trxTrace(
				call(1, 0, "failed"),
				call(2, 1, "succeeded"),
				call(3, 2, "succeeded"),
				call(4, 3, "succeeded"),
				call(5, 2, "succeeded"),
				call(6, 1, "succeeded"),
				call(7, 1, "succeeded"),
			),
			map[uint32]bool{
				1: true,
				2: true,
				3: true,
				4: true,
				5: true,
				6: true,
				7: true,
			},
		},
		{
			"multi-child-nested-parent-level1-failed",
			trxTrace(
				call(1, 0, "succeeded"),
				call(2, 1, "failed"),
				call(3, 2, "succeeded"),
				call(4, 3, "succeeded"),
				call(5, 2, "succeeded"),
				call(6, 2, "succeeded"),
				call(7, 2, "succeeded"),
				call(8, 7, "succeeded"),
				call(9, 1, "succeeded"),
			),
			map[uint32]bool{
				1: false,
				2: true,
				3: true,
				4: true,
				5: true,
				6: true,
				7: true,
				8: true,
				9: false,
			},
		},
		{
			"multi-child-nested-parent-level2-no-child",
			trxTrace(
				call(1, 0, "succeeded"),
				call(2, 1, "succeeded"),
				call(3, 2, "succeeded"),
				call(4, 3, "succeeded"),
				call(5, 2, "failed"),
				call(6, 2, "failed"),
				call(7, 2, "succeeded"),
				call(8, 7, "succeeded"),
				call(9, 1, "succeeded"),
			),
			map[uint32]bool{
				1: false,
				2: false,
				3: false,
				4: false,
				5: true,
				6: true,
				7: false,
				8: false,
				9: false,
			},
		},
		{
			"multi-child-nested-parent-level2-with-child-with-following-sibling",
			trxTrace(
				call(1, 0, "succeeded"),
				call(2, 1, "succeeded"),
				call(3, 2, "failed"),
				call(4, 3, "succeeded"),
				call(5, 2, "succeeded"),
				call(6, 2, "succeeded"),
				call(7, 2, "succeeded"),
				call(8, 7, "succeeded"),
				call(9, 1, "succeeded"),
			),
			map[uint32]bool{
				1: false,
				2: false,
				3: true,
				4: true,
				5: false,
				6: false,
				7: false,
				8: false,
				9: false,
			},
		},
		{
			"multi-child-nested-parent-level2-with-child-no-following-sibling",
			trxTrace(
				call(1, 0, "succeeded"),
				call(2, 1, "succeeded"),
				call(3, 2, "succeeded"),
				call(4, 3, "succeeded"),
				call(5, 2, "succeeded"),
				call(6, 2, "succeeded"),
				call(7, 2, "failed"),
				call(8, 7, "succeeded"),
				call(9, 1, "succeeded"),
			),
			map[uint32]bool{
				1: false,
				2: false,
				3: false,
				4: false,
				5: false,
				6: false,
				7: true,
				8: true,
				9: false,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.in.PopulateStateReverted()

			for _, call := range test.in.Calls {
				expected, exists := test.expected[call.Index]

				assert.Equal(t, true, exists, "Call %d not in expected map", call.Index)
				assert.Equal(t, expected, call.StateReverted, "Call %d state reverted mismatch", call.Index)
			}
		})
	}
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
