package transform

import (
	"context"
	"github.com/streamingfast/bstream/transform"
	"github.com/streamingfast/dstore"
	"github.com/streamingfast/eth-go"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/codec/v1"
	"github.com/stretchr/testify/require"
	"io"
	"testing"
)

func TestNewEthBlockIndexProvider(t *testing.T) {
	indexStore := dstore.NewMockStore(func(base string, f io.Reader) error {
		return nil
	})
	indexProvider := NewEthBlockIndexProvider(indexStore, nil, nil)
	require.NotNil(t, indexProvider)
	require.IsType(t, transform.GenericBlockIndexProvider{}, *indexProvider)
}

func TestEthBlockIndexProvider_WithinRange(t *testing.T) {
	tests := []struct {
		name           string
		blocks         []*pbcodec.Block
		indexSize      uint64
		indexShortname string
		lowBlockNum    uint64
		wantedBlock    uint64
		isWithinRange  bool
	}{
		{
			name:           "block exists in first index",
			blocks:         testEthBlocks(t, 5),
			indexSize:      2,
			indexShortname: "test",
			lowBlockNum:    0,
			wantedBlock:    11,
			isWithinRange:  true,
		},
		{
			name:           "block exists in second index",
			blocks:         testEthBlocks(t, 5),
			indexSize:      2,
			indexShortname: "test",
			lowBlockNum:    0,
			wantedBlock:    13,
			isWithinRange:  true,
		},
		{
			name:           "block doesn't exist",
			blocks:         testEthBlocks(t, 5),
			indexSize:      2,
			indexShortname: "test",
			lowBlockNum:    0,
			wantedBlock:    69,
			isWithinRange:  false,
		},
	}

	matchAddresses := []eth.Address{eth.Address("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// populate a mock dstore with some index files
			indexStore := testMockstoreWithFiles(t, test.blocks, test.indexSize)

			// spawn an indexProvider with the populated dstore
			indexProvider := NewEthBlockIndexProvider(
				indexStore,
				[]uint64{test.indexSize},
				[]*logAddressSingleFilter{
					{matchAddresses, nil},
				},
			)
			require.NotNil(t, indexProvider)

			// meat and potatoes
			b := indexProvider.WithinRange(context.Background(), test.wantedBlock)
			if test.isWithinRange {
				require.True(t, b)
			} else {
				require.False(t, b)
			}
		})
	}
}
