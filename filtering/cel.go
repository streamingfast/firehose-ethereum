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
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/interpreter"
	pbeth "github.com/streamingfast/sf-ethereum/types/pb/sf/ethereum/type/v1"
	"go.uber.org/zap"
)

var includeNOOP = &CELFilter{code: "", program: nil, name: "include", valueWhenNoop: true}
var excludeNOOP = &CELFilter{code: "", program: nil, name: "exclude", valueWhenNoop: false}

type CELFilter struct {
	name          string
	code          string
	program       cel.Program
	valueWhenNoop bool
}

func NewCELTrxFilter(includeProgramCode, excludeProgramCode string) (TrxFilter, error) {
	return newCELTrxFilter(includeProgramCode, excludeProgramCode)
}

func newCELTrxFilter(includeProgramCode, excludeProgramCode string) (*celTrxFilter, error) {
	includeFilter, err := newCELFiltersInclude(includeProgramCode)
	if err != nil {
		return nil, fmt.Errorf("include filter: %w", err)
	}

	excludeFilter, err := newCELFiltersExclude(excludeProgramCode)
	if err != nil {
		return nil, fmt.Errorf("exclude filter: %w", err)
	}

	return &celTrxFilter{
		IncludeProgram: includeFilter,
		ExcludeProgram: excludeFilter,
	}, nil
}

func (f *celTrxFilter) Matches(transaction interface{}, cache *TrxFilterCache) (bool, []uint32) {
	var calls []*pbeth.Call
	var trace *pbeth.TransactionTrace
	switch trx := transaction.(type) {
	case *pbeth.Transaction:
		if matchesTrx(trx, f.ExcludeProgram, cache) {
			return false, nil
		}
		return matchesTrx(trx, f.IncludeProgram, cache), nil

	case *pbeth.TransactionTrace:
		if matchesTrace(trx, f.ExcludeProgram, cache) {
			return false, nil
		}
		trace = trx
		calls = trx.Calls
	case *pbeth.TransactionTraceWithBlockRef:
		if matchesTrace(trx.Trace, f.ExcludeProgram, cache) {
			return false, nil
		}
		cache.PurgeStaleCalls(trx.BlockRef.Hash)
		trace = trx.Trace
		calls = trx.Trace.Calls
	}

	var trxMatch bool
	var matchingCalls []uint32
	for i, call := range calls {
		if matchesCall(call, trace, f.IncludeProgram, cache) && !matchesCall(call, trace, f.ExcludeProgram, cache) {
			trxMatch = true
			matchingCalls = append(matchingCalls, uint32(i))
		}
	}
	return trxMatch, matchingCalls

}

func matchesCall(call *pbeth.Call, trace *pbeth.TransactionTrace, filter *CELFilter, cache *TrxFilterCache) bool {
	if filter.IsNoop() {
		return filter.valueWhenNoop
	}
	activation := CallActivation{Trace: trace, Call: call, Cache: cache}
	return filter.match(&activation)
}

func matchesTrx(trx *pbeth.Transaction, filter *CELFilter, cache *TrxFilterCache) bool {
	if filter.IsNoop() {
		return filter.valueWhenNoop
	}
	activation := CallActivation{Trx: trx, Cache: cache}
	return filter.match(&activation)
}

func matchesTrace(trace *pbeth.TransactionTrace, filter *CELFilter, cache *TrxFilterCache) bool {
	if filter.IsNoop() {
		return filter.valueWhenNoop
	}
	activation := CallActivation{Trace: trace, Cache: cache}
	return filter.match(&activation)
}

func (f *CELFilter) IsNoop() bool {
	return f.program == nil
}

func (f *CELFilter) String() string {
	return f.code
}

func newCELFiltersInclude(code string) (*CELFilter, error) {
	return newCELFilter("inclusion", code, []string{"", "true", "*"}, true)
}

func newCELFiltersExclude(code string) (*CELFilter, error) {
	return newCELFilter("exclusion", code, []string{"", "false"}, false)
}

func newCELFilter(name string, code string, noopPrograms []string, valueWhenNoop bool) (*CELFilter, error) {
	stripped := strings.ToLower(strings.TrimSpace(code))
	for _, noopProgram := range noopPrograms {
		if stripped == noopProgram {
			return &CELFilter{
				name:          name,
				code:          stripped,
				valueWhenNoop: valueWhenNoop,
			}, nil
		}
	}

	env, err := cel.NewEnv(
		cel.Declarations(
			decls.NewIdent("to", decls.String, nil),
			decls.NewIdent("nonce", decls.String, nil),
			decls.NewIdent("from", decls.String, nil),
			decls.NewIdent("hash", decls.String, nil),
			decls.NewIdent("erc20_to", decls.String, nil),
			decls.NewIdent("erc20_from", decls.String, nil),

			//decls.NewIdent()("value", decls.Uint), // risk of overflow , nilof uint64
			//decls.NewIdent()("value_gwei", decls., nilUint),
			//decls.NewIdent()("value_wei", decls., nilUint),
			//decls.NewIdent()("value_ether", decls., nilUint),
			// decls.NewIdent()("gas_price", decls., nilUint)
			decls.NewIdent("gas_price_gwei", decls.Int, nil),
			decls.NewIdent("gas_limit", decls.Uint, nil),
			decls.NewIdent("input", decls.String, nil),
			// decls.NewIdent("input_bytes", decls.Bytes),

		),
	)
	if err != nil {
		return nil, fmt.Errorf("new env: %w", err)
	}

	exprAst, issues := env.Compile(stripped)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("parse filter: %w", issues.Err())
	}

	if exprAst.ResultType() != decls.Bool {
		return nil, fmt.Errorf("invalid return type %q", exprAst.ResultType())
	}

	prg, err := env.Program(exprAst)
	if err != nil {
		return nil, fmt.Errorf("program: %w", err)
	}

	return &CELFilter{
		name:          name,
		code:          code,
		program:       prg,
		valueWhenNoop: valueWhenNoop,
	}, nil
}

func (f *CELFilter) match(activation interpreter.Activation) (matched bool) {
	if f.IsNoop() {
		return f.valueWhenNoop
	}

	res, _, err := f.program.Eval(activation)
	if err != nil {
		if tracer.Enabled() {
			zlog.Debug("filter program failed", zap.String("name", f.name), zap.Error(err))
		}
		return f.valueWhenNoop
	}

	retval, valid := res.(types.Bool)
	if !valid {
		zlog.Error("return value of our cel program isn't of type bool, this should never happen since we've checked the return value type already")
		return f.valueWhenNoop
	}

	if tracer.Enabled() {
		zlog.Debug("filter program executed correctly", zap.String("name", f.name), zap.Bool("matched", bool(retval)))
	}

	return bool(retval)
}
