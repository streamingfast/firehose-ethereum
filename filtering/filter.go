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
	"sync"
)

type TrxFilter interface {
	fmt.Stringer
	Matches(transaction interface{}, cache *TrxFilterCache) (bool, []uint32)
}

type celTrxFilter struct {
	IncludeProgram *CELFilter
	ExcludeProgram *CELFilter
}

type cachedItems struct {
	to         *string
	from       *string
	erc20To    *string
	erc20From  *string
	erc20Found *bool
}

func NewTrxFilterCache() *TrxFilterCache {
	return &TrxFilterCache{
		calls: make(map[uint32]*cachedItems),
	}
}

type TrxFilterCache struct {
	sync.Mutex
	cachedItems
	calls        map[uint32]*cachedItems
	refBlockHash []byte
}

func (c *TrxFilterCache) PurgeStaleCalls(trxBlockHash []byte) {
	if !bytes.Equal(trxBlockHash, c.refBlockHash) {
		c.refBlockHash = trxBlockHash
		c.calls = make(map[uint32]*cachedItems)
	}
}

func (c *TrxFilterCache) getCall(idx uint32) *cachedItems {
	if call, ok := c.calls[idx]; ok {
		return call
	}
	ci := &cachedItems{}
	c.calls[idx] = ci
	return ci
}
