package codec

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/streamingfast/eth-go"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"google.golang.org/protobuf/proto"
)

var polygonSystemAddress = eth.MustNewAddress("0xffffFFFfFFffffffffffffffFfFFFfffFFFfFFfE")
var polygonNeverRevertedTopic = eth.MustNewBytes("0x4dfe1bbbcf077ddc3e01291eea2d5c70c2b422b415d95645b9adcfd678cb1d63")
var polygonFeeSystemAddress = eth.MustNewAddress("0x0000000000000000000000000000000000001010")
var polygonStateReceiverAddress = eth.MustNewAddress("0x0000000000000000000000000000000000001001")
var polygonValidatorContract = eth.MustNewAddress("0x0000000000000000000000000000000000001000")
var nullAddress = eth.MustNewAddress("0x0000000000000000000000000000000000000000")
var bigIntZero = pbeth.BigIntFromBytes(nil)

type normalizationFeatures struct {
	CombinePolygonSystemTransactions       bool
	ReorderTransactionsAndRenumberOrdinals bool
	UpgradeBlockV2ToV3                     bool
}

func normalizeInPlace(block *pbeth.Block, features *normalizationFeatures, firstTransactionOrdinal uint64) {
	for _, trx := range block.TransactionTraces {
		populateStateReverted(trx) // this needs to run first
	}

	var systemTransactionHashes hashes
	if features.CombinePolygonSystemTransactions {
		block.TransactionTraces, systemTransactionHashes = CombinePolygonSystemTransactions(block.TransactionTraces, block.Number, block.Hash)
	}

	if features.ReorderTransactionsAndRenumberOrdinals {
		reorderTransactionsAndRenumberOrdinals(block, firstTransactionOrdinal)
	}

	// We reconstruct the state reverted value per call, for each transaction traces. We also
	// normalize signature curve points since we were not setting to be alwasy 32 bytes long and
	// sometimes, it would have been only 31 bytes long.
	for _, trx := range block.TransactionTraces {
		populateTrxStatus(trx)

		if len(trx.R) > 0 && len(trx.R) != 32 {
			trx.R = NormalizeSignaturePoint(trx.R)
		}

		if len(trx.S) > 0 && len(trx.S) != 32 {
			trx.S = NormalizeSignaturePoint(trx.S)
		}
	}

	// We leverage StateReverted field inside the `PopulateLogBlockIndices`
	// and as such, it must be invoked after the `PopulateStateReverted` has
	// been executed.
	if err := populateLogBlockIndices(block, systemTransactionHashes); err != nil {
		panic(fmt.Errorf("normalizing log block indices: %w", err))
	}

	if features.UpgradeBlockV2ToV3 {
		upgradeBlockV2ToV3(block)
	}
}

func upgradeBlockV2ToV3(block *pbeth.Block) {
	if block.Ver == 2 {
		for _, trx := range block.TransactionTraces {
			headParents := make(map[uint32]uint32)
			for _, call := range trx.Calls {
				if call.CallType == pbeth.CallType_DELEGATE {
					idx := call.ParentIndex
					for {
						parent := callAtIndex(idx, trx.Calls)
						if parent == nil {
							panic(fmt.Sprintf("normalize_in_place: cannot find call parent of call %d on trx %s", call.Index, eth.Bytes(trx.Hash).Pretty()))
						}
						if parent.CallType == pbeth.CallType_DELEGATE {
							idx = parent.ParentIndex
							if headParent, ok := headParents[parent.ParentIndex]; ok {
								idx = headParent // skip to head parent
							}
							continue
						}
						headParents[call.Index] = idx
						call.Caller = parent.Address
						break
					}
				}
			}
		}
		block.Ver = 3
	}
}

func reorderTransactionsAndRenumberOrdinals(block *pbeth.Block, firstTransactionOrdinal uint64) {
	sort.Slice(block.TransactionTraces, func(i, j int) bool {
		return block.TransactionTraces[i].Index < block.TransactionTraces[j].Index // FIXME currently this is not a good value, the index is always the order in which it was received
	})

	baseline := firstTransactionOrdinal
	for _, trx := range block.TransactionTraces {
		trx.BeginOrdinal += baseline
		for _, call := range trx.Calls {
			if call.BeginOrdinal != 0 {
				call.BeginOrdinal += baseline // consistent with a known small bug: root call has beginOrdinal set to 0
			}
			call.EndOrdinal += baseline
			for _, log := range call.Logs {
				log.Ordinal += baseline
			}
			for _, act := range call.AccountCreations {
				act.Ordinal += baseline
			}
			for _, ch := range call.BalanceChanges {
				ch.Ordinal += baseline
			}
			for _, ch := range call.GasChanges {
				ch.Ordinal += baseline
			}
			for _, ch := range call.NonceChanges {
				ch.Ordinal += baseline
			}
			for _, ch := range call.StorageChanges {
				ch.Ordinal += baseline
			}
			for _, ch := range call.CodeChanges {
				ch.Ordinal += baseline
			}

		}
		for _, log := range trx.Receipt.Logs {
			log.Ordinal += baseline
		}
		trx.EndOrdinal += baseline
		baseline = trx.EndOrdinal
	}

	for _, ch := range block.BalanceChanges {
		if ch.Ordinal >= firstTransactionOrdinal {
			ch.Ordinal += baseline
		}
	}
	for _, ch := range block.CodeChanges {
		if ch.Ordinal >= firstTransactionOrdinal {
			ch.Ordinal += baseline
		}
	}

}

type hashes [][]byte

func (h hashes) Contains(in []byte) bool {
	for _, hash := range h {
		if bytes.Equal(hash, in) {
			return true
		}
	}
	return false
}

// CombinePolygonSystemTransactions will identify transactions that are "system transactions" and merge them into a single transaction with a predictive name, like the `bor` client does.
// It reorders the calls and logs to match expected output from RPC API.
func CombinePolygonSystemTransactions(traces []*pbeth.TransactionTrace, blockNum uint64, blockHash []byte) (out []*pbeth.TransactionTrace, systemTransactionHashes hashes) {

	var systemTransactionsToMerge []*pbeth.TransactionTrace
	var unmergeableSystemTransactions []*pbeth.TransactionTrace
	normalTransactions := make([]*pbeth.TransactionTrace, 0, len(traces))

	var highestTrxIndex uint32
	for _, trace := range traces {
		if bytes.Equal(trace.From, polygonSystemAddress) {
			if bytes.Equal(trace.To, polygonStateReceiverAddress) {
				systemTransactionsToMerge = append(systemTransactionsToMerge, trace)
				continue
			}
			if bytes.Equal(trace.To, polygonValidatorContract) {
				unmergeableSystemTransactions = append(unmergeableSystemTransactions, trace)
				continue
			}
			// no other know case for polygon
		}
		if trace.Index > highestTrxIndex {
			highestTrxIndex = trace.Index
		}
		normalTransactions = append(normalTransactions, trace)
	}

	out = normalTransactions
	if systemTransactionsToMerge == nil && unmergeableSystemTransactions == nil {
		return
	}

	if systemTransactionsToMerge != nil {
		var allCalls []*pbeth.Call
		var allLogs []*pbeth.Log
		var beginOrdinal uint64
		var seenFirstBeginOrdinal bool

		var seenFirstCallOrdinal bool
		var lowestCallBeginOrdinal uint64
		var highestCallEndOrdinal uint64

		var endOrdinal uint64
		var callIdxOffset = uint32(1) // initial offset for all calls because of artificial top level call

		for _, trace := range systemTransactionsToMerge {
			var trxLogs []*pbeth.Log
			if !seenFirstBeginOrdinal || trace.BeginOrdinal < beginOrdinal {
				beginOrdinal = trace.BeginOrdinal
				seenFirstBeginOrdinal = true
			}

			if trace.EndOrdinal > endOrdinal {
				endOrdinal = trace.EndOrdinal
			}
			highestCallIndex := callIdxOffset
			for _, call := range trace.Calls {
				if !seenFirstCallOrdinal || call.BeginOrdinal < lowestCallBeginOrdinal {
					lowestCallBeginOrdinal = call.BeginOrdinal
					seenFirstCallOrdinal = true
				}
				if call.EndOrdinal > highestCallEndOrdinal {
					highestCallEndOrdinal = call.EndOrdinal
				}

				call.Index += callIdxOffset

				// all top level calls must be children of the very first (artificial) call.
				call.Depth += 1
				if call.ParentIndex == 0 {
					call.ParentIndex = 1
				} else {
					call.ParentIndex += callIdxOffset
				}
				if call.Index > highestCallIndex {
					highestCallIndex = call.Index
				}
				allCalls = append(allCalls, call)
				// the receipt.logs on these transactions is not populated before
				for _, log := range call.Logs {
					if !call.StateReverted || isPolygonException(log) {
						trxLogs = append(trxLogs, log)
					}
				}
			}
			callIdxOffset = highestCallIndex

			sort.Slice(trxLogs, func(i, j int) bool {
				return trxLogs[i].BlockIndex < trxLogs[j].BlockIndex
			})
			allLogs = append(allLogs, trxLogs...)
		}
		artificialTopLevelCall := &pbeth.Call{
			Index:        1,
			ParentIndex:  0,
			Depth:        0,
			CallType:     pbeth.CallType_CALL,
			GasLimit:     0,
			GasConsumed:  0,
			Caller:       nullAddress,
			Address:      nullAddress,
			Value:        bigIntZero,
			Input:        nil,
			GasChanges:   nil,
			BeginOrdinal: lowestCallBeginOrdinal,
			EndOrdinal:   highestCallEndOrdinal,
		}
		allCalls = append([]*pbeth.Call{artificialTopLevelCall}, allCalls...)

		mergedHash := computePolygonHash(blockNum, blockHash)
		mergedSystemTrx := &pbeth.TransactionTrace{
			Hash:         mergedHash,
			From:         nullAddress,
			To:           nullAddress,
			Nonce:        0,
			GasPrice:     bigIntZero,
			GasLimit:     0,
			Value:        bigIntZero,
			Index:        highestTrxIndex + 1,
			Input:        nil,
			GasUsed:      0,
			Type:         pbeth.TransactionTrace_TRX_TYPE_LEGACY,
			BeginOrdinal: beginOrdinal,
			EndOrdinal:   endOrdinal,
			Calls:        allCalls,
			Status:       pbeth.TransactionTraceStatus_SUCCEEDED,
			Receipt: &pbeth.TransactionReceipt{
				Logs:      allLogs,
				LogsBloom: computeLogsBloom(allLogs),
				// CumulativeGasUsed // Reported as empty from the API. does not impact much because it is the last transaction in the block, this is reset every block.
				// StateRoot // Deprecated EIP 658
			},
		}
		systemTransactionHashes = append(systemTransactionHashes, mergedHash)
		out = append(out, mergedSystemTrx)
		highestTrxIndex++
	}
	for _, tx := range unmergeableSystemTransactions {
		tx.Index = highestTrxIndex + 1
		systemTransactionHashes = append(systemTransactionHashes, tx.Hash)
		out = append(out, tx)
		highestTrxIndex++
	}

	return
}

func NormalizeSignaturePoint(value []byte) []byte {
	if len(value) == 0 {
		return value
	}

	if len(value) < 32 {
		offset := 32 - len(value)

		out := make([]byte, 32)
		copy(out[offset:32], value)

		return out
	}

	return value[0:32]
}

// populateLogBlockIndices fixes the `TransactionReceipt.Logs[].BlockIndex`
// that is not properly populated by our deep mind instrumentation.
func populateLogBlockIndices(block *pbeth.Block, systemTransactionHashes hashes) error {
	// numbering receipts logs
	receiptLogBlockIndex := uint32(0)
	for _, trace := range block.TransactionTraces {
		for _, log := range trace.Receipt.Logs {
			log.BlockIndex = receiptLogBlockIndex
			receiptLogBlockIndex++
		}
	}

	// numbering call logs
	if block.Ver < 2 { // version 1 compatibility (outcome is imperfect)
		callLogBlockIndex := uint32(0)
		for _, trace := range block.TransactionTraces {
			for _, call := range trace.Calls {
				for _, log := range call.Logs {
					if call.StateReverted && !isPolygonException(log) {
						log.BlockIndex = 0
					} else {
						log.BlockIndex = callLogBlockIndex
						callLogBlockIndex++
					}
				}
			}
		}

		return nil
	}
	var callLogsToNumber []*pbeth.Log
	var systemTransactionLogs []*pbeth.Log
	for _, trace := range block.TransactionTraces {
		if systemTransactionHashes.Contains(trace.Hash) { // this system transaction must not have its logs reordered, we omit them for now
			systemTransactionLogs = append(systemTransactionLogs, trace.Receipt.Logs...)
			continue
		}
		for _, call := range trace.Calls {
			for _, log := range call.Logs {
				if call.StateReverted && !isPolygonException(log) {
					log.BlockIndex = 0
				} else {
					callLogsToNumber = append(callLogsToNumber, log)
				}
			}
		}
	}

	sort.Slice(callLogsToNumber, func(i, j int) bool { return callLogsToNumber[i].Ordinal < callLogsToNumber[j].Ordinal })
	// append the systemTransaction logs that were omitted
	callLogsToNumber = append(callLogsToNumber, systemTransactionLogs...)

	// also make a map of those logs
	blockIndexToTraceLog := make(map[uint32]*pbeth.Log)

	for i := 0; i < len(callLogsToNumber); i++ {
		log := callLogsToNumber[i]
		log.BlockIndex = uint32(i)
		if len(log.Topics) == 1 && len(log.Topics[0]) == 0 {
			log.Topics = nil
		}
		if _, ok := blockIndexToTraceLog[log.BlockIndex]; ok {
			return fmt.Errorf("duplicate blockIndex in tweak function")
		}
		blockIndexToTraceLog[log.BlockIndex] = log
	}

	// append Ordinal and Index to the receipt log
	var receiptLogCount int
	for _, trace := range block.TransactionTraces {
		for _, log := range trace.Receipt.Logs {
			receiptLogCount++
			traceLog, ok := blockIndexToTraceLog[log.BlockIndex]
			if !ok {
				return fmt.Errorf("missing tracelog at blockIndex in tweak function")
			}
			log.Ordinal = traceLog.Ordinal
			log.Index = traceLog.Index
			if !proto.Equal(log, traceLog) {
				// Will not error, worse case it fails and we end up with empty strings
				actualLog, _ := json.Marshal(log)
				remappedLog, _ := json.Marshal(traceLog)

				receiptLogCount := 0
				for _, trace := range block.TransactionTraces {
					receiptLogCount += len(trace.Receipt.Logs)
				}

				return fmt.Errorf("error in tweak function for transaction %q (%d receipt logs, %d re-mapped logs): log %s proto not equal re-mapped log %s",
					eth.Hex(trace.Hash),
					receiptLogCount,
					len(blockIndexToTraceLog),
					string(actualLog),
					string(remappedLog),
				)
			}
		}
	}
	if receiptLogCount != len(blockIndexToTraceLog) {
		return fmt.Errorf("error incorrect number of receipt logs in tweak function: %d, expecting %d", receiptLogCount, len(blockIndexToTraceLog))
	}
	return nil
}

func populateTrxStatus(trace *pbeth.TransactionTrace) {
	// transaction trace Status must be populatged according to simple rule: if call 0 fails, transaction fails.
	if trace.Status == pbeth.TransactionTraceStatus_UNKNOWN && len(trace.Calls) >= 1 {
		call := trace.Calls[0]
		switch {
		case call.StatusFailed && call.StatusReverted:
			trace.Status = pbeth.TransactionTraceStatus_REVERTED
		case call.StatusFailed:
			trace.Status = pbeth.TransactionTraceStatus_FAILED
		default:
			trace.Status = pbeth.TransactionTraceStatus_SUCCEEDED
		}
	}
}

func populateStateReverted(trace *pbeth.TransactionTrace) {
	// Calls are ordered by execution index. So the algo is quite simple.
	// We loop through the flat calls, at each call, if the parent is present
	// and reverted, the current call is reverted. Otherwise, if the current call
	// is failed, the state is reverted. In all other cases, we simply continue
	// our iteration loop.
	//
	// This works because we see the parent before its children, and since we
	// trickle down the state reverted value down the children, checking the parent
	// of a call will always tell us if the whole chain of parent/child should
	// be reverted
	//
	calls := trace.Calls
	for _, call := range trace.Calls {
		var parent *pbeth.Call
		if call.ParentIndex > 0 {
			parent = calls[call.ParentIndex-1]
		}

		call.StateReverted = (parent != nil && parent.StateReverted) || call.StatusFailed
	}
}

func callAtIndex(idx uint32, calls []*pbeth.Call) *pbeth.Call {
	for _, call := range calls {
		if call.Index == idx {
			return call
		}
	}
	return nil
}

// polygon has a fee log that will never be skipped even if call failed
func isPolygonException(log *pbeth.Log) bool {
	return bytes.Equal(log.Address, polygonFeeSystemAddress) && len(log.Topics) == 4 && bytes.Equal(log.Topics[0], polygonNeverRevertedTopic)
}

func computeLogsBloom(logs []*pbeth.Log) []byte {
	var bf = new(BloomFilter)
	for _, log := range logs {
		bf.add(log.Address)
		for _, topic := range log.Topics {
			bf.add(topic)
		}
	}
	return bf[:]
}

type BloomFilter [256]byte

func (b *BloomFilter) add(data []byte) {
	hash := eth.Keccak256(data)
	b[256-uint((binary.BigEndian.Uint16(hash)&0x7ff)>>3)-1] |= byte(1 << (hash[1] & 0x7))
	b[256-uint((binary.BigEndian.Uint16(hash[2:])&0x7ff)>>3)-1] |= byte(1 << (hash[3] & 0x7))
	b[256-uint((binary.BigEndian.Uint16(hash[4:])&0x7ff)>>3)-1] |= byte(1 << (hash[5] & 0x7))
}

func computePolygonHash(blockNum uint64, blockHash []byte) []byte {
	enc := make([]byte, 8)
	binary.BigEndian.PutUint64(enc, blockNum)
	key := append(append([]byte("matic-bor-receipt-"), enc...), blockHash...)
	return eth.Keccak256(key)
}
