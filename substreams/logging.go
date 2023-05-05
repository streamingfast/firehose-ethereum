package substreams

import (
	"github.com/streamingfast/logging"
	"go.uber.org/zap"
)

var zlog *zap.Logger
var tracer logging.Tracer

func init() {
	zlog, tracer = logging.PackageLogger("rpc-cache", "github.com/firehose-ethereum/substreams")
}
