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

package nodemanager

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	logplugin "github.com/streamingfast/node-manager/log_plugin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var gethLogLevelRegex = regexp.MustCompile("^(DEBUG|INFO|WARN|ERROR)")

func NewGethToZapLogPlugin(debugDeepMind bool, logger *zap.Logger) *logplugin.ToZapLogPlugin {
	return logplugin.NewToZapLogPlugin(debugDeepMind, logger,
		logplugin.ToZapLogPluginLogLevel(gethLogLevelExtractor),
		logplugin.ToZapLogPluginTransformer(gethLogTransformer),
	)
}

func NewOpenEthereumToZapLogPlugin(debugDeepMind bool, logger *zap.Logger) *logplugin.ToZapLogPlugin {
	// FIXME: This uses Geth version for now, fix this by running our OpenEthereum instrumentation
	// and created new extractor and transformer.
	return NewGethToZapLogPlugin(debugDeepMind, logger)
}

func gethLogLevelExtractor(in string) zapcore.Level {
	if strings.Contains(in, "Upgrade blockchain database version") {
		return zap.InfoLevel
	}

	if strings.Contains(in, "peer connected on snap without compatible eth support") {
		return zap.DebugLevel
	}

	groups := gethLogLevelRegex.FindStringSubmatch(in)
	if len(groups) <= 1 {
		return zap.InfoLevel
	}

	switch groups[1] {
	case "INFO":
		return zap.InfoLevel
	case "WARN":
		return zap.WarnLevel
	case "ERROR":
		return zap.ErrorLevel
	case "DEBUG":
		return zap.DebugLevel
	default:
		return zap.InfoLevel
	}
}

var gethLogTransformRegex = regexp.MustCompile(`^[A-Z]{3,}\s+\[[0-9-_:\|\.]+\]\s+`)

func gethLogTransformer(in string) string {
	return lowerFirst(gethLogTransformRegex.ReplaceAllString(in, ""))
}

func lowerFirst(s string) string {
	if s == "" {
		return ""
	}

	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToLower(r)) + s[n:]
}
