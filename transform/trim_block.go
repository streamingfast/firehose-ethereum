package transform

import (
	"encoding/hex"
	"fmt"

	"github.com/streamingfast/bstream"
	"github.com/streamingfast/bstream/transform"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/codec/v1"
	pbtransform "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/transform/v1"
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
	return "light_block_filter"
}

func (p *LightBlockFilter) Transform(readOnlyBlk *bstream.Block, in transform.Input) (transform.Output, error) {
	ethFullBlock := readOnlyBlk.ToProtocol().(*pbcodec.Block)
	zlog.Debug("running light block transformer",
		zap.String("hash", hex.EncodeToString(ethFullBlock.Hash)),
		zap.Uint64("num", ethFullBlock.Num()),
	)

	// We analyze here to ensure that they are set correctly as they are used when computing the light version
	ethFullBlock.Analyze()

	// FIXME: The block is actually duplicated elsewhere which means that at this point,
	//        we work on our own copy of the block. So we can re-write this code to avoid
	//        all the extra allocation and simply nillify the values that we want to hide
	//        instead
	block := &pbcodec.Block{
		Hash:   ethFullBlock.Hash,
		Number: ethFullBlock.Number,
		Header: &pbcodec.BlockHeader{
			Timestamp:  ethFullBlock.Header.Timestamp,
			ParentHash: ethFullBlock.Header.ParentHash,
		},
	}

	var newTrace func(fullTrxTrace *pbcodec.TransactionTrace) (trxTrace *pbcodec.TransactionTrace)
	newTrace = func(fullTrxTrace *pbcodec.TransactionTrace) (trxTrace *pbcodec.TransactionTrace) {
		trxTrace = &pbcodec.TransactionTrace{
			Hash:    fullTrxTrace.Hash,
			Receipt: fullTrxTrace.Receipt,
			From:    fullTrxTrace.From,
			To:      fullTrxTrace.To,
		}

		trxTrace.Calls = make([]*pbcodec.Call, len(fullTrxTrace.Calls))
		for i, fullCall := range fullTrxTrace.Calls {
			call := &pbcodec.Call{
				Index:               fullCall.Index,
				ParentIndex:         fullCall.ParentIndex,
				Depth:               fullCall.Depth,
				CallType:            fullCall.CallType,
				Caller:              fullCall.Caller,
				Address:             fullCall.Address,
				Value:               fullCall.Value,
				GasLimit:            fullCall.GasLimit,
				GasConsumed:         fullCall.GasConsumed,
				ReturnData:          fullCall.ReturnData,
				Input:               fullCall.Input,
				ExecutedCode:        fullCall.ExecutedCode,
				Suicide:             fullCall.Suicide,
				Logs:                fullCall.Logs,
				Erc20BalanceChanges: fullCall.Erc20BalanceChanges,
				Erc20TransferEvents: fullCall.Erc20TransferEvents,
			}

			trxTrace.Calls[i] = call
		}

		return trxTrace
	}

	traces := make([]*pbcodec.TransactionTrace, len(ethFullBlock.TransactionTraces))
	for i, fullTrxTrace := range ethFullBlock.TransactionTraces {
		traces[i] = newTrace(fullTrxTrace)
	}

	block.TransactionTraces = traces

	return block, nil
}
