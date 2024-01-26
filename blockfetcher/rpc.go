package blockfetcher

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/abourget/llerrgroup"
	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	"github.com/streamingfast/eth-go/rpc"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ToEthBlock func(in *rpc.Block, receipts map[string]*rpc.TransactionReceipt) (*pbeth.Block, map[string]bool)

type BlockFetcher struct {
	rpcClient                *rpc.Client
	latest                   uint64
	latestBlockRetryInterval time.Duration
	fetchInterval            time.Duration
	toEthBlock               ToEthBlock
	lastFetchAt              time.Time
	logger                   *zap.Logger
}

func NewBlockFetcher(rpcClient *rpc.Client, intervalBetweenFetch, latestBlockRetryInterval time.Duration, toEthBlock ToEthBlock, logger *zap.Logger) *BlockFetcher {
	return &BlockFetcher{
		rpcClient:                rpcClient,
		latestBlockRetryInterval: latestBlockRetryInterval,
		toEthBlock:               toEthBlock,
		fetchInterval:            intervalBetweenFetch,
		logger:                   logger,
	}
}

func (f *BlockFetcher) Fetch(ctx context.Context, blockNum uint64) (block *pbbstream.Block, err error) {
	f.logger.Debug("fetching block", zap.Uint64("block_num", blockNum))
	for f.latest < blockNum {
		f.latest, err = f.rpcClient.LatestBlockNum(ctx)
		if err != nil {
			return nil, fmt.Errorf("fetching latest block num: %w", err)
		}

		f.logger.Info("got latest block", zap.Uint64("latest", f.latest), zap.Uint64("block_num", blockNum))

		if f.latest < blockNum {
			time.Sleep(f.latestBlockRetryInterval)
			continue
		}
		break
	}

	sinceLastFetch := time.Since(f.lastFetchAt)
	if sinceLastFetch < f.fetchInterval {
		time.Sleep(f.fetchInterval - sinceLastFetch)
	}

	rpcBlock, err := f.rpcClient.GetBlockByNumber(ctx, rpc.BlockNumber(blockNum), rpc.WithGetBlockFullTransaction())
	if err != nil {
		return nil, fmt.Errorf("fetching block %d: %w", blockNum, err)
	}

	receipts, err := FetchReceipts(ctx, rpcBlock, f.rpcClient)
	if err != nil {
		return nil, fmt.Errorf("fetching receipts for block %d %q: %w", rpcBlock.Number, rpcBlock.Hash.Pretty(), err)
	}

	f.logger.Debug("fetched receipts", zap.Int("count", len(receipts)))

	f.lastFetchAt = time.Now()

	if err != nil {
		return nil, fmt.Errorf("fetching logs for block %d %q: %w", rpcBlock.Number, rpcBlock.Hash.Pretty(), err)
	}

	ethBlock, _ := f.toEthBlock(rpcBlock, receipts)
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

func FetchReceipts(ctx context.Context, block *rpc.Block, client *rpc.Client) (out map[string]*rpc.TransactionReceipt, err error) {
	out = make(map[string]*rpc.TransactionReceipt)
	lock := sync.Mutex{}

	eg := llerrgroup.New(10)
	for _, tx := range block.Transactions.Transactions {
		if eg.Stop() {
			continue // short-circuit the loop if we got an error
		}
		hash := tx.Hash
		eg.Go(func() error {
			receipt, err := client.TransactionReceipt(ctx, hash)
			if err != nil {
				return fmt.Errorf("fetching receipt for tx %q: %w", hash.Pretty(), err)
			}
			lock.Lock()
			out[hash.Pretty()] = receipt
			lock.Unlock()
			return err
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return
}
