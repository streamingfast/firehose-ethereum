package transform

import (
	"github.com/streamingfast/eth-go"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/codec/v1"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestLogAddressIndexProvider_FindIndexContaining_LoadIndex(t *testing.T) {
	tests := []struct {
		name        string
		lowBlockNum uint64
		indexSize   uint64
		blocks      []*pbcodec.Block
	}{
		{
			name:        "sunny path",
			lowBlockNum: 10,
			indexSize:   2,
			blocks:      testEthBlocks(t, 5),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// populate a mock dstore with some index files
			indexStore := testMockstoreWithFiles(t, test.blocks, test.indexSize)

			// spawn an indexProvider with the populated dstore
			provider := NewLogAddressIndexProvider(indexStore, test.lowBlockNum, test.indexSize, nil, nil, []uint64{test.indexSize})

			// try to load an index without finding it first
			err := provider.loadIndex(strings.NewReader("bogus"), test.lowBlockNum, test.indexSize)
			require.Error(t, err)

			// try to find indexes with non-existent block nums
			r, lowBlockNum, indexSize := provider.findIndexContaining(42)
			require.Nil(t, r)
			r, lowBlockNum, indexSize = provider.findIndexContaining(69)
			require.Nil(t, r)

			// find the indexes containing specific block nums
			r, lowBlockNum, indexSize = provider.findIndexContaining(10)
			require.NotNil(t, r)
			require.Equal(t, test.lowBlockNum, lowBlockNum)
			require.Equal(t, test.indexSize, indexSize)
			err = provider.loadIndex(r, lowBlockNum, indexSize)
			require.Nil(t, err)
			require.Equal(t, test.indexSize, provider.currentIndex.indexSize)
			require.Equal(t, test.lowBlockNum, provider.currentIndex.lowBlockNum)

			// find the indexes containing a specific block num in another file
			r, lowBlockNum, indexSize = provider.findIndexContaining(12)
			require.NotNil(t, r)
			require.Equal(t, test.lowBlockNum+test.indexSize, lowBlockNum)
			require.Equal(t, test.indexSize, indexSize)
			err = provider.loadIndex(r, lowBlockNum, indexSize)
			require.Nil(t, err)
			require.Equal(t, test.indexSize, provider.currentIndex.indexSize)
			require.Equal(t, test.lowBlockNum+test.indexSize, provider.currentIndex.lowBlockNum)
		})
	}
}

func TestLogAddressIndexProvider_LoadRange(t *testing.T) {
	tests := []struct {
		name                string
		lowBlockNum         uint64
		indexSize           uint64
		blocks              []*pbcodec.Block
		wantedBlock         uint64
		expectedLowBlockNum uint64
	}{
		{
			name:                "sunny path - block in first index",
			lowBlockNum:         0,
			indexSize:           2,
			blocks:              testEthBlocks(t, 5),
			wantedBlock:         11,
			expectedLowBlockNum: 10,
		},
		{
			name:                "sunny path - block in second index",
			lowBlockNum:         0,
			indexSize:           2,
			blocks:              testEthBlocks(t, 5),
			wantedBlock:         13,
			expectedLowBlockNum: 12,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// populate a mock dstore with some index files
			indexStore := testMockstoreWithFiles(t, test.blocks, test.indexSize)

			// spawn an indexProvider with the populated dstore
			provider := NewLogAddressIndexProvider(indexStore, test.lowBlockNum, test.indexSize, nil, nil, []uint64{test.indexSize})
			require.NotNil(t, provider)

			// call loadRange on a non-existent blockNum
			err := provider.loadRange(69)
			require.NotNil(t, err)

			// call loadRange on known block
			err = provider.loadRange(test.wantedBlock)
			require.Nil(t, err)
			require.Equal(t, test.expectedLowBlockNum, provider.currentIndex.lowBlockNum)
			require.Equal(t, test.indexSize, provider.currentIndex.indexSize)
		})
	}
}

func TestLogAddressIndexProvider_WithinRange(t *testing.T) {
	tests := []struct {
		name        string
		lowBlockNum uint64
		indexSize   uint64
		blocks      []*pbcodec.Block
		wantedBlock uint64
	}{
		{
			name:        "sunny path - block in first index",
			lowBlockNum: 0,
			indexSize:   2,
			blocks:      testEthBlocks(t, 5),
			wantedBlock: 11,
		},
		{
			name:        "sunny path - block in second index",
			lowBlockNum: 0,
			indexSize:   2,
			blocks:      testEthBlocks(t, 5),
			wantedBlock: 13,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// populate a mock dstore with some index files
			indexStore := testMockstoreWithFiles(t, test.blocks, test.indexSize)

			// spawn an indexProvider with the populated dstore
			provider := NewLogAddressIndexProvider(indexStore, test.lowBlockNum, test.indexSize, nil, nil, []uint64{test.indexSize})
			require.NotNil(t, provider)

			// call WithinRange on a non-existent blockNum
			b := provider.WithinRange(69)
			require.False(t, b)

			// call loadRange on known blocks
			b = provider.WithinRange(test.wantedBlock)
			require.True(t, b)
			b = provider.WithinRange(test.wantedBlock)
			require.True(t, b)
		})
	}
}

func TestLogAddressIndexProvider_Matches(t *testing.T) {
	tests := []struct {
		name            string
		lowBlockNum     uint64
		indexSize       uint64
		blocks          []*pbcodec.Block
		wantedBlock     uint64
		filterAddresses []eth.Address
		filterEventSigs []eth.Hash
		expectedMatches bool
	}{
		{
			name:        "block exists in first index and filters match",
			lowBlockNum: 0,
			indexSize:   2,
			blocks:      testEthBlocks(t, 5),
			wantedBlock: 11,
			filterAddresses: []eth.Address{
				eth.MustNewAddress("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
			},
			filterEventSigs: []eth.Hash{
				eth.MustNewHash("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
			},
			expectedMatches: true,
		},
		{
			name:        "block exists in second index and filters match ",
			lowBlockNum: 0,
			indexSize:   2,
			blocks:      testEthBlocks(t, 5),
			wantedBlock: 13,
			filterAddresses: []eth.Address{
				eth.MustNewAddress("4444444444444444444444444444444444444444"),
			},
			filterEventSigs: []eth.Hash{
				eth.MustNewHash("4444444444444444444444444444444444444444444444444444444444444444"),
			},
			expectedMatches: true,
		},
		{
			name:        "block exists but filters don't match",
			lowBlockNum: 0,
			indexSize:   2,
			blocks:      testEthBlocks(t, 5),
			wantedBlock: 13,
			filterAddresses: []eth.Address{
				eth.MustNewAddress("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"),
			},
			filterEventSigs: []eth.Hash{
				eth.MustNewHash("1111111111111111111111111111111111111111111111111111111111111111"),
			},
			expectedMatches: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// populate a mock dstore with some index files
			indexStore := testMockstoreWithFiles(t, test.blocks, test.indexSize)

			// spawn an indexProvider with the populated dstore
			provider := NewLogAddressIndexProvider(indexStore, test.lowBlockNum, test.indexSize, nil, nil, []uint64{test.indexSize})
			require.NotNil(t, provider)

			// call Matches on a non-existent blockNum
			b := provider.Matches(69)
			require.False(t, b)

			// call Matches on a known blockNum; provider has no filters
			b = provider.Matches(test.wantedBlock)
			require.False(t, b)

			// call Matches on a known blockNum, with a provider containing filters
			provider = NewLogAddressIndexProvider(indexStore, test.lowBlockNum, test.indexSize, test.filterAddresses, test.filterEventSigs, []uint64{test.indexSize})
			b = provider.Matches(test.wantedBlock)
			if test.expectedMatches {
				require.True(t, b)
			} else {
				require.False(t, b)
			}
		})
	}
}
