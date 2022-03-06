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

package filtering

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/google/cel-go/interpreter"
	"github.com/streamingfast/eth-go"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/codec/v1"
	"go.uber.org/zap"
)

var (
	transferSignatureBytes     = []byte{0xa9, 0x05, 0x9c, 0xbb}
	transferFromSignatureBytes = []byte{0x23, 0xb8, 0x72, 0xdd}
)

var aGWEI = big.NewInt(1e9)

// CallActivation is used public in some private code.
//
// *Warning* We reserve the rights to change it's signature at all time
type CallActivation struct {
	Trace *pbcodec.TransactionTrace
	Trx   *pbcodec.Transaction
	Call  *pbcodec.Call
	Cache *TrxFilterCache
}

func (a *CallActivation) GetTo() string {
	// from call
	if a.Call != nil {
		cc := a.Cache.getCall(a.Call.Index)
		if cc.to != nil {
			return *cc.to
		}
		to := eth.Address(a.Call.Address).Pretty()
		cc.to = &to
		return to
	}

	// from trx cache
	if a.Cache.to != nil {
		return *a.Cache.to
	}

	// from trace or trx
	var to string
	switch {
	case a.Trace != nil:
		to = eth.Address(a.Trace.To).Pretty()
	case a.Trx != nil:
		to = eth.Address(a.Trx.To).Pretty()
	}

	// save trx cache
	a.Cache.to = &to
	return to
}

func (a *CallActivation) ResolveName(name string) (interface{}, bool) {
	if tracer.Enabled() {
		zlog.Debug("trying to resolve activation name", zap.String("name", name))
	}
	if a.Cache == nil {
		a.Cache = NewTrxFilterCache() // failsafe if someone didn't provide any
	}

	a.Cache.Lock()
	defer a.Cache.Unlock()

	// a.cache.purgeStaleCalls(a.blockNum) FIXME: need block num when we have it...

	switch name {
	case "to":
		return a.GetTo(), true

	case "nonce":
		switch {
		case a.Trace != nil:
			return a.Trace.Nonce, true
		case a.Trx != nil:
			return a.Trx.Nonce, true
		}

	case "from":
		return a.GetFrom(), true

	case "hash":
		switch {
		case a.Trace != nil:
			return "0x" + eth.Hash(a.Trace.Hash).String(), true
		case a.Trx != nil:
			return "0x" + eth.Hash(a.Trx.Hash).String(), true
		}

	case "gas_price_gwei":
		switch {
		case a.Trace != nil:
			return int64(new(big.Int).Quo(a.Trace.GasPrice.Native(), aGWEI).Uint64()), true
		case a.Trx != nil:
			return int64(new(big.Int).Quo(a.Trx.GasPrice.Native(), aGWEI).Uint64()), true
		}

	case "gas_limit":
		switch {
		case a.Trace != nil:
			return a.Trace.GasLimit, true
		case a.Trx != nil:
			return a.Trx.GasLimit, true
		}

	case "input":
		return a.GetInput(), true

	case "erc20_from":
		from, _, isERC20 := a.GetERC20Actors()
		if !isERC20 {
			return "", true
		}

		return from, true

	case "erc20_to":
		_, to, isERC20 := a.GetERC20Actors()
		if !isERC20 {
			return "", true
		}

		return to, true
	}

	return nil, false
}

func (a *CallActivation) GetRawInput() []byte {
	switch {
	case a.Call != nil:
		return a.Call.Input
	case a.Trace != nil:
		return a.Trace.Input
	case a.Trx != nil:
		return a.Trx.Input
	}
	return nil
}

func (a *CallActivation) GetInput() string {
	raw := a.GetRawInput()
	if len(raw) > 0 {
		return "0x" + eth.Hash(raw).String()
	}

	return ""
}

func (a *CallActivation) GetFrom() string {
	if a.Call != nil {
		cc := a.Cache.getCall(a.Call.Index)
		if cc.from != nil {
			return *cc.from
		}
		from := eth.Address(a.Call.Caller).Pretty()
		cc.from = &from
		return from
	}

	if a.Cache.from != nil {
		return *a.Cache.from
	}

	// from trace or trx
	var from string
	switch {
	case a.Trace != nil:
		from = eth.Address(a.Trace.From).Pretty()
	case a.Trx != nil:
		from = eth.Address(a.Trx.From).Pretty()
	}

	// save trx cache
	a.Cache.from = &from
	return from
}

func (a *CallActivation) GetERC20From() (string, bool) {
	if a.Cache.erc20Found != nil {
		return *a.Cache.erc20From, *a.Cache.erc20Found
	}
	from, _, found := a.GetERC20Actors()
	return from, found
}
func (a *CallActivation) GetERC20To() (string, bool) {
	if a.Cache.erc20Found != nil {
		return *a.Cache.erc20To, *a.Cache.erc20Found
	}
	_, to, found := a.GetERC20Actors()
	return to, found
}

func (a *CallActivation) GetERC20Actors() (erc20from, erc20to string, found bool) {
	if a.Cache.erc20Found != nil {
		return *a.Cache.erc20From, *a.Cache.erc20To, *a.Cache.erc20Found
	}

	input := a.GetRawInput()
	if len(input) < 4 {
		return "", "", false
	}

	signature := input[0:4]

	switch {
	case bytes.Equal(signature, transferSignatureBytes): // transfer(address to, uint256 value)
		if len(input) != 4+32+32 {
			break
		}

		offset := 4 + 12
		end := offset + 20
		address := input[offset:end]

		// We are in an ERC20 `transfer(to, quantity)` call, the `from` is the one in the transaction and/or call
		erc20from = a.GetFrom()
		erc20to = eth.Address(address).Pretty()
		found = true

	case bytes.Equal(signature, transferFromSignatureBytes): // transferFrom(address from, address to, uint256 value)
		if len(input) != 4+32+32+32 {
			break
		}

		offset := 4 + 12
		end := offset + 20
		from := input[offset:end]

		offset = 4 + 32 + 12
		end = offset + 20
		to := input[offset:end]

		erc20from = eth.Address(from).Pretty()
		erc20to = eth.Address(to).Pretty()
		found = true
	}
	a.Cache.erc20From = &erc20from
	a.Cache.erc20To = &erc20to
	a.Cache.erc20Found = &found
	return erc20from, erc20to, found
}

func (a *CallActivation) Parent() interpreter.Activation { return nil }

func (f *celTrxFilter) String() string {
	return fmt.Sprintf("[include: %s, exclude: %s]", f.IncludeProgram.String(), f.ExcludeProgram.String())
}
