package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/streamingfast/bstream"
	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	"github.com/streamingfast/cli"
	firecore "github.com/streamingfast/firehose-core"
	fhCmd "github.com/streamingfast/firehose-core/cmd"
	"github.com/streamingfast/firehose-ethereum/codec"
	ethss "github.com/streamingfast/firehose-ethereum/substreams"
	"github.com/streamingfast/firehose-ethereum/transform"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"github.com/streamingfast/logging"
	pbfirehose "github.com/streamingfast/pbgo/sf/firehose/v2"
	"github.com/streamingfast/substreams/wasm"
	"go.uber.org/zap"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func init() {
	bstream.NormalizeBlockID = func(in string) string {
		return strings.TrimPrefix(strings.ToLower(in), "0x")
	}
}

func main() {
	fhCmd.Main(Chain())
}

var chain *firecore.Chain[*pbeth.Block]

func Chain() *firecore.Chain[*pbeth.Block] {
	if chain != nil {
		return chain
	}

	chain = &firecore.Chain[*pbeth.Block]{
		ShortName:            "eth",
		LongName:             "Ethereum",
		ExecutableName:       "geth",
		FullyQualifiedModule: "github.com/streamingfast/firehose-ethereum",
		Version:              version,
		DefaultBlockType:     "sf.ethereum.type.v2.Block",

		// Ensure that if you ever modify test, modify also `types/init.go#init` so that the `bstream.InitGeneric` there fits us
		BlockFactory: func() firecore.Block { return new(pbeth.Block) },

		BlockIndexerFactories: map[string]firecore.BlockIndexerFactory[*pbeth.Block]{
			transform.CombinedIndexerShortName: transform.NewEthCombinedIndexer,
		},

		BlockTransformerFactories: map[protoreflect.FullName]firecore.BlockTransformerFactory{
			transform.HeaderOnlyMessageName:     transform.NewHeaderOnlyTransformFactory,
			transform.CombinedFilterMessageName: transform.NewCombinedFilterTransformFactory,

			transform.MultiCallToFilterMessageName: transform.NewMultiCallToFilterTransformFactory,
			transform.MultiLogFilterMessageName:    transform.NewMultiLogFilterTransformFactory,
		},

		ConsoleReaderFactory: codec.NewConsoleReader,

		RegisterExtraStartFlags: func(flags *pflag.FlagSet) {
			// The "\n" is there on purpose to improve readability of the added elements
			flags.String("reader-node-bootstrap-data-url", "", firecore.DefaultReaderNodeBootstrapDataURLFlagDescription()+"\n"+cli.Dedent(`
				If the URL ends with json, it will be treated as a genesis.json file and the node will be bootstrapped with it. The bootstrapping
				is done by calling 'geth init' with the genesis file. If you need more custom logic, think about using 'bash://...' instead
				which execute any bash script and offers more flexibility.
			`)+"\n")

			flags.StringArray("substreams-rpc-endpoints", nil, "Remote endpoints to contact to satisfy Substreams 'eth_call's")
			flags.Uint64("substreams-rpc-gas-limit", 50_000_000, "Gas limit to set when calling RPC (set it to 0 for arbitrum chains, otherwise you should keep 50M)")
		},

		RegisterSubstreamsExtensions: func() (wasm.WASMExtensioner, error) {
			rpcGasLimit := viper.GetUint64("substreams-rpc-gas-limit")
			rpcEndpoints := viper.GetStringSlice("substreams-rpc-endpoints")

			commaCheck := func(ss []string) bool {
				for _, s := range ss {
					if strings.Contains(s, ",") {
						return true
					}
				}
				return false
			}
			if commaCheck(rpcEndpoints) {
				return nil, fmt.Errorf("rpc endpoints cannot contain commas")
			}

			rpcData := fmt.Sprintf("%d,%s", rpcGasLimit, strings.Join(rpcEndpoints, ","))
			return ethss.NewRPCExtensioner(map[string]string{
				"rpc_eth_call": rpcData,
			}), nil
		},

		ReaderNodeBootstrapperFactory: firecore.DefaultReaderNodeBootstrapper(newReaderNodeBootstrapper),

		Tools: &firecore.ToolsConfig[*pbeth.Block]{

			RegisterExtraCmd: func(chain *firecore.Chain[*pbeth.Block], parent *cobra.Command, zlog *zap.Logger, tracer logging.Tracer) error {
				parent.AddCommand(compareOneblockRPCCmd())
				parent.AddCommand(newCompareBlocksStoreRPCCmd(zlog))
				parent.AddCommand(newCompareBlocksRPCCmd(zlog))
				parent.AddCommand(newFixOrdinalsCmd(zlog))
				parent.AddCommand(newFixAnyTypeCmd(zlog))
				parent.AddCommand(newPollRPCBlocksCmd(zlog))
				parent.AddCommand(newPollerCmd(zlog, tracer))
				parent.AddCommand(newOptimismPollerCmd(zlog, tracer))
				parent.AddCommand(newScanForUnknownStatusCmd(zlog))

				registerGethEnforcePeersCmd(parent, chain.BinaryName(), zlog, tracer)

				return nil
			},

			TransformFlags: &firecore.TransformFlags{
				Register: func(flags *pflag.FlagSet) {
					flags.Bool("header-only", false, "Apply the HeaderOnly transform sending back Block's header only (with few top-level fields), exclusive option")
					flags.String("call-filters", "", "call filters (format: '[address1[+address2[+...]]]:[eventsig1[+eventsig2[+...]]]")
					flags.String("log-filters", "", "log filters (format: '[address1[+address2[+...]]]:[eventsig1[+eventsig2[+...]]]")
					flags.Bool("send-all-block-headers", false, "ask for all the blocks to be sent (header-only if there is no match)")
				},

				Parse: parseTransformFlags,
			},
		},

		InfoResponseFiller: fillChainInfoResponse,
	}
	return chain
}

// Version value, injected via go build `ldflags` at build time, **must** not be removed or inlined
var version = "dev"

var fillChainInfoResponse = func(block *pbbstream.Block, resp *pbfirehose.InfoResponse) error {
	resp.FirstStreamableBlockId = block.Id
	resp.BlockIdEncoding = pbfirehose.InfoResponse_BLOCK_ID_ENCODING_HEX

	ethBlock := &pbeth.Block{}
	if err := block.Payload.UnmarshalTo(ethBlock); err != nil {
		return fmt.Errorf("cannot decode first streamable block: %w", err)
	}

	var detailLevel string
	switch ethBlock.DetailLevel {
	case pbeth.Block_DETAILLEVEL_BASE:
		detailLevel = "base"
	case pbeth.Block_DETAILLEVEL_EXTENDED:
		detailLevel = "extended"
	default:
		return fmt.Errorf("unknown detail level in block: %q", ethBlock.DetailLevel)
	}

	var flagDetailLevel string
	for _, feature := range resp.BlockFeatures {
		if feature == "base" || feature == "extended" || feature == "hybrid" {
			flagDetailLevel = feature
			break
		}
	}
	switch flagDetailLevel {
	case "":
		resp.BlockFeatures = append(resp.BlockFeatures, detailLevel)
	case "hybrid":
		// we ignore the detail level from blocks in this case
	default:
		if flagDetailLevel != detailLevel {
			return fmt.Errorf("detail level defined in flag: %q inconsistent with the one seen in blocks %q", flagDetailLevel, detailLevel)
		}
	}

	if resp.ChainName != "" {
		if chain := knownChains.byName(resp.ChainName); chain != nil {
			if block.Number == chain.genesisBlockNumber && chain.genesisBlockID != block.Id { // we can only check if first-streamable-block == gensis block
				return fmt.Errorf("chain name defined in flag: %q inconsistent with the genesis block ID %q (expected: %q)", resp.ChainName, ox(block.Id), ox(chain.genesisBlockID))
			}
			resp.ChainName = chain.name
			resp.ChainNameAliases = chain.aliases
		} else if chain := knownChains.byBlock(block.Number, block.Id); chain != nil {
			return fmt.Errorf("chain name defined in flag: %q inconsistent with the one discovered from genesis block %q", resp.ChainName, chain.name)
		}
	} else {
		if chain := knownChains.byBlock(block.Number, block.Id); chain != nil {
			resp.ChainName = chain.name
			resp.ChainNameAliases = chain.aliases
		}
	}

	return nil
}

func ox(in string) string {
	return "0x" + in
}

type knownChain struct {
	name               string
	aliases            []string
	genesisBlockID     string
	genesisBlockNumber uint64
}

type chainList []*knownChain

func (c chainList) byBlock(blockNum uint64, blockID string) *knownChain {
	for _, chain := range knownChains {
		if chain.genesisBlockNumber == blockNum && chain.genesisBlockID == blockID {
			return chain
		}
	}
	return nil
}

func (c chainList) byName(name string) *knownChain {
	for _, chain := range knownChains {
		if chain.name == name {
			return chain
		}
		for _, alias := range chain.aliases {
			if alias == name {
				return chain
			}
		}
	}
	return nil
}

var knownChains = chainList{
	{
		name:               "mainnet",
		aliases:            []string{"ethereum"},
		genesisBlockID:     "d4e56740f876aef8c010b86a40d5f56745a118d0906a34e69aec8c0db1cb8fa3",
		genesisBlockNumber: 0,
	},
	{
		name:               "matic",
		aliases:            []string{"polygon"},
		genesisBlockID:     "a9c28ce2141b56c474f1dc504bee9b01eb1bd7d1a507580d5519d4437a97de1b",
		genesisBlockNumber: 0,
	},
	{
		name:               "bsc",
		aliases:            []string{"bnb"},
		genesisBlockID:     "0d21840abff46b96c84b2ac9e10e4f5cdaeb5693cb665db62a2f3b02d2d57b5b",
		genesisBlockNumber: 0,
	},
}
