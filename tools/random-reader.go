package tools

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"strconv"
	"time"

	"go.uber.org/zap"

	"github.com/spf13/cobra"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/dstore"
)

var randomReadCmd = &cobra.Command{
	Use:   "random-read [source-store] [start-block] [stop-block]",
	Short: "randomly read blocks from a store in a given range. will loop for 3 hours or until killed.",
	Args:  cobra.ExactArgs(3),
	RunE:  randomReadE,
}

func init() {
	Cmd.AddCommand(randomReadCmd)
}

func randomReadE(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	ctx, cancel := context.WithTimeout(ctx, 1*time.Hour)
	defer cancel()

	srcStore, err := dstore.NewDBinStore(args[0])
	if err != nil {
		return fmt.Errorf("unable to create source store: %w", err)
	}

	start := mustParseUint64(args[1])
	stop := mustParseUint64(args[2])

	if stop <= start {
		return fmt.Errorf("stop block must be greater than start block")
	}

	//get all merged block bundles in the range
	var bundles []string
	zlog.Debug("walking source store", zap.String("start", fmt.Sprintf("%010d", start)))
	err = srcStore.WalkFrom(ctx, "", fmt.Sprintf("%010d", start), func(filename string) error {
		i, err := strconv.Atoi(filename)
		if err != nil {
			return err
		}
		if uint64(i) > stop {
			return io.EOF
		}

		zlog.Debug("found bundle", zap.String("filename", filename))
		bundles = append(bundles, filename)
		return nil
	})

	if err != nil && err != io.EOF {
		return err
	}

	zlog.Debug("done walking source store", zap.Int("bundles", len(bundles)))

	for {
		select {
		case <-ctx.Done():
			log.Print("done")
			return nil

		default:
			///
		}

		//get a random bundle
		filename := bundles[rand.Intn(len(bundles))]

		//get a random block from the bundle and read it
		err = func() error { // this is in a func like this in order to defer the rc.Close() call correctly.
			zlog.Debug("opening bundle", zap.String("filename", filename))

			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
			defer cancel()

			rc, err := srcStore.OpenObject(ctx, filename)
			if err != nil {
				return fmt.Errorf("failed to open %s: %w", filename, err)
			}
			defer rc.Close()

			br, err := bstream.GetBlockReaderFactory.New(rc)
			if err != nil {
				return fmt.Errorf("creating block reader: %w", err)
			}

			// iterate through the blocks in the file
		Out:
			for {
				b, err := br.Read()
				if err == io.EOF {
					break Out
				}

				//
				p := b.ToProtocol()
				_ = p

				zlog.Debug("read block", zap.String("id", b.ID()), zap.Uint64("num", b.Num()))

				if err != nil {
					return fmt.Errorf("reading block: %w", err)
				}
			}

			zlog.Info("finished reading bundle", zap.String("filename", filename))

			return nil
		}()

		if err != nil {
			panic(err)
		}
	}

	return nil
}
