package transform

import (
	"bytes"
	"context"
	"fmt"
	"github.com/RoaringBitmap/roaring/roaring64"
	"github.com/golang/protobuf/proto"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/dstore"
	"github.com/streamingfast/eth-go"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/codec/v1"
	pbtransforms "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/transforms/v1"
	"go.uber.org/zap"
	"io/ioutil"
	"time"
)

// LogAddressIndex will return false positives when matching addr AND eventSignatures
type LogAddressIndex struct {
	// TODO: implement save
	// TODO: implement load
	// TODO: maybe use ID() to match address

	addrs       map[string]*roaring64.Bitmap // map[eth address](blocknum bitmap)
	eventSigs   map[string]*roaring64.Bitmap
	lowBlockNum uint64
	indexSize   uint64
	// TODO: add a bloomfilter, populated on load
}

func (i *LogAddressIndex) Marshal() ([]byte, error) {
	pbIndex := NewLogAddressSignatureIndex()

	for k, v := range i.addrs {
		bitmapBytes, err := v.ToBytes()
		if err != nil {
			return nil, err
		}

		pbIndex.Addresses = append(pbIndex.Addresses, &pbtransforms.KeyToBitmap{
			Key:    []byte(k),
			Bitmap: bitmapBytes,
		})
	}

	for k, v := range i.eventSigs {
		bitmapBytes, err := v.ToBytes()
		if err != nil {
			return nil, err
		}

		pbIndex.EventSignatures = append(pbIndex.EventSignatures, &pbtransforms.KeyToBitmap{
			Key:    []byte(k),
			Bitmap: bitmapBytes,
		})
	}

	return proto.Marshal(pbIndex)
}

func NewLogAddressIndex(lowBlockNum, indexSize uint64) *LogAddressIndex {
	return &LogAddressIndex{
		lowBlockNum: lowBlockNum,
		indexSize:   indexSize,
		addrs:       make(map[string]*roaring64.Bitmap),
		eventSigs:   make(map[string]*roaring64.Bitmap),
	}
}

func (i *LogAddressIndex) Unmarshal(in []byte) error {
	pbIndex := NewLogAddressSignatureIndex()

	if err := proto.Unmarshal(in, pbIndex); err != nil {
		return fmt.Errorf("couldn't unmarshal LogAddressSignatureIndex: %s", err)
	}

	for _, addr := range pbIndex.Addresses {
		key := string(addr.Key)

		r64 := roaring64.NewBitmap()
		err := r64.UnmarshalBinary(addr.Bitmap)
		if err != nil {
			return fmt.Errorf("coudln't unmarshal addr bitmap: %s", err)
		}

		i.addrs[key] = r64
	}

	for _, eventSig := range pbIndex.EventSignatures {
		key := string(eventSig.Key)

		r64 := roaring64.NewBitmap()
		err := r64.UnmarshalBinary(eventSig.Bitmap)
		if err != nil {
			return fmt.Errorf("couldn't unmarshal eventSig bitmap: %s", err)
		}

		i.eventSigs[key] = r64
	}

	return nil
}

func (i *LogAddressIndex) matchingBlocks(addrs []eth.Address, eventSigs []eth.Hash) []uint64 {
	addrBitmap := roaring64.NewBitmap()
	for _, addr := range addrs {
		addrString := addr.String()
		if _, ok := i.addrs[addrString]; !ok {
			continue
		}
		addrBitmap.Or(i.addrs[addrString])
	}
	if len(eventSigs) == 0 {
		out := addrBitmap.ToArray()
		if len(out) == 0 {
			return nil
		}
		return out

	}

	sigsBitmap := roaring64.NewBitmap()
	for _, sig := range eventSigs {
		sigString := sig.String()
		if _, ok := i.eventSigs[sigString]; !ok {
			continue
		}
		sigsBitmap.Or(i.eventSigs[sigString])
	}
	if addrBitmap.IsEmpty() {
		out := sigsBitmap.ToArray()
		if len(out) == 0 {
			return nil
		}
		return out
	}

	addrBitmap.And(sigsBitmap) // transforms addrBitmap
	out := addrBitmap.ToArray()
	if len(out) == 0 {
		return nil
	}
	return out

}

func (i *LogAddressIndex) add(addr eth.Address, eventSig eth.Hash, blocknum uint64) {
	i.addAddr(addr, blocknum)
	i.addEventSig(eventSig, blocknum)
}

func (i *LogAddressIndex) addAddr(addr eth.Address, blocknum uint64) {
	addrString := addr.String()
	bitmap, ok := i.addrs[addrString]
	if !ok {
		i.addrs[addrString] = roaring64.BitmapOf(blocknum)
		return
	}
	bitmap.Add(blocknum)
}

func (i *LogAddressIndex) addEventSig(eventSig eth.Hash, blocknum uint64) {
	sigString := eventSig.String()
	bitmap, ok := i.eventSigs[sigString]
	if !ok {
		i.eventSigs[sigString] = roaring64.BitmapOf(blocknum)
		return
	}
	bitmap.Add(blocknum)
}

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
			i.currentIndex.add(log.Address, log.Topics[0], blk.Number)
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

	filename := toIndexFilename(i.indexSize, i.currentIndex.lowBlockNum, "logaddr")
	if err = i.store.WriteObject(ctx, filename, bytes.NewReader(data)); err != nil {
		zlog.Warn("cannot write index file to store",
			zap.String("filename", filename),
			zap.Error(err),
		)
	}

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
