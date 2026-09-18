package main

import (
	"strings"
	"testing"

	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"github.com/stretchr/testify/require"
)

func Test_removeLargeKeccakPreimagesFromEthereumBlock(t *testing.T) {
	// Map values are hex-encoded, so the cut-off is 512 characters.
	slotDerivation := strings.Repeat("ab", 64)    // 64 bytes, a mapping with a value-type key
	atTheLimit := strings.Repeat("cd", 256)       // exactly 256 bytes, kept
	justOverTheLimit := strings.Repeat("ef", 257) // 257 bytes, dropped
	wayOver := strings.Repeat("01", 65536)        // 64 KiB, dropped

	block := &pbeth.Block{
		SystemCalls: []*pbeth.Call{
			{
				KeccakPreimages: map[string]string{
					"sys-keep": slotDerivation,
					"sys-drop": wayOver,
				},
			},
		},
		TransactionTraces: []*pbeth.TransactionTrace{
			{
				Calls: []*pbeth.Call{
					{
						KeccakPreimages: map[string]string{
							"keep-limit": atTheLimit,
							"drop-over":  justOverTheLimit,
						},
					},
					{
						KeccakPreimages: map[string]string{
							"keep-empty": "",
							"keep-slot":  slotDerivation,
						},
					},
					{
						KeccakPreimages: nil,
					},
				},
			},
		},
	}

	removed := removeLargeKeccakPreimagesFromEthereumBlock(block)
	require.Equal(t, 2, removed)

	require.Equal(t, map[string]string{"sys-keep": slotDerivation}, block.SystemCalls[0].KeccakPreimages)
	require.Equal(t, map[string]string{"keep-limit": atTheLimit}, block.TransactionTraces[0].Calls[0].KeccakPreimages)
	require.Equal(t, map[string]string{"keep-empty": "", "keep-slot": slotDerivation}, block.TransactionTraces[0].Calls[1].KeccakPreimages)
	require.Nil(t, block.TransactionTraces[0].Calls[2].KeccakPreimages)
}

func Test_removeLargeKeccakPreimagesFromEthereumBlock_nilBlock(t *testing.T) {
	require.Equal(t, 0, removeLargeKeccakPreimagesFromEthereumBlock(nil))
}
