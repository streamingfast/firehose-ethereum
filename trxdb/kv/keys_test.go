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

package kv

import (
	"encoding/hex"
	"testing"
	"time"

	ct "github.com/streamingfast/sf-ethereum/codec/testing"
	"github.com/stretchr/testify/require"
)

func TestKeyer_PackBlocksKey(t *testing.T) {
	key := "00000002aa"
	packed := Keys.PackBlocksKey(ct.Hash(key).Bytes(t))
	unpacked := Keys.UnpackBlocksKey(packed)
	require.Equal(t, key, hex.EncodeToString(unpacked))
}

func TestKeyer_PackTimelineKey(t *testing.T) {
	expectedBlockID := "00000002aa"
	expectedBlockTime := time.Unix(0, 0).UTC()

	packed := Keys.PackTimelineKey(true, expectedBlockTime, expectedBlockID)
	blockTime, blockID := Keys.UnpackTimelineKey(true, packed)
	require.Equal(t, expectedBlockID, blockID)
	require.Equal(t, expectedBlockTime, blockTime)
}
