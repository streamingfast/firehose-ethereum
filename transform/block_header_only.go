package transform

import (
	"encoding/hex"
	"fmt"

	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	"github.com/streamingfast/bstream/transform"
	"github.com/streamingfast/dstore"
	pbtransform "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/transform/v1"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

var HeaderOnlyMessageName = proto.MessageName(&pbtransform.HeaderOnly{})

func NewHeaderOnlyTransformFactory(_ dstore.Store, _ []uint64) (*transform.Factory, error) {
	return HeaderOnlyTransformFactory, nil
}

var HeaderOnlyTransformFactory = &transform.Factory{
	Obj: &pbtransform.HeaderOnly{},
	NewFunc: func(message *anypb.Any) (transform.Transform, error) {
		mname := message.MessageName()
		if mname != HeaderOnlyMessageName {
			return nil, fmt.Errorf("expected type url %q, recevied %q ", HeaderOnlyMessageName, message.TypeUrl)
		}

		filter := &pbtransform.HeaderOnly{}
		err := proto.Unmarshal(message.Value, filter)
		if err != nil {
			return nil, fmt.Errorf("unexpected unmarshall error: %w", err)
		}
		return &HeaderOnlyFilter{}, nil
	},
}

type HeaderOnlyFilter struct{}

func (p *HeaderOnlyFilter) String() string {
	return "light block filter"
}

func (p *HeaderOnlyFilter) Transform(readOnlyBlk *pbbstream.Block, in transform.Input) (transform.Output, error) {

	ethFullBlock := &pbeth.Block{}
	err := readOnlyBlk.Payload.UnmarshalTo(ethFullBlock)
	if err != nil {
		return nil, fmt.Errorf("mashalling block: %w", err)
	}

	zlog.Debug("running header only transformer",
		zap.String("hash", hex.EncodeToString(ethFullBlock.Hash)),
		zap.Uint64("num", ethFullBlock.Num()),
	)

	// FIXME: The block is actually duplicated elsewhere which means that at this point,
	//        we work on our own copy of the block. So we can re-write this code to avoid
	//        all the extra allocation and simply nillify the values that we want to hide
	//        instead
	return &pbeth.Block{
		Ver:    ethFullBlock.Ver,
		Hash:   ethFullBlock.Hash,
		Number: ethFullBlock.Number,
		Size:   ethFullBlock.Size,
		Header: ethFullBlock.Header,
	}, nil
}
