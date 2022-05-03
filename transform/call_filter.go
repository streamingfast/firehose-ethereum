package transform

import (
	"bytes"
	"fmt"

	"github.com/streamingfast/bstream/transform"
	"github.com/streamingfast/dstore"
	"github.com/streamingfast/eth-go"
	pbtransform "github.com/streamingfast/sf-ethereum/types/pb/sf/ethereum/transform/v1"
	pbeth "github.com/streamingfast/sf-ethereum/types/pb/sf/ethereum/type/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

var MultiCallToFilterMessageName = proto.MessageName(&pbtransform.MultiCallToFilter{})

type CallToFilter struct {
	addresses  []eth.Address
	signatures []eth.Hash
}

func (f *CallToFilter) Addresses() []eth.Address {
	return f.addresses
}

func (f *CallToFilter) Signatures() []eth.Hash {
	return f.signatures
}

func NewCallToFilter(in *pbtransform.CallToFilter) (*CallToFilter, error) {
	if len(in.Addresses) == 0 && len(in.Signatures) == 0 {
		return nil, fmt.Errorf("a call filter transform requires at-least one address or one method signature")
	}

	f := &CallToFilter{}
	for _, addr := range in.Addresses {
		f.addresses = append(f.addresses, addr)
	}
	for _, sig := range in.Signatures {
		f.signatures = append(f.signatures, sig)
	}

	return f, nil

}

func (p *CallToFilter) matchAddress(src eth.Address) bool {
	if len(p.addresses) == 0 {
		return true
	}
	for _, addr := range p.addresses {
		if bytes.Equal(addr, src) {
			return true
		}
	}
	return false
}

func (p *CallToFilter) matchSignature(src eth.Hash) bool {
	if len(p.signatures) == 0 {
		return true
	}
	for _, topic := range p.signatures {
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

// backwards compatibility, returns a combined filter now
func MultiCallToFilterFactory(indexStore dstore.Store, possibleIndexSizes []uint64) *transform.Factory {
	return &transform.Factory{
		Obj: &pbtransform.MultiCallToFilter{},
		NewFunc: func(message *anypb.Any) (transform.Transform, error) {
			mname := message.MessageName()
			if mname != MultiCallToFilterMessageName {
				return nil, fmt.Errorf("expected type url %q, recevied %q", MultiCallToFilterMessageName, message.TypeUrl)
			}

			filter := &pbtransform.MultiCallToFilter{}
			err := proto.Unmarshal(message.Value, filter)
			if err != nil {
				return nil, fmt.Errorf("unexpected unmarshall error: %w", err)
			}

			return newCombinedFilter(filter.CallFilters, nil, indexStore, possibleIndexSizes)
		},
	}
}
