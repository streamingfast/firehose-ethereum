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

// The sanitizer below is used both by 'fireeth tools compare-blocks' and by the reader node
// running in test mode ('--reader-node-test-mode'), hence the chain-wide 'FIREETH_COMPARE_'
// prefix of those environment variables.
var ignoreOrdinals = boolEnv("FIREETH_COMPARE_IGNORE_ORDINALS", "FIREETH_TOOLS_COMPARE_IGNORE_ORDINALS")
var ignoreSystemCallsOrder = boolEnv("FIREETH_COMPARE_IGNORE_SYSTEM_CALLS_ORDER", "FIREETH_TOOLS_COMPARE_IGNORE_OPSTACK_SYSTEM_CALLS_ORDER")
var ignoreGas = boolEnv("FIREETH_COMPARE_IGNORE_GAS", "FIREETH_TOOLS_COMPARE_IGNORE_GAS")
var ignoreNoopCodeChanges = boolEnv("FIREETH_COMPARE_IGNORE_NOOP_CODE_CHANGES", "FIREETH_TOOLS_COMPARE_IGNORE_NOOP_CODE_CHANGES")
var ignoreKeccak = boolEnv("FIREETH_COMPARE_IGNORE_KECCAK", "FIREETH_TOOLS_COMPARE_IGNORE_KECCAK")
var ignoreRevertedCallStorageChanges = boolEnv("FIREETH_COMPARE_IGNORE_REVERTED_CALL_STORAGE_CHANGES", "FIREETH_TOOLS_COMPARE_IGNORE_REVERTED_CALL_STORAGE_CHANGES")
var ignoreSystemCallGasLimit = boolEnv("FIREETH_COMPARE_IGNORE_SYSTEM_CALL_GAS_LIMIT", "FIREETH_TOOLS_COMPARE_IGNORE_OPSTACK_SYSTEM_GAS_LIMIT")
var ignoreWithdrawals = boolEnv("FIREETH_COMPARE_IGNORE_WITHDRAWALS")

// boolEnv returns true if any of the given environment variables is set to "true", the
// first name is the current one, the others are deprecated aliases kept for compatibility.
func boolEnv(names ...string) bool {
	for _, name := range names {
		if os.Getenv(name) == "true" {
			return true
		}
	}

	return false
}

// systemCaller is the well-known system address (0xffff...fffe) used as the caller of system
// calls (EIP-4788 beacon roots, EIP-2935 block hashes, EIP-7002/7251 requests, OP-Stack system
// transactions). It is not chain-specific.
var systemCaller = []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xfe}

const (
	// systemCallGasLimitGeth is the gas limit geth gives to system calls.
	systemCallGasLimitGeth = 30000000

	// systemCallGasLimitRevm is the gas limit recent revm versions give to system calls, on every
	// chain and every fork: 30_000_000 + SSTORE_SET_BYTES(64) * CPSB_GLAMSTERDAM(1530) *
	// SYSTEM_MAX_SSTORES_PER_CALL(16), see revm-handler's SYSTEM_CALL_GAS_LIMIT (EIP-8037).
	systemCallGasLimitRevm = 31566720
)

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
	for _, b := range ethBlock.Header.LogsBloom {
		if b != 0 {
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
	}
	if ignoreNoopCodeChanges {
		removeNoopCodeChangesFromEthereumBlock(ethBlock)
	}
	if ignoreRevertedCallStorageChanges {
		removeRevertedCallStorageChangesFromEthereumBlock(ethBlock)
	}
	if ignoreWithdrawals {
		removeWithdrawalsFromEthereumBlock(ethBlock)
	}

	if ignoreSystemCallGasLimit {
		for _, call := range ethBlock.SystemCalls {
			normalizeSystemCallGasLimit(call)
		}
		for _, tx := range ethBlock.TransactionTraces {
			for _, call := range tx.Calls {
				normalizeSystemCallGasLimit(call)
			}
		}
	}

	if ignoreSystemCallsOrder {
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
		if ignoreOrdinals {
			for _, l := range tx.Receipt.Logs {
				l.Ordinal = 0
			}
		}

		// Receipt logs blooms are always removed for comparison, some producers
		// don't fill them at all.
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

	ethBlock.Ver = 5 // we don't want errors on version differences

	out, err := firecore.EncodeBlock(firecore.BlockEnveloppe{
		Block:  ethBlock,
		LIBNum: block.LibNum,
	})
	if err != nil {
		panic(fmt.Errorf("unable to encode block: %w", err))
	}

	return out
}

func normalizeSystemCallGasLimit(call *pbeth.Call) {
	if call.GasLimit == systemCallGasLimitGeth && bytes.Equal(call.Caller, systemCaller) {
		call.GasLimit = systemCallGasLimitRevm
	}
}
