package transform

import (
	"github.com/golang/protobuf/proto"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/jsonpb"
	pbbstream "github.com/streamingfast/pbgo/sf/bstream/v1"
	_ "github.com/streamingfast/sf-ethereum/codec"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/codec/v1"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func testBlock(t *testing.T) *bstream.Block {
	file, err := os.Open("./testdata/block.json")
	require.NoError(t, err)

	b := &pbcodec.Block{}
	err = jsonpb.Unmarshal(file, b)
	require.NoError(t, err)

	blk := &bstream.Block{
		Id:             b.ID(),
		Number:         b.Number,
		PreviousId:     b.PreviousID(),
		LibNum:         1,
		PayloadKind:    pbbstream.Protocol_ETH,
		PayloadVersion: 1,
	}

	protoCnt, err := proto.Marshal(b)
	require.NoError(t, err)

	blk, err = bstream.GetBlockPayloadSetter(blk, protoCnt)
	require.NoError(t, err)
	return blk
}
