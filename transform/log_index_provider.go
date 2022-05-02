package transform

import (
	"github.com/RoaringBitmap/roaring/roaring64"
	"github.com/streamingfast/bstream/transform"
	"github.com/streamingfast/dstore"
	"github.com/streamingfast/eth-go"
)

const LogAddrIndexShortName = "logaddrsig"
const NP = "" // no prefix

// addrSigSingleFilter represents a combination of interesting eth.Address and eth.Hash
// can be composed into an array for more complex filtering
type addrSigSingleFilter struct {
	addrs []eth.Address
	sigs  []eth.Hash
}

// NewEthLogIndexProvider instantiates and returns a new EthLogIndexProvider
func NewEthLogIndexProvider(
	store dstore.Store,
	possibleIndexSizes []uint64,
	filters []*addrSigSingleFilter,
) *transform.GenericBlockIndexProvider {
	return transform.NewGenericBlockIndexProvider(
		store,
		LogAddrIndexShortName,
		possibleIndexSizes,
		getFilterFunc(filters),
	)
}

// getFilterFunc provides the filterFunc used by the transform.GenericBlockIndexProvider.
// Ethereum chain-specific filtering is provided by a combination of logAddressSingleFilter
// The filterFunc accepts a transform.BlockIndex, whose KV payload is a map[string]*roaring64.bitmap
func getFilterFunc(filters []*addrSigSingleFilter) func(transform.BitmapGetter) []uint64 {
	return func(getFunc transform.BitmapGetter) (matchingBlocks []uint64) {
		out := roaring64.NewBitmap()
		for _, f := range filters {
			fbit := filterBitmap(f, getFunc, NP)
			out.Or(fbit)
		}
		return nilIfEmpty(out.ToArray())
	}
}

// filterBitmap is a switchboard method which determines
// if we're interested in filtering the provided index by eth.Address, eth.Hash, or both
func filterBitmap(f *addrSigSingleFilter, getFunc transform.BitmapGetter, idxPrefix string) *roaring64.Bitmap {
	wantAddresses := len(f.addrs) != 0
	wantSigs := len(f.sigs) != 0

	switch {
	case wantAddresses && !wantSigs:
		return addressBitmap(f.addrs, getFunc, idxPrefix)
	case wantSigs && !wantAddresses:
		return sigsBitmap(f.sigs, getFunc, idxPrefix)
	case wantAddresses && wantSigs:
		a := addressBitmap(f.addrs, getFunc, idxPrefix)
		b := sigsBitmap(f.sigs, getFunc, idxPrefix)
		a.And(b)
		return a
	default:
		panic("filterBitmap: unsupported case")
	}
}

// addressBitmap attempts to find the blockNums corresponding to the provided eth.Address
func addressBitmap(addrs []eth.Address, getFunc transform.BitmapGetter, idxPrefix string) *roaring64.Bitmap {
	out := roaring64.NewBitmap()
	for _, addr := range addrs {
		addrString := idxPrefix + addr.String()
		if bm := getFunc(addrString); bm != nil {
			out.Or(bm)
		}
	}
	return out
}

// sigsBitmap attemps to find the blockNums corresponding to the provided eth.Hash
func sigsBitmap(sigs []eth.Hash, getFunc transform.BitmapGetter, idxPrefix string) *roaring64.Bitmap {
	out := roaring64.NewBitmap()
	for _, sig := range sigs {
		bm := getFunc(idxPrefix + sig.String())
		if bm == nil {
			continue
		}
		out.Or(bm)
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
