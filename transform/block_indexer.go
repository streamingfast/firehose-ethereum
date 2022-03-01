package transform

import (
	"encoding/hex"

	"github.com/streamingfast/bstream/transform"
	"github.com/streamingfast/dstore"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/codec/v1"
)

type BlockIndexer interface {
	Add(keys []string, blockNum uint64)
}

// EthBlockIndexer wraps a bstream.transform.BlockIndexer for chain-specific use on Ethereum
type EthBlockIndexer struct {
	BlockIndexer BlockIndexer
}

// NewEthBlockIndexer instantiates and returns a new EthBlockIndexer
func NewEthBlockIndexer(indexStore dstore.Store, indexSize uint64) *EthBlockIndexer {
	bi := transform.NewBlockIndexer(indexStore, indexSize, LogAddrIndexShortName)
	return &EthBlockIndexer{
		BlockIndexer: bi,
	}
}

// ProcessBlock implements chain-specific logic for Ethereum bstream.Block's
func (i *EthBlockIndexer) ProcessBlock(blk *pbcodec.Block) {
	var keys []string

	for _, trace := range blk.TransactionTraces {
		for _, log := range trace.Receipt.Logs {
			var evSig []byte
			if len(log.Topics) != 0 {
				// @todo(froch, 22022022) parameterize the topics of interest
				evSig = log.Topics[0]
			}

			keys = append(keys, hex.EncodeToString(log.Address))
			keys = append(keys, hex.EncodeToString(evSig))
		}
	}

	i.BlockIndexer.Add(keys, blk.Number)
	return
}
