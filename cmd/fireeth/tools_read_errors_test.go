package main

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/streamingfast/bstream"
	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	"github.com/streamingfast/dstore"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// writeCorruptBundle writes a merged blocks bundle named 0000000100 containing one
// valid block followed by a truncated dbin frame (frame length says 1000 bytes but
// only 3 are present), simulating a corrupt/truncated file.
func writeCorruptBundle(t *testing.T, ctx context.Context, store dstore.Store) {
	t.Helper()

	payload, err := anypb.New(&pbeth.Block{
		Hash:   []byte{0xaa},
		Number: 100,
		Header: &pbeth.BlockHeader{
			ParentHash: []byte{0xbb},
			Timestamp:  timestamppb.New(time.Unix(1700000000, 0)),
		},
	})
	require.NoError(t, err)

	buf := bytes.NewBuffer(nil)
	writer, err := bstream.NewDBinBlockWriter(buf)
	require.NoError(t, err)
	require.NoError(t, writer.Write(&pbbstream.Block{
		Id:        "aa",
		Number:    100,
		ParentId:  "bb",
		ParentNum: 99,
		LibNum:    99,
		Timestamp: timestamppb.New(time.Unix(1700000000, 0)),
		Payload:   payload,
	}))

	// truncated frame: length prefix claims 1000 bytes, only 3 follow
	buf.Write([]byte{0x00, 0x00, 0x03, 0xe8, 0x01, 0x02, 0x03})

	require.NoError(t, store.WriteObject(ctx, "0000000100", bytes.NewReader(buf.Bytes())))
}

func testCommandWithContext(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Flags().Uint64("merged-blocks-bundle-size", 100, "")
	cmd.SetContext(ctx)
	return cmd
}

func TestFixAnyTypeReturnsErrorOnCorruptBundle(t *testing.T) {
	ctx := context.Background()
	srcURL := "file://" + t.TempDir()
	dstURL := "file://" + t.TempDir()

	srcStore, err := dstore.NewDBinStore(srcURL)
	require.NoError(t, err)
	writeCorruptBundle(t, ctx, srcStore)

	err = createFixAnyTypeE(zap.NewNop())(testCommandWithContext(ctx), []string{srcURL, dstURL, "100", "200"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "reading block")
}

func TestFindUnknownStatusReturnsErrorOnCorruptBundle(t *testing.T) {
	ctx := context.Background()
	srcURL := "file://" + t.TempDir()
	dstURL := "file://" + t.TempDir()

	srcStore, err := dstore.NewDBinStore(srcURL)
	require.NoError(t, err)
	writeCorruptBundle(t, ctx, srcStore)

	err = scanForUnknownStatusE(zap.NewNop())(testCommandWithContext(ctx), []string{srcURL, dstURL, "100", "200"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "reading block")
}
