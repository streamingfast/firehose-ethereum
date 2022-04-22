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

	"github.com/mitchellh/go-testing-interface"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/eth-go"
	"github.com/streamingfast/jsonpb"
	"github.com/streamingfast/logging"
	pbbstream "github.com/streamingfast/pbgo/sf/bstream/v1"
	"github.com/streamingfast/sf-ethereum/types"
	pbeth "github.com/streamingfast/sf-ethereum/types/pb/sf/ethereum/type/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var zlog, _ = logging.PackageLogger("sfeth", "github.com/streamingfast/sf-ethereum/types/testing")

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

func Block(t testing.T, blkHash string, components ...interface{}) *pbeth.Block {
	// This is for testing purposes, so it's easier to convey the id and the num from a single element
	ref := bstream.NewBlockRefFromID(blkHash)

	pbblock := &pbeth.Block{
		Ver:    2,
		Hash:   toBytes(t, ref.ID()),
		Number: ref.Num(),
	}

	blockTime, err := time.Parse(time.RFC3339, "2006-01-02T15:04:05.5Z")
	require.NoError(t, err)

	pbblock.Header = &pbeth.BlockHeader{
		ParentHash: toBytes(t, fmt.Sprintf("%08x%s", pbblock.Number-1, blkHash[8:])),
		Timestamp:  timestamppb.New(blockTime),
	}

	for _, component := range components {
		switch v := component.(type) {
		case *pbeth.TransactionTrace:
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

func Trx(t testing.T, components ...interface{}) *pbeth.Transaction {
	trx := &pbeth.Transaction{}
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
			trx.GasPrice = pbeth.BigIntFromNative(v.ToBigInt(t))
		case Value:
			trx.Value = pbeth.BigIntFromNative(v.ToBigInt(t))
		default:
			failInvalidComponent(t, "trx", component)
		}
	}

	return trx
}

func TrxTrace(t testing.T, components ...interface{}) *pbeth.TransactionTrace {
	trace := &pbeth.TransactionTrace{
		Receipt: &pbeth.TransactionReceipt{},
	}

	for _, component := range components {
		switch v := component.(type) {
		case hash:
			trace.Hash = hexString(v).bytes(t)
		case from:
			trace.From = hexString(v).bytes(t)
		case to:
			trace.To = hexString(v).bytes(t)
		case GasPrice:
			trace.GasPrice = pbeth.BigIntFromNative(v.ToBigInt(t))
		case Nonce:
			trace.Nonce = uint64(v)
		case *pbeth.Call:
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

func Call(t testing.T, components ...interface{}) *pbeth.Call {
	call := &pbeth.Call{}
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
		case *pbeth.BalanceChange:
			call.BalanceChanges = append(call.BalanceChanges, v)
		case *pbeth.NonceChange:
			call.NonceChanges = append(call.NonceChanges, v)
		case *pbeth.StorageChange:
			call.StorageChanges = append(call.StorageChanges, v)
		case *pbeth.Log:
			call.Logs = append(call.Logs, v)
		default:
			failInvalidComponent(t, "call", component)
		}
	}

	if call.Value == nil {
		call.Value = pbeth.BigIntFromNative(big.NewInt(0))
	}

	if call.CallType == pbeth.CallType_UNSPECIFIED {
		call.CallType = pbeth.CallType_CALL
	}

	return call
}

func BalanceChange(t testing.T, address address, values string, components ...interface{}) *pbeth.BalanceChange {
	datas := strings.Split(values, "/")

	balanceChange := &pbeth.BalanceChange{
		Address: hexString(address).bytes(t),
	}

	toBigIntBytes := func(value string) []byte {
		bigValue, succeed := new(big.Int).SetString(value, 10)
		require.True(t, succeed, "unable to convert value to BigInt")

		return bigValue.Bytes()
	}

	if datas[0] != "" {
		balanceChange.OldValue = pbeth.BigIntFromBytes(toBigIntBytes(datas[0]))
	}

	if datas[1] != "" {
		balanceChange.NewValue = pbeth.BigIntFromBytes(toBigIntBytes(datas[1]))
	}

	return balanceChange
}

func NonceChange(t testing.T, address address, values string, components ...interface{}) *pbeth.NonceChange {
	datas := strings.Split(values, "/")

	nonceChange := &pbeth.NonceChange{
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

func StorageChange(t testing.T, address address, key hash, data string, components ...interface{}) *pbeth.StorageChange {
	datas := strings.Split(data, "/")

	storageChange := &pbeth.StorageChange{
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

func Log(t testing.T, address address, components ...interface{}) *pbeth.Log {
	log := &pbeth.Log{
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

func ToTimestamp(t time.Time) *timestamppb.Timestamp {
	return timestamppb.New(t)
}

func ToBstreamBlock(t testing.T, block *pbeth.Block) *bstream.Block {
	blk, err := types.BlockFromProto(block)
	require.NoError(t, err)

	return blk
}

func ToBstreamBlocks(t testing.T, blocks []*pbeth.Block) (out []*bstream.Block) {
	out = make([]*bstream.Block, len(blocks))
	for i, block := range blocks {
		out[i] = ToBstreamBlock(t, block)
	}
	return
}

func ToPbbstreamBlock(t testing.T, block *pbeth.Block) *pbbstream.Block {
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
