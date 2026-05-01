package main

import (
	"bytes"
	"fmt"
	"os"
	"sort"

	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	firecore "github.com/streamingfast/firehose-core"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
)

var ignoreOrdinals = os.Getenv("FIREETH_TOOLS_COMPARE_IGNORE_ORDINALS") == "true"
var ignoreOPStackSystemCallsOrder = os.Getenv("FIREETH_TOOLS_COMPARE_IGNORE_OPSTACK_SYSTEM_CALLS_ORDER") == "true"
var ignoreGas = os.Getenv("FIREETH_TOOLS_COMPARE_IGNORE_GAS") == "true"
var ignoreNoopCodeChanges = os.Getenv("FIREETH_TOOLS_COMPARE_IGNORE_NOOP_CODE_CHANGES") == "true"
var ignoreKeccak = os.Getenv("FIREETH_TOOLS_COMPARE_IGNORE_KECCAK") == "true"
var ignoreRevertedCallStorageChanges = os.Getenv("FIREETH_TOOLS_COMPARE_IGNORE_REVERTED_CALL_STORAGE_CHANGES") == "true"

func SanitizeEthereumBlockForCompare(block *pbbstream.Block) *pbbstream.Block {
	untypedEthBlock, err := block.Payload.UnmarshalNew()
	if err != nil {
		panic(fmt.Errorf("unexpected block message %s, unable to unmarshal to proto.Message: %w", block.Payload.TypeUrl, err))
	}

	ethBlock, ok := untypedEthBlock.(*pbeth.Block)
	if !ok {
		panic(fmt.Errorf("unexpected block message %s, unable to cast to pbeth.Block", block.Payload.TypeUrl))
	}

	// The TotalDifficulty field has been removed in newer versions of the Geth,
	// so it's impossible to have it good in all cases, forcing it to nil.
	if ethBlock.Header.TotalDifficulty != nil {
		ethBlock.Header.TotalDifficulty = nil
	}
	var hasLogBloom bool
	for _, byte := range ethBlock.Header.LogsBloom {
		if byte != '0' {
			hasLogBloom = true
			break
		}
	}
	if !hasLogBloom {
		ethBlock.Header.LogsBloom = nil
	}

	if ignoreOrdinals {
		for _, c := range ethBlock.BalanceChanges {
			c.Ordinal = 0
		}
		for _, c := range ethBlock.CodeChanges {
			c.Ordinal = 0
		}
	}
	if ignoreGas {
		removeGasChangesFromEthereumBlock(ethBlock)
		ethBlock.Ver = 5 // with removedgas, we are at ver=5
	}
	if ignoreNoopCodeChanges {
		removeNoopCodeChangesFromEthereumBlock(ethBlock)
	}
	if ignoreRevertedCallStorageChanges {
		removeRevertedCallStorageChangesFromEthereumBlock(ethBlock)
	}

	if ignoreOPStackSystemCallsOrder {
		// reorder system calls by using all the fields as sorting keys
		sort.Slice(ethBlock.SystemCalls, func(i, j int) bool {
			si, sj := ethBlock.SystemCalls[i], ethBlock.SystemCalls[j]
			if cmp := bytes.Compare(si.Address, sj.Address); cmp != 0 {
				return cmp < 0
			}
			if cmp := bytes.Compare(si.Caller, sj.Caller); cmp != 0 {
				return cmp < 0
			}
			if cmp := bytes.Compare(si.AddressDelegatesTo, sj.AddressDelegatesTo); cmp != 0 {
				return cmp < 0
			}
			if si.GasConsumed != sj.GasConsumed {
				return si.GasConsumed < sj.GasConsumed
			}
			return si.GasLimit < sj.GasLimit
		})
		for _, call := range ethBlock.SystemCalls {
			call.BeginOrdinal = 0
			call.EndOrdinal = 0
			call.Index = 1
			for _, c := range call.StorageChanges {
				c.Ordinal = 0
			}
			for _, c := range call.CodeChanges {
				c.Ordinal = 0
			}
			for _, c := range call.NonceChanges {
				c.Ordinal = 0
			}
			for _, log := range call.Logs {
				log.Ordinal = 0
			}
		}
	}
	for _, tx := range ethBlock.TransactionTraces {
		if ignoreOrdinals {
			tx.BeginOrdinal = 0
			tx.EndOrdinal = 0
		}
		var hasLogBloom bool
		for _, byte := range tx.Receipt.LogsBloom {
			if byte != '0' {
				hasLogBloom = true
				break
			}
		}
		if !hasLogBloom {
			tx.Receipt.LogsBloom = nil
		}
		if ignoreOrdinals {
			for _, l := range tx.Receipt.Logs {
				l.Ordinal = 0
			}
		}

		tx.Receipt.LogsBloom = nil
		for _, call := range tx.Calls {
			if ignoreOrdinals {
				call.BeginOrdinal = 0
				call.EndOrdinal = 0
				for _, l := range call.Logs {
					l.Ordinal = 0
				}
				for _, c := range call.BalanceChanges {
					c.Ordinal = 0
				}
				for _, c := range call.GasChanges {
					c.Ordinal = 0
				}
				for _, c := range call.NonceChanges {
					c.Ordinal = 0
				}
				for _, c := range call.CodeChanges {
					c.Ordinal = 0
				}
				for _, c := range call.StorageChanges {
					c.Ordinal = 0
				}
			}

			if ignoreKeccak {
				call.KeccakPreimages = nil
			}
			if call.FailureReason != "" {
				call.FailureReason = "<error replaced for comparison>"
			}
		}
	}

	out, err := firecore.EncodeBlock(firecore.BlockEnveloppe{
		Block:  ethBlock,
		LIBNum: block.LibNum,
	})
	if err != nil {
		panic(fmt.Errorf("unable to encode block: %w", err))
	}

	return out
}
