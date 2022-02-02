package transform

import (
	"fmt"
	pbtransforms "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/transforms/v1"
)

func lowBoundary(i uint64, mod uint64) uint64 {
	return i - (i % mod)
}

func toIndexFilename(bundleSize, baseBlockNum uint64, shortname string) string {
	return fmt.Sprintf("%010d.%d.%s.idx", baseBlockNum, bundleSize, shortname)
}

func NewLogAddressSignatureIndex() *pbtransforms.LogAddressSignatureIndex {
	return &pbtransforms.LogAddressSignatureIndex{
		Addresses:       []*pbtransforms.KeyToBitmap{},
		EventSignatures: []*pbtransforms.KeyToBitmap{},
	}
}
