package substreams

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/streamingfast/eth-go/rpc"
	pbethss "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/substreams/v1"
	pbsubstreams "github.com/streamingfast/substreams/pb/sf/substreams/v1"
	"github.com/streamingfast/substreams/wasm"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
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
	// Accept either or both rpc_eth_call and rpc_eth_get_balance
	rpcInfoCall, okCall := in["rpc_eth_call"]
	rpcInfoBal, okBal := in["rpc_eth_get_balance"]
	if !okCall && !okBal {
		return map[string]map[string]wasm.WASMExtension{}, nil
	}

	var partsGlobal []string

	if okCall {
		partsGlobal = strings.Split(rpcInfoCall, ",")
	} else {
		partsGlobal = strings.Split(rpcInfoBal, ",")
	}

	var gasLimit uint64 = 50_000_000 //default gas limit

	if len(partsGlobal) > 1 {
		gasLimitString := partsGlobal[0]
		gasLimitRes, err := strconv.ParseUint(gasLimitString, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parsing gas limit: %w", err)
		}
		gasLimit = gasLimitRes
	}

	var callURLs []string
	var balURLs []string

	if okCall {
		parts := strings.Split(rpcInfoCall, ",")
		if len(parts) > 1 {
			callURLs = parts[1:]
		} else {
			callURLs = []string{rpcInfoCall}
		}
	}

	if okBal {
		parts := strings.Split(rpcInfoBal, ",")
		if len(parts) > 1 {
			balURLs = parts[1:]
		} else {
			balURLs = []string{rpcInfoBal}
		}
	}

	eng, err := NewRPCEngine(callURLs, balURLs, gasLimit)
	if err != nil {
		return nil, fmt.Errorf("creating new RPC engine: %w", err)
	}

	extMap := make(map[string]wasm.WASMExtension)
	if okCall {
		extMap["eth_call"] = eng.ETHCall
	}
	if okBal {
		extMap["eth_get_balance"] = eng.ETHGetBalance
	}

	result := map[string]map[string]wasm.WASMExtension{
		"rpc": extMap,
	}

	return result, nil
}

type RPCEngine struct {
	gasLimit uint64

	callClients            []*rpc.Client
	currentCallClientIndex int

	balanceClients            []*rpc.Client
	currentBalanceClientIndex int
	LastestFallbackDuration   *time.Duration
}

func NewRPCEngine(callEndpoints []string, balanceEndpoints []string, gasLimit uint64) (*RPCEngine, error) {
	zlog.Debug("creating new Substreams RPC engine",
		zap.Strings("call_endpoints", callEndpoints),
		zap.Strings("get_balance_endpoints", balanceEndpoints),
		zap.Uint64("gas_limit", gasLimit),
	)

	httpClient := &http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: true, // don't reuse connections
		},
	}
	opts := []rpc.Option{
		rpc.WithHttpClient(httpClient),
	}

	var callClients []*rpc.Client
	for _, endpoint := range callEndpoints {
		callClients = append(callClients, rpc.NewClient(endpoint, opts...))
	}

	var balanceClients []*rpc.Client
	for _, endpoint := range balanceEndpoints {
		balanceClients = append(balanceClients, rpc.NewClient(endpoint, opts...))
	}

	if len(callClients) == 1 {
		zlog.Debug("balancing of eth_call requests to multiple RPC clients is disabled because you only configured 1 RPC client")
	}
	if len(balanceClients) == 1 {
		zlog.Debug("eth_get_balance balancing disabled (only 1 endpoint)")
	}
	engine := &RPCEngine{
		gasLimit:       gasLimit,
		callClients:    callClients,
		balanceClients: balanceClients,
	}

	if value := os.Getenv("LATEST_FALL_BACK_DURATION"); value != "" {
		d, err := time.ParseDuration(value)
		if err != nil {
			return nil, fmt.Errorf("parsing LATEST_FALL_BACK_DURATION env var: %w", err)
		}
		engine.LastestFallbackDuration = &d
	}

	return engine, nil

}

func (e *RPCEngine) callClient() *rpc.Client {
	return e.callClients[e.currentCallClientIndex]
}

func (e *RPCEngine) rollCallClient() {
	if e.currentCallClientIndex == len(e.callClients)-1 {
		e.currentCallClientIndex = 0
	} else {
		e.currentCallClientIndex++
	}
}

func (e *RPCEngine) balanceClient() *rpc.Client {
	return e.balanceClients[e.currentBalanceClientIndex]
}

func (e *RPCEngine) rollBalanceClient() {
	if e.currentBalanceClientIndex == len(e.balanceClients)-1 {
		e.currentBalanceClientIndex = 0
	} else {
		e.currentBalanceClientIndex++
	}
}

func (e *RPCEngine) ETHCall(ctx context.Context, traceID string, clock *pbsubstreams.Clock, in []byte) (out []byte, err error) {
	// We set `retryCount` parameter to `-1` (infinite retry) here so it means `deterministic` return value will always be `true` and we can safely ignore it
	out, _, err = e.ethCall(ctx, -1, traceID, clock, in)
	return out, err
}

func (e *RPCEngine) ethCall(ctx context.Context, retryCount int, traceID string, clock *pbsubstreams.Clock, in []byte) (out []byte, deterministic bool, err error) {
	calls := &pbethss.RpcCalls{}
	if err := proto.Unmarshal(in, calls); err != nil {
		return nil, false, fmt.Errorf("unmarshal rpc calls proto: %w", err)
	}

	if len(calls.Calls) == 0 {
		// A empty byte slice is a valid output that will lead to 0 responses
		return make([]byte, 0), false, nil
	}

	if err := e.validateCalls(calls); err != nil {
		return nil, true, err
	}

	res, deterministic, err := e.rpcCalls(ctx, traceID, retryCount, clock.Id, clock.Timestamp, calls)
	if err != nil {
		return nil, deterministic, err
	}

	cnt, err := proto.Marshal(res)
	if err != nil {
		return nil, false, fmt.Errorf("marshal rpc responses proto: %w", err)
	}

	return cnt, deterministic, nil
}

func (e *RPCEngine) ETHGetBalance(ctx context.Context, traceID string, clock *pbsubstreams.Clock, in []byte) ([]byte, error) {
	out, _, err := e.ethGetBalance(ctx, -1, traceID, clock, in)
	return out, err
}

func (e *RPCEngine) ethGetBalance(
	ctx context.Context,
	retryCount int,
	traceID string,
	clock *pbsubstreams.Clock,
	in []byte,
) (outBytes []byte, deterministic bool, err error) {

	reqMsg := &pbethss.RpcGetBalanceRequests{}
	if err := proto.Unmarshal(in, reqMsg); err != nil {
		return nil, false, fmt.Errorf("unmarshal get_balance proto: %w", err)
	}

	if len(reqMsg.Requests) == 0 {
		return []byte{}, true, nil
	}

	rpcReqs := make([]*rpc.RPCRequest, len(reqMsg.Requests))
	for i, r := range reqMsg.Requests {
		addrHex := "0x" + hex.EncodeToString(r.Address)

		// Add 0x prefix to block hash
		blockParam := r.Block
		if blockParam != "" && !strings.HasPrefix(blockParam, "0x") {
			blockParam = "0x" + blockParam
		}

		rpcReqs[i] = &rpc.RPCRequest{
			Method: "eth_getBalance",
			Params: []interface{}{addrHex, blockParam},
		}
	}

	responses, det, err := e.rpcDoWithRetry(ctx, traceID, retryCount, clock.Id, rpcReqs)
	if err != nil {
		return nil, det, err
	}

	outMsg := &pbethss.RpcGetBalanceResponses{}
	for _, resp := range responses.Responses {
		outMsg.Responses = append(outMsg.Responses, &pbethss.RpcGetBalanceResponse{
			Balance: resp.Balance,
			Failed:  resp.Failed,
		})
	}
	outBytes, err = proto.Marshal(outMsg)
	return outBytes, det, err
}

// rpcDoWithRetry does exactly what rpcCalls does for eth_call, but for eth_getBalance. It returns a fully-populated *RpcGetBalanceResponses plus the deterministic flag.
func (e *RPCEngine) rpcDoWithRetry(
	ctx context.Context,
	traceID string,
	retryCount int,
	blockHash string,
	reqs []*rpc.RPCRequest,
) (*pbethss.RpcGetBalanceResponses, bool, error) {
	var delay time.Duration
	var attempt int

	for {
		if len(reqs) == 0 {
			return &pbethss.RpcGetBalanceResponses{}, true, nil
		}
		time.Sleep(delay)
		attempt++
		delay = minDuration(time.Duration(attempt*500)*time.Millisecond, 10*time.Second)

		client := e.balanceClient()
		rpcResps, err := client.DoRequests(ctx, reqs)
		if err != nil {

			if ctx.Err() != nil {
				zlog.Info("stopping rpc calls here, context is canceled", zap.String("trace_id", traceID))
				return nil, false, err
			}
			if retryCount == 0 || (retryCount > 0 && attempt > retryCount) {
				return nil, false, err
			}
			e.rollBalanceClient()
			zlog.Warn("retrying eth_getBalance on RPC error",
				zap.String("trace_id", traceID),
				zap.Error(err),
				zap.Stringer("endpoint", client),
			)
			continue
		}

		out := &pbethss.RpcGetBalanceResponses{}
		deterministic := true

		for _, r := range rpcResps {

			resp := &pbethss.RpcGetBalanceResponse{
				Failed: r.Err != nil,
			}
			if r.Err == nil {
				content := r.Content
				if strings.HasPrefix(content, "0x") {
					content = content[2:]
				}

				// Pad odd-length hex strings with leading zero
				if len(content)%2 == 1 {
					content = "0" + content
				}

				raw, hexErr := hex.DecodeString(content)
				if hexErr != nil {
					resp.Failed = true
				} else {
					resp.Balance = raw
				}
			} else {
				zlog.Error("RPC error in eth_getBalance", zap.String("trace_id", traceID), zap.Error(r.Err))
			}
			out.Responses = append(out.Responses, resp)

			if !r.Deterministic() {
				deterministic = false
				break
			}
		}

		if retryCount == 0 || deterministic {
			return out, deterministic, nil
		}

		e.rollBalanceClient()
	}
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

func (e *RPCEngine) validateCalls(calls *pbethss.RpcCalls) (err error) {
	for i, call := range calls.Calls {
		if len(call.ToAddr) != 20 {
			err = multierr.Append(err, fmt.Errorf("invalid call #%d: 'ToAddr' should contain 20 bytes, got %d bytes", i, len(call.ToAddr)))
		}
	}

	return err
}

var evmExecutionExecutionTimeoutRegex = regexp.MustCompile(`execution aborted \(timeout\s*=\s*[^\)]+\)`)

// rpcCalls performs the RPC calls retrying forever on error if `retryCount` is set to -1. If `retryCount`
// is sets to 0, no retry is attempted. If `retryCount` is > 0, it will retry `retryCount` times.
//
// If there is no retry or if partial retry, deterministic will be always `false`. Otherwise, it can only
// be `true` (since we retry either forever or until we hit a deterministic error).
//
// Note that the `retryCount` value should be set to something else than -1 only for testing purposes, production
// code paths should always set it to -1 (infinite retry).
func (e *RPCEngine) rpcCalls(ctx context.Context, traceID string, retryCount int, blockHash string, blockTimestamp *timestamppb.Timestamp, calls *pbethss.RpcCalls) (out *pbethss.RpcResponses, deterministic bool, err error) {
	reqs := make([]*rpc.RPCRequest, len(calls.Calls))

	blockRef := rpc.BlockHash(blockHash)
	if e.LastestFallbackDuration != nil {
		blockTime := blockTimestamp.AsTime()
		blockAge := time.Since(blockTime)
		if blockAge > *e.LastestFallbackDuration {
			blockRef = rpc.LatestBlock
		}
	}

	for i, call := range calls.Calls {
		reqs[i] = rpc.NewRawETHCall(rpc.CallParams{
			To:       call.ToAddr,
			GasLimit: e.gasLimit,
			Data:     call.Data,
		}, blockRef).ToRequest()
	}

	var lastError error
	var delay time.Duration
	var attemptNumber int
	for {
		time.Sleep(delay)

		attemptNumber += 1
		delay = minDuration(time.Duration(attemptNumber*500)*time.Millisecond, 10*time.Second)

		// Kept here because later we roll it, but we still want to log the one that generated the error
		client := e.callClient()

		lastRequestSince := time.Now()
		out, err := client.DoRequests(ctx, reqs)
		if err != nil {
			if ctx.Err() != nil {
				callDesc, _ := rpc.MarshalJSONRPC(calls)
				zlog.Info("stopping rpc calls here, context is canceled", zap.String("trace_id", traceID))
				var lastErrorStr string
				if lastError != nil {
					lastErrorStr = "last error: " + lastError.Error()
				} else {
					lastErrorStr = "no errors"
				}

				cancelCause := context.Cause(ctx)
				return nil, false, fmt.Errorf("timeout while doing eth_call, waiting for rpc provider for %s (%d attempt(s), %s). Call details: %s, cause: %w", time.Since(lastRequestSince), attemptNumber, lastErrorStr, string(callDesc), cancelCause)

			}

			// Never retry on retry attempted max count
			if retryCount == 0 || (retryCount > 0 && attemptNumber > retryCount) {
				return nil, false, err
			}

			e.rollCallClient()
			zlog.Warn("retrying RPCCall on RPC error", zap.String("trace_id", traceID), zap.Error(err), zap.String("at_block", blockHash), zap.Stringer("endpoint", client), zap.Reflect("request", reqs[0]))
			lastError = err
			continue
		}

		if e.LastestFallbackDuration != nil {
			// Check for invalid params error (-32602), if found, retry with latest block
			retryWithLatest := false
			for _, resp := range out {
				if resp.Err != nil {
					var rpcErr *rpc.ErrResponse
					if errors.As(resp.Err, &rpcErr) && rpcErr.Code == -32602 {
						retryWithLatest = true
						break
					}
				}
			}
			if retryWithLatest && blockRef != rpc.LatestBlock {
				blockRef = rpc.LatestBlock
				// Rebuild reqs with latest block
				for i, call := range calls.Calls {
					reqs[i] = rpc.NewRawETHCall(rpc.CallParams{
						To:       call.ToAddr,
						GasLimit: e.gasLimit,
						Data:     call.Data,
					}, blockRef).ToRequest()
				}
				zlog.Warn("retrying RPCCall with latest block due to invalid params error (-32602)", zap.String("trace_id", traceID), zap.String("original_block", blockHash))
				continue
			}

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

		if retryCount == 0 {
			return toProtoResponses(out), deterministicResp, nil
		}

		if !deterministicResp {
			e.rollCallClient()
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

func minDuration(a, b time.Duration) time.Duration {
	if a <= b {
		return a
	}
	return b
}
