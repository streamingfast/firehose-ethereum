package codec

import (
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"testing"

	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	firecore "github.com/streamingfast/firehose-core"
	"github.com/streamingfast/firehose-core/node-manager/consoleline"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

func TestConsoleReader_DecodedBlockLinesMatchTextLines(t *testing.T) {
	tests := []struct {
		name     string
		initLine string
		partial  bool
	}{
		{"protocol 3.0", "FIRE INIT 3.0 geth 1.14.0-fh3.0", false},
		{"protocol 3.1", "FIRE INIT 3.1 geth 1.14.0-fh3.1", true},
		{"protocol 1.0 read as 3.0", "FIRE INIT 1.0 poller v1.0.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := []string{"some node log line", tt.initLine}
			for num := uint64(10); num < 13; num++ {
				lines = append(lines, "INFO unrelated", testFireBlockLine(t, num, tt.partial))
			}
			output := strings.Join(lines, "\n") + "\n"

			fromText, err := readBlocksFromText(output)
			require.NoError(t, err)
			require.Len(t, fromText, 3)

			fromDecoded, decodedCount, err := readBlocksFromDecodedLines(output)
			require.NoError(t, err)
			assert.Equal(t, 3, decodedCount, "every block line should have been decoded by the splitter")

			require.Len(t, fromDecoded, len(fromText))
			for i := range fromText {
				assert.True(t, proto.Equal(fromText[i], fromDecoded[i]), "block %d differs:\ntext:    %v\ndecoded: %v", i, fromText[i], fromDecoded[i])
			}

			if tt.partial {
				assert.Equal(t, int32(2), fromDecoded[0].PartialIndex)
				assert.True(t, fromDecoded[0].LastPartial)
			}
		})
	}
}

func TestConsoleReader_DecodedBlockLineInvalidPayload(t *testing.T) {
	output := "FIRE INIT 3.0 geth 1.14.0-fh3.0\nFIRE BLOCK 10 aa 9 bb 9 1000 not*base64\n"

	_, decodedCount, err := readBlocksFromDecodedLines(output)
	require.Error(t, err)
	assert.Equal(t, 1, decodedCount)
	assert.Contains(t, err.Error(), "decoding base64 block payload")
	assert.Contains(t, err.Error(), `line header "10 aa 9 bb 9 1000"`)
}

func testFireBlockLine(t *testing.T, num uint64, partial bool) string {
	t.Helper()

	payload, err := proto.Marshal(&pbeth.Block{Number: num, Hash: []byte{byte(num)}, Ver: 3})
	require.NoError(t, err)

	partialField := ""
	if partial {
		partialField = " 1002"
	}

	return fmt.Sprintf("FIRE BLOCK %d%s %x %d %x %d %d %s",
		num, partialField, []byte{byte(num)}, num-1, []byte{byte(num - 1)}, num-2, 1_700_000_000_000_000_000+num,
		base64.StdEncoding.EncodeToString(payload),
	)
}

func readBlocksFromText(output string) ([]*pbbstream.Block, error) {
	lines := make(chan string, 100)
	for _, line := range strings.Split(strings.TrimSuffix(output, "\n"), "\n") {
		lines <- line
	}
	close(lines)

	return readAllBlocks(lines, nil)
}

// readBlocksFromDecodedLines splits output the way the reader node does when its console
// reader implements ReadLines, so block payloads are decoded while they are read.
func readBlocksFromDecodedLines(output string) (blocks []*pbbstream.Block, decodedCount int, err error) {
	consoleLines := make(chan consoleline.Line, 100)
	splitter := consoleline.NewSplitter(0,
		func(line string) { consoleLines <- consoleline.Line{Text: line} },
		func(block *consoleline.Block) {
			decodedCount++
			consoleLines <- consoleline.Line{Block: block}
		},
	)
	if _, err := io.Copy(splitter, strings.NewReader(output)); err != nil {
		return nil, 0, err
	}
	close(consoleLines)

	blocks, err = readAllBlocks(make(chan string), consoleLines)
	return blocks, decodedCount, err
}

func readAllBlocks(lines chan string, consoleLines <-chan consoleline.Line) ([]*pbbstream.Block, error) {
	consoleReader, err := NewConsoleReader(lines, firecore.NewBlockEncoder(), zap.NewNop(), nil)
	if err != nil {
		return nil, err
	}

	reader := consoleReader.(*ConsoleReader)
	defer reader.Close()

	if consoleLines != nil {
		reader.ReadLines(consoleLines)
	}

	var out []*pbbstream.Block
	for {
		block, err := reader.ReadBlock()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return out, err
		}

		out = append(out, block)
	}
}
