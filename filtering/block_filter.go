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

package filtering

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/streamingfast/bstream"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/codec/v1"
	"go.uber.org/zap"
)

type FilteringPreprocessor struct {
	Filter *BlockFilter
}

func (f *FilteringPreprocessor) PreprocessBlock(blk *bstream.Block) (interface{}, error) {
	return f.Filter.Transform(blk), nil
}

type BlockFilter struct {
	IncludeProgram blocknumBasedCELFilter
	ExcludeProgram blocknumBasedCELFilter
}

func NewBlockFilter(includeProgramCode, excludeProgramCode []string) (*BlockFilter, error) {
	includeFilter, err := newBlockCELFiltersInclude(includeProgramCode)
	if err != nil {
		return nil, fmt.Errorf("include filter: %w", err)
	}

	excludeFilter, err := newBlockCELFiltersExclude(excludeProgramCode)
	if err != nil {
		return nil, fmt.Errorf("exclude filter: %w", err)
	}

	return &BlockFilter{
		IncludeProgram: includeFilter,
		ExcludeProgram: excludeFilter,
	}, nil
}

func (f *BlockFilter) Transform(blk *bstream.Block) *bstream.Block {
	include := f.IncludeProgram.choose(blk.Number)
	exclude := f.ExcludeProgram.choose(blk.Number)

	// Don't decode the bstream block at all so we save a costly unpacking when both filters are no-op filters
	if include.IsNoop() && exclude.IsNoop() {
		return nil
	}

	clone := blk.Clone()
	block := clone.ToNative().(*pbcodec.Block)

	transformInPlaceV2(block, include, exclude)
	return clone
}

func transformInPlaceV2(block *pbcodec.Block, include, exclude *CELFilter) {
	wasFiltered := block.FilteringApplied

	block.FilteringApplied = true
	block.FilteringIncludeFilterExpr = combineFilters(block.FilteringIncludeFilterExpr, include)
	block.FilteringExcludeFilterExpr = combineFilters(block.FilteringExcludeFilterExpr, exclude)

	var filteredTrxTrace []*pbcodec.TransactionTrace
	filteredTotalCallCount := uint32(0)

	trxTraces := block.TransactionTraces
	for _, trxTrace := range trxTraces {
		trxTraceAddedToFiltered := false

		for _, call := range trxTrace.Calls {
			if wasFiltered {
				// For now, multiple filter is only additive, so if the call already matched, it must continue to match, if it's
				// not the case, it would mean the second filter would not matched this call. At the same time, if we filter out
				// at the "system" level certain calls,
				if call.FilteringMatched {
					continue
				}
			}

			passes := shouldProcess(trxTrace, call, include, exclude)
			if !passes {
				continue
			}

			call.FilteringMatched = true
			filteredTotalCallCount++

			if !trxTraceAddedToFiltered {
				filteredTrxTrace = append(filteredTrxTrace, trxTrace)
				trxTraceAddedToFiltered = true
			}
		}
	}

	block.TransactionTraces = filteredTrxTrace
	zlog.Debug("filtered transaction traces", zap.Uint32("filtered_call_count", filteredTotalCallCount), zap.Int("filtered_transaction_trace_count", len(filteredTrxTrace)))
}

func shouldProcess(trxTrace *pbcodec.TransactionTrace, call *pbcodec.Call, include, exclude *CELFilter) (pass bool) {
	activation := &CallActivation{trxTrace, nil, call, nil}
	// If the include program does not match, there is nothing more to do here
	if !include.match(activation) {
		return false
	}

	// At this point, the inclusion expr matched, let's check it was included but should be now excluded based on the exclusion filter
	if exclude.match(activation) {
		return false
	}

	// We are included and NOT excluded, this transaction trace/action trace match the block filter
	return true
}

type blocknumBasedCELFilter map[uint64]*CELFilter

func (bbcf blocknumBasedCELFilter) String() (out string) {
	if len(bbcf) == 1 {
		for _, v := range bbcf {
			return v.code
		}
	}
	var arr []uint64
	for k := range bbcf {
		arr = append(arr, k)
	}
	sort.Slice(arr, func(i int, j int) bool { return arr[i] < arr[j] })
	for _, k := range arr {
		out += fmt.Sprintf("#%d;%s", k, bbcf[k].code)
	}
	return
}

func (bbcf blocknumBasedCELFilter) choose(blknum uint64) (out *CELFilter) {
	var highestMatchingKey uint64
	for k, v := range bbcf {
		if blknum >= k && k >= highestMatchingKey {
			highestMatchingKey = k
			out = v
		}
	}
	return
}

func newBlockCELFiltersInclude(codes []string) (blocknumBasedCELFilter, error) {
	return newCELFilters("inclusion", codes, []string{"", "true", "*"}, true)
}

func newBlockCELFiltersExclude(codes []string) (blocknumBasedCELFilter, error) {
	return newCELFilters("exclusion", codes, []string{"", "false"}, false)
}

func newCELFilters(name string, codes []string, noopPrograms []string, valueWhenNoop bool) (filtersMap blocknumBasedCELFilter, err error) {
	filtersMap = make(map[uint64]*CELFilter)
	for _, code := range codes {
		parsedCode, blockNum, err := parseBlocknumBasedCode(code)
		if err != nil {
			return nil, err
		}

		filter, err := newCELFilter(name, parsedCode, noopPrograms, valueWhenNoop)
		if err != nil {
			return nil, err
		}
		if _, ok := filtersMap[blockNum]; ok {
			return nil, fmt.Errorf("blocknum %d declared twice in filter", blockNum)
		}

		filtersMap[blockNum] = filter
	}
	if _, ok := filtersMap[0]; !ok { // create noop filtermap
		filtersMap[0] = &CELFilter{
			name:          name,
			code:          "",
			valueWhenNoop: valueWhenNoop,
		}
	}

	return
}

func combineFilters(prev string, next *CELFilter) string {
	if prev == "" {
		return next.code
	}
	if next.IsNoop() {
		return prev
	}
	return fmt.Sprintf("%s;;;%s", prev, next.code)
}

func filterExprContains(appliedFilters, newFilter string) bool {
	if newFilter == "" {
		return true
	}

	applied := strings.Split(appliedFilters, ";;;")
	for _, x := range applied {
		if newFilter == x {
			return true
		}
	}
	return false
}

func parseBlocknumBasedCode(code string) (out string, blocknum uint64, err error) {
	parts := strings.SplitN(code, ";", 2)
	if len(parts) == 1 {
		return parts[0], 0, nil
	}
	if !strings.HasPrefix(parts[0], "#") {
		return "", 0, fmt.Errorf("invalid block num part")
	}
	blocknum, err = strconv.ParseUint(strings.TrimLeft(parts[0], "#"), 10, 64)
	out = strings.Trim(parts[1], " ")
	return
}
