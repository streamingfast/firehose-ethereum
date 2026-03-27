package blockfetcher

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/abourget/llerrgroup"
	"github.com/streamingfast/bstream"
	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	"github.com/streamingfast/derr"
	"github.com/streamingfast/eth-go"
	"github.com/streamingfast/eth-go/rpc"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// receipts and logs are mutually exclusive
type ToEthBlock func(in *rpc.Block, receipts map[string]*rpc.TransactionReceipt, logs map[string][]eth.Log, logger *zap.Logger) (*pbeth.Block, map[string]bool)

type BlockFetcher struct {
	latest                     uint64
	latestBlockRetryInterval   time.Duration
	fetchInterval              time.Duration
	toEthBlock                 ToEthBlock
	lastFetchAt                time.Time
	parallelTrxWorkers         int
	allowEmptyReceiptsOnBlock0 bool
	skipReceipts               bool
	logger                     *zap.Logger
}

func NewBlockFetcher(intervalBetweenFetch, latestBlockRetryInterval time.Duration, parallelTrxWorkers int, toEthBlock ToEthBlock, logger *zap.Logger) *BlockFetcher {
	return &BlockFetcher{
		latestBlockRetryInterval: latestBlockRetryInterval,
		toEthBlock:               toEthBlock,
		fetchInterval:            intervalBetweenFetch,
		parallelTrxWorkers:       parallelTrxWorkers,
		logger:                   logger,
	}
}

// SkipReceipts sets whether to skip fetching receipts for transactions. When true, the Logs will be fetched directly instead, using a single RPC call.
// Some transaction fields will be missing by using this technique, like gasUsed, cumulativeGasUsed, status, log Index...
func (f *BlockFetcher) SkipReceipts(skip bool) {
	f.skipReceipts = skip
}

func (f *BlockFetcher) IsBlockAvailable(blockNum uint64) bool {
	return blockNum <= f.latest
}

func (f *BlockFetcher) FetchPBEth(ctx context.Context, rpcClient *rpc.Client, blockNum uint64) (block *pbeth.Block, err error) {

	f.logger.Debug("fetching block", zap.Uint64("block_num", blockNum))
	for f.latest < blockNum {
		f.latest, err = rpcClient.LatestBlockNum(ctx)
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

	rpcBlock, err := rpcClient.GetBlockByNumber(ctx, rpc.BlockNumber(blockNum), rpc.WithGetBlockFullTransaction())
	if err != nil {
		return nil, fmt.Errorf("fetching block %d: %w", blockNum, err)
	}

	blockHash := eth.Bytes(rpcBlock.Hash.Bytes())
	var receipts map[string]*rpc.TransactionReceipt
	var logs map[string][]eth.Log
	if f.skipReceipts {
		logs, err = FetchLogs(ctx, blockHash, rpcClient)
		if err != nil {
			return nil, fmt.Errorf("fetching logs for block %d %q: %w", rpcBlock.Number, rpcBlock.Hash.Pretty(), err)
		}

	} else {
		receipts, err = FetchReceipts(ctx, rpcBlock, rpcClient, f.parallelTrxWorkers, f.allowEmptyReceiptsOnBlock0)
		if err != nil {
			return nil, fmt.Errorf("fetching receipts for block %d %q: %w", rpcBlock.Number, rpcBlock.Hash.Pretty(), err)
		}
	}

	f.logger.Debug("fetched receipts", zap.Int("count", len(receipts)))

	f.lastFetchAt = time.Now()

	ethBlock, _ := f.toEthBlock(rpcBlock, receipts, logs, f.logger)
	return ethBlock, nil
}

func (f *BlockFetcher) Fetch(ctx context.Context, rpcClient *rpc.Client, blockNum uint64) (block *pbbstream.Block, err error) {
	ethBlock, err := f.FetchPBEth(ctx, rpcClient, blockNum)
	if err != nil {
		return nil, err
	}

	anyBlock, err := anypb.New(ethBlock)
	if err != nil {
		return nil, fmt.Errorf("create any block: %w", err)
	}

	return &pbbstream.Block{
		Number:    ethBlock.Number,
		Id:        ethBlock.GetFirehoseBlockID(),
		ParentId:  ethBlock.GetFirehoseBlockParentID(),
		Timestamp: timestamppb.New(ethBlock.GetFirehoseBlockTime()),
		LibNum:    ethBlockLIBNum(ethBlock),
		ParentNum: ethBlock.GetFirehoseBlockParentNumber(),
		Payload:   anyBlock,
	}, nil
}

func FetchLogs(ctx context.Context, blockHash eth.Bytes, client *rpc.Client) (out map[string][]eth.Log, err error) {
	r, err := client.Logs(ctx, rpc.LogsParams{
		BlockHash: blockHash,
	})
	if err != nil {
		return nil, err
	}
	out = make(map[string][]eth.Log)
	for _, log := range r {
		trxHash := log.TransactionHash.String()
		_, ok := out[trxHash]
		if !ok {
			out[trxHash] = []eth.Log{}
		}
		converted := log.ToLog()
		converted.Index = converted.BlockIndex // mimic the behavior with using receipts
		out[trxHash] = append(out[trxHash], converted)
	}
	return out, nil
}

// jsonRPCMethodNotFound is the JSON-RPC error code for "method not found", indicating
// the RPC endpoint does not support the called method.
const jsonRPCMethodNotFound = -32601

func FetchReceipts(ctx context.Context, block *rpc.Block, client *rpc.Client, parallelTrxWorkers int, allowEmptyReceiptsOnBlock0 bool) (out map[string]*rpc.TransactionReceipt, err error) {
	out, err = fetchBlockReceipts(ctx, block, client, allowEmptyReceiptsOnBlock0)
	if err == nil {
		return out, nil
	}

	// Only fall back to individual fetching when the RPC endpoint does not support
	// eth_getBlockReceipts (method not found). Any other error is returned directly.
	var rpcErr *rpc.ErrResponse
	if !errors.As(err, &rpcErr) || rpcErr.Code != jsonRPCMethodNotFound {
		return nil, err
	}

	// FIXME: Cache the fact that this client doesn't support eth_getBlockReceipts so
	// subsequent invocations skip the batch call and go straight to individual fetching.

	return fetchReceiptsIndividually(ctx, block, client, parallelTrxWorkers, allowEmptyReceiptsOnBlock0)
}

// fetchBlockReceipts fetches all receipts for a block in a single RPC call using eth_getBlockReceipts.
func fetchBlockReceipts(ctx context.Context, block *rpc.Block, client *rpc.Client, allowEmptyReceiptsOnBlock0 bool) (out map[string]*rpc.TransactionReceipt, err error) {
	receipts, err := client.BlockReceipts(ctx, rpc.BlockNumber(uint64(block.Number)))
	if err != nil {
		return nil, fmt.Errorf("fetching block receipts: %w", err)
	}

	out = make(map[string]*rpc.TransactionReceipt, len(receipts))
	for i, receipt := range receipts {
		if receipt == nil {
			if block.Number == 0 && allowEmptyReceiptsOnBlock0 {
				receipt = &rpc.TransactionReceipt{
					TransactionHash: block.Transactions.Transactions[i].Hash,
					BlockHash:       block.Hash,
					BlockNumber:     block.Number,
					From:            block.Transactions.Transactions[i].From,
					To:              block.Transactions.Transactions[i].To,
				}
			} else {
				return nil, fmt.Errorf("receipt is nil for transaction %s", block.Transactions.Transactions[i].Hash.Pretty())
			}
		}
		out[receipt.TransactionHash.Pretty()] = receipt
	}

	return out, nil
}

// fetchReceiptsIndividually fetches receipts one per transaction, used as fallback when eth_getBlockReceipts is not supported.
func fetchReceiptsIndividually(ctx context.Context, block *rpc.Block, client *rpc.Client, parallelTrxWorkers int, allowEmptyReceiptsOnBlock0 bool) (out map[string]*rpc.TransactionReceipt, err error) {
	out = make(map[string]*rpc.TransactionReceipt)
	lock := sync.Mutex{}

	eg := llerrgroup.New(parallelTrxWorkers)
	for _, tx := range block.Transactions.Transactions {
		if eg.Stop() {
			continue // short-circuit the loop if we got an error
		}
		hash := tx.Hash
		eg.Go(func() error {
			var receipt *rpc.TransactionReceipt
			err := derr.RetryContext(ctx, 5, func(ctx context.Context) error {
				r, err := client.TransactionReceipt(ctx, hash)
				if err != nil {
					return err
				}
				if r == nil {
					if block.Number == 0 && allowEmptyReceiptsOnBlock0 {
						r = &rpc.TransactionReceipt{
							TransactionHash: hash,
							BlockHash:       block.Hash,
							BlockNumber:     block.Number,
							From:            tx.From,
							To:              tx.To,
						}
					} else {
						return derr.NewFatalError(fmt.Errorf("receipt is nil"))
					}
				}

				receipt = r
				return nil
			})
			if err != nil {
				return fmt.Errorf("fetching receipt for tx %s: %w", hash.Pretty(), err)
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

func ethBlockLIBNum(b *pbeth.Block) uint64 {
	if b.Number <= bstream.GetProtocolFirstStreamableBlock+200 {
		return bstream.GetProtocolFirstStreamableBlock
	}

	return b.Number - 200
}
