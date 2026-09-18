package main

import (
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
)

// maxComparableKeccakPreimageSize is the preimage size, in bytes, above which a tracer that caps
// `Call.keccak_preimages` records no entry.
const maxComparableKeccakPreimageSize = 256

// removeLargeKeccakPreimagesFromCall drops every `keccak_preimages` entry on the call whose
// preimage exceeds maxComparableKeccakPreimageSize bytes, and returns how many it dropped. Map
// values are hex-encoded, hence the comparison against twice that size.
//
// Shared by the compare sanitizer, which runs it on both sides of a diff so a capped node can be
// compared against one still emitting the large entries, and by
// 'fireeth tools remove-large-keccak-preimages', which runs it over already-written blocks.
func removeLargeKeccakPreimagesFromCall(call *pbeth.Call) int {
	if call == nil {
		return 0
	}

	removed := 0
	for hash, preimage := range call.KeccakPreimages {
		if len(preimage) > 2*maxComparableKeccakPreimageSize {
			delete(call.KeccakPreimages, hash)
			removed++
		}
	}

	return removed
}

// removeLargeKeccakPreimagesFromEthereumBlock applies removeLargeKeccakPreimagesFromCall to every
// call in the block, and returns how many entries it dropped in total.
func removeLargeKeccakPreimagesFromEthereumBlock(block *pbeth.Block) int {
	if block == nil {
		return 0
	}

	removed := 0
	for _, systemCall := range block.SystemCalls {
		removed += removeLargeKeccakPreimagesFromCall(systemCall)
	}

	for _, trace := range block.TransactionTraces {
		for _, call := range trace.Calls {
			removed += removeLargeKeccakPreimagesFromCall(call)
		}
	}

	return removed
}
