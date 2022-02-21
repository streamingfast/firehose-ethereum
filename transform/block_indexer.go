package transform

import (
	"encoding/hex"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/bstream/transform"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/codec/v1"
	"go.uber.org/zap"
)

type EthBlockIndexer struct {
	bi *transform.BlockIndexer
}

func (i *EthBlockIndexer) ProcessBlock(blk *pbcodec.Block) error {

	// init lower bound
	if i.bi.CurrentIndex == nil {
		switch {

		case blk.Num()%i.bi.IndexSize == 0:
			// we're on a boundary
			i.bi.CurrentIndex = transform.NewBlockIndex(blk.Number, i.bi.IndexSize)

		case blk.Number == bstream.GetProtocolFirstStreamableBlock:
			// handle offset
			lb := lowBoundary(blk.Num(), i.bi.IndexSize)
			i.bi.CurrentIndex = transform.NewBlockIndex(lb, i.bi.IndexSize)

		default:
			zlog.Warn("couldn't determine boundary for block", zap.Uint64("blk_num", blk.Num()))
			return nil
		}
	}

	// upper bound reached
	if blk.Num() >= i.bi.CurrentIndex.LowBlockNum()+i.bi.IndexSize {
		if err := i.bi.WriteIndex(); err != nil {
			zlog.Warn("couldn't write index", zap.Error(err))
		}
		lb := lowBoundary(blk.Number, i.bi.IndexSize)
		i.bi.CurrentIndex = transform.NewBlockIndex(lb, i.bi.IndexSize)
	}

	for _, trace := range blk.TransactionTraces {
		for _, log := range trace.Receipt.Logs {
			var evSig []byte
			if len(log.Topics) != 0 {
				evSig = log.Topics[0]
			}

			i.bi.CurrentIndex.Add(hex.EncodeToString(log.Address), blk.Number)
			i.bi.CurrentIndex.Add(hex.EncodeToString(evSig), blk.Number)
		}
	}

	return nil
}
