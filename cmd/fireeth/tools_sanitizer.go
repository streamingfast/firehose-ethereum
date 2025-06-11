package main

import (
	"fmt"

	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	firecore "github.com/streamingfast/firehose-core"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
)

func SanitizeEthereumBlockForCompare(block *pbbstream.Block) *pbbstream.Block {
	untypedEthBlock, err := block.Payload.UnmarshalNew()
	if err != nil {
		panic(fmt.Errorf("unexpected block message %s, unable to unmarshal to proto.Message: %w", block.Payload.TypeUrl, err))
	}

	ethBlock, ok := untypedEthBlock.(*pbeth.Block)
	if !ok {
		panic(fmt.Errorf("unexpected block message %s, unable to cast to pbeth.Block", block.Payload.TypeUrl))
	}

	// The TotalDifficulty field has been removed in newer versions of the Geth,
	// so it's impossible to have it good in all cases, forcing it to nil.
	if ethBlock.Header.TotalDifficulty != nil {
		ethBlock.Header.TotalDifficulty = nil
	}
	var hasLogBloom bool
	for _, byte := range ethBlock.Header.LogsBloom {
		if byte != '0' {
			hasLogBloom = true
			break
		}
	}
	if !hasLogBloom {
		ethBlock.Header.LogsBloom = nil
	}

	for _, tx := range ethBlock.TransactionTraces {
		var hasLogBloom bool
		for _, byte := range tx.Receipt.LogsBloom {
			if byte != '0' {
				hasLogBloom = true
				break
			}
		}
		if !hasLogBloom {
			tx.Receipt.LogsBloom = nil
		}
		tx.Receipt.LogsBloom = nil
		for _, call := range tx.Calls {
			if call.FailureReason != "" {
				call.FailureReason = "<error replaced for comparison>"
			}
		}
	}

	out, err := firecore.EncodeBlock(firecore.BlockEnveloppe{
		Block:  ethBlock,
		LIBNum: block.LibNum,
	})
	if err != nil {
		panic(fmt.Errorf("unable to encode block: %w", err))
	}

	return out
}
