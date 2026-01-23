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

package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	jd "github.com/josephburnett/jd/lib"
	"github.com/spf13/cobra"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/bstream/blockstream"
	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	"github.com/streamingfast/cli"
	"github.com/streamingfast/eth-go/rpc"
	firecore "github.com/streamingfast/firehose-core"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

func addCompareLastPartialToFullBlockInStreamCmd(chain *firecore.Chain[*pbeth.Block], logger *zap.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compare-lastpartial-to-fullblock-in-stream <source> [<stop>]",
		Short: "Compare partial blocks (partialIndex==10) to full blocks (partialIndex==0)",
		Long: cli.Dedent(`
			Connects to a 'relayer' source and compares "last seen" partial blocks of a given number
			to full blocks with partialIndex==0 using proto.Equal().

			The connection is done over gRPC on the 'sf.bstream.v1.BlockStream/Blocks'
			endpoint of the source.

			For each full block received (partialIndex==0), it prints whether it matched
			the previous partialIndex==10, differs from it (with diff), or if the
			partialIndex==10 was not seen for that block.

			You can pass a stop block to the command, can be absolute or relative. If relative,
			only +N is supported, where N is the number of blocks seen so far. Default is
			to stream forever.
		`),
		Args: cobra.RangeArgs(1, 2),
		RunE: createRelayerStreamE(chain, logger),
		Example: examplePrefixed("fireeth tools compare-lastpartial-to-fullblock-in-stream", `
			# Compare blocks from relayer forever
			localhost:13011

			# Compare 100 blocks from relayer
			localhost:13011 +100

			# Compare until block 1000000
			localhost:13011 1000000
		`),
	}

	return cmd
}

func createRelayerStreamE(chain *firecore.Chain[*pbeth.Block], logger *zap.Logger) firecore.CommandExecutor {
	return func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		endpoint := args[0]

		stop := int64(-1)
		stopIsRelative := false

		if len(args) == 2 {
			input := args[1]

			var err error
			stop, err = strconv.ParseInt(strings.TrimPrefix(input, "+"), 10, 64)
			cli.NoError(err, "Invalid stop block number argument %q", args[1])

			stopIsRelative = strings.HasPrefix(input, "+")
		}

		blockCount := 0
		seenIndices := make(map[uint64][]int32) // blockNum -> list of partial indices seen
		seenBlocks := make(map[uint64]*pbeth.Block)

		source := blockstream.NewSource(
			ctx,
			endpoint,
			1,
			bstream.HandlerFunc(func(blk *pbbstream.Block, obj any) error {
				// Track all partial indices seen for each block
				if blk.PartialIndex != 0 {
					seenIndices[blk.Number] = append(seenIndices[blk.Number], blk.PartialIndex)

					ethBlock := &pbeth.Block{}
					if err := blk.Payload.UnmarshalTo(ethBlock); err != nil {
						return fmt.Errorf("unable to unmarshal block payload: %w", err)
					}
					seenBlocks[blk.Number] = ethBlock
					return nil
				}

				// Process full blocks (partialIndex == 0)
				if blk.PartialIndex == 0 {
					ethBlock := &pbeth.Block{}
					if err := blk.Payload.UnmarshalTo(ethBlock); err != nil {
						return fmt.Errorf("unable to unmarshal block payload: %w", err)
					}

					// Format seen indices
					seenIdxStr := ""
					if indices, ok := seenIndices[blk.Number]; ok && len(indices) > 0 {
						sort.Slice(indices, func(i, j int) bool { return indices[i] < indices[j] })
						idxStrings := make([]string, len(indices))
						for i, idx := range indices {
							idxStrings[i] = fmt.Sprintf("%d", idx)
						}
						seenIdxStr = fmt.Sprintf(" (seen idx: %s)", strings.Join(idxStrings, ", "))
					}

					// Check if we have a partial block to compare
					lastPartialBlock, ok := seenBlocks[blk.Number]
					if !ok {
						fmt.Printf("Block %d: No lastPartial seen for this block%s\n", blk.Number, seenIdxStr)
					} else {
						// Compare the blocks
						if proto.Equal(lastPartialBlock, ethBlock) {
							fmt.Printf("Block %d: ✓ MATCH - lastPartial matches full block%s\n", blk.Number, seenIdxStr)
						} else {
							fmt.Printf("Block %d: ✗ DIFFER - lastPartial differs from full block%s\n", blk.Number, seenIdxStr)

							if hex.EncodeToString(lastPartialBlock.Hash) != hex.EncodeToString(ethBlock.Hash) {
								fmt.Printf("Hashes differ: %s != %s\n", hex.EncodeToString(lastPartialBlock.Hash), hex.EncodeToString(ethBlock.Hash))
							} else {

								// Generate and print diff
								partial10JSON, err := rpc.MarshalJSONRPCIndent(lastPartialBlock, "", "  ")
								if err != nil {
									return fmt.Errorf("cannot marshal partial10 block to JSON: %w", err)
								}
								fullJSON, err := rpc.MarshalJSONRPCIndent(ethBlock, "", "  ")
								if err != nil {
									return fmt.Errorf("cannot marshal full block to JSON: %w", err)
								}

								partial10Doc, err := jd.ReadJsonString(string(partial10JSON))
								if err != nil {
									return fmt.Errorf("cannot read partial10 JSON: %w", err)
								}
								fullDoc, err := jd.ReadJsonString(string(fullJSON))
								if err != nil {
									return fmt.Errorf("cannot read full JSON: %w", err)
								}

								diff := partial10Doc.Diff(fullDoc)
								fmt.Printf("Diff (lastPartial -> full):\n%s\n", diff.Render())
							}
						}
					}

					// Clean up seen indices for this block
					delete(seenIndices, blk.Number)

					blockCount++

					// Continue forever if no stop provider
					if stop == -1 {
						return nil
					}

					if stopIsRelative && blockCount >= int(stop) {
						return bstream.ErrStopBlockReached
					}

					if !stopIsRelative && blk.Number >= uint64(stop) {
						return bstream.ErrStopBlockReached
					}
				}

				return nil
			}),
			blockstream.WithRequester("firecore tools"),
		)

		// Blocking
		source.Run()
		cli.Ensure(source.Err() == nil || errors.Is(source.Err(), bstream.ErrStopBlockReached), "Source run failed: %s", source.Err())

		return nil
	}
}
