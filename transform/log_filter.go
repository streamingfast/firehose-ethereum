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
				f.EventSignatures = append(f.EventSignatures, sig)
			}

			return f, nil
		},
	}
}

type LogFilter struct {
	Addresses       []eth.Address
	EventSignatures []eth.Hash

	indexStore         dstore.Store
	possibleIndexSizes []uint64
}

func (p *LogFilter) String() string {
	var addresses []string
	var signatures []string
	for _, a := range p.Addresses {
		addresses = append(addresses, a.Pretty())
	}
	for _, s := range p.EventSignatures {
		signatures = append(signatures, s.Pretty())
	}
	return fmt.Sprintf("LogFilter{addrs: %s, evt_sigs: %s}", strings.Join(addresses, ","), strings.Join(signatures, ","))

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

func (p *LogFilter) matchEventSignature(topics [][]byte) bool {
	if len(p.EventSignatures) == 0 {
		return true
	}
	if len(topics) == 0 {
		return false
	}
	for _, topic := range p.EventSignatures {
		if bytes.Equal(topic, topics[0]) {
			return true
		}
	}
	return false
}

func (p *LogFilter) Transform(readOnlyBlk *bstream.Block, in transform.Input) (transform.Output, error) {
	ethBlock := readOnlyBlk.ToProtocol().(*pbeth.Block)
	traces := []*pbeth.TransactionTrace{}
	for _, trace := range ethBlock.TransactionTraces {
		match := false
		for _, log := range trace.Receipt.Logs {
			if p.matchAddress(log.Address) && p.matchEventSignature(log.Topics) {
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

	if len(p.Addresses) == 0 && len(p.EventSignatures) == 0 {
		return nil
	}

	filter := &addrSigSingleFilter{
		p.Addresses,
		p.EventSignatures,
	}
	return NewEthLogIndexProvider(
		p.indexStore,
		p.possibleIndexSizes,
		[]*addrSigSingleFilter{filter},
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
					ff.EventSignatures = append(ff.EventSignatures, sig)
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

func (p *MultiLogFilter) String() string {
	var descs []string
	for _, f := range p.filters {
		descs = append(descs, f.String())
	}
	return fmt.Sprintf("(%s)", strings.Join(descs, "),("))
}

func (p *MultiLogFilter) Transform(readOnlyBlk *bstream.Block, in transform.Input) (transform.Output, error) {
	ethBlock := readOnlyBlk.ToProtocol().(*pbeth.Block)
	traces := []*pbeth.TransactionTrace{}
	for _, trace := range ethBlock.TransactionTraces {
		match := false
		for _, log := range trace.Receipt.Logs {
			for _, filter := range p.filters {

				if filter.matchAddress(log.Address) && filter.matchEventSignature(log.Topics) {
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
	var filters []*addrSigSingleFilter
	for _, f := range p.filters {
		filters = append(filters, &addrSigSingleFilter{
			addrs: f.Addresses,
			sigs:  f.EventSignatures,
		})
	}

	return NewEthLogIndexProvider(
		p.indexStore,
		p.possibleIndexSizes,
		filters,
	)
}
