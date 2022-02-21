package transform

import (
	"context"
	"strings"
	"testing"

	"github.com/streamingfast/eth-go"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/codec/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogAddressIndexProvider_NewLogAddressIndexProvider(t *testing.T) {

	aaaaAddr := eth.MustNewAddress("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	ccccAddr := eth.MustNewAddress("cccccccccccccccccccccccccccccccccccccccc")
	ddddAddr := eth.MustNewAddress("dddddddddddddddddddddddddddddddddddddddd")
	ccccSig := eth.MustNewHash("cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc")
	ddddSig := eth.MustNewHash("dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd")
	tests := []struct {
		name               string
		lowBlockNum        uint64
		indexSize          uint64
		blocks             []*pbcodec.Block
		filters            []*logAddressSingleFilter
		expectedNextBlocks []uint64
		expectNilProvider  bool
	}{
		{
			name:               "new with matches",
			lowBlockNum:        10,
			indexSize:          2,
			blocks:             testEthBlocks(t, 5),
			filters:            []*logAddressSingleFilter{{[]eth.Address{aaaaAddr, ccccAddr}, nil}},
			expectedNextBlocks: []uint64{10, 11},
			expectNilProvider:  false,
		},
		{
			name:               "new with single match",
			lowBlockNum:        10,
			indexSize:          2,
			blocks:             testEthBlocks(t, 5),
			filters:            []*logAddressSingleFilter{{[]eth.Address{ccccAddr}, nil}},
			expectedNextBlocks: []uint64{10},
			expectNilProvider:  false,
		},
		{
			name:              "new nil when no filter",
			lowBlockNum:       10,
			indexSize:         2,
			filters:           nil,
			expectNilProvider: true,
		},

		{
			name:        "new matching multifilter with sigs",
			lowBlockNum: 10,
			indexSize:   2,
			blocks:      testEthBlocks(t, 5),
			filters: []*logAddressSingleFilter{
				{[]eth.Address{ccccAddr}, []eth.Hash{ccccSig}},
				{[]eth.Address{ddddAddr}, []eth.Hash{ddddSig}},
			},
			expectedNextBlocks: []uint64{10, 11},
			expectNilProvider:  false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// populate a mock dstore with some index files
			indexStore := testMockstoreWithFiles(t, test.blocks, test.indexSize)

			// spawn an indexProvider with the populated dstore
			provider := NewLogAddressIndexProvider(indexStore, test.filters, []uint64{test.indexSize})
			if test.expectNilProvider {
				require.Nil(t, provider)
				return
			}
			require.NotNil(t, provider)

			ctx := context.Background()
			err := provider.loadRange(ctx, test.lowBlockNum)
			require.NoError(t, err)

			assert.Equal(t, test.expectedNextBlocks, provider.currentMatchingBlocks)
		})
	}

}

func TestLogAddressIndexProvider_FindIndexContaining_LoadIndex(t *testing.T) {
	initiallowBlockNum := uint64(10)
	initialindexSize := uint64(2)
	blocks := testEthBlocks(t, 5)
	matchAddresses := []eth.Address{eth.Address("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")}

	// populate a mock dstore with some index files
	indexStore := testMockstoreWithFiles(t, blocks, initialindexSize)

	// spawn an indexProvider with the populated dstore
	provider := NewLogAddressIndexProvider(indexStore, []*logAddressSingleFilter{{matchAddresses, nil}}, []uint64{initialindexSize})
	require.NotNil(t, provider)

	// try to load an index without finding it first
	err := provider.loadIndex(strings.NewReader("bogus"), initiallowBlockNum, initialindexSize)
	require.Error(t, err)

	ctx := context.Background()

	// try to find indexes with non-existent block nums
	r, lowBlockNum, indexSize := provider.findIndexContaining(ctx, 42)
	require.Nil(t, r)
	r, lowBlockNum, indexSize = provider.findIndexContaining(ctx, 69)
	require.Nil(t, r)

	// find the index containing a known block num
	r, lowBlockNum, indexSize = provider.findIndexContaining(ctx, 10)
	require.NotNil(t, r)
	require.Equal(t, lowBlockNum, lowBlockNum)
	require.Equal(t, indexSize, indexSize)
	err = provider.loadIndex(r, lowBlockNum, indexSize)
	require.Nil(t, err)
	require.Equal(t, indexSize, provider.currentIndex.indexSize)
	require.Equal(t, lowBlockNum, provider.currentIndex.lowBlockNum)

	// find the index containing a known block num, from another index file
	r, lowBlockNum, indexSize = provider.findIndexContaining(ctx, 12)
	require.NotNil(t, r)
	require.Equal(t, lowBlockNum, provider.currentIndex.lowBlockNum+indexSize)
	require.Equal(t, indexSize, provider.currentIndex.indexSize)
	err = provider.loadIndex(r, lowBlockNum, indexSize)
	require.Nil(t, err)
	require.Equal(t, lowBlockNum, provider.currentIndex.lowBlockNum)
	require.Equal(t, indexSize, provider.currentIndex.indexSize)
}

func TestLogAddressIndexProvider_LoadRange(t *testing.T) {
	tests := []struct {
		name                string
		lowBlockNum         uint64
		indexSize           uint64
		blocks              []*pbcodec.Block
		wantedBlock         uint64
		expectedLowBlockNum uint64
		expectError         bool
	}{
		{
			name:                "block exists in first index",
			lowBlockNum:         0,
			indexSize:           2,
			blocks:              testEthBlocks(t, 5),
			wantedBlock:         11,
			expectedLowBlockNum: 10,
			expectError:         false,
		},
		{
			name:                "block exists in second index",
			lowBlockNum:         0,
			indexSize:           2,
			blocks:              testEthBlocks(t, 5),
			wantedBlock:         13,
			expectedLowBlockNum: 12,
			expectError:         false,
		},
		{
			name:        "block doesn't exist",
			lowBlockNum: 0,
			indexSize:   2,
			blocks:      testEthBlocks(t, 5),
			wantedBlock: 69,
			expectError: true,
		},
	}

	matchAddresses := []eth.Address{eth.Address("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// populate a mock dstore with some index files
			indexStore := testMockstoreWithFiles(t, test.blocks, test.indexSize)

			// spawn an indexProvider with the populated dstore
			provider := NewLogAddressIndexProvider(indexStore, []*logAddressSingleFilter{{matchAddresses, nil}}, []uint64{test.indexSize})
			require.NotNil(t, provider)

			ctx := context.Background()
			// call loadRange on known block
			err := provider.loadRange(ctx, test.wantedBlock)
			if test.expectError {
				require.Error(t, err)
				return
			}
			require.Nil(t, err)
			require.Equal(t, test.expectedLowBlockNum, provider.currentIndex.lowBlockNum)
			require.Equal(t, test.indexSize, provider.currentIndex.indexSize)
		})
	}
}

func TestLogAddressIndexProvider_WithinRange(t *testing.T) {
	tests := []struct {
		name          string
		lowBlockNum   uint64
		indexSize     uint64
		blocks        []*pbcodec.Block
		wantedBlock   uint64
		expectMatches bool
	}{
		{
			name:          "block exists in first index",
			lowBlockNum:   0,
			indexSize:     2,
			blocks:        testEthBlocks(t, 5),
			wantedBlock:   11,
			expectMatches: true,
		},
		{
			name:          "block exists in second index",
			lowBlockNum:   0,
			indexSize:     2,
			blocks:        testEthBlocks(t, 5),
			wantedBlock:   13,
			expectMatches: true,
		},
		{
			name:          "block doesn't exist",
			lowBlockNum:   0,
			indexSize:     2,
			blocks:        testEthBlocks(t, 5),
			wantedBlock:   69,
			expectMatches: false,
		},
	}

	matchAddresses := []eth.Address{eth.Address("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// populate a mock dstore with some index files
			indexStore := testMockstoreWithFiles(t, test.blocks, test.indexSize)

			// spawn an indexProvider with the populated dstore
			provider := NewLogAddressIndexProvider(indexStore, []*logAddressSingleFilter{{matchAddresses, nil}}, []uint64{test.indexSize})
			require.NotNil(t, provider)

			// call loadRange on known blocks
			b := provider.WithinRange(context.Background(), test.wantedBlock)
			if test.expectMatches {
				require.True(t, b)
			} else {
				require.False(t, b)
			}
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
			indexStore := testMockstoreWithFiles(t, test.blocks, test.indexSize)
			provider := NewLogAddressIndexProvider(indexStore, []*logAddressSingleFilter{{test.filterAddresses, test.filterEventSigs}}, []uint64{test.indexSize})

			b, err := provider.Matches(context.Background(), test.wantedBlock)
			require.NoError(t, err)
			if test.expectedMatches {
				require.True(t, b)
			} else {
				require.False(t, b)
			}
		})
	}
}

func TestLogAddressIndexProvider_NextMatching(t *testing.T) {
	tests := []struct {
		name                        string
		lowBlockNum                 uint64
		indexSize                   uint64
		blocks                      []*pbcodec.Block
		wantedBlock                 uint64
		filterAddresses             []eth.Address
		filterEventSigs             []eth.Hash
		expectedNextBlockNum        uint64
		expectedPassedIndexBoundary bool
	}{
		{
			name:        "block exists in first index and filters match block in second index",
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
			expectedNextBlockNum:        13,
			expectedPassedIndexBoundary: false,
		},
		{
			name:        "block exists in first index and filters match block outside bounds",
			lowBlockNum: 0,
			indexSize:   2,
			blocks:      testEthBlocks(t, 5),
			wantedBlock: 10,
			filterAddresses: []eth.Address{
				eth.MustNewAddress("cccccccccccccccccccccccccccccccccccccccc"),
			},
			filterEventSigs: []eth.Hash{
				eth.MustNewHash("cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"),
			},
			expectedNextBlockNum:        14,
			expectedPassedIndexBoundary: true,
		},
		{
			name:        "filters don't match any known blocks",
			lowBlockNum: 0,
			indexSize:   2,
			blocks:      testEthBlocks(t, 5),
			wantedBlock: 10,
			filterAddresses: []eth.Address{
				eth.MustNewAddress("ffffffffffffffffffffffffffffffffffffffff"),
			},
			filterEventSigs: []eth.Hash{
				eth.MustNewHash("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			},
			expectedNextBlockNum:        14,
			expectedPassedIndexBoundary: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			indexStore := testMockstoreWithFiles(t, test.blocks, test.indexSize)
			provider := NewLogAddressIndexProvider(indexStore, []*logAddressSingleFilter{{test.filterAddresses, test.filterEventSigs}}, []uint64{test.indexSize})

			nextBlockNum, passedIndexBoundary, err := provider.NextMatching(context.Background(), test.wantedBlock, 0)
			require.NoError(t, err)
			require.Equal(t, passedIndexBoundary, test.expectedPassedIndexBoundary)
			require.Equal(t, nextBlockNum, test.expectedNextBlockNum)
		})
	}
}
