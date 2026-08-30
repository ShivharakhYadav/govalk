// Package store implements govalk's sharded, in-memory key-value store.
package store

import (
	"errors"
	"hash/maphash"
	"time"
)

// ErrWrongType is returned when a command expects a key to hold one
// value type but it currently holds another - Redis's WRONGTYPE
// semantics.
var ErrWrongType = errors.New("WRONGTYPE Operation against a key holding the wrong kind of value")

// numShards is a fixed constant, not dynamically resized: resizing
// would reintroduce the coordination problem sharding exists to avoid
// (PLAN.md §3 "Performance"). A reasonable default for hackathon-scale
// concurrency; P31's load test revisits this with real measurements
// rather than guessing further now.
const numShards = 32

// Store is a sharded, in-memory key-value store. Each key routes
// deterministically to exactly one shard via a seeded hash, so
// operations on a single key are serialized by that shard's mutex while
// operations on different keys usually proceed concurrently across
// different shards.
type Store struct {
	shards []*shard
	seed   maphash.Seed
}

// New returns an empty Store. The shard-routing hash is seeded per
// instance via maphash.MakeSeed() (crypto/rand-backed), not a fixed
// seed: this prevents an attacker from precomputing a set of keys that
// all collide into the same shard and degrade concurrency to a single
// global lock. See PLAN.md §0 decision 3.
func New() *Store {
	shards := make([]*shard, numShards)
	for i := range shards {
		shards[i] = newShard()
	}
	return &Store{shards: shards, seed: maphash.MakeSeed()}
}

func (s *Store) shardFor(key string) *shard {
	h := maphash.String(s.seed, key)
	return s.shards[h%uint64(len(s.shards))]
}

func nowNano() int64 { return time.Now().UnixNano() }

// Get returns the string value for key. ok is false if key does not
// exist or has expired. Returns ErrWrongType if key holds a non-string
// value.
func (s *Store) Get(key string) (value []byte, ok bool, err error) {
	e := s.shardFor(key).lookupLive(key, nowNano())
	if e == nil {
		return nil, false, nil
	}
	if e.kind != kindString {
		return nil, false, ErrWrongType
	}
	return e.str, true, nil
}

// Exists reports whether key currently holds a live (non-expired)
// value, regardless of its type - matching Redis EXISTS semantics.
func (s *Store) Exists(key string) bool {
	return s.shardFor(key).lookupLive(key, nowNano()) != nil
}

// Set stores value as a string under key, replacing whatever key
// previously held (any type, any TTL). If expiresAt is nonzero, it is
// the UnixNano deadline the new entry expires at; zero means no TTL.
// The value and its TTL are set atomically as a single new entry: a
// concurrent reader can never observe the value without its intended
// TTL, or vice versa.
func (s *Store) Set(key string, value []byte, expiresAt int64) {
	sh := s.shardFor(key)
	sh.mu.Lock()
	defer sh.mu.Unlock()
	sh.data[key] = &entry{kind: kindString, str: value, expiresAt: expiresAt}
}

// Delete removes key regardless of its current value's type or
// remaining TTL. Reports whether key was live (existed and had not
// already expired) beforehand - an already-expired-but-not-yet-reaped
// key is treated as already absent, matching Redis's own lazy-expiry
// semantics, even though Delete opportunistically reaps it here either
// way.
func (s *Store) Delete(key string) bool {
	sh := s.shardFor(key)
	sh.mu.Lock()
	defer sh.mu.Unlock()

	e, ok := sh.data[key]
	if !ok {
		return false
	}
	delete(sh.data, key)
	return !e.expired(nowNano())
}

// SetTTL sets key's expiration deadline to the given UnixNano absolute
// deadline, leaving its value and type unchanged. Reports whether key
// was live beforehand; a nonexistent or already-expired key is left
// untouched (there is nothing to set a TTL on), opportunistically
// reaping the expired entry either way, and reported as false -
// matching real Redis EXPIRE's "0 if key does not exist" return.
func (s *Store) SetTTL(key string, deadline int64) bool {
	sh := s.shardFor(key)
	sh.mu.Lock()
	defer sh.mu.Unlock()

	e, ok := sh.data[key]
	if !ok {
		return false
	}
	if e.expired(nowNano()) {
		delete(sh.data, key)
		return false
	}
	sh.data[key] = e.withTTL(deadline)
	return true
}

// TTL reports key's remaining time-to-live. found is false if key does
// not exist (or has already expired). hasTTL is false if key exists but
// carries no expiration - remaining is meaningless in that case. When
// both are true, remaining is the time left until expiry (never
// negative; an entry can't be found as live and also already past its
// deadline, since lookupLive reaps expired entries as it goes).
func (s *Store) TTL(key string) (remaining time.Duration, hasTTL bool, found bool) {
	now := nowNano()
	e := s.shardFor(key).lookupLive(key, now)
	if e == nil {
		return 0, false, false
	}
	if e.expiresAt == 0 {
		return 0, false, true
	}
	return time.Duration(e.expiresAt - now), true, true
}

// Persist removes key's TTL if it has one, leaving its value unchanged.
// Reports whether a TTL was actually removed - false if key doesn't
// exist or exists but already had no TTL, matching real Redis PERSIST's
// return convention.
func (s *Store) Persist(key string) bool {
	sh := s.shardFor(key)
	sh.mu.Lock()
	defer sh.mu.Unlock()

	e, ok := sh.data[key]
	if !ok {
		return false
	}
	if e.expired(nowNano()) {
		delete(sh.data, key)
		return false
	}
	if e.expiresAt == 0 {
		return false
	}
	sh.data[key] = e.withTTL(0)
	return true
}
