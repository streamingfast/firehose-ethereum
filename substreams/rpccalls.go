package substreams

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/streamingfast/eth-go/rpc"
	pbethss "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/substreams/v1"
	pbsubstreams "github.com/streamingfast/substreams/pb/sf/substreams/v1"
	"github.com/streamingfast/substreams/wasm"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

// interfaces, living in `streamingfast/substreams:extensions.go`

type RPCExtensioner struct {
	params map[string]string
}

func NewRPCExtensioner(params map[string]string) *RPCExtensioner {
	return &RPCExtensioner{params: params}
}

func (e *RPCExtensioner) Params() map[string]string {
	return e.params
}

func (e *RPCExtensioner) WASMExtensions(in map[string]string) (map[string]map[string]wasm.WASMExtension, error) {
	// set default values from flags ????
	switch len(in) {
	case 0:
		return nil, nil
	case 1:
		break
	default:
		return nil, fmt.Errorf("unsupported wasm extensions: %v (only 'rpc_eth_call' is implemented)", in)
	}

	rpcURL, found := in["rpc_eth_call"]
	if !found {
		return nil, fmt.Errorf("unsupported wasm extensions: %v (only 'rpc_eth_call' is implemented)", in)
	}

	// TODO: split rpcURL into multiplie
	// `https://patate.com/mykey?gas_limit=12,http://carotte.com/mykey2?gas_limit=15` -> flag on substreams-tier1
	// --rpc-endpoints= ^^
	// this goes into sf.substreams.intern.v2/ProcessRangeRequest under the extension map, key = `rpc_eth_call`
	eng, err := NewRPCEngine([]string{rpcURL}, 50_000_000) //todo: get gas limit from url
	if err != nil {
		return nil, fmt.Errorf("creating new RPC engine: %w", err)
	}
	return map[string]map[string]wasm.WASMExtension{
		"rpc": {
			"eth_call": eng.ETHCall,
		},
	}, nil
}

type RPCEngine struct {
	gasLimit uint64

	rpcClients            []*rpc.Client
	currentRpcClientIndex int

	endpoints []string
}

func NewRPCEngine(rpcEndpoints []string, gasLimit uint64) (*RPCEngine, error) {
	zlog.Debug("creating new Substreams RPC engine",
		zap.Strings("rpc_endpoints", rpcEndpoints),
		zap.Uint64("gas_limit", gasLimit),
	)

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
		zlog.Debug("balancing of requests to multiple RPC client is disabled because you only configured 1 RPC client")
	}

	return &RPCEngine{
		rpcClients: rpcClients,
		gasLimit:   gasLimit,
		endpoints:  rpcEndpoints,
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

	if err := e.validateCalls(ctx, calls); err != nil {
		return nil, true, err
	}

	res, deterministic, err := e.rpcCalls(ctx, traceID, alwaysRetry, clock.Id, calls)
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
func (e *RPCEngine) rpcCalls(ctx context.Context, traceID string, alwaysRetry bool, blockHash string, calls *pbethss.RpcCalls) (out *pbethss.RpcResponses, deterministic bool, err error) {
	reqs := make([]*rpc.RPCRequest, len(calls.Calls))
	for i, call := range calls.Calls {
		reqs[i] = rpc.NewRawETHCall(rpc.CallParams{
			To:       call.ToAddr,
			GasLimit: e.gasLimit,
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
				zlog.Debug("stopping rpc calls here, context is canceled", zap.String("trace_id", traceID))
				return nil, false, err
			}

			e.rollRpcClient()
			zlog.Warn("retrying RPCCall on RPC error", zap.String("trace_id", traceID), zap.Error(err), zap.String("at_block", blockHash), zap.Stringer("endpoint", client), zap.Reflect("request", reqs[0]))
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

				zlog.Warn("retrying RPCCall on non-deterministic RPC call error", zap.String("trace_id", traceID), zap.Error(resp.Err), zap.String("at_block", blockHash), zap.Stringer("endpoint", client))
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
