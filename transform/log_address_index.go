package transform

import (
	"github.com/RoaringBitmap/roaring/roaring64"
	"github.com/streamingfast/bstream"
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

func (i *LogAddressIndex) matchingBlocks(addrs []eth.Address, eventSigs []eth.Hash) (out []uint64) {

	addrBitmap := roaring64.NewBitmap()
	for _, addr := range addrs {
		addrString := addr.String()
		if _, ok := i.addrs[addrString]; !ok {
			continue
		}
		addrBitmap.Or(i.addrs[addrString])
	}
	out = addrBitmap.ToArray()

	return
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

func (i *LogAddressIndexer) ProcessBlock(blk *bstream.Block, obj interface{}) error {
	// TODO: manage lowBlockNum/indexSize, call Save()
	ethBlock := blk.ToProtocol().(*pbcodec.Block)
	for _, trace := range ethBlock.TransactionTraces {
		for _, log := range trace.Receipt.Logs {
			i.currentIndex.add(log.Address, log.Topics[0], blk.Number)
		}
	}
	return nil
}
