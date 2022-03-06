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
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func RegisterCommonFlags(_ *zap.Logger, cmd *cobra.Command) error {
	//Common stores configuration flags
	cmd.Flags().String("common-blocks-store-url", MergedBlocksStoreURL, "[COMMON] Store URL (with prefix) where to read/write merged blocks.")
	cmd.Flags().String("common-oneblock-store-url", OneBlockStoreURL, "[COMMON] Store URL (with prefix) to read/write one-block files.")
	cmd.Flags().String("common-blockstream-addr", RelayerServingAddr, "[COMMON] gRPC endpoint to get real-time blocks.")

	cmd.Flags().Bool("common-atm-cache-enabled", false, "[COMMON] enable ATM caching")
	cmd.Flags().String("common-atm-cache-dir", ATMDirectory, "[COMMON] ATM cache file directory.")
	cmd.Flags().Int("common-atm-max-recent-entry-bytes", 20*1024^3, "[COMMON] ATM cache max size in bytes of recent entry heap")
	cmd.Flags().Int("common-atm-max-entry-by-age-bytes", 20*1024^3, "[COMMON] ATM cache max size in bytes of age entry heap")

	// Network config
	cmd.Flags().Uint32("common-chain-id", DefaultChainID, "[COMMON] ETH chain ID (from EIP-155) as returned from JSON-RPC 'eth_chainId' call Used by: dgraphql")
	cmd.Flags().Uint32("common-network-id", DefaultNetworkID, "[COMMON] ETH network ID as returned from JSON-RPC 'net_version' call. Used by: miner-geth-node, mindreader-geth-node, mindreader-openeth-node, peering-geth-node, peering-openeth-node")
	cmd.Flags().String("common-deployment-id", DefaultDeploymentID, "[COMMON] Deployment ID, used for some billing functions by dgraphql")

	//// Authentication, metering and rate limiter plugins
	cmd.Flags().String("common-auth-plugin", "null://", "[COMMON] Auth plugin URI, see streamingfast/dauth repository")
	cmd.Flags().String("common-metering-plugin", "null://", "[COMMON] Metering plugin URI, see streamingfast/dmetering repository")
	cmd.Flags().String("common-ratelimiter-plugin", "null://", "[COMMON] Rate Limiter plugin URI, see streamingfast/dauth repository")

	//// Database connection strings

	// System Behavior
	cmd.Flags().Duration("common-system-shutdown-signal-delay", 0, "[COMMON] Add a delay between receiving SIGTERM signal and shutting down apps. Apps will respond negatively to /healthz during this period")

	return nil
}
