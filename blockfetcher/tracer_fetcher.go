package blockfetcher

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	"github.com/streamingfast/eth-go/rpc"
	"github.com/streamingfast/firehose-ethereum/block"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"time"
)

type TracerBlockFetcher struct {
	fetcher *BlockFetcher
}

func (f *TracerBlockFetcher) IsBlockAvailable(requested uint64) bool {
	return f.fetcher.IsBlockAvailable(requested)
}

func (f *TracerBlockFetcher) Fetch(ctx context.Context, client *rpc.Client, blkNum uint64) (b *pbbstream.Block, skipped bool, err error) {
	respStr, err := client.DoRequest(ctx, "debug_traceFirehoseBlockByNumber", []interface{}{fmt.Sprintf("0x%x", blkNum), nil})
	if err != nil {
		return nil, false, fmt.Errorf("failed to trace block %d: %w", blkNum, err)
	}

	// The response is base64-encoded
	lineBytes, err := base64.StdEncoding.DecodeString(respStr)
	if err != nil {
		return nil, false, fmt.Errorf("failed to decode base64 response for block %d: %w", blkNum, err)
	}
	lineBytes = bytes.TrimRight(lineBytes, "\n")
	lastSpace := bytes.LastIndexByte(lineBytes, ' ')
	if lastSpace == -1 {
		return nil, false, fmt.Errorf("invalid firehose block line for block %d", blkNum)
	}
	blockBase64 := lineBytes[lastSpace+1:]

	blockData, err := base64.StdEncoding.DecodeString(string(blockBase64))
	if err != nil {
		return nil, false, fmt.Errorf("failed to decode block payload for block %d: %w", blkNum, err)
	}

	ethBlock := &pbeth.Block{}
	if err := proto.Unmarshal(blockData, ethBlock); err != nil {
		return nil, false, fmt.Errorf("failed to unmarshal traced block %d: %w", blkNum, err)
	}

	payload, err := anypb.New(ethBlock)
	if err != nil {
		return nil, false, fmt.Errorf("failed to wrap block %d: %w", blkNum, err)
	}

	bstreamBlock := &pbbstream.Block{
		Number:    ethBlock.Number,
		Id:        ethBlock.GetFirehoseBlockID(),
		ParentId:  ethBlock.GetFirehoseBlockParentID(),
		Timestamp: timestamppb.New(ethBlock.GetFirehoseBlockTime()),
		LibNum:    ethBlockLIBNum(ethBlock),
		ParentNum: ethBlock.GetFirehoseBlockParentNumber(),
		Payload:   payload,
	}

	return bstreamBlock, false, nil
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
