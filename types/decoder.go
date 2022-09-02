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
	"fmt"

	"github.com/streamingfast/bstream"
	pbbstream "github.com/streamingfast/pbgo/sf/bstream/v1"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"google.golang.org/protobuf/proto"
)

func BlockDecoder(blk *bstream.Block) (interface{}, error) {
	if blk.Kind() != pbbstream.Protocol_ETH {
		return nil, fmt.Errorf("expected kind %s, got %s", pbbstream.Protocol_ETH, blk.Kind())
	}

	if blk.Version() != 2 && blk.Version() != 1 {
		return nil, fmt.Errorf("this decoder only knows about version 1 and 2, got %d", blk.Version())
	}

	block := new(pbeth.Block)
	pl, err := blk.Payload.Get()
	if err != nil {
		return nil, fmt.Errorf("unable to get payload: %s", err)
	}

	err = proto.Unmarshal(pl, block)
	if err != nil {
		return nil, fmt.Errorf("unable to decode payload: %s", err)
	}

	return block, nil
}
