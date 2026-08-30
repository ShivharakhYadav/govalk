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
