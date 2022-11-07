package substreams

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/streamingfast/eth-go"
	pbethss "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/substreams/v1"
	pbsubstreams "github.com/streamingfast/substreams/pb/sf/substreams/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

var clockBlock1 = &pbsubstreams.Clock{Number: 1, Id: "0x10155bcb0fab82ccdc5edc8577f0f608ae059f93720172d11ca0fc01438b08a5"}

func TestRPCEngine_rpcCalls(t *testing.T) {
	localCache := t.TempDir()
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

	engine, err := NewRPCEngine(localCache, []string{server.URL}, 1)
	require.NoError(t, err)

	request := &pbsubstreams.Request{}

	engine.registerRequestCache(request, NoOpCache{})

	address := eth.MustNewAddress("0xea674fdde714fd979de3edf0f56aa9716b898ec8")
	data := eth.MustNewMethodDef("decimals()").MethodID()

	protoCalls, err := proto.Marshal(&pbethss.RpcCalls{Calls: []*pbethss.RpcCall{{ToAddr: address, Data: data}}})
	require.NoError(t, err)

	out, deterministic, err := engine.ethCall(context.Background(), false, request, clockBlock1, protoCalls)
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
	type want struct {
		deterministic bool
		response      *pbethss.RpcResponse
	}

	tests := []struct {
		err     string
		wantOut want
	}{
		{
			`{"code": -32000, "message": "execution aborted (timeout = 5s)"}`,
			want{deterministic: true, response: &pbethss.RpcResponse{Failed: true}},
		},
		{
			`{"code": -32000, "message": "execution aborted (timeout = 30s)"}`,
			want{deterministic: true, response: &pbethss.RpcResponse{Failed: true}},
		},
		{
			`{"code":-32000,"message":"out of gas"}`,
			want{deterministic: true, response: &pbethss.RpcResponse{Failed: true}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.err, func(t *testing.T) {
			localCache := t.TempDir()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Write([]byte(fmt.Sprintf(`{"jsonrpc":"2.0","id":"0x1","error":%s}`, tt.err)))
			}))
			defer server.Close()

			engine, err := NewRPCEngine(localCache, []string{server.URL}, 1)
			require.NoError(t, err)

			request := &pbsubstreams.Request{}

			engine.registerRequestCache(request, NoOpCache{})

			address := eth.MustNewAddress("0x0000000000000000000000000000000000000000")
			data := eth.MustNewMethodDef("any()").MethodID()

			protoCalls, err := proto.Marshal(&pbethss.RpcCalls{Calls: []*pbethss.RpcCall{{ToAddr: address, Data: data}}})
			require.NoError(t, err)

			out, deterministic, err := engine.ethCall(context.Background(), false, request, clockBlock1, protoCalls)
			require.NoError(t, err)
			require.Equal(t, tt.wantOut.deterministic, deterministic)

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
