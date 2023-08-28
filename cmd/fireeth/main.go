package main

import (
	"github.com/spf13/cobra"
	firecore "github.com/streamingfast/firehose-core"
	"github.com/streamingfast/firehose-ethereum/codec"
	"github.com/streamingfast/firehose-ethereum/transform"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"github.com/streamingfast/logging"
	"github.com/streamingfast/node-manager/mindreader"
	pbbstream "github.com/streamingfast/pbgo/sf/bstream/v1"
	"go.uber.org/zap"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func init() {
	firecore.UnsafePayloadKind = pbbstream.Protocol_ETH
}

func main() {
	firecore.Main(&firecore.Chain[*pbeth.Block]{
		ShortName:            "eth",
		LongName:             "Ethereum",
		ExecutableName:       "geth",
		FullyQualifiedModule: "github.com/streamingfast/firehose-ethereum",
		Version:              version,

		Protocol:        "ETH",
		ProtocolVersion: 1,

		BlockFactory: func() firecore.Block { return new(pbeth.Block) },

		BlockIndexerFactories: map[string]firecore.BlockIndexerFactory[*pbeth.Block]{
			transform.CombinedIndexerShortName: transform.NewEthCombinedIndexer,
		},

		BlockTransformerFactories: map[protoreflect.FullName]firecore.BlockTransformerFactory{
			transform.HeaderOnlyMessageName:     transform.NewHeaderOnlyTransformFactory,
			transform.CombinedFilterMessageName: transform.NewCombinedFilterTransformFactory,

			// Still needed?
			transform.MultiCallToFilterMessageName: transform.NewMultiCallToFilterTransformFactory,
			transform.MultiLogFilterMessageName:    transform.NewMultiLogFilterTransformFactory,
		},

		ConsoleReaderFactory: func(lines chan string, blockEncoder firecore.BlockEncoder, logger *zap.Logger, tracer logging.Tracer) (mindreader.ConsolerReader, error) {
			// FIXME: This was hardcoded also in the previouse firehose-near version, Firehose will break if this is not available
			// blockEncoder
			return codec.NewConsoleReader(logger, lines)
		},

		// ReaderNodeBootstrapperFactory: newReaderNodeBootstrapper,

		Tools: &firecore.ToolsConfig[*pbeth.Block]{
			BlockPrinter: printBlock,

			RegisterExtraCmd: func(chain *firecore.Chain[*pbeth.Block], toolsCmd *cobra.Command, zlog *zap.Logger, tracer logging.Tracer) error {
				// toolsCmd.AddCommand(newToolsGenerateNodeKeyCmd(chain))
				// toolsCmd.AddCommand(newToolsBackfillCmd(zlog))

				return nil
			},

			TransformFlags: map[string]*firecore.TransformFlag{
				// "receipt-account-filters": {
				// 	Description: "Comma-separated accounts to use as filter/index. If it contains a colon (:), it will be interpreted as <prefix>:<suffix> (each of which can be empty, ex: 'hello:' or ':world')",
				// 	Parser:      parseReceiptAccountFilters,
				// },
			},
		},
	})
}

// Version value, injected via go build `ldflags` at build time, **must** not be removed or inlined
var version = "dev"
