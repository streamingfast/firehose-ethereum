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
	kvdbstore "github.com/streamingfast/kvdb/store"
	"go.uber.org/zap"
)

func (db *DB) SetLogger(logger *zap.Logger) error {
	db.logger = logger
	db.logger.Debug("db is now using custom logger")
	return nil
}

func (db *DB) SetPurgeableStore(ttl, purgeInterval uint64) error {
	if traceEnabled {
		zlog.Debug("applying purge interval")
	}

	if db.store != nil {
		db.store = kvdbstore.NewPurgeableStore([]byte{TblTTL}, db.store, ttl)
	}

	db.purgeInterval = purgeInterval
	return nil
}
