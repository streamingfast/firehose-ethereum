package transform

import (
	"encoding/hex"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/bstream/transform"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/codec/v1"
	"go.uber.org/zap"
)

type EthBlockIndexer struct {
	BlockIndexer *transform.BlockIndexer
}

func (i *EthBlockIndexer) ProcessBlock(blk *pbcodec.Block) error {

	// init lower bound
	if i.BlockIndexer.CurrentIndex == nil {
		switch {

		case blk.Num()%i.BlockIndexer.IndexSize == 0:
			// we're on a boundary
			i.BlockIndexer.CurrentIndex = transform.NewBlockIndex(blk.Number, i.BlockIndexer.IndexSize)

		case blk.Number == bstream.GetProtocolFirstStreamableBlock:
			// handle offset
			lb := lowBoundary(blk.Num(), i.BlockIndexer.IndexSize)
			i.BlockIndexer.CurrentIndex = transform.NewBlockIndex(lb, i.BlockIndexer.IndexSize)

		default:
			zlog.Warn("couldn't determine boundary for block", zap.Uint64("blk_num", blk.Num()))
			return nil
		}
	}

	// upper bound reached
	if blk.Num() >= i.BlockIndexer.CurrentIndex.LowBlockNum()+i.BlockIndexer.IndexSize {
		if err := i.BlockIndexer.WriteIndex(); err != nil {
			zlog.Warn("couldn't write index", zap.Error(err))
		}
		lb := lowBoundary(blk.Number, i.BlockIndexer.IndexSize)
		i.BlockIndexer.CurrentIndex = transform.NewBlockIndex(lb, i.BlockIndexer.IndexSize)
	}

	for _, trace := range blk.TransactionTraces {
		for _, log := range trace.Receipt.Logs {
			var evSig []byte
			if len(log.Topics) != 0 {
				// @todo(froch, 22022022) parameterize the topics of interest
				evSig = log.Topics[0]
			}

			i.BlockIndexer.CurrentIndex.Add(hex.EncodeToString(log.Address), blk.Number)
			i.BlockIndexer.CurrentIndex.Add(hex.EncodeToString(evSig), blk.Number)
		}
	}

	return nil
}
