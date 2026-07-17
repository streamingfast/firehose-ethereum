package codec

import (
	"io"
	"testing"

	firecore "github.com/streamingfast/firehose-core"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// readAllBlocksFromLines feeds raw console lines to a ConsoleReader and
// returns every Ethereum block it produces, decoded from the bstream payload.
func readAllBlocksFromLines(t *testing.T, lines []string) []*pbeth.Block {
	t.Helper()

	ch := make(chan string, len(lines))
	for _, line := range lines {
		ch <- line
	}
	close(ch)

	consoleReader, err := NewConsoleReader(ch, firecore.NewBlockEncoder(), zap.NewNop(), nil)
	require.NoError(t, err)

	reader := consoleReader.(*ConsoleReader)
	defer reader.Close()

	var out []*pbeth.Block
	for {
		blk, err := reader.ReadBlock()
		if err == io.EOF {
			return out
		}
		require.NoError(t, err)

		ethBlock := &pbeth.Block{}
		require.NoError(t, blk.Payload.UnmarshalTo(ethBlock))
		out = append(out, ethBlock)
	}
}

const testInitLine = "FIRE INIT 2.2 geth 1.10.17-fh2.2"

// beginApplyTrxLine returns a minimal valid BEGIN_APPLY_TRX line (fh 2.2 format).
func beginApplyTrxLine(hash string, ordinal string) string {
	return "FIRE BEGIN_APPLY_TRX " + hash +
		" a63e668919f50a591f5a23fb77881a347d10c081" + // to
		" 01" + // value
		" 1b 0101 0202" + // v r s
		" 21000" + // gas
		" 01" + // gas price
		" 0" + // nonce
		" ." + // input
		" 00" + // access list (0 entries)
		" . ." + // maxFeePerGas maxPriorityFeePerGas
		" 0 " + // trx type
		ordinal
}

func endBlockLine(num string) string {
	// only works for block numbers 0-9 (decimal == hexadecimal)
	return "FIRE END_BLOCK " + num + ` 500 {"header":{"number":"0x` + num + `","hash":"0x` + repeatHexByte("aa", 32) + `","parentHash":"0x` + repeatHexByte("bb", 32) + `","timestamp":"0x1"}}`
}

func repeatHexByte(b string, count int) (out string) {
	for i := 0; i < count; i++ {
		out += b
	}
	return
}

func TestConsoleReader_FailedApplyTrxDoesNotLeakSystemCallsIntoNextBlock(t *testing.T) {
	trxHash := repeatHexByte("cc", 32)

	lines := []string{
		testInitLine,

		// Block 1: a system call (e.g. EIP-4788 beacon root) executes, then the
		// block is aborted mid-transaction by FAILED_APPLY_TRX.
		"FIRE BEGIN_BLOCK 1",
		"FIRE SYSTEM_CALL_START",
		"FIRE EVM_RUN_CALL CALL 1 10",
		"FIRE EVM_END_CALL 1 0 . 11",
		"FIRE SYSTEM_CALL_END",
		beginApplyTrxLine(trxHash, "12"),
		"FIRE FAILED_APPLY_TRX invalid transaction: wrong chain id",

		// Block 2: clean block, no system call at all.
		"FIRE BEGIN_BLOCK 2",
		beginApplyTrxLine(trxHash, "20"),
		"FIRE EVM_RUN_CALL CALL 1 21",
		"FIRE EVM_END_CALL 1 0 . 22",
		"FIRE END_APPLY_TRX 21000 . 21000 . 23 []",
		"FIRE FINALIZE_BLOCK 2",
		endBlockLine("2"),
	}

	blocks := readAllBlocksFromLines(t, lines)
	require.Len(t, blocks, 1)

	block := blocks[0]
	require.Equal(t, uint64(2), block.Number)
	require.Len(t, block.TransactionTraces, 1)

	// The system call belonged to aborted block 1, it must not leak into block 2
	assert.Empty(t, block.SystemCalls, "system calls from the aborted block must not leak into the next block")
}

func TestConsoleReader_CancelBlockResetsEVMCallStack(t *testing.T) {
	trxHash := repeatHexByte("cc", 32)

	lines := []string{
		testInitLine,

		// Block 1: a transaction with two nested calls still running (no
		// EVM_END_CALL yet) when the block is cancelled mid-transaction.
		"FIRE BEGIN_BLOCK 1",
		beginApplyTrxLine(trxHash, "10"),
		"FIRE EVM_RUN_CALL CALL 1 11",
		"FIRE EVM_RUN_CALL CALL 2 12",
		"FIRE CANCEL_BLOCK 1 failing validation",

		// Block 2: clean transaction with a root call and one nested call.
		"FIRE BEGIN_BLOCK 2",
		beginApplyTrxLine(trxHash, "20"),
		"FIRE EVM_RUN_CALL CALL 1 21",
		"FIRE EVM_RUN_CALL CALL 2 22",
		"FIRE EVM_END_CALL 2 0 . 23",
		"FIRE EVM_END_CALL 1 0 . 24",
		"FIRE END_APPLY_TRX 21000 . 21000 . 25 []",
		"FIRE FINALIZE_BLOCK 2",
		endBlockLine("2"),
	}

	blocks := readAllBlocksFromLines(t, lines)
	require.Len(t, blocks, 1)

	block := blocks[0]
	require.Equal(t, uint64(2), block.Number)
	require.Len(t, block.TransactionTraces, 1)

	calls := block.TransactionTraces[0].Calls
	require.Len(t, calls, 2)

	rootCall := calls[0]
	assert.Equal(t, uint32(1), rootCall.Index)
	assert.Equal(t, uint32(0), rootCall.ParentIndex, "root call must have no parent")
	assert.Equal(t, uint32(0), rootCall.Depth, "root call must be at depth 0")

	nestedCall := calls[1]
	assert.Equal(t, uint32(2), nestedCall.Index)
	assert.Equal(t, uint32(1), nestedCall.ParentIndex, "nested call's parent must be the root call")
	assert.Equal(t, uint32(1), nestedCall.Depth, "nested call must be at depth 1")
}
