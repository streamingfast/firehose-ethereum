package tools

import (
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/firehose-ethereum/types"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	sftools "github.com/streamingfast/sf-tools"
)

func init() {
	Cmd.AddCommand(NormalizeMergedBlocksCmd)
}

var NormalizeMergedBlocksCmd = sftools.GetMergedBlocksNormalizer(zlog, tracer, normalize)

func normalize(in *bstream.Block) (*bstream.Block, error) {
	block := in.ToProtocol().(*pbeth.Block)
	block.NormalizeInPlace()
	return types.BlockFromProto(block, in.LibNum)
}
