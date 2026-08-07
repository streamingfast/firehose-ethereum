package main

import (
	"slices"
	"sort"

	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
)

// removeWithdrawalsFromEthereumBlock removes all traces of beacon chain withdrawals from
// the block: the withdrawals list itself, the header's withdrawals root and the block-level
// balance changes caused by them. Ordinals of the remaining elements are shifted down so
// blocks produced with and without withdrawals support can be compared.
func removeWithdrawalsFromEthereumBlock(block *pbeth.Block) {
	if block == nil {
		return
	}

	removedOrdinals := collectRemovedWithdrawalOrdinals(block)

	block.Withdrawals = nil
	if block.Header != nil {
		block.Header.WithdrawalsRoot = nil
	}

	block.BalanceChanges = slices.DeleteFunc(block.BalanceChanges, isWithdrawalBalanceChange)
	for _, systemCall := range block.SystemCalls {
		systemCall.BalanceChanges = slices.DeleteFunc(systemCall.BalanceChanges, isWithdrawalBalanceChange)
	}
	for _, trace := range block.TransactionTraces {
		for _, call := range trace.Calls {
			call.BalanceChanges = slices.DeleteFunc(call.BalanceChanges, isWithdrawalBalanceChange)
		}
	}

	shiftBlockOrdinals(block, removedOrdinals)
}

func isWithdrawalBalanceChange(balanceChange *pbeth.BalanceChange) bool {
	return balanceChange.GetReason() == pbeth.BalanceChange_REASON_WITHDRAWAL
}

func collectRemovedWithdrawalOrdinals(block *pbeth.Block) []uint64 {
	var out []uint64
	collect := func(balanceChanges []*pbeth.BalanceChange) {
		for _, balanceChange := range balanceChanges {
			if isWithdrawalBalanceChange(balanceChange) && balanceChange.Ordinal > 0 {
				out = append(out, balanceChange.Ordinal)
			}
		}
	}

	collect(block.BalanceChanges)
	for _, systemCall := range block.SystemCalls {
		collect(systemCall.BalanceChanges)
	}
	for _, trace := range block.TransactionTraces {
		for _, call := range trace.Calls {
			collect(call.BalanceChanges)
		}
	}

	if len(out) == 0 {
		return nil
	}

	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return slices.Compact(out)
}
