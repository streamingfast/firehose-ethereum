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

func BlockFromProto(b *pbeth.Block, libNum uint64) (*bstream.Block, error) {
	blockTime, err := b.Time()
	if err != nil {
		return nil, err
	}

	content, err := proto.Marshal(b)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal to binary form: %s", err)
	}

	blk := &bstream.Block{
		Id:             b.ID(),
		Number:         b.Number,
		PreviousId:     b.PreviousID(),
		Timestamp:      blockTime,
		LibNum:         libNum,
		PayloadKind:    pbbstream.Protocol_ETH,
		PayloadVersion: b.Ver,
	}

	return bstream.GetBlockPayloadSetter(blk, content)
}
