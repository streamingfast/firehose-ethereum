package transform

import (
	"io"
	"os"

	"github.com/mitchellh/go-testing-interface"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/dstore"
	"github.com/streamingfast/eth-go"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"github.com/streamingfast/jsonpb"
	pbbstream "github.com/streamingfast/pbgo/sf/bstream/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func testBlockFromFiles(t testing.T, filename string) *bstream.Block {
	file, err := os.Open("./testdata/" + filename)
	require.NoError(t, err)

	b := &pbeth.Block{}
	err = jsonpb.Unmarshal(file, b)
	require.NoError(t, err)

	blk := &bstream.Block{
		Id:             b.ID(),
		Number:         b.Number,
		PreviousId:     b.PreviousID(),
		LibNum:         1,
		PayloadKind:    pbbstream.Protocol_ETH,
		PayloadVersion: 2,
	}

	protoCnt, err := proto.Marshal(b)
	require.NoError(t, err)

	blk, err = bstream.GetBlockPayloadSetter(blk, protoCnt)
	require.NoError(t, err)
	return blk
}

func testEthBlock(t testing.T, blkNum uint64, addrs, sigs []string) *pbeth.Block {

	if len(addrs) == 0 || len(sigs) == 0 {
		t.Fatal("require at least 1 addr and 1 sig")
	}

	var logs1 []*pbeth.Log
	for _, addr := range addrs {
		logs1 = append(logs1, &pbeth.Log{
			Address: eth.MustNewAddress(addr),
			Topics: [][]byte{
				eth.MustNewHash(sigs[0]),
			},
		})
	}

	var logs2 []*pbeth.Log
	for _, sig := range sigs {
		logs2 = append(logs2, &pbeth.Log{
			Address: eth.MustNewAddress(addrs[0]),
			Topics: [][]byte{
				eth.MustNewHash(sig),
			},
		})
	}

	var calls1 []*pbeth.Call
	for _, addr := range addrs {
		calls1 = append(calls1, &pbeth.Call{
			Address: eth.MustNewAddress(addr),
			Input:   eth.MustNewHash(sigs[0]),
		})
	}
	var calls2 []*pbeth.Call
	for _, sig := range sigs {
		calls2 = append(calls2, &pbeth.Call{
			Address: eth.MustNewAddress(addrs[0]),
			Input:   eth.MustNewHash(sig),
		})
	}

	return &pbeth.Block{
		Number: blkNum,
		TransactionTraces: []*pbeth.TransactionTrace{
			{
				Hash:   eth.MustNewHash("0xDEADBEEF"),
				Status: pbeth.TransactionTraceStatus_SUCCEEDED,
				Receipt: &pbeth.TransactionReceipt{
					Logs: logs1,
				},
				Calls: calls1,
			},
			{
				Hash:   eth.MustNewHash("0xBEEFDEAD"),
				Status: pbeth.TransactionTraceStatus_SUCCEEDED,
				Receipt: &pbeth.TransactionReceipt{
					Logs: logs2,
				},
				Calls: calls2,
			},
		},
	}
}

// testEthBlocks returns a slice of pbeth.Block's
// it takes a size parameter, to truncate with [:size]
func testEthBlocks(t testing.T, size int) []*pbeth.Block {
	blocks := []*pbeth.Block{
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
				"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
		),
		testEthBlock(t, 14,
			[]string{
				"5555555555555555555555555555555555555555",
				"7777777777777777777777777777777777777777",
				"cccccccccccccccccccccccccccccccccccccccc",
			},
			[]string{
				"6666666666666666666666666666666666666666666666666666666666666666",
				"7777777777777777777777777777777777777777777777777777777777777777",
				"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
			},
		),
	}

	if size != 0 {
		return blocks[:size]
	}
	return blocks
}

// testBlockIndexMockStoreWithFiles will populate a MockStore with indexes of the provided Blocks, according to the provided indexSize
// this implementation uses an EthLogIndexer to write the index files
func testMockstoreWithFiles(t testing.T, blocks []*pbeth.Block, indexSize uint64) *dstore.MockStore {
	results := make(map[string][]byte)

	// spawn an indexStore which will populate the results
	indexStore := dstore.NewMockStore(func(base string, f io.Reader) error {
		content, err := io.ReadAll(f)
		require.NoError(t, err)
		results[base] = content
		return nil
	})

	// spawn an indexer with our mock indexStore
	indexer := NewEthCombinedIndexerLegacy(indexStore, indexSize)
	for _, blk := range blocks {
		// feed the indexer
		indexer.ProcessBlock(blk)
	}

	// populate a new indexStore with the prior results
	indexStore = dstore.NewMockStore(nil)
	for indexName, indexContents := range results {
		indexStore.SetFile(indexName, indexContents)
	}

	return indexStore
}
