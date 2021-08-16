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
	"github.com/streamingfast/sf-ethereum/trxdb"
	"go.uber.org/zap"
)

type DB struct {
	store store.KVStore

	enc *trxdb.ProtoEncoder
	dec *trxdb.ProtoDecoder

	purgeInterval uint64
	logger        *zap.Logger
}

var dbCachePool = make(map[string]trxdb.DB)
var dbCachePoolLock sync.Mutex

func init() {
	trxdbFactory := func(dsn string) (trxdb.DB, error) {
		return New(dsn)
	}

	trxdb.Register("badger", trxdbFactory)
	trxdb.Register("tikv", trxdbFactory)
	trxdb.Register("bigkv", trxdbFactory)
	trxdb.Register("cznickv", trxdbFactory)
}

type dsnOptions struct {
	reads  []string
	writes []string
}

func New(dsn string) (*DB, error) {
	zlog.Debug("creating kv db", zap.String("dsn", dsn))
	db := &DB{
		enc:    trxdb.NewProtoEncoder(),
		dec:    trxdb.NewProtoDecoder(),
		logger: zap.NewNop(),
	}

	cleanDsn, _, err := parseAndCleanDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("unable to parse and clean kv driver dsn: %w", err)
	}

	kvStore, err := newCachedKVDB(cleanDsn)
	if err != nil {
		return nil, fmt.Errorf("unable retrieve kvdb driver: %w", err)
	}

	db.store = kvStore

	return db, nil
}
