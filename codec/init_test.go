package codec

import (
	// Imported for side-effects
	"github.com/streamingfast/firehose-ethereum/types"
	_ "github.com/streamingfast/firehose-ethereum/types"

	"github.com/streamingfast/logging"
)

var zlog, _ = logging.PackageLogger("fireeth", "github.com/streamingfast/firehose-ethereum/codec")

func init() {
	types.InitFireCore()
	logging.InstantiateLoggers()
}

type ObjectReader func() (interface{}, error)
