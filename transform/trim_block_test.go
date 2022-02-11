package transform

import (
	"testing"

	"github.com/streamingfast/bstream/transform"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/codec/v1"
	pbtransforms "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/transforms/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"
)

func lightBlockTransform(t *testing.T) *anypb.Any {
	transform := &pbtransforms.LightBlock{}
	a, err := anypb.New(transform)
	require.NoError(t, err)
	return a
}

func TestBlockLight_Transform(t *testing.T) {
	transformReg := transform.NewRegistry()
	transformReg.Register(LightBlockFilterFactory)

	transforms := []*anypb.Any{lightBlockTransform(t)}

	preprocFunc, x, err := transformReg.BuildFromTransforms(transforms)
	require.NoError(t, err)
	require.Nil(t, x)

	blk := testBlockFromFiles(t, "block.json")

	output, err := preprocFunc(blk)
	require.NoError(t, err)

	pbcodecBlock := output.(*pbcodec.Block)
	assert.Equal(t, blk.Number, pbcodecBlock.Number)
}
