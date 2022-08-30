package cli

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/streamingfast/bstream"
	bstransform "github.com/streamingfast/bstream/transform"
	"github.com/streamingfast/dlauncher/launcher"
	indexerApp "github.com/streamingfast/index-builder/app/index-builder"
	"github.com/streamingfast/sf-ethereum/transform"
	pbeth "github.com/streamingfast/sf-ethereum/types/pb/sf/ethereum/type/v2"
)

func init() {
	launcher.RegisterApp(zlog, &launcher.AppDef{
		ID:          "combined-index-builder",
		Title:       "Combined Index Builder",
		Description: "Produces a combined index for a given set of blocks",
		RegisterFlags: func(cmd *cobra.Command) error {
			cmd.Flags().Uint64("combined-index-builder-index-size", 10000, "size of combined index bundles that will be created")
			cmd.Flags().Uint64("combined-index-builder-start-block", 0, "block number to start indexing")
			cmd.Flags().Uint64("combined-index-builder-stop-block", 0, "block number to stop indexing")
			cmd.Flags().String("combined-index-builder-grpc-listen-addr", IndexBuilderServiceAddr, "Address to listen for grpc-based healthz check")
			return nil
		},
		InitFunc: func(runtime *launcher.Runtime) error {
			return nil
		},
		FactoryFunc: func(runtime *launcher.Runtime) (launcher.App, error) {

			mergedBlocksStoreURL, _, _, err := GetCommonStoresURLs(runtime.AbsDataDir)
			if err != nil {
				return nil, err
			}

			indexStore, lookupIdxSizes, err := GetIndexStore(runtime.AbsDataDir)
			if err != nil {
				return nil, err
			}

			startBlockResolver := func(ctx context.Context) (uint64, error) {
				select {
				case <-ctx.Done():
					return 0, ctx.Err()
				default:
				}

				startBlockNum := bstransform.FindNextUnindexed(
					ctx,
					viper.GetUint64("combined-index-builder-start-block"),
					lookupIdxSizes,
					transform.CombinedIndexerShortName,
					indexStore,
				)

				return startBlockNum, nil
			}
			stopBlockNum := viper.GetUint64("combined-index-builder-stop-block")

			combinedIndexer := transform.NewEthCombinedIndexer(indexStore, viper.GetUint64("combined-index-builder-index-size"))
			handler := bstream.HandlerFunc(func(blk *bstream.Block, obj interface{}) error {
				combinedIndexer.ProcessBlock(blk.ToNative().(*pbeth.Block))
				return nil
			})

			app := indexerApp.New(&indexerApp.Config{
				BlockHandler:       handler,
				StartBlockResolver: startBlockResolver,
				EndBlock:           stopBlockNum,
				BlockStorePath:     mergedBlocksStoreURL,
				GRPCListenAddr:     viper.GetString("combined-index-builder-grpc-listen-addr"),
			})

			return app, nil
		},
	})
}
