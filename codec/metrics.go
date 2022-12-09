package codec

import (
	"github.com/streamingfast/dmetrics"
)

var metrics = dmetrics.NewSet(dmetrics.PrefixNameWith("console_reader"))

func init() {
	metrics.Register()
}

var BlockReadCount = metrics.NewCounter("block_read_count", "The number of blocks read by the Console Reader")
var TransactionReadCount = metrics.NewCounter("trx_read_count", "The number of transactions read by the Console Reader")

var BlockTotalParseTime = metrics.NewCounter("block_total_parse_time", "The total parse time (wall clock) it took to extract all blocks so far")
var TrxTotalParseTime = metrics.NewCounter("trx_total_parse_time", "The total parse time (wall clock) it took to extract all transactions so far")
