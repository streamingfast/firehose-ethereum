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
	"io/ioutil"
	"os"
	"testing"

	"github.com/streamingfast/kvdb/store/badger"
	_ "github.com/streamingfast/kvdb/store/badger"
	"github.com/streamingfast/logging"
	_ "github.com/streamingfast/sf-ethereum/codec"
	"github.com/streamingfast/sf-ethereum/trxdb"
	"github.com/streamingfast/sf-ethereum/trxdb/trxdbtest"
	"github.com/stretchr/testify/require"
)

func init() {
	logging.TestingOverride()
}

func TestAll(t *testing.T) {
	trxdbtest.TestAll(t, "kv", newTestDBFactory(t))
}

func newTestDBFactory(t *testing.T) trxdbtest.DriverFactory {
	return func() (trxdb.DB, trxdbtest.DriverCleanupFunc) {
		tmp, err := ioutil.TempDir("", "trxdb")
		require.NoError(t, err)

		db, err := New(fmt.Sprintf("badger://%s/test.db?createTables=true", tmp))
		require.NoError(t, err)

		closer := func() {
			db.store.(*badger.Store).Close()
			os.RemoveAll(tmp)
		}

		return db, closer
	}
}
