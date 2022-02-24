package transform

import (
	"context"
	"github.com/streamingfast/bstream/transform"
	"github.com/streamingfast/dstore"
)

// EthBlockIndexProvider wraps a bstream.transform.BlockIndexProvider for chain-specific use on Ethereum
type EthBlockIndexProvider struct {
	BlockIndexProvider *transform.BlockIndexProvider
}

// NewEthBlockIndexProvider instantiates and returns a new EthBlockIndexProvider
func NewEthBlockIndexProvider(
	store dstore.Store,
	indexShortname string,
	possibleIndexSizes []uint64,
	filterFunc func(index *transform.BlockIndex) (matchingBlocks []uint64),
) *EthBlockIndexProvider {
	indexProvider := transform.NewBlockIndexProvider(store, indexShortname, possibleIndexSizes, filterFunc)
	return &EthBlockIndexProvider{
		BlockIndexProvider: indexProvider,
	}
}

// WithinRange determines the existence of an index which includes the provided blockNum
// it also attempts to pre-emptively load the index (read-ahead)
func (ip *EthBlockIndexProvider) WithinRange(blockNum uint64) bool {
	ctx := context.Background()
	return ip.BlockIndexProvider.WithinRange(ctx, blockNum)
}

// Matches returns true if the provided blockNum matches entries in the index
func (ip *EthBlockIndexProvider) Matches(blockNum uint64) (bool, error) {
	ctx := context.Background()
	return ip.BlockIndexProvider.Matches(ctx, blockNum)
}

// NextMatching attempts to find the next matching blockNum which matches the provided filter.
// It can determine if a match is found within the bounds of the known index, of outside those bounds.
// If no match corresponds to the filter, it will return the highest available blockNum
func (ip *EthBlockIndexProvider) NextMatching(blockNum uint64, exclusiveUpTo uint64) (num uint64, passedIndexBoundary bool, err error) {
	ctx := context.Background()
	return ip.BlockIndexProvider.NextMatching(ctx, blockNum, exclusiveUpTo)
}
