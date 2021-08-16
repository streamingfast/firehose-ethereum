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
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	pbcodec "github.com/streamingfast/sf-ethereum/pb/dfuse/ethereum/codec/v1"
)

type Hash []byte

func (h Hash) String() string {
	return hex.EncodeToString(h)
}

type Uint64 uint64

func (i *Uint64) UnmarshalJSON(data []byte) (err error) {
	if len(data) == 0 {
		return errors.New("empty value")
	}

	var value uint64
	if data[0] == '"' {
		var s string
		if err = json.Unmarshal(data, &s); err != nil {
			return err
		}

		if Has0xPrefix(s) {
			value, err = strconv.ParseUint(SanitizeHex(s), 16, 64)
		} else {
			value, err = strconv.ParseUint(s, 10, 64)
		}
	} else {
		err = json.Unmarshal(data, &value)
	}

	if err != nil {
		return err
	}

	*i = Uint64(value)
	return nil
}

type HexBytes []byte

func (t HexBytes) MarshalJSON() ([]byte, error) {
	return json.Marshal(hex.EncodeToString(t))
}

func (t *HexBytes) UnmarshalJSON(data []byte) (err error) {
	var s string
	err = json.Unmarshal(data, &s)
	if err != nil {
		return
	}

	*t, err = DecodeHex(s)
	return
}

func (t HexBytes) String() string {
	return hex.EncodeToString(t)
}

type BlockHeader struct {
	Hash        HexBytes `json:"hash"`
	ParentHash  HexBytes `json:"parentHash"`
	UncleHash   HexBytes `json:"sha3Uncles"`
	Coinbase    HexBytes `json:"miner"`
	Root        HexBytes `json:"stateRoot"`
	TxHash      HexBytes `json:"transactionsRoot"`
	ReceiptHash HexBytes `json:"receiptsRoot"`
	Bloom       HexBytes `json:"logsBloom"`
	Difficulty  HexBytes `json:"difficulty"`
	Number      Uint64   `json:"number"`
	GasLimit    Uint64   `json:"gasLimit"`
	GasUsed     Uint64   `json:"gasUsed"`
	Time        Uint64   `json:"timestamp"`
	Extra       HexBytes `json:"extraData"`
	MixDigest   HexBytes `json:"mixHash"`
	Nonce       Uint64   `json:"nonce"`
}

type Log struct {
	Address HexBytes   `json:"address"`
	Topics  []HexBytes `json:"topics"`
	Data    HexBytes   `json:"data"`
}

func MustBalanceChangeReasonFromString(reason string) pbcodec.BalanceChange_Reason {
	if reason == "ignored" {
		panic("receive ignored balance change reason, we do not expect this as valid input for block generation")
	}

	// There was a typo at some point, let's accept it still until Geth with typo fix is rolled out
	if reason == "reward_transfaction_fee" {
		return pbcodec.BalanceChange_REASON_REWARD_TRANSACTION_FEE
	}

	enumID := pbcodec.BalanceChange_Reason_value["REASON_"+strings.ToUpper(reason)]
	if enumID == 0 {
		panic(fmt.Errorf("receive unknown balance change reason, received reason string is %q", reason))
	}

	return pbcodec.BalanceChange_Reason(enumID)
}

func MustGasChangeReasonFromString(reason string) pbcodec.GasChange_Reason {
	enumID := pbcodec.GasChange_Reason_value["REASON_"+strings.ToUpper(reason)]
	if enumID == 0 {
		panic(fmt.Errorf("receive unknown gas change reason, received reason string is %q", reason))
	}

	return pbcodec.GasChange_Reason(enumID)
}

func MustGasEventIDFromString(reason string) pbcodec.GasEvent_Id {
	enumID := pbcodec.GasEvent_Id_value["ID_"+strings.ToUpper(reason)]
	if enumID == 0 {
		panic(fmt.Errorf("receive unknown gas event id, received id string is %q", reason))
	}

	return pbcodec.GasEvent_Id(enumID)
}
