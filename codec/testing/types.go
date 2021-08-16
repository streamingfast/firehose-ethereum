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

package ct

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes"
	tspb "github.com/golang/protobuf/ptypes/timestamp"
	"github.com/mitchellh/go-testing-interface"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/eth-go"
	"github.com/streamingfast/jsonpb"
	"github.com/streamingfast/logging"
	pbbstream "github.com/streamingfast/pbgo/dfuse/bstream/v1"
	"github.com/streamingfast/sf-ethereum/codec"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/dfuse/ethereum/codec/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

var zlog *zap.Logger

func init() {
	logging.Register("github.com/streamingfast/sf-ethereum/codec/testing", &zlog)
}

type from hexString

func From(in string) from     { return from(newHexString(in)) }
func FromFull(in string) from { return from(newHexString(in, Full, length(20))) }

type to hexString

func To(in string) to     { return to(newHexString(in)) }
func ToFull(in string) to { return to(newHexString(in, Full, length(20))) }

type previousHash hexString

func PreviousHash(in string) previousHash { return previousHash(newHexString(in)) }
func PreviousHashFull(in string) previousHash {
	return previousHash(newHexString(in, Full, length(20)))
}

func Block(t testing.T, blkHash string, components ...interface{}) *pbcodec.Block {
	// This is for testing purposes, so it's easier to convey the id and the num from a single element
	ref := bstream.NewBlockRefFromID(blkHash)

	pbblock := &pbcodec.Block{
		Ver:    1,
		Hash:   toBytes(t, ref.ID()),
		Number: ref.Num(),
	}

	blockTime, err := time.Parse(time.RFC3339, "2006-01-02T15:04:05.5Z")
	require.NoError(t, err)

	blockTimestamp, err := ptypes.TimestampProto(blockTime)
	require.NoError(t, err)

	pbblock.Header = &pbcodec.BlockHeader{
		ParentHash: toBytes(t, fmt.Sprintf("%08x%s", pbblock.Number-1, blkHash[8:])),
		Timestamp:  blockTimestamp,
	}

	for _, component := range components {
		switch v := component.(type) {
		case *pbcodec.TransactionTrace:
			pbblock.TransactionTraces = append(pbblock.TransactionTraces, v)
		case previousHash:
			pbblock.Header.ParentHash = hexString(v).bytes(t)
		default:
			failInvalidComponent(t, "block", component)
		}
	}

	if os.Getenv("DEBUG") != "" {
		marshaler := &jsonpb.Marshaler{}
		out, err := marshaler.MarshalToString(pbblock)
		require.NoError(t, err)

		// We re-normalize to a plain map[string]interface{} so it's printed as JSON and not a proto default String implementation
		normalizedOut := map[string]interface{}{}
		require.NoError(t, json.Unmarshal([]byte(out), &normalizedOut))

		zlog.Debug("created test block", zap.Any("block", normalizedOut))
	}

	return pbblock
}

type Nonce uint64
type InputData string
type GasLimit uint64
type GasPrice string
type Value string

func (p GasPrice) ToBigInt(t testing.T) *big.Int {
	return Ether(p).ToBigInt(t)
}

func (v Value) ToBigInt(t testing.T) *big.Int {
	return Ether(v).ToBigInt(t)
}

var b1e18 = new(big.Int).Exp(big.NewInt(1), big.NewInt(18), nil)

type Ether string

func (e Ether) ToBigInt(t testing.T) *big.Int {
	in := string(e)
	if strings.HasSuffix(in, " ETH") {
		raw := strings.TrimSuffix(in, " ETH")

		dotIndex := strings.Index(in, ".")
		if dotIndex >= 0 {
			raw = raw[0:dotIndex] + raw[dotIndex+1:]
		}

		if len(raw) < 19 {
			raw = raw + strings.Repeat("0", 19-len(raw))
		} else if len(raw) > 19 {
			raw = raw[0:19]
		}

		out, worked := new(big.Int).SetString(raw, 10)
		require.True(t, worked, "Conversion of %q to big.Int failed", raw)

		return out.Mul(out, b1e18)
	}

	out, worked := new(big.Int).SetString(in, 0)
	require.True(t, worked, "Conversion of %q to big.Int failed", in)
	return out
}

func Trx(t testing.T, components ...interface{}) *pbcodec.Transaction {
	trx := &pbcodec.Transaction{}
	for _, component := range components {
		switch v := component.(type) {
		case hash:
			trx.Hash = hexString(v).bytes(t)
		case from:
			trx.From = hexString(v).bytes(t)
		case to:
			trx.To = hexString(v).bytes(t)
		case InputData:
			trx.Input = toBytes(t, string(v))
		case Nonce:
			trx.Nonce = uint64(v)
		case GasLimit:
			trx.GasLimit = uint64(v)
		case GasPrice:
			trx.GasPrice = pbcodec.BigIntFromNative(v.ToBigInt(t))
		case Value:
			trx.Value = pbcodec.BigIntFromNative(v.ToBigInt(t))
		default:
			failInvalidComponent(t, "trx", component)
		}
	}

	return trx
}

func TrxTrace(t testing.T, components ...interface{}) *pbcodec.TransactionTrace {
	trace := &pbcodec.TransactionTrace{}
	for _, component := range components {
		switch v := component.(type) {
		case hash:
			trace.Hash = hexString(v).bytes(t)
		case from:
			trace.From = hexString(v).bytes(t)
		case to:
			trace.To = hexString(v).bytes(t)
		case GasPrice:
			trace.GasPrice = pbcodec.BigIntFromNative(v.ToBigInt(t))
		case Nonce:
			trace.Nonce = uint64(v)
		case *pbcodec.Call:
			trace.Calls = append(trace.Calls, v)
		default:
			failInvalidComponent(t, "trx_trace", component)
		}
	}

	return trace
}

type caller hexString

func Caller(in string) caller     { return caller(newHexString(in)) }
func CallerFull(in string) caller { return caller(newHexString(in, Full, length(20))) }

func Call(t testing.T, components ...interface{}) *pbcodec.Call {
	call := &pbcodec.Call{}
	for _, component := range components {
		switch v := component.(type) {
		case from:
			call.Caller = hexString(v).bytes(t)
		case caller:
			call.Caller = hexString(v).bytes(t)
		case address:
			call.Address = hexString(v).bytes(t)
		case to:
			call.Address = hexString(v).bytes(t)
		case *pbcodec.BalanceChange:
			call.BalanceChanges = append(call.BalanceChanges, v)
		case *pbcodec.NonceChange:
			call.NonceChanges = append(call.NonceChanges, v)
		case *pbcodec.StorageChange:
			call.StorageChanges = append(call.StorageChanges, v)
		case *pbcodec.Log:
			call.Logs = append(call.Logs, v)
		default:
			failInvalidComponent(t, "call", component)
		}
	}

	if call.Value == nil {
		call.Value = pbcodec.BigIntFromNative(big.NewInt(0))
	}

	if call.CallType == pbcodec.CallType_UNSPECIFIED {
		call.CallType = pbcodec.CallType_CALL
	}

	return call
}

func BalanceChange(t testing.T, address address, values string, components ...interface{}) *pbcodec.BalanceChange {
	datas := strings.Split(values, "/")

	balanceChange := &pbcodec.BalanceChange{
		Address: hexString(address).bytes(t),
	}

	toBigIntBytes := func(value string) []byte {
		bigValue, succeed := new(big.Int).SetString(value, 10)
		require.True(t, succeed, "unable to convert value to BigInt")

		return bigValue.Bytes()
	}

	if datas[0] != "" {
		balanceChange.OldValue = pbcodec.BigIntFromBytes(toBigIntBytes(datas[0]))
	}

	if datas[1] != "" {
		balanceChange.NewValue = pbcodec.BigIntFromBytes(toBigIntBytes(datas[1]))
	}

	return balanceChange
}

func NonceChange(t testing.T, address address, values string, components ...interface{}) *pbcodec.NonceChange {
	datas := strings.Split(values, "/")

	nonceChange := &pbcodec.NonceChange{
		Address: hexString(address).bytes(t),
	}

	toUint64 := func(value string) uint64 {
		nonce, err := strconv.ParseUint(value, 10, 64)
		require.NoError(t, err, "unable to convert nonce to uint64")

		return nonce
	}

	if datas[0] != "" {
		nonceChange.OldValue = toUint64(datas[0])
	}

	if datas[1] != "" {
		nonceChange.NewValue = toUint64(datas[1])
	}

	return nonceChange
}

func StorageChange(t testing.T, address address, key hash, data string, components ...interface{}) *pbcodec.StorageChange {
	datas := strings.Split(data, "/")

	storageChange := &pbcodec.StorageChange{
		Address: hexString(address).bytes(t),
		Key:     hexString(key).bytes(t),
	}

	if datas[0] != "" {
		storageChange.OldValue = toFilledBytes(t, datas[0], 32)
	}

	if datas[1] != "" {
		storageChange.NewValue = toFilledBytes(t, datas[1], 32)
	}

	return storageChange
}

type logTopic hexString

func LogTopic(in string) logTopic     { return logTopic(newHexString(in)) }
func LogTopicFull(in string) logTopic { return logTopic(newHexString(in, Full, length(32))) }

type LogData string

func Log(t testing.T, address address, components ...interface{}) *pbcodec.Log {
	log := &pbcodec.Log{
		Address: hexString(address).bytes(t),
	}

	for _, component := range components {
		switch v := component.(type) {
		case logTopic:
			log.Topics = append(log.Topics, hexString(v).bytes(t))
		case LogData:
			log.Data = toBytes(t, string(v))

		default:
			failInvalidComponent(t, "log", component)
		}
	}

	return log
}

func ToTimestamp(t time.Time) *tspb.Timestamp {
	el, err := ptypes.TimestampProto(t)
	if err != nil {
		panic(err)
	}

	return el
}

func ToBstreamBlock(t testing.T, block *pbcodec.Block) *bstream.Block {
	blk, err := codec.BlockFromProto(block)
	require.NoError(t, err)

	return blk
}

func ToBstreamBlocks(t testing.T, blocks []*pbcodec.Block) (out []*bstream.Block) {
	out = make([]*bstream.Block, len(blocks))
	for i, block := range blocks {
		out[i] = ToBstreamBlock(t, block)
	}
	return
}

func ToPbbstreamBlock(t testing.T, block *pbcodec.Block) *pbbstream.Block {
	blk, err := ToBstreamBlock(t, block).ToProto()
	require.NoError(t, err)

	return blk
}

type address hexString

func Address(in string) address     { return address(newHexString(in)) }
func AddressFull(in string) address { return address(newHexString(in, Full, length(20))) }

func (a address) Bytes(t testing.T) []byte  { return hexString(a).bytes(t) }
func (a address) String(t testing.T) string { return hexString(a).string(t) }

type hash hexString

func Hash(in string) hash     { return hash(newHexString(in)) }
func HashFull(in string) hash { return hash(newHexString(in, Full, length(32))) }

func (a hash) Bytes(t testing.T) []byte  { return hexString(a).bytes(t) }
func (a hash) String(t testing.T) string { return hexString(a).string(t) }

func toBytes(t testing.T, in string) []byte {
	out, err := hex.DecodeString(eth.SanitizeHex(in))
	require.NoError(t, err)

	return out
}

func toFilledBytes(t testing.T, in string, length int) []byte {
	out := toBytes(t, in)
	if len(out) == length {
		return out
	}

	if len(out) < length {
		copied := make([]byte, length)
		copy(copied, out)
		out = copied
	} else {
		// Necessarly longer
		out = out[0:length]
	}

	return out
}

type expand bool
type length int

const Full expand = true

type hexString struct {
	in     string
	expand bool
	length int
}

func newHexString(in string, opts ...interface{}) (out hexString) {
	out.in = in
	for _, opt := range opts {
		switch v := opt.(type) {
		case expand:
			out.expand = bool(v)
		case length:
			out.length = int(v)
		}
	}
	return
}

func (h hexString) bytes(t testing.T) []byte {
	if h.expand {
		return toFilledBytes(t, h.in, h.length)
	}

	return toBytes(t, h.in)
}

func (h hexString) string(t testing.T) string {
	return hex.EncodeToString(h.bytes(t))
}

type ignoreComponent func(v interface{}) bool

func failInvalidComponent(t testing.T, tag string, component interface{}, options ...interface{}) {
	shouldIgnore := ignoreComponent(func(v interface{}) bool { return false })
	for _, option := range options {
		switch v := option.(type) {
		case ignoreComponent:
			shouldIgnore = v
		}
	}

	if shouldIgnore(component) {
		return
	}

	require.FailNowf(t, "invalid component", "Invalid %s component of type %T", tag, component)
}

func logInvalidComponent(tag string, component interface{}) {
	zlog.Info(fmt.Sprintf("invalid %s component of type %T", tag, component))
}
