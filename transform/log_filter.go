package transform

import (
	"bytes"
	"fmt"

	"github.com/streamingfast/dstore"

	"github.com/streamingfast/eth-go"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/streamingfast/bstream"
	"github.com/streamingfast/bstream/transform"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/codec/v1"
	pbtransform "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/transform/v1"
	"google.golang.org/protobuf/proto"
)

var LogFilterMessageName = proto.MessageName(&pbtransform.LogFilter{})
var MultiLogFilterMessageName = proto.MessageName(&pbtransform.MultiLogFilter{})

func LogFilterFactory(indexStore dstore.Store, possibleIndexSizes []uint64) *transform.Factory {
	return &transform.Factory{
		Obj: &pbtransform.LogFilter{},
		NewFunc: func(message *anypb.Any) (transform.Transform, error) {
			mname := message.MessageName()
			if mname != LogFilterMessageName {
				return nil, fmt.Errorf("expected type url %q, recevied %q ", LogFilterMessageName, message.TypeUrl)
			}

			filter := &pbtransform.LogFilter{}
			err := proto.Unmarshal(message.Value, filter)
			if err != nil {
				return nil, fmt.Errorf("unexpected unmarshall error: %w", err)
			}

			if len(filter.Addresses) == 0 && len(filter.EventSignatures) == 0 {
				return nil, fmt.Errorf("a log filter transform requires at-least one address or one event signature")
			}

			f := &LogFilter{
				indexStore:         indexStore,
				possibleIndexSizes: possibleIndexSizes,
			}

			for _, addr := range filter.Addresses {
				f.Addresses = append(f.Addresses, addr)
			}
			for _, sig := range filter.EventSignatures {
				f.EventSigntures = append(f.EventSigntures, sig)
			}

			return f, nil
		},
	}
}

type LogFilter struct {
	Addresses      []eth.Address
	EventSigntures []eth.Hash

	indexStore         dstore.Store
	possibleIndexSizes []uint64
}

func (p *LogFilter) matchAddress(src eth.Address) bool {
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

func (p *LogFilter) matchEventSignature(src eth.Hash) bool {
	if len(p.EventSigntures) == 0 {
		return true
	}
	for _, topic := range p.EventSigntures {
		if bytes.Equal(topic, src) {
			return true
		}
	}
	return false
}

func (p *LogFilter) Transform(readOnlyBlk *bstream.Block, in transform.Input) (transform.Output, error) {
	ethBlock := readOnlyBlk.ToProtocol().(*pbcodec.Block)
	traces := []*pbcodec.TransactionTrace{}
	for _, trace := range ethBlock.TransactionTraces {
		match := false
		for _, log := range trace.Receipt.Logs {
			if p.matchAddress(log.Address) && p.matchEventSignature(log.Topics[0]) {
				match = true
				break
			}
		}
		if match {
			traces = append(traces, trace)
		}
	}
	ethBlock.TransactionTraces = traces
	return ethBlock, nil
}

// GetIndexProvider will instantiate a new LogAddressIndex conforming to the bstream.BlockIndexProvider interface
func (p *LogFilter) GetIndexProvider() bstream.BlockIndexProvider {
	if p.indexStore == nil {
		return nil
	}

	if len(p.Addresses) == 0 && len(p.EventSigntures) == 0 {
		return nil
	}

	filter := &logAddressSingleFilter{
		p.Addresses,
		p.EventSigntures,
	}
	return NewEthBlockIndexProvider(
		p.indexStore,
		p.possibleIndexSizes,
		[]*logAddressSingleFilter{filter},
	)
}

func MultiLogFilterFactory(indexStore dstore.Store, possibleIndexSizes []uint64) *transform.Factory {
	return &transform.Factory{
		Obj: &pbtransform.MultiLogFilter{},
		NewFunc: func(message *anypb.Any) (transform.Transform, error) {
			mname := message.MessageName()
			if mname != MultiLogFilterMessageName {
				return nil, fmt.Errorf("expected type url %q, recevied %q ", LogFilterMessageName, message.TypeUrl)
			}

			filter := &pbtransform.MultiLogFilter{}
			err := proto.Unmarshal(message.Value, filter)
			if err != nil {
				return nil, fmt.Errorf("unexpected unmarshall error: %w", err)
			}

			if len(filter.LogFilters) == 0 {
				return nil, fmt.Errorf("a multi log filter transform requires at-least one basic log filter")
			}

			f := &MultiLogFilter{
				indexStore:         indexStore,
				possibleIndexSizes: possibleIndexSizes,
			}

			for _, bf := range filter.LogFilters {
				if len(bf.Addresses) == 0 && len(bf.EventSignatures) == 0 {
					return nil, fmt.Errorf("a log filter transform requires at-least one address or one event signature")
				}
				ff := LogFilter{}

				for _, addr := range bf.Addresses {
					ff.Addresses = append(ff.Addresses, addr)
				}
				for _, sig := range bf.EventSignatures {
					ff.EventSigntures = append(ff.EventSigntures, sig)
				}
				f.filters = append(f.filters, ff)
			}

			return f, nil
		},
	}
}

type MultiLogFilter struct {
	filters            []LogFilter
	indexStore         dstore.Store
	possibleIndexSizes []uint64
}

func (p *MultiLogFilter) Transform(readOnlyBlk *bstream.Block, in transform.Input) (transform.Output, error) {
	ethBlock := readOnlyBlk.ToProtocol().(*pbcodec.Block)
	traces := []*pbcodec.TransactionTrace{}
	for _, trace := range ethBlock.TransactionTraces {
		match := false
		for _, log := range trace.Receipt.Logs {
			for _, filter := range p.filters {
				if filter.matchAddress(log.Address) && filter.matchEventSignature(log.Topics[0]) {
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

// GetIndexProvider will instantiate a new LogAddressIndex conforming to the bstream.BlockIndexProvider interface
func (p *MultiLogFilter) GetIndexProvider() bstream.BlockIndexProvider {
	if p.indexStore == nil {
		return nil
	}

	if len(p.filters) == 0 {
		return nil
	}
	var filters []*logAddressSingleFilter
	for _, f := range p.filters {
		filters = append(filters, &logAddressSingleFilter{
			addrs:     f.Addresses,
			eventSigs: f.EventSigntures,
		})
	}

	return NewEthBlockIndexProvider(
		p.indexStore,
		p.possibleIndexSizes,
		filters,
	)
}
