package transform

import (
	"os"

	"github.com/mitchellh/go-testing-interface"
	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/anypb"
)

func testBlockFromFiles(t testing.T, filename string) *pbbstream.Block {
	file, err := os.ReadFile("./testdata/" + filename)
	require.NoError(t, err)

	b := &pbeth.Block{}
	err = protojson.Unmarshal(file, b)
	require.NoError(t, err)

	anyBlock, err := anypb.New(b)
	require.NoError(t, err)

	blk := &pbbstream.Block{
		Id:             b.ID(),
		Number:         b.Number,
		ParentId:       b.PreviousID(),
		LibNum:         1,
		PayloadKind:    pbbstream.Protocol_ETH,
		PayloadVersion: 2,
		Payload:        anyBlock,
	}

	return blk
}
