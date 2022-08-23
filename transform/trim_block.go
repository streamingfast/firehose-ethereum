package transform

import (
	"encoding/hex"
	"fmt"

	"github.com/streamingfast/bstream"
	"github.com/streamingfast/bstream/transform"
	pbtransform "github.com/streamingfast/sf-ethereum/types/pb/sf/ethereum/transform/v1"
	pbeth "github.com/streamingfast/sf-ethereum/types/pb/sf/ethereum/type/v2"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

var LightBlockMessageName = proto.MessageName(&pbtransform.LightBlock{})

var LightBlockFilterFactory = &transform.Factory{
	Obj: &pbtransform.LightBlock{},
	NewFunc: func(message *anypb.Any) (transform.Transform, error) {
		mname := message.MessageName()
		if mname != LightBlockMessageName {
			return nil, fmt.Errorf("expected type url %q, recevied %q ", LightBlockMessageName, message.TypeUrl)
		}

		filter := &pbtransform.LightBlock{}
		err := proto.Unmarshal(message.Value, filter)
		if err != nil {
			return nil, fmt.Errorf("unexpected unmarshall error: %w", err)
		}
		return &LightBlockFilter{}, nil
	},
}

type LightBlockFilter struct{}

func (p *LightBlockFilter) String() string {
	return "light block filter"
}

func (p *LightBlockFilter) Transform(readOnlyBlk *bstream.Block, in transform.Input) (transform.Output, error) {
	ethFullBlock := readOnlyBlk.ToProtocol().(*pbeth.Block)
	zlog.Debug("running light block transformer",
		zap.String("hash", hex.EncodeToString(ethFullBlock.Hash)),
		zap.Uint64("num", ethFullBlock.Num()),
	)

	// FIXME: The block is actually duplicated elsewhere which means that at this point,
	//        we work on our own copy of the block. So we can re-write this code to avoid
	//        all the extra allocation and simply nillify the values that we want to hide
	//        instead
	block := &pbeth.Block{
		Hash:   ethFullBlock.Hash,
		Number: ethFullBlock.Number,
		Header: &pbeth.BlockHeader{
			Timestamp:  ethFullBlock.Header.Timestamp,
			ParentHash: ethFullBlock.Header.ParentHash,
		},
	}

	var newTrace func(fullTrxTrace *pbeth.TransactionTrace) (trxTrace *pbeth.TransactionTrace)
	newTrace = func(fullTrxTrace *pbeth.TransactionTrace) (trxTrace *pbeth.TransactionTrace) {
		trxTrace = &pbeth.TransactionTrace{
			Hash:    fullTrxTrace.Hash,
			Receipt: fullTrxTrace.Receipt,
			From:    fullTrxTrace.From,
			To:      fullTrxTrace.To,
		}

		trxTrace.Calls = make([]*pbeth.Call, len(fullTrxTrace.Calls))
		for i, fullCall := range fullTrxTrace.Calls {
			call := &pbeth.Call{
				Index:        fullCall.Index,
				ParentIndex:  fullCall.ParentIndex,
				Depth:        fullCall.Depth,
				CallType:     fullCall.CallType,
				Caller:       fullCall.Caller,
				Address:      fullCall.Address,
				Value:        fullCall.Value,
				GasLimit:     fullCall.GasLimit,
				GasConsumed:  fullCall.GasConsumed,
				ReturnData:   fullCall.ReturnData,
				Input:        fullCall.Input,
				ExecutedCode: fullCall.ExecutedCode,
				Suicide:      fullCall.Suicide,
				Logs:         fullCall.Logs,
			}

			trxTrace.Calls[i] = call
		}

		return trxTrace
	}

	traces := make([]*pbeth.TransactionTrace, len(ethFullBlock.TransactionTraces))
	for i, fullTrxTrace := range ethFullBlock.TransactionTraces {
		traces[i] = newTrace(fullTrxTrace)
	}

	block.TransactionTraces = traces

	return block, nil
}
