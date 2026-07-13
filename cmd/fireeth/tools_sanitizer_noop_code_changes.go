package main

import (
	"bytes"
	"slices"
	"sort"

	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
)

func removeNoopCodeChangesFromEthereumBlock(block *pbeth.Block) {
	if block == nil {
		return
	}

	removedOrdinals := collectNoopCodeChangeOrdinals(block)
	shiftBlockOrdinals(block, removedOrdinals)

	block.CodeChanges = filterNoopCodeChanges(block.CodeChanges)

	for _, systemCall := range block.SystemCalls {
		systemCall.CodeChanges = filterNoopCodeChanges(systemCall.CodeChanges)
	}

	for _, trace := range block.TransactionTraces {
		for _, call := range trace.Calls {
			call.CodeChanges = filterNoopCodeChanges(call.CodeChanges)
		}
	}
}

func collectNoopCodeChangeOrdinals(block *pbeth.Block) []uint64 {
	if block == nil {
		return nil
	}

	var out []uint64

	for _, codeChange := range block.CodeChanges {
		if codeChange.Ordinal > 0 && isNoopCodeChange(codeChange) {
			out = append(out, codeChange.Ordinal)
		}
	}

	for _, systemCall := range block.SystemCalls {
		for _, codeChange := range systemCall.CodeChanges {
			if codeChange.Ordinal > 0 && isNoopCodeChange(codeChange) {
				out = append(out, codeChange.Ordinal)
			}
		}
	}

	for _, trace := range block.TransactionTraces {
		for _, call := range trace.Calls {
			for _, codeChange := range call.CodeChanges {
				if codeChange.Ordinal > 0 && isNoopCodeChange(codeChange) {
					out = append(out, codeChange.Ordinal)
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

func isNoopCodeChange(c *pbeth.CodeChange) bool {
	return bytes.Equal(c.OldHash, c.NewHash)
}

func filterNoopCodeChanges(changes []*pbeth.CodeChange) []*pbeth.CodeChange {
	var out []*pbeth.CodeChange
	for _, c := range changes {
		if !isNoopCodeChange(c) {
			out = append(out, c)
		}
	}
	return out
}
