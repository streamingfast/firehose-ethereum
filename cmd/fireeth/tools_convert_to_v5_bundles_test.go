package main

import (
	"testing"

	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_mergedBlocksBundler(t *testing.T) {
	blockRange := func(from, to uint64) []*pbbstream.Block {
		var out []*pbbstream.Block
		for num := from; num < to; num++ {
			out = append(out, &pbbstream.Block{Number: num})
		}
		return out
	}

	type sourceFile struct {
		low    uint64
		blocks []*pbbstream.Block
	}
	sourceFiles := func(from, to, size uint64) (out []sourceFile) {
		for low := from; low < to; low += size {
			out = append(out, sourceFile{low, blockRange(low, low+size)})
		}
		return
	}

	type bundle struct {
		low   uint64
		first uint64
		last  uint64
		count int
	}

	tests := []struct {
		name        string
		bundleSize  uint64
		lowBlockNum uint64
		stop        uint64
		sourceSize  uint64
		sourceFiles []sourceFile
		expected    []bundle
		expectDone  bool
	}{
		{
			name:        "100 to 1000",
			bundleSize:  1000,
			lowBlockNum: 1000,
			stop:        3000,
			sourceSize:  100,
			sourceFiles: sourceFiles(1000, 3000, 100),
			expected:    []bundle{{1000, 1000, 1999, 1000}, {2000, 2000, 2999, 1000}},
			expectDone:  true,
		},
		{
			name:        "1000 to 100, range inside one source file",
			bundleSize:  100,
			lowBlockNum: 1200,
			stop:        1400,
			sourceSize:  1000,
			sourceFiles: []sourceFile{{1000, blockRange(1000, 2000)}},
			expected:    []bundle{{1200, 1200, 1299, 100}, {1300, 1300, 1399, 100}},
			expectDone:  true,
		},
		{
			name:        "source ends before the last bundle is complete",
			bundleSize:  1000,
			lowBlockNum: 0,
			stop:        2000,
			sourceSize:  100,
			sourceFiles: sourceFiles(0, 1500, 100),
			expected:    []bundle{{0, 0, 999, 1000}},
			expectDone:  false,
		},
		{
			name:        "file without its own blocks carries the previous block",
			bundleSize:  100,
			lowBlockNum: 0,
			stop:        300,
			sourceSize:  100,
			sourceFiles: []sourceFile{
				{0, blockRange(0, 50)},
				{100, []*pbbstream.Block{{Number: 49}}},
				{200, blockRange(250, 300)},
			},
			expected:   []bundle{{0, 0, 49, 50}, {100, 49, 49, 1}, {200, 250, 299, 50}},
			expectDone: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var written []bundle
			bundler := &mergedBlocksBundler{
				bundleSize:   tt.bundleSize,
				lowBlockNum:  tt.lowBlockNum,
				stopBlockNum: tt.stop,
				write: func(low uint64, blocks []*pbbstream.Block) error {
					written = append(written, bundle{low, blocks[0].Number, blocks[len(blocks)-1].Number, len(blocks)})
					return nil
				},
			}

			for _, file := range tt.sourceFiles {
				require.NoError(t, bundler.add(file.blocks))
				require.NoError(t, bundler.writeBundlesEndingBefore(file.low+tt.sourceSize))
			}

			assert.Equal(t, tt.expected, written)
			assert.Equal(t, tt.expectDone, bundler.done())
		})
	}
}
