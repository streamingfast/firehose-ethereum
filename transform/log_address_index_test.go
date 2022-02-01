package transform

import (
	"testing"

	"github.com/RoaringBitmap/roaring/roaring64"
	"github.com/streamingfast/bstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogAddressIndexer(t *testing.T) {
	tests := []struct {
		name             string
		blocks           []*bstream.Block
		lowBlockNum      uint64
		indexSize        uint64
		expectAddresses  map[string][]uint64
		expectSignatures map[string][]uint64
	}{
		{
			name:      "sunny",
			indexSize: 100,
			blocks: []*bstream.Block{
				testBlock(t, "blk10.json"),
				testBlock(t, "blk11.json"),
			},
			expectAddresses: map[string][]uint64{
				"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa": {10, 11},
				"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb": {10, 11},
				"cccccccccccccccccccccccccccccccccccccccc": {10},
				"dddddddddddddddddddddddddddddddddddddddd": {11},
			},
			expectSignatures: map[string][]uint64{
				"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa": {10, 11},
				"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb": {10, 11},
				"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc": {10},
				"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd": {11},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			indexer := &LogAddressIndexer{
				currentIndex: &LogAddressIndex{
					lowBlockNum: test.lowBlockNum,
					addrs:       make(map[string]*roaring64.Bitmap),
					eventSigs:   make(map[string]*roaring64.Bitmap),
				},
			}
			for _, blk := range test.blocks {
				err := indexer.ProcessBlock(blk, nil)
				require.NoError(t, err)
			}
			assert.Equal(t, len(test.expectAddresses), len(indexer.currentIndex.addrs))
			for addr, expectMatches := range test.expectAddresses {
				m, ok := indexer.currentIndex.addrs[addr]
				require.True(t, ok)
				arr := m.ToArray()
				assert.Equal(t, expectMatches, arr)
			}

			assert.Equal(t, len(test.expectSignatures), len(indexer.currentIndex.eventSigs))
			for sig, expectMatches := range test.expectSignatures {
				m, ok := indexer.currentIndex.eventSigs[sig]
				require.True(t, ok)
				arr := m.ToArray()
				assert.Equal(t, expectMatches, arr)
			}

			// TODO another test with same index but multiple queries
			//			outs := indexer.currentIndex.matchingBlocks([]eth.Address{eth.MustNewAddress("0xcccccccccccccccccccccccccccccccccccccccc")}, nil)
			//			fmt.Println(outs)
		})
	}

}
