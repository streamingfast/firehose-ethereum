package substreams

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/streamingfast/dstore"
	"github.com/streamingfast/eth-go"
	"github.com/streamingfast/eth-go/rpc"
	pbethss "github.com/streamingfast/sf-ethereum/types/pb/sf/ethereum/substreams/v1"
	pbsubstreams "github.com/streamingfast/substreams/pb/sf/substreams/v1"
	"github.com/streamingfast/substreams/pipeline"
	"github.com/streamingfast/substreams/wasm"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

// interfaces, living in `streamingfast/substreams:extensions.go`

type extension struct {
	rpcClients   []*rpc.Client
	cacheManager *Cache
}

type RPCEngine struct {
	rpcCacheStore dstore.Store

	rpcClients            []*rpc.Client
	currentRpcClientIndex int
	cacheChunkSizeInBlock uint64

	perRequestCache     map[*pbsubstreams.Request]*Cache
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
		Timeout: 10 * time.Second,
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
		perRequestCache:       map[*pbsubstreams.Request]*Cache{},
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
			"eth_call": e.ethCall,
		},
	}
}

func (e *RPCEngine) PipelineOptions(ctx context.Context, request *pbsubstreams.Request) []pipeline.Option {
	pipelineCache := NewCache(ctx, e.rpcCacheStore, uint64(request.StartBlockNum), e.cacheChunkSizeInBlock)
	e.registerRequestCache(request, pipelineCache)

	preBlock := func(ctx context.Context, clock *pbsubstreams.Clock) error {
		pipelineCache.UpdateCache(ctx, clock.Number)
		return nil
	}

	postJob := func(ctx context.Context, clock *pbsubstreams.Clock) error {
		pipelineCache.Save(ctx)
		e.unregisterRequestCache(request)
		return nil
	}

	return []pipeline.Option{
		pipeline.WithPreBlockHook(preBlock),
		pipeline.WithPostJobHook(postJob),
	}
}

func (e *RPCEngine) registerRequestCache(req *pbsubstreams.Request, c *Cache) {
	e.perRequestCacheLock.Lock()
	defer e.perRequestCacheLock.Unlock()

	e.perRequestCache[req] = c
}

func (e *RPCEngine) unregisterRequestCache(req *pbsubstreams.Request) {
	e.perRequestCacheLock.Lock()
	defer e.perRequestCacheLock.Unlock()

	delete(e.perRequestCache, req)
}

func (e *RPCEngine) ethCall(ctx context.Context, request *pbsubstreams.Request, clock *pbsubstreams.Clock, in []byte) (out []byte, err error) {
	calls := &pbethss.RpcCalls{}
	if err := proto.Unmarshal(in, calls); err != nil {
		return nil, fmt.Errorf("unmarshal RpcCalls proto: %w", err)
	}

	e.perRequestCacheLock.RLock()
	cache := e.perRequestCache[request]
	e.perRequestCacheLock.RUnlock()
	if cache == nil {
		panic("no cache initialized for this request?!")
	}

	res := e.rpcCalls(ctx, cache, clock.Number, calls)
	cnt, err := proto.Marshal(res)
	if err != nil {
		return nil, fmt.Errorf("marshal RpcResponses proto: %w", err)
	}

	return cnt, nil
}

type RPCCall struct {
	ToAddr          string
	MethodSignature string // ex: "name() (string)"
}

func (c *RPCCall) ToString() string {
	return fmt.Sprintf("%s:%s", c.ToAddr, c.MethodSignature)
}

type RPCResponse struct {
	Decoded       []interface{}
	Raw           string
	DecodingError error
	CallError     error // always deterministic
}

func (e *RPCEngine) rpcCalls(ctx context.Context, cache *Cache, blockNum uint64, calls *pbethss.RpcCalls) (out *pbethss.RpcResponses) {
	callsBytes, _ := proto.Marshal(calls)
	cacheKey := fmt.Sprintf("%d:%x", blockNum, sha256.Sum256(callsBytes))
	if len(callsBytes) != 0 {
		val, found := cache.Get(cacheKey)
		if found {
			out = &pbethss.RpcResponses{}
			err := proto.Unmarshal(val, out)
			if err == nil {
				return out
			}
		}
	}

	var reqs []*rpc.RPCRequest
	for _, call := range calls.Calls {
		req := &rpc.RPCRequest{
			Params: []interface{}{
				map[string]interface{}{
					"to":   eth.Hex(call.ToAddr).Pretty(),
					"data": eth.Hex(call.MethodSignature).Pretty(),
					"gas":  50_000_000,
				},
				blockNum,
			},
			Method: "eth_call",
		}
		reqs = append(reqs, req)
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
			if errors.Is(err, context.Canceled) {
				zlog.Debug("stopping rpc calls here, context is canceled")
				return nil
			}

			e.rollRpcClient()
			zlog.Warn("retrying RPCCall on RPC error", zap.Error(err), zap.Uint64("at_block", blockNum), zap.Stringer("endpoint", client))
			continue
		}

		deterministicResp := true
		for _, resp := range out {
			if !resp.Deterministic() {
				zlog.Warn("retrying RPCCall on non-deterministic RPC call error", zap.Error(resp.Err), zap.Uint64("at_block", blockNum), zap.Stringer("endpoint", client))
				deterministicResp = false
				break
			}
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

		return resp
	}
}

// ToProtoCalls is a wrapper for previous format
func ToProtoCalls(in []*RPCCall) (out *pbethss.RpcCalls) {
	for _, call := range in {
		methodSig := eth.MustNewMethodDef(call.MethodSignature).MethodID()
		toAddr := eth.MustNewAddress(call.ToAddr)
		out.Calls = append(out.Calls, &pbethss.RpcCall{
			ToAddr:          toAddr,
			MethodSignature: methodSig,
		})
	}
	return
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
	return fmt.Sprintf("%x:%x", c.ToAddr, c.MethodSignature)
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
