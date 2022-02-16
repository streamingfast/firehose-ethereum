package transform

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"time"

	"github.com/streamingfast/eth-go"

	"github.com/streamingfast/dstore"
	"go.uber.org/zap"
)

const LogAddressIdxShortname string = "logaddr"

// LogAddressIndexProvider responds to queries about LogAddressIndexes
type LogAddressIndexProvider struct {
	currentIndex          *LogAddressIndex
	currentMatchingBlocks []uint64
	filterAddresses       []eth.Address
	filterEventSigs       []eth.Hash
	indexOpsTimeout       time.Duration
	possibleIndexSizes    []uint64
	store                 dstore.Store
}

func NewLogAddressIndexProvider(
	store dstore.Store,
	filterAddresses []eth.Address,
	filterEventSigs []eth.Hash,
	possibleIndexSizes []uint64,
) *LogAddressIndexProvider {
	// todo(froch, 20220207): firm up what these values will be
	if possibleIndexSizes == nil {
		possibleIndexSizes = []uint64{100000, 10000, 1000, 100}
	}
	if len(filterAddresses) == 0 && len(filterEventSigs) == 0 {
		return nil
	}

	return &LogAddressIndexProvider{
		store:              store,
		indexOpsTimeout:    15 * time.Second,
		filterAddresses:    filterAddresses,
		filterEventSigs:    filterEventSigs,
		possibleIndexSizes: possibleIndexSizes,
	}
}

// WithinRange determines the existence of an index which includes the provided blockNum
// it also attempts to pre-emptively load the index (read-ahead)
func (ip *LogAddressIndexProvider) WithinRange(ctx context.Context, blockNum uint64) bool {
	ctx, cancel := context.WithTimeout(ctx, ip.indexOpsTimeout)
	defer cancel()

	if ip.currentIndex != nil && ip.currentIndex.lowBlockNum <= blockNum && (ip.currentIndex.lowBlockNum+ip.currentIndex.indexSize) > blockNum {
		return true
	}

	r, lowBlockNum, indexSize := ip.findIndexContaining(ctx, blockNum)
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
func (ip *LogAddressIndexProvider) Matches(ctx context.Context, blockNum uint64) (bool, error) {
	if err := ip.loadRange(ctx, blockNum); err != nil {
		return false, fmt.Errorf("couldn't load range: %s", err)
	}

	for _, matchingBlock := range ip.currentMatchingBlocks {
		if blockNum == matchingBlock {
			return true, nil
		}
	}

	return false, nil
}

func (ip *LogAddressIndexProvider) NextMatching(ctx context.Context, blockNum uint64, exclusiveUpTo uint64) (num uint64, passedIndexBoundary bool, err error) {
	if err = ip.loadRange(ctx, blockNum); err != nil {
		return 0, false, fmt.Errorf("couldn't load range: %s", err)
	}

	for {
		for _, block := range ip.currentMatchingBlocks {
			if block > blockNum {
				return block, false, nil
			}
		}

		nextBaseBlock := ip.currentIndex.lowBlockNum + ip.currentIndex.indexSize
		if exclusiveUpTo != 0 && nextBaseBlock >= exclusiveUpTo {
			return exclusiveUpTo, false, nil
		}
		err := ip.loadRange(ctx, nextBaseBlock)
		if err != nil {
			return nextBaseBlock, true, nil
		}
	}
}

// loadRange will traverse available indexes until it finds the desired blockNum
func (ip *LogAddressIndexProvider) loadRange(ctx context.Context, blockNum uint64) error {
	if ip.currentIndex != nil && blockNum >= ip.currentIndex.lowBlockNum && blockNum < ip.currentIndex.lowBlockNum+ip.currentIndex.indexSize {
		return nil
	}

	// truncate any prior matching blocks
	ip.currentMatchingBlocks = []uint64{}

	ctx, cancel := context.WithTimeout(ctx, ip.indexOpsTimeout)
	defer cancel()

	r, lowBlockNum, indexSize := ip.findIndexContaining(ctx, blockNum)
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
func (ip *LogAddressIndexProvider) findIndexContaining(ctx context.Context, blockNum uint64) (r io.Reader, lowBlockNum, indexSize uint64) {

	for _, size := range ip.possibleIndexSizes {
		var err error

		base := lowBoundary(blockNum, size)
		filename := toIndexFilename(size, base, LogAddressIdxShortname)

		r, err = ip.store.OpenObject(ctx, filename)
		if err == dstore.ErrNotFound {
			zlog.Debug("couldn't find index file",
				zap.String("filename", filename),
				zap.Uint64("blockNum", blockNum),
			)
			continue
		}
		if err != nil {
			if ctx.Err() != nil {
				return
			}
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
