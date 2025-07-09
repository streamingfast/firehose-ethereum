package transform

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/streamingfast/bstream"

	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"

	"github.com/RoaringBitmap/roaring/roaring64"
	"github.com/streamingfast/bstream/transform"
	"github.com/streamingfast/dstore"
	"github.com/streamingfast/eth-go"
	firecore "github.com/streamingfast/firehose-core"
	pbtransform "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/transform/v1"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

const IdxPrefixEmpty = "" //
const IdxPrefixLog = "L"  // log prefix for combined index
const IdxPrefixCall = "C" // call prefix for combined index

const CombinedIndexerShortName = "combined"

type Indexer interface {
	Add(keys []string, blockNum uint64)
}

var CombinedFilterMessageName = proto.MessageName(&pbtransform.CombinedFilter{})

func NewCombinedFilterTransformFactory(indexStore dstore.Store, possibleIndexSizes []uint64) (*transform.Factory, error) {
	return CombinedFilterTransformFactory(indexStore, possibleIndexSizes), nil
}

func CombinedFilterTransformFactory(indexStore dstore.Store, possibleIndexSizes []uint64) *transform.Factory {
	return &transform.Factory{
		Obj: &pbtransform.CombinedFilter{},
		NewFunc: func(message *anypb.Any) (transform.Transform, error) {
			mname := message.MessageName()
			if mname != CombinedFilterMessageName {
				return nil, fmt.Errorf("expected type url %q, received %q ", CombinedFilterMessageName, message.TypeUrl)
			}

			filter := &pbtransform.CombinedFilter{}
			err := proto.Unmarshal(message.Value, filter)
			if err != nil {
				return nil, fmt.Errorf("unexpected unmarshall error: %w", err)
			}

			if len(filter.CallFilters) == 0 && len(filter.LogFilters) == 0 && !filter.SendAllBlockHeaders {
				return nil, fmt.Errorf("a combined filter transform requires at-least one callto filter, one log filter or it must have have send_all_block_headers enabled")
			}

			return newCombinedFilter(filter.CallFilters, filter.LogFilters, indexStore, possibleIndexSizes, filter.SendAllBlockHeaders)

		},
	}
}

func newCombinedFilter(pbCallToFilters []*pbtransform.CallToFilter, pbLogFilters []*pbtransform.LogFilter, indexStore dstore.Store, possibleIndexSizes []uint64, sendAllBlockHeaders bool) (*CombinedFilter, error) {
	var callToFilters []*CallToFilter
	if l := len(pbCallToFilters); l > 0 {
		callToFilters = make([]*CallToFilter, l)
		for i, in := range pbCallToFilters {
			f, err := NewCallToFilter(in)
			if err != nil {
				return nil, err
			}
			callToFilters[i] = f
		}
	}

	var logFilters []*LogFilter

	if l := len(pbLogFilters); l > 0 {
		logFilters = make([]*LogFilter, l)
		for i, in := range pbLogFilters {
			f, err := NewLogFilter(in)
			if err != nil {
				return nil, err
			}
			logFilters[i] = f
		}
	}

	f := &CombinedFilter{
		CallToFilters:       callToFilters,
		LogFilters:          logFilters,
		indexStore:          indexStore,
		possibleIndexSizes:  possibleIndexSizes,
		sendAllBlockHeaders: sendAllBlockHeaders,
	}

	return f, nil
}

type CombinedFilter struct {
	CallToFilters []*CallToFilter
	LogFilters    []*LogFilter

	indexStore         dstore.Store
	possibleIndexSizes []uint64

	sendAllBlockHeaders bool
}

type EthCombinedIndexer struct {
	BlockIndexer Indexer
}

func NewEthCombinedIndexer(indexStore dstore.Store, indexSize uint64) (firecore.BlockIndexer[*pbeth.Block], error) {
	return NewEthCombinedIndexerLegacy(indexStore, indexSize), nil
}

func NewEthCombinedIndexerLegacy(indexStore dstore.Store, indexSize uint64) *EthCombinedIndexer {
	bi := transform.NewBlockIndexer(indexStore, indexSize, CombinedIndexerShortName)
	return &EthCombinedIndexer{
		BlockIndexer: bi,
	}
}

// ProcessBlock implements chain-specific logic for Ethereum pbbstream.Block's
func (i *EthCombinedIndexer) ProcessBlock(blk *pbeth.Block) error {
	keys := make(map[string]bool)
	for _, trace := range blk.TransactionTraces {
		for key := range callKeys(trace, IdxPrefixCall) {
			keys[key] = true
		}
		for key := range logKeys(trace, IdxPrefixLog) {
			keys[key] = true
		}
	}
	keyArray := make([]string, 0, len(keys))
	for key := range keys {
		keyArray = append(keyArray, key)
	}

	i.BlockIndexer.Add(keyArray, blk.Number)
	return nil
}

func addSigString(in AddressSignatureFilter, limit int) string {
	var addresses []string
	var signatures []string
	for i, a := range in.Addresses() {
		if i > limit {
			break
		}
		addresses = append(addresses, a.Pretty())
	}
	for i, s := range in.Signatures() {
		if i > limit {
			break
		}
		signatures = append(signatures, s.Pretty())
	}
	return fmt.Sprintf("{addrs: %s, sigs: %s}", strings.Join(addresses, ","), strings.Join(signatures, ","))

}

func truncate(in string, size int, suffix string) string {
	if tracer.Enabled() {
		return in
	}
	if len(in) < size {
		return in
	}
	return in[0:size] + suffix
}

func (f *CombinedFilter) String() string {
	limit := 5
	debug := os.Getenv("DEBUG_FILTERS") == "true"
	if debug {
		limit = 999999
	}

	callFilters := make([]string, len(f.CallToFilters))
	for i, f := range f.CallToFilters {
		callFilters[i] = addSigString(f, limit)
	}
	logFilters := make([]string, len(f.LogFilters))
	for i, f := range f.LogFilters {
		logFilters[i] = addSigString(f, limit)
	}

	if debug {
		return fmt.Sprintf("Combined filter: Calls:[%s], Logs:[%s], SendAllBlockHeaders: %v", strings.Join(callFilters, ","), strings.Join(logFilters, ","), f.sendAllBlockHeaders)
	}

	return fmt.Sprintf("Combined filter: Calls:[%s], Logs:[%s], SendAllBlockHeaders: %v", truncate(strings.Join(callFilters, ","), 90, "...}"), truncate(strings.Join(logFilters, ","), 90, "...}"), f.sendAllBlockHeaders)
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

func (f *CombinedFilter) Transform(readOnlyBlk *pbbstream.Block, in transform.Input) (transform.Output, error) {
	ethBlock := &pbeth.Block{}
	err := readOnlyBlk.Payload.UnmarshalTo(ethBlock)
	if err != nil {
		return nil, fmt.Errorf("mashalling block: %w", err)
	}

	traces := []*pbeth.TransactionTrace{}
	for _, trace := range ethBlock.TransactionTraces {
		if f.matches(trace) {
			traces = append(traces, trace)
		}
	}
	ethBlock.TransactionTraces = traces
	return ethBlock, nil
}

// GetIndexProvider will instantiate a new index conforming to the pbbstream.BlockIndexProvider interface
func (f *CombinedFilter) GetIndexProvider() bstream.BlockIndexProvider {
	if f.indexStore == nil {
		return nil
	}

	if f.sendAllBlockHeaders {
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
	return func(bitmaps transform.BitmapGetter) (matchingBlocks []uint64) {
		out := roaring64.NewBitmap()
		for _, f := range logFilters {
			fbit := filterBitmap(f, bitmaps, IdxPrefixLog)
			out.Or(fbit)
		}
		for _, f := range callFilters {
			fbit := filterBitmap(f, bitmaps, IdxPrefixCall)
			out.Or(fbit)
		}
		return nilIfEmpty(out.ToArray())
	}
}

func logKeys(trace *pbeth.TransactionTrace, prefix string) map[string]bool {
	out := make(map[string]bool)
	if trace.Receipt == nil {
		return out
	}
	for _, log := range trace.Receipt.Logs {
		if log == nil {
			continue
		}
		out[prefix+hex.EncodeToString(log.Address)] = true
		if len(log.Topics) != 0 {
			out[prefix+hex.EncodeToString(log.Topics[0])] = true
		}
	}
	return out
}
func callKeys(trace *pbeth.TransactionTrace, prefix string) map[string]bool {
	out := make(map[string]bool)
	for _, call := range trace.Calls {
		out[prefix+hex.EncodeToString(call.Address)] = true
		if sig := call.Method(); sig != nil {
			out[prefix+hex.EncodeToString(sig)] = true
		}
	}
	return out
}

type AddressSignatureFilter interface {
	Addresses() []eth.Address
	Signatures() []eth.Hash
}

// filterBitmap is a switchboard method which determines
// if we're interested in filtering the provided index by eth.Address, eth.Hash, or both
func filterBitmap(f AddressSignatureFilter, bitmaps transform.BitmapGetter, idxPrefix string) *roaring64.Bitmap {
	wantAddresses := len(f.Addresses()) != 0
	wantSigs := len(f.Signatures()) != 0

	switch {
	case wantAddresses && !wantSigs:
		return addressBitmap(f.Addresses(), bitmaps, idxPrefix)
	case wantSigs && !wantAddresses:
		return sigsBitmap(f.Signatures(), bitmaps, idxPrefix)
	case wantAddresses && wantSigs:
		a := addressBitmap(f.Addresses(), bitmaps, idxPrefix)
		b := sigsBitmap(f.Signatures(), bitmaps, idxPrefix)
		a.And(b)
		return a
	default:
		panic("filterBitmap: unsupported case")
	}
}

// addressBitmap attempts to find the blockNums corresponding to the provided eth.Address
func addressBitmap(addrs []eth.Address, bitmaps transform.BitmapGetter, idxPrefix string) *roaring64.Bitmap {
	out := roaring64.NewBitmap()
	for _, addr := range addrs {
		addrString := idxPrefix + addr.String()
		if bm := bitmaps.Get(addrString); bm != nil {
			out.Or(bm)
		}
	}
	return out
}

// sigsBitmap attemps to find the blockNums corresponding to the provided eth.Hash
func sigsBitmap(sigs []eth.Hash, bitmaps transform.BitmapGetter, idxPrefix string) *roaring64.Bitmap {
	out := roaring64.NewBitmap()
	for _, sig := range sigs {
		bm := bitmaps.Get(idxPrefix + sig.String())
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
