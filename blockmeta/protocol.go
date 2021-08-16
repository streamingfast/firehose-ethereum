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

package blockmeta

import (
	"context"
	"fmt"

	"github.com/streamingfast/blockmeta"
	"github.com/streamingfast/sf-ethereum/trxdb"
	"go.uber.org/zap"
)

var DB trxdb.DB

func init() {
	blockmeta.GetBlockNumFromID = blockNumFromID
}

func blockNumFromID(ctx context.Context, id string) (uint64, error) {
	if DB == nil {
		return 0, fmt.Errorf("DB is not set, cannot retrieve block bum from id")
	}

	blockRef, err := DB.GetBlock(ctx, trxdb.MustHexDecode(id))
	if err != nil {
		zlog.Warn("failed to get block from db. will try from api", zap.Error(err))
		block, err2 := getBlockByID(id, APIs[0])
		if err2 != nil {
			return 0, fmt.Errorf("failed to get block num from both DB and API: %s : %s", err2, err)
		}
		return block.Number, nil
	}
	return blockRef.Block.Number, err
}
