package substreams

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sync"
	"time"

	"github.com/streamingfast/dstore"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

type Cache interface {
	Get(key string) ([]byte, bool)
	Set(key string, value []byte)
	Save(ctx context.Context)
	UpdateCache(ctx context.Context, blockNum uint64)
}

type CacheKey string
type KV map[CacheKey][]byte

type StoreBackedCache struct {
	store dstore.Store

	kv KV

	startBlock uint64
	endBlock   uint64

	totalHits   *atomic.Uint64
	totalMisses *atomic.Uint64

	cacheSize uint64

	mu sync.RWMutex
}

func NewStoreBackedCache(ctx context.Context, store dstore.Store, blockNum, cacheSize uint64) *StoreBackedCache {
	c := &StoreBackedCache{
		store:       store,
		cacheSize:   cacheSize,
		totalHits:   atomic.NewUint64(0),
		totalMisses: atomic.NewUint64(0),
	}

	startBlock, endBlock := computeStartAndEndBlock(blockNum, cacheSize)

	c.load(ctx, startBlock, endBlock)
	c.startTracking(ctx)

	return c
}

func (c *StoreBackedCache) Get(key string) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if d, found := c.kv[CacheKey(key)]; found {
		c.totalHits.Inc()
		return d, found
	}

	c.totalMisses.Inc()
	return nil, false
}

func (c *StoreBackedCache) Set(key string, value []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.kv[CacheKey(key)] = value
}

func (c *StoreBackedCache) Save(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()
	save(ctx, c.store, cacheFileName(c.startBlock, c.endBlock), c.kv)
}

func (c *StoreBackedCache) UpdateCache(ctx context.Context, blockNum uint64) {
	if !c.contains(blockNum) {
		c.Save(ctx)
		c.load(ctx, c.endBlock, c.endBlock+c.cacheSize)
	}
}

func (c *StoreBackedCache) contains(blockNum uint64) bool {
	return blockNum >= c.startBlock && blockNum < c.endBlock
}

func (c *StoreBackedCache) startTracking(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				c.log()
				return
			case <-time.After(5 * time.Second):
				c.log()
			}
		}
	}()
}

func (c *StoreBackedCache) log() {
	hits := c.totalHits.Load()
	misses := c.totalMisses.Load()

	zlog.Debug("rpc cache_performance",
		zap.Uint64("hits", hits),
		zap.Uint64("misses", misses),
		zap.Float64("hit_rate", float64(hits)/float64(hits+misses)),
	)
}

func (c *StoreBackedCache) load(ctx context.Context, startBlock, endBlock uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	filename := cacheFileName(startBlock, endBlock)
	kv := load(ctx, c.store, filename)

	c.kv = kv
	c.startBlock = startBlock
	c.endBlock = endBlock
}

func load(ctx context.Context, store dstore.Store, filename string) (kv KV) {
	kv = make(KV)

	if store == nil {
		zlog.Debug("skipping rpcCache load: no read store is defined")
		return
	}

	obj, err := store.OpenObject(ctx, filename)
	if err != nil {
		zlog.Debug("rpc cache not found", zap.String("filename", filename), zap.String("read_store_url", store.BaseURL().Redacted()), zap.Error(err))
		return
	}

	b, err := ioutil.ReadAll(obj)
	if err != nil {
		zlog.Info("cannot read all rpc cache bytes", zap.String("filename", filename), zap.String("read_store_url", store.BaseURL().Redacted()), zap.Error(err))
		return
	}

	err = json.Unmarshal(b, &kv)
	if err != nil {
		zlog.Info("cannot unmarshal rpc cache", zap.String("filename", filename), zap.String("read_store_url", store.BaseURL().Redacted()), zap.Error(err))
		return
	}

	return
}

func save(ctx context.Context, store dstore.Store, filename string, kv KV) {
	if store == nil {
		zlog.Debug("skipping rpccache save: no store is defined")
		return
	}

	b, err := json.Marshal(kv)
	if err != nil {
		zlog.Info("cannot marshal rpc cache to bytes", zap.Error(err))
		return
	}
	ioReader := bytes.NewReader(b)

	err = store.WriteObject(ctx, filename, ioReader)
	if err != nil {
		zlog.Info("cannot write rpc cache to store", zap.String("filename", filename), zap.String("write_store_url", store.BaseURL().Redacted()), zap.Error(err))
	}

	return
}

func cacheFileName(start, end uint64) string {
	return fmt.Sprintf("cache-%010d-%010d.cache", start, end)
}

func computeStartAndEndBlock(blockNum, cacheSize uint64) (startBlock, endBlock uint64) {
	startBlock = blockNum - blockNum%cacheSize
	endBlock = startBlock + cacheSize
	return
}

type NoOpCache struct {
}

func (NoOpCache) Get(key string) ([]byte, bool)                    { return nil, false }
func (NoOpCache) Set(key string, value []byte)                     {}
func (NoOpCache) Save(ctx context.Context)                         {}
func (NoOpCache) UpdateCache(ctx context.Context, blockNum uint64) {}
