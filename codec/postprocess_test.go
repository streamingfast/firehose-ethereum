package codec

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/streamingfast/eth-go"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var H = hex.EncodeToString

// B is a shortcut for (must) hex.DecodeString
var B = func(s string) []byte {
	out, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}

	return out
}

func TestCombinePolygonSystemTransactions(t *testing.T) {

	normalTrx := func(id string) *pbeth.TransactionTrace {
		return &pbeth.TransactionTrace{
			Hash: B(id),
		}
	}
	systemTrx := func(id string, beginOrdinal, endOrdinal uint64, calls []*pbeth.Call) *pbeth.TransactionTrace {
		return &pbeth.TransactionTrace{
			To:           polygonMergeableTrxAddress,
			From:         polygonSystemAddress,
			Hash:         B(id),
			Calls:        calls,
			BeginOrdinal: beginOrdinal,
			EndOrdinal:   endOrdinal,
		}
	}

	call := func(index, parent, depth uint32, beginOrdinal, endOrdinal uint64) *pbeth.Call {
		call := &pbeth.Call{
			Index:        index,
			ParentIndex:  parent,
			Depth:        depth,
			BeginOrdinal: beginOrdinal,
			EndOrdinal:   endOrdinal,
		}
		return call
	}

	systemTrxHash := H(computePolygonHash(0, nil))

	tests := []struct {
		name string
		in   []*pbeth.TransactionTrace

		expectedTrxIDs                []string
		expectedSystemTrxBeginOrdinal uint64
		expectedSystemTrxEndOrdinal   uint64
		expectedCalls                 []*pbeth.Call
	}{
		{
			"no system trx",
			[]*pbeth.TransactionTrace{
				normalTrx("aa"),
				normalTrx("bb"),
			},
			[]string{"aa", "bb"},
			0,
			0,
			nil,
		},
		{
			"single system trx, single call",
			[]*pbeth.TransactionTrace{
				normalTrx("aa"),
				normalTrx("bb"),
				systemTrx("cc", 1, 4, []*pbeth.Call{
					call(1, 0, 0, 2, 3),
				}),
			},
			[]string{"aa", "bb", systemTrxHash},
			1,
			4,
			[]*pbeth.Call{
				call(1, 0, 0, 2, 3),
				call(2, 1, 1, 2, 3),
			},
		},
		{
			"single system trx, nested calls",
			[]*pbeth.TransactionTrace{
				normalTrx("aa"),
				systemTrx("cc", 1, 10, []*pbeth.Call{
					call(1, 0, 0, 2, 9),
					call(2, 1, 1, 3, 6),
					call(3, 2, 2, 4, 5),
					call(4, 1, 1, 7, 8),
				}),
			},
			[]string{"aa", systemTrxHash},
			1,
			10,
			[]*pbeth.Call{
				call(1, 0, 0, 2, 9),
				call(2, 1, 1, 2, 9),
				call(3, 2, 2, 3, 6),
				call(4, 3, 3, 4, 5),
				call(5, 2, 2, 7, 8),
			},
		},
		{
			"multiple system trx, nested calls",
			[]*pbeth.TransactionTrace{
				normalTrx("aa"),
				systemTrx("cc", 1, 10, []*pbeth.Call{
					call(1, 0, 0, 2, 9),
					call(2, 1, 1, 3, 6),
					call(3, 2, 2, 4, 5),
					call(4, 1, 1, 7, 8),
				}),
				systemTrx("dd", 11, 20, []*pbeth.Call{
					call(1, 0, 0, 12, 19),
					call(2, 1, 1, 13, 16),
					call(3, 2, 2, 14, 15),
					call(4, 1, 1, 17, 18),
				}),

				systemTrx("dd", 21, 30, []*pbeth.Call{
					call(1, 0, 0, 22, 29),
					call(2, 1, 1, 23, 28),
					call(3, 2, 2, 24, 27),
					call(4, 3, 3, 25, 26),
				}),
			},
			[]string{"aa", systemTrxHash},
			1,
			30,
			[]*pbeth.Call{
				call(1, 0, 0, 2, 29),

				call(2, 1, 1, 2, 9),
				call(3, 2, 2, 3, 6),
				call(4, 3, 3, 4, 5),
				call(5, 2, 2, 7, 8),

				call(6, 1, 1, 12, 19),
				call(7, 6, 2, 13, 16),
				call(8, 7, 3, 14, 15),
				call(9, 6, 2, 17, 18),

				call(10, 1, 1, 22, 29),
				call(11, 10, 2, 23, 28),
				call(12, 11, 3, 24, 27),
				call(13, 12, 4, 25, 26),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			out := CombinePolygonSystemTransactions(test.in, 0, nil)

			var systemTrx *pbeth.TransactionTrace
			var trxIDs []string
			for _, trx := range out {
				trxIDs = append(trxIDs, H(trx.Hash))
				if H(trx.Hash) == systemTrxHash {
					systemTrx = trx
				}
			}
			assert.Equal(t, test.expectedTrxIDs, trxIDs)

			if test.expectedCalls == nil {
				require.Nil(t, systemTrx, "expected to find no system transaction")
				return
			}

			assert.Equal(t, test.expectedSystemTrxBeginOrdinal, systemTrx.BeginOrdinal)
			assert.Equal(t, test.expectedSystemTrxEndOrdinal, systemTrx.EndOrdinal)

			for i := range test.expectedCalls {
				assert.Equal(t, test.expectedCalls[i].Index, systemTrx.Calls[i].Index, fmt.Sprintf("call number %d", i))
				assert.Equal(t, test.expectedCalls[i].ParentIndex, systemTrx.Calls[i].ParentIndex, fmt.Sprintf("call index %d", systemTrx.Calls[i].Index))
				assert.Equal(t, test.expectedCalls[i].Depth, systemTrx.Calls[i].Depth, fmt.Sprintf("call index %d", systemTrx.Calls[i].Index))
				assert.Equal(t, test.expectedCalls[i].BeginOrdinal, systemTrx.Calls[i].BeginOrdinal, fmt.Sprintf("call index %d", systemTrx.Calls[i].Index))
				assert.Equal(t, test.expectedCalls[i].EndOrdinal, systemTrx.Calls[i].EndOrdinal, fmt.Sprintf("call index %d", systemTrx.Calls[i].Index))
			}

		})
	}

}

func TestComputeLogsBloom(t *testing.T) {
	jsondata := `{
               "logs": [
                 { "address": "0x8397259c983751daf40400790063935a11afa28a", "topics": ["0xf091cd9cbbaff01426d8183042dff452ef18e6690f19816d5dd114e00761e0e8"] },
                 { "address": "0xbc822318284ad00cdc0ad7610d510c20431e8309", "topics": ["0x53a33d22ff9200fa66da20c899619dc2a429273409f028609fd7bf9bf77d03ae"] },
                 { "address": "0x7f4fb56b9c85bab8b89c8879a660f7eaaa95a3a8", "topics": ["0x0fe89019817f63ee7249ee3d71c789fa752a3852542ddfa0d7b43a9bb8cc9226"] },
                 { "address": "0x7a9a3395afb32f923a142dbc56467ae5675ce5ec", "topics": ["0x830b4c0efef1115af5191b1c6a31cb7859224a5c32e05d6e494cdc9315ac64f1"] },
                 { "address": "0x7a9a3395afb32f923a142dbc56467ae5675ce5ec", "topics": ["0x5c7a2ce7c3302f78e27b903b0323f1956dd55f0bf5c4f402b1796638b3dcce1f"] },
                 { "address": "0x7f4fb56b9c85bab8b89c8879a660f7eaaa95a3a8", "topics": ["0x0fe89019817f63ee7249ee3d71c789fa752a3852542ddfa0d7b43a9bb8cc9226"] },
                 { "address": "0xe06229f72124c7936e42c6fbd645ee688419d5e5", "topics": ["0x830b4c0efef1115af5191b1c6a31cb7859224a5c32e05d6e494cdc9315ac64f1"] },
                 { "address": "0xe06229f72124c7936e42c6fbd645ee688419d5e5", "topics": ["0x5c7a2ce7c3302f78e27b903b0323f1956dd55f0bf5c4f402b1796638b3dcce1f"] },
                 { "address": "0x7f4fb56b9c85bab8b89c8879a660f7eaaa95a3a8", "topics": ["0x0fe89019817f63ee7249ee3d71c789fa752a3852542ddfa0d7b43a9bb8cc9226"] },
                 { "address": "0x8397259c983751daf40400790063935a11afa28a", "topics": ["0xf091cd9cbbaff01426d8183042dff452ef18e6690f19816d5dd114e00761e0e8"] },
                 { "address": "0x341a6166a30b93ebc0c9a7a596aaca22b89b2f22", "topics": [
                         "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef",
                         "0x0000000000000000000000000000000000000000000000000000000000000000",
                         "0x000000000000000000000000e3a652de6015903255f19f0299053d795dce665f"
                 ]}
               ],
               "logsBloom": "0x08010000400000000000080080000000000000401200000000000000000400000000040000000000000000000010000000000000400000000000001000000000000000000000200010008008400000040000002000000000000000000000010000000000020001000010000000000800000000000000000000080010000000001000000000000000000000000000000000000000400200040000000000000000000000000000000000000000000000000000000000000000000004000000004200000002000400000000800000000000000002000000000000000000000020000000008000800000000000000000000000000000004000000000000000000000"
         }`

	v := map[string]interface{}{}
	err := json.Unmarshal([]byte(jsondata), &v)
	require.NoError(t, err)

	var goodLogs []*pbeth.Log
	logs := v["logs"].([]interface{})
	for _, rawLog := range logs {
		log := rawLog.(map[string]interface{})
		rawAddress := log["address"].(string)
		rawTopics := log["topics"].([]interface{})

		out := &pbeth.Log{
			Address: eth.MustNewAddress(rawAddress),
		}

		for _, rawTopic := range rawTopics {
			out.Topics = append(out.Topics, eth.MustNewHex(rawTopic.(string)))
		}
		goodLogs = append(goodLogs, out)
	}
	bloom := computeLogsBloom(goodLogs)

	expected := eth.MustNewHex(v["logsBloom"].(string))

	assert.Equal(t, hex.EncodeToString(expected), hex.EncodeToString(bloom))
}

func TestPopulateStateReverted(t *testing.T) {
	trxTrace := func(calls ...*pbeth.Call) *pbeth.TransactionTrace {
		return &pbeth.TransactionTrace{
			Hash:  B("ff"),
			Calls: calls,
		}
	}

	call := func(index, parent uint32, status string) *pbeth.Call {
		call := &pbeth.Call{
			Index:       index,
			ParentIndex: parent,
		}

		if status == "failed" {
			call.StatusFailed = true
		}

		if status != "failed" && status != "succeeded" {
			require.Fail(t, "only failed or succeeded status are permitted")
		}

		return call
	}

	tests := []struct {
		name     string
		in       *pbeth.TransactionTrace
		expected map[uint32]bool
	}{
		{
			"single-call-success",
			trxTrace(
				call(1, 0, "succeeded"),
			),
			map[uint32]bool{
				1: false,
			},
		},
		{
			"single-call-failed",
			trxTrace(
				call(1, 0, "failed"),
			),
			map[uint32]bool{
				1: true,
			},
		},
		{
			"single-child-success",
			trxTrace(
				call(1, 0, "succeeded"),
				call(2, 1, "succeeded"),
			),
			map[uint32]bool{
				1: false,
				2: false,
			},
		},
		{
			"single-child-success-child-failed",
			trxTrace(
				call(1, 0, "succeeded"),
				call(2, 1, "failed"),
			),
			map[uint32]bool{
				1: false,
				2: true,
			},
		},
		{
			"single-child-success-parent-failed",
			trxTrace(
				call(1, 0, "failed"),
				call(2, 1, "succeeded"),
			),
			map[uint32]bool{
				1: true,
				2: true,
			},
		},
		{
			"multi-child-all-success",
			trxTrace(
				call(1, 0, "succeeded"),
				call(2, 1, "succeeded"),
				call(3, 1, "succeeded"),
				call(4, 1, "succeeded"),
			),
			map[uint32]bool{
				1: false,
				2: false,
				3: false,
				4: false,
			},
		},
		{
			"multi-child-all-success-middle-child-failed",
			trxTrace(
				call(1, 0, "succeeded"),
				call(2, 1, "succeeded"),
				call(3, 1, "succeeded"),
				call(4, 1, "succeeded"),
			),
			map[uint32]bool{
				1: false,
				2: false,
				3: false,
				4: false,
			},
		},
		{
			"multi-child-nested-all-success",
			trxTrace(
				call(1, 0, "succeeded"),
				call(2, 1, "succeeded"),
				call(3, 2, "succeeded"),
				call(4, 3, "succeeded"),
				call(5, 2, "succeeded"),
				call(6, 1, "succeeded"),
				call(7, 1, "succeeded"),
			),
			map[uint32]bool{
				1: false,
				2: false,
				3: false,
				4: false,
				5: false,
				6: false,
				7: false,
			},
		},
		{
			"multi-child-nested-only-root-failed",
			trxTrace(
				call(1, 0, "failed"),
				call(2, 1, "succeeded"),
				call(3, 2, "succeeded"),
				call(4, 3, "succeeded"),
				call(5, 2, "succeeded"),
				call(6, 1, "succeeded"),
				call(7, 1, "succeeded"),
			),
			map[uint32]bool{
				1: true,
				2: true,
				3: true,
				4: true,
				5: true,
				6: true,
				7: true,
			},
		},
		{
			"multi-child-nested-parent-level1-failed",
			trxTrace(
				call(1, 0, "succeeded"),
				call(2, 1, "failed"),
				call(3, 2, "succeeded"),
				call(4, 3, "succeeded"),
				call(5, 2, "succeeded"),
				call(6, 2, "succeeded"),
				call(7, 2, "succeeded"),
				call(8, 7, "succeeded"),
				call(9, 1, "succeeded"),
			),
			map[uint32]bool{
				1: false,
				2: true,
				3: true,
				4: true,
				5: true,
				6: true,
				7: true,
				8: true,
				9: false,
			},
		},
		{
			"multi-child-nested-parent-level2-no-child",
			trxTrace(
				call(1, 0, "succeeded"),
				call(2, 1, "succeeded"),
				call(3, 2, "succeeded"),
				call(4, 3, "succeeded"),
				call(5, 2, "failed"),
				call(6, 2, "failed"),
				call(7, 2, "succeeded"),
				call(8, 7, "succeeded"),
				call(9, 1, "succeeded"),
			),
			map[uint32]bool{
				1: false,
				2: false,
				3: false,
				4: false,
				5: true,
				6: true,
				7: false,
				8: false,
				9: false,
			},
		},
		{
			"multi-child-nested-parent-level2-with-child-with-following-sibling",
			trxTrace(
				call(1, 0, "succeeded"),
				call(2, 1, "succeeded"),
				call(3, 2, "failed"),
				call(4, 3, "succeeded"),
				call(5, 2, "succeeded"),
				call(6, 2, "succeeded"),
				call(7, 2, "succeeded"),
				call(8, 7, "succeeded"),
				call(9, 1, "succeeded"),
			),
			map[uint32]bool{
				1: false,
				2: false,
				3: true,
				4: true,
				5: false,
				6: false,
				7: false,
				8: false,
				9: false,
			},
		},
		{
			"multi-child-nested-parent-level2-with-child-no-following-sibling",
			trxTrace(
				call(1, 0, "succeeded"),
				call(2, 1, "succeeded"),
				call(3, 2, "succeeded"),
				call(4, 3, "succeeded"),
				call(5, 2, "succeeded"),
				call(6, 2, "succeeded"),
				call(7, 2, "failed"),
				call(8, 7, "succeeded"),
				call(9, 1, "succeeded"),
			),
			map[uint32]bool{
				1: false,
				2: false,
				3: false,
				4: false,
				5: false,
				6: false,
				7: true,
				8: true,
				9: false,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			populateStateReverted(test.in)

			for _, call := range test.in.Calls {
				expected, exists := test.expected[call.Index]

				assert.Equal(t, true, exists, "Call %d not in expected map", call.Index)
				assert.Equal(t, expected, call.StateReverted, "Call %d state reverted mismatch", call.Index)
			}
		})
	}
}
