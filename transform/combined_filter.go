package transform

import (
	"encoding/hex"
	"fmt"

	"github.com/RoaringBitmap/roaring/roaring64"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/bstream/transform"
	"github.com/streamingfast/dstore"
	"github.com/streamingfast/eth-go"
	pbtransform "github.com/streamingfast/sf-ethereum/types/pb/sf/ethereum/transform/v1"
	pbeth "github.com/streamingfast/sf-ethereum/types/pb/sf/ethereum/type/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

const NP = ""  //
const LP = "L" // log prefix for combined index
const CP = "C" // call prefix for combined index

const CombinedIndexerShortName = "combined"

type Indexer interface {
	Add(keys []string, blockNum uint64)
}

var CombinedFilterMessageName = proto.MessageName(&pbtransform.CombinedFilter{})

func CombinedFilterFactory(indexStore dstore.Store, possibleIndexSizes []uint64) *transform.Factory {
	return &transform.Factory{
		Obj: &pbtransform.CombinedFilter{},
		NewFunc: func(message *anypb.Any) (transform.Transform, error) {
			mname := message.MessageName()
			if mname != CombinedFilterMessageName {
				return nil, fmt.Errorf("expected type url %q, recevied %q ", CombinedFilterMessageName, message.TypeUrl)
			}

			filter := &pbtransform.CombinedFilter{}
			err := proto.Unmarshal(message.Value, filter)
			if err != nil {
				return nil, fmt.Errorf("unexpected unmarshall error: %w", err)
			}

			if len(filter.CallFilters) == 0 && len(filter.LogFilters) == 0 {
				return nil, fmt.Errorf("a combined filter transform requires at-least one callto filter or one logfilter")
			}

			return newCombinedFilter(filter.CallFilters, filter.LogFilters, indexStore, possibleIndexSizes)

		},
	}
}

func newCombinedFilter(pbCallToFilters []*pbtransform.CallToFilter, pbLogFilters []*pbtransform.LogFilter, indexStore dstore.Store, possibleIndexSizes []uint64) (*CombinedFilter, error) {
	var callToFilters []*CallToFilter
	for _, in := range pbCallToFilters {
		f, err := NewCallToFilter(in)
		if err != nil {
			return nil, err
		}
		callToFilters = append(callToFilters, f)
	}

	var logFilters []*LogFilter
	for _, in := range pbLogFilters {
		f, err := NewLogFilter(in)
		if err != nil {
			return nil, err
		}
		logFilters = append(logFilters, f)
	}

	f := &CombinedFilter{
		CallToFilters:      callToFilters,
		LogFilters:         logFilters,
		indexStore:         indexStore,
		possibleIndexSizes: possibleIndexSizes,
	}

	return f, nil
}

type CombinedFilter struct {
	CallToFilters []*CallToFilter
	LogFilters    []*LogFilter

	indexStore         dstore.Store
	possibleIndexSizes []uint64
}

type EthCombinedIndexer struct {
	BlockIndexer Indexer
}

func NewEthCombinedIndexer(indexStore dstore.Store, indexSize uint64) *EthCombinedIndexer {
	bi := transform.NewBlockIndexer(indexStore, indexSize, CombinedIndexerShortName)
	return &EthCombinedIndexer{
		BlockIndexer: bi,
	}
}

// ProcessBlock implements chain-specific logic for Ethereum bstream.Block's
func (i *EthCombinedIndexer) ProcessBlock(blk *pbeth.Block) {
	var keys []string
	for _, trace := range blk.TransactionTraces {
		for _, key := range callKeys(trace, CP) {
			keys = append(keys, key)
		}
		for _, key := range logKeys(trace, LP) {
			keys = append(keys, key)
		}
	}

	i.BlockIndexer.Add(keys, blk.Number)
	return
}

func (f *CombinedFilter) String() string {
	//	var addresses []string
	//	var signatures []string
	//	for _, a := range p.Addresses {
	//		addresses = append(addresses, a.Pretty())
	//	}
	//	for _, s := range p.EventSignatures {
	//		signatures = append(signatures, s.Pretty())
	//	}
	//	return fmt.Sprintf("LogFilter{addrs: %s, evt_sigs: %s}", strings.Join(addresses, ","), strings.Join(signatures, ","))
	//
	return "combinedFilter"
}

func (f *CombinedFilter) matches(trace *pbeth.TransactionTrace) bool {
	for _, lf := range f.LogFilters {
		if lf.matches(trace) {
			return true
		}
	}
	for _, cf := range f.CallToFilters {
		if cf.matches(trace) {
			return true
		}
	}
	return false
}

func (f *CombinedFilter) Transform(readOnlyBlk *bstream.Block, in transform.Input) (transform.Output, error) {
	ethBlock := readOnlyBlk.ToProtocol().(*pbeth.Block)
	traces := []*pbeth.TransactionTrace{}
	for _, trace := range ethBlock.TransactionTraces {
		if f.matches(trace) {
			traces = append(traces, trace)
		}
	}
	ethBlock.TransactionTraces = traces
	return ethBlock, nil
}

// GetIndexProvider will instantiate a new index conforming to the bstream.BlockIndexProvider interface
func (f *CombinedFilter) GetIndexProvider() bstream.BlockIndexProvider {
	if f.indexStore == nil {
		return nil
	}

	if len(f.CallToFilters) == 0 && len(f.LogFilters) == 0 {
		return nil
	}

	return transform.NewGenericBlockIndexProvider(
		f.indexStore,
		CombinedIndexerShortName,
		f.possibleIndexSizes,
		getcombinedFilterFunc(f.CallToFilters, f.LogFilters),
	)

}

func getcombinedFilterFunc(callFilters []*CallToFilter, logFilters []*LogFilter) func(transform.BitmapGetter) []uint64 {
	return func(getFunc transform.BitmapGetter) (matchingBlocks []uint64) {
		out := roaring64.NewBitmap()
		for _, f := range logFilters {
			fbit := filterBitmap(f, getFunc, LP)
			out.Or(fbit)
		}
		for _, f := range callFilters {
			fbit := filterBitmap(f, getFunc, CP)
			out.Or(fbit)
		}
		return nilIfEmpty(out.ToArray())
	}
}

func logKeys(trace *pbeth.TransactionTrace, prefix string) (out []string) {
	for _, log := range trace.Receipt.Logs {
		var evSig []byte
		if len(log.Topics) != 0 {
			// @todo(froch, 22022022) parameterize the topics of interest
			evSig = log.Topics[0]
		}

		out = append(out, prefix+hex.EncodeToString(log.Address), prefix+hex.EncodeToString(evSig))
	}

	return
}
func callKeys(trace *pbeth.TransactionTrace, prefix string) (out []string) {
	for _, call := range trace.Calls {
		out = append(out, prefix+hex.EncodeToString(call.Address))
		if sig := call.Method(); sig != nil {
			out = append(out, prefix+hex.EncodeToString(sig))
		}
	}

	return
}

type AddressSignatureFilter interface {
	Addresses() []eth.Address
	Signatures() []eth.Hash
}

// filterBitmap is a switchboard method which determines
// if we're interested in filtering the provided index by eth.Address, eth.Hash, or both
func filterBitmap(f AddressSignatureFilter, getFunc transform.BitmapGetter, idxPrefix string) *roaring64.Bitmap {
	wantAddresses := len(f.Addresses()) != 0
	wantSigs := len(f.Signatures()) != 0

	switch {
	case wantAddresses && !wantSigs:
		return addressBitmap(f.Addresses(), getFunc, idxPrefix)
	case wantSigs && !wantAddresses:
		return sigsBitmap(f.Signatures(), getFunc, idxPrefix)
	case wantAddresses && wantSigs:
		a := addressBitmap(f.Addresses(), getFunc, idxPrefix)
		b := sigsBitmap(f.Signatures(), getFunc, idxPrefix)
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
