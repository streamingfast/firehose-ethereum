package substreams

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/streamingfast/dstore"
	"github.com/streamingfast/eth-go/rpc"
	pbethss "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/substreams/v1"
	pbsubstreams "github.com/streamingfast/substreams/pb/sf/substreams/v1"
	"github.com/streamingfast/substreams/pipeline"
	"github.com/streamingfast/substreams/wasm"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

// interfaces, living in `streamingfast/substreams:extensions.go`

type extension struct {
	rpcClients   []*rpc.Client
	cacheManager *StoreBackedCache
}

type RPCEngine struct {
	rpcCacheStore dstore.Store

	rpcClients            []*rpc.Client
	currentRpcClientIndex int
	cacheChunkSizeInBlock uint64

	perRequestCache     map[string]Cache
	perRequestCacheLock sync.RWMutex
}

func NewRPCEngine(rpcCachePath string, rpcEndpoints []string, cacheChunkSizeInBlock uint64) (*RPCEngine, error) {
	rpcCacheStore, err := dstore.NewStore(rpcCachePath, "", "", false)
	if err != nil {
		return nil, fmt.Errorf("setting up rpc cache store: %w", err)
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: true, // don't reuse connections
		},
		Timeout: 60 * time.Second,
	}
	opts := []rpc.Option{
		rpc.WithHttpClient(httpClient),
	}

	var rpcClients []*rpc.Client
	for _, endpoint := range rpcEndpoints {
		rpcClients = append(rpcClients, rpc.NewClient(endpoint, opts...))
	}

	if len(rpcClients) == 1 {
		zlog.Warn("balancing of requests to multiple RPC client is disabled because you only configured 1 RPC client")
	}

	return &RPCEngine{
		perRequestCache:       map[string]Cache{},
		rpcCacheStore:         rpcCacheStore,
		rpcClients:            rpcClients,
		cacheChunkSizeInBlock: cacheChunkSizeInBlock,
	}, nil
}

func (e *RPCEngine) rpcClient() *rpc.Client {
	return e.rpcClients[e.currentRpcClientIndex]
}

func (e *RPCEngine) rollRpcClient() {
	if e.currentRpcClientIndex == len(e.rpcClients)-1 {
		e.currentRpcClientIndex = 0
		return
	}

	e.currentRpcClientIndex++
}

func (e *RPCEngine) WASMExtensions() map[string]map[string]wasm.WASMExtension {
	return map[string]map[string]wasm.WASMExtension{
		"rpc": {
			"eth_call": e.ETHCall,
		},
	}
}

func (e *RPCEngine) PipelineOptions(ctx context.Context, startBlockNum, stopBlockNum uint64, traceID string) []pipeline.Option {
	pipelineCache := NewStoreBackedCache(ctx, e.rpcCacheStore, startBlockNum, e.cacheChunkSizeInBlock)
	e.registerRequestCache(traceID, pipelineCache)

	preBlock := func(ctx context.Context, clock *pbsubstreams.Clock) error {
		pipelineCache.UpdateCache(ctx, clock.Number)
		return nil
	}

	postJob := func(ctx context.Context, clock *pbsubstreams.Clock) error {
		if clock.Number < stopBlockNum {
			return nil
		}
		pipelineCache.Save(ctx)
		e.unregisterRequestCache(traceID)
		return nil
	}

	return []pipeline.Option{
		pipeline.WithPreBlockHook(preBlock),
		pipeline.WithPostJobHook(postJob),
	}
}

func (e *RPCEngine) registerRequestCache(traceID string, c Cache) {
	e.perRequestCacheLock.Lock()
	defer e.perRequestCacheLock.Unlock()
	e.perRequestCache[traceID] = c
	zlog.Debug("register request cache", zap.String("trace_id", traceID))
}

func (e *RPCEngine) unregisterRequestCache(traceID string) {
	e.perRequestCacheLock.Lock()
	defer e.perRequestCacheLock.Unlock()
	if tracer.Enabled() {
		zlog.Debug("unregister request cache", zap.String("trace_id", traceID))
	}
	delete(e.perRequestCache, traceID)
}

func (e *RPCEngine) ETHCall(ctx context.Context, traceID string, clock *pbsubstreams.Clock, in []byte) (out []byte, err error) {
	// We set `alwaysRetry` parameter to `true` here so it means `deterministic` return value will always be `true` and we can safely ignore it
	out, _, err = e.ethCall(ctx, true, traceID, clock, in)
	return out, err
}

func (e *RPCEngine) ethCall(ctx context.Context, alwaysRetry bool, traceID string, clock *pbsubstreams.Clock, in []byte) (out []byte, deterministic bool, err error) {
	calls := &pbethss.RpcCalls{}
	if err := proto.Unmarshal(in, calls); err != nil {
		return nil, false, fmt.Errorf("unmarshal rpc calls proto: %w", err)
	}

	e.perRequestCacheLock.RLock()
	cache, found := e.perRequestCache[traceID]
	e.perRequestCacheLock.RUnlock()

	if !found {
		panic(fmt.Sprintf("cache not found for trace ID %s", traceID))
	}

	if cache == nil {
		panic("no cache initialized for this request")
	}

	if err := e.validateCalls(ctx, calls); err != nil {
		return nil, true, err
	}

	res, deterministic, err := e.rpcCalls(ctx, alwaysRetry, cache, clock.Id, calls)
	if err != nil {
		return nil, deterministic, err
	}

	cnt, err := proto.Marshal(res)
	if err != nil {
		return nil, false, fmt.Errorf("marshal rpc responses proto: %w", err)
	}

	return cnt, deterministic, nil
}

type RPCCall struct {
	ToAddr string
	Data   string // ex: "name() (string)"
}

func (c *RPCCall) ToString() string {
	return fmt.Sprintf("%s:%s", c.ToAddr, c.Data)
}

type RPCResponse struct {
	Decoded       []interface{}
	Raw           string
	DecodingError error
	CallError     error // always deterministic
}

func (e *RPCEngine) validateCalls(ctx context.Context, calls *pbethss.RpcCalls) (err error) {
	for i, call := range calls.Calls {
		if len(call.ToAddr) != 20 {
			err = multierr.Append(err, fmt.Errorf("invalid call #%d: 'ToAddr' should contain 20 bytes, got %d bytes", i, len(call.ToAddr)))
		}
	}

	return err
}

var evmExecutionExecutionTimeoutRegex = regexp.MustCompile(`execution aborted \(timeout\s*=\s*[^\)]+\)`)

// rpcsCalls performs the RPC calls with full retry unless `alwaysRetry` is `false` in which case output is
// returned right away. If `alwaysRetry` is sets to `true` than `deterministic` will always return `true`
// and `err` will always be nil.
func (e *RPCEngine) rpcCalls(ctx context.Context, alwaysRetry bool, cache Cache, blockHash string, calls *pbethss.RpcCalls) (out *pbethss.RpcResponses, deterministic bool, err error) {
	callsBytes, _ := proto.Marshal(calls)
	cacheKey := fmt.Sprintf("%s:%x", blockHash, sha256.Sum256(callsBytes))
	if len(callsBytes) != 0 {
		val, found := cache.Get(cacheKey)
		if found {
			out = &pbethss.RpcResponses{}
			err := proto.Unmarshal(val, out)
			if err == nil {
				return out, true, nil
			}
		}
	}

	reqs := make([]*rpc.RPCRequest, len(calls.Calls))
	for i, call := range calls.Calls {
		reqs[i] = rpc.NewRawETHCall(rpc.CallParams{
			To:       call.ToAddr,
			GasLimit: 50_000_000,
			Data:     call.Data,
		}, rpc.BlockHash(blockHash)).ToRequest()
	}

	var delay time.Duration
	var attemptNumber int
	for {
		time.Sleep(delay)

		attemptNumber += 1
		delay = minDuration(time.Duration(attemptNumber*500)*time.Millisecond, 10*time.Second)

		// Kept here because later we roll it, but we still want to log the one that generated the error
		client := e.rpcClient()

		out, err := client.DoRequests(ctx, reqs)
		if err != nil {
			if !alwaysRetry {
				return nil, false, err
			}

			if errors.Is(err, context.Canceled) {
				zlog.Debug("stopping rpc calls here, context is canceled")
				return nil, false, err
			}

			e.rollRpcClient()
			zlog.Warn("retrying RPCCall on RPC error", zap.Error(err), zap.String("at_block", blockHash), zap.Stringer("endpoint", client), zap.Reflect("request", reqs[0]))
			continue
		}

		deterministicResp := true
		for _, resp := range out {
			if !resp.Deterministic() {
				if resp.Err != nil {
					if rpcErr, ok := resp.Err.(*rpc.ErrResponse); ok {
						if evmExecutionExecutionTimeoutRegex.MatchString(rpcErr.Message) {
							deterministicResp = true
							break
						}
					}
				}

				zlog.Warn("retrying RPCCall on non-deterministic RPC call error", zap.Error(resp.Err), zap.String("at_block", blockHash), zap.Stringer("endpoint", client))
				deterministicResp = false
				break
			}
		}

		if !alwaysRetry {
			return toProtoResponses(out), deterministicResp, nil
		}

		if !deterministicResp {
			e.rollRpcClient()
			continue
		}

		resp := toProtoResponses(out)

		if deterministicResp {
			if encodedResp, err := proto.Marshal(resp); err == nil {
				cache.Set(cacheKey, encodedResp)
			}
		}

		return resp, deterministicResp, nil
	}
}

func toProtoResponses(in []*rpc.RPCResponse) (out *pbethss.RpcResponses) {
	out = &pbethss.RpcResponses{}
	for _, resp := range in {
		newResp := &pbethss.RpcResponse{}
		if resp.Err != nil {
			newResp.Failed = true
		} else {
			if !strings.HasPrefix(resp.Content, "0x") {
				newResp.Failed = true
			} else {
				bytes, err := hex.DecodeString(resp.Content[2:])
				if err != nil {
					newResp.Failed = true
				} else {
					newResp.Raw = bytes
				}
			}
		}
		out.Responses = append(out.Responses, newResp)
	}
	return
}

func callToString(c *pbethss.RpcCall) string {
	return fmt.Sprintf("%x:%x", c.ToAddr, c.Data)
}

func toRPCResponse(in []*rpc.RPCResponse) (out []*RPCResponse) {
	for _, rpcResp := range in {
		decoded, decodingError := rpcResp.Decode()
		out = append(out, &RPCResponse{
			Decoded:       decoded,
			DecodingError: decodingError,
			CallError:     rpcResp.Err,
			Raw:           rpcResp.Content,
		})
	}
	return
}

func minDuration(a, b time.Duration) time.Duration {
	if a <= b {
		return a
	}
	return b
}
