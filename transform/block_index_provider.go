package transform

import (
	"github.com/RoaringBitmap/roaring/roaring64"
	"github.com/streamingfast/bstream/transform"
	"github.com/streamingfast/dstore"
	"github.com/streamingfast/eth-go"
)

const indexShortname = "logaddr"

// NewEthBlockIndexProvider instantiates and returns a new EthBlockIndexProvider
func NewEthBlockIndexProvider(
	store dstore.Store,
	possibleIndexSizes []uint64,
	filters []*logAddressSingleFilter,
) *transform.GenericBlockIndexProvider {
	return transform.NewGenericBlockIndexProvider(
		store,
		indexShortname,
		possibleIndexSizes,
		getFilterFunc(filters),
	)
}

// getFilterFunc provides the filterFunc used by the transform.GenericBlockIndexProvider.
// Ethereum chain-specific filtering is provided by a combination of logAddressSingleFilter
// The filterFunc accepts a transform.BlockIndex, whose KV payload is a map[string]*roaring64.bitmap
func getFilterFunc(filters []*logAddressSingleFilter) func(index *transform.BlockIndex) (matchingBlocks []uint64) {
	return func(index *transform.BlockIndex) (matchingBlocks []uint64) {
		out := roaring64.NewBitmap()
		for _, f := range filters {
			fbit := filterBitmap(f, index)
			out.Or(fbit)
		}
		return nilIfEmpty(out.ToArray())
	}
}

// filterBitmap is a switchboard method which determines
// if we're interested in filtering the provided index by eth.Address, eth.Hash, or both
func filterBitmap(f *logAddressSingleFilter, index *transform.BlockIndex) *roaring64.Bitmap {
	wantAddresses := len(f.addrs) != 0
	wantSigs := len(f.eventSigs) != 0

	switch {
	case wantAddresses && !wantSigs:
		return addressBitmap(f.addrs, index)
	case wantSigs && !wantAddresses:
		return sigsBitmap(f.eventSigs, index)
	case wantAddresses && wantSigs:
		a := addressBitmap(f.addrs, index)
		b := sigsBitmap(f.eventSigs, index)
		a.And(b)
		return a
	default:
		panic("filterBitmap: unsupported case")
	}
}

// addressBitmap attempts to find the blockNums corresponding to the provided eth.Address
func addressBitmap(addrs []eth.Address, index *transform.BlockIndex) *roaring64.Bitmap {
	out := roaring64.NewBitmap()
	for _, addr := range addrs {
		addrString := addr.String()
		if bm, ok := index.KV()[addrString]; ok {
			out.Or(bm)
		}
	}
	return out
}

// sigsBitmap attemps to find the blockNums corresponding to the provided eth.Hash
func sigsBitmap(sigs []eth.Hash, index *transform.BlockIndex) *roaring64.Bitmap {
	out := roaring64.NewBitmap()
	for _, sig := range sigs {
		sigString := sig.String()
		if _, ok := index.KV()[sigString]; !ok {
			continue
		}
		out.Or(index.KV()[sigString])
	}
	return out
}

// nilIfEmpty is a convenience method which returns nil if the provided slice is empty
func nilIfEmpty(in []uint64) []uint64 {
	if len(in) == 0 {
		return nil
	}
	return in
}
