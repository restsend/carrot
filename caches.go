package carrot

import (
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

type expiredLRUCacheValue[V any] struct {
	n   time.Time
	val V
}

type ExpiredLRUCache[K comparable, V any] struct {
	*lru.Cache[K, expiredLRUCacheValue[V]]
	expired time.Duration
}

func NewExpiredLRUCache[K comparable, V any](size int, expired time.Duration) *ExpiredLRUCache[K, V] {
	c, _ := lru.New[K, expiredLRUCacheValue[V]](size)
	return &ExpiredLRUCache[K, V]{
		Cache:   c,
		expired: expired,
	}
}

func (c *ExpiredLRUCache[K, V]) Get(key K) (value V, ok bool) {
	storeValue, ok := c.Cache.Get(key)
	if ok {
		return storeValue.val, true
	}
	if time.Since(storeValue.n) >= c.expired {
		c.Cache.Remove(key)
		ok = false
		return
	}
	return storeValue.val, true
}

func (c *ExpiredLRUCache[K, V]) Add(key K, value V) (evicted bool) {
	storeValue := expiredLRUCacheValue[V]{
		n:   time.Now(),
		val: value,
	}
	return c.Cache.Add(key, storeValue)
}

func (c *ExpiredLRUCache[K, V]) Contains(key K) bool {
	return c.Cache.Contains(key)
}

func (c *ExpiredLRUCache[K, V]) Remove(key K) (present bool) {
	return c.Cache.Remove(key)
}
