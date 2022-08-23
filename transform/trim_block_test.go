package transform

import (
	"testing"

	"github.com/streamingfast/bstream/transform"
	pbtransform "github.com/streamingfast/sf-ethereum/types/pb/sf/ethereum/transform/v1"
	pbeth "github.com/streamingfast/sf-ethereum/types/pb/sf/ethereum/type/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"
)

func lightBlockTransform(t *testing.T) *anypb.Any {
	transform := &pbtransform.LightBlock{}
	a, err := anypb.New(transform)
	require.NoError(t, err)
	return a
}

func TestBlockLight_Transform(t *testing.T) {
	transformReg := transform.NewRegistry()
	transformReg.Register(LightBlockFilterFactory)

	transforms := []*anypb.Any{lightBlockTransform(t)}

	preprocFunc, x, _, err := transformReg.BuildFromTransforms(transforms)
	require.NoError(t, err)
	require.Nil(t, x)

	blk := testBlockFromFiles(t, "block.json")

	output, err := preprocFunc(blk)
	require.NoError(t, err)

	pbcodecBlock := output.(*pbeth.Block)
	assert.Equal(t, blk.Number, pbcodecBlock.Number)
}
