package transform

import (
	"context"
	"fmt"
	"github.com/pingcap/log"
	"github.com/streamingfast/dstore"
	"github.com/streamingfast/eth-go"
	"go.uber.org/zap"
	"io"
	"io/ioutil"
	"time"
)

const LogAddressIdxShortname string = "logaddr"

//type IndexProvider interface {
//	WithinRange(blockNum uint64) bool
//	Matches(blockNum uint64) bool
//	NextMatching(blockNum uint64) (num uint64, done bool) // when done is true, the returned blocknum is the first Unindexed block
//}

// LogAddressIndexProvider responds to queries about LogAddressIndexes
type LogAddressIndexProvider struct {
	store           dstore.Store
	indexOpsTimeout time.Duration

	currentIndex *LogAddressIndex

	currentMatchingBlocks []uint64
	filterAddresses       []eth.Address
	filterEventSigs       []eth.Hash
	possibleIndexSizes    []uint64
}

func NewLogAddressIndexProvider(
	store dstore.Store,
	lowBlockNum uint64,
	indexSize uint64,
	filterAddresses []eth.Address,
	filterEventSigs []eth.Hash,
	possibleIndexSizes []uint64,

) *LogAddressIndexProvider {
	// todo(froch, 20220702): firm up what these values will be
	if possibleIndexSizes == nil {
		possibleIndexSizes = []uint64{100000, 10000, 1000, 100}
	}

	return &LogAddressIndexProvider{
		store:              store,
		currentIndex:       NewLogAddressIndex(lowBlockNum, indexSize),
		indexOpsTimeout:    2 * time.Second,
		filterAddresses:    filterAddresses,
		filterEventSigs:    filterEventSigs,
		possibleIndexSizes: possibleIndexSizes,
	}
}

// WithinRange determines if we have an index which includes this number.
func (ip *LogAddressIndexProvider) WithinRange(blockNum uint64) bool {
	r, lowBlockNum, indexSize := ip.findIndexContaining(blockNum)
	if r == nil {
		return false
	}
	if err := ip.loadIndex(r, lowBlockNum, indexSize); err != nil {
		zlog.Error("couldn't load index", zap.Error(err))
		return false
	}
	return true
}

// Matches returns true if the provided blockNum matches entries in the index
func (ip *LogAddressIndexProvider) Matches(blockNum uint64) bool {
	if err := ip.loadRange(blockNum); err != nil {
		log.Error("couldn't load range", zap.Error(err))
		// shouldn't happen; return true is the safest choice so the consumer receives all data
		return true
	}
	// asset blockNum > current.Low and blockNum < currentindex.Low + size
	// if not
	//    loadRange blockNum
	// for _, matchingBlocks := range currentindex.matchingBlocks()
	//	  if block.Num == blockNum: return true
	return false
}

// NextMatching will return the next block matching our request
func (ip *LogAddressIndexProvider) NextMatching(blockNum uint64) (num uint64, done bool) {
	if err := ip.loadRange(blockNum); err != nil {
		log.Warn("couldn't load range", zap.Error(err))
		// shouldn't happen; return the input blocknum and consider done
		return blockNum, true
	}

	for {
		for _, block := range ip.currentMatchingBlocks {
			if block > blockNum {
				return blockNum, false
			}
		}

		nextBaseBlock := ip.currentIndex.lowBlockNum + ip.currentIndex.indexSize
		err := ip.loadRange(nextBaseBlock)
		if err != nil {
			return nextBaseBlock, true
		}
	}
}

// loadRange will traverse available indexes until it finds the desired blockNum
func (ip *LogAddressIndexProvider) loadRange(blockNum uint64) error {
	if blockNum >= ip.currentIndex.lowBlockNum && blockNum < ip.currentIndex.lowBlockNum+ip.currentIndex.indexSize {
		return nil
	}

	// truncate any prior matching blocks
	ip.currentMatchingBlocks = []uint64{}

	r, lowBlockNum, indexSize := ip.findIndexContaining(blockNum)
	if r == nil {
		return fmt.Errorf("couldn't find index containing block_num: %d", blockNum)
	}
	if err := ip.loadIndex(r, lowBlockNum, indexSize); err != nil {
		return err
	}

	return nil
}

// findIndexContaining tries to find an index file in dstore containing the provided blockNum
// if such a file exists, returns an io.Reader; nil otherwise
func (ip *LogAddressIndexProvider) findIndexContaining(blockNum uint64) (r io.Reader, lowBlockNum, indexSize uint64) {
	ctx, cancel := context.WithTimeout(context.Background(), ip.indexOpsTimeout)
	defer cancel()

	for _, size := range ip.possibleIndexSizes {
		var err error

		base := lowBoundary(blockNum, size)
		filename := toIndexFilename(size, base, LogAddressIdxShortname)

		r, err = ip.store.OpenObject(ctx, filename)
		if err == dstore.ErrNotFound {
			zlog.Debug("couldn't find index file", zap.String("filename", filename))
			continue
		}
		if err != nil {
			zlog.Error("couldn't open index from dstore", zap.Error(err))
			continue
		}

		return r, base, size
	}

	return
}

// loadIndex will populate the indexProvider's currentIndex from the provided io.Reader
func (ip *LogAddressIndexProvider) loadIndex(r io.Reader, lowBlockNum, indexSize uint64) error {
	obj, err := ioutil.ReadAll(r)
	if err != nil {
		return fmt.Errorf("couldn't read index: %s", err)
	}

	newIdx := NewLogAddressIndex(lowBlockNum, indexSize)
	err = newIdx.Unmarshal(obj)
	if err != nil {
		return fmt.Errorf("couldn't unmarshal index: %s", err)
	}

	ip.currentIndex = newIdx
	ip.currentMatchingBlocks = ip.currentIndex.matchingBlocks(ip.filterAddresses, ip.filterEventSigs)
	return nil
}
