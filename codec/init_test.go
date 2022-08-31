package codec

import "github.com/streamingfast/logging"

var zlog, _ = logging.PackageLogger("fireeth", "github.com/streamingfast/sf-ethereum/node-mananager/codec")

func init() {
	logging.InstantiateLoggers()
}

type ObjectReader func() (interface{}, error)
