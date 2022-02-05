package transform

import (
	"github.com/streamingfast/dstore"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/codec/v1"
	"github.com/stretchr/testify/require"
	"io"
	"io/ioutil"
	"strings"
	"testing"
)

func TestLogAddressIndexProvider_FindIndexContaining(t *testing.T) {
	tests := []struct {
		name      string
		indexSize uint64
		blocks    []*pbcodec.Block
	}{
		{
			name:      "sunny path",
			indexSize: 2,
			blocks:    testEthBlocks(t, 5),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			results := make(map[string][]byte)

			// spawn an indexStore which will populate the results
			indexStore := dstore.NewMockStore(func(base string, f io.Reader) error {
				content, err := ioutil.ReadAll(f)
				require.NoError(t, err)
				results[base] = content
				return nil
			})

			// spawn an indexer with our mock indexStore
			indexer := NewLogAddressIndexer(indexStore, test.indexSize)
			for _, blk := range test.blocks {
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

			// spawn an indexProvider with the new dstore
			provider := NewLogAddressIndexProvider(indexStore)

			// try to load an index without finding it first
			err := provider.loadIndex(strings.NewReader("bogus"))
			require.Error(t, err)

			// find the indexes containing specific block nums
			r := provider.findIndexContaining(10)
			require.NotNil(t, r)
			err = provider.loadIndex(r)
			require.Nil(t, err)
			require.Equal(t, uint64(2), provider.currentIndex.indexSize)
			require.Equal(t, uint64(10), provider.currentIndex.lowBlockNum)

			r = provider.findIndexContaining(12)
			require.NotNil(t, r)
			err = provider.loadIndex(r)
			require.Nil(t, err)
			require.Equal(t, uint64(2), provider.currentIndex.indexSize)
			require.Equal(t, uint64(12), provider.currentIndex.lowBlockNum)

			r = provider.findIndexContaining(42)
			require.Nil(t, r)
			r = provider.findIndexContaining(69)
			require.Nil(t, r)
		})
	}
}
