package tools

import (
	"sync"

	"github.com/streamingfast/bstream"
	"github.com/streamingfast/firehose-ethereum/types"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	sftools "github.com/streamingfast/sf-tools"
	"go.uber.org/zap"
)

var logUpgrade sync.Once

func init() {
	Cmd.AddCommand(UpgradeMergedBlocksCmd)
}

var UpgradeMergedBlocksCmd = sftools.GetMergedBlocksUpgrader(zlog, tracer, normalize)

func normalize(in *bstream.Block) (*bstream.Block, error) {
	block := in.ToProtocol().(*pbeth.Block)

	prevVersion := block.Ver
	block.NormalizeInPlace()
	logUpgrade.Do(func() {
		if prevVersion == block.Ver {
			zlog.Error("nothing to do: previous version is the same as available version", zap.Int32("version", prevVersion))
		}
		zlog.Info("Upgrading ethereum blocks",
			zap.Int32("previous_version", prevVersion),
			zap.Int32("dest_version", block.Ver),
		)
	})
	return types.BlockFromProto(block, in.LibNum)
}
