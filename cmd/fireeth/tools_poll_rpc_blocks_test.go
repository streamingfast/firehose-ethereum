package main

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/streamingfast/firehose-ethereum/codec"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestFireBlockLineParsedByConsoleReader(t *testing.T) {
	blockTime := time.Unix(1700000000, 123456789).UTC()
	blockHash := []byte{0xab, 0xcd, 0xef, 0x01}
	parentHash := []byte{0x12, 0x34, 0x56, 0x78}

	ethBlock := &pbeth.Block{
		Hash:   blockHash,
		Number: 42,
		Ver:    3,
		Header: &pbeth.BlockHeader{
			ParentHash: parentHash,
			Timestamp:  timestamppb.New(blockTime),
		},
	}

	line, err := fireBlockLine(ethBlock)
	require.NoError(t, err)

	lines := make(chan string, 2)
	lines <- "FIRE INIT 3.0 local v1.0.0"
	lines <- line
	close(lines)

	reader, err := codec.NewConsoleReader(lines, nil, zap.NewNop(), nil)
	require.NoError(t, err)

	block, err := reader.ReadBlock()
	require.NoError(t, err)

	require.Equal(t, uint64(42), block.Number)
	require.Equal(t, hex.EncodeToString(blockHash), block.Id)
	require.Equal(t, uint64(41), block.ParentNum)
	require.Equal(t, hex.EncodeToString(parentHash), block.ParentId)
	require.Equal(t, uint64(41), block.LibNum)
	require.Equal(t, blockTime.UnixNano(), block.Timestamp.AsTime().UnixNano())

	// payload round-trips back to the original eth block
	parsed := &pbeth.Block{}
	require.NoError(t, block.Payload.UnmarshalTo(parsed))
	require.Equal(t, uint64(42), parsed.Number)
	require.Equal(t, blockHash, parsed.Hash)
	require.Equal(t, parentHash, parsed.Header.ParentHash)
}
