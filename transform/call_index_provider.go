package transform

import (
	"github.com/streamingfast/bstream/transform"
	"github.com/streamingfast/dstore"
)

const CallAddrIndexShortName = "calladdrsig"

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
