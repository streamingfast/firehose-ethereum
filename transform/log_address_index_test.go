package transform

import (
	"testing"

	"github.com/RoaringBitmap/roaring/roaring64"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/eth-go"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/codec/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogAddressIndexer(t *testing.T) {
	tests := []struct {
		name   string
		blocks []*bstream.Block
		//		lowBlockNum      uint64
		//		indexSize        uint64
		expectAddresses  map[string][]uint64
		expectSignatures map[string][]uint64
	}{
		{
			name: "sunny",
			//indexSize: 100,
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
					//					lowBlockNum: test.lowBlockNum,
					addrs:     make(map[string]*roaring64.Bitmap),
					eventSigs: make(map[string]*roaring64.Bitmap),
				},
			}
			for _, blk := range test.blocks {
				indexer.ProcessEthBlock(blk.ToProtocol().(*pbcodec.Block))
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
		})
	}

}

func TestLogAddressIndex_Matching(t *testing.T) {
	tests := []struct {
		name          string
		reqAddresses  []string
		reqSignatures []string
		expectBlocks  []uint64
	}{
		{
			name: "single address",
			reqAddresses: []string{
				"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
			expectBlocks: []uint64{10, 11},
		},
		{
			name: "single address singleblock",
			reqAddresses: []string{
				"cccccccccccccccccccccccccccccccccccccccc",
			},
			expectBlocks: []uint64{10},
		},
		{
			name: "two addresses",
			reqAddresses: []string{
				"cccccccccccccccccccccccccccccccccccccccc",
				"dddddddddddddddddddddddddddddddddddddddd",
			},
			expectBlocks: []uint64{10, 11},
		},
		{
			name: "duplicate address match",
			reqAddresses: []string{
				"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				"cccccccccccccccccccccccccccccccccccccccc",
				"dddddddddddddddddddddddddddddddddddddddd",
			},
			expectBlocks: []uint64{10, 11},
		},
		{
			name: "addr and sig",
			reqAddresses: []string{
				"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
			reqSignatures: []string{
				"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
			expectBlocks: []uint64{10, 11},
		},
		{
			name: "addr and restrictive sig",
			reqAddresses: []string{
				"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
			reqSignatures: []string{
				"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
			},
			expectBlocks: []uint64{11},
		},
		{
			name: "restrictive addr and good sig",
			reqAddresses: []string{
				"dddddddddddddddddddddddddddddddddddddddd",
			},
			reqSignatures: []string{
				"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
			expectBlocks: []uint64{11},
		},
		{
			name: "no joining match",
			reqAddresses: []string{
				"cccccccccccccccccccccccccccccccccccccccc",
			},
			reqSignatures: []string{
				"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
			},
			expectBlocks: nil,
		},
		{
			name: "nothing matches",
			reqAddresses: []string{
				"ff00ffffffffffffffffffffffffffffffffffff",
			},
			reqSignatures: []string{
				"ff00ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
			},
			expectBlocks: nil,
		},
		{
			name: "only signature",
			reqSignatures: []string{
				"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
			expectBlocks: []uint64{10, 11},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			indexer := &LogAddressIndexer{
				currentIndex: &LogAddressIndex{
					addrs:     make(map[string]*roaring64.Bitmap),
					eventSigs: make(map[string]*roaring64.Bitmap),
				},
			}
			testBlocks := []*bstream.Block{
				testBlock(t, "blk10.json"),
				testBlock(t, "blk11.json"),
			}
			for _, blk := range testBlocks {
				indexer.ProcessEthBlock(blk.ToProtocol().(*pbcodec.Block))
			}

			var reqAddresses []eth.Address
			for _, addr := range test.reqAddresses {
				reqAddresses = append(reqAddresses, eth.MustNewAddress("0x"+addr))
			}

			var reqSignatures []eth.Hash
			for _, sig := range test.reqSignatures {
				reqSignatures = append(reqSignatures, eth.MustNewHash("0x"+sig))
			}

			matching := indexer.currentIndex.matchingBlocks(reqAddresses, reqSignatures)

			assert.Equal(t, test.expectBlocks, matching)
		})
	}

}
