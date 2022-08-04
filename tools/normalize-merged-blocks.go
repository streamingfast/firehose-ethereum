package tools

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/bstream/stream"
	"github.com/streamingfast/dstore"
	"github.com/streamingfast/sf-ethereum/types"
	pbeth "github.com/streamingfast/sf-ethereum/types/pb/sf/ethereum/type/v1"
	"go.uber.org/zap"
)

func init() {
	Cmd.AddCommand(NormalizeMergedBlocksCmd)
}

var NormalizeMergedBlocksCmd = &cobra.Command{
	Use:     "normalize-merged-blocks <source> <destination> <start> <stop> ",
	Short:   "from a merged-blocks source, rewrite normalized blocks to a merged-blocks destination. normalized blocks are FINAL only, with fixed log ordinals",
	Args:    cobra.ExactArgs(4),
	RunE:    normalizeMergedBlocksE,
	Example: "sfeth tools normalized-merged-blocks /my/original/merged-blocks /destination/merged-blocks 0 10000",
}

func normalizeMergedBlocksE(cmd *cobra.Command, args []string) error {

	source := args[0]
	sourceStore, err := dstore.NewDBinStore(source)
	if err != nil {
		return fmt.Errorf("reading source store: %w", err)
	}

	dest := args[1]
	destStore, err := dstore.NewStore(dest, "dbin.zst", "zstd", true) // overwrites
	if err != nil {
		return fmt.Errorf("reading destination store: %w", err)
	}

	start, err := strconv.ParseUint(args[2], 10, 64)
	if err != nil {
		return fmt.Errorf("parsing start block num: %w", err)
	}
	stop, err := strconv.ParseUint(args[3], 10, 64)
	if err != nil {
		return fmt.Errorf("parsing stop block num: %w", err)
	}

	writer := &mergedBlocksWriter{
		store:         destStore,
		lowBlockNum:   start,
		stopBlockNum:  stop,
		writerFactory: bstream.GetBlockWriterFactory,
		tweakBlock:    normalize,
		logger:        zlog,
	}
	stream := stream.New([]dstore.Store{sourceStore}, int64(start), writer, stream.WithForkableSteps(bstream.StepIrreversible))

	err = stream.Run(context.Background())
	if errors.Is(err, io.EOF) {
		zlog.Info("Complete!")
		return nil
	}
	return err
}

func normalize(in *bstream.Block) (*bstream.Block, error) {
	block := in.ToProtocol().(*pbeth.Block)
	types.NormalizeBlockInPlace(block)
	return types.BlockFromProto(block)
}

type mergedBlocksWriter struct {
	store        dstore.Store
	lowBlockNum  uint64
	stopBlockNum uint64

	blocks        []*bstream.Block
	writerFactory bstream.BlockWriterFactory
	logger        *zap.Logger
	tweakBlock    func(*bstream.Block) (*bstream.Block, error)
}

func (w *mergedBlocksWriter) ProcessBlock(blk *bstream.Block, obj interface{}) error {

	if len(w.blocks) == 0 && w.lowBlockNum == 0 { // initial block
		if blk.Number%100 == 0 || blk.Number == bstream.GetProtocolFirstStreamableBlock {
			w.lowBlockNum = lowBoundary(blk.Number)
			w.blocks = append(w.blocks, blk)
			return nil
		} else {
			return fmt.Errorf("received unexpected block %s (not a boundary, not the first streamable block %d)", blk, bstream.GetProtocolFirstStreamableBlock)
		}
	}

	if w.tweakBlock != nil {
		b, err := w.tweakBlock(blk)
		if err != nil {
			return fmt.Errorf("tweaking block: %w", err)
		}
		blk = b
	}

	// perfect aligned @99 case (no block skipping on that chain)
	if blk.Number == w.lowBlockNum+99 {
		w.blocks = append(w.blocks, blk)

		if err := w.writeBundle(); err != nil {
			return err
		}
		return nil
	}
	// there was no @99 so we went over
	if blk.Number >= w.lowBlockNum+100 {
		if err := w.writeBundle(); err != nil {
			return err
		}
	}

	if blk.Number >= w.stopBlockNum {
		return io.EOF
	}

	w.blocks = append(w.blocks, blk)

	return nil
}

func filename(num uint64) string {
	return fmt.Sprintf("%010d", num)
}

func (w *mergedBlocksWriter) writeBundle() error {
	file := filename(w.lowBlockNum)
	w.logger.Info("writing merged file to store (suffix: .dbin.zst)", zap.String("filename", file), zap.Uint64("lowBlockNum", w.lowBlockNum))

	if len(w.blocks) == 0 {
		return fmt.Errorf("no blocks to write to bundle")
	}
	writeDone := make(chan struct{})

	pr, pw := io.Pipe()
	defer func() {
		pw.Close()
		<-writeDone
	}()

	go func() {
		err := w.store.WriteObject(context.Background(), file, pr)
		if err != nil {
			w.logger.Error("writing to store", zap.Error(err))
		}
		w.lowBlockNum += 100
		w.blocks = nil
		close(writeDone)
	}()

	blockWriter, err := w.writerFactory.New(pw)
	if err != nil {
		return err
	}

	for _, blk := range w.blocks {
		if err := blockWriter.Write(blk); err != nil {
			return err
		}
	}

	return err
}

func lowBoundary(i uint64) uint64 {
	return i - (i % 100)
}
