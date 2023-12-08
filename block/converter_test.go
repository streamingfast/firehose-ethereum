package block

import (
	"fmt"
	"testing"

	"github.com/streamingfast/eth-go/rpc"
	"github.com/stretchr/testify/assert"
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

			out := convertTrx(in, nil, ordinal, receipt)

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
