package store

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
