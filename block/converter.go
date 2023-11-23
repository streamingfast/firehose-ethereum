package block

import (
	"bytes"
	"fmt"
	"time"

	"github.com/holiman/uint256"
	"github.com/streamingfast/eth-go"
	"github.com/streamingfast/eth-go/rpc"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ToEthBlock func(in *rpc.Block, logs []*rpc.LogEntry) (*pbeth.Block, map[string]bool)

func RpcToEthBlock(in *rpc.Block, logs []*rpc.LogEntry) (*pbeth.Block, map[string]bool) {

	trx, hashesWithoutTo := toFirehoseTraces(in.Transactions, logs)

	out := &pbeth.Block{
		DetailLevel:       pbeth.Block_DETAILLEVEL_BASE,
		Hash:              in.Hash.Bytes(),
		Number:            uint64(in.Number),
		Ver:               3,
		Size:              uint64(in.BlockSize),
		Uncles:            toFirehoseUncles(in.Uncles),
		TransactionTraces: trx,
		BalanceChanges:    nil, // not available
		CodeChanges:       nil, // not available
		Header: &pbeth.BlockHeader{
			ParentHash:       in.ParentHash.Bytes(),
			Coinbase:         in.Miner,
			UncleHash:        in.UnclesSHA3,
			StateRoot:        in.StateRoot.Bytes(),
			TransactionsRoot: in.TransactionsRoot.Bytes(),
			ReceiptRoot:      in.ReceiptsRoot.Bytes(),
			LogsBloom:        in.LogsBloom.Bytes(),
			Difficulty:       BigIntFromEthUint256(in.Difficulty),
			TotalDifficulty:  BigIntFromEthUint256(in.TotalDifficulty),
			Number:           uint64(in.Number),
			GasLimit:         uint64(in.GasLimit),
			GasUsed:          uint64(in.GasUsed),
			Timestamp:        timestamppb.New(time.Time(in.Timestamp)),
			ExtraData:        in.ExtraData.Bytes(),
			Nonce:            uint64(in.Nonce),
			Hash:             in.Hash.Bytes(),
			MixHash:          in.MixHash.Bytes(),
			BaseFeePerGas:    BigIntFromEthUint256(in.BaseFeePerGas),
			WithdrawalsRoot:  nil, // not available
			TxDependency:     nil, // not available
		},
	}
	return out, hashesWithoutTo
}

func toFirehoseUncles(in []eth.Hash) []*pbeth.BlockHeader {
	out := make([]*pbeth.BlockHeader, len(in))
	for i := range in {
		out[i] = &pbeth.BlockHeader{
			Hash: in[i].Bytes(),
		}
	}
	return out
}

func toAccessList(in rpc.AccessList) []*pbeth.AccessTuple {
	out := make([]*pbeth.AccessTuple, len(in))
	for i, v := range in {
		out[i] = &pbeth.AccessTuple{
			Address: v.Address,
		}
		if v.StorageKeys != nil {
			out[i].StorageKeys = make([][]byte, len(v.StorageKeys))
			for ii, vv := range v.StorageKeys {
				out[i].StorageKeys[ii] = []byte(vv)
			}
		}
	}

	return out
}

func toFirehoseTraces(in *rpc.BlockTransactions, logs []*rpc.LogEntry) (traces []*pbeth.TransactionTrace, hashesWithoutTo map[string]bool) {

	ordinal := uint64(0)

	receipts, _ := in.Receipts()
	out := make([]*pbeth.TransactionTrace, len(receipts))
	hashesWithoutTo = make(map[string]bool)
	for i := range receipts {
		txHash := eth.Hash(receipts[i].Hash.Bytes()).String()
		var toBytes []byte
		if receipts[i].To != nil {
			toBytes = receipts[i].To.Bytes()
		} else {
			hashesWithoutTo[txHash] = true
		}

		out[i] = &pbeth.TransactionTrace{
			Hash:     receipts[i].Hash.Bytes(),
			To:       toBytes,
			Nonce:    uint64(receipts[i].Nonce),
			GasLimit: uint64(receipts[i].Gas),
			GasPrice: BigIntFromEthUint256(receipts[i].GasPrice),
			Input:    receipts[i].Input.Bytes(),
			Value:    BigIntFromEthUint256(receipts[i].Value),
			From:     receipts[i].From.Bytes(),
			Index:    uint32(receipts[i].TransactionIndex),
			Receipt:  &pbeth.TransactionReceipt{
				// Logs: ,            // filled below
				// CumulativeGasUsed: // only available on getTransactionReceipt
				// StateRoot:         // only available on getTransactionReceipt
				// LogsBloom:         // only available on getTransactionReceipt
			},
			V:            pbeth.NewBigInt(int64(receipts[i].V)).Bytes,
			R:            BigIntFromEthUint256(receipts[i].R).Bytes,
			S:            BigIntFromEthUint256(receipts[i].S).Bytes,
			AccessList:   toAccessList(receipts[i].AccessList),
			BeginOrdinal: ordinal,

			// Status:  // only available on getTransactionReceipt
			// Type:    // only available on getTransactionReceipt
			// GasUsed: // only available on getTransactionReceipt
			// MaxFeePerGas:            // not available on RPC
			// MaxPriorityFeePerGas:    // not available on RPC
			// ReturnData:              // not available on RPC
			// PublicKey:               // not available on RPC
			// Calls:                   // not available on RPC
		}
		ordinal++

		var prevBlockIndex *uint32
		for li, log := range logs {
			currentBlockIndex := log.ToLog().BlockIndex
			if log.TransactionHash.String() == txHash {
				out[i].Receipt.Logs = append(out[i].Receipt.Logs, &pbeth.Log{
					Address:    log.Address.Bytes(),       //[]byte   `protobuf:"bytes,1,opt,name=address,proto3" json:"address,omitempty"`
					Topics:     HashesToBytes(log.Topics), //[][]byte `protobuf:"bytes,2,rep,name=topics,proto3" json:"topics,omitempty"`
					Data:       log.Data.Bytes(),          //[]byte   `protobuf:"bytes,3,opt,name=data,proto3" json:"data,omitempty"`
					BlockIndex: currentBlockIndex,         //uint32 `protobuf:"varint,6,opt,name=blockIndex,proto3" json:"blockIndex,omitempty"`
					Ordinal:    ordinal,
					Index:      uint32(li),
				})
				if prevBlockIndex != nil && currentBlockIndex-1 != *prevBlockIndex {
					panic(fmt.Errorf("block index mismatch: %d != %d", currentBlockIndex, *prevBlockIndex))
				}

				prevBlockIndex = &currentBlockIndex
				ordinal++
			}
		}
		out[i].EndOrdinal = ordinal
		ordinal++

	}
	return out, hashesWithoutTo
}

func BigIntFromEthUint256(in *eth.Uint256) *pbeth.BigInt {
	if in == nil {
		return &pbeth.BigInt{}
	}

	in32 := (*uint256.Int)(in).Bytes32()
	slice := bytes.TrimLeft(in32[:], string([]byte{0}))
	if len(slice) == 0 {
		return &pbeth.BigInt{}
	}
	return pbeth.BigIntFromBytes(slice)
}

func BigIntFromEthUint256Padded32(in *eth.Uint256) *pbeth.BigInt {
	if in == nil {
		return &pbeth.BigInt{}
	}

	in32 := (*uint256.Int)(in).Bytes32()

	if in32 == [32]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0} {
		return &pbeth.BigInt{}
	}
	return pbeth.BigIntFromBytes(in32[:])
}

func HashesToBytes(in []eth.Hash) [][]byte {
	out := make([][]byte, len(in))
	for i := range in {
		out[i] = in[i].Bytes()
	}
	return out
}
