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

package codec

import (
	"github.com/streamingfast/eth-go"
)

type BlockHeader struct {
	Hash             eth.Hash       `json:"hash"`
	ParentHash       eth.Hash       `json:"parentHash"`
	UncleHash        eth.Hash       `json:"sha3Uncles"`
	Coinbase         eth.Address    `json:"miner"`
	Root             eth.Hash       `json:"stateRoot"`
	TxHash           eth.Hash       `json:"transactionsRoot"`
	ReceiptHash      eth.Hash       `json:"receiptsRoot"`
	Bloom            eth.Hex        `json:"logsBloom"`
	Difficulty       eth.Hex        `json:"difficulty"`
	Number           eth.Uint64     `json:"number"`
	GasLimit         eth.Uint64     `json:"gasLimit"`
	GasUsed          eth.Uint64     `json:"gasUsed"`
	Time             eth.Uint64     `json:"timestamp"`
	Extra            eth.Hex        `json:"extraData"`
	MixDigest        eth.Hash       `json:"mixHash"`
	Nonce            eth.Uint64     `json:"nonce"`
	BaseFeePerGas    eth.Hex        `json:"baseFeePerGas"`
	WithdrawalsHash  eth.Hash       `json:"withdrawalsRoot"`
	BlobGasUsed      eth.Uint64     `json:"blobGasUsed"`
	ExcessBlobGas    eth.Uint64     `json:"excessBlobGas"`
	ParentBeaconRoot eth.Hash       `json:"parentBeaconBlockRoot"`
	TxDependency     [][]eth.Uint64 `json:"txDependency"`
}

type Log struct {
	Address eth.Address `json:"address"`
	Topics  []eth.Hash  `json:"topics"`
	Data    eth.Hex     `json:"data"`
}

type endBlockInfo struct {
	Header             *BlockHeader   `json:"header"`
	Uncles             []*BlockHeader `json:"uncles"`
	TotalDifficulty    eth.Hex        `json:"totalDifficulty"`
	FinalizedBlockNum  eth.Uint64     `json:"finalizedBlockNum"`
	FinalizedBlockHash eth.Hash       `json:"finalizedBlockHash"`
}
