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

package tools

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"sync"

	jd "github.com/josephburnett/jd/lib"
	"github.com/spf13/cobra"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/cli"
	"github.com/streamingfast/dstore"
	"github.com/streamingfast/eth-go/rpc"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	sftools "github.com/streamingfast/sf-tools"
	"go.uber.org/multierr"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var compareBlocksCmd = &cobra.Command{
	Use:   "compare-blocks <reference_blocks_store> <current_blocks_store> [<block_range>]",
	Short: "Checks for any differences between two block stores between a specified range. (To compare the likeness of two block ranges, for example)",
	Long: cli.Dedent(`
		The 'compare-blocks' takes in two paths to stores of merged blocks and a range specifying the blocks you
		want to compare, written as: '<start>:<finish>'. It will output the status of the likeness of every
		100,000 blocks, on completion, or on encountering a difference. Increments that contain a difference will
		be communicated as well as the blocks within that contain differences. Increments that do not have any
		differences will be outputted as identical.

		After passing through the blocks, it will output instructions on how to locate a specific difference
		based on the blocks that were given. This is done by applying the '--diff' flag before your args.

		Commands inputted with '--diff' will display the blocks that have differences, as well as the
		difference.
	`),
	Args: cobra.ExactArgs(3),
	RunE: compareBlocksE,
	Example: ExamplePrefixed("fireeth tools compare-blocks", `
		# Run over full block range
		reference_store/ current_store/ 0:16000000

		# Run over specific block range, displaying differences in blocks
		--diff reference_store/ current_store/ 100:200
	`),
}

func init() {
	Cmd.AddCommand(compareBlocksCmd)
	compareBlocksCmd.PersistentFlags().Bool("diff", false, "When activated, difference is displayed for each block with a difference")
	compareBlocksCmd.PersistentFlags().Bool("include-unknown-fields", false, "When activated, the 'unknown fields' in the protobuf message will also be compared. These would not generate any difference when unmarshalled with the current protobuf definition.")
}

func sanitizeBlock(block *pbeth.Block) *pbeth.Block {
	for _, trxTrace := range block.TransactionTraces {
		for _, call := range trxTrace.Calls {
			if call.FailureReason != "" {
				call.FailureReason = "(varying field)"
			}
		}
	}

	return block
}

func readBundle(ctx context.Context, filename string, store dstore.Store, stopBlock uint64) ([]string, map[string]*pbeth.Block, error) {

	fileReader, err := store.OpenObject(ctx, filename)
	if err != nil {
		return nil, nil, fmt.Errorf("creating reader: %w", err)
	}

	blockReader, err := bstream.GetBlockReaderFactory.New(fileReader)
	if err != nil {
		return nil, nil, fmt.Errorf("creating block reader: %w", err)
	}

	var blockHashes []string
	blocksMap := make(map[string]*pbeth.Block)
	for {
		curBlock, err := blockReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("reading blocks: %w", err)
		}
		if curBlock.Number >= stopBlock {
			break
		}

		curBlockPB := sanitizeBlock(curBlock.ToProtocol().(*pbeth.Block))
		blockHashes = append(blockHashes, string(curBlockPB.Hash))
		blocksMap[string(curBlockPB.Hash)] = curBlockPB
	}

	return blockHashes, blocksMap, nil
}

func compareBlocksE(cmd *cobra.Command, args []string) error {
	displayDiff := mustGetBool(cmd, "diff")
	ignoreUnknown := !mustGetBool(cmd, "include-unknown-fields")
	segmentSize := uint64(100000)

	ctx := cmd.Context()
	blockRange, err := bstream.ParseRange(args[2])
	if err != nil {
		return fmt.Errorf("parsing range: %w", err)
	}
	blockRangeSize, err := blockRange.Size()
	if err != nil {
		return fmt.Errorf("checking for valid range: %w", err)
	}
	if blockRangeSize == 0 {
		return fmt.Errorf("invalid block range")
	}

	stopBlock := *blockRange.EndBlock()
	blockRangePrefix := sftools.WalkBlockPrefix(sftools.BlockRange{
		Start: blockRange.StartBlock(),
		Stop:  stopBlock,
	}, 100)

	// Create stores
	storeReference, err := dstore.NewDBinStore(args[0])
	if err != nil {
		return fmt.Errorf("unable to create store at path %q: %w", args[0], err)
	}
	storeCurrent, err := dstore.NewDBinStore(args[1])
	if err != nil {
		return fmt.Errorf("unable to create store at path %q: %w", args[1], err)
	}

	segments, err := blockRange.Split(segmentSize)
	if err != nil {
		return fmt.Errorf("unable to split blockrage in segments: %w", err)
	}
	processState := &state{
		segments: segments,
	}

	err = storeReference.Walk(ctx, blockRangePrefix, func(filename string) (err error) {
		fileStartBlock, err := strconv.Atoi(filename)
		if err != nil {
			return fmt.Errorf("parsing filename: %w", err)
		}

		// If reached end of range
		if *blockRange.EndBlock() <= uint64(fileStartBlock) {
			return dstore.StopIteration
		}

		if blockRange.Contains(uint64(fileStartBlock)) {
			var wg sync.WaitGroup
			var bundleErrLock sync.Mutex
			var bundleReadErr error
			var referenceBlockHashes []string
			var referenceBlocks map[string]*pbeth.Block
			var currentBlocks map[string]*pbeth.Block

			wg.Add(1)
			go func() {
				defer wg.Done()
				referenceBlockHashes, referenceBlocks, err = readBundle(ctx, filename, storeReference, stopBlock)
				if err != nil {
					bundleErrLock.Lock()
					bundleReadErr = multierr.Append(bundleReadErr, err)
					bundleErrLock.Unlock()
				}
			}()

			wg.Add(1)
			go func() {
				defer wg.Done()
				_, currentBlocks, err = readBundle(ctx, filename, storeCurrent, stopBlock)
				if err != nil {
					bundleErrLock.Lock()
					bundleReadErr = multierr.Append(bundleReadErr, err)
					bundleErrLock.Unlock()
				}
			}()
			wg.Wait()
			if bundleReadErr != nil {
				return fmt.Errorf("reading bundles: %w", bundleReadErr)
			}

			for _, referenceBlockHash := range referenceBlockHashes {
				referenceBlock := referenceBlocks[referenceBlockHash]
				currentBlock, existsInCurrent := currentBlocks[referenceBlockHash]

				var isEqual bool
				if existsInCurrent {
					var differences []string
					isEqual, differences = compare(referenceBlock, currentBlock, ignoreUnknown)
					if !isEqual {
						fmt.Printf("- Block (%s) is different\n", referenceBlock.AsRef())
						if displayDiff {
							for _, diff := range differences {
								fmt.Println("  · ", diff)
							}
						}
					}
				}
				processState.process(referenceBlock.Number, !isEqual, !existsInCurrent)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("walking files: %w", err)
	}
	processState.print()

	return nil
}

type state struct {
	segments                   []*bstream.Range
	currentSegmentIdx          int
	blocksCountedInThisSegment int
	differencesFound           int
	missingBlocks              int
	totalBlocksCounted         int
}

func (s *state) process(blockNum uint64, isDifferent bool, isMissing bool) {
	if !s.segments[s.currentSegmentIdx].Contains(blockNum) { // moving forward
		s.print()
		for i := s.currentSegmentIdx; i < len(s.segments); i++ {
			if s.segments[i].Contains(blockNum) {
				s.currentSegmentIdx = i
				s.totalBlocksCounted += s.blocksCountedInThisSegment
				s.differencesFound = 0
				s.missingBlocks = 0
				s.blocksCountedInThisSegment = 0
			}
		}
	}

	s.totalBlocksCounted++
	if isMissing {
		s.missingBlocks++
	} else if isDifferent {
		s.differencesFound++
	}

}

func (s *state) print() {
	endBlock := "∞"
	if end := s.segments[s.currentSegmentIdx].EndBlock(); end != nil {
		endBlock = fmt.Sprintf("%d", *end)
	}
	if s.totalBlocksCounted == 0 {
		fmt.Printf("✖ No blocks were found at all for segment %d - %s\n", s.segments[s.currentSegmentIdx].StartBlock(), endBlock)
		return
	}

	if s.differencesFound == 0 && s.missingBlocks == 0 {
		fmt.Printf("✓ Segment %d - %s has no differences (%d blocks counted)\n", s.segments[s.currentSegmentIdx].StartBlock(), endBlock, s.totalBlocksCounted)
		return
	}

	if s.differencesFound == 0 && s.missingBlocks == 0 {
		fmt.Printf("✓~ Segment %d - %s has no differences but does have %d missing blocks (%d blocks counted)\n", s.segments[s.currentSegmentIdx].StartBlock(), endBlock, s.missingBlocks, s.totalBlocksCounted)
		return
	}

	fmt.Printf("✖ Segment %d - %s has %d different blocks and %d missing blocks (%d blocks counted)\n", s.segments[s.currentSegmentIdx].StartBlock(), endBlock, s.differencesFound, s.missingBlocks, s.totalBlocksCounted)
}

func compare(reference, current *pbeth.Block, ignoreUnknown bool) (isEqual bool, differences []string) {
	if reference == nil && current == nil {
		return true, nil
	}
	if reflect.TypeOf(reference).Kind() == reflect.Ptr && reference == current {
		return true, nil
	}

	referenceMsg := reference.ProtoReflect()
	currentMsg := current.ProtoReflect()
	if referenceMsg.IsValid() && !currentMsg.IsValid() {
		return false, []string{fmt.Sprintf("reference block is valid protobuf message, but current block is invalid")}
	}
	if !referenceMsg.IsValid() && currentMsg.IsValid() {
		return false, []string{fmt.Sprintf("reference block is invalid protobuf message, but current block is valid")}
	}

	if ignoreUnknown {
		referenceMsg.SetUnknown(nil)
		currentMsg.SetUnknown(nil)
		reference = referenceMsg.Interface().(*pbeth.Block)
		current = currentMsg.Interface().(*pbeth.Block)
	} else {
		x := referenceMsg.GetUnknown()
		y := currentMsg.GetUnknown()

		if !bytes.Equal(x, y) {
			// from https://github.com/protocolbuffers/protobuf-go/tree/v1.28.1/proto
			mx := make(map[protoreflect.FieldNumber]protoreflect.RawFields)
			my := make(map[protoreflect.FieldNumber]protoreflect.RawFields)
			for len(x) > 0 {
				fnum, _, n := protowire.ConsumeField(x)
				mx[fnum] = append(mx[fnum], x[:n]...)
				x = x[n:]
			}
			for len(y) > 0 {
				fnum, _, n := protowire.ConsumeField(y)
				my[fnum] = append(my[fnum], y[:n]...)
				y = y[n:]
			}
			for k, v := range mx {
				vv, ok := my[k]
				if !ok {
					differences = append(differences, fmt.Sprintf("reference block contains unknown protobuf field number %d (%x), but current block does not", k, v))
					continue
				}
				if !bytes.Equal(v, vv) {
					differences = append(differences, fmt.Sprintf("unknown protobuf field number %d has different values. Reference: %x, current: %x", k, v, vv))
				}
			}
			for k := range my {
				v, ok := my[k]
				if !ok {
					differences = append(differences, fmt.Sprintf("current block contains unknown protobuf field number %d (%x), but reference block does not", k, v))
					continue
				}
			}
		}
	}

	if !proto.Equal(reference, current) {
		ref, err := rpc.MarshalJSONRPCIndent(reference, "", " ")
		mustNoError(err)
		cur, err := rpc.MarshalJSONRPCIndent(current, "", " ")
		mustNoError(err)
		r, err := jd.ReadJsonString(string(ref))
		mustNoError(err)
		c, err := jd.ReadJsonString(string(cur))
		mustNoError(err)

		if diff := r.Diff(c).Render(); diff != "" {
			differences = append(differences, diff)
		}
		return false, differences
	}
	return true, nil
}

func mustNoError(err error) {
	if err != nil {
		panic(err)
	}
}
