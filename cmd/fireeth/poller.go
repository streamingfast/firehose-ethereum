package main

import (
	"fmt"
	"path"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/streamingfast/cli"
	"github.com/streamingfast/cli/sflags"
	"github.com/streamingfast/eth-go/rpc"
	firecore "github.com/streamingfast/firehose-core"
	"github.com/streamingfast/firehose-core/blockpoller"
	firecorerpc "github.com/streamingfast/firehose-core/rpc"
	"github.com/streamingfast/firehose-ethereum/blockfetcher"
	"github.com/streamingfast/firehose-ethereum/cmd/fireeth/rpcprovider"
	"github.com/streamingfast/logging"
	"go.uber.org/zap"
)

func newPollerCmd(logger *zap.Logger, tracer logging.Tracer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "poller",
		Short: "poll blocks from different sources",
		Long: cli.Dedent(`
			Polls blocks from one or more RPC providers. The first provider is the preferred one,
			the next ones are used as fallback when the preferred one errors out.

			Providers are declared either through the '<rpc-endpoint>' positional argument, which is
			always the highest priority provider and is named "default", or through the repeatable
			'--provider' flag, or both. The '--provider' value is '<url>[#<name>]', the provider's
			name being everything after the last '#' of the value:

			    --provider=https://mainnet.example.com/v1/<api-key>          named "mainnet.example.com", the URL's host
			    --provider=https://mainnet.example.com/v1/<api-key>#primary  named "primary"

			Provider names must be unique. They identify the provider in the logs and they are what
			'--headers' uses to send a header to a single provider, through that same '#<name>'
			suffix, again taken after the last '#' of the value:

			    --headers="X-Api-Key: shared"                  sent to every provider
			    --headers="Authorization: Bearer xyz#primary"  sent only to the "primary" provider

			'--headers' declares a single header and is repeated to send more than one, its value
			being taken verbatim, commas included.

			Since both suffixes are taken after the last '#', a URL or a header value legitimately
			ending with '#<something>' is going to be mis-parsed, there is no escaping syntax. A
			'#' anywhere else in the value, like in the middle of an API key, is left alone.

			Examples:

			    fireeth tools poller optimism https://endpoint.example.com 1000

			    fireeth tools poller optimism 1000 \
			      --provider=https://primary.example.com/v1/key#primary \
			      --provider=https://fallback.example.com/key#fallback \
			      --headers="Authorization: Bearer token#fallback"
		`),
	}

	cmd.PersistentFlags().Uint("parallel-workers", 20, "number of parallel workers to fetch transaction receipts")
	cmd.PersistentFlags().Uint("block-fetch-batch-size", 1, "number of blocks to fetch (and serialize) in parallel")
	cmd.PersistentFlags().StringArray("provider", nil, "RPC provider to poll blocks from, in the form '<url>[#<name>]', repeat the flag to declare multiple providers, the flag order being the priority order (first one preferred, next ones used as fallback). The name defaults to the URL's host and is what identifies the provider in logs and in '--headers'. It is taken after the last '#' of the value")
	cmd.PersistentFlags().StringArrayP("headers", "H", nil, "header to send with each request, either 'Key: Value' to send it to every provider or 'Key: Value#<provider-name>' to send it only to that provider, repeat the flag to send multiple headers (ex: '-H \"key1: value1\" -H \"Authorization: Bearer xyz#fallback\"'). The value is taken verbatim, commas included. The provider name is taken after the last '#' of the value, so a header value legitimately ending with '#<something>' is going to be mis-parsed, there is no escaping syntax")
	cmd.PersistentFlags().Duration("providers-failback-interval", 10*time.Minute, "interval at which the declared provider order is re-preferred, moving polling back to the preferred provider after a transient error moved it to a fallback one. Set to 0 to stick to the fallback provider until the process is restarted")
	cmd.PersistentFlags().Bool("allow-empty-receipts-on-block-0", false, "whether to accept empty receipts on block 0, filling them with 'empty' transactions (useful for TRON-EVM)")

	cmd.AddCommand(newOptimismPollerCmd(logger, tracer))
	cmd.AddCommand(newArbOnePollerCmd(logger, tracer))
	cmd.AddCommand(newGenericEVMPollerCmd(logger, tracer))
	cmd.AddCommand(newFirehoseTracerPollerCmd(logger, tracer))
	return cmd
}

func newOptimismPollerCmd(logger *zap.Logger, tracer logging.Tracer) *cobra.Command {
	// identical as generic-evm for now
	cmd := &cobra.Command{
		Use:   "optimism [<rpc-endpoint>] <first-streamable-block>",
		Short: "poll blocks from optimism rpc",
		Args:  cobra.RangeArgs(1, 2),
		RunE:  pollerRunE(logger, tracer),
	}
	cmd.Flags().Duration("interval-between-fetch", 0, "interval between fetch")
	cmd.Flags().Duration("max-block-fetch-duration", 5*time.Second, "maximum delay before retrying a block fetch")

	return cmd
}
func newArbOnePollerCmd(logger *zap.Logger, tracer logging.Tracer) *cobra.Command {
	// identical as generic-evm for now
	cmd := &cobra.Command{
		Use:   "arb-one [<rpc-endpoint>] <first-streamable-block>",
		Short: "poll blocks from arb-one rpc",
		Args:  cobra.RangeArgs(1, 2),
		RunE:  pollerRunE(logger, tracer),
	}
	cmd.Flags().Duration("interval-between-fetch", 0, "interval between fetch")
	cmd.Flags().Duration("max-block-fetch-duration", 5*time.Second, "maximum delay before retrying a block fetch")

	return cmd
}
func newGenericEVMPollerCmd(logger *zap.Logger, tracer logging.Tracer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generic-evm [<rpc-endpoint>] <first-streamable-block>",
		Short: "poll blocks from a generic EVM RPC endpoint",
		Args:  cobra.RangeArgs(1, 2),
		RunE:  pollerRunE(logger, tracer),
	}
	cmd.Flags().Duration("interval-between-fetch", 0, "interval between fetch")
	cmd.Flags().Duration("max-block-fetch-duration", 5*time.Second, "maximum delay before retrying a block fetch")

	return cmd
}

func newFirehoseTracerPollerCmd(logger *zap.Logger, tracer logging.Tracer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "firehose-tracer-api [<rpc-endpoint>] <first-streamable-block>",
		Short: "poll blocks using debug_traceFirehoseBlockByNumber",
		Long: cli.Dedent(`
			This poller connects to a Firehose-enabled RPC endpoint and fetches
			blocks using the "debug_traceFirehoseBlockByNumber" method. It retrieves the
			full Firehose block data along with execution traces, enabling advanced debugging
			and block-level analysis.

			*Experimental*: This tool is not production-ready. Intended for development
			and debugging purposes only.
		`),
		Args: cobra.RangeArgs(1, 2),
		RunE: pollerRunEForTracer(logger),
	}
	cmd.Flags().Duration("interval-between-fetch", 0, "interval between fetch")
	cmd.Flags().Duration("max-block-fetch-duration", 5*time.Second, "maximum delay before retrying a block fetch")

	return cmd
}

func pollerRunE(logger *zap.Logger, tracer logging.Tracer) firecore.CommandExecutor {
	return pollerRunEInternal(logger, tracer, false)
}

func pollerRunEForTracer(logger *zap.Logger) func(cmd *cobra.Command, args []string) error {
	return pollerRunEInternal(logger, nil, true)
}

func pollerRunEInternal(logger *zap.Logger, tracer logging.Tracer, useTracer bool) firecore.CommandExecutor {
	return func(cmd *cobra.Command, args []string) (err error) {
		// Two forms are accepted, '<rpc-endpoint> <first-streamable-block>' where the endpoint
		// becomes the highest priority provider, and '<first-streamable-block>' alone in which
		// case all providers come from the '--provider' flag.
		var rpcEndpoint string
		firstStreamableBlockArg := args[0]
		if len(args) == 2 {
			rpcEndpoint, firstStreamableBlockArg = args[0], args[1]
		}

		//dataDir := cmd.Flag("data-dir").Value.String()
		fetchInterval := sflags.MustGetDuration(cmd, "interval-between-fetch")
		parallelWorkers := sflags.MustGetInt(cmd, "parallel-workers")
		blockFetchBatchSize := sflags.MustGetInt(cmd, "block-fetch-batch-size")
		maxBlockFetchDuration := sflags.MustGetDuration(cmd, "max-block-fetch-duration")
		failbackInterval := sflags.MustGetDuration(cmd, "providers-failback-interval")

		dataDir := sflags.MustGetString(cmd, "data-dir")
		stateDir := path.Join(dataDir, "poller-state")

		firstStreamableBlock, err := strconv.ParseUint(firstStreamableBlockArg, 10, 64)
		if err != nil {
			return fmt.Errorf("unable to parse first streamable block %q: %w", firstStreamableBlockArg, err)
		}

		providers, err := rpcprovider.Parse(rpcEndpoint, sflags.MustGetStringArray(cmd, "provider"), sflags.MustGetStringArray(cmd, "headers"))
		if err != nil {
			return err
		}

		redactedProviders := make([]string, len(providers))
		for i, provider := range providers {
			redactedProviders[i] = provider.Name + "=" + provider.RedactedURL()
		}

		logger.Info("launching firehose-ethereum poller",
			zap.Strings("rpc_providers", redactedProviders),
			zap.String("data_dir", dataDir),
			zap.String("state_dir", stateDir),
			zap.Duration("fetch_interval", fetchInterval),
			zap.Duration("max_block_fetch_duration", maxBlockFetchDuration),
			zap.Duration("providers_failback_interval", failbackInterval),
			zap.Uint64("first_streamable_block", firstStreamableBlock),
			zap.Int("block_fetch_batch_size", blockFetchBatchSize),
		)
		rpcClients := firecorerpc.NewClients[*rpc.Client](maxBlockFetchDuration, firecorerpc.NewStickyRollingStrategy[*rpc.Client](), logger)

		for _, provider := range providers {
			opts := make([]rpc.Option, len(provider.Headers))
			for i, header := range provider.Headers {
				opts[i] = rpc.WithHttpHeader(header.Key, header.Value)
			}

			rpcClients.AddNamed(rpc.NewClient(provider.URL, opts...), provider.Name)
		}

		// The rolling strategy is sticky forever, a single transient error on the preferred provider
		// moves polling to a fallback one until the process restarts. Periodically re-preferring the
		// declared order bounds how long we stay on a fallback provider.
		if failbackInterval > 0 && len(providers) > 1 {
			go func() {
				for range time.Tick(failbackInterval) {
					logger.Debug("re-preferring declared provider order", zap.String("preferred_provider", providers[0].Name))
					rpcClients.Reset()
				}
			}()
		}

		var fetcher blockpoller.BlockFetcher[*rpc.Client]
		if useTracer {
			fetcher = blockfetcher.NewTracerBlockFetcher(fetchInterval, 1*time.Second, parallelWorkers, sflags.MustGetBool(cmd, "allow-empty-receipts-on-block-0"), logger)
		} else {
			fetcher = blockfetcher.NewGenericBlockFetcher(fetchInterval, 1*time.Second, parallelWorkers, sflags.MustGetBool(cmd, "allow-empty-receipts-on-block-0"), logger)
		}
		handler := blockpoller.NewFireBlockHandler("type.googleapis.com/sf.ethereum.type.v2.Block")
		poller := blockpoller.New[*rpc.Client](fetcher, handler, rpcClients, blockpoller.WithStoringState[*rpc.Client](stateDir), blockpoller.WithLogger[*rpc.Client](logger))

		err = poller.Run(firstStreamableBlock, nil, blockFetchBatchSize)
		if err != nil {
			return fmt.Errorf("running poller: %w", err)
		}

		return nil
	}
}
