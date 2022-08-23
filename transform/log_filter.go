package transform

import (
	"bytes"
	"fmt"

	"github.com/streamingfast/bstream/transform"
	"github.com/streamingfast/dstore"
	"github.com/streamingfast/eth-go"
	pbtransform "github.com/streamingfast/sf-ethereum/types/pb/sf/ethereum/transform/v1"
	pbeth "github.com/streamingfast/sf-ethereum/types/pb/sf/ethereum/type/v2"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

var MultiLogFilterMessageName = proto.MessageName(&pbtransform.MultiLogFilter{})

type LogFilter struct {
	addresses       []eth.Address
	eventSignatures []eth.Hash
}

func (f *LogFilter) Addresses() []eth.Address {
	return f.addresses
}

func (f *LogFilter) Signatures() []eth.Hash {
	return f.eventSignatures
}

func NewLogFilter(in *pbtransform.LogFilter) (*LogFilter, error) {
	if len(in.Addresses) == 0 && len(in.EventSignatures) == 0 {
		return nil, fmt.Errorf("a log filter transform requires at-least one address or one event signature")
	}

	f := &LogFilter{
		addresses:       make([]eth.Address, len(in.Addresses)),
		eventSignatures: make([]eth.Hash, len(in.EventSignatures)),
	}
	for i, addr := range in.Addresses {
		f.addresses[i] = addr
	}
	for i, sig := range in.EventSignatures {
		f.eventSignatures[i] = sig
	}
	return f, nil
}

func (p *LogFilter) matchAddress(src eth.Address) bool {
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

func (p *LogFilter) matchEventSignature(topics [][]byte) bool {
	if len(p.eventSignatures) == 0 {
		return true
	}
	if len(topics) == 0 {
		return false
	}
	for _, topic := range p.eventSignatures {
		if bytes.Equal(topic, topics[0]) {
			return true
		}
	}
	return false
}

func (p *LogFilter) matches(trace *pbeth.TransactionTrace) bool {
	for _, log := range trace.Receipt.Logs {
		if p.matchAddress(log.Address) && p.matchEventSignature(log.Topics) {
			return true
		}
	}
	return false
}

// backwards compatibility, returns a combined filter now
func MultiLogFilterFactory(indexStore dstore.Store, possibleIndexSizes []uint64) *transform.Factory {
	return &transform.Factory{
		Obj: &pbtransform.MultiLogFilter{},
		NewFunc: func(message *anypb.Any) (transform.Transform, error) {
			mname := message.MessageName()
			if mname != MultiLogFilterMessageName {
				return nil, fmt.Errorf("expected type url %q, recevied %q ", MultiLogFilterMessageName, message.TypeUrl)
			}

			filter := &pbtransform.MultiLogFilter{}
			err := proto.Unmarshal(message.Value, filter)
			if err != nil {
				return nil, fmt.Errorf("unexpected unmarshall error: %w", err)
			}
			return newCombinedFilter(nil, filter.LogFilters, indexStore, possibleIndexSizes)
		},
	}
}
