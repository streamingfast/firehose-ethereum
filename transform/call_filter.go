package transform

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/streamingfast/bstream"
	"github.com/streamingfast/bstream/transform"
	"github.com/streamingfast/dstore"
	"github.com/streamingfast/eth-go"
	pbtransform "github.com/streamingfast/sf-ethereum/types/pb/sf/ethereum/transform/v1"
	pbeth "github.com/streamingfast/sf-ethereum/types/pb/sf/ethereum/type/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

var CallToFilterMessageName = proto.MessageName(&pbtransform.CallToFilter{})
var MultiCallToFilterMessageName = proto.MessageName(&pbtransform.MultiCallToFilter{})

func CallToFilterFactory(indexStore dstore.Store, possibleIndexSizes []uint64) *transform.Factory {
	return &transform.Factory{
		Obj: &pbtransform.CallToFilter{},
		NewFunc: func(message *anypb.Any) (transform.Transform, error) {
			mname := message.MessageName()
			if mname != CallToFilterMessageName {
				return nil, fmt.Errorf("expected type url %q, recevied %q ", CallToFilterMessageName, message.TypeUrl)
			}

			filter := &pbtransform.CallToFilter{}
			err := proto.Unmarshal(message.Value, filter)
			if err != nil {
				return nil, fmt.Errorf("unexpected unmarshall error: %w", err)
			}
			f := &CallToFilter{
				indexStore:         indexStore,
				possibleIndexSizes: possibleIndexSizes,
			}
			if err := f.load(filter); err != nil {
				return nil, err
			}
			return f, nil
		},
	}
}

type CallToFilter struct {
	Addresses  []eth.Address
	Signatures []eth.Hash

	indexStore         dstore.Store
	possibleIndexSizes []uint64
}

func (f *CallToFilter) load(in *pbtransform.CallToFilter) error {
	if len(in.Addresses) == 0 && len(in.Signatures) == 0 {
		return fmt.Errorf("a call filter transform requires at-least one address or one method signature")
	}

	for _, addr := range in.Addresses {
		f.Addresses = append(f.Addresses, addr)
	}
	for _, sig := range in.Signatures {
		f.Signatures = append(f.Signatures, sig)
	}

	return nil

}

func (p *CallToFilter) String() string {
	var addresses []string
	var signatures []string
	for _, a := range p.Addresses {
		addresses = append(addresses, a.Pretty())
	}
	for _, s := range p.Signatures {
		signatures = append(signatures, s.Pretty())
	}
	return fmt.Sprintf("CallFilter:{addrs: %s, sigs: %s}", strings.Join(addresses, ","), strings.Join(signatures, ","))

}

func (p *CallToFilter) matchAddress(src eth.Address) bool {
	if len(p.Addresses) == 0 {
		return true
	}
	for _, addr := range p.Addresses {
		if bytes.Equal(addr, src) {
			return true
		}
	}
	return false
}

func (p *CallToFilter) matchSignature(src eth.Hash) bool {
	if len(p.Signatures) == 0 {
		return true
	}
	for _, topic := range p.Signatures {
		if bytes.Equal(topic, src) {
			return true
		}
	}
	return false
}

func (p *CallToFilter) matches(trace *pbeth.TransactionTrace) bool {
	for _, call := range trace.Calls {
		if p.matchAddress(call.Address) && p.matchSignature(call.Method()) {
			return true
		}
	}
	return false
}

func (p *CallToFilter) Transform(readOnlyBlk *bstream.Block, in transform.Input) (transform.Output, error) {
	ethBlock := readOnlyBlk.ToProtocol().(*pbeth.Block)
	traces := []*pbeth.TransactionTrace{}
	for _, trace := range ethBlock.TransactionTraces {
		if p.matches(trace) {
			traces = append(traces, trace)
		}
	}
	ethBlock.TransactionTraces = traces
	return ethBlock, nil
}

// GetIndexProvider will instantiate a new CallToAddressIndex conforming to the bstream.BlockIndexProvider interface
func (p *CallToFilter) GetIndexProvider() bstream.BlockIndexProvider {
	if p.indexStore == nil {
		return nil
	}

	if len(p.Addresses) == 0 && len(p.Signatures) == 0 {
		return nil
	}

	filter := &addrSigSingleFilter{
		p.Addresses,
		p.Signatures,
	}
	return NewEthCallIndexProvider(
		p.indexStore,
		p.possibleIndexSizes,
		[]*addrSigSingleFilter{filter},
	)
}

func MultiCallToFilterFactory(indexStore dstore.Store, possibleIndexSizes []uint64) *transform.Factory {
	return &transform.Factory{
		Obj: &pbtransform.MultiCallToFilter{},
		NewFunc: func(message *anypb.Any) (transform.Transform, error) {
			mname := message.MessageName()
			if mname != MultiCallToFilterMessageName {
				return nil, fmt.Errorf("expected type url %q, recevied %q ", LogFilterMessageName, message.TypeUrl)
			}

			filter := &pbtransform.MultiCallToFilter{}
			err := proto.Unmarshal(message.Value, filter)
			if err != nil {
				return nil, fmt.Errorf("unexpected unmarshall error: %w", err)
			}

			if len(filter.CallFilters) == 0 {
				return nil, fmt.Errorf("a multi log filter transform requires at-least one basic log filter")
			}

			f := &MultiCallToFilter{
				indexStore:         indexStore,
				possibleIndexSizes: possibleIndexSizes,
			}

			for _, bf := range filter.CallFilters {
				if len(bf.Addresses) == 0 && len(bf.Signatures) == 0 {
					return nil, fmt.Errorf("a log filter transform requires at-least one address or one event signature")
				}
				ff := CallToFilter{}

				for _, addr := range bf.Addresses {
					ff.Addresses = append(ff.Addresses, addr)
				}
				for _, sig := range bf.Signatures {
					ff.Signatures = append(ff.Signatures, sig)
				}
				f.filters = append(f.filters, ff)
			}

			return f, nil
		},
	}
}

type MultiCallToFilter struct {
	filters            []CallToFilter
	indexStore         dstore.Store
	possibleIndexSizes []uint64
}

func (p *MultiCallToFilter) String() string {
	var descs []string
	for _, f := range p.filters {
		descs = append(descs, f.String())
	}
	return fmt.Sprintf("(%s)", strings.Join(descs, "),("))
}

func (p *MultiCallToFilter) Transform(readOnlyBlk *bstream.Block, in transform.Input) (transform.Output, error) {
	ethBlock := readOnlyBlk.ToProtocol().(*pbeth.Block)
	traces := []*pbeth.TransactionTrace{}
	for _, trace := range ethBlock.TransactionTraces {
		match := false
		for _, call := range trace.Calls {
			for _, filter := range p.filters {
				if filter.matchAddress(call.Address) && filter.matchSignature(call.Method()) {
					match = true
					break // a single filter matching is enough
				}
			}
		}
		if match {
			traces = append(traces, trace)
		}
	}
	ethBlock.TransactionTraces = traces
	return ethBlock, nil
}

// GetIndexProvider will instantiate a new CallAddressIndex conforming to the bstream.BlockIndexProvider interface
func (p *MultiCallToFilter) GetIndexProvider() bstream.BlockIndexProvider {
	if p.indexStore == nil {
		return nil
	}

	if len(p.filters) == 0 {
		return nil
	}
	var filters []*addrSigSingleFilter
	for _, f := range p.filters {
		filters = append(filters, &addrSigSingleFilter{
			addrs: f.Addresses,
			sigs:  f.Signatures,
		})
	}

	return NewEthCallIndexProvider(
		p.indexStore,
		p.possibleIndexSizes,
		filters,
	)
}
