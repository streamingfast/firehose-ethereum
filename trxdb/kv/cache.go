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
	"fmt"
	"sync"

	"github.com/streamingfast/kvdb/store"
	"go.uber.org/zap"
)

var storeCachePool = make(map[string]store.KVStore)
var storeCachePoolLock sync.Mutex

func newCachedKVDB(dsn string) (out store.KVStore, err error) {
	storeCachePoolLock.Lock()
	defer storeCachePoolLock.Unlock()

	out = storeCachePool[dsn]
	if out == nil {
		zlog.Debug("kv store store is not cached for this DSN, creating a new one",
			zap.String("dsn", dsn),
		)
		out, err = store.New(dsn)
		if err != nil {
			return nil, fmt.Errorf("new kvdb store: %w", err)
		}
		storeCachePool[dsn] = out
	} else {
		zlog.Debug("re-using cached kv store",
			zap.String("dsn", dsn),
		)
	}
	return out, nil
}
