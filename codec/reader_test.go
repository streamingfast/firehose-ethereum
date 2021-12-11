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
	"io"
	"os"
	"testing"
	"time"

	"github.com/streamingfast/bstream"
	pbbstream "github.com/streamingfast/pbgo/sf/bstream/v1"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/codec/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlockReader(t *testing.T) {
	t.Skip("need to fix blocks.eth.dbins header so instead of eth, its ETH")

	readerFactory := bstream.GetBlockReaderFactory

	file, err := os.Open("./testdata/blocks.eth.dbins")
	require.NoError(t, err)

	reader, err := readerFactory.New(file)
	require.NoError(t, err)

	blk1, err := reader.Read()
	require.NoError(t, err)

	assert.Equal(t, "dfe2e70d6c116a541101cecbb256d7402d62125f6ddc9b607d49edc989825c64", blk1.ID())
	assert.Equal(t, uint64(100), blk1.Num())
	assert.Equal(t, "db10afd3efa45327eb284c83cc925bd9bd7966aea53067c1eebe0724d124ec1e", blk1.PreviousID())
	assert.Equal(t, uint64(1), blk1.LIBNum())
	assert.Equal(t, "1970-01-17T15:31:10Z", blk1.Time().Format(time.RFC3339))
	assert.Equal(t, pbbstream.Protocol_ETH, blk1.Kind())
	assert.Equal(t, int32(1), blk1.Version())

	payload1, err := blk1.Payload.Get()
	require.NoError(t, err, "getting payload")
	assert.Equal(t, 597, len(payload1), string(payload1))

	dblk1 := blk1.ToNative().(*pbcodec.Block)
	assert.Equal(t, 0, len(dblk1.TransactionTraces))
	assert.Equal(t, uint64(100), dblk1.Number)
	assert.Equal(t, "dfe2e70d6c116a541101cecbb256d7402d62125f6ddc9b607d49edc989825c64", hex.EncodeToString(dblk1.Hash))

	// Block #101, skip
	_, err = reader.Read()
	require.NoError(t, err)

	// Block #102, skip
	_, err = reader.Read()
	require.NoError(t, err)

	// Block #103, skip
	_, err = reader.Read()
	require.NoError(t, err)

	blk5, err := reader.Read()
	require.NoError(t, err)

	assert.Equal(t, "7faae5e905007d146c15b22dcb736935cb344f88be0d35fe656701e84d52398e", blk5.ID())
	assert.Equal(t, uint64(104), blk5.Num())
	assert.Equal(t, "39bef3da2cd14e02781b576050dc426606149bff937a4af43e65417e6e98c713", blk5.PreviousID())
	assert.Equal(t, uint64(1), blk5.LIBNum())
	assert.Equal(t, "1970-01-17T15:31:10Z", blk5.Time().Format(time.RFC3339))
	assert.Equal(t, pbbstream.Protocol_ETH, blk5.Kind())
	assert.Equal(t, int32(1), blk5.Version())

	payload2, err := blk5.Payload.Get()
	require.NoError(t, err, "getting payload")
	assert.Equal(t, 598, len(payload2), string(payload2))

	dblk5 := blk5.ToNative().(*pbcodec.Block)
	assert.Equal(t, 0, len(dblk5.TransactionTraces))
	assert.Equal(t, uint64(104), dblk5.Number)
	assert.Equal(t, "7faae5e905007d146c15b22dcb736935cb344f88be0d35fe656701e84d52398e", hex.EncodeToString(dblk5.Hash))

	_, err = reader.Read()
	require.Equal(t, io.EOF, err)
}
