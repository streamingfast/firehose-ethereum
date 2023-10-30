package main

import (
	"encoding/hex"
	"fmt"
	"io"

	"github.com/streamingfast/bstream"
	"github.com/streamingfast/cli"
	firecore "github.com/streamingfast/firehose-core"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
)

var blockEncoder = firecore.NewBlockEncoder()

func printBlock(blk *bstream.Block, alsoPrintTransactions bool, out io.Writer) error {
	block := blk.ToProtocol().(*pbeth.Block)

	if _, err := fmt.Fprintf(out, "Block #%d (%s) (prev: %s): %d transactions\n",
		block.Num(),
		block.ID(),
		block.PreviousID()[0:7],
		len(block.TransactionTraces),
	); err != nil {
		return err
	}

	if alsoPrintTransactions {
		for _, trx := range block.TransactionTraces {
			if _, err := fmt.Fprintf(out, "  - Transaction %s\n", hex.EncodeToString(trx.Hash)); err != nil {
				return err
			}
		}
	}

	return nil
}

func ExamplePrefixed(prefix, examples string) string {
	return string(cli.ExamplePrefixed(prefix, examples))
}
