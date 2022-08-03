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
	pbbstream "github.com/streamingfast/pbgo/sf/bstream/v1"
)

const (
	Protocol = pbbstream.Protocol_ETH

	DefaultChainID      uint32 = 123
	DefaultNetworkID    uint32 = 123
	DefaultDeploymentID string = "eth-local"

	OneBlockStoreURL     string = "file://{sf-data-dir}/storage/one-blocks"
	ForkedBlocksStoreURL string = "file://{sf-data-dir}/storage/forked-blocks"
	MergedBlocksStoreURL string = "file://{sf-data-dir}/storage/merged-blocks"
	SnapshotsURL         string = "file://{sf-data-dir}/storage/snapshots"
	StateDBDSN           string = "badger://{sf-data-dir}/storage/statedb"

	MetricsListenAddr string = ":9102"

	BlocksCacheDirectory         string = "{sf-data-dir}/blocks-cache"
	BlockstreamGRPCServingAddr   string = ":13039"
	BlockstreamHTTPServingAddr   string = ":13040"
	EVMExecutorGRPCServingAddr   string = ":13036"
	FirehoseGRPCServingAddr      string = ":13042"
	MergerServingAddr            string = ":13012"
	MindreaderNodeManagerAPIAddr string = ":13009"
	MindreaderGRPCAddr           string = ":13010"
	NodeManagerAPIAddr           string = ":13041"
	RelayerServingAddr           string = ":13011"
	TokenMetaServingAddr         string = ":13039"
	TraderServingAddr            string = ":13038"
	StateDBServingAddr           string = ":13029"
	StateDBGRPCServingAddr       string = ":13035"

	// Geth instance port definitions
	MindreaderNodeP2PPort string = "30305"
	MindreaderNodeRPCPort string = "8547"
	NodeP2PPort           string = "30303"
	NodeRPCPort           string = "8545"
	devMinerAddress       string = "0x821b55d8abe79bc98f05eb675fdc50dfe796b7ab"
)
