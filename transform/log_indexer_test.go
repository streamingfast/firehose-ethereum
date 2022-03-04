package transform

import (
	"io"
	"testing"

	"github.com/streamingfast/dstore"

	pbcodec "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/codec/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEthLogIndexer(t *testing.T) {
	indexStore := dstore.NewMockStore(func(base string, f io.Reader) error {
		return nil
	})
	indexSize := uint64(10)

	indexer := NewEthLogIndexer(indexStore, indexSize)
	require.NotNil(t, indexer)
	require.IsType(t, EthLogIndexer{}, *indexer)
}

func TestEthLogIndexer(t *testing.T) {
	tests := []struct {
		name             string
		blocks           []*pbcodec.Block
		expectedAddCalls []addCall
	}{
		{
			name:   "sunny",
			blocks: testEthBlocks(t, 3),
			expectedAddCalls: []addCall{
				{
					map[string]bool{
						"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa":                         true,
						"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb":                         true,
						"cccccccccccccccccccccccccccccccccccccccc":                         true,
						"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa": true,
						"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb": true,
						"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc": true,
					},
					10,
				},
				{
					map[string]bool{
						"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa":                         true,
						"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb":                         true,
						"dddddddddddddddddddddddddddddddddddddddd":                         true,
						"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa": true,
						"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb": true,
						"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd": true,
					},
					11,
				},
				{
					map[string]bool{
						"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa":                         true,
						"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb":                         true,
						"1111111111111111111111111111111111111111":                         true,
						"0000000000000000000000000000000000000000000000000000000000000000": true,
						"1111111111111111111111111111111111111111111111111111111111111111": true,
						"2222222222222222222222222222222222222222222222222222222222222222": true,
					},
					12,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testGenericIndexer := &testBlockIndexer{}
			indexer := &EthLogIndexer{
				BlockIndexer: testGenericIndexer,
			}

			for _, blk := range test.blocks {
				indexer.ProcessBlock(blk)
			}

			assert.Equal(t, test.expectedAddCalls, testGenericIndexer.calls)

		})
	}
}

type addCall struct {
	keys     map[string]bool
	blockNum uint64
}

type testBlockIndexer struct {
	calls []addCall
}

func (i *testBlockIndexer) Add(keys []string, blockNum uint64) {
	keymap := make(map[string]bool)
	for _, key := range keys {
		keymap[key] = true
	}
	i.calls = append(i.calls, addCall{
		keys:     keymap,
		blockNum: blockNum,
	})
}
