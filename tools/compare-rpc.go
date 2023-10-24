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

package tools

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/holiman/uint256"
	jd "github.com/josephburnett/jd/lib"
	"github.com/mostynb/go-grpc-compression/zstd"
	"github.com/spf13/cobra"
	"github.com/streamingfast/cli"
	"github.com/streamingfast/eth-go"
	"github.com/streamingfast/eth-go/rpc"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"github.com/streamingfast/firehose/client"
	pbfirehose "github.com/streamingfast/pbgo/sf/firehose/v2"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

var compareBlocksRPCCmd = &cobra.Command{
	Use:   "compare-blocks-rpc <firehose-endpoint> <rpc-endpoint> <start-block> <stop-block>",
	Short: "Checks for any differences between a Firehose and RPC endpoint (get_block) for a specified range.",
	Long: cli.Dedent(`
		The 'compare-blocks-rpc' takes in a firehose URL, an RPC endpoint URL and inclusive start/stop block numbers.
	`),
	Args: cobra.ExactArgs(4),
	RunE: compareBlocksRPCE,
	Example: ExamplePrefixed("fireeth tools compare-blocks-rpc", `
		# Run over full block range
		mainnet.eth.streamingfast.io:443 http://localhost:8545 1000000 1001000
	`),
}

func init() {
	Cmd.AddCommand(compareBlocksRPCCmd)
	compareBlocksRPCCmd.PersistentFlags().Bool("diff", false, "When activated, difference is displayed for each block with a difference")
	compareBlocksRPCCmd.Flags().BoolP("plaintext", "p", false, "Use plaintext connection to Firehose")
	compareBlocksRPCCmd.Flags().BoolP("insecure", "k", false, "Use SSL connection to Firehose but skip SSL certificate validation")

	compareBlocksRPCCmd.Flags().StringP("api-token-env-var", "a", "FIREHOSE_API_TOKEN", "Look for a JWT in this environment variable to authenticate against endpoint")

	//compareBlocksRPCCmd.PersistentFlags().String("write-rpc-cache", "compared-rpc-blocks.jsonl", "When non-empty, the results of the RPC calls will be appended to this JSONL file")
	//compareBlocksRPCCmd.PersistentFlags().String("read-rpc-cache", "compared-rpc-blocks.jsonl", "When non-empty, this file will be parsed before doing any RPC calls")
}

func compareBlocksRPCE(cmd *cobra.Command, args []string) error {

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
	apiTokenEnvVar := mustGetString(cmd, "api-token-env-var")
	jwt := os.Getenv(apiTokenEnvVar)

	plaintext := mustGetBool(cmd, "plaintext")
	insecure := mustGetBool(cmd, "insecure")

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
		zlog.Warn("cannot read header")
	} else {
		if hosts := meta.Get("hostname"); len(hosts) != 0 {
			zlog = zlog.With(zap.String("remote_hostname", hosts[0]))
		}
	}
	zlog.Info("connected")

	respChan := make(chan *pbeth.Block, 100)

	allDone := make(chan struct{})
	go func() {

		for fhBlock := range respChan {

			rpcBlock, err := cli.GetBlockByNumber(ctx, rpc.BlockNumber(fhBlock.Number), rpc.WithGetBlockFullTransaction())
			if err != nil {
				panic(err)
			}

			logs, err := cli.Logs(ctx, rpc.LogsParams{
				FromBlock: rpc.BlockNumber(fhBlock.Number),
				ToBlock:   rpc.BlockNumber(fhBlock.Number),
			})
			if err != nil {
				panic(err)
			}

			identical, diffs := CompareFirehoseToRPC(fhBlock, rpcBlock, logs)
			if !identical {
				fmt.Println("different", diffs)
			} else {
				fmt.Println(fhBlock.Number, "identical")
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
		blk, err := decodeAnyPB(response.Block)
		if err != nil {
			return fmt.Errorf("error while decoding block: %w", err)
		}
		respChan <- blk.ToProtocol().(*pbeth.Block)
	}
	close(respChan)
	<-allDone

	return nil
}

func bigIntFromEthUint256(in *eth.Uint256) *pbeth.BigInt {
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

func toFirehoseBlock(in *rpc.Block, logs []*rpc.LogEntry) (*pbeth.Block, map[string]bool) {

	trx, hashesWithoutTo := toFirehoseTraces(in.Transactions, logs)

	out := &pbeth.Block{
		Hash:              in.Hash.Bytes(),
		Number:            uint64(in.Number),
		Ver:               3,
		Size:              uint64(in.BlockSize),
		Uncles:            toFirehoseUncles(in.Uncles),
		TransactionTraces: trx,
		Header: &pbeth.BlockHeader{
			ParentHash: in.ParentHash.Bytes(),
			// Coinbase:         nil, // FIXME
			// UncleHash:        nil,
			StateRoot:        in.StateRoot.Bytes(),
			TransactionsRoot: in.TransactionsRoot.Bytes(),
			ReceiptRoot:      in.ReceiptsRoot.Bytes(),
			LogsBloom:        in.LogsBloom.Bytes(),
			Difficulty:       bigIntFromEthUint256(in.Difficulty),
			TotalDifficulty:  bigIntFromEthUint256(in.TotalDifficulty),
			Number:           uint64(in.Number),
			GasLimit:         uint64(in.GasLimit),
			GasUsed:          uint64(in.GasUsed),
			Timestamp:        timestamppb.New(time.Time(in.Timestamp)),
			ExtraData:        in.ExtraData.Bytes(),
			Nonce:            uint64(in.Nonce),
			Hash:             in.Hash.Bytes(),
			MixHash:          in.MixHash.Bytes(),
			BaseFeePerGas:    bigIntFromEthUint256(in.BaseFeePerGas),
			// WithdrawalsRoot: in.WithdrawalsRoot, // FIXME
			// TxDependency: in.TxDependency // FIXME
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

func toFirehoseTraces(in *rpc.BlockTransactions, logs []*rpc.LogEntry) (traces []*pbeth.TransactionTrace, hashesWithoutTo map[string]bool) {

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
			GasPrice: bigIntFromEthUint256(receipts[i].GasPrice),
			Input:    receipts[i].Input.Bytes(),
			Value:    bigIntFromEthUint256(receipts[i].Value),
			From:     receipts[i].From.Bytes(),
			Index:    uint32(receipts[i].TransactionIndex),
			Receipt:  &pbeth.TransactionReceipt{
				// filled next
			},
			V: pbeth.NewBigInt(int64(receipts[i].V)).Bytes,
			//R: bigIntFromEthUint256(receipts[i].R).Bytes,
			//S: bigIntFromEthUint256(receipts[i].S).Bytes,
		}

		for _, log := range logs {
			if eth.Hash(log.TransactionHash).String() == txHash {
				out[i].Receipt.Logs = append(out[i].Receipt.Logs, &pbeth.Log{
					Address:    log.Address.Bytes(),            //[]byte   `protobuf:"bytes,1,opt,name=address,proto3" json:"address,omitempty"`
					Topics:     hashesToBytes(log.Topics),      //[][]byte `protobuf:"bytes,2,rep,name=topics,proto3" json:"topics,omitempty"`
					Data:       log.Data.Bytes(),               //[]byte   `protobuf:"bytes,3,opt,name=data,proto3" json:"data,omitempty"`
					BlockIndex: uint32(log.ToLog().BlockIndex), //uint32 `protobuf:"varint,6,opt,name=blockIndex,proto3" json:"blockIndex,omitempty"`
				})
			}
		}

	}
	return out, hashesWithoutTo
}

func hashesToBytes(in []eth.Hash) [][]byte {
	out := make([][]byte, len(in))
	for i := range in {
		out[i] = in[i].Bytes()
	}
	return out
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
	in.Coinbase = nil
	in.Timestamp = nil
	in.TxDependency = nil
	in.UncleHash = nil
	in.WithdrawalsRoot = nil

	if in.BaseFeePerGas == nil {
		in.BaseFeePerGas = &pbeth.BigInt{}
	}

	if len(in.Difficulty.Bytes) == 1 && in.Difficulty.Bytes[0] == 0x0 {
		in.Difficulty.Bytes = nil
	}
}

func stripFirehoseBlock(in *pbeth.Block, hashesWithoutTo map[string]bool) {
	// clean up internal values
	msg := in.ProtoReflect()
	msg.SetUnknown(nil)
	in = msg.Interface().(*pbeth.Block)

	in.Ver = 0
	stripFirehoseHeader(in.Header)
	stripFirehoseUncles(in.Uncles)
	stripFirehoseTransactionTraces(in.TransactionTraces, hashesWithoutTo)

	// ARB-ONE FIX
	if in.Header.TotalDifficulty.Uint64() == 2 {
		in.Header.TotalDifficulty = pbeth.NewBigInt(int64(in.Number) - 22207816)
	}

	// FIXME temp
	in.BalanceChanges = nil
	in.CodeChanges = nil
}

func stripFirehoseTransactionTraces(in []*pbeth.TransactionTrace, hashesWithoutTo map[string]bool) {
	idx := uint32(0)
	for _, trace := range in {

		if hashesWithoutTo[eth.Hash(trace.Hash).String()] {
			trace.To = nil // FIXME: we could compute this from nonce+address
		}

		trace.BeginOrdinal = 0
		trace.EndOrdinal = 0
		trace.AccessList = nil

		trace.GasUsed = 0 // FIXME receipt?

		if trace.GasPrice == nil {
			trace.GasPrice = &pbeth.BigInt{}
		}

		// FIXME ...
		trace.R = nil
		trace.S = nil

		trace.Type = 0
		trace.AccessList = nil
		trace.MaxFeePerGas = nil
		trace.MaxPriorityFeePerGas = nil

		trace.ReturnData = nil
		trace.PublicKey = nil

		trace.Status = 0
		stripFirehoseTrxReceipt(trace.Receipt)
		trace.Calls = nil

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
	in.LogsBloom = nil
	in.StateRoot = nil
	in.CumulativeGasUsed = 0
}

func CompareFirehoseToRPC(fhBlock *pbeth.Block, rpcBlock *rpc.Block, logs []*rpc.LogEntry) (isEqual bool, differences []string) {
	if fhBlock == nil && rpcBlock == nil {
		return true, nil
	}

	rpcAsPBEth, hashesWithoutTo := toFirehoseBlock(rpcBlock, logs)
	stripFirehoseBlock(fhBlock, hashesWithoutTo)

	if !proto.Equal(fhBlock, rpcAsPBEth) {
		fh, err := rpc.MarshalJSONRPCIndent(fhBlock, "", " ")
		mustNoError(err)
		rpc, err := rpc.MarshalJSONRPCIndent(rpcAsPBEth, "", " ")
		mustNoError(err)
		f, err := jd.ReadJsonString(string(fh))
		mustNoError(err)
		r, err := jd.ReadJsonString(string(rpc))
		mustNoError(err)
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
