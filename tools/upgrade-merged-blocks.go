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
	// this is a hack for a previous incomplete normalize call regarding nested DELEGATE calls. the v2->v3 upgrade is idempotent
	if block.Ver == 3 {
		block.Ver = 2
	}
	block.NormalizeInPlace()
	logUpgrade.Do(func() {
		if prevVersion == block.Ver {
			if prevVersion == 3 {
				zlog.Info("Re-applying normalize version 2->3")
			} else {
				zlog.Error("nothing to do on first block: previous version is the same as available version", zap.Int32("version", prevVersion), zap.Uint64("first_block", block.Number))
			}
		}
		zlog.Info("Upgrading ethereum blocks",
			zap.Int32("previous_version", prevVersion),
			zap.Int32("dest_version", block.Ver),
		)
	})
	return types.BlockFromProto(block, in.LibNum)
}
