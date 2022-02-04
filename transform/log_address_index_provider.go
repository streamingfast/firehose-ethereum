package transform

import (
	"github.com/streamingfast/dstore"
	"github.com/streamingfast/eth-go"
)

//type IndexProvider interface {
//	WithinRange(blockNum uint64) bool
//	Matches(blockNum uint64) bool
//	NextMatching(blockNum uint64) (num uint64, done bool) // when done is true, the returned blocknum is the first Unindexed block
//}

type LogAddressIndexProvider struct {
	store                 dstore.Store
	filterAddresses       []eth.Address
	filterEventSigs       []eth.Hash
	currentIndex          *LogAddressIndex
	currentMatchingBlocks []uint64
}

func (ip *LogAddressIndexProvider) WithinRange(blockNum uint64) bool {
	return true
}

func (ip *LogAddressIndexProvider) Matches(blockNum uint64) bool {
	return true
}

func (ip *LogAddressIndexProvider) NextMatching(blockNum uint64) (num uint64, done bool) {
	return 0, true
}
