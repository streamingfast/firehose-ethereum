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
				GasChanges: []*pbeth.GasChange{{OldValue: 10, NewValue: 5}},
			},
		},
		TransactionTraces: []*pbeth.TransactionTrace{
			{
				Calls: []*pbeth.Call{
					{
						GasChanges:     []*pbeth.GasChange{{OldValue: 100, NewValue: 50}},
						BalanceChanges: []*pbeth.BalanceChange{{Ordinal: 1}},
					},
					{
						GasChanges: []*pbeth.GasChange{{OldValue: 10, NewValue: 0}},
					},
				},
			},
		},
	}

	removeGasChangesFromEthereumBlock(block)

	require.Nil(t, block.SystemCalls[0].GasChanges)
	require.Nil(t, block.TransactionTraces[0].Calls[0].GasChanges)
	require.Nil(t, block.TransactionTraces[0].Calls[1].GasChanges)
	require.Len(t, block.TransactionTraces[0].Calls[0].BalanceChanges, 1)
}

func Test_removeGasChangesFromEthereumBlock_nil(t *testing.T) {
	require.NotPanics(t, func() {
		removeGasChangesFromEthereumBlock(nil)
	})
}
