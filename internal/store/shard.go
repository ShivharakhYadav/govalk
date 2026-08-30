package store

import "sync"

// shard is one partition of the sharded store: an independent mutex
// guarding an independent key space. Splitting the keyspace across
// shards reduces lock contention under concurrent access to different
// keys - it does NOT change the Big-O of any single operation, which
// remains O(1) average for a map lookup within whichever one shard a
// key hashes to. See PLAN.md §3 "Time / space complexity".
type shard struct {
	mu   sync.RWMutex
	data map[string]*entry
}

func newShard() *shard {
	return &shard{data: make(map[string]*entry)}
}

// lookupLive returns the entry for key if it exists and has not
// expired, or nil otherwise - deleting the entry first if it has
// expired (the "lazy expiry on read" mechanism, PLAN.md §3
// "Concurrency"). This is the single place every read path checks
// liveness, so the tricky locking below only has to be gotten right
// once.
//
// Locking: takes a read lock for the common case (key absent, or
// present and not expired), so concurrent reads - even of the same live
// key - never block each other. Only upgrades to a write lock when an
// expired entry is actually found, and re-validates after acquiring it
// (double-checked locking): another goroutine may have already deleted
// or overwritten the key in the gap between releasing the read lock and
// acquiring the write lock. The shard mutex, not any single call, is
// the atomicity boundary for a key (PLAN.md §3 "Correctness").
func (sh *shard) lookupLive(key string, now int64) *entry {
	sh.mu.RLock()
	e, ok := sh.data[key]
	if !ok {
		sh.mu.RUnlock()
		return nil
	}
	if !e.expired(now) {
		sh.mu.RUnlock()
		return e
	}
	sh.mu.RUnlock()

	sh.mu.Lock()
	defer sh.mu.Unlock()

	cur, ok := sh.data[key]
	if !ok {
		return nil // already deleted by someone else
	}
	if cur == e {
		// Still the same expired entry we saw under the read lock:
		// reap it.
		delete(sh.data, key)
		return nil
	}
	// Replaced with a different entry since we last looked (e.g. a
	// concurrent Set) - report its current liveness, not the stale
	// snapshot's.
	if cur.expired(now) {
		return nil
	}
	return cur
}
