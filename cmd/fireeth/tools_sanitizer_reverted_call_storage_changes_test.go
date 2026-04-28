package main

import (
	"testing"

	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"github.com/stretchr/testify/require"
)

// Test_removeRevertedCallStorageChangesFromEthereumBlock exercises the full
// removal-and-shift path across all three flavors of calls:
//
//   - Calls[0]: Index=0, StatusReverted=true  → NOT affected (root call is exempt)
//   - Calls[1]: Index=1, StatusReverted=true  → storage changes REMOVED, ordinals shifted
//   - Calls[2]: Index=2, StatusReverted=false → NOT affected (not reverted)
//
// Removed ordinals (sorted): [22, 24]
// shiftOrdinal uses "how many removed ordinals are ≤ value" as the shift amount.
func Test_removeRevertedCallStorageChangesFromEthereumBlock(t *testing.T) {
	block := &pbeth.Block{
		// System calls are completely outside the scope of this function.
		SystemCalls: []*pbeth.Call{
			{
				BeginOrdinal:   1,
				EndOrdinal:     5,
				StorageChanges: []*pbeth.StorageChange{{Ordinal: 3}},
			},
		},
		BalanceChanges: []*pbeth.BalanceChange{{Ordinal: 8}},
		TransactionTraces: []*pbeth.TransactionTrace{
			{
				BeginOrdinal: 10,
				EndOrdinal:   30,
				Receipt: &pbeth.TransactionReceipt{
					Logs: []*pbeth.Log{{Ordinal: 29}},
				},
				Calls: []*pbeth.Call{
					{
						// Index=0, reverted: exempt from removal.
						Index:          0,
						StatusReverted: true,
						BeginOrdinal:   10,
						EndOrdinal:     20,
						StorageChanges: []*pbeth.StorageChange{{Ordinal: 12}},
						BalanceChanges: []*pbeth.BalanceChange{{Ordinal: 14}},
					},
					{
						// Index=1, reverted: storage changes removed.
						Index:          1,
						StatusReverted: true,
						BeginOrdinal:   21,
						EndOrdinal:     26,
						StorageChanges: []*pbeth.StorageChange{
							{Ordinal: 22}, // removed
							{Ordinal: 24}, // removed
						},
						NonceChanges: []*pbeth.NonceChange{{Ordinal: 25}},
					},
					{
						// Index=2, not reverted: untouched.
						Index:          2,
						StatusReverted: false,
						BeginOrdinal:   27,
						EndOrdinal:     30,
						StorageChanges: []*pbeth.StorageChange{{Ordinal: 28}},
						Logs:           []*pbeth.Log{{Ordinal: 29}},
					},
				},
			},
		},
	}

	removeRevertedCallStorageChangesFromEthereumBlock(block)

	// ── System call: untouched ────────────────────────────────────────────────
	require.Equal(t, uint64(1), block.SystemCalls[0].BeginOrdinal)
	require.Equal(t, uint64(5), block.SystemCalls[0].EndOrdinal)
	require.Equal(t, uint64(3), block.SystemCalls[0].StorageChanges[0].Ordinal)

	// ── Block-level balance change: below all removed ordinals, no shift ──────
	require.Equal(t, uint64(8), block.BalanceChanges[0].Ordinal)

	tx := block.TransactionTraces[0]

	// ── Transaction: EndOrdinal shifts by 2 (22 and 24 are both ≤ 30) ────────
	require.Equal(t, uint64(10), tx.BeginOrdinal) // below 22, no shift
	require.Equal(t, uint64(28), tx.EndOrdinal)   // 30 − 2

	// ── Calls[0] (Index=0, reverted): storage changes kept, ordinals shift ───
	c0 := tx.Calls[0]
	require.Equal(t, uint64(10), c0.BeginOrdinal)              // below 22, no shift
	require.Equal(t, uint64(20), c0.EndOrdinal)                // below 22, no shift
	require.Len(t, c0.StorageChanges, 1)                       // kept
	require.Equal(t, uint64(12), c0.StorageChanges[0].Ordinal) // below 22, no shift
	require.Equal(t, uint64(14), c0.BalanceChanges[0].Ordinal) // below 22, no shift

	// ── Calls[1] (Index=1, reverted): storage changes gone, others shift ─────
	c1 := tx.Calls[1]
	require.Nil(t, c1.StorageChanges)
	require.Equal(t, uint64(21), c1.BeginOrdinal)            // 21 < 22, no shift
	require.Equal(t, uint64(24), c1.EndOrdinal)              // 26 − 2
	require.Equal(t, uint64(23), c1.NonceChanges[0].Ordinal) // 25 − 2

	// ── Calls[2] (Index=2, not reverted): storage changes kept, ordinals shift
	c2 := tx.Calls[2]
	require.Len(t, c2.StorageChanges, 1)
	require.Equal(t, uint64(25), c2.BeginOrdinal)              // 27 − 2
	require.Equal(t, uint64(28), c2.EndOrdinal)                // 30 − 2
	require.Equal(t, uint64(26), c2.StorageChanges[0].Ordinal) // 28 − 2
	require.Equal(t, uint64(27), c2.Logs[0].Ordinal)           // 29 − 2
}

func Test_removeRevertedCallStorageChangesFromEthereumBlock_nil(t *testing.T) {
	require.NotPanics(t, func() { removeRevertedCallStorageChangesFromEthereumBlock(nil) })
}

// A root call (Index=0) that is reverted must NOT have its storage changes removed.
func Test_removeRevertedCallStorageChangesFromEthereumBlock_rootRevertedCallIsExempt(t *testing.T) {
	block := &pbeth.Block{
		TransactionTraces: []*pbeth.TransactionTrace{
			{
				BeginOrdinal: 1,
				EndOrdinal:   5,
				Calls: []*pbeth.Call{
					{
						Index:          0,
						StatusReverted: true,
						BeginOrdinal:   1,
						EndOrdinal:     5,
						StorageChanges: []*pbeth.StorageChange{{Ordinal: 3}},
					},
				},
			},
		},
	}

	removeRevertedCallStorageChangesFromEthereumBlock(block)

	c := block.TransactionTraces[0].Calls[0]
	require.Len(t, c.StorageChanges, 1)
	require.Equal(t, uint64(3), c.StorageChanges[0].Ordinal) // unchanged
}

// A non-reverted call must never have its storage changes touched.
func Test_removeRevertedCallStorageChangesFromEthereumBlock_nonRevertedCallUntouched(t *testing.T) {
	block := &pbeth.Block{
		TransactionTraces: []*pbeth.TransactionTrace{
			{
				BeginOrdinal: 1,
				EndOrdinal:   5,
				Calls: []*pbeth.Call{
					{
						Index:          1,
						StatusReverted: false,
						BeginOrdinal:   1,
						EndOrdinal:     5,
						StorageChanges: []*pbeth.StorageChange{{Ordinal: 3}},
					},
				},
			},
		},
	}

	removeRevertedCallStorageChangesFromEthereumBlock(block)

	c := block.TransactionTraces[0].Calls[0]
	require.Len(t, c.StorageChanges, 1)
	require.Equal(t, uint64(3), c.StorageChanges[0].Ordinal) // unchanged
}

// A storage change with ordinal 0 on a reverted call is removed but must not
// contribute to the ordinal shift of other entries.
func Test_removeRevertedCallStorageChangesFromEthereumBlock_zeroOrdinalDoesNotShift(t *testing.T) {
	block := &pbeth.Block{
		TransactionTraces: []*pbeth.TransactionTrace{
			{
				BeginOrdinal: 10,
				EndOrdinal:   11,
				Calls: []*pbeth.Call{
					{
						Index:          1,
						StatusReverted: true,
						BeginOrdinal:   10,
						EndOrdinal:     11,
						StorageChanges: []*pbeth.StorageChange{{Ordinal: 0}},
						Logs:           []*pbeth.Log{{Ordinal: 11}},
					},
				},
			},
		},
	}

	removeRevertedCallStorageChangesFromEthereumBlock(block)

	c := block.TransactionTraces[0].Calls[0]
	require.Nil(t, c.StorageChanges)
	require.Equal(t, uint64(10), c.BeginOrdinal)    // no shift
	require.Equal(t, uint64(11), c.EndOrdinal)      // no shift
	require.Equal(t, uint64(11), c.Logs[0].Ordinal) // no shift
}
