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

// lowerFirst lowers the first character of the string with an exception case
// for strings starting with 2 consecutive upper characters or more which is common
// when a string start with an acronym like `HTTP status code is 200`.
func lowerFirst(s string) string {
	if s == "" {
		return ""
	}

	firstRune, n := utf8.DecodeRuneInString(s)
	if firstRune == utf8.RuneError {
		// String is malformed, let's pass the problem to someone else
		return s
	}

	// Now that we have our first character and offset (n), if there is a following
	// character to this and the following character is upper case, we won't lower
	// case first, it's an exception so that `HTTP` remains `HTTP`.
	afterFirst := s[n:]
	if afterFirst != "" {
		secondRune, _ := utf8.DecodeRuneInString(afterFirst)
		if secondRune == utf8.RuneError {
			// String is malformed, let's pass the problem to someone else
			return s
		}

		if unicode.IsUpper(secondRune) {
			// We have our exception case, two consecutive upper case runes, we keep it untouched
			return s
		}
	}

	return string(unicode.ToLower(firstRune)) + afterFirst
}
