package pbtrxstream

import pbeth "github.com/streamingfast/sf-ethereum/types/pb/sf/ethereum/type/v1"

func (t *Transaction) FromTransactionTrace(trace *pbeth.TransactionTrace) {
	t.To = trace.To
	t.Nonce = trace.Nonce
	t.GasPrice = trace.GasPrice
	t.GasLimit = trace.GasLimit
	t.Value = trace.Value
	t.Input = trace.Input
	t.V = trace.V
	t.R = trace.R
	t.S = trace.S
	t.Hash = trace.Hash
	t.From = trace.From
}
