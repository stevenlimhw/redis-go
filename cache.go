package main

import (
	"io"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

type Cache struct {
	CacheMap map[string]*CacheEntry
	cacheHead *CacheEntry // most recently used (sentinel)
	cacheTail *CacheEntry // least recently used (sentinel)

	mutex sync.RWMutex
	done chan struct{}
	once sync.Once
	stats Stats

	logger *log.Logger
}

type CacheEntry struct {
	Value string
	ExpiryTime time.Time
	Ttl time.Duration

	// track lru eviction
	prev *CacheEntry
	next *CacheEntry
}

const defaultCacheTtl = 60 * time.Second // in secs

func NewCache(w io.Writer) *Cache {
	cacheMap := make(map[string]*CacheEntry)
	done := make(chan struct{})

	head := &CacheEntry{}
	tail := &CacheEntry{}
	head.next = tail
	tail.prev = head

	cache := &Cache{
		CacheMap:	cacheMap, 
		done: 		done,
		logger: 	log.New(w, "[cache] ", 0),
		cacheHead: head,
		cacheTail: tail,
	}

	go cache.cleanup()
	return cache
}

func (c *Cache) Get(key string) (string, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	entry, ok := c.CacheMap[key]

	if !ok {
		atomic.AddInt64(&c.stats.Misses, 1)
		return "", false
	}

	if entry.isExpired() {
		c.deleteFromCache(key)
		atomic.AddInt64(&c.stats.Misses, 1)

		c.logger.Printf("GET EXPIRED key=%q\n", key)
		return "", false
	}

	c.logger.Printf("GET key=%q val=%q ttl=%v expires=%s\n",
    key,
    entry.Value,
    entry.Ttl,
    entry.ExpiryTime.Format(time.DateTime),
	)

	// move entry to most recently used (front of list)
	c.updateEntryToMru(entry)

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

	// set as most recently used
	c.updateEntryToMru(entry)

	c.logger.Printf("SET key=%q val=%q ttl=%v expires=%s\n",
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

	c.deleteFromCache(key)
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

	c.cacheHead = &CacheEntry{}
	c.cacheTail = &CacheEntry{}
	c.cacheHead.next = c.cacheTail
	c.cacheTail.prev = c.cacheHead
}

// returns remaining lifetime of a key
func (c *Cache) Ttl(key string) time.Duration {
	c.mutex.RLock()
	entry, ok := c.CacheMap[key]
	c.mutex.RUnlock()

	if !ok {
		c.logger.Printf("TTL key=%q not found\n", key) 
		return -1
	}

	if entry.isExpired() {
		return 0
	}

	currentTtl := entry.ExpiryTime.Sub(time.Now())

	c.logger.Printf("TTL key=%q, ttl=%q\n", key, currentTtl)
	return currentTtl
}

// refresh(key): resets a key's TTL back to its original value without changing its value
func (c *Cache) Refresh(key string) time.Duration {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	entry, ok := c.CacheMap[key]
	if !ok {
		c.logger.Printf("REFRESH ttl key=%q failed to execute", key)
		return -1
	}

	ttl := entry.Ttl
	entry.ExpiryTime = time.Now().Add(ttl)
	return ttl
}

// persist(key): removes the expiry from a key, making it permanent
func (c *Cache) Persist(key string) bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	entry, ok := c.CacheMap[key]

	if !ok {
		c.logger.Printf("PERSIST key=%q failed to execute", key)
		return false
	}

	entry.ExpiryTime = time.Time{}
	entry.Ttl = -1
	return true
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
					c.deleteFromCache(key)
					c.logger.Printf("EVICT key=%q val=%q\n", key, entry.Value)
				}
			}
		  c.mutex.Unlock()
		case <-c.done:
			return // exits goroutine
		}
	}
}

// only call this method with write lock acquired
func (c *Cache) deleteFromCache(key string) {
	entry, ok := c.CacheMap[key]
	if !ok {
		c.logger.Printf("[deleteFromCache] failed to find entry in cache map")
		return
	}

	// remove entry from lru list
	prevEntry := entry.prev
	nextEntry := entry.next
	if prevEntry != nil {
		prevEntry.next = nextEntry
	}
	if nextEntry != nil {
		nextEntry.prev = prevEntry
	}

	// remove entry from cache map
	delete(c.CacheMap, key)
	atomic.AddInt64(&c.stats.Evicts, 1)
}

// only call this method with write lock acquired
func (c *Cache) updateEntryToMru(entry *CacheEntry) {
	// remove from original position in cache list
	prev := entry.prev
	next := entry.next
	if prev != nil {
		prev.next = next
	}
	if next != nil {
		next.prev = prev
	}

	// rewire entry to front of cache list
	mruSentinel := c.cacheHead
	mruSentinelNext := mruSentinel.next
	mruSentinel.next = entry
	entry.prev = mruSentinel

	entry.next = mruSentinelNext
	mruSentinelNext.prev = entry
}

func calculateExpiry(ttl time.Duration) time.Time {
	return time.Now().Add(ttl)
}

func (entry *CacheEntry) isExpired() bool {
	if entry.Ttl == -1 || entry.ExpiryTime.IsZero() {
		return false // persisted keys never expire
	}
	return time.Now().After(entry.ExpiryTime)
}

