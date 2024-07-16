package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/cli"
	firecore "github.com/streamingfast/firehose-core"
	fhCmd "github.com/streamingfast/firehose-core/cmd"
	"github.com/streamingfast/firehose-ethereum/codec"
	ethss "github.com/streamingfast/firehose-ethereum/substreams"
	"github.com/streamingfast/firehose-ethereum/transform"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"github.com/streamingfast/logging"
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
	}
	return chain
}

// Version value, injected via go build `ldflags` at build time, **must** not be removed or inlined
var version = "dev"
