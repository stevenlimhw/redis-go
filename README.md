redis-go
========

A lightweight in-memory key-value cache implemented in Go, inspired by Redis.
Built as a learning project to explore caching concepts, concurrency, and Go idioms.


------------------------------------------------------------------------
OVERVIEW
------------------------------------------------------------------------

redis-go is an in-memory cache that supports setting key-value pairs with
a time-to-live (TTL), automatic expiry, background cleanup, and basic
observability via stats. It is designed to be simple, thread-safe, and
extensible.


------------------------------------------------------------------------
IMPLEMENTED FEATURES
------------------------------------------------------------------------

1. Core Cache Operations (Set, Get)

   Status: Done

   Functional Requirements
   - Set a key-value pair with a TTL in seconds
   - Get a value by key, returning the value and a boolean indicating success
   - If TTL is not provided (0), fall back to a default TTL of 60 seconds
   - Expired entries are detected on Get and removed immediately
   - All operations are logged to stdout with key, value, TTL, and expiry time

   Non-Functional Requirements
   - All reads and writes are protected by a read-write mutex (sync.RWMutex)
   - Get uses a read lock to allow concurrent reads
   - Set uses a write lock to ensure safe mutation


2. Background Cleanup Goroutine

   Status: Done

   Functional Requirements
   - A background goroutine starts automatically when a cache is created
   - The goroutine sweeps the cache every 5 seconds and removes expired entries
   - Each eviction is logged to stdout
   - The goroutine can be stopped cleanly by calling Stop()
   - Stop() is idempotent and safe to call multiple times

   Non-Functional Requirements
   - The cleanup goroutine uses a time.Ticker to trigger sweeps at a fixed interval
   - A done channel is used to signal the goroutine to exit
   - sync.Once ensures the done channel is only closed once, preventing panics
   - The sweep acquires a write lock only during the deletion phase


3. Delete, Exists, Clear

   Status: Done

   Functional Requirements
   - Delete(key): removes a single key from the cache immediately
   - Exists(key): returns true if a key exists and has not expired
   - Clear(): removes all entries from the cache

   Non-Functional Requirements
   - Delete and Clear acquire a write lock
   - Exists acquires a read lock and checks expiry without side effects
   - Exists guards against nil pointer dereference when a key is not found


4. Hit / Miss / Eviction Stats

   Status: Done

   Functional Requirements
   - Track the number of cache hits, misses, and evictions
   - A hit is counted when Get returns a valid, non-expired value
   - A miss is counted when Get finds no key, or finds an expired key
   - An eviction is counted when an expired key is deleted, either via Get or cleanup
   - Stats() returns a snapshot of current counters
   - Stats implement fmt.Stringer for pretty-printed output

   Non-Functional Requirements
   - Counters are updated using sync/atomic for lock-free, thread-safe increments
   - Stats() reads counters using atomic.LoadInt64 to return a consistent snapshot
   - Stats are stored on the Cache struct and never reset automatically


5. TTL Management (ttl, refresh, persist)

   Status: Planned

   Functional Requirements
   - ttl(key): returns the remaining lifetime of a key as a time.Duration
   - refresh(key): resets a key's TTL back to its original value without changing its value
   - persist(key): removes the expiry from a key, making it permanent

   Non-Functional Requirements
   - ttl() should be read-only and use a read lock
   - refresh() and persist() mutate entry state and require a write lock
   - All three should return an error or boolean if the key does not exist


6. Max Capacity and Eviction Policy

   Status: Planned

   Functional Requirements
   - Support a configurable max number of entries at cache creation time
   - When the cache is full, evict an entry based on a chosen policy before inserting
   - Support at minimum one eviction policy: LRU (Least Recently Used)
   - Optionally support LFU (Least Frequently Used) and FIFO

   Non-Functional Requirements
   - LRU requires tracking last-accessed time per entry, updated on every Get
   - Eviction on Set must be atomic with the insert to avoid race conditions
   - Max capacity of 0 means unlimited (default behaviour, backward compatible)


7. Persistence (Snapshot and Restore)

   Status: Planned

   Functional Requirements
   - SaveSnapshot(path): serializes the current cache state to disk
   - LoadSnapshot(path): restores cache state from a file on startup
   - Only non-expired entries should be saved in a snapshot
   - On load, entries whose TTL has already elapsed should be discarded

   Non-Functional Requirements
   - Serialization format should be JSON or gob
   - SaveSnapshot acquires a read lock to avoid blocking writes during serialization
   - File writes should be atomic where possible (write to temp file, then rename)


8. HTTP Server Layer

   Status: Planned

   Functional Requirements
   - Expose cache operations over HTTP REST endpoints
   - POST   /set         - set a key-value pair with optional TTL
   - GET    /get?key=    - retrieve a value by key
   - DELETE /delete?key= - delete a key
   - GET    /exists?key= - check if a key exists
   - DELETE /clear       - clear all entries
   - GET    /stats       - return cache stats as JSON

   Non-Functional Requirements
   - Server should be configurable with a listener address (already stubbed in main.go)
   - Each handler should return appropriate HTTP status codes
   - Request and response bodies should be JSON
   - Server should reuse the same Cache instance and its existing mutex guarantees


------------------------------------------------------------------------
NON-FUNCTIONAL REQUIREMENTS (GLOBAL)
------------------------------------------------------------------------

- Thread Safety: all cache operations are safe for concurrent use
- Observability: all mutations and evictions are logged to stdout
- Testability: all features have corresponding unit tests
- Idiomatic Go: follows standard Go conventions for naming, error handling,
  and concurrency patterns
- No external dependencies: uses only the Go standard library


------------------------------------------------------------------------
PROJECT STRUCTURE
------------------------------------------------------------------------

  main.go       - entry point, wires up and runs the cache
  cache.go      - Cache struct, Set, Get, Delete, Exists, Clear, cleanup
  stats.go      - Stats struct, Stats() method, String() pretty printer


------------------------------------------------------------------------
RUNNING THE PROJECT
------------------------------------------------------------------------

  Run:
    go run .

  Test:
    go test ./... -v

  Test with race detector:
    go test ./... -race
