package blockfetcher

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/streamingfast/eth-go/rpc"
	"github.com/streamingfast/firehose-core/blockpoller"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

var (
	_ blockpoller.HeadBlockNumberFetcher[*rpc.Client] = (*BlockFetcher)(nil)
	_ blockpoller.HeadBlockNumberFetcher[*rpc.Client] = (*OptimismBlockFetcher)(nil)
	_ blockpoller.HeadBlockNumberFetcher[*rpc.Client] = (*TracerBlockFetcher)(nil)
)

func TestFetchHeadBlockNumber(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "eth_blockNumber", req.Method)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": "0x32"})
	}))
	defer server.Close()

	client := rpc.NewClient(server.URL)
	fetchers := map[string]blockpoller.HeadBlockNumberFetcher[*rpc.Client]{
		"generic": NewGenericBlockFetcher(0, time.Second, 1, false, zap.NewNop()),
		"tracer":  NewTracerBlockFetcher(0, time.Second, 1, false, zap.NewNop()),
	}

	for name, fetcher := range fetchers {
		t.Run(name, func(t *testing.T) {
			head, err := fetcher.FetchHeadBlockNumber(context.Background(), client)
			require.NoError(t, err)
			assert.Equal(t, uint64(50), head)
		})
	}
}
