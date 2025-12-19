package blockfetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	"github.com/streamingfast/eth-go/rpc"
	"github.com/streamingfast/firehose-ethereum/block"
	"go.uber.org/zap"
)

type TracerBlockFetcher struct {
	fetcher *BlockFetcher
}

func (f *TracerBlockFetcher) IsBlockAvailable(requested uint64) bool {
	return f.fetcher.IsBlockAvailable(requested)
}

func (f *TracerBlockFetcher) Fetch(ctx context.Context, client *rpc.Client, blkNum uint64) (b *pbbstream.Block, skipped bool, err error) {
	respStr, err := client.DoRequest(ctx, "debug_traceFirehoseBlockByNumber", []any{fmt.Sprintf("0x%x", blkNum), nil})
	if err != nil {
		return nil, false, fmt.Errorf("failed to trace block %d: %w", blkNum, err)
	}

	var tracedBlock *pbbstream.Block
	if err := json.Unmarshal([]byte(respStr), &tracedBlock); err != nil {
		return nil, false, fmt.Errorf("failed to unmarshal traced block JSON for block %d: %w", blkNum, err)
	}

	return tracedBlock, false, nil
}

func NewTracerBlockFetcher(intervalBetweenFetch time.Duration,
	latestBlockRetryInterval time.Duration,
	parallelTrxWorkers int,
	allowEmptyReceiptsOnBlock0 bool,
	logger *zap.Logger) *TracerBlockFetcher {
	fetcher := NewBlockFetcher(intervalBetweenFetch, latestBlockRetryInterval, parallelTrxWorkers, block.RpcToEthBlock, logger)
	if allowEmptyReceiptsOnBlock0 {
		fetcher.allowEmptyReceiptsOnBlock0 = true
	}
	return &TracerBlockFetcher{
		fetcher: fetcher,
	}
}

func (f *TracerBlockFetcher) PollingInterval() time.Duration { return 5 * time.Second }
