package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type Cache struct {
	CacheMap map[string]*CacheEntry
	mutex sync.RWMutex
	done chan struct{}
	once sync.Once
	stats Stats
}

type CacheEntry struct {
	Value string
	ExpiryTime time.Time
	Ttl time.Duration
}

const defaultCacheTtl = 60 * time.Second // in secs

func NewCache() *Cache {
	cacheMap := make(map[string]*CacheEntry)
	done := make(chan struct{})
	cache := &Cache{CacheMap:cacheMap, done: done}
	go cache.cleanup()
	return cache
}

func (c *Cache) Get(key string) (string, bool) {
	c.mutex.RLock()
	entry, ok := c.CacheMap[key]
	c.mutex.RUnlock()

	if !ok {
		atomic.AddInt64(&c.stats.Misses, 1)
		return "", false
	}

	if entry.isExpired() {
		c.mutex.Lock()
		delete(c.CacheMap, key)
		atomic.AddInt64(&c.stats.Misses, 1)
		atomic.AddInt64(&c.stats.Evicts, 1)
		c.mutex.Unlock()

		fmt.Printf("[cache] GET key=%q val=%q ttl=%v expires=%s\n",
  	  key,
  	  entry.Value,
  	  entry.Ttl,
    	entry.ExpiryTime.Format(time.DateTime),
		)
		return "", false
	}

	fmt.Printf("[cache] GET key=%q val=%q ttl=%v expires=%s\n",
    key,
    entry.Value,
    entry.Ttl,
    entry.ExpiryTime.Format(time.DateTime),
	)

	atomic.AddInt64(&c.stats.Hits, 1)
	return entry.Value, ok
}

func (c *Cache) Set(key string, val string, ttlInSeconds int) (*CacheEntry, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// fallback if ttl is not provided
	ttl := time.Duration(ttlInSeconds) * time.Second
	if ttl == 0 {
		ttl = defaultCacheTtl
	}

	expiryTime := calculateExpiry(ttl)
	entry := &CacheEntry{Value: val, ExpiryTime: expiryTime, Ttl: ttl}
	c.CacheMap[key] = entry
	fmt.Printf("[cache] SET key=%q val=%q ttl=%v expires=%s\n",
    key,
    val,
    ttl,
    expiryTime.Format(time.DateTime),
	)
	return entry, true
}

func (c *Cache) Stop() {
	c.once.Do(func () {
		close(c.done)
	})
}

func (c *Cache) Delete(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	delete(c.CacheMap, key)
}

func (c *Cache) Exists(key string) bool {
	c.mutex.RLock()
	entry, ok := c.CacheMap[key]
	c.mutex.RUnlock()

	if !ok {
		return false
	}
	return !entry.isExpired()
}

func (c *Cache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.CacheMap = make(map[string]*CacheEntry)
}

func (c *Cache) cleanup() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			// sweep expired entries
		  c.mutex.Lock()
			for key, entry := range c.CacheMap {
				if entry.isExpired() {
					delete(c.CacheMap, key)
					atomic.AddInt64(&c.stats.Evicts, 1)
					fmt.Printf("[cache] EVICT key=%q val=%q\n", key, entry.Value)
				}
			}
		  c.mutex.Unlock()
		case <-c.done:
			return // exits goroutine
		}
	}
}

func calculateExpiry(ttl time.Duration) time.Time {
	return time.Now().Add(ttl)
}

func (entry *CacheEntry) isExpired() bool {
	return time.Now().After(entry.ExpiryTime)
}

