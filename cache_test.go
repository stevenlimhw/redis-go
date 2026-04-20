package main

import (
	"io"
	"sync"
	"testing"
	"time"
)

// --- NewCache ---

func TestNewCache_IsNotNil(t *testing.T) {
	cache := NewCache(io.Discard)
	if cache == nil {
		t.Fatal("expected non-nil cache")
	}
}

func TestNewCache_MapIsInitialized(t *testing.T) {
	cache := NewCache(io.Discard)
	if cache.CacheMap == nil {
		t.Fatal("expected CacheMap to be initialized, got nil")
	}
}

func TestNewCache_IsEmpty(t *testing.T) {
	cache := NewCache(io.Discard)
	if len(cache.CacheMap) != 0 {
		t.Fatalf("expected empty cache, got %d entries", len(cache.CacheMap))
	}
}

// --- Set ---

func TestSet_StoresValue(t *testing.T) {
	cache := NewCache(io.Discard)
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
	cache := NewCache(io.Discard)
	entry, _ := cache.Set("key1", "value1", 30)
	expectedTTL := 30 * time.Second
	if entry.Ttl != expectedTTL {
		t.Errorf("expected TTL %v, got %v", expectedTTL, entry.Ttl)
	}
}

func TestSet_DefaultTTLWhenZero(t *testing.T) {
	cache := NewCache(io.Discard)
	entry, _ := cache.Set("key1", "value1", 0)
	if entry.Ttl != defaultCacheTtl {
		t.Errorf("expected default TTL %v, got %v", defaultCacheTtl, entry.Ttl)
	}
}

func TestSet_ExpiryTimeIsInFuture(t *testing.T) {
	cache := NewCache(io.Discard)
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
	cache := NewCache(io.Discard)
	cache.Set("key1", "first", 60)
	cache.Set("key1", "second", 60)

	val, ok := cache.Get("key1")
	if !ok || val != "second" {
		t.Errorf("expected %q, got %q", "second", val)
	}
}

func TestSet_MultipleKeys(t *testing.T) {
	cache := NewCache(io.Discard)
	cache.Set("a", "alpha", 60)
	cache.Set("b", "beta", 60)
	cache.Set("c", "gamma", 60)

	if len(cache.CacheMap) != 3 {
		t.Errorf("expected 3 entries, got %d", len(cache.CacheMap))
	}
}

// --- Get ---

func TestGet_ReturnsStoredValue(t *testing.T) {
	cache := NewCache(io.Discard)
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
	cache := NewCache(io.Discard)

	val, ok := cache.Get("nonexistent")
	if ok {
		t.Error("expected ok=false for missing key")
	}
	if val != "" {
		t.Errorf("expected empty string, got %q", val)
	}
}

func TestGet_ExpiredEntryReturnsFalse(t *testing.T) {
	cache := NewCache(io.Discard)
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
	cache := NewCache(io.Discard)
	cache.Set("temp", "gone-soon", 1)

	time.Sleep(1100 * time.Millisecond)
	cache.Get("temp") // triggers deletion

	if _, exists := cache.CacheMap["temp"]; exists {
		t.Error("expected expired entry to be removed from CacheMap")
	}
}

func TestGet_ActiveEntryIsNotDeleted(t *testing.T) {
	cache := NewCache(io.Discard)
	cache.Set("persist", "alive", 60)

	cache.Get("persist")

	if _, exists := cache.CacheMap["persist"]; !exists {
		t.Error("expected non-expired entry to remain in CacheMap")
	}
}

func TestGet_JustBeforeExpiry(t *testing.T) {
	cache := NewCache(io.Discard)
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
	cache := NewCache(io.Discard)
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
	cache := NewCache(io.Discard)
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
	cache := NewCache(io.Discard)
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
	cache := NewCache(io.Discard)
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
	cache := NewCache(io.Discard)
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
	cache := NewCache(io.Discard)
	cache.Stop()

	defer func() {
		if r := recover(); r != nil {
			t.Error("expected Stop() to be safe to call twice, but it panicked")
		}
	}()

	cache.Stop() // should not panic
}

func TestDelete_RemovesCorrectEntry(t *testing.T) {
	cache := NewCache(io.Discard)
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
	cache := NewCache(io.Discard)
	cache.Set("key", "val", 10)

	ok := cache.Exists("key")
	if !ok {
		t.Error("expected key 'key' to exist in the cache, but not found")
	}
}

func TestExists_CannotFindCorrectEntry(t *testing.T) {
	cache := NewCache(io.Discard)

	ok := cache.Exists("key")
	if ok {
		t.Error("expected key 'key' to not exist in cache, but found it instead")
	}

}

func TestClear_RemovesAllEntries(t *testing.T) {
	cache := NewCache(io.Discard)
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
	cache := NewCache(io.Discard)
	defer cache.Stop()

	ttl := cache.Ttl("nonexistent")
	if ttl != -1 {
		t.Errorf("expected -1 for missing key, got %v", ttl)
	}
}

func TestTtl_ReturnsZeroForExpiredKey(t *testing.T) {
	cache := NewCache(io.Discard)
	defer cache.Stop()

	cache.Set("key", "val", 1)
	time.Sleep(1100 * time.Millisecond)

	ttl := cache.Ttl("key")
	if ttl != 0 {
		t.Errorf("expected 0 for expired key, got %v", ttl)
	}
}

func TestTtl_ReturnsPositiveDurationForActiveKey(t *testing.T) {
	cache := NewCache(io.Discard)
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
	cache := NewCache(io.Discard)
	defer cache.Stop()

	cache.Set("key", "val", 60)
	ttl1 := cache.Ttl("key")
	time.Sleep(500 * time.Millisecond)
	ttl2 := cache.Ttl("key")

	if ttl2 >= ttl1 {
		t.Errorf("expected TTL to decrease over time, got ttl1=%v ttl2=%v", ttl1, ttl2)
	}
}

// --- isExpired ---

func TestIsExpired_ZeroTimeReturnsFalse(t *testing.T) {
	entry := &CacheEntry{
		Value:      "val",
		ExpiryTime: time.Time{}, // zero value — persisted key
	}
	if entry.isExpired() {
		t.Error("expected isExpired=false for zero ExpiryTime")
	}
}

func TestIsExpired_ExactlyNowIsExpired(t *testing.T) {
	entry := &CacheEntry{
		Value:      "val",
		ExpiryTime: time.Now().Add(-1 * time.Millisecond),
		Ttl:        1 * time.Second,
	}
	if !entry.isExpired() {
		t.Error("expected isExpired=true for expiry in the past")
	}
}

// --- Persist ---

func TestPersist_SetsExpiryTimeToZero(t *testing.T) {
	cache := NewCache(io.Discard)
	defer cache.Stop()

	cache.Set("key", "val", 60)
	cache.Persist("key")

	cache.mutex.RLock()
	entry := cache.CacheMap["key"]
	cache.mutex.RUnlock()

	if !entry.ExpiryTime.IsZero() {
		t.Errorf("expected ExpiryTime to be zero after Persist, got %v", entry.ExpiryTime)
	}
}

func TestPersist_IsExpiredReturnsFalseAfterPersist(t *testing.T) {
	cache := NewCache(io.Discard)
	defer cache.Stop()

	cache.Set("key", "val", 60)
	cache.Persist("key")

	cache.mutex.RLock()
	entry := cache.CacheMap["key"]
	cache.mutex.RUnlock()

	if entry.isExpired() {
		t.Error("expected isExpired=false after Persist")
	}
}

func TestPersist_KeyNeverExpires(t *testing.T) {
	cache := NewCache(io.Discard)
	defer cache.Stop()

	cache.Set("key", "val", 1)
	cache.Persist("key")
	time.Sleep(1500 * time.Millisecond)

	_, ok := cache.Get("key")
	if !ok {
		t.Error("expected persisted key to still exist after original TTL elapsed")
	}
}

func TestPersist_ReturnsFalseForMissingKey(t *testing.T) {
	cache := NewCache(io.Discard)
	defer cache.Stop()

	ok := cache.Persist("nonexistent")
	if ok {
		t.Error("expected false for missing key, got true")
	}
}

func TestPersist_DoesNotChangeValue(t *testing.T) {
	cache := NewCache(io.Discard)
	defer cache.Stop()

	cache.Set("key", "permanent", 60)
	cache.Persist("key")

	val, ok := cache.Get("key")
	if !ok || val != "permanent" {
		t.Errorf("expected value unchanged after Persist, got %q ok=%v", val, ok)
	}
}

func TestPersist_KeySurvivesBackgroundCleanup(t *testing.T) {
	cache := NewCache(io.Discard)
	defer cache.Stop()

	cache.Set("key", "val", 1)
	cache.Persist("key")
	time.Sleep(7 * time.Second) // wait for cleanup sweep

	if !cache.Exists("key") {
		t.Error("expected persisted key to survive background cleanup")
	}
}

// --- helpers ---

// lruOrder returns keys from MRU to LRU, excluding sentinels
func lruOrder(c *Cache) []string {
	var keys []string
	cur := c.cacheHead.next
	for cur != c.cacheTail {
		keys = append(keys, cur.Value) // Value holds the key name in these tests
		cur = cur.next
	}
	return keys
}

// lruOrderReverse returns keys from LRU to MRU, excluding sentinels
func lruOrderReverse(c *Cache) []string {
	var keys []string
	cur := c.cacheTail.prev
	for cur != c.cacheHead {
		keys = append(keys, cur.Value)
		cur = cur.prev
	}
	return keys
}

func strSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] { //nolint:gosec
			return false
		}
	}
	return true
}

// --- Sentinel initialisation ---

func TestLRU_SentinelsLinkedOnInit(t *testing.T) {
	cache := NewCache(io.Discard)
	defer cache.Stop()

	if cache.cacheHead.next != cache.cacheTail {
		t.Error("expected head.next == tail on init")
	}
	if cache.cacheTail.prev != cache.cacheHead {
		t.Error("expected tail.prev == head on init")
	}
}

func TestLRU_SentinelsHaveNilOuterPointers(t *testing.T) {
	cache := NewCache(io.Discard)
	defer cache.Stop()

	if cache.cacheHead.prev != nil {
		t.Error("expected head.prev == nil")
	}
	if cache.cacheTail.next != nil {
		t.Error("expected tail.next == nil")
	}
}

// --- Set inserts at front ---

func TestLRU_SetInsertsAtFront(t *testing.T) {
	cache := NewCache(io.Discard)
	defer cache.Stop()

	cache.Set("a", "a", 60)

	if cache.cacheHead.next.Value != "a" {
		t.Errorf("expected 'a' at front, got %q", cache.cacheHead.next.Value)
	}
}

func TestLRU_SetMaintainsMRUOrder(t *testing.T) {
	cache := NewCache(io.Discard)
	defer cache.Stop()

	cache.Set("a", "a", 60)
	cache.Set("b", "b", 60)
	cache.Set("c", "c", 60)

	// c was set last, so should be MRU
	got := lruOrder(cache)
	want := []string{"c", "b", "a"}
	if !strSliceEqual(got, want) {
		t.Errorf("expected MRU order %v, got %v", want, got)
	}
}

func TestLRU_SetLinkIsConsistentForwardAndBackward(t *testing.T) {
	cache := NewCache(io.Discard)
	defer cache.Stop()

	cache.Set("a", "a", 60)
	cache.Set("b", "b", 60)
	cache.Set("c", "c", 60)

	fwd := lruOrder(cache)
	rev := lruOrderReverse(cache)

	// reverse of forward should equal reverse traversal
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	if !strSliceEqual(fwd, rev) {
		t.Errorf("forward and backward traversal disagree: fwd=%v rev=%v", fwd, rev)
	}
}

// --- Get promotes to front ---

func TestLRU_GetPromotesToFront(t *testing.T) {
	cache := NewCache(io.Discard)
	defer cache.Stop()

	cache.Set("a", "a", 60)
	cache.Set("b", "b", 60)
	cache.Set("c", "c", 60)
	// order: c, b, a

	cache.Get("a") // a should move to front
	// expected: a, c, b

	got := lruOrder(cache)
	want := []string{"a", "c", "b"}
	if !strSliceEqual(got, want) {
		t.Errorf("expected order %v after Get('a'), got %v", want, got)
	}
}

func TestLRU_GetMRUEntryKeepsItAtFront(t *testing.T) {
	cache := NewCache(io.Discard)
	defer cache.Stop()

	cache.Set("a", "a", 60)
	cache.Set("b", "b", 60)
	// order: b, a

	cache.Get("b") // already MRU, should stay at front

	got := lruOrder(cache)
	want := []string{"b", "a"}
	if !strSliceEqual(got, want) {
		t.Errorf("expected order %v, got %v", want, got)
	}
}

func TestLRU_GetLRUEntryMovesItToFront(t *testing.T) {
	cache := NewCache(io.Discard)
	defer cache.Stop()

	cache.Set("a", "a", 60)
	cache.Set("b", "b", 60)
	cache.Set("c", "c", 60)
	// order: c, b, a — 'a' is LRU

	cache.Get("a")
	// expected: a, c, b

	if cache.cacheHead.next.Value != "a" {
		t.Errorf("expected 'a' at front after Get, got %q", cache.cacheHead.next.Value)
	}
	if cache.cacheTail.prev.Value != "b" {
		t.Errorf("expected 'b' as LRU after Get, got %q", cache.cacheTail.prev.Value)
	}
}

func TestLRU_GetMaintainsConsistentLinksAfterPromotion(t *testing.T) {
	cache := NewCache(io.Discard)
	defer cache.Stop()

	cache.Set("a", "a", 60)
	cache.Set("b", "b", 60)
	cache.Set("c", "c", 60)

	cache.Get("a")

	fwd := lruOrder(cache)
	rev := lruOrderReverse(cache)
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	if !strSliceEqual(fwd, rev) {
		t.Errorf("links inconsistent after Get: fwd=%v rev=%v", fwd, rev)
	}
}

// --- Delete removes from list ---

func TestLRU_DeleteRemovesFromList(t *testing.T) {
	cache := NewCache(io.Discard)
	defer cache.Stop()

	cache.Set("a", "a", 60)
	cache.Set("b", "b", 60)
	cache.Set("c", "c", 60)
	// order: c, b, a

	cache.Delete("b")

	got := lruOrder(cache)
	want := []string{"c", "a"}
	if !strSliceEqual(got, want) {
		t.Errorf("expected order %v after Delete('b'), got %v", want, got)
	}
}

func TestLRU_DeleteMRUUpdatesHead(t *testing.T) {
	cache := NewCache(io.Discard)
	defer cache.Stop()

	cache.Set("a", "a", 60)
	cache.Set("b", "b", 60)
	// order: b (MRU), a

	cache.Delete("b")

	if cache.cacheHead.next.Value != "a" {
		t.Errorf("expected 'a' as new MRU after deleting 'b', got %q", cache.cacheHead.next.Value)
	}
}

func TestLRU_DeleteLRUUpdatesTail(t *testing.T) {
	cache := NewCache(io.Discard)
	defer cache.Stop()

	cache.Set("a", "a", 60)
	cache.Set("b", "b", 60)
	// order: b, a (LRU)

	cache.Delete("a")

	if cache.cacheTail.prev.Value != "b" {
		t.Errorf("expected 'b' as new LRU after deleting 'a', got %q", cache.cacheTail.prev.Value)
	}
}

func TestLRU_DeleteOnlyEntryLeavesSentinelsLinked(t *testing.T) {
	cache := NewCache(io.Discard)
	defer cache.Stop()

	cache.Set("a", "a", 60)
	cache.Delete("a")

	if cache.cacheHead.next != cache.cacheTail {
		t.Error("expected head.next == tail after deleting only entry")
	}
	if cache.cacheTail.prev != cache.cacheHead {
		t.Error("expected tail.prev == head after deleting only entry")
	}
}

func TestLRU_DeleteMaintainsConsistentLinks(t *testing.T) {
	cache := NewCache(io.Discard)
	defer cache.Stop()

	cache.Set("a", "a", 60)
	cache.Set("b", "b", 60)
	cache.Set("c", "c", 60)

	cache.Delete("b")

	fwd := lruOrder(cache)
	rev := lruOrderReverse(cache)
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	if !strSliceEqual(fwd, rev) {
		t.Errorf("links inconsistent after Delete: fwd=%v rev=%v", fwd, rev)
	}
}

// --- Expiry removes from list ---

func TestLRU_ExpiredGetRemovesFromList(t *testing.T) {
	cache := NewCache(io.Discard)
	defer cache.Stop()

	cache.Set("a", "a", 60)
	cache.Set("b", "b", 1) // expires in 1s
	cache.Set("c", "c", 60)
	// order: c, b, a

	time.Sleep(1100 * time.Millisecond)
	cache.Get("b") // triggers removal

	got := lruOrder(cache)
	want := []string{"c", "a"}
	if !strSliceEqual(got, want) {
		t.Errorf("expected order %v after expired Get('b'), got %v", want, got)
	}
}

func TestLRU_ExpiredGetMaintainsConsistentLinks(t *testing.T) {
	cache := NewCache(io.Discard)
	defer cache.Stop()

	cache.Set("a", "a", 60)
	cache.Set("b", "b", 1)
	cache.Set("c", "c", 60)

	time.Sleep(1100 * time.Millisecond)
	cache.Get("b")

	fwd := lruOrder(cache)
	rev := lruOrderReverse(cache)
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	if !strSliceEqual(fwd, rev) {
		t.Errorf("links inconsistent after expired Get: fwd=%v rev=%v", fwd, rev)
	}
}

// --- Clear resets list ---

func TestLRU_ClearResetsSentinels(t *testing.T) {
	cache := NewCache(io.Discard)
	defer cache.Stop()

	cache.Set("a", "a", 60)
	cache.Set("b", "b", 60)
	cache.Clear()

	if cache.cacheHead.next != cache.cacheTail {
		t.Error("expected head.next == tail after Clear")
	}
	if cache.cacheTail.prev != cache.cacheHead {
		t.Error("expected tail.prev == head after Clear")
	}
}

func TestLRU_ClearListIsEmpty(t *testing.T) {
	cache := NewCache(io.Discard)
	defer cache.Stop()

	cache.Set("a", "a", 60)
	cache.Set("b", "b", 60)
	cache.Clear()

	got := lruOrder(cache)
	if len(got) != 0 {
		t.Errorf("expected empty list after Clear, got %v", got)
	}
}

func TestLRU_SetAfterClearInsertsCorrectly(t *testing.T) {
	cache := NewCache(io.Discard)
	defer cache.Stop()

	cache.Set("a", "a", 60)
	cache.Clear()
	cache.Set("b", "b", 60)

	got := lruOrder(cache)
	want := []string{"b"}
	if !strSliceEqual(got, want) {
		t.Errorf("expected %v after Clear+Set, got %v", want, got)
	}
}
