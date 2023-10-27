package codec

import (
	// Import for its side-effect (registering necessary bstream
	_ "github.com/streamingfast/firehose-ethereum/types"
	"github.com/streamingfast/logging"
)

var zlog, _ = logging.PackageLogger("fireeth", "github.com/streamingfast/firehose-ethereum/codec")

func init() {
	logging.InstantiateLoggers()
}

type ObjectReader func() (interface{}, error)
