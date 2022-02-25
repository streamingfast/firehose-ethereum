package transform

import (
	"io"
	"io/ioutil"
	"testing"

	"github.com/streamingfast/dstore"

	pbcodec "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/codec/v1"
	"github.com/stretchr/testify/require"
)

func TestNewEthBlockIndexer(t *testing.T) {
	indexStore := dstore.NewMockStore(func(base string, f io.Reader) error {
		return nil
	})
	indexSize := uint64(10)

	indexer := NewEthBlockIndexer(indexStore, indexSize)
	require.NotNil(t, indexer)
	require.IsType(t, EthBlockIndexer{}, *indexer)
}

func TestEthBlockIndexer(t *testing.T) {
	tests := []struct {
		name                 string
		blocks               []*pbcodec.Block
		indexSize            uint64
		indexShortname       string
		shouldWriteFile      bool
		shouldReadFile       bool
		expectedResultsLen   int
		expectedKvAfterWrite map[string][]uint64
		expectedKvAfterRead  map[string][]uint64
	}{
		{
			name:               "sunny within bounds",
			indexSize:          10,
			indexShortname:     "test",
			shouldWriteFile:    false,
			shouldReadFile:     false,
			blocks:             testEthBlocks(t, 2),
			expectedResultsLen: 1,
			expectedKvAfterWrite: map[string][]uint64{
				"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa":                         {10, 11},
				"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb":                         {10, 11},
				"cccccccccccccccccccccccccccccccccccccccc":                         {10},
				"dddddddddddddddddddddddddddddddddddddddd":                         {11},
				"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa": {10, 11},
				"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb": {10, 11},
				"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc": {10},
				"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd": {11},
			},
		},
		{
			name:               "sunny and we wrote an index",
			indexSize:          2,
			indexShortname:     "test",
			shouldWriteFile:    true,
			shouldReadFile:     true,
			blocks:             testEthBlocks(t, 3),
			expectedResultsLen: 1,
			expectedKvAfterWrite: map[string][]uint64{
				"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa":                         {12},
				"1111111111111111111111111111111111111111":                         {12},
				"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb":                         {12},
				"0000000000000000000000000000000000000000000000000000000000000000": {12},
				"1111111111111111111111111111111111111111111111111111111111111111": {12},
				"2222222222222222222222222222222222222222222222222222222222222222": {12},
			},
			expectedKvAfterRead: map[string][]uint64{
				"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa":                         {10, 11},
				"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb":                         {10, 11},
				"cccccccccccccccccccccccccccccccccccccccc":                         {10},
				"dddddddddddddddddddddddddddddddddddddddd":                         {11},
				"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa": {10, 11},
				"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb": {10, 11},
				"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc": {10},
				"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd": {11},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			results := make(map[string][]byte)

			// spawn an indexStore which will populate the results
			indexStore := dstore.NewMockStore(func(base string, f io.Reader) error {
				if test.shouldWriteFile {
					content, err := ioutil.ReadAll(f)
					require.NoError(t, err)
					results[base] = content
				}
				return nil
			})

			// spawn an EthBlockIndexer with our mock indexStore
			indexer := NewEthBlockIndexer(indexStore, test.indexSize)

			// feed the indexer
			for _, blk := range test.blocks {
				indexer.ProcessBlock(blk)
			}

			// check our resulting KV
			require.NotNil(t, indexer.BlockIndexer.KV())
			require.Equal(t, len(indexer.BlockIndexer.KV()), len(test.expectedKvAfterWrite))
			for expectedK, expectedV := range test.expectedKvAfterWrite {
				actualV, ok := indexer.BlockIndexer.KV()[expectedK]
				require.True(t, ok)
				arr := actualV.ToArray()
				require.Equal(t, arr, expectedV)
			}

			// check that we wrote-out to dstore
			if test.shouldWriteFile {
				require.Equal(t, test.expectedResultsLen, len(results))
			}

			if test.shouldReadFile {
				// populate a new indexStore with the prior results
				indexStore = dstore.NewMockStore(nil)
				for indexName, indexContents := range results {
					indexStore.SetFile(indexName, indexContents)
				}

				// spawn a new BlockIndexer with the new IndexStore
				indexer = NewEthBlockIndexer(indexStore, test.indexSize)

				for indexName, _ := range results {
					// attempt to read back the index
					err := indexer.BlockIndexer.ReadIndex(indexName)
					require.NoError(t, err)

					// check our resulting KV
					require.NotNil(t, indexer.BlockIndexer.KV())
					require.Equal(t, len(indexer.BlockIndexer.KV()), len(test.expectedKvAfterRead))
					for expectedK, expectedV := range test.expectedKvAfterRead {
						actualV, ok := indexer.BlockIndexer.KV()[expectedK]
						require.True(t, ok)
						arr := actualV.ToArray()
						require.Equal(t, arr, expectedV)
					}
				}
			}
		})
	}
}
