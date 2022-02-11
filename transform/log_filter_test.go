package transform

import (
	"github.com/streamingfast/dstore"
	"io"
	"testing"

	"github.com/streamingfast/bstream/transform"
	"github.com/streamingfast/eth-go"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/codec/v1"
	pbtransforms "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/transforms/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"
)

func logFilterTransform(t *testing.T, addresses []eth.Address, topics []eth.Hash) *anypb.Any {
	transform := &pbtransforms.BasicLogFilter{}
	for _, addr := range addresses {
		transform.Addresses = append(transform.Addresses, addr.Bytes())
	}
	for _, topic := range topics {
		transform.EventSignatures = append(transform.EventSignatures, topic.Bytes())
	}
	a, err := anypb.New(transform)
	require.NoError(t, err)
	return a
}

func TestLogFilter_Transform(t *testing.T) {
	tests := []struct {
		name               string
		addresses          []eth.Address
		topics             []eth.Hash
		expectError        bool
		expectTracesLength int
		possibleIndexSizes []int
	}{
		{
			name:               "Transfer events",
			topics:             []eth.Hash{eth.MustNewHash("ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef")},
			expectTracesLength: 82,
		},
		{
			name:               "WETH Contract Transfer events",
			addresses:          []eth.Address{eth.MustNewAddress("0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2")},
			topics:             []eth.Hash{eth.MustNewHash("ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef")},
			expectTracesLength: 36,
		},
		{
			name:               "WETH Contract event logs",
			addresses:          []eth.Address{eth.MustNewAddress("0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2")},
			expectTracesLength: 38,
		},
	}

	transformReg := transform.NewRegistry()
	transformReg.Register(BasicLogFilterFactory(nil, nil))

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			transforms := []*anypb.Any{logFilterTransform(t, test.addresses, test.topics)}

			preprocFunc, err := transformReg.BuildFromTransforms(transforms)
			require.NoError(t, err)

			blk := testBlockFromFiles(t, "block.json")

			output, err := preprocFunc(blk)
			if test.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				pbcodecBlock := output.(*pbcodec.Block)
				assert.Equal(t, test.expectTracesLength, len(pbcodecBlock.TransactionTraces))
			}
		})
	}
}

func TestBasicLogFilter_GetIndexProvider(t *testing.T) {
	tests := []struct {
		name        string
		indexStore  dstore.Store
		addrs       []eth.Address
		sigs        []eth.Hash
		expectedNil bool
	}{
		{
			name: "with store and addresses and sigs",
			indexStore: dstore.NewMockStore(func(base string, f io.Reader) (err error) {
				return nil
			}),
			addrs: []eth.Address{
				eth.MustNewAddress("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
			},
			sigs: []eth.Hash{
				eth.MustNewHash("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
			},
			expectedNil: false,
		},
		{
			name:       "no store with addresses and sigs",
			indexStore: nil,
			addrs: []eth.Address{
				eth.MustNewAddress("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
			},
			sigs: []eth.Hash{
				eth.MustNewHash("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
			},
			expectedNil: true,
		},
		{
			name:       "no store no addresses with sigs",
			indexStore: nil,
			addrs:      nil,
			sigs: []eth.Hash{
				eth.MustNewHash("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
			},
			expectedNil: true,
		},
		{
			name:        "no store no addresses no sigs",
			indexStore:  nil,
			addrs:       nil,
			sigs:        nil,
			expectedNil: true,
		},
	}

	possibleIndexSizes := []uint64{10000, 1000, 100}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := &BasicLogFilter{
				indexStore:         test.indexStore,
				possibleIndexSizes: possibleIndexSizes,
				Addresses:          test.addrs,
				EventSigntures:     test.sigs,
			}
			p := f.GetIndexProvider()
			if test.expectedNil {
				require.Nil(t, p)
			} else {
				require.NotNil(t, p)
			}
		})
	}
}
