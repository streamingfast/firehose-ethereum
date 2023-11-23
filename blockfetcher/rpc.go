package blockfetcher

import (
	"context"
	"fmt"
	"time"

	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"

	"github.com/streamingfast/eth-go/rpc"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ToEthBlock func(in *rpc.Block, logs []*rpc.LogEntry) (*pbeth.Block, map[string]bool)

type BlockFetcher struct {
	rpcClient                *rpc.Client
	latest                   uint64
	latestBlockRetryInterval time.Duration
	toEthBlock               ToEthBlock
}

func NewBlockFetcher(rpcClient *rpc.Client, latestBlockRetryInterval time.Duration, toEthBlock ToEthBlock) *BlockFetcher {
	return &BlockFetcher{
		rpcClient:                rpcClient,
		latestBlockRetryInterval: latestBlockRetryInterval,
		toEthBlock:               toEthBlock,
	}
}

func (f *BlockFetcher) Fetch(ctx context.Context, blockNum uint64) (block *pbbstream.Block, err error) {
	for f.latest <= blockNum {
		latest, err := f.rpcClient.LatestBlockNum(ctx)
		if err != nil {
			return nil, fmt.Errorf("fetching latest block num: %w", err)
		}

		if latest <= blockNum {
			time.Sleep(f.latestBlockRetryInterval)
			continue
		}
		break
	}

	rpcBlock, err := f.rpcClient.GetBlockByNumber(ctx, rpc.BlockNumber(blockNum), rpc.WithGetBlockFullTransaction())
	if err != nil {
		return nil, fmt.Errorf("fetching block %d: %w", blockNum, err)
	}

	logs, err := f.rpcClient.Logs(ctx, rpc.LogsParams{
		FromBlock: rpc.BlockNumber(blockNum),
		ToBlock:   rpc.BlockNumber(blockNum),
	})

	if err != nil {
		return nil, fmt.Errorf("fetching logs for block %d %q: %w", rpcBlock.Number, rpcBlock.Hash.Pretty(), err)
	}

	ethBlock, _ := f.toEthBlock(rpcBlock, logs)
	anyBlock, err := anypb.New(ethBlock)
	if err != nil {
		return nil, fmt.Errorf("create any block: %w", err)
	}

	return &pbbstream.Block{
		Number:    ethBlock.Number,
		Id:        ethBlock.GetFirehoseBlockID(),
		ParentId:  ethBlock.GetFirehoseBlockParentID(),
		Timestamp: timestamppb.New(ethBlock.GetFirehoseBlockTime()),
		LibNum:    ethBlock.LIBNum(),
		ParentNum: ethBlock.GetFirehoseBlockParentNumber(),
		Payload:   anyBlock,
	}, nil
}
