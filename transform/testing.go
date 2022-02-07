package transform

import (
	"github.com/streamingfast/dstore"
	"io"
	"io/ioutil"
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

func testEthBlock(t *testing.T, blkNum uint64, addrs, sigs []string) *pbcodec.Block {

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

func testEthBlocks(t *testing.T, size int) []*pbcodec.Block {
	blocks := []*pbcodec.Block{
		testEthBlock(t, 10,
			[]string{
				"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				"cccccccccccccccccccccccccccccccccccccccc",
			},
			[]string{
				"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
			},
		),
		testEthBlock(t, 11,
			[]string{
				"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				"dddddddddddddddddddddddddddddddddddddddd",
			},
			[]string{
				"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
				"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
		),
		testEthBlock(t, 12,
			[]string{
				"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"1111111111111111111111111111111111111111",
				"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			},
			[]string{
				"0000000000000000000000000000000000000000000000000000000000000000",
				"1111111111111111111111111111111111111111111111111111111111111111",
				"2222222222222222222222222222222222222222222222222222222222222222",
			},
		),
		testEthBlock(t, 13,
			[]string{
				"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"4444444444444444444444444444444444444444",
				"5555555555555555555555555555555555555555",
			},
			[]string{
				"3333333333333333333333333333333333333333333333333333333333333333",
				"4444444444444444444444444444444444444444444444444444444444444444",
				"5555555555555555555555555555555555555555555555555555555555555555",
			},
		),
		testEthBlock(t, 14,
			[]string{
				"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"7777777777777777777777777777777777777777",
				"cccccccccccccccccccccccccccccccccccccccc",
			},
			[]string{
				"6666666666666666666666666666666666666666666666666666666666666666",
				"7777777777777777777777777777777777777777777777777777777777777777",
				"8888888888888888888888888888888888888888888888888888888888888888",
			},
		),
	}

	if size != 0 {
		return blocks[:size]
	}
	return blocks
}

// testMockstoreWithFiles will populate a MockStore with indexes of the provided Blocks, according to the provided indexSize
func testMockstoreWithFiles(t *testing.T, blocks []*pbcodec.Block, indexSize uint64) *dstore.MockStore {
	results := make(map[string][]byte)

	// spawn an indexStore which will populate the results
	indexStore := dstore.NewMockStore(func(base string, f io.Reader) error {
		content, err := ioutil.ReadAll(f)
		require.NoError(t, err)
		results[base] = content
		return nil
	})

	// spawn an indexer with our mock indexStore
	indexer := NewLogAddressIndexer(indexStore, indexSize)
	for _, blk := range blocks {
		// feed the indexer
		err := indexer.ProcessEthBlock(blk)
		require.NoError(t, err)
	}

	// check that dstore wrote the index files
	require.Equal(t, 2, len(results))

	// populate a new indexStore with the prior results
	indexStore = dstore.NewMockStore(nil)
	for indexName, indexContents := range results {
		indexStore.SetFile(indexName, indexContents)
	}

	return indexStore
}
