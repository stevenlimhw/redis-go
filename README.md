# redis-go

Claude prompts:
- You are a principal backend engineer at Google, working on an in-house cache module in Golang.
- For the purposes of this discussion, the cache module will be a simple one, that can get, set and add expiry ttl.
    - Do not write code at all at this planning stage. Write in simple txt file.
- What do you prepare first for planning? What are the list of things to plan?
- What is the architecture and what patterns to follow?


---
notes
---

- TTL semantics: per-entry TTL, global default TTL, or both?
- Return types: (value, bool) vs (value, error) -- pick one and document why
- Context propagation: should Get/Set accept context.Context

eviction policy
- must be defined before data structure choices, lru

expiry mechanism
- ttl / expiry: lazy expiry (check on Get) vs active expiry (background goroutine sweeps)
- precision requirement: second-level vs millisecond-level
    - clock source: time.Now() is sufficient

concurrency model
- single global mutex
- read-write lock (sync.RWMutex)

data structures
- primary store: map[string]*entry per shard
- entry struct fields: value, expiry (int64 unix nano), any LRU pointers
- lru list representation?
- memory overhead per entry must be estimated

error handling strategy
- define sentinel errors: ErrNotFound, ErrExpired, ErrCacheFull
- panic vs error return for programmer errors (e.g. nil key)

observability
- counters: hits, misses, evictions, expirations, sets, deletes
- gauges: current entry count, current memory bytes

config
- Config struct at construction time (funcitonal options pattern)
- what has safe defaults vs what is mandatory?
- Document every knob: max entries, max bytes, default TTL, shard count, sweep interval.

