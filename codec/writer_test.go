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
	"bytes"
	"testing"

	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/streamingfast/bstream"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/dfuse/ethereum/codec/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlockWriter(t *testing.T) {
	writerFactory := bstream.GetBlockWriterFactory

	buffer := bytes.NewBuffer([]byte{})
	blockWriter, err := writerFactory.New(buffer)
	require.NoError(t, err)

	block1 := &pbcodec.Block{
		Hash:   []byte{0x01},
		Number: 2,
		Header: &pbcodec.BlockHeader{
			Timestamp: &timestamp.Timestamp{},
		},
		Ver: 1,
	}

	blk1, err := BlockFromProto(block1)
	require.NoError(t, err)

	err = blockWriter.Write(blk1)
	require.NoError(t, err)

	block2 := &pbcodec.Block{
		Hash:   []byte{0x02},
		Number: 2,
		Header: &pbcodec.BlockHeader{
			Timestamp: &timestamp.Timestamp{},
		},
		Ver: 1,
	}

	blk2, err := BlockFromProto(block2)
	require.NoError(t, err)

	err = blockWriter.Write(blk2)
	require.NoError(t, err)

	// Reader part (to validate the data)

	readerFactory := bstream.GetBlockReaderFactory
	blockReader, err := readerFactory.New(buffer)
	require.NoError(t, err)

	readBlk1, err := blockReader.Read()
	require.NotNil(t, readBlk1)
	require.NoError(t, err)

	readBlock1 := readBlk1.ToNative().(*pbcodec.Block)
	assert.Equal(t, []byte{0x01}, readBlock1.Hash)

	readBlk2, err := blockReader.Read()
	require.NotNil(t, readBlk2)
	require.NoError(t, err)

	readBlock2 := readBlk2.ToNative().(*pbcodec.Block)
	assert.Equal(t, []byte{0x02}, readBlock2.Hash)
}
