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

package kv

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/streamingfast/bstream"
)

const (
	TblPrefixBlocks    = 0x00
	TblPrefixBlockNums = 0x01
	TblPrefixIrrBlks   = 0x02

	TblPrefixTrxTraces = 0x10

	TblTTL = 0x20

	idxPrefixTimelineFwd = 0x80
	idxPrefixTimelineBck = 0x81
)

var Keys Keyer

type Keyer struct{}

// Blocks virtual table

func (k Keyer) PackBlocksKey(blockHash []byte) []byte {
	return append([]byte{TblPrefixBlocks}, blockHash...)
}

func (Keyer) UnpackBlocksKey(key []byte) (blockHash []byte) {
	return key[1:]
}

func (Keyer) StartOfBlocksTable() []byte { return []byte{TblPrefixBlocks} }
func (Keyer) EndOfBlocksTable() []byte   { return []byte{TblPrefixBlocks + 1} }

// BlockNums virtual table

func (Keyer) PackBlockNumsKey(blockNum uint64, blockHash []byte) []byte {
	return append([]byte{TblPrefixBlockNums}, revBlockNumBlockHashBytes(blockNum, blockHash)...)
}

func (k Keyer) PackBlockNumsPrefix(blockNum uint64) []byte {
	return append([]byte{TblPrefixBlockNums}, revBlockNumBytes(blockNum)...)
}

func (Keyer) UnpackBlockNumsKey(key []byte) (blockRef bstream.BlockRef) {
	num := math.MaxUint64 - binary.BigEndian.Uint64(key[1:])
	hash := hex.EncodeToString(key[9:])

	return bstream.NewBlockRef(hash, num)
}

func (Keyer) UnpackBlockNumsKeyHash(key []byte) (hash []byte) {
	return key[9:]
}

func (Keyer) StartOfBlockNumsTable() []byte { return []byte{TblPrefixBlockNums} }
func (Keyer) EndOfBlockNumsTable() []byte   { return []byte{TblPrefixBlockNums + 1} }

// Irr Blocks virt table

func (Keyer) PackIrrBlocksKey(num uint64, hash []byte) []byte {
	return append([]byte{TblPrefixIrrBlks}, revBlockNumBlockHashBytes(num, hash)...)
}

func (Keyer) PackIrrBlocksKeyRef(blockRef bstream.BlockRef) []byte {
	return append([]byte{TblPrefixIrrBlks}, revBlockNumBlockHashBytes(blockRef.Num(), hashAsString(blockRef.ID()).MustBytes())...)
}

func (k Keyer) PackIrrBlocksPrefix(blockNum uint64) []byte {
	return append([]byte{TblPrefixIrrBlks}, revBlockNumBytes(blockNum)...)
}

func (Keyer) UnpackIrrBlocksKey(key []byte) (blockRef bstream.BlockRef) {
	num := math.MaxUint64 - binary.BigEndian.Uint64(key[1:])
	hash := hex.EncodeToString(key[9:])

	return bstream.NewBlockRef(hash, num)
}

func (Keyer) UnpackIrrBlocksKeyHash(key []byte) (hash []byte) {
	return key[9:]
}

func (Keyer) StartOfIrrBlockTable() []byte { return []byte{TblPrefixIrrBlks} }
func (Keyer) EndOfIrrBlockTable() []byte   { return []byte{TblPrefixIrrBlks + 1} }

// TrxTrace virt table
func (k Keyer) PackTrxTracesKey(trxHash []byte, blockNum uint64, blockHash []byte) []byte {
	return append([]byte{TblPrefixTrxTraces}, trxHashRevBlockNumBlockHashBytes(trxHash, blockNum, blockHash)...)
}

func (k Keyer) UnpackTrxTracesKey(key []byte) (trxHash, blockRef bstream.BlockRef) {
	panic("not implemented")
}

func (k Keyer) PackTrxTracesPrefix(trxHash []byte) []byte {
	return k.packTrxPrefix(TblPrefixTrxTraces, trxHash)
}

func (Keyer) StartOfTrxTracesTable() []byte { return []byte{TblPrefixTrxTraces} }
func (Keyer) EndOfTrxTracesTable() []byte   { return []byte{TblPrefixTrxTraces + 1} }

// Timeline indexes

func (Keyer) PackTimelineKey(fwd bool, blockTime time.Time, blockHash string) []byte {
	bKey, err := hex.DecodeString(blockHash)
	if err != nil {
		panic(fmt.Sprintf("failed to decode block ID %q: %s", blockHash, err))
	}

	tKey := make([]byte, 9)
	if fwd {
		tKey[0] = idxPrefixTimelineFwd
	} else {
		tKey[0] = idxPrefixTimelineBck
	}
	nano := uint64(blockTime.UnixNano() / 100000000)
	if !fwd {
		nano = maxUnixTimestampDeciSeconds - nano
	}
	binary.BigEndian.PutUint64(tKey[1:], nano)
	return append(tKey, bKey...)
}

var maxUnixTimestampDeciSeconds = uint64(99999999999)

func (Keyer) UnpackTimelineKey(fwd bool, key []byte) (blockTime time.Time, blockHash string) {
	t := binary.BigEndian.Uint64(key[1:9]) // skip table prefix
	if !fwd {
		t = maxUnixTimestampDeciSeconds - t
	}
	ns := (int64(t) % 10) * 100000000
	blockTime = time.Unix(int64(t)/10, ns).UTC()
	blockHash = hex.EncodeToString(key[9:])
	return
}

func (k Keyer) PackTimelinePrefix(fwd bool, blockTime time.Time) []byte {
	return k.PackTimelineKey(fwd, blockTime, "")
}

func (Keyer) StartOfTimelineIndex(fwd bool) []byte {
	if fwd {
		return []byte{idxPrefixTimelineFwd}
	}
	return []byte{idxPrefixTimelineBck}
}

func (Keyer) EndOfTimelineIndex(fwd bool) []byte {
	if fwd {
		return []byte{idxPrefixTimelineFwd + 1}
	}
	return []byte{idxPrefixTimelineBck + 1}
}

func (Keyer) packTrxPrefix(prefix byte, trxHashPrefix []byte) []byte {
	return append([]byte{prefix}, trxHashPrefix...)
}

func revBlockNumBytes(blockNum uint64) []byte {
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes, math.MaxUint64-blockNum)

	return bytes
}

func revBlockNumBlockHashBytes(blockNum uint64, blockHash []byte) []byte {
	bytes := make([]byte, 8+len(blockHash))
	binary.BigEndian.PutUint64(bytes, math.MaxUint64-blockNum)
	copy(bytes[8:], blockHash)

	return bytes
}

func trxHashRevBlockNumBlockHashBytes(trxHash []byte, blockNum uint64, blockHash []byte) []byte {
	bytes := make([]byte, len(trxHash)+8+len(blockHash))
	copy(bytes, trxHash)

	offset := len(trxHash)
	binary.BigEndian.PutUint64(bytes[offset:], math.MaxUint64-blockNum)

	offset += 8
	copy(bytes[offset:], blockHash)

	return bytes
}

// zapKey can be used to wrap a key (in `[]byte` form) and enable "Debug"
// output stringer,
type zapKey []byte

func (s zapKey) String() string {
	return hex.EncodeToString([]byte(s))
}

// zapKeys can be used to wrap an array of key (in `[]byte` form) and enable "Debug"
// output stringer that can be used by zap to limit runtime footprint of performing the
// string transformation.
type zapKeys [][]byte

func (s zapKeys) String() string {
	count := len(s)
	all := make([]string, count)
	for i, key := range s {
		all[i] = hex.EncodeToString(key)
	}

	if count > 25 {
		return strings.Join(all, ", ") + fmt.Sprintf(" and %d more (%d total) ...", count-25, count)
	}

	return strings.Join(all, ", ")
}
