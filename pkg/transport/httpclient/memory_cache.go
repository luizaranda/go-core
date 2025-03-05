package httpclient

import (
	"time"

	"github.com/karlseguin/ccache/v2"
	"github.com/luizaranda/go-core/pkg/transport"
)

// A Cache interface is used by HTTP client to store and retrieve responses.
type Cache interface {
	transport.Cache

	// Close is used to signal a shutdown of the cache when you are done with
	// it. This allows the cleaning goroutines to exit and ensures references
	// are not kept to the cache preventing GC of the entire cache.
	//
	// Implementations are permitted to panic on operations performed on the
	// cache after Close is called. Package httpclient will never call Close,
	// it's a responsibility that's left to the user.
	Close() error
}

// MiB represents an integer value in Mega Bytes (1024*1024 bytes). It is used
// to indicate the local in-memory cache size.
type MiB int64

func (m MiB) bytes() int64 { return int64(m * 1024 * 1024) }

var (
	// DefaultCache is the default cache used by the HTTP Client when response
	// caching is enabled and no custom cache is given.
	DefaultCache transport.Cache = NewLocalCache(500)
)

type cache struct{ cache *ccache.Cache }

// sizedBytes is an slice alias which provides the Size() method, to fulfill
// the ccache.Sized interface and allow ccache to measure item size correctly.
type sizedBytes []byte

func (s sizedBytes) Size() int64 {
	// ccache has an overhead of ~350 bytes per entry that's not taken into
	// account. We add it so that the memory tracking is more precise.
	return int64(len(s)) + 350
}

// NewLocalCache instantiates a new memory cache, specifically tailored for
// working with this package HTTP response caching mechanisms.
//
// The cache will optimistically try to keep it's total memory consumption
// bellow the given maxSize megabytes. Because GC of cached items runs in
// background, and the Go GC needs to collect the memory afterwards, at any
// point in time the cache may consume more memory.
//
// It is highly recommended against users using this cache for other purposes
// other than this package HTTP caching mechanisms.
//
// If creating a cache that has a short life, then in order to avoid memory
// leaks the user is required to call Close() on the cache when it finishes
// using it.
func NewLocalCache(maxSize MiB) Cache {
	// Cache size in bytes.
	bytes := maxSize.bytes()

	// Set amount of items to prune when memory is low.
	gcThreshold := uint32(maxSize) / 10

	if gcThreshold == 0 {
		gcThreshold = 1
	}

	cfg := ccache.Configure().
		MaxSize(bytes).
		ItemsToPrune(gcThreshold)

	return &cache{
		cache: ccache.New(cfg),
	}
}

func (c *cache) Get(key string) ([]byte, bool) {
	item := c.cache.Get(key)
	if item == nil {
		return nil, false
	}

	bytes, ok := item.Value().(sizedBytes)
	return bytes, ok
}

func (c *cache) Set(key string, responseBytes []byte) {
	c.cache.Set(key, sizedBytes(responseBytes), 1*time.Hour)
}

func (c *cache) Delete(key string) {
	c.cache.Delete(key)
}

func (c *cache) Close() error {
	c.cache.Stop()
	return nil
}
