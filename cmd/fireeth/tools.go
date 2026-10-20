package main

import (
	"fmt"

	"github.com/spf13/cobra"
	pbbstream "github.com/streamingfast/bstream/pb/sf/bstream/v1"
	"github.com/streamingfast/cli"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"google.golang.org/protobuf/types/known/anypb"
)

// repackBlock returns a copy of block whose payload is ethBlock. ethBlock must keep the
// number, hash, parent hash and timestamp of the block it was decoded from.
func repackBlock(block *pbbstream.Block, ethBlock *pbeth.Block) (*pbbstream.Block, error) {
	payload, err := anypb.New(ethBlock)
	if err != nil {
		return nil, fmt.Errorf("re-packing the block: %w", err)
	}

	return &pbbstream.Block{
		Id:        block.Id,
		Number:    block.Number,
		ParentId:  block.ParentId,
		ParentNum: block.ParentNum,
		Timestamp: block.Timestamp,
		LibNum:    block.LibNum,
		Payload:   payload,
	}, nil
}

func examplePrefixed(prefix, examples string) string {
	return string(cli.ExamplePrefixed(prefix, examples))
}

func registerGroup(parent *cobra.Command, group cli.CommandOption) {
	group.Apply(parent)
}
