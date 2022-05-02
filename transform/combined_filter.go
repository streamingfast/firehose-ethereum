package transform

import (
	"fmt"

	"github.com/RoaringBitmap/roaring/roaring64"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/bstream/transform"
	"github.com/streamingfast/dstore"
	pbtransform "github.com/streamingfast/sf-ethereum/types/pb/sf/ethereum/transform/v1"
	pbeth "github.com/streamingfast/sf-ethereum/types/pb/sf/ethereum/type/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

const LP = "L" // log prefix for combined index
const CP = "C" // call prefix for combined index

const CombinedIndexerShortName = "combined"

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

			var callToFilters []*CallToFilter
			for _, in := range filter.CallFilters {
				f := &CallToFilter{}
				if err := f.load(in); err != nil {
					return nil, err
				}
				callToFilters = append(callToFilters, f)
			}

			var logFilters []*LogFilter
			for _, in := range filter.LogFilters {
				f := &LogFilter{}
				if err := f.load(in); err != nil {
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
		},
	}
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
		for _, key := range callKeys(trace, NP) {
			keys = append(keys, key)
		}
		for _, key := range logKeys(trace, NP) {
			keys = append(keys, key)
		}
	}

	i.BlockIndexer.Add(keys, blk.Number)
	return
}

func (f *CombinedFilter) String() string {
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

	var callFilters []*addrSigSingleFilter
	for _, cf := range f.CallToFilters {
		filter := &addrSigSingleFilter{
			cf.Addresses,
			cf.Signatures,
		}
		callFilters = append(callFilters, filter)
	}

	var logFilters []*addrSigSingleFilter
	for _, lf := range f.LogFilters {
		filter := &addrSigSingleFilter{
			lf.Addresses,
			lf.EventSignatures,
		}
		logFilters = append(logFilters, filter)
	}

	return transform.NewGenericBlockIndexProvider(
		f.indexStore,
		CombinedIndexerShortName,
		f.possibleIndexSizes,
		getcombinedFilterFunc(callFilters, logFilters),
	)

}

func getcombinedFilterFunc(logFilters []*addrSigSingleFilter, callFilters []*addrSigSingleFilter) func(transform.BitmapGetter) []uint64 {
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
