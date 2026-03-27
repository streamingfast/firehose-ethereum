package blockfetcher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/streamingfast/eth-go"
	"github.com/streamingfast/eth-go/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testBlockHash = eth.MustNewHash("0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890")
	testTxHash1   = eth.MustNewHash("0x1111111111111111111111111111111111111111111111111111111111111111")
	testTxHash2   = eth.MustNewHash("0x2222222222222222222222222222222222222222222222222222222222222222")
	testFrom      = eth.MustNewAddress("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	testTo        = eth.MustNewAddress("0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
)

func testBlock(number uint64) *rpc.Block {
	return &rpc.Block{
		Hash:   testBlockHash,
		Number: eth.Uint64(number),
		Transactions: &rpc.BlockTransactions{
			Transactions: []rpc.Transaction{
				{Hash: testTxHash1, From: testFrom, To: &testTo},
				{Hash: testTxHash2, From: testFrom, To: &testTo},
			},
		},
	}
}

func blockReceiptsResponse() string {
	return `{"jsonrpc":"2.0","id":"0x1","result":[` +
		`{"transactionHash":"0x1111111111111111111111111111111111111111111111111111111111111111",` +
		`"transactionIndex":"0x0","blockHash":"0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",` +
		`"blockNumber":"0xa","from":"0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",` +
		`"gasUsed":"0x5208","cumulativeGasUsed":"0x5208","status":"0x1","logs":[],"logsBloom":"0x00","type":"0x0"},` +
		`{"transactionHash":"0x2222222222222222222222222222222222222222222222222222222222222222",` +
		`"transactionIndex":"0x1","blockHash":"0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",` +
		`"blockNumber":"0xa","from":"0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",` +
		`"gasUsed":"0x5208","cumulativeGasUsed":"0xa410","status":"0x1","logs":[],"logsBloom":"0x00","type":"0x0"}]}`
}

func txReceiptResponse(txHash string, index int, cumulativeGas string) string {
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":"0x1","result":{`+
		`"transactionHash":"%s",`+
		`"transactionIndex":"0x%x","blockHash":"0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",`+
		`"blockNumber":"0xa","from":"0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",`+
		`"gasUsed":"0x5208","cumulativeGasUsed":"%s","status":"0x1","logs":[],"logsBloom":"0x00","type":"0x0"}}`,
		txHash, index, cumulativeGas)
}

func TestFetchReceipts_FallbackToIndividual(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buffer := bytes.NewBuffer(nil)
		_, err := buffer.ReadFrom(r.Body)
		require.NoError(t, err)

		var req map[string]interface{}
		err = json.Unmarshal(buffer.Bytes(), &req)
		require.NoError(t, err)

		method := req["method"].(string)
		callCount++

		switch method {
		case "eth_getBlockReceipts":
			// Simulate unsupported method
			w.Write([]byte(`{"jsonrpc":"2.0","id":"0x1","error":{"code":-32601,"message":"method not found"}}`))
		case "eth_getTransactionReceipt":
			params := req["params"].([]interface{})
			txHash := params[0].(string)
			switch txHash {
			case testTxHash1.Pretty():
				w.Write([]byte(txReceiptResponse(testTxHash1.Pretty(), 0, "0x5208")))
			case testTxHash2.Pretty():
				w.Write([]byte(txReceiptResponse(testTxHash2.Pretty(), 1, "0xa410")))
			default:
				t.Fatalf("unexpected tx hash: %s", txHash)
			}
		default:
			t.Fatalf("unexpected method: %s", method)
		}
	}))
	defer server.Close()

	client := rpc.NewClient(server.URL)
	block := testBlock(10)

	out, err := FetchReceipts(context.Background(), block, client, 2, false)
	require.NoError(t, err)
	require.Len(t, out, 2)

	assert.Contains(t, out, testTxHash1.Pretty())
	assert.Contains(t, out, testTxHash2.Pretty())
	// Should have called: 1 batch (failed) + 2 individual = 3 total calls
	assert.GreaterOrEqual(t, callCount, 3)
}

func TestFetchReceipts_NonMethodNotFoundErrorReturnedDirectly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a server error (not a method-not-found)
		w.Write([]byte(`{"jsonrpc":"2.0","id":"0x1","error":{"code":-32000,"message":"internal server error"}}`))
	}))
	defer server.Close()

	client := rpc.NewClient(server.URL)
	block := testBlock(10)

	_, err := FetchReceipts(context.Background(), block, client, 2, false)
	require.Error(t, err)
	// Should NOT fall back to individual fetching — error returned directly
	assert.Contains(t, err.Error(), "internal server error")
}

func TestFetchReceipts_BatchSuccess(t *testing.T) {
	batchCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buffer := bytes.NewBuffer(nil)
		_, err := buffer.ReadFrom(r.Body)
		require.NoError(t, err)

		var req map[string]interface{}
		err = json.Unmarshal(buffer.Bytes(), &req)
		require.NoError(t, err)

		method := req["method"].(string)
		if method == "eth_getBlockReceipts" {
			batchCalled = true
			w.Write([]byte(blockReceiptsResponse()))
		} else {
			t.Fatalf("should not call individual receipt method, got: %s", method)
		}
	}))
	defer server.Close()

	client := rpc.NewClient(server.URL)
	block := testBlock(10)

	out, err := FetchReceipts(context.Background(), block, client, 2, false)
	require.NoError(t, err)
	require.True(t, batchCalled)
	require.Len(t, out, 2)
}
