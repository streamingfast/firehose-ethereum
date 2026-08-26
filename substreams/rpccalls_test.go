package substreams

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/streamingfast/eth-go"
	pbethss "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/substreams/v1"
	pbsubstreams "github.com/streamingfast/substreams/pb/sf/substreams/v1"
	"github.com/streamingfast/substreams/reqctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var clockBlock1 = &pbsubstreams.Clock{Number: 1, Id: "0x10155bcb0fab82ccdc5edc8577f0f608ae059f93720172d11ca0fc01438b08a5"}

func assertProtoEqual(t *testing.T, expected proto.Message, actual proto.Message) {
	t.Helper()

	if !proto.Equal(expected, actual) {
		expectedAsJSON, err := protojson.Marshal(expected)
		require.NoError(t, err)

		actualAsJSON, err := protojson.Marshal(actual)
		require.NoError(t, err)

		expectedAsMap := map[string]interface{}{}
		err = json.Unmarshal(expectedAsJSON, &expectedAsMap)
		require.NoError(t, err)

		actualAsMap := map[string]interface{}{}
		err = json.Unmarshal(actualAsJSON, &actualAsMap)
		require.NoError(t, err)

		// We use equal is not equal above so we get a good diff, if the first condition failed, the second will also always
		// fail which is what we want here
		assert.Equal(t, expectedAsMap, actualAsMap)
	}
}

func TestRPCEngine_rpcCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buffer := bytes.NewBuffer(nil)
		_, err := buffer.ReadFrom(r.Body)
		require.NoError(t, err)

		assert.Equal(t,
			`[{"params":[{"to":"0xea674fdde714fd979de3edf0f56aa9716b898ec8","gas":"0x2faf080","data":"0x313ce567"},{"blockHash":"0x10155bcb0fab82ccdc5edc8577f0f608ae059f93720172d11ca0fc01438b08a5"}],"method":"eth_call","jsonrpc":"2.0","id":"0x1"}]`,
			buffer.String(),
		)

		w.Write([]byte(`{"jsonrpc":"2.0","id":"0x1","result":"0x0000000000000000000000000000000000000000000000000000000000000012"}`))
	}))

	engine, err := NewRPCEngine([]string{server.URL}, []string{server.URL}, 50_000_000)
	require.NoError(t, err)

	traceID := "someTraceID"

	address := eth.MustNewAddress("0xea674fdde714fd979de3edf0f56aa9716b898ec8")
	data := eth.MustNewMethodDef("decimals()").MethodID()

	protoCalls, err := proto.Marshal(&pbethss.RpcCalls{Calls: []*pbethss.RpcCall{{ToAddr: address, Data: data}}})
	require.NoError(t, err)

	out, deterministic, err := engine.ethCall(context.Background(), 0, traceID, clockBlock1, protoCalls)
	require.NoError(t, err)
	require.True(t, deterministic)

	responses := &pbethss.RpcResponses{}
	err = proto.Unmarshal(out, responses)
	require.NoError(t, err)

	assertProtoEqual(t, &pbethss.RpcResponses{
		Responses: []*pbethss.RpcResponse{
			{Raw: eth.MustNewBytes("0x0000000000000000000000000000000000000000000000000000000000000012"), Failed: false},
		},
	}, responses)
}

func TestRPCEngine_rpcCalls_noCallsInInput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Fail(t, "The server should never been called")
	}))

	engine, err := NewRPCEngine([]string{server.URL}, []string{server.URL}, 50_000_000)
	require.NoError(t, err)

	traceID := "someTraceID"

	protoCalls, err := proto.Marshal(&pbethss.RpcCalls{Calls: []*pbethss.RpcCall{}})
	require.NoError(t, err)

	out, deterministic, err := engine.ethCall(context.Background(), 1, traceID, clockBlock1, protoCalls)
	require.NoError(t, err)
	require.False(t, deterministic)

	responses := &pbethss.RpcResponses{}
	err = proto.Unmarshal(out, responses)
	require.NoError(t, err)

	assertProtoEqual(t, &pbethss.RpcResponses{
		Responses: []*pbethss.RpcResponse{},
	}, responses)
}

func TestRPCEngine_rpcCalls_retry(t *testing.T) {
	invokedCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buffer := bytes.NewBuffer(nil)
		_, err := buffer.ReadFrom(r.Body)
		require.NoError(t, err)

		invokedCount++
		if invokedCount == 1 {
			// Error than retry
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			assert.Equal(t,
				`[{"params":[{"to":"0xea674fdde714fd979de3edf0f56aa9716b898ec8","gas":"0x2faf080","data":"0x313ce567"},{"blockHash":"0x10155bcb0fab82ccdc5edc8577f0f608ae059f93720172d11ca0fc01438b08a5"}],"method":"eth_call","jsonrpc":"2.0","id":"0x1"}]`,
				buffer.String(),
			)

			w.Write([]byte(`{"jsonrpc":"2.0","id":"0x1","result":"0x0000000000000000000000000000000000000000000000000000000000000012"}`))
		}
	}))

	engine, err := NewRPCEngine([]string{server.URL}, []string{server.URL}, 50_000_000)
	require.NoError(t, err)

	traceID := "someTraceID"
	address := eth.MustNewAddress("0xea674fdde714fd979de3edf0f56aa9716b898ec8")
	data := eth.MustNewMethodDef("decimals()").MethodID()

	protoCalls, err := proto.Marshal(&pbethss.RpcCalls{Calls: []*pbethss.RpcCall{{ToAddr: address, Data: data}}})
	require.NoError(t, err)

	out, deterministic, err := engine.ethCall(context.Background(), 1, traceID, clockBlock1, protoCalls)
	require.NoError(t, err)
	require.True(t, deterministic)

	responses := &pbethss.RpcResponses{}
	err = proto.Unmarshal(out, responses)
	require.NoError(t, err)

	assertProtoEqual(t, &pbethss.RpcResponses{
		Responses: []*pbethss.RpcResponse{
			{Raw: eth.MustNewBytes("0x0000000000000000000000000000000000000000000000000000000000000012"), Failed: false},
		},
	}, responses)
}

func TestRPCEngine_rpcCalls_determisticErrorMessages(t *testing.T) {
	rpcCall := func(address string, data []byte) *pbethss.RpcCall {
		ethAddress := eth.MustNewAddressLoose(address)

		return &pbethss.RpcCall{ToAddr: ethAddress, Data: data}
	}

	dummyRPCCall := rpcCall("0x0000000000000000000000000000000000000000", eth.MustNewMethodDef("any()").MethodID())

	type want struct {
		deterministic bool
		response      *pbethss.RpcResponse
	}

	tests := []struct {
		name        string
		rpcCall     *pbethss.RpcCall
		response    string
		wantOut     want
		expectedErr require.ErrorAssertionFunc
	}{
		{
			"exection timeout 5s",
			dummyRPCCall,
			`{"code": -32000, "message": "execution aborted (timeout = 5s)"}`,
			want{deterministic: true, response: &pbethss.RpcResponse{Failed: true}},
			require.NoError,
		},
		{
			"exection timeout 30s",
			dummyRPCCall,
			`{"code": -32000, "message": "execution aborted (timeout = 30s)"}`,
			want{deterministic: true, response: &pbethss.RpcResponse{Failed: true}},
			require.NoError,
		},
		{
			"out of gas",
			dummyRPCCall,
			`{"code":-32000,"message":"out of gas"}`,
			want{deterministic: true, response: &pbethss.RpcResponse{Failed: true}},
			require.NoError,
		},
		{
			"invalid request error code",
			dummyRPCCall,
			`{"code":-32602,"message":"invalid request"}`,
			want{deterministic: true, response: &pbethss.RpcResponse{Failed: true}},
			require.NoError,
		},
		{
			"invalid request error code, invalid argument",
			dummyRPCCall,
			`{"code":-32602,"message":"invalid argument 0: json: cannot unmarshal hex string without 0x prefix into Go struct field CallArgs.to of type common.Address"}`,
			want{deterministic: true, response: &pbethss.RpcResponse{Failed: true}},
			require.NoError,
		},
		{
			"invalid json-rpc request error code",
			dummyRPCCall,
			`{"code":-32600,"message":"invalid request"}`,
			want{deterministic: true, response: &pbethss.RpcResponse{Failed: true}},
			require.NoError,
		},
		{
			"invalid RpcCall",
			rpcCall("aa", eth.MustNewMethodDef("any()").MethodID()),
			`{"code":-32602,"message":"invalid request"}`,
			want{deterministic: true, response: &pbethss.RpcResponse{Failed: false}},
			func(tt require.TestingT, err error, _ ...interface{}) {
				require.EqualError(tt, err, "invalid call #0: 'ToAddr' should contain 20 bytes, got 1 bytes")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Write([]byte(fmt.Sprintf(`{"jsonrpc":"2.0","id":"0x1","error":%s}`, tt.response)))
			}))
			defer server.Close()

			engine, err := NewRPCEngine([]string{server.URL}, []string{server.URL}, 50_000_000)
			require.NoError(t, err)

			traceID := "someTraceID"

			protoCalls, err := proto.Marshal(&pbethss.RpcCalls{Calls: []*pbethss.RpcCall{tt.rpcCall}})
			require.NoError(t, err)

			out, deterministic, err := engine.ethCall(context.Background(), 0, traceID, clockBlock1, protoCalls)
			tt.expectedErr(t, err)
			require.Equal(t, tt.wantOut.deterministic, deterministic)

			if err != nil {
				return
			}

			responses := &pbethss.RpcResponses{}
			err = proto.Unmarshal(out, responses)
			require.NoError(t, err)

			assertProtoEqual(t, &pbethss.RpcResponses{
				Responses: []*pbethss.RpcResponse{
					tt.wantOut.response,
				},
			}, responses)
		})
	}
}

func TestRPCEngine_rpcCalls_retryWithFallback(t *testing.T) {
	err := os.Setenv(EthCallFallbackDurationEnvVar, "1h")
	require.NoError(t, err)
	defer os.Unsetenv(EthCallFallbackDurationEnvVar)

	ctx := context.Background()
	ctx = reqctx.WithEthCallFallbackToLatestDuration(ctx, 1*time.Hour)

	invokedCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buffer := bytes.NewBuffer(nil)
		_, err := buffer.ReadFrom(r.Body)
		require.NoError(t, err)

		invokedCount++
		if invokedCount == 1 {
			// First call: return -32602 error
			assert.Contains(t, buffer.String(), `"blockHash":"0x10155bcb0fab82ccdc5edc8577f0f608ae059f93720172d11ca0fc01438b08a5"`)
			w.Write([]byte(`{"jsonrpc":"2.0","id":"0x1","error":{"code":-32602,"message":"Block requested not found. Request might be querying historical state that is not available. If possible, reformulate query to point to more recent blocks"}}`))
		} else {
			// Second call: should have "latest"
			assert.Contains(t, buffer.String(), `"latest"`)
			assert.NotContains(t, buffer.String(), `"blockHash"`)
			w.Write([]byte(`{"jsonrpc":"2.0","id":"0x1","result":"0x0000000000000000000000000000000000000000000000000000000000000012"}`))
		}
	}))
	defer server.Close()

	engine, err := NewRPCEngine([]string{server.URL}, []string{server.URL}, 50_000_000)
	require.NoError(t, err)

	traceID := "someTraceID"
	address := eth.MustNewAddress("0xea674fdde714fd979de3edf0f56aa9716b898ec8")
	data := eth.MustNewMethodDef("decimals()").MethodID()

	protoCalls, err := proto.Marshal(&pbethss.RpcCalls{Calls: []*pbethss.RpcCall{{ToAddr: address, Data: data}}})
	require.NoError(t, err)

	clock := &pbsubstreams.Clock{Number: 1, Id: "0x10155bcb0fab82ccdc5edc8577f0f608ae059f93720172d11ca0fc01438b08a5", Timestamp: timestamppb.Now()}
	out, deterministic, err := engine.ethCall(ctx, 1, traceID, clock, protoCalls)
	require.NoError(t, err)
	require.True(t, deterministic)

	responses := &pbethss.RpcResponses{}
	err = proto.Unmarshal(out, responses)
	require.NoError(t, err)

	assertProtoEqual(t, &pbethss.RpcResponses{
		Responses: []*pbethss.RpcResponse{
			{Raw: eth.MustNewBytes("0x0000000000000000000000000000000000000000000000000000000000000012"), Failed: false},
		},
	}, responses)
}
func TestRPCEngine_rpcCalls_retryWithImmediateLatestFallback(t *testing.T) {
	err := os.Setenv(EthCallFallbackDurationEnvVar, "1h")
	require.NoError(t, err)
	defer os.Unsetenv(EthCallFallbackDurationEnvVar)

	ctx := context.Background()
	ctx = reqctx.WithEthCallFallbackToLatestDuration(ctx, 1*time.Hour)

	//invokedCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buffer := bytes.NewBuffer(nil)
		_, err := buffer.ReadFrom(r.Body)
		require.NoError(t, err)
		assert.Contains(t, buffer.String(), `"latest"`)
		assert.NotContains(t, buffer.String(), `"blockHash"`)
		w.Write([]byte(`{"jsonrpc":"2.0","id":"0x1","result":"0x0000000000000000000000000000000000000000000000000000000000000012"}`))
	}))

	defer server.Close()

	engine, err := NewRPCEngine([]string{server.URL}, []string{server.URL}, 50_000_000)
	require.NoError(t, err)

	traceID := "someTraceID"
	address := eth.MustNewAddress("0xea674fdde714fd979de3edf0f56aa9716b898ec8")
	data := eth.MustNewMethodDef("decimals()").MethodID()

	protoCalls, err := proto.Marshal(&pbethss.RpcCalls{Calls: []*pbethss.RpcCall{{ToAddr: address, Data: data}}})
	require.NoError(t, err)

	blockTime := time.Now().Add(-2 * time.Hour)
	clock := &pbsubstreams.Clock{Number: 1, Id: "0x10155bcb0fab82ccdc5edc8577f0f608ae059f93720172d11ca0fc01438b08a5", Timestamp: timestamppb.New(blockTime)}
	out, deterministic, err := engine.ethCall(ctx, 1, traceID, clock, protoCalls)
	require.NoError(t, err)
	require.True(t, deterministic)

	responses := &pbethss.RpcResponses{}
	err = proto.Unmarshal(out, responses)
	require.NoError(t, err)

	assertProtoEqual(t, &pbethss.RpcResponses{
		Responses: []*pbethss.RpcResponse{
			{Raw: eth.MustNewBytes("0x0000000000000000000000000000000000000000000000000000000000000012"), Failed: false},
		},
	}, responses)
}

func TestRPCEngine_ethGetBalance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		_, err := buf.ReadFrom(r.Body)
		require.NoError(t, err)

		expected := `[{"params":["0xea674fdde714fd979de3edf0f56aa9716b898ec8","` + clockBlock1.Id + `"],"method":"eth_getBalance","jsonrpc":"2.0","id":"0x1"}]`
		require.Equal(t, expected, buf.String())

		w.Write([]byte(`{"jsonrpc":"2.0","id":"0x1","result":"0x0000000000000000000000000000000000000000000000000000000000000012"}`))
	}))
	defer server.Close()

	engine, err := NewRPCEngine([]string{server.URL}, []string{server.URL}, 50_000_000)
	require.NoError(t, err)

	reqProto := &pbethss.RpcGetBalanceRequests{
		Requests: []*pbethss.RpcGetBalanceRequest{{
			Address: eth.MustNewAddress("0xea674fdde714fd979de3edf0f56aa9716b898ec8"),
			Block:   clockBlock1.Id,
		}},
	}
	in, err := proto.Marshal(reqProto)
	require.NoError(t, err)

	out, det, err := engine.ethGetBalance(context.Background(), 1, "traceID", clockBlock1, in)
	require.NoError(t, err)
	require.True(t, det)

	got := &pbethss.RpcGetBalanceResponses{}
	require.NoError(t, proto.Unmarshal(out, got))

	assertProtoEqual(t,
		&pbethss.RpcGetBalanceResponses{Responses: []*pbethss.RpcGetBalanceResponse{{
			Balance: eth.MustNewBytes("0x0000000000000000000000000000000000000000000000000000000000000012"),
			Failed:  false,
		}}},
		got,
	)
}

func TestRPCEngine_ethGetBalance_noRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Fail(t, "server should not be called when there are no requests")
	}))
	defer server.Close()

	engine, err := NewRPCEngine([]string{server.URL}, []string{server.URL}, 50_000_000)
	require.NoError(t, err)

	// empty requests
	reqProto := &pbethss.RpcGetBalanceRequests{Requests: []*pbethss.RpcGetBalanceRequest{}}
	in, err := proto.Marshal(reqProto)
	require.NoError(t, err)

	out, det, err := engine.ethGetBalance(context.Background(), 1, "traceID", clockBlock1, in)
	require.NoError(t, err)
	require.True(t, det)
	require.Empty(t, out)
}

func TestRPCEngine_ethGetBalance_retry(t *testing.T) {
	count := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write([]byte(`{"jsonrpc":"2.0","id":"0x1","result":"0x01"}`))
	}))
	defer server.Close()

	engine, err := NewRPCEngine([]string{server.URL}, []string{server.URL}, 50_000_000)
	require.NoError(t, err)

	reqProto := &pbethss.RpcGetBalanceRequests{
		Requests: []*pbethss.RpcGetBalanceRequest{{
			Address: eth.MustNewAddress("0xea674fdde714fd979de3edf0f56aa9716b898ec8"),
			Block:   clockBlock1.Id,
		}},
	}
	in, err := proto.Marshal(reqProto)
	require.NoError(t, err)

	out, det, err := engine.ethGetBalance(context.Background(), 1, "traceID", clockBlock1, in)
	require.NoError(t, err)
	require.True(t, det)

	got := &pbethss.RpcGetBalanceResponses{}
	require.NoError(t, proto.Unmarshal(out, got))

	assertProtoEqual(t,
		&pbethss.RpcGetBalanceResponses{
			Responses: []*pbethss.RpcGetBalanceResponse{
				{Balance: eth.MustNewBytes("0x01"), Failed: false},
			},
		},
		got,
	)
}

func TestRPCEngine_ethGetBalance_retryWithFallback(t *testing.T) {
	err := os.Setenv(EthCallFallbackDurationEnvVar, "1h")
	require.NoError(t, err)
	defer os.Unsetenv(EthCallFallbackDurationEnvVar)

	ctx := context.Background()
	ctx = reqctx.WithEthCallFallbackToLatestDuration(ctx, 1*time.Hour)

	count := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buffer := bytes.NewBuffer(nil)
		_, err := buffer.ReadFrom(r.Body)
		require.NoError(t, err)

		count++
		if count == 1 {
			// First call uses the block hash, return -32602 error
			assert.Contains(t, buffer.String(), clockBlock1.Id)
			w.Write([]byte(`{"jsonrpc":"2.0","id":"0x1","error":{"code":-32602,"message":"Invalid params"}}`))
		} else {
			// Second call should use "latest"
			assert.Equal(t,
				`[{"params":["0xea674fdde714fd979de3edf0f56aa9716b898ec8","latest"],"method":"eth_getBalance","jsonrpc":"2.0","id":"0x1"}]`,
				buffer.String(),
			)
			w.Write([]byte(`{"jsonrpc":"2.0","id":"0x1","result":"0x01"}`))
		}
	}))
	defer server.Close()

	engine, err := NewRPCEngine([]string{server.URL}, []string{server.URL}, 50_000_000)
	require.NoError(t, err)

	reqProto := &pbethss.RpcGetBalanceRequests{
		Requests: []*pbethss.RpcGetBalanceRequest{{
			Address: eth.MustNewAddress("0xea674fdde714fd979de3edf0f56aa9716b898ec8"),
			Block:   clockBlock1.Id,
		}},
	}
	in, err := proto.Marshal(reqProto)
	require.NoError(t, err)

	// Recent block: initial request uses the block hash, the -32602 answer
	// triggers a single fallback retry with "latest"
	clock := &pbsubstreams.Clock{Number: 1, Id: clockBlock1.Id, Timestamp: timestamppb.Now()}
	out, det, err := engine.ethGetBalance(ctx, 1, "traceID", clock, in)
	require.NoError(t, err)
	require.True(t, det)
	require.Equal(t, 2, count)

	got := &pbethss.RpcGetBalanceResponses{}
	require.NoError(t, proto.Unmarshal(out, got))

	assertProtoEqual(t,
		&pbethss.RpcGetBalanceResponses{
			Responses: []*pbethss.RpcGetBalanceResponse{
				{Balance: eth.MustNewBytes("0x01"), Failed: false},
			},
		},
		got,
	)
}

func TestRPCEngine_ethGetBalance_blockTagPassthrough(t *testing.T) {
	for _, tag := range []string{"latest", "pending", "earliest", "safe", "finalized"} {
		t.Run(tag, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				buffer := bytes.NewBuffer(nil)
				_, err := buffer.ReadFrom(r.Body)
				require.NoError(t, err)

				// Block tags must be passed through unchanged, not mangled into "0x<tag>"
				assert.Equal(t,
					`[{"params":["0xea674fdde714fd979de3edf0f56aa9716b898ec8","`+tag+`"],"method":"eth_getBalance","jsonrpc":"2.0","id":"0x1"}]`,
					buffer.String(),
				)
				w.Write([]byte(`{"jsonrpc":"2.0","id":"0x1","result":"0x01"}`))
			}))
			defer server.Close()

			engine, err := NewRPCEngine([]string{server.URL}, []string{server.URL}, 50_000_000)
			require.NoError(t, err)

			reqProto := &pbethss.RpcGetBalanceRequests{
				Requests: []*pbethss.RpcGetBalanceRequest{{
					Address: eth.MustNewAddress("0xea674fdde714fd979de3edf0f56aa9716b898ec8"),
					Block:   tag,
				}},
			}
			in, err := proto.Marshal(reqProto)
			require.NoError(t, err)

			out, det, err := engine.ethGetBalance(context.Background(), 1, "traceID", clockBlock1, in)
			require.NoError(t, err)
			require.True(t, det)

			got := &pbethss.RpcGetBalanceResponses{}
			require.NoError(t, proto.Unmarshal(out, got))

			assertProtoEqual(t,
				&pbethss.RpcGetBalanceResponses{Responses: []*pbethss.RpcGetBalanceResponse{
					{Balance: eth.MustNewBytes("0x01"), Failed: false},
				}},
				got,
			)
		})
	}
}

func TestRPCEngine_ethGetBalance_persistent32602_noInfiniteLoop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = reqctx.WithEthCallFallbackToLatestDuration(ctx, 1*time.Hour)

	var mu sync.Mutex
	count := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		count++
		mu.Unlock()
		// Endpoint persistently answers -32602, even for "latest"
		w.Write([]byte(`{"jsonrpc":"2.0","id":"0x1","error":{"code":-32602,"message":"Invalid params"}}`))
	}))
	defer server.Close()

	engine, err := NewRPCEngine([]string{server.URL}, []string{server.URL}, 50_000_000)
	require.NoError(t, err)

	reqProto := &pbethss.RpcGetBalanceRequests{
		Requests: []*pbethss.RpcGetBalanceRequest{{
			Address: eth.MustNewAddress("0xea674fdde714fd979de3edf0f56aa9716b898ec8"),
			Block:   clockBlock1.Id,
		}},
	}
	in, err := proto.Marshal(reqProto)
	require.NoError(t, err)

	// Recent block so the initial request uses the block hash, then falls back to "latest"
	clock := &pbsubstreams.Clock{Number: 1, Id: clockBlock1.Id, Timestamp: timestamppb.Now()}

	var out []byte
	var det bool
	var callErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		out, det, callErr = engine.ethGetBalance(ctx, 1, "traceID", clock, in)
	}()

	select {
	case <-done:
	case <-time.After(20 * time.Second):
		cancel()
		<-done
		t.Fatal("ethGetBalance never returned: infinite -32602 fallback retry loop")
	}

	require.NoError(t, callErr)
	require.True(t, det)

	// One attempt with the block hash, one with "latest", then stop
	mu.Lock()
	require.Equal(t, 2, count)
	mu.Unlock()

	got := &pbethss.RpcGetBalanceResponses{}
	require.NoError(t, proto.Unmarshal(out, got))

	assertProtoEqual(t,
		&pbethss.RpcGetBalanceResponses{Responses: []*pbethss.RpcGetBalanceResponse{
			{Failed: true},
		}},
		got,
	)
}

func TestRPCEngine_ethGetBalance_useBlockNumber_hexEncoding(t *testing.T) {
	ctx := reqctx.WithEthCallUseBlockNumberDuration(context.Background(), 1*time.Hour)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buffer := bytes.NewBuffer(nil)
		_, err := buffer.ReadFrom(r.Body)
		require.NoError(t, err)

		// Block number must be hex encoded ("0x..."), like the eth_call path, not decimal
		assert.Equal(t,
			`[{"params":["0xea674fdde714fd979de3edf0f56aa9716b898ec8","0xbc614e"],"method":"eth_getBalance","jsonrpc":"2.0","id":"0x1"}]`,
			buffer.String(),
		)
		w.Write([]byte(`{"jsonrpc":"2.0","id":"0x1","result":"0x01"}`))
	}))
	defer server.Close()

	engine, err := NewRPCEngine([]string{server.URL}, []string{server.URL}, 50_000_000)
	require.NoError(t, err)

	reqProto := &pbethss.RpcGetBalanceRequests{
		Requests: []*pbethss.RpcGetBalanceRequest{{
			Address: eth.MustNewAddress("0xea674fdde714fd979de3edf0f56aa9716b898ec8"),
			Block:   "0x10155bcb0fab82ccdc5edc8577f0f608ae059f93720172d11ca0fc01438b08a5",
		}},
	}
	in, err := proto.Marshal(reqProto)
	require.NoError(t, err)

	// Block is 2 hours old, older than the 1 hour block-number duration
	clock := &pbsubstreams.Clock{
		Number:    12345678,
		Id:        "0x10155bcb0fab82ccdc5edc8577f0f608ae059f93720172d11ca0fc01438b08a5",
		Timestamp: timestamppb.New(time.Now().Add(-2 * time.Hour)),
	}

	out, det, err := engine.ethGetBalance(ctx, 1, "traceID", clock, in)
	require.NoError(t, err)
	require.True(t, det)

	got := &pbethss.RpcGetBalanceResponses{}
	require.NoError(t, proto.Unmarshal(out, got))

	assertProtoEqual(t,
		&pbethss.RpcGetBalanceResponses{Responses: []*pbethss.RpcGetBalanceResponse{
			{Balance: eth.MustNewBytes("0x01"), Failed: false},
		}},
		got,
	)
}

func TestRPCEngine_rpcCalls_useBlockNumber_olderThanDuration(t *testing.T) {
	ctx := context.Background()
	ctx = reqctx.WithEthCallUseBlockNumberDuration(ctx, 1*time.Hour)

	invokedCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buffer := bytes.NewBuffer(nil)
		_, err := buffer.ReadFrom(r.Body)
		require.NoError(t, err)

		invokedCount++
		// Should use block number instead of block hash for old blocks
		assert.Contains(t, buffer.String(), `"0x1"`)
		assert.NotContains(t, buffer.String(), `"blockHash"`)
		w.Write([]byte(`{"jsonrpc":"2.0","id":"0x1","result":"0x0000000000000000000000000000000000000000000000000000000000000012"}`))
	}))
	defer server.Close()

	engine, err := NewRPCEngine([]string{server.URL}, []string{server.URL}, 50_000_000)
	require.NoError(t, err)

	traceID := "someTraceID"
	address := eth.MustNewAddress("0xea674fdde714fd979de3edf0f56aa9716b898ec8")
	data := eth.MustNewMethodDef("decimals()").MethodID()

	protoCalls, err := proto.Marshal(&pbethss.RpcCalls{Calls: []*pbethss.RpcCall{{ToAddr: address, Data: data}}})
	require.NoError(t, err)

	// Create a clock with timestamp 2 hours ago (older than 1 hour duration)
	oldTimestamp := timestamppb.New(time.Now().Add(-2 * time.Hour))
	clock := &pbsubstreams.Clock{Number: 1, Id: "0x10155bcb0fab82ccdc5edc8577f0f608ae059f93720172d11ca0fc01438b08a5", Timestamp: oldTimestamp}

	out, deterministic, err := engine.ethCall(ctx, 1, traceID, clock, protoCalls)
	require.NoError(t, err)
	require.True(t, deterministic)
	require.Equal(t, 1, invokedCount)

	responses := &pbethss.RpcResponses{}
	err = proto.Unmarshal(out, responses)
	require.NoError(t, err)

	assertProtoEqual(t, &pbethss.RpcResponses{
		Responses: []*pbethss.RpcResponse{
			{Raw: eth.MustNewBytes("0x0000000000000000000000000000000000000000000000000000000000000012"), Failed: false},
		},
	}, responses)
}

func TestRPCEngine_rpcCalls_useBlockHash_newerThanDuration(t *testing.T) {
	ctx := context.Background()
	ctx = reqctx.WithEthCallUseBlockNumberDuration(ctx, 1*time.Hour)

	invokedCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buffer := bytes.NewBuffer(nil)
		_, err := buffer.ReadFrom(r.Body)
		require.NoError(t, err)

		invokedCount++
		// Should use block hash for recent blocks
		assert.Contains(t, buffer.String(), `"blockHash":"0x10155bcb0fab82ccdc5edc8577f0f608ae059f93720172d11ca0fc01438b08a5"`)
		assert.NotContains(t, buffer.String(), `,"0x1"]`)
		assert.NotContains(t, buffer.String(), `"latest"`)
		w.Write([]byte(`{"jsonrpc":"2.0","id":"0x1","result":"0x0000000000000000000000000000000000000000000000000000000000000012"}`))
	}))
	defer server.Close()

	engine, err := NewRPCEngine([]string{server.URL}, []string{server.URL}, 50_000_000)
	require.NoError(t, err)

	traceID := "someTraceID"
	address := eth.MustNewAddress("0xea674fdde714fd979de3edf0f56aa9716b898ec8")
	data := eth.MustNewMethodDef("decimals()").MethodID()

	protoCalls, err := proto.Marshal(&pbethss.RpcCalls{Calls: []*pbethss.RpcCall{{ToAddr: address, Data: data}}})
	require.NoError(t, err)

	// Create a clock with recent timestamp (newer than 1 hour duration)
	recentTimestamp := timestamppb.New(time.Now().Add(-30 * time.Minute))
	clock := &pbsubstreams.Clock{Number: 1, Id: "0x10155bcb0fab82ccdc5edc8577f0f608ae059f93720172d11ca0fc01438b08a5", Timestamp: recentTimestamp}

	out, deterministic, err := engine.ethCall(ctx, 1, traceID, clock, protoCalls)
	require.NoError(t, err)
	require.True(t, deterministic)
	require.Equal(t, 1, invokedCount)

	responses := &pbethss.RpcResponses{}
	err = proto.Unmarshal(out, responses)
	require.NoError(t, err)

	assertProtoEqual(t, &pbethss.RpcResponses{
		Responses: []*pbethss.RpcResponse{
			{Raw: eth.MustNewBytes("0x0000000000000000000000000000000000000000000000000000000000000012"), Failed: false},
		},
	}, responses)
}

func TestRPCEngine_rpcCalls_useBlockHash_zeroDuration(t *testing.T) {
	ctx := context.Background()
	ctx = reqctx.WithEthCallUseBlockNumberDuration(ctx, 0)

	invokedCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buffer := bytes.NewBuffer(nil)
		_, err := buffer.ReadFrom(r.Body)
		require.NoError(t, err)

		invokedCount++
		// Should always use block hash when duration is 0, regardless of block age
		assert.Contains(t, buffer.String(), `"blockHash":"0x10155bcb0fab82ccdc5edc8577f0f608ae059f93720172d11ca0fc01438b08a5"`)
		assert.NotContains(t, buffer.String(), `,"0x1"]`)
		assert.NotContains(t, buffer.String(), `"latest"`)
		w.Write([]byte(`{"jsonrpc":"2.0","id":"0x1","result":"0x0000000000000000000000000000000000000000000000000000000000000012"}`))
	}))
	defer server.Close()

	engine, err := NewRPCEngine([]string{server.URL}, []string{server.URL}, 50_000_000)
	require.NoError(t, err)

	traceID := "someTraceID"
	address := eth.MustNewAddress("0xea674fdde714fd979de3edf0f56aa9716b898ec8")
	data := eth.MustNewMethodDef("decimals()").MethodID()

	protoCalls, err := proto.Marshal(&pbethss.RpcCalls{Calls: []*pbethss.RpcCall{{ToAddr: address, Data: data}}})
	require.NoError(t, err)

	// Even with very old timestamp, should still use block hash when duration is 0
	oldTimestamp := timestamppb.New(time.Now().Add(-24 * time.Hour))
	clock := &pbsubstreams.Clock{Number: 1, Id: "0x10155bcb0fab82ccdc5edc8577f0f608ae059f93720172d11ca0fc01438b08a5", Timestamp: oldTimestamp}

	out, deterministic, err := engine.ethCall(ctx, 1, traceID, clock, protoCalls)
	require.NoError(t, err)
	require.True(t, deterministic)
	require.Equal(t, 1, invokedCount)

	responses := &pbethss.RpcResponses{}
	err = proto.Unmarshal(out, responses)
	require.NoError(t, err)

	assertProtoEqual(t, &pbethss.RpcResponses{
		Responses: []*pbethss.RpcResponse{
			{Raw: eth.MustNewBytes("0x0000000000000000000000000000000000000000000000000000000000000012"), Failed: false},
		},
	}, responses)
}

func TestRPCEngine_rpcCalls_blockNumberWithoutFallback(t *testing.T) {
	ctx := context.Background()
	ctx = reqctx.WithEthCallUseBlockNumberDuration(ctx, 30*time.Minute)
	// No fallback duration set

	invokedCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buffer := bytes.NewBuffer(nil)
		_, err := buffer.ReadFrom(r.Body)
		require.NoError(t, err)

		invokedCount++
		// Should use block number since block is older than 30min and no fallback is set
		assert.Contains(t, buffer.String(), `"0x1"`)
		assert.NotContains(t, buffer.String(), `"blockHash"`)
		assert.NotContains(t, buffer.String(), `"latest"`)
		w.Write([]byte(`{"jsonrpc":"2.0","id":"0x1","result":"0x0000000000000000000000000000000000000000000000000000000000000012"}`))
	}))
	defer server.Close()

	engine, err := NewRPCEngine([]string{server.URL}, []string{server.URL}, 50_000_000)
	require.NoError(t, err)

	traceID := "someTraceID"
	address := eth.MustNewAddress("0xea674fdde714fd979de3edf0f56aa9716b898ec8")
	data := eth.MustNewMethodDef("decimals()").MethodID()

	protoCalls, err := proto.Marshal(&pbethss.RpcCalls{Calls: []*pbethss.RpcCall{{ToAddr: address, Data: data}}})
	require.NoError(t, err)

	// Block is 60 minutes old - older than blockNumber duration (30min)
	timestamp := timestamppb.New(time.Now().Add(-60 * time.Minute))
	clock := &pbsubstreams.Clock{Number: 1, Id: "0x10155bcb0fab82ccdc5edc8577f0f608ae059f93720172d11ca0fc01438b08a5", Timestamp: timestamp}

	out, deterministic, err := engine.ethCall(ctx, 1, traceID, clock, protoCalls)
	require.NoError(t, err)
	require.True(t, deterministic)
	require.Equal(t, 1, invokedCount)

	responses := &pbethss.RpcResponses{}
	err = proto.Unmarshal(out, responses)
	require.NoError(t, err)

	assertProtoEqual(t, &pbethss.RpcResponses{
		Responses: []*pbethss.RpcResponse{
			{Raw: eth.MustNewBytes("0x0000000000000000000000000000000000000000000000000000000000000012"), Failed: false},
		},
	}, responses)
}

func TestRPCEngine_rpcCalls_blockNumberWithFallbackRetry(t *testing.T) {
	ctx := context.Background()
	ctx = reqctx.WithEthCallUseBlockNumberDuration(ctx, 30*time.Minute)
	ctx = reqctx.WithEthCallFallbackToLatestDuration(ctx, 2*time.Hour)

	invokedCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buffer := bytes.NewBuffer(nil)
		_, err := buffer.ReadFrom(r.Body)
		require.NoError(t, err)

		invokedCount++
		if invokedCount == 1 {
			// First call: should use block number (block is older than 30min but newer than 2h)
			assert.Contains(t, buffer.String(), `"0x1"`)
			assert.NotContains(t, buffer.String(), `"blockHash"`)
			assert.NotContains(t, buffer.String(), `"latest"`)
			// Return -32602 error to trigger fallback
			w.Write([]byte(`{"jsonrpc":"2.0","id":"0x1","error":{"code":-32602,"message":"Block requested not found"}}`))
		} else {
			// Second call: should fallback to latest
			assert.Contains(t, buffer.String(), `"latest"`)
			assert.NotContains(t, buffer.String(), `"blockHash"`)
			w.Write([]byte(`{"jsonrpc":"2.0","id":"0x1","result":"0x0000000000000000000000000000000000000000000000000000000000000012"}`))
		}
	}))
	defer server.Close()

	engine, err := NewRPCEngine([]string{server.URL}, []string{server.URL}, 50_000_000)
	require.NoError(t, err)

	traceID := "someTraceID"
	address := eth.MustNewAddress("0xea674fdde714fd979de3edf0f56aa9716b898ec8")
	data := eth.MustNewMethodDef("decimals()").MethodID()

	protoCalls, err := proto.Marshal(&pbethss.RpcCalls{Calls: []*pbethss.RpcCall{{ToAddr: address, Data: data}}})
	require.NoError(t, err)

	// Block is 60 minutes old - older than blockNumber duration (30min) but newer than fallback duration (2h)
	timestamp := timestamppb.New(time.Now().Add(-60 * time.Minute))
	clock := &pbsubstreams.Clock{Number: 1, Id: "0x10155bcb0fab82ccdc5edc8577f0f608ae059f93720172d11ca0fc01438b08a5", Timestamp: timestamp}

	out, deterministic, err := engine.ethCall(ctx, 1, traceID, clock, protoCalls)
	require.NoError(t, err)
	require.True(t, deterministic)
	require.Equal(t, 2, invokedCount)

	responses := &pbethss.RpcResponses{}
	err = proto.Unmarshal(out, responses)
	require.NoError(t, err)

	assertProtoEqual(t, &pbethss.RpcResponses{
		Responses: []*pbethss.RpcResponse{
			{Raw: eth.MustNewBytes("0x0000000000000000000000000000000000000000000000000000000000000012"), Failed: false},
		},
	}, responses)
}

func TestRPCEngine_rpcCalls_fallbackOverridesBlockNumber(t *testing.T) {
	ctx := context.Background()
	ctx = reqctx.WithEthCallUseBlockNumberDuration(ctx, 2*time.Hour)
	ctx = reqctx.WithEthCallFallbackToLatestDuration(ctx, 1*time.Hour)

	invokedCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buffer := bytes.NewBuffer(nil)
		_, err := buffer.ReadFrom(r.Body)
		require.NoError(t, err)

		invokedCount++
		// Should use latest due to fallback duration being shorter and taking precedence
		assert.Contains(t, buffer.String(), `"latest"`)
		assert.NotContains(t, buffer.String(), `"blockHash"`)
		w.Write([]byte(`{"jsonrpc":"2.0","id":"0x1","result":"0x0000000000000000000000000000000000000000000000000000000000000012"}`))
	}))
	defer server.Close()

	engine, err := NewRPCEngine([]string{server.URL}, []string{server.URL}, 50_000_000)
	require.NoError(t, err)

	traceID := "someTraceID"
	address := eth.MustNewAddress("0xea674fdde714fd979de3edf0f56aa9716b898ec8")
	data := eth.MustNewMethodDef("decimals()").MethodID()

	protoCalls, err := proto.Marshal(&pbethss.RpcCalls{Calls: []*pbethss.RpcCall{{ToAddr: address, Data: data}}})
	require.NoError(t, err)

	// Block is 90 minutes old - older than fallback duration (1h) but newer than blockNumber duration (2h)
	// Fallback should take precedence and use "latest"
	timestamp := timestamppb.New(time.Now().Add(-90 * time.Minute))
	clock := &pbsubstreams.Clock{Number: 1, Id: "0x10155bcb0fab82ccdc5edc8577f0f608ae059f93720172d11ca0fc01438b08a5", Timestamp: timestamp}

	out, deterministic, err := engine.ethCall(ctx, 1, traceID, clock, protoCalls)
	require.NoError(t, err)
	require.True(t, deterministic)
	require.Equal(t, 1, invokedCount)

	responses := &pbethss.RpcResponses{}
	err = proto.Unmarshal(out, responses)
	require.NoError(t, err)

	assertProtoEqual(t, &pbethss.RpcResponses{
		Responses: []*pbethss.RpcResponse{
			{Raw: eth.MustNewBytes("0x0000000000000000000000000000000000000000000000000000000000000012"), Failed: false},
		},
	}, responses)
}

// countingConnServer is an HTTP test server counting how many distinct TCP connections it
// accepted, which is what tells connection reuse apart from connection churn.
func countingConnServer(t *testing.T) (*httptest.Server, func() int) {
	t.Helper()

	var mutex sync.Mutex
	connections := map[net.Conn]bool{}

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"jsonrpc":"2.0","id":"0x1","result":"0x0000000000000000000000000000000000000000000000000000000000000012"}`))
	}))

	server.Config.ConnState = func(conn net.Conn, state http.ConnState) {
		if state != http.StateNew {
			return
		}

		mutex.Lock()
		defer mutex.Unlock()
		connections[conn] = true
	}

	server.Start()
	t.Cleanup(server.Close)

	return server, func() int {
		mutex.Lock()
		defer mutex.Unlock()
		return len(connections)
	}
}

func callEngineTimes(t *testing.T, engine *RPCEngine, times int) {
	t.Helper()

	address := eth.MustNewAddress("0xea674fdde714fd979de3edf0f56aa9716b898ec8")
	data := eth.MustNewMethodDef("decimals()").MethodID()

	protoCalls, err := proto.Marshal(&pbethss.RpcCalls{Calls: []*pbethss.RpcCall{{ToAddr: address, Data: data}}})
	require.NoError(t, err)

	for range times {
		_, _, err := engine.ethCall(context.Background(), 0, "someTraceID", clockBlock1, protoCalls)
		require.NoError(t, err)
	}
}

func TestRPCEngineReusesConnections(t *testing.T) {
	server, connectionCount := countingConnServer(t)

	engine, err := NewRPCEngine([]string{server.URL}, []string{server.URL}, 50_000_000)
	require.NoError(t, err)

	callEngineTimes(t, engine, 5)

	assert.Equal(t, 1, connectionCount(), "the 5 batches should all have gone through a single connection")
}

func TestRPCEngineDisableKeepAlives(t *testing.T) {
	server, connectionCount := countingConnServer(t)

	httpClient := NewHTTPClient(ConnectionOptions{DisableKeepAlives: true})
	engine, err := NewRPCEngine([]string{server.URL}, []string{server.URL}, 50_000_000, WithHTTPClient(httpClient))
	require.NoError(t, err)

	callEngineTimes(t, engine, 5)

	assert.Equal(t, 5, connectionCount(), "opting out of keep-alives must go back to one connection per batch")
}

// Substreams builds a new engine for every tier2 job, so the pool only ever gets reused if it
// lives on the extensioner rather than on the engine.
func TestRPCExtensionerSharesConnectionsAcrossEngines(t *testing.T) {
	server, connectionCount := countingConnServer(t)

	extensioner := NewRPCExtensioner(map[string]string{
		"rpc_eth_call": fmt.Sprintf("50000000,%s", server.URL),
	}, DefaultConnectionOptions())

	for range 3 {
		_, err := extensioner.WASMExtensions(extensioner.Params())
		require.NoError(t, err)

		engine, err := NewRPCEngine([]string{server.URL}, nil, 50_000_000, WithHTTPClient(extensioner.httpClient))
		require.NoError(t, err)

		callEngineTimes(t, engine, 2)
	}

	assert.Equal(t, 1, connectionCount(), "engines built from the same extensioner must share the connection pool")
}
