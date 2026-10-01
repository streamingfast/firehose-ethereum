package main

import (
	"bytes"
	"cmp"
	"encoding/hex"
	"slices"
	"sort"

	"github.com/holiman/uint256"
	"github.com/streamingfast/firehose-ethereum/codec"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
)

// convertedBlockVersion is the `Block.ver` written by convertEthereumBlockToV5.
const convertedBlockVersion = 5

// convertEthereumBlockToV5 rewrites a block of version 2, 3 or 4 in place so it matches what a
// version 5 tracer records. Every step leaves an already converted block unchanged, so blocks
// of version 5 written by a tracer that did not filter keccak preimages can go through it too.
func convertEthereumBlockToV5(block *pbeth.Block) {
	if block == nil {
		return
	}

	fixVersion3KnownIssues := block.Ver < 4
	codec.UpgradeBlockV2ToV3(block)

	if fixVersion3KnownIssues {
		moveTransactionOrdinalsAfterSystemCalls(block)
	}

	// Must run before anything is removed, it relies on the ordinal the tracer left unused
	for _, trace := range block.TransactionTraces {
		setMissingCallOrdinals(block, trace)
	}

	for _, trace := range block.TransactionTraces {
		removeEmptyTopicOfRevertedLogs(trace)
		if fixVersion3KnownIssues {
			populateFromRootCall(trace)
		}
	}

	block.BalanceChanges = slices.DeleteFunc(block.BalanceChanges, isNoopBalanceChange)
	block.CodeChanges = slices.DeleteFunc(block.CodeChanges, isNoopCodeChange)
	for _, call := range block.SystemCalls {
		removeUnsupportedChangesFromCall(call)
	}
	for _, trace := range block.TransactionTraces {
		for _, call := range trace.Calls {
			removeUnsupportedChangesFromCall(call)
		}
	}

	// Must run after the no-op storage changes are removed, only written slots keep a preimage
	for _, calls := range splitSystemCalls(block.SystemCalls) {
		keepStorageSlotKeccakPreimages(calls)
	}
	for _, trace := range block.TransactionTraces {
		keepStorageSlotKeccakPreimages(trace.Calls)
	}

	renumberBlockOrdinals(block)

	// Must run once every call has its ordinals, calls are counted in the order they start and end
	limitBlockCallData(block, maxBlockEncodedSize)

	block.Ver = convertedBlockVersion
}

// moveTransactionOrdinalsAfterSystemCalls moves the ordinals of the transactions above those of
// the system calls on blocks where they overlap.
//
// On a block with system calls, the tracers of version 3 and below restarted the ordinals at the
// first transaction, then added the last system call `end_ordinal` to the block level changes
// made after the last transaction only. Adding it to the transactions too gives ordinals that
// are unique in the block.
//
// Blocks from Prague onward are left as they are: some of their system calls run after the
// transactions, so how far the transactions must move is unknown.
func moveTransactionOrdinalsAfterSystemCalls(block *pbeth.Block) {
	if len(block.SystemCalls) == 0 || len(block.TransactionTraces) == 0 || block.GetHeader().GetRequestsHash() != nil {
		return
	}

	lastSystemCallOrdinal := uint64(0)
	for _, call := range block.SystemCalls {
		forEachCallOrdinal(call, func(ordinal *uint64) {
			lastSystemCallOrdinal = max(lastSystemCallOrdinal, *ordinal)
		})
	}

	firstTransactionOrdinal := block.TransactionTraces[0].BeginOrdinal
	for _, trace := range block.TransactionTraces {
		firstTransactionOrdinal = min(firstTransactionOrdinal, trace.BeginOrdinal)
	}
	if firstTransactionOrdinal > lastSystemCallOrdinal {
		return
	}

	for _, trace := range block.TransactionTraces {
		forEachTraceOrdinal(trace, func(ordinal *uint64) {
			if *ordinal != 0 {
				*ordinal += lastSystemCallOrdinal
			}
		})
	}
}

// removeEmptyTopicOfRevertedLogs removes the single empty topic that the tracers of version 3 and
// below recorded on a log without topics emitted by a call whose state was reverted.
func removeEmptyTopicOfRevertedLogs(trace *pbeth.TransactionTrace) {
	for _, call := range trace.Calls {
		if !call.StateReverted {
			continue
		}
		for _, log := range call.Logs {
			if len(log.Topics) == 1 && len(log.Topics[0]) == 0 {
				log.Topics = nil
			}
		}
	}
}

// populateFromRootCall fills the two fields the tracers of version 3 and below left empty and
// that have a copy elsewhere in the transaction: `TransactionTrace.return_data` is the root
// call's return data, and the input of a root CREATE call is the transaction's input.
func populateFromRootCall(trace *pbeth.TransactionTrace) {
	if len(trace.Calls) == 0 {
		return
	}

	root := trace.Calls[0]
	if len(trace.ReturnData) == 0 {
		trace.ReturnData = root.ReturnData
	}
	if root.CallType == pbeth.CallType_CREATE && len(root.Input) == 0 {
		root.Input = trace.Input
	}
}

const (
	// maxCallInputBytesPerTransaction is how many input bytes the internal calls of one
	// transaction may record before later ones are cut to their selector.
	maxCallInputBytesPerTransaction = 50 * 1024 * 1024

	// maxReturnDataBytesPerTransaction is how many return data bytes the internal calls of one
	// transaction may record before later ones are left out.
	maxReturnDataBytesPerTransaction = 25 * 1024 * 1024

	// maxBlockEncodedSize is the largest encoded block that keeps the two limits above.
	maxBlockEncodedSize = 1024 * 1024 * 1024

	callSelectorSize = 4
)

// limitBlockCallData applies the call input and return data limits of the tracers to every
// transaction and system call of the block, then halves both limits and applies them again for
// as long as the block encodes to more than `maxEncodedSize` bytes.
//
// Must stay in sync with the tracers, see call_data_limit.rs in evm-firehose-tracer-rs.
func limitBlockCallData(block *pbeth.Block, maxEncodedSize int) {
	inputLimit, returnDataLimit := maxCallInputBytesPerTransaction, maxReturnDataBytesPerTransaction
	systemCalls := splitSystemCalls(block.SystemCalls)

	for {
		for _, trace := range block.TransactionTraces {
			limitCallData(trace.Calls, inputLimit, returnDataLimit)
		}
		for _, calls := range systemCalls {
			limitCallData(calls, inputLimit, returnDataLimit)
		}

		if (inputLimit == 0 && returnDataLimit == 0) || block.SizeVT() <= maxEncodedSize {
			return
		}

		inputLimit /= 2
		returnDataLimit /= 2
	}
}

// limitCallData applies the limits to the calls of one transaction or of one system call. Once
// the internal calls that started before a call hold more than `inputLimit` bytes of input, the
// call keeps only its selector. Once those that ended before it hold more than
// `returnDataLimit` bytes of return data, the call keeps none. The root call is never truncated.
func limitCallData(calls []*pbeth.Call, inputLimit, returnDataLimit int) {
	inputs, returnData := 0, 0
	for _, call := range calls {
		if call.Depth != 0 {
			inputs += len(call.Input)
			returnData += len(call.ReturnData)
		}
	}
	if inputs <= inputLimit && returnData <= returnDataLimit {
		return
	}

	ordered := slices.Clone(calls)

	slices.SortStableFunc(ordered, func(a, b *pbeth.Call) int { return cmp.Compare(a.BeginOrdinal, b.BeginOrdinal) })
	total := 0
	for _, call := range ordered {
		if call.Depth == 0 {
			continue
		}
		if total > inputLimit && len(call.Input) > callSelectorSize {
			call.Input = call.Input[:callSelectorSize]
			call.InputTruncated = true
		}
		total += len(call.Input)
	}

	slices.SortStableFunc(ordered, func(a, b *pbeth.Call) int { return cmp.Compare(a.EndOrdinal, b.EndOrdinal) })
	total = 0
	for _, call := range ordered {
		if call.Depth == 0 {
			continue
		}
		if total > returnDataLimit && len(call.ReturnData) > 0 {
			call.ReturnData = nil
			call.ReturnDataTruncated = true
		}
		total += len(call.ReturnData)
	}
}

// removeUnsupportedChangesFromCall removes what a version 5 tracer never records on a call:
// gas changes, account creations and state changes whose old and new values are equal.
func removeUnsupportedChangesFromCall(call *pbeth.Call) {
	call.GasChanges = nil
	call.AccountCreations = nil
	call.BalanceChanges = slices.DeleteFunc(call.BalanceChanges, isNoopBalanceChange)
	call.NonceChanges = slices.DeleteFunc(call.NonceChanges, isNoopNonceChange)
	call.CodeChanges = slices.DeleteFunc(call.CodeChanges, isNoopCodeChange)
	call.StorageChanges = slices.DeleteFunc(call.StorageChanges, isNoopStorageChange)
}

func isNoopBalanceChange(c *pbeth.BalanceChange) bool {
	return equalIgnoringLeadingZeros(c.GetOldValue().GetBytes(), c.GetNewValue().GetBytes())
}

func isNoopNonceChange(c *pbeth.NonceChange) bool {
	return c.OldValue == c.NewValue
}

func isNoopStorageChange(c *pbeth.StorageChange) bool {
	return equalIgnoringLeadingZeros(c.OldValue, c.NewValue)
}

func equalIgnoringLeadingZeros(a, b []byte) bool {
	return bytes.Equal(bytes.TrimLeft(a, "\x00"), bytes.TrimLeft(b, "\x00"))
}

// setMissingCallOrdinals gives an ordinal to the calls of the transaction recorded with a
// `begin_ordinal` of 0, and to a root call recorded with an `end_ordinal` of 0 (genesis block
// of version 3 and below).
//
// Calls are visited last to first, so the calls a call made already have their ordinal when it
// is visited.
func setMissingCallOrdinals(block *pbeth.Block, trace *pbeth.TransactionTrace) {
	for i := len(trace.Calls) - 1; i >= 0; i-- {
		call := trace.Calls[i]
		if call.BeginOrdinal != 0 {
			continue
		}

		if call.ParentIndex == 0 {
			call.BeginOrdinal = rootCallBeginOrdinal(block, trace, call)
			continue
		}

		// A call started right before the first thing it recorded
		if first := firstOrdinalWithinCall(trace, call); first != 0 {
			insertOrdinalAt(block, first)
			call.BeginOrdinal = first
		}
	}

	if len(trace.Calls) > 0 && trace.Calls[0].EndOrdinal == 0 && trace.EndOrdinal != 0 {
		end := trace.EndOrdinal
		insertOrdinalAt(block, end)
		trace.Calls[0].EndOrdinal = end
	}
}

// rootCallBeginOrdinal returns the ordinal at which the root call of the transaction started.
//
// The tracers that record a root call `begin_ordinal` of 0 still consumed an ordinal when the
// call started, after the balance and nonce changes made before the call (gas purchase, sender
// nonce bump), which are attached to the root call. That ordinal is the first one above the
// transaction's `begin_ordinal` that nothing in the transaction uses.
//
// When there is no such ordinal, a new one is inserted right after the transaction's
// `begin_ordinal`.
func rootCallBeginOrdinal(block *pbeth.Block, trace *pbeth.TransactionTrace, root *pbeth.Call) uint64 {
	used := map[uint64]struct{}{}
	forEachTraceOrdinal(trace, func(ordinal *uint64) {
		used[*ordinal] = struct{}{}
	})

	unused := trace.BeginOrdinal + 1
	for {
		if _, found := used[unused]; !found {
			break
		}
		unused++
	}

	limit := trace.EndOrdinal
	if root.EndOrdinal != 0 {
		limit = root.EndOrdinal
	}
	for _, call := range trace.Calls {
		if call != root && call.BeginOrdinal != 0 {
			limit = min(limit, call.BeginOrdinal)
		}
	}
	if unused < limit {
		return unused
	}

	insertOrdinalAt(block, trace.BeginOrdinal+1)
	return trace.BeginOrdinal + 1
}

// firstOrdinalWithinCall returns the lowest ordinal recorded by the call or by the calls it
// made, 0 when there is none.
func firstOrdinalWithinCall(trace *pbeth.TransactionTrace, call *pbeth.Call) uint64 {
	first := uint64(0)
	visit := func(ordinal *uint64) {
		if *ordinal != 0 && (first == 0 || *ordinal < first) {
			first = *ordinal
		}
	}

	forEachCallOrdinal(call, visit)
	for _, child := range trace.Calls {
		if child != call && child.ParentIndex == call.Index {
			visit(&child.BeginOrdinal)
		}
	}

	return first
}

// insertOrdinalAt frees `at` by moving every ordinal of the block at or above it up by one.
func insertOrdinalAt(block *pbeth.Block, at uint64) {
	forEachBlockOrdinal(block, func(ordinal *uint64) {
		if *ordinal >= at {
			*ordinal++
		}
	})
}

// renumberBlockOrdinals rewrites the ordinals of the block as 1, 2, 3, ... in their current
// order. Elements sharing an ordinal (a log and its copy in the receipt) keep sharing one, and
// an ordinal of 0 stays 0.
func renumberBlockOrdinals(block *pbeth.Block) {
	var ordinals []uint64
	forEachBlockOrdinal(block, func(ordinal *uint64) {
		if *ordinal != 0 {
			ordinals = append(ordinals, *ordinal)
		}
	})

	slices.Sort(ordinals)
	ordinals = slices.Compact(ordinals)

	forEachBlockOrdinal(block, func(ordinal *uint64) {
		if *ordinal != 0 {
			index, _ := slices.BinarySearch(ordinals, *ordinal)
			*ordinal = uint64(index) + 1
		}
	})
}

// forEachBlockOrdinal calls `fn` with every ordinal field of the block.
func forEachBlockOrdinal(block *pbeth.Block, fn func(ordinal *uint64)) {
	for _, c := range block.BalanceChanges {
		fn(&c.Ordinal)
	}
	for _, c := range block.CodeChanges {
		fn(&c.Ordinal)
	}
	for _, call := range block.SystemCalls {
		forEachCallOrdinal(call, fn)
	}
	for _, trace := range block.TransactionTraces {
		forEachTraceOrdinal(trace, fn)
	}
}

func forEachTraceOrdinal(trace *pbeth.TransactionTrace, fn func(ordinal *uint64)) {
	fn(&trace.BeginOrdinal)
	fn(&trace.EndOrdinal)
	for _, log := range trace.GetReceipt().GetLogs() {
		fn(&log.Ordinal)
	}
	for _, call := range trace.Calls {
		forEachCallOrdinal(call, fn)
	}
}

func forEachCallOrdinal(call *pbeth.Call, fn func(ordinal *uint64)) {
	fn(&call.BeginOrdinal)
	fn(&call.EndOrdinal)
	for _, c := range call.StorageChanges {
		fn(&c.Ordinal)
	}
	for _, c := range call.BalanceChanges {
		fn(&c.Ordinal)
	}
	for _, c := range call.NonceChanges {
		fn(&c.Ordinal)
	}
	for _, c := range call.CodeChanges {
		fn(&c.Ordinal)
	}
	for _, c := range call.GasChanges {
		fn(&c.Ordinal)
	}
	for _, c := range call.AccountCreations {
		fn(&c.Ordinal)
	}
	for _, log := range call.Logs {
		fn(&log.Ordinal)
	}
}

// splitSystemCalls groups `Block.system_calls`, which lists the calls of every system call one
// after the other, into the calls of each system call. Some tracers list the root call of a
// system call first and others last, so an internal call goes with the root call whose ordinals
// surround its own.
func splitSystemCalls(calls []*pbeth.Call) (out [][]*pbeth.Call) {
	var roots []*pbeth.Call
	for _, call := range calls {
		if call.Depth == 0 {
			roots = append(roots, call)
			out = append(out, []*pbeth.Call{call})
		}
	}

	for _, call := range calls {
		if call.Depth == 0 {
			continue
		}
		for i, root := range roots {
			if root.BeginOrdinal <= call.BeginOrdinal && call.EndOrdinal <= root.EndOrdinal {
				out[i] = append(out[i], call)
				break
			}
		}
	}

	return out
}

// keccakFilterMaxDepth is how many levels of nested hashing are followed from a storage key.
// Must stay in sync with the tracers, see keccak_filter.go in evm-firehose-tracer-go.
const keccakFilterMaxDepth = 16

// keepStorageSlotKeccakPreimages removes from `Call.keccak_preimages` the entries a tracer
// filtering preimages does not record. `calls` are all the calls of one transaction or of one
// system call. An entry is kept when its preimage is at most 256 bytes and:
//
//   - a storage change key is its hash, or its hash plus at most 2^64-1, or
//   - its hash, or its hash plus such an offset, appears inside the preimage of a kept entry,
//     following at most keccakFilterMaxDepth such levels.
//
// A preimage recorded as "." (empty input, version 3 and below) is rewritten as an empty string.
func keepStorageSlotKeccakPreimages(calls []*pbeth.Call) {
	preimages := map[[32]byte][]byte{}
	for _, call := range calls {
		for hash, preimage := range call.KeccakPreimages {
			decodedHash, decodedPreimage, ok := decodeKeccakPreimage(hash, preimage)
			if !ok || len(decodedPreimage) > maxComparableKeccakPreimageSize {
				delete(call.KeccakPreimages, hash)
				continue
			}

			preimages[decodedHash] = decodedPreimage
			if preimage == "." {
				call.KeccakPreimages[hash] = ""
			}
		}
	}

	kept := storageSlotKeccakHashes(calls, preimages)
	for _, call := range calls {
		for hash := range call.KeccakPreimages {
			decodedHash, _ := hex.DecodeString(hash)
			if _, found := kept[[32]byte(decodedHash)]; !found {
				delete(call.KeccakPreimages, hash)
			}
		}
		if len(call.KeccakPreimages) == 0 {
			call.KeccakPreimages = nil
		}
	}
}

func decodeKeccakPreimage(hash, preimage string) (decodedHash [32]byte, decodedPreimage []byte, ok bool) {
	hashBytes, err := hex.DecodeString(hash)
	if err != nil || len(hashBytes) != 32 {
		return decodedHash, nil, false
	}

	if preimage != "." {
		decodedPreimage, err = hex.DecodeString(preimage)
		if err != nil {
			return decodedHash, nil, false
		}
	}

	return [32]byte(hashBytes), decodedPreimage, true
}

// storageSlotKeccakHashes returns the hashes of `preimages` that explain one of the storage
// change keys of `calls`.
func storageSlotKeccakHashes(calls []*pbeth.Call, preimages map[[32]byte][]byte) map[[32]byte]struct{} {
	kept := map[[32]byte]struct{}{}
	if len(preimages) == 0 {
		return kept
	}

	sorted := make([][32]byte, 0, len(preimages))
	for hash := range preimages {
		sorted = append(sorted, hash)
	}
	slices.SortFunc(sorted, func(a, b [32]byte) int { return bytes.Compare(a[:], b[:]) })

	var frontier [][32]byte
	keep := func(hash [32]byte, into *[][32]byte) {
		if _, found := kept[hash]; !found {
			kept[hash] = struct{}{}
			*into = append(*into, hash)
		}
	}

	for _, call := range calls {
		for _, change := range call.StorageChanges {
			if len(change.Key) != 32 {
				continue
			}
			if base, ok := keccakSlotBase(sorted, [32]byte(change.Key)); ok {
				keep(base, &frontier)
			}
		}
	}

	for depth := 0; depth < keccakFilterMaxDepth && len(frontier) > 0; depth++ {
		var next [][32]byte
		for _, hash := range frontier {
			for _, word := range innerKeccakHashCandidates(preimages[hash]) {
				if inner, ok := keccakSlotBase(sorted, word); ok {
					keep(inner, &next)
				}
			}
		}
		frontier = next
	}

	return kept
}

// keccakSlotBase returns the largest hash at or below `key` when `key` is less than 2^64
// above it, which covers an exact match.
func keccakSlotBase(sorted [][32]byte, key [32]byte) ([32]byte, bool) {
	i := sort.Search(len(sorted), func(i int) bool { return bytes.Compare(sorted[i][:], key[:]) > 0 })
	if i == 0 {
		return [32]byte{}, false
	}

	base := sorted[i-1]
	distance := new(uint256.Int).Sub(new(uint256.Int).SetBytes32(key[:]), new(uint256.Int).SetBytes32(base[:]))
	return base, distance.IsUint64()
}

// innerKeccakHashCandidates returns the places an inner hash sits in a preimage: each
// 32-byte word (value-type mapping keys, Vyper's slot-first layout) and the last 32 bytes
// (Solidity puts the slot after a `string` or `bytes` key).
func innerKeccakHashCandidates(preimage []byte) [][32]byte {
	candidates := make([][32]byte, 0, len(preimage)/32+1)
	for offset := 0; offset+32 <= len(preimage); offset += 32 {
		candidates = append(candidates, [32]byte(preimage[offset:offset+32]))
	}
	if len(preimage) > 32 && len(preimage)%32 != 0 {
		candidates = append(candidates, [32]byte(preimage[len(preimage)-32:]))
	}

	return candidates
}
