package block

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/streamingfast/eth-go"
	"github.com/streamingfast/eth-go/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertTrx(t *testing.T) {
	tests := []struct {
		beginOrdinal uint64
		logs         []*rpc.LogEntry
	}{
		{
			beginOrdinal: 0,
			logs:         nil,
		},
		{
			beginOrdinal: 0,
			logs: []*rpc.LogEntry{
				{},
				{},
				{},
			},
		},
		{
			beginOrdinal: 10,
			logs: []*rpc.LogEntry{
				{},
				{},
				{},
			},
		},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			in := &rpc.Transaction{}
			ordinal := &counter{
				val: test.beginOrdinal,
			}

			receipt := &rpc.TransactionReceipt{
				Logs: test.logs,
			}

			out := convertTrx(in, nil, ordinal, receipt, nil)

			i := test.beginOrdinal
			assert.Equal(t, i, out.BeginOrdinal)
			i++

			for _, outlog := range out.Receipt.Logs {
				assert.Equal(t, outlog.Ordinal, i)
				i++
			}

			assert.Equal(t, i, out.EndOrdinal)
		})
	}

}

// TestToSetCodeAuthorizations checks that the RPC->pbeth wiring populates the recovered
// 'authority' (via rpc.SetCodeAuthorization.Authority, see eth-go) while leaving 'discarded'
// unset, since it is a Firehose-only, execution-dependent flag the RPC doesn't expose.
// The exact recovery math (RLP payload, magic byte, signature recovery byte offset) is eth-go's
// responsibility and is tested there against the same fixture data.
// TestToFirehoseWithdrawals checks the EIP-4895 conversion, including that a block without
// withdrawals (nil slice, e.g. pre-Shanghai) stays nil rather than becoming an empty slice,
// so it compares equal to a Firehose block that never had withdrawals (see compare-blocks-rpc).
func TestToFirehoseWithdrawals(t *testing.T) {
	assert.Nil(t, ToFirehoseWithdrawals(nil))

	out := ToFirehoseWithdrawals([]rpc.Withdrawal{})
	assert.NotNil(t, out)
	assert.Len(t, out, 0)

	address, err := eth.NewAddress("0x8d58f7e2e58471b46d20a66a61f4cde3c78ab6c0")
	require.NoError(t, err)

	out = ToFirehoseWithdrawals([]rpc.Withdrawal{
		{Index: 0, Validator: 96, Address: eth.Address(address), Amount: 429863},
	})
	require.Len(t, out, 1)
	assert.Equal(t, uint64(0), out[0].Index)
	assert.Equal(t, uint64(96), out[0].ValidatorIndex)
	assert.Equal(t, uint64(429863), out[0].Amount)
	assert.Equal(t, []byte(address), out[0].Address)
}

func TestToSetCodeAuthorizations(t *testing.T) {
	tests := []struct {
		name          string
		json          string
		wantAuthority string // empty means nil/unrecoverable
	}{
		{
			name:          "valid signature",
			json:          `{"chainId":"0x539","address":"0x48b92f7af3a76831e4f5da69959554ad2c134a90","nonce":"0x1","yParity":"0x0","r":"0x7d229398f62c67ab5341ee6b433f3828d23f4b5315b7c7c7f5ec93bd81c16b2a","s":"0xfb34e96be711b61f95f9a4e718b6a77a9dea9eb307dd303a194240ed49a02d5"}`,
			wantAuthority: "0x71562b71999873db5b286df957af199ec94617f7",
		},
		{
			name:          "invalid all-zero signature cannot be recovered",
			json:          `{"chainId":"0x0","address":"0xa1128904651b17348a17344b066b4a0ae69ae06f","nonce":"0x0","yParity":"0x1","r":"0x0","s":"0x0"}`,
			wantAuthority: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var auth rpc.SetCodeAuthorization
			require.NoError(t, json.Unmarshal([]byte(test.json), &auth))

			out := toSetCodeAuthorizations(rpc.AuthorizationList{auth})
			require.Len(t, out, 1)
			assert.False(t, out[0].Discarded)

			if test.wantAuthority == "" {
				assert.Nil(t, out[0].Authority)
				return
			}

			expected, err := eth.NewAddress(test.wantAuthority)
			require.NoError(t, err)
			assert.Equal(t, []byte(expected), out[0].Authority)
		})
	}
}
