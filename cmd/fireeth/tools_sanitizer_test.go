package main

import (
	"testing"
	"time"

	firecore "github.com/streamingfast/firehose-core"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestSanitizeEthereumBlockForCompareLogsBloom(t *testing.T) {
	realBloom := make([]byte, 256)
	realBloom[3] = 0x40
	realBloom[128] = 0x01

	tests := []struct {
		name          string
		bloom         []byte
		expectedBloom []byte
	}{
		{
			name:          "all-zero bloom is normalized to nil",
			bloom:         make([]byte, 256),
			expectedBloom: nil,
		},
		{
			name:          "real bloom is kept",
			bloom:         realBloom,
			expectedBloom: realBloom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ethBlock := &pbeth.Block{
				Hash:   []byte{0xaa, 0xbb},
				Number: 2,
				Ver:    3,
				Header: &pbeth.BlockHeader{
					ParentHash: []byte{0xcc, 0xdd},
					Timestamp:  timestamppb.New(time.Unix(1700000000, 0)),
					LogsBloom:  append([]byte(nil), tt.bloom...),
				},
				TransactionTraces: []*pbeth.TransactionTrace{
					{
						Receipt: &pbeth.TransactionReceipt{
							LogsBloom: append([]byte(nil), tt.bloom...),
						},
					},
				},
			}

			blk, err := firecore.EncodeBlock(firecore.BlockEnveloppe{Block: ethBlock, LIBNum: 1})
			require.NoError(t, err)

			sanitized := SanitizeEthereumBlockForCompare(blk)

			msg, err := sanitized.Payload.UnmarshalNew()
			require.NoError(t, err)
			ethOut, ok := msg.(*pbeth.Block)
			require.True(t, ok)

			require.Equal(t, tt.expectedBloom, ethOut.Header.LogsBloom)

			// receipt logs blooms are always removed for comparison
			require.Nil(t, ethOut.TransactionTraces[0].Receipt.LogsBloom)
		})
	}
}
