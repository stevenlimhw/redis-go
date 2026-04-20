package main

import (
	"fmt"
	"sync/atomic"
)

type Stats struct {
	Hits   int64
	Misses int64
	Evicts int64
}

func (c *Cache) Stats() Stats {
	return Stats{
		Hits:   atomic.LoadInt64(&c.stats.Hits),
		Misses: atomic.LoadInt64(&c.stats.Misses),
		Evicts: atomic.LoadInt64(&c.stats.Evicts),
	}
}

func (s Stats) String() string {
	return fmt.Sprintf(
		"Stats\n  Hits:   %d\n  Misses: %d\n  Evicts: %d\n",
		s.Hits, s.Misses, s.Evicts,
	)
}
