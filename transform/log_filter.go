package transform

import (
	"bytes"
	"fmt"
	"github.com/streamingfast/eth-go"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/streamingfast/bstream"
	"github.com/streamingfast/bstream/transform"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/codec/v1"
	pbtransforms "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/transforms/v1"
	"google.golang.org/protobuf/proto"
)

var LogFilterMessageName = proto.MessageName(&pbtransforms.BasicLogFilter{})

var NewBasicLogFilterFactory = func(message *anypb.Any) (transform.Transform, error) {
	mname := message.MessageName()
	if mname != LogFilterMessageName {
		return nil, fmt.Errorf("expected type url %q, recevied %q ", LogFilterMessageName, message.TypeUrl)
	}

	filter := &pbtransforms.BasicLogFilter{}
	err := proto.Unmarshal(message.Value, filter)
	if err != nil {
		return nil, fmt.Errorf("unexpected unmarshall error: %w", err)
	}

	f := &BasicLogFilter{}
	for _, addr := range filter.Addresses {
		f.Addresses = append(f.Addresses, addr)
	}
	for _, sig := range filter.EventSignatures {
		f.EventSigntures = append(f.EventSigntures, sig)
	}
	return f, nil
}

type BasicLogFilter struct {
	Addresses      []eth.Address
	EventSigntures []eth.Hash
}

func (p *BasicLogFilter) matchAddress(src eth.Address) bool {
	for _, addr := range p.Addresses {
		if bytes.Equal(addr, src) {
			return true
		}
	}
	return false
}

func (p *BasicLogFilter) matchTopic(src eth.Hash) bool {
	for _, topic := range p.EventSigntures {
		if bytes.Equal(topic, src) {
			return true
		}
	}
	return false
}

func (p *BasicLogFilter) matchCall(call *pbcodec.Call) bool {
	for _, log := range call.Logs {
		if p.matchAddress(log.Address) {
			return true
		}

		if p.matchTopic(log.Topics[0]) {
			return true
		}
	}
	return false
}

func (p *BasicLogFilter) Transform(readOnlyBlk *bstream.Block, in transform.Input) (transform.Output, error) {
	ethBlock := readOnlyBlk.ToProtocol().(*pbcodec.Block)
	traces := []*pbcodec.TransactionTrace{}
	for _, trace := range ethBlock.TransactionTraces {
		match := false
		for _, call := range trace.Calls {
			if p.matchCall(call) {
				match = true
			}
		}
		if match {
			traces = append(traces, trace)
		}
	}
	ethBlock.TransactionTraces = traces
	return ethBlock, nil
}
