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

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lithammer/dedent"
	"github.com/logrusorgru/aurora"
	"github.com/spf13/viper"
	"github.com/streamingfast/dstore"
	"github.com/streamingfast/logging"
	"go.uber.org/zap"
)

var DefaultLevelInfo = logging.LoggerDefaultLevel(zap.InfoLevel)

// MustReplaceDataDir is used in sf-ethereum-priv
func MustReplaceDataDir(dataDir, in string) string {
	d, err := filepath.Abs(dataDir)
	if err != nil {
		panic(fmt.Errorf("file path abs: %w", err))
	}

	in = strings.Replace(in, "{sf-data-dir}", d, -1)
	return in
}

var commonStoresCreated bool
var indexStoreCreated bool

func GetCommonStoresURLs(dataDir string) (mergedBlocksStoreURL, oneBlocksStoreURL, forkedBlocksStoreURL string, err error) {
	mergedBlocksStoreURL = MustReplaceDataDir(dataDir, viper.GetString("common-merged-blocks-store-url"))
	oneBlocksStoreURL = MustReplaceDataDir(dataDir, viper.GetString("common-one-block-store-url"))
	forkedBlocksStoreURL = MustReplaceDataDir(dataDir, viper.GetString("common-forked-blocks-store-url"))

	if commonStoresCreated {
		return
	}

	if err = mkdirStorePathIfLocal(forkedBlocksStoreURL); err != nil {
		return
	}
	if err = mkdirStorePathIfLocal(oneBlocksStoreURL); err != nil {
		return
	}
	if err = mkdirStorePathIfLocal(mergedBlocksStoreURL); err != nil {
		return
	}
	commonStoresCreated = true
	return
}

func GetIndexStore(dataDir string) (indexStore dstore.Store, possibleIndexSizes []uint64, err error) {
	indexStoreURL := MustReplaceDataDir(dataDir, viper.GetString("common-index-store-url"))

	if indexStoreURL != "" {
		s, err := dstore.NewStore(indexStoreURL, "", "", false)
		if err != nil {
			return nil, nil, fmt.Errorf("couldn't create indexStore: %w", err)
		}
		if !indexStoreCreated {
			if err = mkdirStorePathIfLocal(indexStoreURL); err != nil {
				return nil, nil, err
			}
		}
		indexStoreCreated = true
		indexStore = s
	}

	for _, size := range viper.GetIntSlice("common-block-index-sizes") {
		if size < 0 {
			return nil, nil, fmt.Errorf("invalid negative size for common-block-index-sizes: %d", size)
		}
		possibleIndexSizes = append(possibleIndexSizes, uint64(size))
	}
	return
}

func mkdirStorePathIfLocal(storeURL string) (err error) {
	if dirs := getDirsToMake(storeURL); len(dirs) > 0 {
		zlog.Debug("creating directory and its parent(s)", zap.String("directory", storeURL))
		err = makeDirs(dirs)
	}
	return
}

func getDirsToMake(storeURL string) []string {
	parts := strings.Split(storeURL, "://")
	if len(parts) > 1 {
		if parts[0] != "file" {
			// Not a local store, nothing to do
			return nil
		}
		storeURL = parts[1]
	}

	// Some of the store URL are actually a file directly, let's try our best to cope for that case
	filename := filepath.Base(storeURL)
	if strings.Contains(filename, ".") {
		storeURL = filepath.Dir(storeURL)
	}

	// If we reach here, it's a local store path
	return []string{storeURL}
}

func makeDirs(directories []string) error {
	for _, directory := range directories {
		err := os.MkdirAll(directory, 0755)
		if err != nil {
			return fmt.Errorf("failed to create directory %q: %w", directory, err)
		}
	}

	return nil
}

func dfuseAbsoluteDataDir() (string, error) {
	return filepath.Abs(viper.GetString("global-data-dir"))
}

//var gethVersionRegexp = regexp.MustCompile("Version: ([0-9]+)\\.([0-9]+)\\.([0-9]+)(-(.*))?")
//var deepMindFlagRegexp = regexp.MustCompile(regexp.QuoteMeta("--firehose-deep-mind"))
//
//type gethVersion struct {
//	full string
//
//	major  int
//	minor  int
//	patch  int
//	suffix string
//
//	hasDeepMind bool
//}
//
//// NewGethVersionFromSystem runs the `geth` binary found in `PATH` enviornment
//// variable and extract the version from it.
//func newGethVersionFromSystem() (out gethVersion, err error) {
//	cmd := exec.Command(viper.GetString("global-node-path"), "version")
//	versionStdout, err := cmd.Output()
//	if err != nil {
//		err = fmt.Errorf("unable to run command %q: %w", cmd.String(), err)
//		return
//	}
//
//	cmd = exec.Command(viper.GetString("global-node-path"), "--help")
//	helpStdout, err := cmd.Output()
//	if err != nil {
//		err = fmt.Errorf("unable to run command %q: %w", cmd.String(), err)
//		return
//	}
//
//	return newGethVersionFromString(string(versionStdout), string(helpStdout))
//}
//
//// NewGethVersionFromString parsed the received string and return a structured object
//// representing the version information.
//func newGethVersionFromString(version string, help string) (out gethVersion, err error) {
//	matches := gethVersionRegexp.FindAllStringSubmatch(version, -1)
//	if len(matches) == 0 {
//		err = fmt.Errorf("unable to parse version %q, expected to match %s", version, gethVersionRegexp)
//		return
//	}
//
//	zlog.Debug("geth version regexp matched", zap.Reflect("matches", matches))
//
//	// We don't care for multiple matches for now
//	match := matches[0]
//
//	// We skip the errors since the regex match only digits on those groups
//	out.major, _ = strconv.Atoi(match[1])
//	out.minor, _ = strconv.Atoi(match[2])
//	out.patch, _ = strconv.Atoi(match[3])
//
//	if len(match) >= 5 {
//		out.suffix = match[5]
//	}
//
//	out.full = fmt.Sprintf("%d.%d.%d", out.major, out.minor, out.patch)
//	if out.suffix != "" {
//		out.full += "-" + out.suffix
//	}
//
//	matches = deepMindFlagRegexp.FindAllStringSubmatch(help, -1)
//	out.hasDeepMind = len(matches) > 0
//
//	return
//}
//
//func (v gethVersion) String() string {
//	return v.full
//}
//
//func (v gethVersion) supportsDeepMind(deepMindMajor int) bool {
//	// FIXME: We have not implemented version checking yet
//	return v.hasDeepMind
//}
//
//func checkGethVersionOrExit() {
//
//	version, err := newGethVersionFromSystem()
//	if err != nil {
//		zlog.Debug("unable to extract geth version from system", zap.Error(err))
//		cliErrorAndExit(dedentf(`
//			We were unable to detect "geth" version on your system. This can be due to
//			one of the following reasons:
//			- You don't have "geth" installed on your system
//			- It's installed but no referred by your PATH environment variable, so we did not find it
//			- It's installed but execution of "geth version" or "geth --help" failed
//
//			Make sure you have a Firehose instrumented 'geth' binary, follow instructions
//			at https://github.com/streamingfast/sf-ethereum/blob/develop/DEPENDENCIES.md#firehose-instrumented-ethereum-prebuilt-binaries
//			to find how to install it.
//
//			If you have your Firehose instrumented 'geth' binary outside your PATH, use --geth-path=<location>
//			argument to specify path to it.
//
//			If you think this is a mistake, you can re-run this command adding --skip-checks, which
//			will not perform this check.
//		`))
//	}
//
//	if !version.supportsDeepMind(12) {
//		cliErrorAndExit(dedentf(`
//			The "geth" binary found on your system with version %s does not seem to be a Firehose
//			instrumented binary. Maybe your Firehose instrumented 'geth' binary is not in your
//			PATH environment variable?
//
//			Make sure you have a Firehose instrumented 'geth' binary, follow instructions
//			at https://github.com/streamingfast/sf-ethereum/blob/develop/DEPENDENCIES.md#firehose-instrumented-ethereum-prebuilt-binaries
//			to find how to install it.
//
//			If you have your Firehose instrumented 'geth' binary outside your PATH, use --geth-path=<location>
//			argument to specify path to it.
//
//			If you think this is a mistake, you can re-run this command adding --skip-checks, which
//			will not perform this check.
//		`, version))
//	}
//}

func cliErrorAndExit(message string) {
	fmt.Println(aurora.Red(message).String())
	os.Exit(1)
}

func dedentf(format string, args ...interface{}) string {
	return fmt.Sprintf(dedent.Dedent(strings.TrimPrefix(format, "\n")), args...)
}
