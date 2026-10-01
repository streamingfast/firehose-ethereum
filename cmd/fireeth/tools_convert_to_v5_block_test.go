package main

import (
	"encoding/hex"
	"strings"
	"testing"

	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func Test_convertEthereumBlockToV5_ordinals(t *testing.T) {
	block := &pbeth.Block{
		Ver: 3,
		TransactionTraces: []*pbeth.TransactionTrace{
			{
				// Ordinal 4 was consumed by the root call start and left unused
				BeginOrdinal: 1,
				EndOrdinal:   16,
				Receipt:      &pbeth.TransactionReceipt{Logs: []*pbeth.Log{{Ordinal: 11}}},
				Calls: []*pbeth.Call{
					{
						Index:        1,
						BeginOrdinal: 0,
						EndOrdinal:   15,
						BalanceChanges: []*pbeth.BalanceChange{
							{Ordinal: 2, OldValue: bigInt(10), NewValue: bigInt(7)},
							{Ordinal: 14, OldValue: bigInt(7), NewValue: bigInt(7)},
						},
						NonceChanges: []*pbeth.NonceChange{
							{Ordinal: 3, OldValue: 1, NewValue: 2},
							{Ordinal: 5, OldValue: 2, NewValue: 2},
						},
						GasChanges: []*pbeth.GasChange{{Ordinal: 6, OldValue: 10, NewValue: 5}},
					},
					{
						Index:        2,
						ParentIndex:  1,
						BeginOrdinal: 7,
						EndOrdinal:   13,
						StorageChanges: []*pbeth.StorageChange{
							{Ordinal: 8, OldValue: word(1), NewValue: word(1)},
							{Ordinal: 9, OldValue: word(1), NewValue: word(2)},
						},
						AccountCreations: []*pbeth.AccountCreation{{Ordinal: 10}},
						Logs:             []*pbeth.Log{{Ordinal: 11}},
						CodeChanges:      []*pbeth.CodeChange{{Ordinal: 12, OldHash: word(1), NewHash: word(1)}},
					},
				},
			},
		},
		BalanceChanges: []*pbeth.BalanceChange{
			{Ordinal: 17, OldValue: bigInt(1), NewValue: bigInt(2)},
			{Ordinal: 18, OldValue: nil, NewValue: bigInt(0)},
		},
	}

	convertEthereumBlockToV5(block)

	assertProtoEqual(t, &pbeth.Block{
		Ver: 5,
		TransactionTraces: []*pbeth.TransactionTrace{
			{
				BeginOrdinal: 1,
				EndOrdinal:   10,
				Receipt:      &pbeth.TransactionReceipt{Logs: []*pbeth.Log{{Ordinal: 7}}},
				Calls: []*pbeth.Call{
					{
						Index:          1,
						BeginOrdinal:   4,
						EndOrdinal:     9,
						BalanceChanges: []*pbeth.BalanceChange{{Ordinal: 2, OldValue: bigInt(10), NewValue: bigInt(7)}},
						NonceChanges:   []*pbeth.NonceChange{{Ordinal: 3, OldValue: 1, NewValue: 2}},
					},
					{
						Index:          2,
						ParentIndex:    1,
						BeginOrdinal:   5,
						EndOrdinal:     8,
						StorageChanges: []*pbeth.StorageChange{{Ordinal: 6, OldValue: word(1), NewValue: word(2)}},
						Logs:           []*pbeth.Log{{Ordinal: 7}},
					},
				},
			},
		},
		BalanceChanges: []*pbeth.BalanceChange{{Ordinal: 11, OldValue: bigInt(1), NewValue: bigInt(2)}},
	}, block)

	converted := proto.Clone(block)
	convertEthereumBlockToV5(block)
	assertProtoEqual(t, converted, block)
}

func Test_setMissingCallOrdinals(t *testing.T) {
	tests := []struct {
		name     string
		trace    *pbeth.TransactionTrace
		expected *pbeth.TransactionTrace
	}{
		{
			name: "root call without unused ordinal starts right after the transaction",
			trace: &pbeth.TransactionTrace{
				BeginOrdinal: 1,
				EndOrdinal:   4,
				Calls: []*pbeth.Call{
					{Index: 1, EndOrdinal: 3, NonceChanges: []*pbeth.NonceChange{{Ordinal: 2}}},
				},
			},
			expected: &pbeth.TransactionTrace{
				BeginOrdinal: 1,
				EndOrdinal:   5,
				Calls: []*pbeth.Call{
					{Index: 1, BeginOrdinal: 2, EndOrdinal: 4, NonceChanges: []*pbeth.NonceChange{{Ordinal: 3}}},
				},
			},
		},
		{
			name: "unused ordinal after a child call started is not the root call start",
			trace: &pbeth.TransactionTrace{
				BeginOrdinal: 1,
				EndOrdinal:   7,
				Calls: []*pbeth.Call{
					{Index: 1, EndOrdinal: 6},
					{Index: 2, ParentIndex: 1, BeginOrdinal: 2, EndOrdinal: 3},
				},
			},
			expected: &pbeth.TransactionTrace{
				BeginOrdinal: 1,
				EndOrdinal:   8,
				Calls: []*pbeth.Call{
					{Index: 1, BeginOrdinal: 2, EndOrdinal: 7},
					{Index: 2, ParentIndex: 1, BeginOrdinal: 3, EndOrdinal: 4},
				},
			},
		},
		{
			name: "child calls start right before the first thing they recorded",
			trace: &pbeth.TransactionTrace{
				BeginOrdinal: 1,
				EndOrdinal:   9,
				Calls: []*pbeth.Call{
					{Index: 1, BeginOrdinal: 2, EndOrdinal: 8, StorageChanges: []*pbeth.StorageChange{{Ordinal: 3}}},
					{Index: 2, ParentIndex: 1, EndOrdinal: 6},
					{Index: 3, ParentIndex: 2, EndOrdinal: 5, Logs: []*pbeth.Log{{Ordinal: 4}}},
					{Index: 4, ParentIndex: 1, EndOrdinal: 7},
				},
			},
			expected: &pbeth.TransactionTrace{
				BeginOrdinal: 1,
				EndOrdinal:   12,
				Calls: []*pbeth.Call{
					{Index: 1, BeginOrdinal: 2, EndOrdinal: 11, StorageChanges: []*pbeth.StorageChange{{Ordinal: 3}}},
					{Index: 2, ParentIndex: 1, BeginOrdinal: 4, EndOrdinal: 8},
					{Index: 3, ParentIndex: 2, BeginOrdinal: 5, EndOrdinal: 7, Logs: []*pbeth.Log{{Ordinal: 6}}},
					{Index: 4, ParentIndex: 1, BeginOrdinal: 9, EndOrdinal: 10},
				},
			},
		},
		{
			name: "genesis root call without begin and end ordinals",
			trace: &pbeth.TransactionTrace{
				BeginOrdinal: 1,
				EndOrdinal:   4,
				Calls: []*pbeth.Call{
					{Index: 1, BalanceChanges: []*pbeth.BalanceChange{{Ordinal: 2}, {Ordinal: 3}}},
				},
			},
			expected: &pbeth.TransactionTrace{
				BeginOrdinal: 1,
				EndOrdinal:   6,
				Calls: []*pbeth.Call{
					{Index: 1, BeginOrdinal: 2, EndOrdinal: 5, BalanceChanges: []*pbeth.BalanceChange{{Ordinal: 3}, {Ordinal: 4}}},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			block := &pbeth.Block{TransactionTraces: []*pbeth.TransactionTrace{test.trace}}
			setMissingCallOrdinals(block, test.trace)
			assertProtoEqual(t, test.expected, test.trace)
		})
	}
}

func Test_convertEthereumBlockToV5_delegateCallerOfVersion2(t *testing.T) {
	block := &pbeth.Block{
		Ver: 2,
		TransactionTraces: []*pbeth.TransactionTrace{
			{
				BeginOrdinal: 1,
				EndOrdinal:   6,
				Calls: []*pbeth.Call{
					{Index: 1, BeginOrdinal: 2, EndOrdinal: 5, Address: []byte{0xaa}, Caller: []byte{0x01}},
					{Index: 2, ParentIndex: 1, BeginOrdinal: 3, EndOrdinal: 4, CallType: pbeth.CallType_DELEGATE, Address: []byte{0xbb}, Caller: []byte{0x01}},
				},
			},
		},
	}

	convertEthereumBlockToV5(block)

	assert.Equal(t, int32(5), block.Ver)
	assert.Equal(t, []byte{0xaa}, block.TransactionTraces[0].Calls[1].Caller)
}

func Test_keepStorageSlotKeccakPreimages(t *testing.T) {
	hash := func(b byte) []byte { return word(b) }
	key := func(b []byte) string { return hex.EncodeToString(b) }
	plusOne := func(b []byte) []byte {
		out := append([]byte{}, b...)
		out[31]++
		return out
	}

	mapping := hash(0xa1)       // written directly
	array := hash(0xb1)         // written at array + 1
	inner := hash(0xc1)         // appears in the preimage of `mapping`
	innerOffset := hash(0xd1)   // inner + 1 appears in the preimage of `inner`
	stringKey := hash(0xe1)     // appears as the last 32 bytes of an unaligned preimage
	readOnly := hash(0xf1)      // never leads to a written slot
	tooLarge := hash(0x71)      // written directly, preimage above 256 bytes
	emptyInput := hash(0x51)    // written directly, preimage recorded as "."
	otherCallOnly := hash(0x41) // written by a call, computed by another

	calls := []*pbeth.Call{
		{
			Index: 1,
			StorageChanges: []*pbeth.StorageChange{
				{Key: mapping},
				{Key: plusOne(array)},
				{Key: tooLarge},
				{Key: emptyInput},
				{Key: otherCallOnly},
			},
			KeccakPreimages: map[string]string{
				key(mapping):    key(inner) + key(word(0x02)),
				key(array):      key(word(0x03)),
				key(readOnly):   key(word(0x04)),
				key(tooLarge):   strings.Repeat("00", 257),
				key(emptyInput): ".",
				"zz":            "00",
			},
		},
		{
			Index:       2,
			ParentIndex: 1,
			KeccakPreimages: map[string]string{
				key(inner):         key(word(0x05)) + key(plusOne(innerOffset)),
				key(innerOffset):   "abcdef" + key(stringKey),
				key(stringKey):     key(word(0x06)),
				key(otherCallOnly): key(word(0x07)),
			},
		},
		{
			Index:           3,
			ParentIndex:     1,
			KeccakPreimages: map[string]string{key(readOnly): key(word(0x04))},
		},
	}

	keepStorageSlotKeccakPreimages(calls)

	assert.Equal(t, map[string]string{
		key(mapping):    key(inner) + key(word(0x02)),
		key(array):      key(word(0x03)),
		key(emptyInput): "",
	}, calls[0].KeccakPreimages)
	assert.Equal(t, map[string]string{
		key(inner):         key(word(0x05)) + key(plusOne(innerOffset)),
		key(innerOffset):   "abcdef" + key(stringKey),
		key(stringKey):     key(word(0x06)),
		key(otherCallOnly): key(word(0x07)),
	}, calls[1].KeccakPreimages)
	assert.Nil(t, calls[2].KeccakPreimages)
}

func Test_splitSystemCalls(t *testing.T) {
	rootFirst := []*pbeth.Call{
		{Index: 1, BeginOrdinal: 1, EndOrdinal: 4},
		{Index: 2, ParentIndex: 1, Depth: 1, BeginOrdinal: 2, EndOrdinal: 3},
		{Index: 1, BeginOrdinal: 5, EndOrdinal: 6},
	}
	groups := splitSystemCalls(rootFirst)
	require.Len(t, groups, 2)
	assert.Equal(t, []*pbeth.Call{rootFirst[0], rootFirst[1]}, groups[0])
	assert.Equal(t, []*pbeth.Call{rootFirst[2]}, groups[1])

	rootLast := []*pbeth.Call{
		{Index: 1, BeginOrdinal: 1, EndOrdinal: 2},
		{Index: 2, ParentIndex: 1, Depth: 1, BeginOrdinal: 4, EndOrdinal: 5},
		{Index: 1, BeginOrdinal: 3, EndOrdinal: 6},
	}
	groups = splitSystemCalls(rootLast)
	require.Len(t, groups, 2)
	assert.Equal(t, []*pbeth.Call{rootLast[0]}, groups[0])
	assert.Equal(t, []*pbeth.Call{rootLast[2], rootLast[1]}, groups[1])

	assert.Empty(t, splitSystemCalls(nil))
}

func Test_convertEthereumBlockToV5_version3KnownIssues(t *testing.T) {
	block := &pbeth.Block{
		Ver:    3,
		Header: &pbeth.BlockHeader{},
		SystemCalls: []*pbeth.Call{
			{Index: 1, BeginOrdinal: 1, EndOrdinal: 3, StorageChanges: []*pbeth.StorageChange{{Ordinal: 2, OldValue: word(1), NewValue: word(2)}}},
		},
		TransactionTraces: []*pbeth.TransactionTrace{
			{
				// Ordinals restarted at the first transaction, 2 is the unused root call start
				BeginOrdinal: 1,
				EndOrdinal:   6,
				Input:        []byte{0xc0, 0xde},
				Calls: []*pbeth.Call{
					{Index: 1, CallType: pbeth.CallType_CREATE, EndOrdinal: 5, ReturnData: []byte{0x01}},
					{
						Index: 2, ParentIndex: 1, Depth: 1, CallType: pbeth.CallType_CALL, BeginOrdinal: 3, EndOrdinal: 4,
						StateReverted: true,
						Logs:          []*pbeth.Log{{Topics: [][]byte{{}}}, {Topics: [][]byte{word(1)}}},
					},
				},
			},
		},
		BalanceChanges: []*pbeth.BalanceChange{{Ordinal: 10, OldValue: bigInt(1), NewValue: bigInt(2)}},
	}

	convertEthereumBlockToV5(block)

	assertProtoEqual(t, &pbeth.Block{
		Ver:    5,
		Header: &pbeth.BlockHeader{},
		SystemCalls: []*pbeth.Call{
			{Index: 1, BeginOrdinal: 1, EndOrdinal: 3, StorageChanges: []*pbeth.StorageChange{{Ordinal: 2, OldValue: word(1), NewValue: word(2)}}},
		},
		TransactionTraces: []*pbeth.TransactionTrace{
			{
				BeginOrdinal: 4,
				EndOrdinal:   9,
				Input:        []byte{0xc0, 0xde},
				ReturnData:   []byte{0x01},
				Calls: []*pbeth.Call{
					{Index: 1, CallType: pbeth.CallType_CREATE, BeginOrdinal: 5, EndOrdinal: 8, Input: []byte{0xc0, 0xde}, ReturnData: []byte{0x01}},
					{
						Index: 2, ParentIndex: 1, Depth: 1, CallType: pbeth.CallType_CALL, BeginOrdinal: 6, EndOrdinal: 7,
						StateReverted: true,
						Logs:          []*pbeth.Log{{}, {Index: 1, Topics: [][]byte{word(1)}}},
					},
				},
			},
		},
		BalanceChanges: []*pbeth.BalanceChange{{Ordinal: 10, OldValue: bigInt(1), NewValue: bigInt(2)}},
	}, block)
}

func Test_convertEthereumBlockToV5_systemCallInsideTransaction(t *testing.T) {
	// Ordinals 2 and 3 were consumed by the two calls when they started and left unused
	block := &pbeth.Block{
		Ver:    3,
		Header: &pbeth.BlockHeader{},
		SystemCalls: []*pbeth.Call{
			{Index: 2, ParentIndex: 1, Depth: 1, BeginOrdinal: 6, EndOrdinal: 7},
			{Index: 1, BeginOrdinal: 4, EndOrdinal: 9, GasChanges: []*pbeth.GasChange{{Ordinal: 5}}, StorageChanges: []*pbeth.StorageChange{{Ordinal: 8, OldValue: word(1), NewValue: word(2)}}},
		},
		TransactionTraces: []*pbeth.TransactionTrace{
			{
				BeginOrdinal: 1,
				EndOrdinal:   13,
				Calls: []*pbeth.Call{
					{Index: 1, EndOrdinal: 12},
					{Index: 2, ParentIndex: 1, Depth: 1, EndOrdinal: 11, StorageChanges: []*pbeth.StorageChange{{Ordinal: 10, OldValue: word(1), NewValue: word(2)}}},
				},
			},
		},
	}

	convertEthereumBlockToV5(block)

	assertProtoEqual(t, &pbeth.Block{
		Ver:    5,
		Header: &pbeth.BlockHeader{},
		SystemCalls: []*pbeth.Call{
			{Index: 2, ParentIndex: 1, Depth: 1, BeginOrdinal: 5, EndOrdinal: 6},
			{Index: 1, BeginOrdinal: 4, EndOrdinal: 8, StorageChanges: []*pbeth.StorageChange{{Ordinal: 7, OldValue: word(1), NewValue: word(2)}}},
		},
		TransactionTraces: []*pbeth.TransactionTrace{
			{
				BeginOrdinal: 1,
				EndOrdinal:   12,
				Calls: []*pbeth.Call{
					{Index: 1, BeginOrdinal: 2, EndOrdinal: 11},
					{Index: 2, ParentIndex: 1, Depth: 1, BeginOrdinal: 3, EndOrdinal: 10, StorageChanges: []*pbeth.StorageChange{{Ordinal: 9, OldValue: word(1), NewValue: word(2)}}},
				},
			},
		},
	}, block)
}

func Test_setLogIndexes(t *testing.T) {
	trace := &pbeth.TransactionTrace{
		Receipt: &pbeth.TransactionReceipt{Logs: []*pbeth.Log{{Ordinal: 3}, {Ordinal: 9}}},
		Calls: []*pbeth.Call{
			{Index: 1, Logs: []*pbeth.Log{{Ordinal: 3}, {Ordinal: 9, Index: 1}}},
			{Index: 2, ParentIndex: 1, StateReverted: true, Logs: []*pbeth.Log{{Ordinal: 5}, {Ordinal: 6}}},
		},
	}

	setLogIndexes(trace)

	assertProtoEqual(t, &pbeth.TransactionTrace{
		Receipt: &pbeth.TransactionReceipt{Logs: []*pbeth.Log{{Ordinal: 3}, {Ordinal: 9, Index: 3}}},
		Calls: []*pbeth.Call{
			{Index: 1, Logs: []*pbeth.Log{{Ordinal: 3}, {Ordinal: 9, Index: 3}}},
			{Index: 2, ParentIndex: 1, StateReverted: true, Logs: []*pbeth.Log{{Ordinal: 5, Index: 1}, {Ordinal: 6, Index: 2}}},
		},
	}, trace)
}

func Test_isMissingWithdrawals(t *testing.T) {
	withdrawals := []*pbeth.Withdrawal{{Index: 1}}
	deposit := []*pbeth.TransactionTrace{{Type: pbeth.TransactionTrace_TRX_TYPE_OPTIMISM_DEPOSIT}}

	tests := []struct {
		name     string
		block    *pbeth.Block
		expected bool
	}{
		{"header without root", &pbeth.Block{Header: &pbeth.BlockHeader{}}, false},
		{"root of an empty list", &pbeth.Block{Header: &pbeth.BlockHeader{WithdrawalsRoot: emptyWithdrawalsRoot}}, false},
		{"root without withdrawals", &pbeth.Block{Header: &pbeth.BlockHeader{WithdrawalsRoot: word(1)}}, true},
		{"root with withdrawals", &pbeth.Block{Header: &pbeth.BlockHeader{WithdrawalsRoot: word(1)}, Withdrawals: withdrawals}, false},
		{"OP Stack block", &pbeth.Block{Header: &pbeth.BlockHeader{WithdrawalsRoot: word(1)}, TransactionTraces: deposit}, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, isMissingWithdrawals(test.block))
		})
	}
}

func Test_moveTransactionOrdinalsAfterSystemCalls_skipsPragueBlocks(t *testing.T) {
	block := &pbeth.Block{
		Header:            &pbeth.BlockHeader{RequestsHash: word(1)},
		SystemCalls:       []*pbeth.Call{{Index: 1, BeginOrdinal: 1, EndOrdinal: 2}},
		TransactionTraces: []*pbeth.TransactionTrace{{BeginOrdinal: 1, EndOrdinal: 2}},
	}

	moveTransactionOrdinalsAfterSystemCalls(block)

	assert.Equal(t, uint64(1), block.TransactionTraces[0].BeginOrdinal)
}

func Test_limitCallData(t *testing.T) {
	call := func(depth uint32, begin, end uint64, input, returnData int) *pbeth.Call {
		return &pbeth.Call{Depth: depth, BeginOrdinal: begin, EndOrdinal: end, Input: make([]byte, input), ReturnData: make([]byte, returnData)}
	}
	sizes := func(calls []*pbeth.Call) (out [][2]int) {
		for _, c := range calls {
			out = append(out, [2]int{len(c.Input), len(c.ReturnData)})
		}
		return out
	}

	t.Run("truncates the calls after the limit is passed", func(t *testing.T) {
		calls := []*pbeth.Call{
			call(0, 1, 100, 64, 64),
			call(1, 2, 3, 6, 3),
			call(1, 4, 5, 6, 3),
			call(1, 6, 7, 2, 0),
			call(1, 8, 9, 6, 3),
		}

		limitCallData(calls, 10, 5)

		assert.Equal(t, [][2]int{{64, 64}, {6, 3}, {6, 3}, {2, 0}, {4, 0}}, sizes(calls))
		assert.False(t, calls[3].InputTruncated || calls[3].ReturnDataTruncated)
		assert.True(t, calls[4].InputTruncated && calls[4].ReturnDataTruncated)
	})

	t.Run("inputs count in start order and return data in end order", func(t *testing.T) {
		calls := []*pbeth.Call{
			call(0, 1, 100, 64, 64),
			call(1, 2, 5, 8, 8),
			call(2, 3, 4, 8, 8),
		}

		limitCallData(calls, 7, 7)

		assert.Equal(t, [][2]int{{64, 64}, {8, 0}, {4, 8}}, sizes(calls))
	})

	t.Run("halves the limits until the block fits", func(t *testing.T) {
		block := &pbeth.Block{TransactionTraces: []*pbeth.TransactionTrace{{Calls: []*pbeth.Call{
			call(0, 1, 100, 64, 64),
			call(1, 2, 3, 1024, 1024),
			call(1, 4, 5, 1024, 1024),
			call(1, 6, 7, 1024, 1024),
		}}}}

		limitBlockCallData(block, 1)

		assert.Equal(t, [][2]int{{64, 64}, {1024, 1024}, {4, 0}, {4, 0}}, sizes(block.TransactionTraces[0].Calls))
	})
}

func bigInt(value byte) *pbeth.BigInt {
	return &pbeth.BigInt{Bytes: []byte{value}}
}

// word returns a 32 bytes value ending with `last`.
func word(last byte) []byte {
	out := make([]byte, 32)
	out[0] = 0x10
	out[31] = last
	return out
}

func assertProtoEqual(t *testing.T, expected, actual proto.Message) {
	t.Helper()

	if !proto.Equal(expected, actual) {
		assert.Equal(t, expected.(interface{ String() string }).String(), actual.(interface{ String() string }).String())
		t.FailNow()
	}
}
