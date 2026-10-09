package main

import (
	"fmt"

	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
)

// mergedBlocksBundler groups blocks, received in order, into merged-blocks files of `bundleSize`
// blocks, from the bundle starting at `lowBlockNum` up to `stopBlockNum` (exclusive, on a bundle
// boundary). A bundle is written once all of its blocks were received.
type mergedBlocksBundler struct {
	bundleSize   uint64
	lowBlockNum  uint64
	stopBlockNum uint64
	write        func(lowBlockNum uint64, blocks []*pbbstream.Block) error

	blocks []*pbbstream.Block

	// lastBlock is the last block received, written alone in a bundle that holds no block, like
	// the merger does on chains that skip block numbers
	lastBlock *pbbstream.Block
}

// add receives the next blocks. Those outside of the bundles to write are dropped.
func (b *mergedBlocksBundler) add(blocks []*pbbstream.Block) error {
	for _, blk := range blocks {
		if blk.Number < b.lowBlockNum {
			b.lastBlock = blk
			continue
		}

		if err := b.writeBundlesEndingBefore(blk.Number); err != nil {
			return err
		}

		if blk.Number >= b.stopBlockNum {
			continue
		}

		b.blocks = append(b.blocks, blk)
		b.lastBlock = blk
	}
	return nil
}

// writeBundlesEndingBefore writes every bundle whose last block is below `blockNum`, all of their
// blocks were received.
func (b *mergedBlocksBundler) writeBundlesEndingBefore(blockNum uint64) error {
	for b.lowBlockNum+b.bundleSize <= blockNum && b.lowBlockNum < b.stopBlockNum {
		blocks := b.blocks
		if len(blocks) == 0 {
			if b.lastBlock == nil {
				return fmt.Errorf("no block to write in merged-blocks file %s", filename(b.lowBlockNum))
			}
			blocks = []*pbbstream.Block{b.lastBlock}
		}

		if err := b.write(b.lowBlockNum, blocks); err != nil {
			return fmt.Errorf("writing merged-blocks file %s: %w", filename(b.lowBlockNum), err)
		}

		b.blocks = nil
		b.lowBlockNum += b.bundleSize
	}
	return nil
}

// done tells if every bundle up to `stopBlockNum` was written.
func (b *mergedBlocksBundler) done() bool {
	return b.lowBlockNum >= b.stopBlockNum
}
