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

package types

import (
	"strings"

	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"

	"github.com/streamingfast/bstream"
	firecore "github.com/streamingfast/firehose-core"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
)

var _ firecore.Block = (*pbeth.Block)(nil)

var encoder = firecore.NewBlockEncoder()

var BlockAcceptedVersions = []int32{1, 2, 3}

// init is kept for backward compatibility, `InitFireCore()` should be called directly instead in your
// own `init()` function.
func init() {
	InitFireCore()
}

// InitFireCore initializes the firehose-core library and override some specific `bstream` element with the proper
// values for the ETH chain.
//
// You should use this method explicitely in your `init()` function to make the dependency explicit.
func InitFireCore() {
	// Doing it in `types` ensure that does that depend only on us are properly initialized
	firecore.UnsafePayloadKind = pbbstream.Protocol_ETH
	bstream.NormalizeBlockID = func(in string) string {
		return strings.TrimPrefix(strings.ToLower(in), "0x")
	}
}

func BlockFromProto(b *pbeth.Block, libNum uint64) (*pbbstream.Block, error) {
	return encoder.Encode(firecore.BlockEnveloppe{Block: b, LIBNum: libNum})
}
