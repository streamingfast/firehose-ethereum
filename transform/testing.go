package transform

import (
	"os"
	"testing"

	"github.com/streamingfast/eth-go"

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

func testETHBlock(t *testing.T, blkNum uint64, addrs, sigs []string) *pbcodec.Block {

	if len(addrs) == 0 || len(sigs) == 0 {
		t.Fatal("require at least 1 addr and 1 sig")
	}

	var logs1 []*pbcodec.Log
	for _, addr := range addrs {
		logs1 = append(logs1, &pbcodec.Log{
			Address: eth.MustNewAddress(addr),
			Topics: [][]byte{
				eth.MustNewHash(sigs[0]),
			},
		})
	}

	var logs2 []*pbcodec.Log
	for _, sig := range sigs {
		logs2 = append(logs2, &pbcodec.Log{
			Address: eth.MustNewAddress(addrs[0]),
			Topics: [][]byte{
				eth.MustNewHash(sig),
			},
		})
	}

	return &pbcodec.Block{
		Number: blkNum,
		TransactionTraces: []*pbcodec.TransactionTrace{
			{
				Hash:   eth.MustNewHash("0xDEADBEEF"),
				Status: pbcodec.TransactionTraceStatus_SUCCEEDED,
				Receipt: &pbcodec.TransactionReceipt{
					Logs: logs1,
				},
			},
			{
				Hash:   eth.MustNewHash("0xBEEFDEAD"),
				Status: pbcodec.TransactionTraceStatus_SUCCEEDED,
				Receipt: &pbcodec.TransactionReceipt{
					Logs: logs2,
				},
			},
		},
	}
}
