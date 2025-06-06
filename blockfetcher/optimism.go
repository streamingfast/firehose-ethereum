package blockfetcher

import (
	"context"
	"time"

	"go.uber.org/zap"

	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	"github.com/streamingfast/eth-go/rpc"
	"github.com/streamingfast/firehose-ethereum/block"
)

type OptimismBlockFetcher struct {
	fetcher *BlockFetcher
}

func (f *OptimismBlockFetcher) IsBlockAvailable(requested uint64) bool {
	return f.fetcher.IsBlockAvailable(requested)
}

func (f *OptimismBlockFetcher) Fetch(ctx context.Context, rpcClient *rpc.Client, blockNum uint64) (b *pbbstream.Block, skipped bool, err error) {
	blk, err := f.fetcher.Fetch(ctx, rpcClient, blockNum)
	return blk, false, err
}

func NewOptimismBlockFetcher(intervalBetweenFetch time.Duration, latestBlockRetryInterval time.Duration, parallelTrxWorkers int, logger *zap.Logger) *OptimismBlockFetcher {
	fetcher := NewBlockFetcher(intervalBetweenFetch, latestBlockRetryInterval, parallelTrxWorkers, block.RpcToEthBlock, logger)
	return &OptimismBlockFetcher{
		fetcher: fetcher,
	}
}

func (f *OptimismBlockFetcher) PollingInterval() time.Duration { return 5 * time.Second }
