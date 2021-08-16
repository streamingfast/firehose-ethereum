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

package trxdbtest

import (
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/streamingfast/sf-ethereum/trxdb"
)

type DriverCleanupFunc func()
type DriverFactory func() (trxdb.DB, DriverCleanupFunc)

func TestAll(t *testing.T, driverName string, driverFactory DriverFactory) {
	TestAllDbWriter(t, driverName, driverFactory)
	TestAllDbReader(t, driverName, driverFactory)
	TestAllTimelineExplorer(t, driverName, driverFactory)
	TestAllTransactionsReader(t, driverName, driverFactory)
}

type testFunc = func(t *testing.T, driverFactory DriverFactory)

// getFunctionName reads the program counter adddress and return the function
// name representing this address.
//
// The `FuncForPC` format is in the form of `github.com/.../.../package.func`.
// As such, we use `filepath.Base` to obtain the `package.func` part and then
// split it at the `.` to extract the function name.
func getFunctionName(i interface{}) string {
	pcIdentifier := runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
	baseName := filepath.Base(pcIdentifier)
	parts := strings.SplitN(baseName, ".", 2)
	if len(parts) <= 1 {
		return parts[0]
	}

	return parts[1]
}
