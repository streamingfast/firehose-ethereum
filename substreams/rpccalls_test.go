package substreams

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/streamingfast/eth-go"
	pbethss "github.com/streamingfast/firehose-ethereum/types/pb/proto/sf/ethereum/substreams/v1"
	pbsubstreams "github.com/streamingfast/substreams/pb/sf/substreams/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
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

	engine, err := NewRPCEngine([]string{server.URL}, 50_000_000)
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

	engine, err := NewRPCEngine([]string{server.URL}, 50_000_000)
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

	engine, err := NewRPCEngine([]string{server.URL}, 50_000_000)
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

			engine, err := NewRPCEngine([]string{server.URL}, 50_000_000)
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

	engine, err := NewRPCEngine([]string{server.URL}, 50_000_000)
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

	engine, err := NewRPCEngine([]string{server.URL}, 50_000_000)
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

	engine, err := NewRPCEngine([]string{server.URL}, 50_000_000)
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
