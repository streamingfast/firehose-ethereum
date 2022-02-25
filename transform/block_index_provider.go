package transform

import (
	"github.com/streamingfast/bstream/transform"
	"github.com/streamingfast/dstore"
)

const indexShortname = "logaddr"

// NewEthBlockIndexProvider instantiates and returns a new EthBlockIndexProvider
func NewEthBlockIndexProvider(
	store dstore.Store,
	possibleIndexSizes []uint64,
	filters []*logAddressSingleFilter,
) *transform.GenericBlockIndexProvider {
	filterFunc := getFilterFunc(filters)
	indexProvider := transform.NewGenericBlockIndexProvider(store, indexShortname, possibleIndexSizes, filterFunc)
	return indexProvider
}

// getFilterFunc provides the filterFunc used by the transform.GenericBlockIndexProvider.
// Ethereum chain-specific filtering is provided by a combination of logAddressSingleFilter
// The filterFunc accepts a transform.BlockIndex, whose KV payload is a map[string]*roaring64.bitmap
func getFilterFunc(filters []*logAddressSingleFilter) func(index *transform.BlockIndex) (matchingBlocks []uint64) {
	return func(index *transform.BlockIndex) (matchingBlocks []uint64) {
		return nil
	}
}
