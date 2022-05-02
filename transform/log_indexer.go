package transform

import (
	"encoding/hex"

	"github.com/streamingfast/bstream/transform"
	"github.com/streamingfast/dstore"
	pbeth "github.com/streamingfast/sf-ethereum/types/pb/sf/ethereum/type/v1"
)

// EthLogIndexer wraps a bstream.transform.BlockIndexer for chain-specific use on Ethereum
type EthLogIndexer struct {
	BlockIndexer Indexer
}

// NewEthLogIndexer instantiates and returns a new EthLogIndexer
func NewEthLogIndexer(indexStore dstore.Store, indexSize uint64) *EthLogIndexer {
	bi := transform.NewBlockIndexer(indexStore, indexSize, LogAddrIndexShortName)
	return &EthLogIndexer{
		BlockIndexer: bi,
	}
}

func logKeys(trace *pbeth.TransactionTrace, prefix string) (out []string) {
	for _, log := range trace.Receipt.Logs {
		var evSig []byte
		if len(log.Topics) != 0 {
			// @todo(froch, 22022022) parameterize the topics of interest
			evSig = log.Topics[0]
		}

		out = append(out, hex.EncodeToString(log.Address), hex.EncodeToString(evSig))
	}

	return
}

// ProcessBlock implements chain-specific logic for Ethereum bstream.Block's
func (i *EthLogIndexer) ProcessBlock(blk *pbeth.Block) {
	var keys []string
	for _, trace := range blk.TransactionTraces {
		for _, key := range logKeys(trace, NP) {
			keys = append(keys, key)
		}
	}

	i.BlockIndexer.Add(keys, blk.Number)
	return
}
