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
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
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
//
//	of the fork logic.
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

func toUint64Array(in []eth.Uint64) []uint64 {
	out := make([]uint64, len(in))

	for i, el := range in {
		out[i] = uint64(el)
	}
	return out
}

func Uint64NestedArrayFromEthUint(in [][]eth.Uint64) *Uint64NestedArray {
	if in == nil {
		return nil
	}
	out := &Uint64NestedArray{}
	for _, v := range in {
		out.Val = append(out.Val, &Uint64Array{
			Val: toUint64Array(v),
		})
	}
	return out
}

func NewUint64NestedArray(in [][]uint64) *Uint64NestedArray {
	out := &Uint64NestedArray{}
	for _, v := range in {
		out.Val = append(out.Val, &Uint64Array{
			Val: v,
		})
	}
	return out
}

func (a *Uint64NestedArray) ToNative() (out [][]uint64) {
	if a == nil {
		return nil
	}

	for _, v := range a.Val {
		out = append(out, v.Val)
	}
	return
}

func (a *Uint64NestedArray) MarshalJSON() ([]byte, error) {
	if a == nil {
		// FIXME: What is the right behavior regarding JSON to output when there is no bytes? Usually I think it should be omitted
		//        entirely but I'm not sure what a custom JSON marshaler can do here to convey that meaning of ok, omit this field.
		return nil, nil
	}

	native := a.ToNative()
	return json.Marshal(native)
}

func (a *Uint64NestedArray) UnmarshalJSON(in []byte) (err error) {
	var out [][]uint64
	err = json.Unmarshal(in, &out)
	if err != nil {
		return
	}

	dummy := NewUint64NestedArray(out)
	a.Val = dummy.Val
	return
}

func (a *Uint64NestedArray) MarshalJSONPB(marshaler *jsonpb.Marshaler) ([]byte, error) {
	return a.MarshalJSON()
}

func (a *Uint64NestedArray) UnmarshalJSONPB(unmarshaler *jsonpb.Unmarshaler, in []byte) (err error) {
	return a.UnmarshalJSON(in)
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

func (call *Call) Method() []byte {
	if len(call.Input) >= 4 {
		return call.Input[0:4]
	}
	return nil
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
