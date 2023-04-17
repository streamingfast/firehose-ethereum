package tools

import (
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/firehose-ethereum/types"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	sftools "github.com/streamingfast/sf-tools"
	"go.uber.org/zap"
)

var logUpgrade sync.Once

func init() {
	UpgradeMergedBlocksCmd.Flags().String("variant", "", "Shortname of the geth variant (polygon, bnb, geth), to apply specific upgrade logic")
	Cmd.AddCommand(UpgradeMergedBlocksCmd)
}

var UpgradeMergedBlocksCmd = sftools.GetMergedBlocksUpgrader(zlog, tracer, normalize)

func normalize(cmd *cobra.Command, in *bstream.Block) (*bstream.Block, error) {
	block := in.ToProtocol().(*pbeth.Block)

	prevVersion := block.Ver
	// this is a hack for a previous incomplete normalize call regarding nested DELEGATE calls. the v2->v3 upgrade is idempotent
	if block.Ver == 3 {
		block.Ver = 2
	}

	variant := pbeth.VariantUnset

	variantStr := strings.ToLower(mustGetString(cmd, "variant"))
	switch variantStr {
	case "polygon":
		variant = pbeth.VariantPolygon
	case "geth":
		variant = pbeth.VariantGeth
	case "bnb", "bsc":
		variant = pbeth.VariantBNB
	default:
		zlog.Warn("unknown pbeth variant to this tool, ignoring variant-specific upgrades", zap.String("given value", variantStr))
	}

	block.NormalizeInPlace(variant)
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
