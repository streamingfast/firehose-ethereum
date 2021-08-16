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
	"net/url"
	"strings"

	"github.com/streamingfast/kvdb/store"
	"go.uber.org/zap"
)

func parseAndCleanDSN(dsn string) (cleanDsn string, opt *dsnOptions, err error) {
	zlog.Debug("parsing DSN", zap.String("dsn", dsn))

	dsnOptions := &dsnOptions{
		reads:  []string{},
		writes: []string{},
	}
	dsnURL, err := url.Parse(dsn)
	if err != nil {
		err = fmt.Errorf("invalid dsn: %w", err)
		return
	}

	query, err := url.ParseQuery(dsnURL.RawQuery)
	if err != nil {
		err = fmt.Errorf("invalid query: %w", err)
		return
	}

	for _, readValues := range query["read"] {
		dsnOptions.reads = append(dsnOptions.reads, strings.Split(readValues, ",")...)
	}

	if len(dsnOptions.reads) == 0 {
		dsnOptions.reads = append(dsnOptions.reads, "all")
	}

	for _, writeValues := range query["write"] {
		dsnOptions.writes = append(dsnOptions.writes, strings.Split(writeValues, ",")...)
	}

	if len(dsnOptions.writes) == 0 {
		dsnOptions.writes = append(dsnOptions.writes, "all")
	}

	cleanDsn, err = store.RemoveDSNOptions(dsn, "read", "write", "blk_marker")
	if err != nil {
		err = fmt.Errorf("Unable to clean dsn: %w", err)
	}

	return cleanDsn, dsnOptions, nil
}
