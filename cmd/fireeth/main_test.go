package main

import (
	"bytes"
	"testing"

	"github.com/streamingfast/bstream"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"github.com/test-go/testify/require"
)

func Test_Encode_Decode_Block(t *testing.T) {
	Chain.Validate()
	Chain.Init()

	original, err := Chain.BlockEncoder.Encode(&pbeth.Block{
		Number: 1,
		Header: &pbeth.BlockHeader{},
	})
	require.NoError(t, err)

	require.Equal(t, uint64(1), original.ToProtocol().(*pbeth.Block).Number)

	buffer := bytes.NewBuffer(nil)
	writer, err := bstream.GetBlockWriterFactory.New(buffer)
	require.NoError(t, err)

	require.NoError(t, writer.Write(original))

	reader, err := bstream.GetBlockReaderFactory.New(buffer)
	require.NoError(t, err)

	hydrated, err := reader.Read()
	require.NoError(t, err)

	require.Equal(t, uint64(1), hydrated.ToProtocol().(*pbeth.Block).Number)
}
