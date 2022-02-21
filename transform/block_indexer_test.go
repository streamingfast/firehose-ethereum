package transform

import (
	"github.com/streamingfast/bstream/transform"
	"io"
	"io/ioutil"
	"testing"

	"github.com/streamingfast/dstore"

	pbcodec "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/codec/v1"
	"github.com/stretchr/testify/require"
)

func TestEthBlockIndexer(t *testing.T) {
	tests := []struct {
		name                 string
		blocks               []*pbcodec.Block
		indexSize            uint64
		shouldWriteFile      bool
		shouldReadFile       bool
		expectedKvAfterWrite map[string][]uint64
	}{
		{
			name:            "sunny within bounds",
			indexSize:       10,
			shouldWriteFile: false,
			shouldReadFile:  false,
			blocks:          testEthBlocks(t, 2),
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
			bi := transform.NewBlockIndexer(indexStore, test.indexSize, "test")
			indexer := EthBlockIndexer{bi: bi}

			for _, blk := range test.blocks {
				// feed the indexer
				err := indexer.ProcessBlock(blk)
				require.NoError(t, err)
			}

			// check our resulting KV
			require.NotNil(t, bi.CurrentIndex.KV())
			require.Equal(t, len(bi.CurrentIndex.KV()), len(test.expectedKvAfterWrite))
			for expectedK, expectedV := range test.expectedKvAfterWrite {
				actualV, ok := bi.CurrentIndex.KV()[expectedK]
				require.True(t, ok)
				arr := actualV.ToArray()
				require.Equal(t, arr, expectedV)
			}
		})
	}

}
