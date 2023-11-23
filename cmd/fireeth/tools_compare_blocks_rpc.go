// Copyright 2021 dfuse Platform Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"

	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"

	jd "github.com/josephburnett/jd/lib"
	"github.com/mostynb/go-grpc-compression/zstd"
	"github.com/spf13/cobra"
	"github.com/streamingfast/cli"
	"github.com/streamingfast/cli/sflags"
	"github.com/streamingfast/eth-go"
	"github.com/streamingfast/eth-go/rpc"
	firecore "github.com/streamingfast/firehose-core"
	"github.com/streamingfast/firehose-ethereum/block"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"github.com/streamingfast/firehose/client"
	pbfirehose "github.com/streamingfast/pbgo/sf/firehose/v2"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func newCompareBlocksRPCCmd(logger *zap.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compare-blocks-rpc <firehose-endpoint> <rpc-endpoint> <start-block> <stop-block>",
		Short: "Checks for any differences between a Firehose and RPC endpoint (get_block) for a specified range.",
		Long: cli.Dedent(`
			The 'compare-blocks-rpc' takes in a firehose URL, an RPC endpoint URL and inclusive start/stop block numbers.
		`),
		Args: cobra.ExactArgs(4),
		RunE: createCompareBlocksRPCE(logger),
		Example: examplePrefixed("fireeth tools compare-blocks-rpc", `
			# Run over full block range
			mainnet.eth.streamingfast.io:443 http://localhost:8545 1000000 1001000
		`),
	}

	cmd.PersistentFlags().Bool("diff", false, "When activated, difference is displayed for each block with a difference")
	cmd.Flags().BoolP("plaintext", "p", false, "Use plaintext connection to Firehose")
	cmd.Flags().BoolP("insecure", "k", false, "Use SSL connection to Firehose but skip SSL certificate validation")

	cmd.Flags().StringP("api-token-env-var", "a", "FIREHOSE_API_TOKEN", "Look for a JWT in this environment variable to authenticate against endpoint")

	return cmd
}

func createCompareBlocksRPCE(logger *zap.Logger) firecore.CommandExecutor {
	return func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		firehoseEndpoint := args[0]
		rpcEndpoint := args[1]
		cli := rpc.NewClient(rpcEndpoint)
		start, err := strconv.ParseInt(args[2], 10, 64)
		if err != nil {
			return fmt.Errorf("parsing start block num: %w", err)
		}
		stop, err := strconv.ParseUint(args[3], 10, 64)
		if err != nil {
			return fmt.Errorf("parsing stop block num: %w", err)
		}
		apiTokenEnvVar := sflags.MustGetString(cmd, "api-token-env-var")
		jwt := os.Getenv(apiTokenEnvVar)

		plaintext := sflags.MustGetBool(cmd, "plaintext")
		insecure := sflags.MustGetBool(cmd, "insecure")

		firehoseClient, connClose, grpcCallOpts, err := client.NewFirehoseClient(firehoseEndpoint, jwt, insecure, plaintext)
		if err != nil {
			return err
		}
		defer connClose()

		grpcCallOpts = append(grpcCallOpts, grpc.UseCompressor(zstd.Name))

		request := &pbfirehose.Request{
			StartBlockNum:   start,
			StopBlockNum:    stop,
			FinalBlocksOnly: true,
		}

		stream, err := firehoseClient.Blocks(ctx, request, grpcCallOpts...)
		if err != nil {
			return fmt.Errorf("unable to start blocks stream: %w", err)
		}

		meta, err := stream.Header()
		if err != nil {
			logger.Warn("cannot read header")
		} else {
			if hosts := meta.Get("hostname"); len(hosts) != 0 {
				logger = logger.With(zap.String("remote_hostname", hosts[0]))
			}
		}
		logger.Info("connected")

		respChan := make(chan *pbeth.Block, 100)

		allDone := make(chan struct{})
		go func() {

			for firehoseBlock := range respChan {
				rpcBlock, err := cli.GetBlockByNumber(ctx, rpc.BlockNumber(firehoseBlock.Number), rpc.WithGetBlockFullTransaction())
				if err != nil {
					panic(err)
				}

				logs, err := cli.Logs(ctx, rpc.LogsParams{
					FromBlock: rpc.BlockNumber(firehoseBlock.Number),
					ToBlock:   rpc.BlockNumber(firehoseBlock.Number),
				})
				if err != nil {
					panic(err)
				}

				identical, diffs := CompareFirehoseToRPC(firehoseBlock, rpcBlock, logs)
				if !identical {
					fmt.Println("different", diffs)
				} else {
					fmt.Println(firehoseBlock.Number, "identical")
				}
			}
			close(allDone)
		}()

		for {
			response, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}
				return fmt.Errorf("stream error while receiving: %w", err)
			}

			ethBlock := &pbeth.Block{}
			if err := anypb.UnmarshalTo(response.Block, ethBlock, proto.UnmarshalOptions{}); err != nil {
				return fmt.Errorf("unmarshalling anypb: %w", err)
			}
			respChan <- ethBlock
		}
		close(respChan)
		<-allDone

		return nil
	}
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
			GasPrice: block.BigIntFromEthUint256(receipts[i].GasPrice),
			Input:    receipts[i].Input.Bytes(),
			Value:    block.BigIntFromEthUint256(receipts[i].Value),
			From:     receipts[i].From.Bytes(),
			Index:    uint32(receipts[i].TransactionIndex),
			Receipt:  &pbeth.TransactionReceipt{
				// Logs: ,            // filled below
				// CumulativeGasUsed: // only available on getTransactionReceipt
				// StateRoot:         // only available on getTransactionReceipt
				// LogsBloom:         // only available on getTransactionReceipt
			},
			V:            pbeth.NewBigInt(int64(receipts[i].V)).Bytes,
			R:            block.BigIntFromEthUint256(receipts[i].R).Bytes,
			S:            block.BigIntFromEthUint256(receipts[i].S).Bytes,
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
					Address:    log.Address.Bytes(),             //[]byte   `protobuf:"bytes,1,opt,name=address,proto3" json:"address,omitempty"`
					Topics:     block.HashesToBytes(log.Topics), //[][]byte `protobuf:"bytes,2,rep,name=topics,proto3" json:"topics,omitempty"`
					Data:       log.Data.Bytes(),                //[]byte   `protobuf:"bytes,3,opt,name=data,proto3" json:"data,omitempty"`
					BlockIndex: currentBlockIndex,               //uint32 `protobuf:"varint,6,opt,name=blockIndex,proto3" json:"blockIndex,omitempty"`
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

// only keep hash
func stripFirehoseUncles(in []*pbeth.BlockHeader) {
	for _, uncle := range in {
		uncle.BaseFeePerGas = nil
		uncle.Coinbase = nil
		uncle.Difficulty = nil
		uncle.ExtraData = nil
		uncle.GasLimit = 0
		uncle.GasUsed = 0
		uncle.LogsBloom = nil
		uncle.MixHash = nil
		uncle.Nonce = 0
		uncle.Number = 0
		uncle.ParentHash = nil
		uncle.ReceiptRoot = nil
		uncle.StateRoot = nil
		uncle.Timestamp = nil
		uncle.TotalDifficulty = nil
		uncle.TransactionsRoot = nil
		uncle.TxDependency = nil
		uncle.UncleHash = nil
		uncle.WithdrawalsRoot = nil
	}
}

func stripFirehoseHeader(in *pbeth.BlockHeader) {
	in.TxDependency = nil
	in.WithdrawalsRoot = nil

	if in.BaseFeePerGas == nil {
		in.BaseFeePerGas = &pbeth.BigInt{}
	}

	if len(in.Difficulty.Bytes) == 1 && in.Difficulty.Bytes[0] == 0x0 {
		in.Difficulty.Bytes = nil
	}
}

func stripFirehoseBlock(in *pbeth.Block, hashesWithoutTo map[string]bool) {
	in.DetailLevel = pbeth.Block_DETAILLEVEL_BASE
	// clean up internal values
	msg := in.ProtoReflect()
	msg.SetUnknown(nil)
	in = msg.Interface().(*pbeth.Block)

	in.Ver = 3
	stripFirehoseHeader(in.Header)
	stripFirehoseUncles(in.Uncles)
	stripFirehoseTransactionTraces(in.TransactionTraces, hashesWithoutTo)

	// ARB-ONE FIX
	if in.Header.TotalDifficulty.Uint64() == 2 { // arb-one-specific
		in.Header.TotalDifficulty = pbeth.NewBigInt(int64(in.Number) - 22207816)
	}

	in.BalanceChanges = nil
	in.CodeChanges = nil
}

func stripFirehoseTransactionTraces(in []*pbeth.TransactionTrace, hashesWithoutTo map[string]bool) {
	idx := uint32(0)
	for _, trace := range in {

		if hashesWithoutTo[eth.Hash(trace.Hash).String()] {
			trace.To = nil // only available on getTransactionReceipt
		}
		trace.BeginOrdinal = 0
		trace.EndOrdinal = 0

		// some transformations may remove 32-bytes-padding, prefixed 0s don't count
		trace.S = bytes.TrimLeft(trace.S, string([]byte{0}))
		trace.R = bytes.TrimLeft(trace.R, string([]byte{0}))

		trace.GasUsed = 0 // only available on getTransactionReceipt
		if trace.GasPrice == nil {
			trace.GasPrice = &pbeth.BigInt{}
		}

		trace.Type = 0 // only available on getTransactionReceipt

		trace.MaxFeePerGas = nil         // not available on RPC
		trace.MaxPriorityFeePerGas = nil // not available on RPC
		trace.ReturnData = nil           // not available on RPC
		trace.PublicKey = nil            // not available on RPC

		trace.Status = 0 // only available on getTransactionReceipt
		stripFirehoseTrxReceipt(trace.Receipt)
		trace.Calls = nil // not available on RPC

		if trace.Value == nil {
			trace.Value = &pbeth.BigInt{}
		}
		idx++
	}
}

func stripFirehoseTrxReceipt(in *pbeth.TransactionReceipt) {
	for _, log := range in.Logs {
		log.Ordinal = 0
		log.Index = 0 // index inside transaction is a pbeth construct, it doesn't exist in RPC interface and we can't reconstruct exactly the same from RPC because the pbeth ones are increased even when a call is reverted.
	}
	in.LogsBloom = nil       // only available on getTransactionReceipt
	in.StateRoot = nil       // only available on getTransactionReceipt
	in.CumulativeGasUsed = 0 // only available on getTransactionReceipt
}

func CompareFirehoseToRPC(fhBlock *pbeth.Block, rpcBlock *rpc.Block, logs []*rpc.LogEntry) (isEqual bool, differences []string) {
	if fhBlock == nil && rpcBlock == nil {
		return true, nil
	}

	rpcAsPBEth, hashesWithoutTo := block.RpcToEthBlock(rpcBlock, logs)
	stripFirehoseBlock(fhBlock, hashesWithoutTo)

	// tweak that new block for comparison
	for _, tx := range rpcAsPBEth.TransactionTraces {
		tx.BeginOrdinal = 0
		tx.EndOrdinal = 0
		tx.S = bytes.TrimLeft(tx.S, string([]byte{0}))
		tx.R = bytes.TrimLeft(tx.R, string([]byte{0}))
		for _, log := range tx.Receipt.Logs {
			log.Ordinal = 0 // that value is firehose-specific, cannot be reconstructed in an identical way from RPC
			log.Index = 0   // that value is firehose-specific, cannot be reconstructed in an identical way from RPC
		}
	}

	if !proto.Equal(fhBlock, rpcAsPBEth) {
		fh, err := rpc.MarshalJSONRPCIndent(fhBlock, "", " ")
		cli.NoError(err, "cannot marshal Firehose block to JSON")
		rpc, err := rpc.MarshalJSONRPCIndent(rpcAsPBEth, "", " ")
		cli.NoError(err, "cannot marshal RPC block to JSON")
		f, err := jd.ReadJsonString(string(fh))
		cli.NoError(err, "cannot read Firehose block JSON")
		r, err := jd.ReadJsonString(string(rpc))
		cli.NoError(err, "cannot read RPC block JSON")
		//		fmt.Println(string(fh))
		//		fmt.Println("RPC")
		//		fmt.Println(string(rpc))

		if diff := r.Diff(f).Render(); diff != "" {
			differences = append(differences, diff)
		}
		return false, differences
	}
	return true, nil
}

func decodeAnyPB(in *anypb.Any) (*pbbstream.Block, error) {
	block := &pbeth.Block{}
	if err := anypb.UnmarshalTo(in, block, proto.UnmarshalOptions{}); err != nil {
		return nil, fmt.Errorf("unmarshal anypb: %w", err)
	}

	// We are downloading only final blocks from the Firehose connection which means the LIB for them
	// can be set to themself (althought we use `- 1` to ensure problem would occur if codde don't like
	// `LIBNum == self.BlockNum`).
	return blockEncoder.Encode(firecore.BlockEnveloppe{Block: block, LIBNum: block.Number - 1})
}
