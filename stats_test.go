package main// --- Stats ---

import (
	"sync"
	"testing"
	"time"
)

func TestStats_InitialValuesAreZero(t *testing.T) {
	cache := NewCache()
	defer cache.Stop()

	stats := cache.Stats()
	if stats.Hits != 0 {
		t.Errorf("expected Hits=0, got %d", stats.Hits)
	}
	if stats.Misses != 0 {
		t.Errorf("expected Misses=0, got %d", stats.Misses)
	}
	if stats.Evicts != 0 {
		t.Errorf("expected Evicts=0, got %d", stats.Evicts)
	}
}

func TestStats_HitIsIncremented(t *testing.T) {
	cache := NewCache()
	defer cache.Stop()

	cache.Set("key", "val", 60)
	cache.Get("key")

	if cache.Stats().Hits != 1 {
		t.Errorf("expected Hits=1, got %d", cache.Stats().Hits)
	}
}

func TestStats_MissIsIncrementedOnMissingKey(t *testing.T) {
	cache := NewCache()
	defer cache.Stop()

	cache.Get("nonexistent")

	if cache.Stats().Misses != 1 {
		t.Errorf("expected Misses=1, got %d", cache.Stats().Misses)
	}
}

func TestStats_MissIsIncrementedOnExpiredKey(t *testing.T) {
	cache := NewCache()
	defer cache.Stop()

	cache.Set("key", "val", 1)
	time.Sleep(1100 * time.Millisecond)
	cache.Get("key")

	if cache.Stats().Misses != 1 {
		t.Errorf("expected Misses=1, got %d", cache.Stats().Misses)
	}
}

func TestStats_EvictIsIncrementedOnExpiredKeyGet(t *testing.T) {
	cache := NewCache()
	defer cache.Stop()

	cache.Set("key", "val", 1)
	time.Sleep(1100 * time.Millisecond)
	cache.Get("key") // triggers eviction via Get

	if cache.Stats().Evicts != 1 {
		t.Errorf("expected Evicts=1, got %d", cache.Stats().Evicts)
	}
}

func TestStats_MultipleHitsAndMisses(t *testing.T) {
	cache := NewCache()
	defer cache.Stop()

	cache.Set("a", "alpha", 60)
	cache.Set("b", "beta", 60)

	cache.Get("a")
	cache.Get("a")
	cache.Get("b")
	cache.Get("nonexistent")

	stats := cache.Stats()
	if stats.Hits != 3 {
		t.Errorf("expected Hits=3, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("expected Misses=1, got %d", stats.Misses)
	}
}

func TestStats_HitsNotAffectedByMisses(t *testing.T) {
	cache := NewCache()
	defer cache.Stop()

	cache.Get("missing1")
	cache.Get("missing2")

	if cache.Stats().Hits != 0 {
		t.Errorf("expected Hits=0 after only misses, got %d", cache.Stats().Hits)
	}
}

func TestStats_ConcurrentGetsAreCountedAccurately(t *testing.T) {
	cache := NewCache()
	defer cache.Stop()

	cache.Set("key", "val", 60)

	var wg sync.WaitGroup
	const goroutines = 100
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cache.Get("key")
		}()
	}
	wg.Wait()

	if cache.Stats().Hits != int64(goroutines) {
		t.Errorf("expected Hits=%d, got %d", goroutines, cache.Stats().Hits)
	}
}
