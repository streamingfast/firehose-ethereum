package main

import (
	"testing"

	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"github.com/stretchr/testify/require"
)

func Test_removeNoopCodeChangesFromEthereumBlock(t *testing.T) {
	hashA := []byte{0x01}
	hashB := []byte{0x02}
	hashC := []byte{0x03}

	block := &pbeth.Block{
		SystemCalls: []*pbeth.Call{
			{
				BeginOrdinal: 1,
				EndOrdinal:   10,
				CodeChanges: []*pbeth.CodeChange{
					{OldHash: hashA, NewHash: hashA, Ordinal: 3}, // NOOP
					{OldHash: hashA, NewHash: hashB, Ordinal: 7}, // not NOOP
				},
				StorageChanges: []*pbeth.StorageChange{{Ordinal: 5}},
			},
		},
		BalanceChanges: []*pbeth.BalanceChange{
			{Ordinal: 20},
		},
		CodeChanges: []*pbeth.CodeChange{
			{OldHash: hashC, NewHash: hashC, Ordinal: 15}, // NOOP
			{OldHash: hashC, NewHash: hashB, Ordinal: 18}, // not NOOP
		},
		TransactionTraces: []*pbeth.TransactionTrace{
			{
				BeginOrdinal: 11,
				EndOrdinal:   22,
				Receipt: &pbeth.TransactionReceipt{
					Logs: []*pbeth.Log{{Ordinal: 21}},
				},
				Calls: []*pbeth.Call{
					{
						BeginOrdinal: 12,
						EndOrdinal:   17,
						CodeChanges: []*pbeth.CodeChange{
							{OldHash: hashB, NewHash: hashB, Ordinal: 13}, // NOOP
							{OldHash: hashB, NewHash: hashC, Ordinal: 16}, // not NOOP
						},
						NonceChanges: []*pbeth.NonceChange{{Ordinal: 14}},
					},
				},
			},
		},
	}

	// Removed NOOP ordinals (sorted): [3, 13, 15]
	removeNoopCodeChangesFromEthereumBlock(block)

	// SystemCalls[0]: BeginOrdinal 1 → shiftOrdinal(1,[3,13,15])=1
	require.Equal(t, uint64(1), block.SystemCalls[0].BeginOrdinal)
	// EndOrdinal 10 → shiftOrdinal(10,[3,13,15])=9
	require.Equal(t, uint64(9), block.SystemCalls[0].EndOrdinal)
	// StorageChanges[0].Ordinal 5 → shiftOrdinal(5,[3,13,15])=4
	require.Equal(t, uint64(4), block.SystemCalls[0].StorageChanges[0].Ordinal)
	// NOOP (ordinal 3) removed; only non-NOOP entry remains
	require.Len(t, block.SystemCalls[0].CodeChanges, 1)
	// CodeChanges[0].Ordinal 7 → shiftOrdinal(7,[3,13,15])=6
	require.Equal(t, uint64(6), block.SystemCalls[0].CodeChanges[0].Ordinal)

	// TransactionTraces[0]: BeginOrdinal 11 → shiftOrdinal(11,[3,13,15])=10
	require.Equal(t, uint64(10), block.TransactionTraces[0].BeginOrdinal)
	// EndOrdinal 22 → shiftOrdinal(22,[3,13,15])=19
	require.Equal(t, uint64(19), block.TransactionTraces[0].EndOrdinal)
	// Receipt.Logs[0].Ordinal 21 → shiftOrdinal(21,[3,13,15])=18
	require.Equal(t, uint64(18), block.TransactionTraces[0].Receipt.Logs[0].Ordinal)

	// Calls[0]: BeginOrdinal 12 → shiftOrdinal(12,[3,13,15])=11
	require.Equal(t, uint64(11), block.TransactionTraces[0].Calls[0].BeginOrdinal)
	// EndOrdinal 17 → shiftOrdinal(17,[3,13,15])=14
	require.Equal(t, uint64(14), block.TransactionTraces[0].Calls[0].EndOrdinal)
	// NOOP (ordinal 13) removed; only non-NOOP entry remains
	require.Len(t, block.TransactionTraces[0].Calls[0].CodeChanges, 1)
	// CodeChanges[0].Ordinal 16 → shiftOrdinal(16,[3,13,15])=13
	require.Equal(t, uint64(13), block.TransactionTraces[0].Calls[0].CodeChanges[0].Ordinal)
	// NonceChanges[0].Ordinal 14 → shiftOrdinal(14,[3,13,15])=12
	require.Equal(t, uint64(12), block.TransactionTraces[0].Calls[0].NonceChanges[0].Ordinal)

	// BalanceChanges[0].Ordinal 20 → shiftOrdinal(20,[3,13,15])=17
	require.Equal(t, uint64(17), block.BalanceChanges[0].Ordinal)

	// block.CodeChanges: NOOP (ordinal 15) removed; only non-NOOP entry remains
	require.Len(t, block.CodeChanges, 1)
	// CodeChanges[0].Ordinal 18 → shiftOrdinal(18,[3,13,15])=15
	require.Equal(t, uint64(15), block.CodeChanges[0].Ordinal)
}

func Test_removeNoopCodeChangesFromEthereumBlock_nil(t *testing.T) {
	require.NotPanics(t, func() { removeNoopCodeChangesFromEthereumBlock(nil) })
}

func Test_removeNoopCodeChangesFromEthereumBlock_noNoop(t *testing.T) {
	hashA := []byte{0x01}
	hashB := []byte{0x02}

	block := &pbeth.Block{
		SystemCalls: []*pbeth.Call{
			{
				BeginOrdinal: 1,
				EndOrdinal:   5,
				CodeChanges: []*pbeth.CodeChange{
					{OldHash: hashA, NewHash: hashB, Ordinal: 3}, // not NOOP
				},
			},
		},
		TransactionTraces: []*pbeth.TransactionTrace{
			{
				BeginOrdinal: 10,
				EndOrdinal:   20,
				Calls: []*pbeth.Call{
					{
						BeginOrdinal: 11,
						EndOrdinal:   15,
						CodeChanges: []*pbeth.CodeChange{
							{OldHash: hashB, NewHash: hashA, Ordinal: 13}, // not NOOP
						},
					},
				},
			},
		},
	}

	removeNoopCodeChangesFromEthereumBlock(block)

	// No NOOPs removed, so no ordinals should shift
	require.Equal(t, uint64(1), block.SystemCalls[0].BeginOrdinal)
	require.Equal(t, uint64(5), block.SystemCalls[0].EndOrdinal)
	require.Len(t, block.SystemCalls[0].CodeChanges, 1)
	require.Equal(t, uint64(3), block.SystemCalls[0].CodeChanges[0].Ordinal)

	require.Equal(t, uint64(10), block.TransactionTraces[0].BeginOrdinal)
	require.Equal(t, uint64(20), block.TransactionTraces[0].EndOrdinal)
	require.Len(t, block.TransactionTraces[0].Calls[0].CodeChanges, 1)
	require.Equal(t, uint64(13), block.TransactionTraces[0].Calls[0].CodeChanges[0].Ordinal)
}

func Test_removeNoopCodeChangesFromEthereumBlock_zeroOrdinalNoopDoesNotShift(t *testing.T) {
	hashA := []byte{0x01}

	block := &pbeth.Block{
		TransactionTraces: []*pbeth.TransactionTrace{
			{
				BeginOrdinal: 10,
				EndOrdinal:   11,
				Calls: []*pbeth.Call{
					{
						BeginOrdinal: 10,
						EndOrdinal:   11,
						CodeChanges:  []*pbeth.CodeChange{{OldHash: hashA, NewHash: hashA, Ordinal: 0}},
						Logs:         []*pbeth.Log{{Ordinal: 11}},
					},
				},
			},
		},
	}

	removeNoopCodeChangesFromEthereumBlock(block)

	// Ordinal 0 is excluded from the shift set, so nothing should shift
	require.Equal(t, uint64(10), block.TransactionTraces[0].BeginOrdinal)
	require.Equal(t, uint64(11), block.TransactionTraces[0].EndOrdinal)
	require.Equal(t, uint64(10), block.TransactionTraces[0].Calls[0].BeginOrdinal)
	require.Equal(t, uint64(11), block.TransactionTraces[0].Calls[0].EndOrdinal)
	require.Equal(t, uint64(11), block.TransactionTraces[0].Calls[0].Logs[0].Ordinal)
	// The NOOP code change is still filtered out even though its ordinal is 0
	require.Nil(t, block.TransactionTraces[0].Calls[0].CodeChanges)
}
