// Copyright 2021 dfuse Platform Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package codec

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/streamingfast/jsonpb"
	"github.com/streamingfast/logging"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/codec/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	logging.TestingOverride()
}

type ObjectReader func() (interface{}, error)

func TestParseFromFile(t *testing.T) {
	tests := []struct {
		deepMindFile     string
		expectedPanicErr error
		readTransaction  bool
	}{
		{"testdata/deep-mind.dmlog", nil, false},
		{"testdata/normalize-r-and-s-curve-points.dmlog", nil, false},
		{"testdata/block_mining_rewards.dmlog", nil, false},
		{"testdata/block_unknown_balance_change.dmlog", errors.New(`receive unknown balance change reason, received reason string is "something_that_will_never_match"`), false},
		{"testdata/read_transaction.dmlog", nil, true},
		{"testdata/polygon_calls_after_finalize.dmlog", nil, false},
		{"testdata/polygon_add_log_0.dmlog", nil, false},
		{
			deepMindFile:     "testdata/lachesis.dmlog",
			expectedPanicErr: nil,
			readTransaction:  false,
		},
	}

	for _, test := range tests {
		t.Run(strings.Replace(test.deepMindFile, "testdata/", "", 1), func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					require.Equal(t, test.expectedPanicErr, r)
				}
			}()

			cr := testFileConsoleReader(t, test.deepMindFile)
			buf := &bytes.Buffer{}
			buf.Write([]byte("["))
			first := true

			for {
				var reader ObjectReader = cr.Read
				if test.readTransaction {
					reader = func() (interface{}, error) {
						return cr.ReadTransaction()
					}
				}

				out, err := reader()
				if v, ok := out.(proto.Message); ok && !isNil(v) {
					if !first {
						buf.Write([]byte(","))
					}
					first = false

					value, err := jsonpb.MarshalIndentToString(v, "  ")
					require.NoError(t, err)

					buf.Write([]byte(value))
				}

				if err == io.EOF {
					break
				}

				if len(buf.Bytes()) != 0 {
					buf.Write([]byte("\n"))
				}

				require.NoError(t, err)
			}
			buf.Write([]byte("]"))

			goldenFile := test.deepMindFile + ".golden.json"
			if os.Getenv("GOLDEN_UPDATE") == "true" {
				ioutil.WriteFile(goldenFile, buf.Bytes(), os.ModePerm)
			}

			cnt, err := ioutil.ReadFile(goldenFile)
			require.NoError(t, err)

			if !assert.Equal(t, string(cnt), buf.String()) {
				t.Error("previous diff:\n" + unifiedDiff(t, cnt, buf.Bytes()))
			}
		})
	}
}

func isNil(v interface{}) bool {
	if v == nil {
		return true
	}

	rv := reflect.ValueOf(v)
	return rv.Kind() == reflect.Ptr && rv.IsNil()
}

func TestGeneratePBBlocks(t *testing.T) {
	t.Skip("generate only when deep-mind.dmlog changes")

	cr := testFileConsoleReader(t, "testdata/deep-mind.dmlog")

	for {
		out, err := cr.Read()
		if out != nil && out.(*pbcodec.Block) != nil {
			dethBlock := out.(*pbcodec.Block)
			bstreamBlock, err := BlockFromProto(dethBlock)
			require.NoError(t, err)

			pbBlock, err := bstreamBlock.ToProto()
			require.NoError(t, err)

			outputFile, err := os.Create(fmt.Sprintf("testdata/pbblocks/battlefield-block.%d.pb", dethBlock.Number))
			require.NoError(t, err)

			bytes, err := proto.Marshal(pbBlock)
			require.NoError(t, err)

			_, err = outputFile.Write(bytes)
			require.NoError(t, err)

			outputFile.Close()
		}

		if err == io.EOF {
			break
		}

		require.NoError(t, err)
	}
}

func consumeBlock(t *testing.T, reader *ConsoleReader) *pbcodec.Block {
	t.Helper()

	block, err := reader.Read()
	if block == nil || block.(*pbcodec.Block) == nil {
		require.Fail(t, err.Error())
	}

	return block.(*pbcodec.Block)
}

func consumeSingleBlock(t *testing.T, reader *ConsoleReader) *pbcodec.Block {
	t.Helper()

	block := consumeBlock(t, reader)
	consumeToEOF(t, reader)

	return block
}

func consumeToEOF(t *testing.T, reader *ConsoleReader) {
	block, err := reader.Read()
	require.Nil(t, block)
	require.Equal(t, err, io.EOF)

	return
}

func testFileConsoleReader(t *testing.T, filename string) *ConsoleReader {
	t.Helper()

	fl, err := os.Open(filename)
	require.NoError(t, err)

	cr := testReaderConsoleReader(t, make(chan string, 10000), func() { fl.Close() })

	go cr.ProcessData(fl)

	return cr
}

//func testStringConsoleReader(t *testing.T, data string) *ConsoleReader {
//	t.Helper()
//
//	return testReaderConsoleReader(t, bytes.NewBufferString(data), func() {})
//}

func testReaderConsoleReader(t *testing.T, lines chan string, closer func()) *ConsoleReader {
	t.Helper()

	l := &ConsoleReader{
		lines: lines,
		close: closer,
		ctx:   &parseCtx{},
	}

	return l
}

func wrapIntoTrxAndBlock(logLines ...string) string {
	var lines []string

	lines = append(lines,
		"DMLOG BEGIN_BLOCK 1",
		"DMLOG BEGIN_APPLY_TRX 00 0000000000000000000000000000000000000002 0 0 0 0 1 1 0 .",
		"DMLOG TRX_FROM 0000000000000000000000000000000000000001",
	)

	lines = append(lines, logLines...)

	lines = append(lines,
		"DMLOG END_APPLY_TRX 0 00 0 00 []",
		"DMLOG FINALIZE_BLOCK 1",
		`DMLOG END_BLOCK 1 75 {"header":{"parentHash":"0x516c52b54a987de8c84b75473121289016815af11be0de4c3866782aad5da0de","sha3Uncles":"0x329e27a3918e236ec90f316843571e8a63f7856b0bce3245dcbe29bc24ce8612","miner":"0x0000000000000000000000000000000000000000","stateRoot":"0x859e6de662a49383a9ffca14c34cf9a9ba29ed5bb104dce462096f00dcda3bb7","transactionsRoot":"0x8e37940b3bbbedca3d683a3b98e6521fa98d3339afafaf4adb2bc811c9a351bf","receiptsRoot":"0x505ea764be1086185661f20548c503e9a313b506c53f87d02ebf0a5314e82605","logsBloom":"0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002000000000100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000080000000000000000000000000000000000000000040000000000000000000000000000000000000000000000000000000000000000000000000000040000000000000000008000000000000","difficulty":"0x0","number":"0x1","gasUsed":"0x0","gasLimit":"0x0","timestamp":"0x5d43a215","extraData":""}}`,
	)

	return strings.Join(lines, "\n")
}

func TestValueParsing(t *testing.T) {
	testValue := "deff"
	expectedValue := &pbcodec.BigInt{
		Bytes: big.NewInt(int64(57087)).Bytes(),
	}
	value := pbcodec.BigIntFromBytes(FromHex(testValue, "TESTING value"))
	require.Equal(t, expectedValue, value)

}

func bytesListToHexList(bytesList [][]byte) []string {
	hexes := make([]string, len(bytesList))
	for i, bytes := range bytesList {
		hexes[i] = hex.EncodeToString(bytes)
	}

	return hexes
}

func unifiedDiff(t *testing.T, cnt1, cnt2 []byte) string {
	file1 := "/tmp/gotests-linediff-1"
	file2 := "/tmp/gotests-linediff-2"
	err := ioutil.WriteFile(file1, cnt1, 0600)
	require.NoError(t, err)

	err = ioutil.WriteFile(file2, cnt2, 0600)
	require.NoError(t, err)

	cmd := exec.Command("diff", "-u", file1, file2)
	out, _ := cmd.Output()

	return string(out)
}
