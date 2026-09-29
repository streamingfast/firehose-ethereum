package main

import (
	"testing"

	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"github.com/stretchr/testify/assert"
)

// TestStripFirehoseTrxReceipt_Nil reproduces reth emitting an empty tx trace with no receipt
// (e.g. an Amsterdam transaction that runs out of gas before its root frame): the receipt must
// not be dereferenced, so a missing receipt can be reported as a diff instead of panicking.
func TestStripFirehoseTrxReceipt_Nil(t *testing.T) {
	assert.NotPanics(t, func() {
		stripFirehoseTrxReceipt(nil)
	})
}

func TestStripFirehoseTransactionTraces_NilReceipt(t *testing.T) {
	traces := []*pbeth.TransactionTrace{
		{
			Hash:    []byte{0x01},
			Receipt: nil,
		},
	}

	assert.NotPanics(t, func() {
		stripFirehoseTransactionTraces(traces, map[string]bool{})
	})
	assert.Nil(t, traces[0].Receipt)
}
