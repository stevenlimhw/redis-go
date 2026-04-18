package main

import (
	"sync"
	"testing"
	"time"
)

// --- NewCache ---

func TestNewCache_IsNotNil(t *testing.T) {
	cache := NewCache()
	if cache == nil {
		t.Fatal("expected non-nil cache")
	}
}

func TestNewCache_MapIsInitialized(t *testing.T) {
	cache := NewCache()
	if cache.CacheMap == nil {
		t.Fatal("expected CacheMap to be initialized, got nil")
	}
}

func TestNewCache_IsEmpty(t *testing.T) {
	cache := NewCache()
	if len(cache.CacheMap) != 0 {
		t.Fatalf("expected empty cache, got %d entries", len(cache.CacheMap))
	}
}

// --- Set ---

func TestSet_StoresValue(t *testing.T) {
	cache := NewCache()
	entry, ok := cache.Set("key1", "value1", 60)
	if !ok {
		t.Fatal("expected set to return true")
	}
	if entry == nil {
		t.Fatal("expected non-nil entry")
	}
	if entry.Value != "value1" {
		t.Errorf("expected value %q, got %q", "value1", entry.Value)
	}
}

func TestSet_TTLIsApplied(t *testing.T) {
	cache := NewCache()
	entry, _ := cache.Set("key1", "value1", 30)
	expectedTTL := 30 * time.Second
	if entry.Ttl != expectedTTL {
		t.Errorf("expected TTL %v, got %v", expectedTTL, entry.Ttl)
	}
}

func TestSet_DefaultTTLWhenZero(t *testing.T) {
	cache := NewCache()
	entry, _ := cache.Set("key1", "value1", 0)
	if entry.Ttl != defaultCacheTtl {
		t.Errorf("expected default TTL %v, got %v", defaultCacheTtl, entry.Ttl)
	}
}

func TestSet_ExpiryTimeIsInFuture(t *testing.T) {
	cache := NewCache()
	before := time.Now()
	entry, _ := cache.Set("key1", "value1", 60)
	after := time.Now()

	if entry.ExpiryTime.Before(before.Add(59 * time.Second)) {
		t.Errorf("expiry time %v is too early", entry.ExpiryTime)
	}
	if entry.ExpiryTime.After(after.Add(61 * time.Second)) {
		t.Errorf("expiry time %v is too late", entry.ExpiryTime)
	}
}

func TestSet_OverwritesExistingKey(t *testing.T) {
	cache := NewCache()
	cache.Set("key1", "first", 60)
	cache.Set("key1", "second", 60)

	val, ok := cache.Get("key1")
	if !ok || val != "second" {
		t.Errorf("expected %q, got %q", "second", val)
	}
}

func TestSet_MultipleKeys(t *testing.T) {
	cache := NewCache()
	cache.Set("a", "alpha", 60)
	cache.Set("b", "beta", 60)
	cache.Set("c", "gamma", 60)

	if len(cache.CacheMap) != 3 {
		t.Errorf("expected 3 entries, got %d", len(cache.CacheMap))
	}
}

// --- Get ---

func TestGet_ReturnsStoredValue(t *testing.T) {
	cache := NewCache()
	cache.Set("dog", "woof", 60)

	val, ok := cache.Get("dog")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if val != "woof" {
		t.Errorf("expected %q, got %q", "woof", val)
	}
}

func TestGet_MissingKeyReturnsFalse(t *testing.T) {
	cache := NewCache()

	val, ok := cache.Get("nonexistent")
	if ok {
		t.Error("expected ok=false for missing key")
	}
	if val != "" {
		t.Errorf("expected empty string, got %q", val)
	}
}

func TestGet_ExpiredEntryReturnsFalse(t *testing.T) {
	cache := NewCache()
	cache.Set("temp", "gone-soon", 1) // 1 second TTL

	time.Sleep(1100 * time.Millisecond)

	val, ok := cache.Get("temp")
	if ok {
		t.Error("expected ok=false for expired key")
	}
	if val != "" {
		t.Errorf("expected empty string for expired key, got %q", val)
	}
}

func TestGet_ExpiredEntryIsDeletedFromMap(t *testing.T) {
	cache := NewCache()
	cache.Set("temp", "gone-soon", 1)

	time.Sleep(1100 * time.Millisecond)
	cache.Get("temp") // triggers deletion

	if _, exists := cache.CacheMap["temp"]; exists {
		t.Error("expected expired entry to be removed from CacheMap")
	}
}

func TestGet_ActiveEntryIsNotDeleted(t *testing.T) {
	cache := NewCache()
	cache.Set("persist", "alive", 60)

	cache.Get("persist")

	if _, exists := cache.CacheMap["persist"]; !exists {
		t.Error("expected non-expired entry to remain in CacheMap")
	}
}

func TestGet_JustBeforeExpiry(t *testing.T) {
	cache := NewCache()
	cache.Set("key", "val", 2) // 2 second TTL

	time.Sleep(500 * time.Millisecond) // well within TTL

	val, ok := cache.Get("key")
	if !ok || val != "val" {
		t.Errorf("expected valid entry before expiry, got val=%q ok=%v", val, ok)
	}
}

// --- isExpired ---

func TestIsExpired_FutureExpiryReturnsFalse(t *testing.T) {
	entry := &CacheEntry{
		Value:      "x",
		ExpiryTime: time.Now().Add(10 * time.Second),
		Ttl:        10 * time.Second,
	}
	if entry.isExpired() {
		t.Error("expected isExpired=false for future expiry")
	}
}

func TestIsExpired_PastExpiryReturnsTrue(t *testing.T) {
	entry := &CacheEntry{
		Value:      "x",
		ExpiryTime: time.Now().Add(-1 * time.Second),
		Ttl:        1 * time.Second,
	}
	if !entry.isExpired() {
		t.Error("expected isExpired=true for past expiry")
	}
}

// --- calculateExpiry ---

func TestCalculateExpiry_IsApproximatelyNowPlusTTL(t *testing.T) {
	ttl := 5 * time.Minute
	before := time.Now()
	expiry := calculateExpiry(ttl)
	after := time.Now()

	if expiry.Before(before.Add(ttl)) {
		t.Errorf("expiry %v is before expected minimum %v", expiry, before.Add(ttl))
	}
	if expiry.After(after.Add(ttl + time.Second)) {
		t.Errorf("expiry %v is after expected maximum %v", expiry, after.Add(ttl+time.Second))
	}
}

// --- Concurrency ---

func TestConcurrentSetAndGet(t *testing.T) {
	cache := NewCache()
	var wg sync.WaitGroup
	const goroutines = 50

	// Concurrent sets
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := string(rune('a' + i%26))
			cache.Set(key, "value", 60)
		}(i)
	}

	// Concurrent gets
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := string(rune('a' + i%26))
			cache.Get(key)
		}(i)
	}

	wg.Wait() // should not deadlock or panic
}

func TestConcurrentSetOnSameKey(t *testing.T) {
	cache := NewCache()
	var wg sync.WaitGroup
	const goroutines = 100

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cache.Set("shared", "value", 60)
		}()
	}

	wg.Wait()

	val, ok := cache.Get("shared")
	if !ok || val != "value" {
		t.Errorf("expected key to exist after concurrent sets, got val=%q ok=%v", val, ok)
	}
}

// --- Cleanup / Background Eviction ---

func TestCleanup_ExpiredEntriesAreRemoved(t *testing.T) {
	cache := NewCache()
	defer cache.Stop()

	cache.Set("a", "alpha", 1) // expires in 1s
	cache.Set("b", "beta", 60) // long-lived

	time.Sleep(7 * time.Second) // wait for cleanup ticker (5s) + buffer

	cache.mutex.RLock()
	_, expiredExists := cache.CacheMap["a"]
	_, activeExists := cache.CacheMap["b"]
	cache.mutex.RUnlock()

	if expiredExists {
		t.Error("expected expired key 'a' to be removed by cleanup")
	}
	if !activeExists {
		t.Error("expected active key 'b' to remain after cleanup")
	}
}

func TestCleanup_NonExpiredEntriesAreUntouched(t *testing.T) {
	cache := NewCache()
	defer cache.Stop()

	cache.Set("x", "xray", 60)
	cache.Set("y", "yankee", 60)

	time.Sleep(7 * time.Second)

	cache.mutex.RLock()
	defer cache.mutex.RUnlock()

	if _, ok := cache.CacheMap["x"]; !ok {
		t.Error("expected key 'x' to still exist")
	}
	if _, ok := cache.CacheMap["y"]; !ok {
		t.Error("expected key 'y' to still exist")
	}
}

func TestStop_StopsCleanupGoroutine(t *testing.T) {
	cache := NewCache()
	cache.Set("k", "v", 1)

	cache.Stop() // stop before cleanup fires

	time.Sleep(7 * time.Second)

	// Entry may still be in map since cleanup was stopped
	// Just verify Stop() doesn't panic or deadlock
	cache.mutex.RLock()
	defer cache.mutex.RUnlock()
	// reaching here means no deadlock/panic — test passes
}

func TestStop_IsIdempotent(t *testing.T) {
	cache := NewCache()
	cache.Stop()

	defer func() {
		if r := recover(); r != nil {
			t.Error("expected Stop() to be safe to call twice, but it panicked")
		}
	}()

	cache.Stop() // should not panic
}

func TestDelete_RemovesCorrectEntry(t *testing.T) {
	cache := NewCache()
	cache.Set("key", "val", 10)

	val, ok := cache.Get("key")
	if !ok || val != "val" {
		t.Error("expected key 'key' to still exist, but not found")
	}

	cache.Delete("key")
	val, ok = cache.Get("key")
	if ok || val == "val" {
		t.Error("expected key 'key' to be deleted from cache, but found")
	}
}

func TestExists_FoundCorrectEntry(t *testing.T) {
	cache := NewCache()
	cache.Set("key", "val", 10)

	ok := cache.Exists("key")
	if !ok {
		t.Error("expected key 'key' to exist in the cache, but not found")
	}
}

func TestExists_CannotFindCorrectEntry(t *testing.T) {
	cache := NewCache()

	ok := cache.Exists("key")
	if ok {
		t.Error("expected key 'key' to not exist in cache, but found it instead")
	}

}

func TestClear_RemovesAllEntries(t *testing.T) {
	cache := NewCache()
	cache.Set("key1", "val1", 1)
	cache.Set("key2", "val2", 1)
	cache.Set("key3", "val3", 1)

	cache.Clear()
	ok1 := cache.Exists("key1")
	ok2 := cache.Exists("key2")
	ok3 := cache.Exists("key3")

	if ok1 || ok2 || ok3 {
		t.Error("expected all keys to not exist in cache, but found at least one")
	}
}

// --- Ttl ---

func TestTtl_ReturnsNegativeOneForMissingKey(t *testing.T) {
    cache := NewCache()
    defer cache.Stop()

    ttl := cache.Ttl("nonexistent")
    if ttl != -1 {
        t.Errorf("expected -1 for missing key, got %v", ttl)
    }
}

func TestTtl_ReturnsZeroForExpiredKey(t *testing.T) {
    cache := NewCache()
    defer cache.Stop()

    cache.Set("key", "val", 1)
    time.Sleep(1100 * time.Millisecond)

    ttl := cache.Ttl("key")
    if ttl != 0 {
        t.Errorf("expected 0 for expired key, got %v", ttl)
    }
}

func TestTtl_ReturnsPositiveDurationForActiveKey(t *testing.T) {
    cache := NewCache()
    defer cache.Stop()

    cache.Set("key", "val", 60)
    ttl := cache.Ttl("key")

    if ttl <= 0 {
        t.Errorf("expected positive TTL for active key, got %v", ttl)
    }
    if ttl > 60*time.Second {
        t.Errorf("expected TTL <= 60s, got %v", ttl)
    }
}

func TestTtl_DecreasesOverTime(t *testing.T) {
    cache := NewCache()
    defer cache.Stop()

    cache.Set("key", "val", 60)
    ttl1 := cache.Ttl("key")
    time.Sleep(500 * time.Millisecond)
    ttl2 := cache.Ttl("key")

    if ttl2 >= ttl1 {
        t.Errorf("expected TTL to decrease over time, got ttl1=%v ttl2=%v", ttl1, ttl2)
    }
}

