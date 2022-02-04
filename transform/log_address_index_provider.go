package transform

import (
	"github.com/streamingfast/dstore"
	"github.com/streamingfast/eth-go"
	"io"
)

//type IndexProvider interface {
//	WithinRange(blockNum uint64) bool
//	Matches(blockNum uint64) bool
//	NextMatching(blockNum uint64) (num uint64, done bool) // when done is true, the returned blocknum is the first Unindexed block
//}

// LogAddressIndexProvider responds to queries about LogAddressIndexes
type LogAddressIndexProvider struct {
	store                 dstore.Store
	filterAddresses       []eth.Address
	filterEventSigs       []eth.Hash
	currentIndex          *LogAddressIndex
	currentMatchingBlocks []uint64
	noMoreIndexes         bool
}

func (ip *LogAddressIndexProvider) WithinRange(blockNum uint64) bool {
	return true
}

func (ip *LogAddressIndexProvider) Matches(blockNum uint64) bool {
	return true
}

func (ip *LogAddressIndexProvider) NextMatching(blockNum uint64) (num uint64, done bool) {
	return 0, ip.noMoreIndexes
}

// loadRangesUntilMatch will traverse available indexes until it finds the next block
func (ip *LogAddressIndexProvider) loadRangesUntilMatch() {
	// truncate any prior matching blocks
	ip.currentMatchingBlocks = []uint64{}

	for {
		if len(ip.currentMatchingBlocks) != 0 {
			zlog.Error("currentMatchingBlocks should be empty, bailing")
			return
		}

		if ip.currentIndex == nil {
			zlog.Error("currentIndex is nil, bailing")
			return
		}

		next := ip.currentIndex.lowBlockNum + ip.currentIndex.indexSize

		r := ip.findIndexContaining(next)
		if r == nil {
			return
		}

		if err := ip.loadIndex(r); err != nil {
			ip.noMoreIndexes = true
			return
		}
	}
}

// findIndexContaining tries to find an index file in dstore containing the provided blockNum
// if such a file exists, return an io.Reader; nil otherwise
func (ip *LogAddressIndexProvider) findIndexContaining(blockNum uint64) io.Reader {
	return nil
}

// loadIndex will populate the indexProvider's currentIndex and currentMatchingBlocks
// from the indexFile found in dstore
func (ip *LogAddressIndexProvider) loadIndex(r io.Reader) error {
	return nil
}
