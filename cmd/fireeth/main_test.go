package main

import (
	"bytes"
	"testing"

	"github.com/streamingfast/bstream"
	firecore "github.com/streamingfast/firehose-core"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"github.com/test-go/testify/require"
)

func Test_Encode_Decode_Block(t *testing.T) {
	Chain().Validate()
	Chain().Init()

	pbBlock, err := Chain().BlockEncoder.Encode(firecore.BlockEnveloppe{Block: &pbeth.Block{
		Number: 1,
		Header: &pbeth.BlockHeader{},
		Ver:    1,
	}, LIBNum: 0})
	require.NoError(t, err)

	require.Equal(t, uint64(1), pbBlock.Number)

	buffer := bytes.NewBuffer(nil)
	writer, err := bstream.NewDBinBlockWriter(buffer)
	require.NoError(t, err)

	require.NoError(t, writer.Write(pbBlock))

	reader, err := bstream.NewDBinBlockReader(buffer)
	require.NoError(t, err)

	hydrated, err := reader.Read()
	require.NoError(t, err)

	require.Equal(t, uint64(1), hydrated.Number)
}
