// Copyright 2021 dfuse Platform Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pbeth

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/streamingfast/bstream"
	"github.com/streamingfast/eth-go"
	"github.com/streamingfast/jsonpb"
	"google.golang.org/protobuf/proto"
)

var b0 = big.NewInt(0)

func MustBlockRefAsProto(ref bstream.BlockRef) *BlockRef {
	if ref == nil || bstream.EqualsBlockRefs(ref, bstream.BlockRefEmpty) {
		return nil
	}

	hash, err := hex.DecodeString(ref.ID())
	if err != nil {
		panic(fmt.Errorf("invalid block hash %q: %w", ref.ID(), err))
	}

	return &BlockRef{
		Hash:   hash,
		Number: ref.Num(),
	}
}

func (b *BlockRef) AsBstreamBlockRef() bstream.BlockRef {
	return bstream.NewBlockRef(hex.EncodeToString(b.Hash), b.Number)
}

// TODO: We should probably memoize all fields that requires computation
//       like ID() and likes.

func (b *Block) ID() string {
	return hex.EncodeToString(b.Hash)
}

func (b *Block) Num() uint64 {
	return b.Number
}

func (b *Block) Time() (time.Time, error) {
	return b.Header.Timestamp.AsTime(), nil
}

func (b *Block) MustTime() time.Time {
	timestamp, err := b.Time()
	if err != nil {
		panic(err)
	}

	return timestamp
}

func (b *Block) PreviousID() string {
	return hex.EncodeToString(b.Header.ParentHash)
}

// FIXME: This logic at some point is hard-coded and will need to be re-visited in regard
//        of the fork logic.
func (b *Block) LIBNum() uint64 {
	if b.Number <= bstream.GetProtocolFirstStreamableBlock+200 {
		return bstream.GetProtocolFirstStreamableBlock
	}

	return b.Number - 200
}

func (b *Block) AsRef() bstream.BlockRef {
	return bstream.NewBlockRef(b.ID(), b.Number)
}

func NewBigInt(in int64) *BigInt {
	return BigIntFromNative(big.NewInt(in))
}

func BigIntFromNative(in *big.Int) *BigInt {
	var bytes []byte
	if in != nil {
		bytes = in.Bytes()
	}

	return &BigInt{Bytes: bytes}
}

// BigIntFromBytes creates a new `pbeth.BigInt` from the received bytes. If the the received
// bytes is nil or of length 0, then `nil` is returned directly.
func BigIntFromBytes(in []byte) *BigInt {
	if len(in) == 0 {
		return nil
	}

	return &BigInt{Bytes: in}
}

func (m *BigInt) Uint64() uint64 {
	if m == nil {
		return 0
	}

	i := new(big.Int).SetBytes(m.Bytes)
	return i.Uint64()
}

func (m *BigInt) Native() *big.Int {
	if m == nil {
		return b0
	}

	z := new(big.Int)
	z.SetBytes(m.Bytes)
	return z
}

func (m *BigInt) MarshalJSON() ([]byte, error) {
	if m == nil {
		// FIXME: What is the right behavior regarding JSON to output when there is no bytes? Usually I think it should be omitted
		//        entirely but I'm not sure what a custom JSON marshaler can do here to convey that meaning of ok, omit this field.
		return nil, nil
	}

	return []byte(`"` + hex.EncodeToString(m.Bytes) + `"`), nil
}

func (m *BigInt) UnmarshalJSON(in []byte) (err error) {
	var s string
	err = json.Unmarshal(in, &s)
	if err != nil {
		return
	}

	m.Bytes, err = hex.DecodeString(s)
	return
}

func (m *BigInt) MarshalJSONPB(marshaler *jsonpb.Marshaler) ([]byte, error) {
	return m.MarshalJSON()
}

func (m *BigInt) UnmarshalJSONPB(unmarshaler *jsonpb.Unmarshaler, in []byte) (err error) {
	return m.UnmarshalJSON(in)
}

func BlockToBuffer(block *Block) ([]byte, error) {
	buf, err := proto.Marshal(block)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func MustBlockToBuffer(block *Block) []byte {
	buf, err := BlockToBuffer(block)
	if err != nil {
		panic(err)
	}
	return buf
}

var polygonSystemAddress = eth.MustNewAddress("0xffffFFFfFFffffffffffffffFfFFFfffFFFfFFfE")

var polygonNeverRevertedTopic = eth.MustNewBytes("0x4dfe1bbbcf077ddc3e01291eea2d5c70c2b422b415d95645b9adcfd678cb1d63")
var polygonFeeSystemAddress = eth.MustNewAddress("0x0000000000000000000000000000000000001010")

// polygon has a fee log that will never be skipped even if call failed
func isPolygonException(log *Log) bool {
	return bytes.Equal(log.Address, polygonFeeSystemAddress) && len(log.Topics) == 4 && bytes.Equal(log.Topics[0], polygonNeverRevertedTopic)
}

// NormalizeBlockInPlace
func (block *Block) NormalizeInPlace() {
	// We reconstruct the state reverted value per call, for each transaction traces. We also
	// normalize signature curve points since we were not setting to be alwasy 32 bytes long and
	// sometimes, it would have been only 31 bytes long.
	for _, trx := range block.TransactionTraces {
		trx.PopulateStateReverted()
		trx.PopulateTrxStatus()

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
	if err := block.PopulateLogBlockIndices(); err != nil {
		panic(fmt.Errorf("normalizing log block indices: %w", err))
	}
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

// PopulateLogBlockIndices fixes the `TransactionReceipt.Logs[].BlockIndex`
// that is not properly populated by our deep mind instrumentation.
func (block *Block) PopulateLogBlockIndices() error {

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
	var callLogsToNumber []*Log
	for _, trace := range block.TransactionTraces {
		if bytes.Equal(polygonSystemAddress, trace.From) { // known "fake" polygon transactions
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

	// also make a map of those logs
	blockIndexToTraceLog := make(map[uint32]*Log)

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
				return fmt.Errorf("error in tweak function: log proto not equal")
			}
		}
	}
	if receiptLogCount != len(blockIndexToTraceLog) {
		return fmt.Errorf("error incorrect number of receipt logs in tweak function: %d, expecting %d", receiptLogCount, len(blockIndexToTraceLog))
	}
	return nil
}

func (trace *TransactionTrace) PopulateTrxStatus() {
	// transaction trace Status must be populatged according to simple rule: if call 0 fails, transaction fails.
	if trace.Status == TransactionTraceStatus_UNKNOWN && len(trace.Calls) >= 1 {
		call := trace.Calls[0]
		switch {
		case call.StatusFailed && call.StatusReverted:
			trace.Status = TransactionTraceStatus_REVERTED
		case call.StatusFailed:
			trace.Status = TransactionTraceStatus_FAILED
		default:
			trace.Status = TransactionTraceStatus_SUCCEEDED
		}
	}
	return
}

func (call *Call) Method() []byte {
	if len(call.Input) >= 4 {
		return call.Input[0:4]
	}
	return nil
}

func (trace *TransactionTrace) PopulateStateReverted() {
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
		var parent *Call
		if call.ParentIndex > 0 {
			parent = calls[call.ParentIndex-1]
		}

		call.StateReverted = (parent != nil && parent.StateReverted) || call.StatusFailed
	}

	return
}

func MustBalanceChangeReasonFromString(reason string) BalanceChange_Reason {
	if reason == "ignored" {
		panic("receive ignored balance change reason, we do not expect this as valid input for block generation")
	}

	// There was a typo at some point, let's accept it still until Geth with typo fix is rolled out
	if reason == "reward_transfaction_fee" {
		return BalanceChange_REASON_REWARD_TRANSACTION_FEE
	}

	enumID := BalanceChange_Reason_value["REASON_"+strings.ToUpper(reason)]
	if enumID == 0 {
		panic(fmt.Errorf("receive unknown balance change reason, received reason string is %q", reason))
	}

	return BalanceChange_Reason(enumID)
}

func MustGasChangeReasonFromString(reason string) GasChange_Reason {
	enumID := GasChange_Reason_value["REASON_"+strings.ToUpper(reason)]
	if enumID == 0 {
		panic(fmt.Errorf("receive unknown gas change reason, received reason string is %q", reason))
	}

	return GasChange_Reason(enumID)
}
