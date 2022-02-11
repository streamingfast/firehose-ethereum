package transform

import (
	"bytes"
	"context"
	"fmt"

	"io/ioutil"
	"time"

	"github.com/streamingfast/bstream"
	"github.com/streamingfast/dstore"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/codec/v1"
	"go.uber.org/zap"
)

type LogAddressIndexer struct {
	currentIndex    *LogAddressIndex
	store           dstore.Store
	indexSize       uint64
	indexOpsTimeout time.Duration
}

func NewLogAddressIndexer(store dstore.Store, indexSize uint64) *LogAddressIndexer {
	return &LogAddressIndexer{
		store:           store,
		currentIndex:    nil,
		indexSize:       indexSize,
		indexOpsTimeout: 15 * time.Second,
	}
}

func (i *LogAddressIndexer) ProcessEthBlock(blk *pbcodec.Block) error {

	// init lower bound
	if i.currentIndex == nil {
		switch {

		case blk.Num()%i.indexSize == 0:
			// we're on a boundary
			i.currentIndex = NewLogAddressIndex(blk.Number, i.indexSize)

		case blk.Number == bstream.GetProtocolFirstStreamableBlock:
			// handle offset
			lb := lowBoundary(blk.Num(), i.indexSize)
			i.currentIndex = NewLogAddressIndex(lb, i.indexSize)

		default:
			zlog.Warn("couldn't determine boundary for block", zap.Uint64("blk_num", blk.Num()))
			return nil
		}
	}

	// upper bound reached
	if blk.Num() >= i.currentIndex.lowBlockNum+i.indexSize {
		if err := i.writeIndex(); err != nil {
			zlog.Warn("cannot write index", zap.Error(err))
		}
		lb := lowBoundary(blk.Number, i.indexSize)
		i.currentIndex = NewLogAddressIndex(lb, i.indexSize)
	}

	for _, trace := range blk.TransactionTraces {
		for _, log := range trace.Receipt.Logs {
			var evSig []byte
			if len(log.Topics) != 0 {
				evSig = log.Topics[0]
			}
			i.currentIndex.add(log.Address, evSig, blk.Number)
		}
	}

	return nil
}

func (i *LogAddressIndexer) String() string {
	return fmt.Sprintf("addresses: %d, methods: %d", len(i.currentIndex.addrs), len(i.currentIndex.eventSigs))
}

func (i *LogAddressIndexer) writeIndex() error {
	ctx, cancel := context.WithTimeout(context.Background(), i.indexOpsTimeout)
	defer cancel()

	if i.currentIndex == nil {
		zlog.Warn("attempted to write nil index")
		return nil
	}

	data, err := i.currentIndex.Marshal()
	if err != nil {
		return err
	}

	filename := toIndexFilename(i.indexSize, i.currentIndex.lowBlockNum, LogAddressIdxShortname)
	if err = i.store.WriteObject(ctx, filename, bytes.NewReader(data)); err != nil {
		zlog.Warn("cannot write index file to store",
			zap.String("filename", filename),
			zap.Error(err),
		)
	}
	zlog.Info("wrote file to store",
		zap.String("filename", filename),
		zap.Uint64("low_block_num", i.currentIndex.lowBlockNum),
	)

	return nil
}

func (i *LogAddressIndexer) readIndex(indexName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), i.indexOpsTimeout)
	defer cancel()

	if i.currentIndex == nil {
		i.currentIndex = NewLogAddressIndex(0, i.indexSize)
	}

	dstoreObj, err := i.store.OpenObject(ctx, indexName)
	if err != nil {
		return fmt.Errorf("couldn't open object %s from dstore: %s", indexName, err)
	}

	obj, err := ioutil.ReadAll(dstoreObj)
	if err != nil {
		return fmt.Errorf("couldn't read %s: %s", indexName, err)
	}

	err = i.currentIndex.Unmarshal(obj)
	if err != nil {
		return fmt.Errorf("couldn't unmarshal %s: %s", indexName, err)
	}

	return nil
}
