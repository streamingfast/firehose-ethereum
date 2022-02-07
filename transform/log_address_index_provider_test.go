package transform

import (
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
			name:                "sunny path, block in first index",
			lowBlockNum:         0,
			indexSize:           2,
			blocks:              testEthBlocks(t, 5),
			wantedBlock:         11,
			expectedLowBlockNum: 10,
		},
		{
			name:                "sunny path, block in second index",
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
