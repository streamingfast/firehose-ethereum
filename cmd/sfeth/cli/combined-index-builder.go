package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/streamingfast/bstream"
	bstransform "github.com/streamingfast/bstream/transform"
	"github.com/streamingfast/dlauncher/launcher"
	"github.com/streamingfast/dstore"
	indexerApp "github.com/streamingfast/index-builder/app/index-builder"
	"github.com/streamingfast/sf-ethereum/transform"
	pbeth "github.com/streamingfast/sf-ethereum/types/pb/sf/ethereum/type/v1"
)

func init() {
	launcher.RegisterApp(zlog, &launcher.AppDef{
		ID:          "combined-index-builder",
		Title:       "Combined Index Builder",
		Description: "Produces a combined index for a given set of blocks",
		RegisterFlags: func(cmd *cobra.Command) error {
			cmd.Flags().Uint64("combined-index-builder-index-size", 10000, "size of combined index bundles that will be created")
			cmd.Flags().IntSlice("combined-index-builder-lookup-index-sizes", []int{1000000, 100000, 10000, 1000}, "index bundle sizes that we will look for on start to find first unindexed block")
			cmd.Flags().Uint64("combined-index-builder-start-block", 0, "block number to start indexing")
			cmd.Flags().Uint64("combined-index-builder-stop-block", 0, "block number to stop indexing")
			cmd.Flags().String("combined-index-builder-grpc-listen-addr", IndexBuilderServiceAddr, "Address to listen for incoming gRPC requests")
			return nil
		},
		InitFunc: func(runtime *launcher.Runtime) error {
			return nil
		},
		FactoryFunc: func(runtime *launcher.Runtime) (launcher.App, error) {
			sfDataDir := runtime.AbsDataDir

			indexStoreURL := MustReplaceDataDir(sfDataDir, viper.GetString("common-index-store-url"))
			blockStoreURL := MustReplaceDataDir(sfDataDir, viper.GetString("common-blocks-store-url"))

			indexStore, err := dstore.NewStore(indexStoreURL, "", "", false)
			if err != nil {
				return nil, err
			}

			var lookupIdxSizes []uint64
			lookupIndexSizes := viper.GetIntSlice("combined-index-builder-lookup-index-sizes")
			for _, size := range lookupIndexSizes {
				if size < 0 {
					return nil, fmt.Errorf("invalid negative size for bundle-sizes: %d", size)
				}
				lookupIdxSizes = append(lookupIdxSizes, uint64(size))
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
				BlockStorePath:     blockStoreURL,
				GRPCListenAddr:     viper.GetString("combined-index-builder-grpc-listen-addr"),
			})

			return app, nil
		},
	})
}
