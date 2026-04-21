package main

import (
	"testing"

	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"github.com/stretchr/testify/require"
)

func Test_removeGasChangesFromEthereumBlock(t *testing.T) {
	block := &pbeth.Block{
		SystemCalls: []*pbeth.Call{
			{
				BeginOrdinal: 1,
				EndOrdinal:   7,
				GasChanges: []*pbeth.GasChange{
					{OldValue: 10, NewValue: 7, Ordinal: 2},
					{OldValue: 7, NewValue: 5, Ordinal: 4},
				},
				StorageChanges: []*pbeth.StorageChange{{Ordinal: 6}},
			},
		},
		BalanceChanges: []*pbeth.BalanceChange{
			{Ordinal: 26},
		},
		CodeChanges: []*pbeth.CodeChange{
			{Ordinal: 27},
		},
		TransactionTraces: []*pbeth.TransactionTrace{
			{
				BeginOrdinal: 10,
				EndOrdinal:   28,
				Receipt: &pbeth.TransactionReceipt{
					Logs: []*pbeth.Log{{Ordinal: 15}},
				},
				Calls: []*pbeth.Call{
					{
						BeginOrdinal:   11,
						EndOrdinal:     14,
						GasChanges:     []*pbeth.GasChange{{OldValue: 100, NewValue: 50, Ordinal: 0}},
						BalanceChanges: []*pbeth.BalanceChange{{Ordinal: 13}},
						NonceChanges:   []*pbeth.NonceChange{{Ordinal: 12}},
					},
					{
						BeginOrdinal: 16,
						EndOrdinal:   24,
						GasChanges: []*pbeth.GasChange{
							{OldValue: 20, NewValue: 18, Ordinal: 17},
							{OldValue: 18, NewValue: 15, Ordinal: 18},
						},
						StorageChanges:   []*pbeth.StorageChange{{Ordinal: 21}},
						Logs:             []*pbeth.Log{{Ordinal: 22}},
						CodeChanges:      []*pbeth.CodeChange{{Ordinal: 20}},
						AccountCreations: []*pbeth.AccountCreation{{Ordinal: 23}},
					},
				},
			},
			{
				BeginOrdinal: 30,
				EndOrdinal:   33,
				Calls: []*pbeth.Call{
					{
						BeginOrdinal: 30,
						EndOrdinal:   33,
						Logs:         []*pbeth.Log{{Ordinal: 32}},
					},
				},
			},
		},
	}

	removeGasChangesFromEthereumBlock(block)

	require.Nil(t, block.SystemCalls[0].GasChanges)
	require.Equal(t, uint64(1), block.SystemCalls[0].BeginOrdinal)
	require.Equal(t, uint64(5), block.SystemCalls[0].EndOrdinal)
	require.Equal(t, uint64(4), block.SystemCalls[0].StorageChanges[0].Ordinal)

	require.Equal(t, uint64(8), block.TransactionTraces[0].BeginOrdinal)
	require.Equal(t, uint64(24), block.TransactionTraces[0].EndOrdinal)
	require.Equal(t, uint64(13), block.TransactionTraces[0].Receipt.Logs[0].Ordinal)

	require.Nil(t, block.TransactionTraces[0].Calls[0].GasChanges)
	require.Equal(t, uint64(9), block.TransactionTraces[0].Calls[0].BeginOrdinal)
	require.Equal(t, uint64(12), block.TransactionTraces[0].Calls[0].EndOrdinal)
	require.Equal(t, uint64(10), block.TransactionTraces[0].Calls[0].NonceChanges[0].Ordinal)
	require.Equal(t, uint64(11), block.TransactionTraces[0].Calls[0].BalanceChanges[0].Ordinal)

	require.Nil(t, block.TransactionTraces[0].Calls[1].GasChanges)
	require.Equal(t, uint64(14), block.TransactionTraces[0].Calls[1].BeginOrdinal)
	require.Equal(t, uint64(20), block.TransactionTraces[0].Calls[1].EndOrdinal)
	require.Equal(t, uint64(17), block.TransactionTraces[0].Calls[1].StorageChanges[0].Ordinal)
	require.Equal(t, uint64(18), block.TransactionTraces[0].Calls[1].Logs[0].Ordinal)
	require.Equal(t, uint64(16), block.TransactionTraces[0].Calls[1].CodeChanges[0].Ordinal)
	require.Equal(t, uint64(19), block.TransactionTraces[0].Calls[1].AccountCreations[0].Ordinal)

	require.Equal(t, uint64(26), block.TransactionTraces[1].BeginOrdinal)
	require.Equal(t, uint64(29), block.TransactionTraces[1].EndOrdinal)
	require.Equal(t, uint64(26), block.TransactionTraces[1].Calls[0].BeginOrdinal)
	require.Equal(t, uint64(29), block.TransactionTraces[1].Calls[0].EndOrdinal)
	require.Equal(t, uint64(28), block.TransactionTraces[1].Calls[0].Logs[0].Ordinal)

	require.Equal(t, uint64(22), block.BalanceChanges[0].Ordinal)
	require.Equal(t, uint64(23), block.CodeChanges[0].Ordinal)
	require.Len(t, block.TransactionTraces[0].Calls[0].BalanceChanges, 1)
}

func Test_removeGasChangesFromEthereumBlock_nil(t *testing.T) {
	require.NotPanics(t, func() {
		removeGasChangesFromEthereumBlock(nil)
	})
}

func Test_removeGasChangesFromEthereumBlock_zeroOrdinalGasChangeDoesNotShift(t *testing.T) {
	block := &pbeth.Block{
		TransactionTraces: []*pbeth.TransactionTrace{
			{
				BeginOrdinal: 10,
				EndOrdinal:   11,
				Calls: []*pbeth.Call{
					{
						BeginOrdinal: 10,
						EndOrdinal:   11,
						GasChanges:   []*pbeth.GasChange{{Ordinal: 0}},
						Logs:         []*pbeth.Log{{Ordinal: 11}},
					},
				},
			},
		},
	}

	removeGasChangesFromEthereumBlock(block)

	require.Equal(t, uint64(10), block.TransactionTraces[0].BeginOrdinal)
	require.Equal(t, uint64(11), block.TransactionTraces[0].EndOrdinal)
	require.Equal(t, uint64(10), block.TransactionTraces[0].Calls[0].BeginOrdinal)
	require.Equal(t, uint64(11), block.TransactionTraces[0].Calls[0].EndOrdinal)
	require.Equal(t, uint64(11), block.TransactionTraces[0].Calls[0].Logs[0].Ordinal)
	require.Nil(t, block.TransactionTraces[0].Calls[0].GasChanges)
}

func Test_collectRemovedGasChangeOrdinals_dedupAndSorted(t *testing.T) {
	block := &pbeth.Block{
		SystemCalls: []*pbeth.Call{
			{
				GasChanges: []*pbeth.GasChange{
					{Ordinal: 9},
					{Ordinal: 0},
				},
			},
		},
		TransactionTraces: []*pbeth.TransactionTrace{
			{
				Calls: []*pbeth.Call{
					{
						GasChanges: []*pbeth.GasChange{
							{Ordinal: 10},
							{Ordinal: 9},
						},
					},
				},
			},
		},
	}

	require.Equal(t, []uint64{9, 10}, collectRemovedGasChangeOrdinals(block))
}
