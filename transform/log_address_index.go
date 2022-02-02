package transform

import (
	"fmt"

	"github.com/RoaringBitmap/roaring/roaring64"
	"github.com/streamingfast/dstore"
	"github.com/streamingfast/eth-go"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/codec/v1"
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

func NewLogAddressIndexer(store dstore.Store) *LogAddressIndexer {
	return &LogAddressIndexer{
		store: store,
		currentIndex: &LogAddressIndex{
			//					lowBlockNum: test.lowBlockNum,
			addrs:     make(map[string]*roaring64.Bitmap),
			eventSigs: make(map[string]*roaring64.Bitmap),
		},
	}

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
	currentIndex *LogAddressIndex
	store        dstore.Store
}

func (i *LogAddressIndexer) String() string {
	return fmt.Sprintf("addresses: %d, methods: %d", len(i.currentIndex.addrs), len(i.currentIndex.eventSigs))
}

func (i *LogAddressIndexer) ProcessEthBlock(blk *pbcodec.Block) {
	// TODO: manage lowBlockNum/indexSize, call Save()
	for _, trace := range blk.TransactionTraces {
		for _, log := range trace.Receipt.Logs {
			i.currentIndex.add(log.Address, log.Topics[0], blk.Number)
		}
	}
}
