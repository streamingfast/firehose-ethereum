package transform

import (
	"github.com/streamingfast/bstream/transform"
	"github.com/streamingfast/dstore"
	"github.com/streamingfast/eth-go"
)

const CallAddrIndexShortName = "calladdrsig"

type callAddressSingleFilter struct {
	addrs []eth.Address
	sigs  []eth.Hash
}

func NewEthCallIndexProvider(
	store dstore.Store,
	possibleIndexSizes []uint64,
	filters []*addrSigSingleFilter,
) *transform.GenericBlockIndexProvider {
	return transform.NewGenericBlockIndexProvider(
		store,
		CallAddrIndexShortName,
		possibleIndexSizes,
		getFilterFunc(filters),
	)
}
