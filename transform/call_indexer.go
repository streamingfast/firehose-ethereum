package transform

import (
	"encoding/hex"

	"github.com/streamingfast/bstream/transform"
	"github.com/streamingfast/dstore"
	pbeth "github.com/streamingfast/sf-ethereum/types/pb/sf/ethereum/type/v1"
)

type Indexer interface {
	Add(keys []string, blockNum uint64)
}

// EthCallIndexer wraps a bstream.transform.BlockIndexer for chain-specific use on Ethereum
type EthCallIndexer struct {
	BlockIndexer Indexer
}

// NewEthCallIndexer instantiates and returns a new EthCallIndexer
func NewEthCallIndexer(indexStore dstore.Store, indexSize uint64) *EthCallIndexer {
	bi := transform.NewBlockIndexer(indexStore, indexSize, CallAddrIndexShortName)
	return &EthCallIndexer{
		BlockIndexer: bi,
	}
}

func callKeys(trace *pbeth.TransactionTrace, prefix string) (out []string) {
	for _, call := range trace.Calls {
		out = append(out, hex.EncodeToString(call.Address))
		if sig := call.Method(); sig != nil {
			out = append(out, hex.EncodeToString(sig))
		}
	}

	return
}

// ProcessBlock implements chain-specific logic for Ethereum bstream.Block's
func (i *EthCallIndexer) ProcessBlock(blk *pbeth.Block) {
	var keys []string
	for _, trace := range blk.TransactionTraces {
		for _, key := range callKeys(trace, NP) {
			keys = append(keys, key)
		}
	}

	i.BlockIndexer.Add(keys, blk.Number)
	return
}
