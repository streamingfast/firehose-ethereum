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
	"strings"

	"github.com/spf13/cobra"
	"github.com/streamingfast/cli"
	"go.uber.org/zap"
)

func RegisterCommonFlags(_ *zap.Logger, cmd *cobra.Command) error {
	// this one belong everywhere
	RootCmd.PersistentFlags().Int("common-first-streamable-block", 0, "[COMMON] first streamable block number")

	//Common stores configuration flags
	cmd.Flags().String("common-merged-blocks-store-url", MergedBlocksStoreURL, "[COMMON] Store URL (with prefix) where to read/write merged blocks.")
	cmd.Flags().String("common-one-block-store-url", OneBlockStoreURL, "[COMMON] Store URL (with prefix) to read/write one-block files.")
	cmd.Flags().String("common-forked-blocks-store-url", ForkedBlocksStoreURL, "[COMMON] Store URL (with prefix) to read/write forked block files that we want to keep")
	cmd.Flags().String("common-index-store-url", IndexStoreURL, "[COMMON] Store URL (with prefix) to read/write index files.")
	cmd.Flags().String("common-live-blocks-addr", RelayerServingAddr, "[COMMON] gRPC endpoint to get real-time blocks.")

	cmd.Flags().IntSlice("common-block-index-sizes", []int{100000, 100000, 10000, 1000}, "index bundle sizes that that are considered valid when looking for block indexes")
	cmd.Flags().Bool("common-blocks-cache-enabled", false, FlagDescription(`
				[COMMON] Use a disk cache to store the blocks data to disk and instead of keeping it in RAM. By enabling this, block's Protobuf content, in bytes,
				is kept on file system instead of RAM. This is done as soon the block is downloaded from storage. This is a tradeoff between RAM and Disk, if you
				are going to serve only a handful of concurrent requests, it's suggested to keep is disabled, if you encounter heavy RAM consumption issue, specially
				by the firehose component, it's definitely a good idea to enable it and configure it properly through the other 'common-blocks-cache-...' flags. The cache is
				split in two portions, one keeping N total bytes of blocks of the most recently used blocks and the other one keeping the N earliest blocks as
				requested by the various consumers of the cache.
			`))
	cmd.Flags().String("common-blocks-cache-dir", BlocksCacheDirectory, FlagDescription(`
				[COMMON] Blocks cache directory where all the block's bytes will be cached to disk instead of being kept in RAM.
				This should be a disk that persists across restarts of the Firehose component to reduce the the strain on the disk
				when restarting and streams reconnects. The size of disk must at least big (with a 10%% buffer) in bytes as the sum of flags'
				value for  'common-blocks-cache-max-recent-entry-bytes' and 'common-blocks-cache-max-entry-by-age-bytes'.
			`))
	cmd.Flags().Int("common-blocks-cache-max-recent-entry-bytes", 20*1024^3, FlagDescription(`
				[COMMON] Blocks cache max size in bytes of the most recently used blocks, after the limit is reached, blocks are evicted from the cache.
			`))
	cmd.Flags().Int("common-blocks-cache-max-entry-by-age-bytes", 20*1024^3, FlagDescription(`
				[COMMON] Blocks cache max size in bytes of the earliest used blocks, after the limit is reached, blocks are evicted from the cache.
			`))

	// Network config
	cmd.Flags().Uint32("common-chain-id", DefaultChainID, "[COMMON] ETH chain ID (from EIP-155) as returned from JSON-RPC 'eth_chainId' call Used by: dgraphql")
	cmd.Flags().Uint32("common-network-id", DefaultNetworkID, "[COMMON] ETH network ID as returned from JSON-RPC 'net_version' call")
	cmd.Flags().String("common-deployment-id", DefaultDeploymentID, "[COMMON] Deployment ID, used for some billing functions by dgraphql")

	//// Authentication, metering and rate limiter plugins
	cmd.Flags().String("common-auth-plugin", "null://", "[COMMON] Auth plugin URI, see streamingfast/dauth repository")
	cmd.Flags().String("common-metering-plugin", "null://", "[COMMON] Metering plugin URI, see streamingfast/dmetering repository")
	cmd.Flags().String("common-ratelimiter-plugin", "null://", "[COMMON] Rate Limiter plugin URI, see streamingfast/dauth repository")

	// System Behavior
	cmd.Flags().Duration("common-system-shutdown-signal-delay", 0, "[COMMON] Add a delay between receiving SIGTERM signal and shutting down apps. Apps will respond negatively to /healthz during this period")

	return nil
}

func FlagDescription(in string, args ...interface{}) string {
	return fmt.Sprintf(strings.Join(strings.Split(string(cli.Description(in)), "\n"), " "), args...)
}
