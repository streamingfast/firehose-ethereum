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

func (t *TransactionTrace) ToTransaction() *Transaction {
	return &Transaction{
		To:       t.To,
		Nonce:    t.Nonce,
		GasPrice: t.GasPrice,
		GasLimit: t.GasLimit,
		Value:    t.Value,
		Input:    t.Input,
		V:        t.V,
		R:        t.R,
		S:        t.S,
		Hash:     t.Hash,
		From:     t.From,
	}
}
