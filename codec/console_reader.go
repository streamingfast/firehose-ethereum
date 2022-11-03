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
	"bufio"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/streamingfast/bstream"
	"github.com/streamingfast/dmetrics"
	"github.com/streamingfast/firehose-ethereum/types"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"go.uber.org/zap"
)

var BlockVersion = int32(0) // this will be set inside readInit()

// ConsoleReader is what reads the `geth` output directly. It builds
// up some LogEntry objects. See `LogReader to read those entries .
type ConsoleReader struct {
	lines chan string
	close func()

	ctx   *parseCtx
	done  chan interface{}
	stats *consoleReaderStats

	logger *zap.Logger
}

func NewConsoleReader(logger *zap.Logger, lines chan string) (*ConsoleReader, error) {
	globalStats := newConsoleReaderStats()
	globalStats.StartPeriodicLogToZap(context.Background(), logger, 30*time.Second)

	l := &ConsoleReader{
		lines: lines,
		close: func() {},

		ctx:   &parseCtx{logger: logger, globalStats: globalStats},
		done:  make(chan interface{}),
		stats: globalStats,

		logger: logger,
	}

	return l, nil
}

// todo: WTF?
func (l *ConsoleReader) Done() <-chan interface{} {
	return l.done
}

func (c *ConsoleReader) Close() {
	c.stats.StopPeriodicLogToZap()
	c.close()
}

type consoleReaderStats struct {
	lastBlock             bstream.BlockRef
	blockRate             *dmetrics.RateCounter
	blockAverageParseTime *dmetrics.AvgDurationCounter
	transactionRate       *dmetrics.AvgCounter

	cancelPeriodicLogger context.CancelFunc
}

func newConsoleReaderStats() *consoleReaderStats {
	return &consoleReaderStats{
		lastBlock:             bstream.BlockRefEmpty,
		blockRate:             dmetrics.NewPerMinuteLocalRateCounter("blocks"),
		blockAverageParseTime: dmetrics.NewAvgDurationCounter(1*time.Minute, 1*time.Millisecond, "processing block"),
		transactionRate:       dmetrics.NewAvgCounter(1*time.Minute, "trxs"),
	}
}

func (s *consoleReaderStats) StartPeriodicLogToZap(ctx context.Context, logger *zap.Logger, logEach time.Duration) {
	ctx, s.cancelPeriodicLogger = context.WithCancel(ctx)

	go func() {
		ticker := time.NewTicker(logEach)
		for {
			select {
			case <-ticker.C:
				logger.Info("reader node statistics", s.ZapFields()...)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (s *consoleReaderStats) StopPeriodicLogToZap() {
	if s.cancelPeriodicLogger != nil {
		s.cancelPeriodicLogger()
	}
}

func (s *consoleReaderStats) ZapFields() []zap.Field {
	return []zap.Field{
		zap.Stringer("block_rate", s.blockRate),
		zap.Stringer("trx_rate", s.transactionRate),
		zap.Stringer("last_block", s.lastBlock),
		zap.Stringer("block_average_parse_time", s.blockAverageParseTime),
	}
}

type parsingStats struct {
	startAt  time.Time
	blockNum uint64
	data     map[string]int
	logger   *zap.Logger
}

func newParsingStats(logger *zap.Logger, block uint64) *parsingStats {
	return &parsingStats{
		startAt:  time.Now(),
		blockNum: block,
		data:     map[string]int{},
		logger:   logger,
	}
}

func (s *parsingStats) log() {
	s.logger.Debug("reader block stats",
		zap.Uint64("block_num", s.blockNum),
		zap.Int64("duration", int64(time.Since(s.startAt))),
		zap.Reflect("stats", s.data),
	)
}

func (s *parsingStats) inc(key string) {
	if s == nil {
		return
	}
	k := strings.ToLower(key)
	value := s.data[k]
	value++
	s.data[k] = value
}

type parseCtx struct {
	currentBlock         *pbeth.Block
	currentTrace         *pbeth.TransactionTrace
	currentTraceLogCount int
	// currentRootCall is a pointer to the first EVM call. It is used to collect
	// CreateAccount, BalanceChange, NonceChanges and append them in order in the first EVM call
	currentRootCall *pbeth.Call
	finalizing      bool

	transactionTraces   []*pbeth.TransactionTrace
	evmCallStackIndexes []int32

	blockStoreURL string

	stats       *parsingStats
	globalStats *consoleReaderStats

	logger *zap.Logger
}

func (c *ConsoleReader) ReadBlock() (out *bstream.Block, err error) {
	v, err := c.next(readBlock)
	if err != nil {
		return nil, err
	}

	if v == nil {
		return nil, fmt.Errorf("console reader read a nil *bstream.Block, this is invalid")
	}

	return v.(*bstream.Block), nil
}

func (c ConsoleReader) ReadTransaction() (trace *pbeth.TransactionTrace, err error) {
	out, err := c.next(readTransaction)
	if err != nil {
		return nil, err
	}

	return out.(*pbeth.TransactionTrace), nil
}

const (
	readBlock       = 1
	readTransaction = 2
)

func (c *ConsoleReader) next(readType int) (out interface{}, err error) {
	ctx := c.ctx

	c.logger.Debug("next", zap.Int("read_type", readType))

	for line := range c.lines {
		switch {
		case strings.HasPrefix(line, "DMLOG "):
			line = line[6:]
		case strings.HasPrefix(line, "FIRE "):
			line = line[5:]
		default:
			continue
		}

		// *Important*
		//
		// We are trying to order the lines based on the amount of time they occur in average
		// in a sample of lines.
		//
		// Easiest way is to use the battelfield dmlog test file we have in the project:
		//
		//     cat codec/testdata/firehose-logs.dmlog | grep -Eo "(DMLOG|FIRE) ([^ ]+)" | sort | uniq -c | sort -nr
		//
		// And order the cases here with the order given by the file.
		//
		// It's a micro-optimization but's worth it.
		switch {
		case strings.HasPrefix(line, "GAS_CHANGE"):
			ctx.stats.inc("GAS_CHANGE")
			err = ctx.readGasChange(line)

		case strings.HasPrefix(line, "BALANCE_CHANGE"):
			ctx.stats.inc("BALANCE_CHANGE")
			err = ctx.readBalanceChange(line)

		case strings.HasPrefix(line, "STORAGE_CHANGE"):
			ctx.stats.inc("STORAGE_CHANGE")
			err = ctx.readStorageChange(line)

		case strings.HasPrefix(line, "NONCE_CHANGE"):
			ctx.stats.inc("NONCE_CHANGE")
			err = ctx.readNonceChange(line)

		case strings.HasPrefix(line, "EVM_RUN_CALL"):
			ctx.stats.inc("EVM_RUN_CALL")
			err = ctx.readEVMRunCall(line)

		case strings.HasPrefix(line, "EVM_PARAM"):
			ctx.stats.inc("EVM_PARAM")
			err = ctx.readEVMParamCall(line)

		case strings.HasPrefix(line, "EVM_END_CALL"):
			ctx.stats.inc("EVM_END_CALL")
			err = ctx.readEVMEndCall(line)

		case strings.HasPrefix(line, "ADD_LOG"):
			ctx.stats.inc("ADD_LOG")
			err = ctx.readAddLog(line)

		case strings.HasPrefix(line, "TRX_FROM"):
			ctx.stats.inc("TRX_FROM")
			err = ctx.readTrxFrom(line)

		case strings.HasPrefix(line, "EVM_KECCAK"):
			ctx.stats.inc("EVM_KECCAK")
			err = ctx.readEVMKeccak(line)

		case strings.HasPrefix(line, "BEGIN_BLOCK") && readType == readBlock:
			err = ctx.readBeginBlock(line)

		case strings.HasPrefix(line, "BEGIN_APPLY_TRX"):
			ctx.stats.inc("BEGIN_APPLY_TRX")
			err = ctx.readApplyTrxBegin(line)

		case strings.HasPrefix(line, "END_APPLY_TRX"):
			ctx.stats.inc("END_APPLY_TRX")
			err = ctx.readApplyTrxEnd(line)

			if readType == readTransaction {
				if len(ctx.transactionTraces) != 1 {
					return nil, fmt.Errorf("expecting to have a single transaction trace, got %d", len(ctx.transactionTraces))
				}

				return ctx.transactionTraces[0], err
			}

		case strings.HasPrefix(line, "CODE_CHANGE"):
			ctx.stats.inc("CODE_CHANGE")
			err = ctx.readCodeChange(line)

		case strings.HasPrefix(line, "SUICIDE_CHANGE"):
			ctx.stats.inc("SUICIDE_CHANGE")
			err = ctx.readSuicideChange(line)

		case strings.HasPrefix(line, "END_BLOCK") && readType == readBlock:
			return ctx.readEndBlock(line)

		case strings.HasPrefix(line, "CREATED_ACCOUNT"):
			ctx.stats.inc("CREATED_ACCOUNT")
			err = ctx.readCreateAccount(line)

		case strings.HasPrefix(line, "EVM_CALL_FAILED"):
			ctx.stats.inc("EVM_CALL_FAILED")
			err = ctx.readEVMCallFailed(line)

		case strings.HasPrefix(line, "EVM_REVERTED"):
			ctx.stats.inc("EVM_CALL_FAILED")
			err = ctx.readEVMReverted(line)

		case strings.HasPrefix(line, "ACCOUNT_WITHOUT_CODE"):
			ctx.stats.inc("ACCOUNT_WITHOUT_CODE")
			err = ctx.readAccountWithoutCode(line)

		case strings.HasPrefix(line, "SKIPPED_TRX"):
			ctx.stats.inc("SKIPPED_TRX")
			err = ctx.readSkippedTrx(line)

		case strings.HasPrefix(line, "FINALIZE_BLOCK") && readType == readBlock:
			ctx.stats.inc("FINALIZE_BLOCK")
			err = ctx.readFinalizeBlock(line)

		case strings.HasPrefix(line, "FAILED_APPLY_TRX") && readType == readBlock:
			// This fails the whole block, and happens when we get a
			// block that is not signed with the right chain ID, but
			// still circulates on the network we're on.  This is
			// freaking wasteful.. so anyway, we just reset
			// everything.
			//
			// This short-circuits FINALIZE_BLOCK, END_APPLY_TRX,
			// END_BLOCK
			ctx.stats.inc("FAILED_APPLY_TRX")
			err = ctx.readFailedApplyTrx(line)

		case strings.HasPrefix(line, "TRX_ENTER_POOL"):
			ctx.stats.inc("TRX_ENTER_POOL")
			continue
		case strings.HasPrefix(line, "TRX_DISCARDED"):
			ctx.stats.inc("TRX_DISCARDED")
			continue

		case strings.HasPrefix(line, "INIT"):
			if err := ctx.readInit(line); err != nil {
				return nil, err
			}

		default:
			return nil, fmt.Errorf("unsupported log line: %q", line)
		}

		if err != nil {
			chunks := strings.SplitN(line, " ", 2)
			return nil, fmt.Errorf("%s: %s (line %q)", chunks[0], err, line)
		}
	}

	c.logger.Info("lines channel has been closed")
	return nil, io.EOF
}

func (c *ConsoleReader) ProcessData(reader io.Reader) error {
	scanner := c.buildScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		c.lines <- line
	}

	if scanner.Err() == nil {
		close(c.lines)
		return io.EOF
	}

	return scanner.Err()
}

func (c *ConsoleReader) buildScanner(reader io.Reader) *bufio.Scanner {
	buf := make([]byte, 50*1024*1024)
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(buf, 50*1024*1024)

	return scanner
}

func (ctx *parseCtx) pushCallIndex(index int32) {
	ctx.evmCallStackIndexes = append(ctx.evmCallStackIndexes, index)
}

func (ctx *parseCtx) popCallIndexReturnParent() (int32, uint32, error) {
	l := len(ctx.evmCallStackIndexes)
	if l == 0 {
		return 0, 0, fmt.Errorf("busted call stack, more pops than pushes")
	}

	ctx.evmCallStackIndexes = ctx.evmCallStackIndexes[:l-1]
	if l == 1 {
		return 0, 0, nil
	}
	return ctx.evmCallStackIndexes[l-2], uint32(l) - 1, nil
}

// Formats
// FIRE BEGIN_BLOCK <NUM>
func (ctx *parseCtx) readBeginBlock(line string) error {
	if BlockVersion == 0 {
		return fmt.Errorf("cannot start reading block: INIT not done")
	}
	chunks, err := SplitInChunks(line, 2)
	if err != nil {
		return fmt.Errorf("split: %s", err)
	}

	blockNum, err := strconv.ParseUint(chunks[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid block num: %s", err)
	}

	if l := len(ctx.transactionTraces); l != 0 {
		return fmt.Errorf("found %d leftover transactionTraces when starting block %d", l, blockNum)
	}

	ctx.stats = newParsingStats(ctx.logger, blockNum)
	ctx.currentBlock = &pbeth.Block{
		Number: blockNum,
		Ver:    BlockVersion,
	}

	return nil
}

// Formats
// FIRE BEGIN_APPLY_TRX <TRX_HASH> <TO> <VALUE> <V> <R> <S> <GAS> <GAS_PRICE> <NONCE> <input> <ACCESS_LIST> <MAX_FEE_PER_GAS> <MAX_PRIORITY_FEE_PER_GAS> <TRX_TYPE> <BEGIN_ORDINAL>

// FIRE BEGIN_APPLY_TRX 2becdee3b9ce9dd9a7274b8f6881e8e8d119ab046502ea90688773ef545731c7 929bc44bbd41ca0e621dc50f7c7e3204ce026258 . 0bf9 2b0d6434d98be6858c367710fff02ed69b86c0b1188a737d3ca55507f363928c 691797966e89cbb83dd45c2b6685ca1dc3734ee0c58ae8ffd935c9823b051e9f 300000 0ba43b7400 58 41c0e1b5 . 0 1

func (ctx *parseCtx) readApplyTrxBegin(line string) error {
	if ctx.currentTrace != nil {
		return fmt.Errorf("received when trx already begun")
	}

	chunks, err := SplitInChunks(line, 16)
	if err != nil {
		return fmt.Errorf("split: %s", err)
	}

	hash := FromHex(chunks[0], "BEGIN_APPLY_TRX txHash")
	to := FromHex(chunks[1], "BEGIN_APPLY_TRX to")
	value := pbeth.BigIntFromBytes(FromHex(chunks[2], "BEGIN_APPLY_TRX value"))

	v := FromHex(chunks[3], "BEGIN_APPLY_TRX v")
	r := FromHex(chunks[4], "BEGIN_APPLY_TRX r")
	s := FromHex(chunks[5], "BEGIN_APPLY_TRX s")
	gas := FromUint64(chunks[6], "BEGIN_APPLY_TRX gas")
	gasPrice := pbeth.BigIntFromBytes(FromHex(chunks[7], "BEGIN_APPLY_TRX gasPrice"))
	nonce := FromUint64(chunks[8], "BEGIN_APPLY_TRX nonce")
	input := FromHex(chunks[9], "BEGIN_APPLY_TRX input")
	accessList, err := decodeAccessList(FromHex(chunks[10], "BEGIN_APPLY_TRX accessList"))
	if err != nil {
		return fmt.Errorf("invalid access list value %q: %w", chunks[10], err)
	}

	maxFeePerGas := pbeth.BigIntFromBytes(FromHex(chunks[11], "BEGIN_APPLY_TRX maxFeePerGas"))
	maxPriorityGasFee := pbeth.BigIntFromBytes(FromHex(chunks[12], "BEGIN_APPLY_TRX maxPriorityFeePerGas"))
	trxType := pbeth.TransactionTrace_Type(FromInt32(chunks[13], "BEGIN_APPLY_TRX trxType"))
	ordinal := FromUint64(chunks[14], "BEGIN_APPLY_TRX ordinal@12")

	ctx.currentTraceLogCount = 0
	ctx.currentTrace = &pbeth.TransactionTrace{
		Index:                uint32(len(ctx.transactionTraces)),
		Hash:                 hash,
		Value:                value,
		V:                    v,
		R:                    pbeth.NormalizeSignaturePoint(r),
		S:                    pbeth.NormalizeSignaturePoint(s),
		GasLimit:             gas,
		GasPrice:             gasPrice,
		Nonce:                nonce,
		Input:                input,
		AccessList:           accessList,
		MaxFeePerGas:         maxFeePerGas,
		MaxPriorityFeePerGas: maxPriorityGasFee,
		Type:                 trxType,
		BeginOrdinal:         ordinal,
	}

	// A contract creation will have the `to` being null. In such case,
	// we fill up the information at a later stage extracting it from contextual logs.
	if to != nil {
		ctx.currentTrace.To = to
	}

	ctx.currentRootCall = &pbeth.Call{
		// We don't know yet its real type, so put CALL and it will be resolved to its final value later on.
		// Using CALL is important because genesis block generates a dummy transaction without a call and
		// it must be of type CALL.
		CallType: pbeth.CallType_CALL,
		Index:    1,
		Address:  to,
		Value:    value,
	}
	ctx.currentTrace.Calls = append(ctx.currentTrace.Calls, ctx.currentRootCall)

	return nil
}

func decodeAccessList(b []byte) (out []*pbeth.AccessTuple, err error) {
	elementCount, byteRead := binary.Uvarint(b)
	if byteRead == 0 {
		return nil, fmt.Errorf("read access tuple length: not enough bytes left, there is only %d bytes", len(b))
	}

	if byteRead < 0 {
		return nil, fmt.Errorf("read access tuple length: value is bigger than 64 bits")
	}

	b = b[byteRead:]

	out = make([]*pbeth.AccessTuple, elementCount)
	for i := 0; i < int(elementCount); i++ {
		out[i] = &pbeth.AccessTuple{}

		if len(b) < 20 {
			return nil, fmt.Errorf("access list at index %d: read contract address: not enough bytes left, expected at least 20 bytes but there is only %d bytes", i, len(b))
		}

		out[i].Address = b[0:20]
		b = b[20:]

		storageKeyCount, byteRead := binary.Uvarint(b)
		if byteRead == 0 {
			return nil, fmt.Errorf("access list at index %d: read storage key length: not enough bytes left, got %d", i, len(b))
		}

		if byteRead < 0 {
			return nil, fmt.Errorf("access list at index %d: read storage key length: value is bigger than 64 bits", i)
		}

		out[i].StorageKeys = make([][]byte, storageKeyCount)
		b = b[byteRead:]

		for j := uint64(0); j < storageKeyCount; j++ {
			if len(b) < 32 {
				return nil, fmt.Errorf("access list at index %d: read straoge key: not enough bytes left, expected at least 32 bytes but there is only %d bytes", i, len(b))
			}

			out[i].StorageKeys[j] = b[0:32]
			b = b[32:]
		}
	}

	if len(b) != 0 {
		return nil, fmt.Errorf("access list buffer not completely consumed, there is still %d bytes left", len(b))
	}

	return
}

// Formats
// FIRE EVM_RUN_CALL CALL 4 6
func (ctx *parseCtx) readEVMRunCall(line string) error {
	if ctx.currentTrace == nil {
		return fmt.Errorf("no transaction started")
	}

	chunks, err := SplitInChunks(line, 4)
	if err != nil {
		return fmt.Errorf("split: %s", err)
	}

	rawCallType := chunks[0] //CALL
	callType := pbeth.CallType(pbeth.CallType_value[rawCallType])
	if callType == 0 {
		return fmt.Errorf("invalid call type %q", rawCallType)
	}

	index := FromInt32(chunks[1], "EVM_RUN_CALL index") //4
	ordinal := FromUint64(chunks[2], "EVM_RUN_CALL ordinal")

	ctx.pushCallIndex(index)

	if index == 1 {
		ctx.currentRootCall.CallType = callType
		return nil
	}

	if int(index-1) != len(ctx.currentTrace.Calls) {
		return fmt.Errorf("index (%d - 1) doesn't match the number of calls on the stack (%d)", index, len(ctx.currentTrace.Calls))
	}

	ctx.currentTrace.Calls = append(ctx.currentTrace.Calls, &pbeth.Call{
		Index:        uint32(index),
		CallType:     callType,
		BeginOrdinal: ordinal,
	})

	return nil
}

// Formats
// FIRE EVM_PARAM CALL 4 a63e668919f50a591f5a23fb77881a347d10c081 0000000000000000000000000000000000003003 defd 2300 .
func (ctx *parseCtx) readEVMParamCall(line string) error {
	if ctx.currentTrace == nil {
		return fmt.Errorf("no transaction started")
	}

	chunks, err := SplitInChunks(line, 8)
	if err != nil {
		return fmt.Errorf("split: %s", err)
	}

	rawCallType := chunks[0] //CALL
	callType := pbeth.CallType(pbeth.CallType_value[rawCallType])
	if callType == 0 {
		return fmt.Errorf("invalid call type %q", rawCallType)
	}
	indexStr := chunks[1]

	evmCall, err := ctx.getCall(indexStr, false, "EVM_PARAM")
	if err != nil {
		return err
	}

	caller := FromHex(chunks[2], "EVM_RUN_CALL caller")
	contractAddress := FromHex(chunks[3], "EVM_RUN_CALL contractAddress")
	value := pbeth.BigIntFromBytes(FromHex(chunks[4], "EVM_RUN_CALL value"))
	gas := FromUint64(chunks[5], "EVM_RUN_CALL gas")
	input := FromHex(chunks[6], "EVM_RUN_CALL input")

	evmCall.Caller = caller
	evmCall.Address = contractAddress
	evmCall.Value = value
	evmCall.GasLimit = gas
	evmCall.Input = input

	// If call type is not a CREATE and `input != []` we assume this call will execute code. Later on, when
	// we see the `ACCOUNT_WITHOUT_CODE` message, we put it to `false` regardless of here since it's impossible
	// for an account without code to execute the `input`.
	evmCall.ExecutedCode = callType != pbeth.CallType_CREATE && len(input) > 0

	return nil
}

// Formats
// FIRE EVM_CALL_FAILED <CALL_INDEX> <GAS_LEFT> <REASON>
func (ctx *parseCtx) readEVMCallFailed(line string) error {
	if ctx.currentTrace == nil {
		return fmt.Errorf("no transaction started")
	}

	chunks, err := SplitInBoundedChunks(line, 4)
	if err != nil {
		return fmt.Errorf("split: %s", err)
	}

	evmCall, err := ctx.getCall(chunks[0], false, "EVM_CALL_FAILED")
	if err != nil {
		return err
	}

	gasLeft := FromUint64(chunks[1], "EVM_CALL_FAILED gasLeft")
	failureReason := chunks[2]

	// FIXME: This would be overwitten by endCall below, check if
	//        we need to make endCall aware of failure/revert and
	//        act accordingly on gas consumed.
	evmCall.GasConsumed = evmCall.GasLimit - gasLeft
	evmCall.StatusFailed = true
	evmCall.FailureReason = failureReason

	return nil
}

// Formats
// FIRE EVM_REVERTED <CALL_INDEX>
func (ctx *parseCtx) readEVMReverted(line string) error {
	chunks, err := SplitInChunks(line, 2)
	if err != nil {
		return fmt.Errorf("split: %s", err)
	}

	evmCall, err := ctx.getCall(chunks[0], false, "EVM_REVERTED")
	if err != nil {
		return err
	}

	evmCall.StatusReverted = true

	return nil
}

// Formats
// FIRE EVM_END_CALL <CALL_INDEX> <GAS_LEFT> <RETURN_VALUE> <ORDINAL>
func (ctx *parseCtx) readEVMEndCall(line string) error {
	chunks, err := SplitInChunks(line, 5)
	if err != nil {
		return fmt.Errorf("split: %s", err)
	}

	evmCall, err := ctx.getCall(chunks[0], false, "EVM_END_CALL")
	if err != nil {
		return err
	}

	gasLeft := FromUint64(chunks[1], "EVM_END_CALL gasLeft")
	ordinal := FromUint64(chunks[3], "EVM_END_CALL ordinal")

	parentIndex, depth, err := ctx.popCallIndexReturnParent()
	if err != nil {
		return err
	}

	// TODO: Add a check to ensure this always results in a valid gas value (i.e. no overflow)
	evmCall.GasConsumed = evmCall.GasLimit - gasLeft
	evmCall.ReturnData = FromHex(chunks[2], "EVM_END_CALL returnData")
	evmCall.ParentIndex = uint32(parentIndex)
	evmCall.Depth = depth
	evmCall.EndOrdinal = ordinal

	return nil
}

// Formats
// FIRE SKIPPED_TRX <REASON>
func (ctx *parseCtx) readSkippedTrx(line string) error {
	if ctx.currentBlock == nil {
		return fmt.Errorf("no block started")
	}
	if ctx.currentTrace == nil {
		return fmt.Errorf("no transaction started")
	}

	// TODO: handle reason?

	ctx.currentTrace = nil
	return nil
}

// Formats
// FIRE END_APPLY_TRX <STATE_ROOT> <CUMULATIVE_GAS_USED> <LOGS_BLOOM> <ORDINAL> { []<deth.Log> }
func (ctx *parseCtx) readApplyTrxEnd(line string) error {
	if ctx.currentTrace == nil {
		return fmt.Errorf("no matching BEGIN_APPLY_TRX")
	}

	trxTrace := ctx.currentTrace

	chunks, err := SplitInBoundedChunks(line, 7)
	if err != nil {
		return fmt.Errorf("split: %s", err)
	}

	gasUsed := FromUint64(chunks[0], "END_APPLY_TRX gasUsed")
	stateRoot := FromHex(chunks[1], "END_APPLY_TRX stateRoot")
	cumulativeGasUsed := FromUint64(chunks[2], "END_APPLY_TRX cumulativeGasUsed")
	logsBloom := FromHex(chunks[3], "END_APPLY_TRX logsBloom")
	ordinal := FromUint64(chunks[4], "END_APPLY_TRX ordinal")

	var logs []*Log
	if err := json.Unmarshal([]byte(chunks[5]), &logs); err != nil {
		return err
	}

	trxTrace.GasUsed = gasUsed
	trxTrace.Receipt = &pbeth.TransactionReceipt{
		StateRoot:         stateRoot,
		CumulativeGasUsed: cumulativeGasUsed,
		LogsBloom:         logsBloom,
	}

	trxTrace.EndOrdinal = ordinal

	var pbLogs []*pbeth.Log
	for i, l := range logs {
		log := &pbeth.Log{
			Index:   uint32(i),
			Address: l.Address,
			Data:    l.Data,
			Topics:  make([][]byte, len(l.Topics)),
		}
		for i, t := range l.Topics {
			log.Topics[i] = t
		}

		pbLogs = append(pbLogs, log)
	}

	if len(trxTrace.To) == 0 {
		if trxTrace.Calls[0].CallType == pbeth.CallType_CREATE {
			trxTrace.To = trxTrace.Calls[0].Address
		} else {
			panic(fmt.Errorf("trx hash %s in block %d has no `to` and none could be computed", hex.EncodeToString(trxTrace.Hash), ctx.currentBlock.Number))
		}
	}

	trxTrace.Receipt.Logs = pbLogs
	trxTrace.PopulateStateReverted()

	ctx.transactionTraces = append(ctx.transactionTraces, trxTrace)
	ctx.currentTrace = nil
	ctx.currentTraceLogCount = 0

	// reset top level for new transaction
	ctx.currentRootCall = nil

	return nil
}

// Formats
// FIRE FINALIZE_BLOCK <NUM>
func (ctx *parseCtx) readFinalizeBlock(line string) error {
	if ctx.currentBlock == nil {
		return fmt.Errorf("no block started")
	}

	chunks, err := SplitInChunks(line, 2)
	if err != nil {
		return fmt.Errorf("split: %s", err)
	}

	blockNum, err := strconv.ParseUint(chunks[0], 10, 64)
	if err != nil {
		return fmt.Errorf("failed to parse blockNum: %s", err)
	}

	if blockNum != ctx.currentBlock.Number {
		return fmt.Errorf("finalizing block does not match active block num, got block num %d but current is block num %d", blockNum, ctx.currentBlock.Number)
	}

	ctx.finalizing = true
	return nil
}

// Formats
// FIRE FAILED_APPLY_TRX transaction failure error message...
func (ctx *parseCtx) readFailedApplyTrx(line string) error {
	if ctx.currentBlock == nil {
		return fmt.Errorf("no block started")
	}
	if ctx.currentTrace == nil {
		return fmt.Errorf("no transaction started")
	}

	chunks, err := SplitInBoundedChunks(line, 2)
	if err != nil {
		return fmt.Errorf("split: %s", err)
	}

	fmt.Printf("FAILED trx %q at block %d (hash unavailable, probably forked): %s\n", hex.EncodeToString(ctx.currentTrace.Hash), ctx.currentBlock.Number, chunks[0])

	ctx.currentBlock = nil
	ctx.transactionTraces = nil
	ctx.currentTrace = nil
	ctx.currentTraceLogCount = 0
	ctx.finalizing = false

	return nil
}

// Formats
// FIRE CREATED_ACCOUNT 4 2af4f4790a71313e0c532072207a77f1e4c1baec 7
func (ctx *parseCtx) readCreateAccount(line string) error {
	chunks, err := SplitInChunks(line, 4)
	if err != nil {
		return fmt.Errorf("split: %s", err)
	}

	callIndex := chunks[0]
	account := FromHex(chunks[1], "CREATED_#")
	ordinal := FromUint64(chunks[2], "CREATED_ACCOUNT ordinal")

	accountCreation := &pbeth.AccountCreation{
		Account: account,
		Ordinal: ordinal,
	}

	if callIndex == "0" {
		if ctx.currentTrace != nil {
			// We have a trace active, so let's add it to it's root call
			ctx.currentRootCall.AccountCreations = append(ctx.currentRootCall.AccountCreations, accountCreation)
		}

		return nil
	}

	evmCall, err := ctx.getCall(callIndex, false, "CREATED_ACCOUNT")
	if err != nil {
		return err
	}

	evmCall.AccountCreations = append(evmCall.AccountCreations, accountCreation)
	return nil
}

/*
FIRE INIT 2.0 polygon 1.10.17-fh+hotfix (deadbeef) ...
*/
func (ctx *parseCtx) readInit(line string) error {
	chunks, err := SplitInBoundedChunks(line, 4)
	if err != nil {
		return fmt.Errorf("split: %s", err)
	}

	fhVersion := chunks[0]
	nodeVariant := chunks[1]
	nodeVersion := chunks[2]

	switch fhVersion {
	case "2.0":
		BlockVersion = 2
	case "2.1":
		BlockVersion = 3
	default:
		return fmt.Errorf("major version of Firehose exchange protocol is unsupported (expected: 2.0 or 2.1, found %s), you are most probably running an incompatible version of the Firehose instrumented 'geth' client", fhVersion)
	}

	ctx.logger.Info("read firehose instrumentation init line",
		zap.String("fh_version", fhVersion),
		zap.String("node_variant", nodeVariant),
		zap.String("node_version", nodeVersion),
	)

	return nil
}

// Format
// FIRE SUICIDE_CHANGE 1 c356a543cec92de8bf1e43a88d09e568e9d3aca3 false .
func (ctx *parseCtx) readSuicideChange(line string) error {
	chunks, err := SplitInChunks(line, 5)
	if err != nil {
		return fmt.Errorf("split: %s", err)
	}

	callIndex := chunks[0]

	if callIndex == "0" {
		return fmt.Errorf("SUICIDE_CHANGE is expected to always happen within a call boundary but just seen SUICIDE_CHANGE outside of a call for block #%d", ctx.currentBlock.Number)
	}

	evmCall, err := ctx.getCall(callIndex, false, "SUICIDE_CHANGE")
	if err != nil {
		return err
	}

	evmCall.Suicide = true

	return nil
}

// Format
// FIRE CODE_CHANGE 2 cb32e940a34b938f9cebe70313fe7e8ca3d23d36 c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470 . 89f3219c608c80bcbb274738ff7a325624cd54c9868b9d54bde369e5ab005bc6 6080604052600080fdfea165627a7a723058204a5d828a5772e67b2eaa10bd570ffa7d9607586e73576cc26299c24348dc64450029 8
// deepmind.Print("CODE_CHANGE", deepmind.CallIndex(), deepmind.Addr(s.address), deepmind.Hex(s.CodeHash()), deepmind.Hex(prevcode),
// deepmind.Hash(codeHash), deepmind.Hex(code), <ORDINAL>)
func (ctx *parseCtx) readCodeChange(line string) error {

	chunks, err := SplitInChunks(line, 8)
	if err != nil {
		return fmt.Errorf("split: %s", err)
	}

	callIndex := chunks[0]

	codeChange := &pbeth.CodeChange{
		Address: FromHex(chunks[1], "CODE_CHANGE address"),
		OldHash: FromHex(chunks[2], "CODE_CHANGE old_hash"),
		OldCode: FromHex(chunks[3], "CODE_CHANGE old_code"),
		NewHash: FromHex(chunks[4], "CODE_CHANGE new_hash"),
		NewCode: FromHex(chunks[5], "CODE_CHANGE new_code"),
		Ordinal: FromUint64(chunks[6], "CODE_CHANGE ordinal"),
	}

	if callIndex == "0" {
		// Handle the case where this is no transaction active, which means the CODE_CHANGE happened at the block level,
		// this happens on some network, for example BSC.
		if ctx.currentTrace == nil {
			if ctx.currentBlock != nil {
				ctx.currentBlock.CodeChanges = append(ctx.currentBlock.CodeChanges, codeChange)
			}
			return nil
		}

		// Genesis block produces a CODE_CHANGE at callIndex "0" which means the transaction root (e.g. outside a Call),
		// so we must generate an error that CODE_CHANGE was received at an unexpected location only block block that are
		// not the genesis block (e.g. block.number == 0).
		if ctx.currentBlock.Number != 0 {
			return fmt.Errorf("CODE_CHANGE is expected to always happen within a trace boundary but just seen CODE_CHANGE directly in block #%d (no active trace)", ctx.currentBlock.Number)
		}
	}

	evmCall, err := ctx.getCall(callIndex, true, "CODE_CHANGE")
	if err != nil {
		return err
	}

	evmCall.CodeChanges = append(evmCall.CodeChanges, codeChange)

	return nil
}

// Formats
// FIRE END_BLOCK <NUM> <SIZE> { header: <BlockHeader>, uncles: []<BlockHeader> }
func (ctx *parseCtx) readEndBlock(line string) (*bstream.Block, error) {
	if ctx.currentBlock == nil {
		return nil, fmt.Errorf("no block started")
	}
	if !ctx.finalizing {
		return nil, fmt.Errorf("block wasn't in finalizing mode")
	}

	chunks, err := SplitInBoundedChunks(line, 4)
	if err != nil {
		return nil, fmt.Errorf("split: %s", err)
	}

	blockNum, err := strconv.ParseUint(chunks[0], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse blockNum: %s", err)
	}

	if blockNum != ctx.currentBlock.Number {
		return nil, fmt.Errorf("end block does not match active block num, got block num %d but current is block num %d", blockNum, ctx.currentBlock.Number)
	}

	size, err := strconv.ParseUint(chunks[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse size: %s", err)
	}

	var endBlockData endBlockInfo
	if err := json.Unmarshal([]byte(chunks[2]), &endBlockData); err != nil {
		return nil, err
	}

	header := FromHeader(endBlockData.Header)
	if header.Number != ctx.currentBlock.Number {
		return nil, fmt.Errorf("header end block does not match active block num, got block num %d but current is block num %d", header.Number, ctx.currentBlock.Number)
	}
	header.TotalDifficulty = pbeth.BigIntFromBytes(endBlockData.TotalDifficulty)

	ctx.currentBlock.Size = size
	ctx.currentBlock.Hash = header.Hash

	ctx.currentBlock.Header = header
	for _, uncle := range endBlockData.Uncles {
		ctx.currentBlock.Uncles = append(ctx.currentBlock.Uncles, FromHeader(uncle))
	}

	ctx.currentBlock.TransactionTraces = ctx.transactionTraces

	ctx.globalStats.lastBlock = ctx.currentBlock.AsRef()
	ctx.globalStats.blockRate.Inc()
	ctx.globalStats.blockAverageParseTime.AddElapsedTime(ctx.stats.startAt)
	ctx.globalStats.transactionRate.IncBy(int64(len(ctx.transactionTraces)))

	block := ctx.currentBlock
	ctx.transactionTraces = nil
	ctx.currentBlock = nil
	ctx.finalizing = false
	ctx.stats.log()

	block.NormalizeInPlace()

	var libNum uint64
	if len(endBlockData.FinalizedBlockHash) > 0 {
		libNum = computeProofOfStakeLIBNum(blockNum, uint64(endBlockData.FinalizedBlockNum), bstream.GetProtocolFirstStreamableBlock)
	} else {
		libNum = computeProofOfWorkLIBNum(block.Number, bstream.GetProtocolFirstStreamableBlock)
	}

	return types.BlockFromProto(block, libNum)
}

func computeProofOfWorkLIBNum(blockNum uint64, firstStreamableBlockNum uint64) uint64 {
	if blockNum <= firstStreamableBlockNum+200 {
		return firstStreamableBlockNum
	}

	return blockNum - 200
}

func computeProofOfStakeLIBNum(blockNum uint64, finalizedBlockNum uint64, firstStreamableBlockNum uint64) uint64 {
	if blockNum <= firstStreamableBlockNum {
		return firstStreamableBlockNum
	}

	// In normal circumstances, we would received something like Block #2500 (Finalized #2400) (e.g. finalized
	// is before/< than block). When doing big reprocessing from an already synced beacon node, you might receive
	// actually Block #2500 (Finalized #5400) (e.g. finalized is after/> than block).
	//
	// When reprocessing and finalized block is after block, we assume block itself is now the LIB num
	if finalizedBlockNum >= blockNum {
		return blockNum
	}

	// Otherwise, finalized block is before block so it's the lib num
	return finalizedBlockNum
}

// Formats
// FIRE STORAGE_CHANGE <CALL_INDEX> <CONTRACT_ADDRESSS> <KEY> <OLD_VALUE> <NEW_VALUE> <ORDINAL>
func (ctx *parseCtx) readStorageChange(line string) error {
	chunks, err := SplitInChunks(line, 7)
	if err != nil {
		return fmt.Errorf("split: %s", err)
	}

	callIndex := chunks[0]
	if callIndex == "0" {
		if ctx.currentBlock == nil || ctx.currentTrace == nil {
			// FIXME: Fow now, let's just skip them, but maybe we should store them at the block level?
			return nil
		}
	}

	evmCall, err := ctx.getCall(callIndex, true, "STORAGE_CHANGE")
	if err != nil {
		return err
	}

	evmCall.StorageChanges = append(evmCall.StorageChanges, &pbeth.StorageChange{
		Address:  FromHex(chunks[1], "STORAGE_CHANGE address"),
		Key:      FromHex(chunks[2], "STORAGE_CHANGE key"),
		OldValue: FromHex(chunks[3], "STORAGE_CHANGE oldValue"),
		NewValue: FromHex(chunks[4], "STORAGE_CHANGE newValue"),
		Ordinal:  FromUint64(chunks[5], "STORAGE_CHANGE ordinal"),
	})

	return nil
}

// Formats
// FIRE BALANCE_CHANGE <CALL_INDEX> <ADDRESSS> <OLD_VALUE> <NEW_VALUE> <REASON> <ORDINAL>
func (ctx *parseCtx) readBalanceChange(line string) error {
	chunks, err := SplitInChunks(line, 7)
	if err != nil {
		return fmt.Errorf("split: %s", err)
	}

	callIndex := chunks[0]

	balanceChange := &pbeth.BalanceChange{
		Address:  FromHex(chunks[1], "BALANCE_CHANGE address"),
		OldValue: pbeth.BigIntFromBytes(FromHex(chunks[2], "BALANCE_CHANGE oldValue")),
		NewValue: pbeth.BigIntFromBytes(FromHex(chunks[3], "BALANCE_CHANGE newValue")),
		Reason:   pbeth.MustBalanceChangeReasonFromString(chunks[4]),
		Ordinal:  FromUint64(chunks[5], "BALANCE_CHANGE ordinal"),
	}

	if ctx.currentTrace == nil && ctx.currentBlock != nil {
		// This is temporary until reason why the `callIndex != "0"` happens, should be fixed now, but quite possible we still have a problem
		ctx.currentBlock.BalanceChanges = append(ctx.currentBlock.BalanceChanges, balanceChange)
		return nil
	}

	if callIndex == "0" {
		if ctx.currentTrace != nil {
			// We have a trace active, so let's add it to it's root call
			ctx.currentRootCall.BalanceChanges = append(ctx.currentRootCall.BalanceChanges, balanceChange)
			return nil
		}

		if ctx.currentBlock != nil {
			// We have no trace active but a block is, so let's add it to the block balance changes
			ctx.currentBlock.BalanceChanges = append(ctx.currentBlock.BalanceChanges, balanceChange)
			return nil
		}

		return nil
	}

	evmCall, err := ctx.getCall(callIndex, false, "BALANCE_CHANGE")
	if err != nil && (balanceChange.Reason == pbeth.BalanceChange_REASON_REWARD_MINE_BLOCK || balanceChange.Reason == pbeth.BalanceChange_REASON_REWARD_MINE_UNCLE) {
		ctx.logger.Warn("Skipping balance change that we cannot link to a transaction, something is broken but is temporary to overcome the problem")
		return nil
	}

	if err != nil {
		return err
	}

	evmCall.BalanceChanges = append(evmCall.BalanceChanges, balanceChange)

	return nil
}

// Formats
// FIRE GAS_CHANGE <CALL_INDEX> <OLD_VALUE> <NEW_VALUE> <REASON> <ORDINAL>
func (ctx *parseCtx) readGasChange(line string) error {
	chunks, err := SplitInChunks(line, 6)
	if err != nil {
		return fmt.Errorf("split: %s", err)
	}

	callIndex := chunks[0]

	gasChange := &pbeth.GasChange{
		OldValue: FromUint64(chunks[1], "GAS_CHANGE OldValue"),
		NewValue: FromUint64(chunks[2], "GAS_CHANGE NewValue"),
		Reason:   pbeth.MustGasChangeReasonFromString(chunks[3]),
		Ordinal:  FromUint64(chunks[4], "GAS_CHANGE ordinal"),
	}

	if callIndex == "0" {
		if ctx.currentTrace != nil {
			// We have a trace active, so let's add it to it's root call
			ctx.currentRootCall.GasChanges = append(ctx.currentRootCall.GasChanges, gasChange)
			return nil
		}

		// We simply ignore those, does not make sens in the context of gas change to have it on block level
		return nil
	}

	evmCall, err := ctx.getCall(callIndex, false, "GAS_CHANGE")
	if err != nil {
		return err
	}

	evmCall.GasChanges = append(evmCall.GasChanges, gasChange)

	return nil
}

// Formats
// FIRE NONCE_CHANGE <CALL_INDEX> <ADDRESS> <OLD_VALUE> <NEW_VALUE> <ORDINAL
func (ctx *parseCtx) readNonceChange(line string) error {
	chunks, err := SplitInChunks(line, 6)
	if err != nil {
		return fmt.Errorf("split: %s", err)
	}

	callIndex := chunks[0]
	nonceChange := &pbeth.NonceChange{
		Address:  FromHex(chunks[1], "NONCE_CHANGE address"),
		OldValue: FromUint64(chunks[2], "NONCE_CHANGE OldValue"),
		NewValue: FromUint64(chunks[3], "NONCE_CHANGE NewValue"),
		Ordinal:  FromUint64(chunks[4], "NONCE_CHANGE ordinal"),
	}

	if callIndex == "0" {
		if ctx.currentTrace != nil {
			// We have a trace active, so let's add it to it's root call
			ctx.currentRootCall.NonceChanges = append(ctx.currentRootCall.NonceChanges, nonceChange)
			return nil
		}

		// We simply ignore those, does not make sens in the context of gas change to have it on block level
		return nil
	}

	evmCall, err := ctx.getCall(callIndex, false, "NONCE_CHANGE")
	if err != nil {
		return err
	}

	evmCall.NonceChanges = append(evmCall.NonceChanges, nonceChange)

	return nil
}

// Formats
// FIRE EVM_KECCAK <CALL_INDEX> <HASH_RESULT> <HASH_INPUT>
func (ctx *parseCtx) readEVMKeccak(line string) error {
	chunks, err := SplitInChunks(line, 4)
	if err != nil {
		return fmt.Errorf("split: %s", err)
	}

	evmCall, err := ctx.getCall(chunks[0], false, "EVM_KECCACK")
	if err != nil {
		return err
	}

	// THOUGHTS: KeccakPreimages is a map[string]string to ease navigation, as the idea is
	//           to walk down the hashed value with it's preimage and do that recursively
	//           in the map to find the original key. As such, it's much easier if each element
	//           is of the same representation.
	//
	//           This is at the expense of storage cost as we store information in a less compact
	//           way know. Would need to see if the storage is really that much decreased when
	//           stored as map[[]byte][]byte (is that possible in Golang and in Protobuf?).
	if evmCall.KeccakPreimages == nil {
		evmCall.KeccakPreimages = make(map[string]string)
	}

	evmCall.KeccakPreimages[chunks[1]] = chunks[2]

	return nil
}

// Formats
// FIRE TRX_FROM <ADDRESS>
func (ctx *parseCtx) readTrxFrom(line string) error {
	chunks, err := SplitInChunks(line, 2)
	if err != nil {
		return fmt.Errorf("split: %s", err)
	}

	if ctx.currentTrace == nil {
		return fmt.Errorf("no matching BEGIN_APPLY_TRX")
	}

	ctx.currentTrace.From = FromHex(chunks[0], "TRX_FROM from")

	if len(ctx.currentTrace.Calls) == 1 && len(ctx.currentRootCall.Caller) == 0 {
		ctx.currentRootCall.Caller = ctx.currentTrace.From
	}
	return nil
}

// Formats
// FIRE ACCOUNT_WITHOUT_CODE <CALL_INDEX>
func (ctx *parseCtx) readAccountWithoutCode(line string) error {
	chunks, err := SplitInChunks(line, 2)
	if err != nil {
		return fmt.Errorf("split: %s", err)
	}

	evmCall, err := ctx.getCall(chunks[0], false, "ACCOUNT_WITHOUT_CODE")
	if err != nil {
		return err
	}

	evmCall.ExecutedCode = false
	return nil
}

// Formats
// FIRE ADD_LOG <CALL_INDEX> <BLOCK_INDEX> <CONTRACT_ADDRESS> <TOPICS> <DATA> <ORDINAL>
func (ctx *parseCtx) readAddLog(line string) error {
	chunks, err := SplitInChunks(line, 7)
	if err != nil {
		return fmt.Errorf("split: %s", err)
	}

	if ctx.currentTrace == nil {
		return fmt.Errorf("no previous transaction context")
	}

	callIndex := chunks[0]
	blockIndex, err := strconv.ParseInt(chunks[1], 10, 32)
	if err != nil {
		return err
	}

	address := FromHex(chunks[2], "ADD_LOG address")
	topicStrings := strings.Split(chunks[3], ",")

	topics := make([][]byte, len(topicStrings))
	for i, topicString := range topicStrings {
		topics[i] = FromHex(topicString, fmt.Sprintf("TOPIC %d", i))
	}
	data := FromHex(chunks[4], "ADD_LOG data")

	ordinal := FromUint64(chunks[5], "ADD_LOG ordinal")

	var evmCall *pbeth.Call
	if callIndex == "0" {
		// We have a trace active, so let's add it to it's root call
		evmCall = ctx.currentRootCall
	} else {
		evmCall, err = ctx.getCall(callIndex, false, "ADD_LOG")
		if err != nil {
			return err
		}
	}

	logIndex := ctx.currentTraceLogCount
	ctx.currentTraceLogCount++

	evmCall.Logs = append(evmCall.Logs, &pbeth.Log{
		Address:    address,
		Index:      uint32(logIndex),
		BlockIndex: uint32(blockIndex),
		Data:       data,
		Topics:     topics,
		Ordinal:    ordinal,
	})

	return nil
}

// getCall returns the Call from the call stack, by index, if the `allowRoot`
// value is `true` and the `index` is 0, the currentTrace.rootCall is returned otherwise
// if it's `false`m when index 0 is encountered, an error is returned.
func (ctx *parseCtx) getCall(indexString string, allowRoot bool, tag string) (*pbeth.Call, error) {
	if ctx.currentTrace == nil {
		return nil, fmt.Errorf("no previous transaction context")
	}

	index, err := strconv.ParseInt(indexString, 10, 32)
	if err != nil {
		return nil, err
	}

	idx := int(index)
	if allowRoot && index == 0 {
		return ctx.currentRootCall, nil
	}

	if idx <= 0 || idx > len(ctx.currentTrace.Calls) {
		return nil, fmt.Errorf("%s call %s doesn't exist, evm call stack depth is %d", tag, indexString, len(ctx.currentTrace.Calls))
	}

	return ctx.currentTrace.Calls[idx-1], nil
}

// splitInChunks split the line in chunks and returns the slice `chunks[1:]`, but verifies
// that there are only exactly one of `validCounts` number of chunks
func SplitInChunks(line string, validCounts ...int) ([]string, error) {
	chunks := strings.SplitN(line, " ", -1)

	var valid bool
	for _, c := range validCounts {
		if len(chunks) == c {
			valid = true
			break
		}
	}
	if !valid {
		return nil, fmt.Errorf("one of %v fields required but found %d fields for line %q", validCounts, len(chunks), line)
	}

	return chunks[1:], nil
}

// splitInBoundedChunks split the line in `count` chunks and returns the slice `chunks[1:count]` (so exclusive end),
// but will accumulate all trailing chunks within the last (for free-form strings, or JSON objects)
func SplitInBoundedChunks(line string, count int) ([]string, error) {
	chunks := strings.SplitN(line, " ", count)
	if len(chunks) != count {
		return nil, fmt.Errorf("%d fields required but found %d fields for line %q", count, len(chunks), line)
	}

	return chunks[1:count], nil
}

func Has0xPrefix(input string) bool {
	return len(input) >= 2 && input[0] == '0' && (input[1] == 'x' || input[1] == 'X')
}
