package transform

import (
	"context"
	"fmt"
	"github.com/streamingfast/dstore"
	"github.com/streamingfast/eth-go"
	"go.uber.org/zap"
	"io"
	"io/ioutil"
	"math"
	"strconv"
	"strings"
	"time"
)

//type IndexProvider interface {
//	WithinRange(blockNum uint64) bool
//	Matches(blockNum uint64) bool
//	NextMatching(blockNum uint64) (num uint64, done bool) // when done is true, the returned blocknum is the first Unindexed block
//}

// LogAddressIndexProvider responds to queries about LogAddressIndexes
type LogAddressIndexProvider struct {
	store dstore.Store

	filterAddresses []eth.Address
	filterEventSigs []eth.Hash

	currentIndex          *LogAddressIndex
	currentMatchingBlocks []uint64

	noMoreIndexes   bool
	indexOpsTimeout time.Duration
}

func NewLogAddressIndexProvider(store dstore.Store) *LogAddressIndexProvider {
	return &LogAddressIndexProvider{
		store:           store,
		indexOpsTimeout: 15 * time.Second,
	}
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
			zlog.Error("currentMatchingBlocks should be empty", zap.Int("len", len(ip.currentMatchingBlocks)))
			return
		}

		if ip.currentIndex == nil {
			zlog.Error("currentIndex is nil")
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
// if such a file exists, returns an io.Reader; nil otherwise
func (ip *LogAddressIndexProvider) findIndexContaining(blockNum uint64) io.Reader {
	ctx, cancel := context.WithTimeout(context.Background(), ip.indexOpsTimeout)
	defer cancel()

	files, err := ip.store.ListFiles(ctx, "", "", math.MaxInt64)
	if err != nil {
		zlog.Error("couldn't read from dstore", zap.Error(err))
		return nil
	}

	for _, file := range files {
		chunks := strings.Split(file, ".")
		lowBlockNumStr := chunks[0]
		indexSizeStr := chunks[1]

		lowBlockNum, err := strconv.ParseUint(lowBlockNumStr, 10, 64)
		if err != nil {
			zlog.Error("couldn't determine lowBlockNum from filename chunk", zap.Error(err))
			return nil
		}

		indexSize, err := strconv.ParseUint(indexSizeStr, 10, 64)
		if err != nil {
			zlog.Error("couldn't determine indexSize from filename chunk", zap.Error(err))
			return nil
		}

		if blockNum >= lowBlockNum && blockNum < lowBlockNum+indexSize {
			r, err := ip.store.OpenObject(ctx, file)
			if err != nil {
				zlog.Error("couldn't open dstore object", zap.Error(err))
			}
			ip.currentIndex = NewLogAddressIndex(lowBlockNum, indexSize)
			return r
		}
	}

	return nil
}

// loadIndex will populate the indexProvider's currentIndex from the provided io.Reader
// its currentLowBlockNum and currentIndexSize properties are determined in findIndexContaining
func (ip *LogAddressIndexProvider) loadIndex(r io.Reader) error {
	if ip.currentIndex == nil {
		return fmt.Errorf("need an index before loading from file")
	}

	obj, err := ioutil.ReadAll(r)
	if err != nil {
		return fmt.Errorf("couldn't read index: %s", err)
	}

	err = ip.currentIndex.Unmarshal(obj)
	if err != nil {
		return fmt.Errorf("couldn't unmarshal index: %s", err)
	}

	return nil
}
