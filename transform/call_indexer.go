package transform

import (
	"encoding/hex"

	"github.com/streamingfast/bstream/transform"
	"github.com/streamingfast/dstore"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/codec/v1"
)

type CallIndexer interface {
	Add(keys []string, blockNum uint64)
}

// EthCallIndexer wraps a bstream.transform.BlockIndexer for chain-specific use on Ethereum
type EthCallIndexer struct {
	BlockIndexer LogIndexer
}

// NewEthCallIndexer instantiates and returns a new EthCallIndexer
func NewEthCallIndexer(indexStore dstore.Store, indexSize uint64) *EthCallIndexer {
	bi := transform.NewBlockIndexer(indexStore, indexSize, CallAddrIndexShortName)
	return &EthCallIndexer{
		BlockIndexer: bi,
	}
}

// ProcessBlock implements chain-specific logic for Ethereum bstream.Block's
func (i *EthCallIndexer) ProcessBlock(blk *pbcodec.Block) {
	var keys []string

	for _, trace := range blk.TransactionTraces {
		for _, call := range trace.Calls {
			keys = append(keys, hex.EncodeToString(call.Address))
			if sig := call.Method(); sig != nil {
				keys = append(keys, hex.EncodeToString(sig))
			}
		}
	}

	i.BlockIndexer.Add(keys, blk.Number)
	return
}
