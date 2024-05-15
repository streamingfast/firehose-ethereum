package block

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/holiman/uint256"
	"github.com/streamingfast/eth-go"
	"github.com/streamingfast/eth-go/rpc"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func RpcToEthBlock(in *rpc.Block, receipts map[string]*rpc.TransactionReceipt, logger *zap.Logger) (*pbeth.Block, map[string]bool) {
	trx, hashesWithoutTo := toFirehoseTraces(in.Transactions, receipts, logger)

	var blobGasUsed *uint64
	if in.BlobGasUsed != nil {
		asUint := uint64(*in.BlobGasUsed)
		blobGasUsed = &asUint
	}

	var excessBlobGas *uint64
	if in.ExcessBlobGas != nil {
		asUint := uint64(*in.ExcessBlobGas)
		excessBlobGas = &asUint
	}

	var parentBeaconRoot []byte
	if in.ParentBeaconBlockRoot != nil {
		parentBeaconRoot = (*in.ParentBeaconBlockRoot).Bytes()
	}

	var withdrawalHash []byte
	if in.WithdrawalsHash != nil {
		withdrawalHash = in.WithdrawalsHash.Bytes()
	}

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
			WithdrawalsRoot:  withdrawalHash,
			BlobGasUsed:      blobGasUsed,
			ExcessBlobGas:    excessBlobGas,
			ParentBeaconRoot: parentBeaconRoot,
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

type counter struct {
	val uint64
}

func (c *counter) next() uint64 {
	prev := c.val
	c.val++
	return prev
}

func convertTrx(transaction *rpc.Transaction, toBytes []byte, ordinal *counter, receipt *rpc.TransactionReceipt) *pbeth.TransactionTrace {
	var out *pbeth.TransactionTrace

	out = &pbeth.TransactionTrace{
		Hash:         transaction.Hash.Bytes(),
		To:           toBytes,
		Nonce:        uint64(transaction.Nonce),
		GasLimit:     uint64(transaction.Gas),
		GasPrice:     BigIntFromEthUint256(transaction.GasPrice),
		Input:        transaction.Input.Bytes(),
		Value:        BigIntFromEthUint256(transaction.Value),
		From:         transaction.From.Bytes(),
		Index:        uint32(transaction.TransactionIndex),
		V:            pbeth.NewBigInt(int64(transaction.V)).Bytes,
		R:            BigIntFromEthUint256(transaction.R).Bytes,
		S:            BigIntFromEthUint256(transaction.S).Bytes,
		AccessList:   toAccessList(transaction.AccessList),
		BeginOrdinal: ordinal.next(), // 0 on first trx

		// MaxFeePerGas:            // not available on RPC
		// MaxPriorityFeePerGas:    // not available on RPC
		// ReturnData:              // not available on RPC
		// PublicKey:               // not available on RPC
		// Calls:                   // not available on RPC
	}

	var fhReceipt *pbeth.TransactionReceipt
	fhReceipt = toFirehoseReceipts(receipt, ordinal) // each log will increment the ordinal by 1
	out.Receipt = fhReceipt

	if receipt != nil {
		if receipt.Status != nil {
			out.Status = toFirehoseReceiptStatus(uint64(*receipt.Status))
		}
		out.Type = pbeth.TransactionTrace_Type(receipt.Type)
		out.GasUsed = uint64(receipt.GasUsed)
	}
	out.EndOrdinal = ordinal.next()

	return out
}

func toFirehoseReceiptStatus(in uint64) pbeth.TransactionTraceStatus {
	switch in {
	case 0:
		return pbeth.TransactionTraceStatus_FAILED
	case 1:
		return pbeth.TransactionTraceStatus_SUCCEEDED
	default:
		panic(fmt.Errorf("invalid receipt status: %d", in))
	}
}

func toFirehoseTraces(in *rpc.BlockTransactions, receipts map[string]*rpc.TransactionReceipt, logger *zap.Logger) (traces []*pbeth.TransactionTrace, hashesWithoutTo map[string]bool) {
	ordinal := &counter{}

	transactions, _ := in.Receipts() //todo: this is confusing, Why is it not call Transactions?
	out := make([]*pbeth.TransactionTrace, len(transactions))
	hashesWithoutTo = make(map[string]bool)
	loggedUnknownReceiptStatus := false
	for i := range transactions {
		txHash := eth.Hash(transactions[i].Hash.Bytes()).String()
		var toBytes []byte
		if transactions[i].To != nil {
			toBytes = transactions[i].To.Bytes()
		} else {
			hashesWithoutTo[txHash] = true
		}

		receipt := receipts[transactions[i].Hash.Pretty()]
		pbTrace := convertTrx(&transactions[i], toBytes, ordinal, receipt)
		if !loggedUnknownReceiptStatus && pbTrace.Status == pbeth.TransactionTraceStatus_UNKNOWN {
			logger.Warn("receipt status is nil, firehose transaction status will be 'Unknown' (if this is pre-byzantium, you should try using an Erigon endpoint to get the status)", zap.String("tx_hash", hex.EncodeToString(pbTrace.Hash)))
			loggedUnknownReceiptStatus = true
		}

		out[i] = pbTrace
	}
	return out, hashesWithoutTo
}

func toFirehoseReceipts(receipt *rpc.TransactionReceipt, ordinal *counter) *pbeth.TransactionReceipt {
	if receipt == nil {
		return &pbeth.TransactionReceipt{}
	}

	var logBloom []byte
	if receipt.LogsBloom != nil {
		logBloom = receipt.LogsBloom.Bytes()
	}

	return &pbeth.TransactionReceipt{
		StateRoot:         receipt.Root,
		CumulativeGasUsed: uint64(receipt.CumulativeGasUsed),
		LogsBloom:         logBloom,
		Logs:              toFirehoseLogs(receipt.Logs, ordinal),
	}
}

func toFirehoseLogs(logs []*rpc.LogEntry, ordinal *counter) []*pbeth.Log {
	out := make([]*pbeth.Log, len(logs))
	for i, log := range logs {
		out[i] = &pbeth.Log{
			Address:    log.Address.Bytes(),
			Topics:     HashesToBytes(logs[i].Topics),
			Data:       log.Data.Bytes(),
			BlockIndex: log.ToLog().BlockIndex,
			Ordinal:    ordinal.next(),
			Index:      uint32(log.LogIndex),
		}
	}
	return out
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
