package main

import (
	"slices"
	"sort"

	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
)

func removeRevertedCallStorageChangesFromEthereumBlock(block *pbeth.Block) {
	if block == nil {
		return
	}

	removedOrdinals := collectRevertedCallStorageChangeOrdinals(block)
	shiftBlockOrdinals(block, removedOrdinals)

	for _, trace := range block.TransactionTraces {
		for _, call := range trace.Calls {
			if call.StateReverted && call.Index != 0 {
				call.StorageChanges = nil
			}
		}
	}
}

func collectRevertedCallStorageChangeOrdinals(block *pbeth.Block) []uint64 {
	if block == nil {
		return nil
	}

	var out []uint64
	for _, trace := range block.TransactionTraces {
		for _, call := range trace.Calls {
			if call.StateReverted && call.Index != 0 {
				for _, storageChange := range call.StorageChanges {
					if storageChange.Ordinal > 0 {
						out = append(out, storageChange.Ordinal)
					}
				}
			}
		}
	}

	if len(out) == 0 {
		return nil
	}

	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return slices.Compact(out)
}
