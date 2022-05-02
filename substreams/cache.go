package substreams

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/streamingfast/dstore"
	"go.uber.org/zap"
	"io/ioutil"
	"sync"
	"time"
)

type CacheKey string
type KV map[CacheKey][]byte

type Cache struct {
	store dstore.Store

	kv KV

	startBlock uint64
	endBlock   uint64

	totalHits   int
	totalMisses int

	cacheSize uint64

	mu sync.RWMutex
}

func NewCache(ctx context.Context, store dstore.Store, blockNum, cacheSize uint64) *Cache {
	c := &Cache{
		store:     store,
		cacheSize: cacheSize,
	}

	startBlock, endBlock := computeStartAndEndBlock(blockNum, cacheSize)

	c.load(ctx, startBlock, endBlock)
	c.startTracking(ctx)

	return c
}

func (c *Cache) Get(key string) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if d, found := c.kv[CacheKey(key)]; found {
		c.totalHits++
		return d, found
	}

	c.totalMisses++
	return nil, false
}

func (c *Cache) Set(key string, value []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.kv[CacheKey(key)] = value
}

func (c *Cache) Save(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()
	save(ctx, c.store, cacheFileName(c.startBlock, c.endBlock), c.kv)
}

func (c *Cache) UpdateCache(ctx context.Context, blockNum uint64) {
	if blockNum >= c.startBlock && blockNum < c.endBlock {
		c.Save(ctx)
	}

	startBlock, endBlock := computeStartAndEndBlock(blockNum, c.cacheSize)

	c.load(ctx, startBlock, endBlock)
}

func (c *Cache) startTracking(ctx context.Context) {
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

func (c *Cache) log() {
	zlog.Info("cache_performance",
		zap.Int("hits", c.totalHits),
		zap.Int("misses", c.totalMisses),
		zap.Float64("hit_rate", float64(c.totalHits)/float64(c.totalHits+c.totalMisses)),
	)
}

func (c *Cache) load(ctx context.Context, startBlock, endBlock uint64) {
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
		zlog.Info("skipping rpcCache load: no read store is defined")
		return
	}

	obj, err := store.OpenObject(ctx, filename)
	if err != nil {
		zlog.Info("rpc cache not found", zap.String("filename", filename), zap.String("read_store_url", store.BaseURL().Redacted()), zap.Error(err))
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
		zlog.Info("skipping rpccache save: no store is defined")
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
	return fmt.Sprintf("cache-%d-%d.cache", start, end)
}

func computeStartAndEndBlock(blockNum, cacheSize uint64) (startBlock, endBlock uint64) {
	startBlock = blockNum - blockNum%cacheSize
	endBlock = startBlock + cacheSize
	return
}
