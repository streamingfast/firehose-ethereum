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

package codec

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	tspb "github.com/golang/protobuf/ptypes/timestamp"

	"github.com/golang/protobuf/ptypes"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/dfuse/ethereum/codec/v1"
)

func FromInt32(input string, tag string) int32 {
	value, err := strconv.ParseInt(input, 10, 32)
	if err != nil {
		panic(fmt.Errorf("%s failed to parse: %s", tag, err))
	}

	return int32(value)
}

func FromUint32(input string, tag string) uint32 {
	value, err := strconv.ParseUint(input, 10, 32)
	if err != nil {
		panic(fmt.Errorf("%s failed to parse: %s", tag, err))
	}

	return uint32(value)
}

func FromUint64(input string, tag string) uint64 {
	value, err := strconv.ParseUint(input, 10, 64)
	if err != nil {
		panic(fmt.Errorf("%s failed to parse: %s", tag, err))
	}

	return value
}

func FromHex(input string, tag string) []byte {
	// The `.` means the value is not present for this field, so let's skip it
	if len(input) == 0 || input == "." {
		return nil
	}

	bytes, err := DecodeHex(input)
	if err != nil {
		panic(fmt.Errorf("%s unable to decode hex string %q: %s", tag, input, err))
	}

	return bytes
}

func DecodeHex(input string) ([]byte, error) {
	bytes, err := hex.DecodeString(SanitizeHex(input))
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

// CanonicalHex receives an input and return it's canonical form,
// i.e. the single unique well-formed which in our case is an all-lower
// case version with even number of characters.
//
// The only differences with `SanitizeHexInput` here is an additional
// call to `strings.ToLower` before returning the result.
func CanonicalHex(input string) string {
	return strings.ToLower(SanitizeHex(input))
}

// PrefixedHex is CanonicalHex but with 0x prefix
func PrefixedHex(input string) string {
	return "0x" + CanonicalHex(input)
}

// ConcatHex concatenates sanitized hex strings
func ConcatHex(with0x bool, in ...string) (out string) {
	if with0x {
		out = "0x"
	}
	for _, s := range in {
		out += SanitizeHex(s)
	}
	return
}

// SanitizeHex removes the prefix `0x` if it exists
// and ensures there is an even number of characters in the string,
// padding on the left of the string is it's not the case.
func SanitizeHex(input string) string {
	if Has0xPrefix(input) {
		input = input[2:]
	}

	if len(input)%2 != 0 {
		input = "0" + input
	}

	return strings.ToLower(input)
}

func ToTimestamp(t time.Time) *tspb.Timestamp {
	el, err := ptypes.TimestampProto(t)
	if err != nil {
		panic(fmt.Errorf("unable to transform time.Time into google proto Timestamp: %s", err))
	}
	return el
}

func FromHeader(header *BlockHeader) *pbcodec.BlockHeader {
	return &pbcodec.BlockHeader{
		ParentHash:       header.ParentHash,
		UncleHash:        header.UncleHash,
		Coinbase:         header.Coinbase,
		StateRoot:        header.Root,
		TransactionsRoot: header.TxHash,
		ReceiptRoot:      header.ReceiptHash,
		LogsBloom:        header.Bloom,
		Difficulty:       pbcodec.BigIntFromBytes(header.Difficulty),
		Number:           uint64(header.Number),
		GasLimit:         uint64(header.GasLimit),
		GasUsed:          uint64(header.GasUsed),
		Timestamp:        ToTimestamp(time.Unix(int64(header.Time), 0)),
		ExtraData:        header.Extra,
		MixHash:          header.MixDigest,
		Nonce:            uint64(header.Nonce),
		Hash:             header.Hash,
	}
}
