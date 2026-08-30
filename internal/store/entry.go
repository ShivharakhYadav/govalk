package store

import (
	"errors"
	"math"
	"time"
)

// kind distinguishes what an entry currently holds. A key holds exactly
// one kind of value at a time - Redis's WRONGTYPE semantics, not a
// govalk invention. kindList exists now, ahead of list commands
// themselves (P16), purely so the type check in Get is genuinely
// kind-generic rather than a string-only special case that would need
// reworking once lists exist.
type kind byte

const (
	kindString kind = iota
	kindList
)

// entry is the value stored per key, plus TTL metadata. Only the
// field(s) matching kind are meaningful.
//
// Entries are immutable once stored in a shard's map: every mutation
// (Set, Delete, and later EXPIRE/PERSIST) replaces the map's *entry
// pointer with a brand new one, never mutates fields of an entry
// already in the map. That is what makes it safe for a read to look up
// an *entry under a shard's read lock, release the lock, and then
// dereference its fields afterward (see shard.lookupLive): the object a
// held pointer refers to never changes underneath the reader, it is
// only ever replaced wholesale.
type entry struct {
	kind      kind
	str       []byte
	expiresAt int64 // UnixNano deadline; 0 means no TTL
}

// expired reports whether e's TTL deadline has passed as of now
// (UnixNano). An entry with no TTL (expiresAt == 0) never expires.
func (e *entry) expired(now int64) bool {
	return e.expiresAt != 0 && e.expiresAt <= now
}

// withTTL returns a copy of e with expiresAt replaced, leaving its value
// untouched. TTL commands (EXPIRE, PERSIST) change only the expiration
// metadata; per entry's own invariant, that still means publishing a
// brand new *entry, never mutating fields of the one already stored.
// Centralizing "what fields carry over" here means only this one
// function needs updating when a future value kind (e.g. lists, P16)
// adds fields of its own.
func (e *entry) withTTL(expiresAt int64) *entry {
	return &entry{kind: e.kind, str: e.str, expiresAt: expiresAt}
}

// ErrExpireOutOfRange is returned by SafeExpireDeadline when a relative
// TTL is large enough that converting it to an absolute deadline would
// overflow int64, rather than silently wrapping around to a bogus
// (possibly already-past) deadline.
var ErrExpireOutOfRange = errors.New("invalid expire time")

// SafeExpireDeadline converts a relative TTL in seconds (as given by the
// EXPIRE command) into an absolute UnixNano deadline relative to now.
//
// immediate=true (err=nil) means seconds <= 0: real Redis treats a
// non-positive EXPIRE as an instruction to delete the key immediately,
// not as an error and not as "no TTL" - the caller must delete the key
// rather than set a deadline.
//
// err is non-nil when seconds is large enough that seconds*time.Second
// added to now would overflow int64. Real Redis rejects an
// out-of-range EXPIRE for the same reason: silently wrapping around
// would turn "expire far in the future" into "expire in the past."
func SafeExpireDeadline(now time.Time, seconds int64) (deadline int64, immediate bool, err error) {
	if seconds <= 0 {
		return 0, true, nil
	}

	const nanosPerSecond = int64(time.Second)
	if seconds > math.MaxInt64/nanosPerSecond {
		return 0, false, ErrExpireOutOfRange
	}
	deltaNanos := seconds * nanosPerSecond

	nowNanos := now.UnixNano()
	if deltaNanos > math.MaxInt64-nowNanos {
		return 0, false, ErrExpireOutOfRange
	}
	return nowNanos + deltaNanos, false, nil
}
