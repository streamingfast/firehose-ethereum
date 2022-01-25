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

package pbcodec

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/jsonpb"
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
	timestamp, err := ptypes.Timestamp(b.Header.Timestamp)
	if err != nil {
		return time.Time{}, fmt.Errorf("unable to turn google proto Timestamp into time.Time: %s", err)
	}

	return timestamp, nil
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
	if b.Number == bstream.GetProtocolFirstStreamableBlock {
		return bstream.GetProtocolGenesisBlock
	}

	if b.Number <= 200 {
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

func BigIntFromBytes(in []byte) *BigInt {
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

// PopulateLogBlockIndices fixes the `TransactionReceipt.Logs[].BlockIndex`
// that is not properly populated by our deep mind instrumentation.
func (block *Block) PopulateLogBlockIndices() {
	receiptLogBlockIndex := uint32(0)
	for _, trace := range block.TransactionTraces {
		for _, log := range trace.Receipt.Logs {
			log.BlockIndex = receiptLogBlockIndex
			receiptLogBlockIndex++
		}
	}

	callLogBlockIndex := uint32(0)
	for _, trace := range block.TransactionTraces {
		for _, call := range trace.Calls {
			for _, log := range call.Logs {
				if call.StateReverted {
					log.BlockIndex = 0
				} else {
					log.BlockIndex = callLogBlockIndex
					callLogBlockIndex++
				}
			}
		}
	}
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
