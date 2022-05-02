package substreams

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_ComputeStartAndEndBlock(t *testing.T) {
	tests := []struct {
		name               string
		blockNum           uint64
		cacheSize          uint64
		expectedStartBlock uint64
		expectedEndBlock   uint64
	}{
		{
			name:               "test 1",
			blockNum:           uint64(68),
			cacheSize:          uint64(10),
			expectedStartBlock: uint64(60),
			expectedEndBlock:   uint64(70),
		},
		{
			name:               "test 2",
			blockNum:           uint64(617),
			cacheSize:          uint64(100),
			expectedStartBlock: uint64(600),
			expectedEndBlock:   uint64(700),
		},
		{
			name:               "test 3",
			blockNum:           uint64(6819999),
			cacheSize:          uint64(10000),
			expectedStartBlock: uint64(6810000),
			expectedEndBlock:   uint64(6820000),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			startBlock, endBlock := computeStartAndEndBlock(test.blockNum, test.cacheSize)

			require.Equal(t, test.expectedStartBlock, startBlock)
			require.Equal(t, test.expectedEndBlock, endBlock)
		})
	}

}
