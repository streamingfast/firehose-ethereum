package transform

import (
	"fmt"

	"github.com/RoaringBitmap/roaring/roaring64"
	"github.com/golang/protobuf/proto"
	"github.com/streamingfast/eth-go"
	pbtransforms "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/transforms/v1"
)

// LogAddressIndex will return false positives when matching addr AND eventSignatures
type LogAddressIndex struct {
	// TODO: maybe use ID() as uint to match address
	// TODO: add a bloomfilter, populated on load

	addrs       map[string]*roaring64.Bitmap // map[eth address](blocknum bitmap)
	eventSigs   map[string]*roaring64.Bitmap
	lowBlockNum uint64
	indexSize   uint64
}

func NewLogAddressIndex(lowBlockNum, indexSize uint64) *LogAddressIndex {
	return &LogAddressIndex{
		lowBlockNum: lowBlockNum,
		indexSize:   indexSize,
		addrs:       make(map[string]*roaring64.Bitmap),
		eventSigs:   make(map[string]*roaring64.Bitmap),
	}
}

func (i *LogAddressIndex) Marshal() ([]byte, error) {
	pbIndex := &pbtransforms.LogAddressSignatureIndex{}

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

func (i *LogAddressIndex) Unmarshal(in []byte) error {
	pbIndex := &pbtransforms.LogAddressSignatureIndex{}

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

func nilIfEmpty(in []uint64) []uint64 {
	if len(in) == 0 {
		return nil
	}
	return in
}

func (i *LogAddressIndex) addressBitmap(addrs []eth.Address) *roaring64.Bitmap {
	out := roaring64.NewBitmap()
	for _, addr := range addrs {
		addrString := addr.String()
		if bm, ok := i.addrs[addrString]; ok {
			out.Or(bm)
		}
	}
	return out
}
func (i *LogAddressIndex) sigsBitmap(sigs []eth.Hash) *roaring64.Bitmap {
	out := roaring64.NewBitmap()
	for _, sig := range sigs {
		sigString := sig.String()
		if _, ok := i.eventSigs[sigString]; !ok {
			continue
		}
		out.Or(i.eventSigs[sigString])
	}
	return out
}

func (i *LogAddressIndex) filterBitmap(f *logAddressSingleFilter) *roaring64.Bitmap {
	wantAddresses := len(f.addrs) != 0
	wantSigs := len(f.eventSigs) != 0

	switch {
	case wantAddresses && !wantSigs:
		return i.addressBitmap(f.addrs)
	case wantSigs && !wantAddresses:
		return i.sigsBitmap(f.eventSigs)
	case wantAddresses && wantSigs:
		a := i.addressBitmap(f.addrs)
		b := i.sigsBitmap(f.eventSigs)
		a.And(b)
		return a
	default:
		panic("unsupported case")
	}
}

func (i *LogAddressIndex) matchingBlocks(filters []*logAddressSingleFilter) []uint64 {
	out := roaring64.NewBitmap()
	for _, f := range filters {
		fbit := i.filterBitmap(f)
		out.Or(fbit)
	}
	return nilIfEmpty(out.ToArray())

}

func (i *LogAddressIndex) add(addr eth.Address, eventSig eth.Hash, blocknum uint64) {
	i.addAddr(addr, blocknum)
	if eventSig == nil {
		return
	}
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
