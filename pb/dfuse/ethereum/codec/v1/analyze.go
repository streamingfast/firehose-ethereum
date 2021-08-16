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

package pbcodec

import (
	"bytes"
	"encoding/hex"

	"github.com/streamingfast/eth-go"
)

var transferTopic = eth.MustNewHash("ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef")

func (block *Block) Analyze() {
	for _, tr := range block.TransactionTraces {
		for _, call := range tr.Calls {
			for _, log := range call.Logs {
				if !isERC20TransferEvent(log) {
					continue
				}

				transferEvent := &ERC20TransferEvent{
					From:   log.Topics[1][12:],
					To:     log.Topics[2][12:],
					Amount: BigIntFromBytes(log.Data[:32]),
				}

				call.Erc20TransferEvents = append(call.Erc20TransferEvents, transferEvent)

				// Find corresponding state changes, produce BalanceChanges events
				balanceFrom := findERC20BalanceChanges(call, transferEvent.From)
				call.Erc20BalanceChanges = append(call.Erc20BalanceChanges, balanceFrom...)
				balanceTo := findERC20BalanceChanges(call, transferEvent.To)
				call.Erc20BalanceChanges = append(call.Erc20BalanceChanges, balanceTo...)
			}
		}
	}
}

func findERC20BalanceChanges(call *Call, holder []byte) (out []*ERC20BalanceChange) {
	keys := erc20StorageKeysForAddress(call, holder)
	for _, key := range keys {
		byteKey, _ := hex.DecodeString(key)
		for _, chng := range call.StorageChanges {
			if bytes.Compare(chng.Key, byteKey) == 0 {
				n := BigIntFromBytes(chng.NewValue)
				o := BigIntFromBytes(chng.OldValue)

				// delta := big.NewInt(0).Sub(n.Native(), o.Native()) cannot get sign !!!! skip it

				out = append(out, &ERC20BalanceChange{
					HolderAddress: holder,
					OldBalance:    o,
					NewBalance:    n,
				})
			}
		}
	}
	return
}

func callToMethod(call *Call) string {
	if len(call.Input) < 4 {
		return ""
	}
	method := hex.EncodeToString(call.Input[0:4])
	if out, ok := eth.KnownSignatures[method]; ok {
		return out
	}
	return ""
}

func erc20StorageKeysForAddress(call *Call, address []byte) (out []string) {
	addrAsHex := hex.EncodeToString(address)
	for hash, preimage := range call.KeccakPreimages {
		// TODO: make sure its the correct length
		if len(preimage) != 128 {
			continue
		}
		// TODO: we should check its all zeroes expect for 4 hex chars (2 bytes).. so
		// we're sure its a top-level variable or something near that..
		if preimage[64:126] != "00000000000000000000000000000000000000000000000000000000000000" {
			// Second part of the keccak should be a top-level contract variable index.
			continue
		}
		// TODO: The address part of the keccak should match the current address, check that the 24:64 actually matches that in the pre-image.
		if preimage[24:64] == addrAsHex {
			out = append(out, hash)
		}
	}
	return
}

func isERC20TransferEvent(log *Log) bool {
	if len(log.Topics) != 3 || len(log.Data) != 32 {
		return false
	}

	return bytes.Equal(log.Topics[0], transferTopic)
}
