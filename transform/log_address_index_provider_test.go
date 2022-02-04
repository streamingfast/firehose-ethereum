package transform

import (
	"github.com/streamingfast/dstore"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/codec/v1"
	"github.com/stretchr/testify/require"
	"io"
	"io/ioutil"
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
			indexSize: 3,
			blocks: []*pbcodec.Block{
				testETHBlock(t, 10,
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
				testETHBlock(t, 11,
					[]string{
						"dddddddddddddddddddddddddddddddddddddddd",
						"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
						"ffffffffffffffffffffffffffffffffffffffff",
					},
					[]string{
						"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
						"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
						"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
					},
				),
				testETHBlock(t, 12,
					[]string{
						"0000000000000000000000000000000000000000",
						"1111111111111111111111111111111111111111",
						"2222222222222222222222222222222222222222",
					},
					[]string{
						"0000000000000000000000000000000000000000000000000000000000000000",
						"1111111111111111111111111111111111111111111111111111111111111111",
						"2222222222222222222222222222222222222222222222222222222222222222",
					},
				),
			},
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
		})
	}
}
