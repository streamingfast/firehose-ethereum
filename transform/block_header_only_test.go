package transform

import (
	"testing"

	"github.com/streamingfast/bstream/transform"
	pbtransform "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/transform/v1"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"
)

func headerOnlyTransform(t *testing.T) *anypb.Any {
	transform := &pbtransform.HeaderOnly{}
	a, err := anypb.New(transform)
	require.NoError(t, err)
	return a
}

func TestHeaderOnly_Transform(t *testing.T) {
	transformReg := transform.NewRegistry()
	transformReg.Register(HeaderOnlyTransformFactory)

	transforms := []*anypb.Any{headerOnlyTransform(t)}

	preprocFunc, x, _, err := transformReg.BuildFromTransforms(transforms)
	require.NoError(t, err)
	require.Nil(t, x)

	blk := testBlockFromFiles(t, "block.json")
	block := blk.ToProtocol().(*pbeth.Block)

	output, err := preprocFunc(blk)
	require.NoError(t, err)

	pbcodecBlock := output.(*pbeth.Block)
	assert.Equal(t, block.Ver, pbcodecBlock.Ver)
	assert.Equal(t, block.Hash, pbcodecBlock.Hash)
	assert.Equal(t, block.Number, pbcodecBlock.Number)
	assert.Equal(t, block.Size, pbcodecBlock.Size)
	assertProtoEqual(t, block.Header, pbcodecBlock.Header)
	assert.Nil(t, pbcodecBlock.Uncles)
	assert.Nil(t, pbcodecBlock.TransactionTraces)
	assert.Nil(t, pbcodecBlock.BalanceChanges)
	assert.Nil(t, pbcodecBlock.CodeChanges)
}
