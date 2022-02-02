package transform

import (
	"github.com/streamingfast/eth-go"
	"os"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/jsonpb"
	pbbstream "github.com/streamingfast/pbgo/sf/bstream/v1"
	_ "github.com/streamingfast/sf-ethereum/codec"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/codec/v1"
	"github.com/stretchr/testify/require"
)

func testBlockFromFiles(t *testing.T, filename string) *bstream.Block {
	file, err := os.Open("./testdata/" + filename)
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

func testBlockFromStructs(t *testing.T, blkNum uint64, sigs []string) *pbcodec.Block {

	longest := len(addrs)
	if len(sigs) > longest {
		longest = len(sigs)
	}




	return &pbcodec.Block{
		Number: blkNum,
		TransactionTraces: []*pbcodec.TransactionTrace{
			{
				Hash:    eth.MustNewHash("0xDEADBEEF"),
				Status:  pbcodec.TransactionTraceStatus_SUCCEEDED,
				Receipt: &pbcodec.TransactionReceipt{
					Logs: ,
				},
			},
		},
	}
}
