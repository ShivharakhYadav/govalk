package server

import (
	"bytes"

	"github.com/ShivharakhYadav/govalk/internal/resp"
	"github.com/ShivharakhYadav/govalk/internal/store"
)

// RegisterStringCommands adds GET, SET, DEL, and EXISTS to d, backed by
// s. TTL support (SET ... EX, EXPIRE/TTL/PERSIST) is P10 - SET here
// always calls s.Set with expiresAt=0 (no TTL). P10 (or a later
// sub-plan) is expected to re-Register "SET" with extended arity and EX
// parsing once that lands; Dispatcher.Register simply replaces the
// existing entry, no conflict.
func RegisterStringCommands(d *Dispatcher, s *store.Store) {
	d.Register("GET", 1, 1, func(args [][]byte) resp.Value {
		val, ok, err := s.Get(string(args[0]))
		if err != nil {
			// store.ErrWrongType's message is already the exact Redis
			// wire text ("WRONGTYPE Operation against a key holding
			// the wrong kind of value") - no "ERR " prefix, since
			// WRONGTYPE is its own error code, not a generic error.
			return resp.NewError(err.Error())
		}
		if !ok {
			return resp.NewNullBulkString()
		}
		return resp.NewBulkString(val)
	})

	d.Register("SET", 2, 2, func(args [][]byte) resp.Value {
		// args[1] is copied, not stored by reference: it's only
		// guaranteed valid for the duration of this call. P2's reader
		// happens to allocate a fresh buffer per bulk string today,
		// but §3 Performance leaves room for a future reused
		// per-connection scratch buffer if profiling ever justifies
		// it - copying here means Store can never end up silently
		// aliasing a connection's read buffer if that changes.
		s.Set(string(args[0]), bytes.Clone(args[1]), 0)
		return resp.NewSimpleString("OK")
	})

	d.Register("DEL", 1, -1, func(args [][]byte) resp.Value {
		var n int64
		for _, k := range args {
			if s.Delete(string(k)) {
				n++
			}
		}
		return resp.NewInteger(n)
	})

	d.Register("EXISTS", 1, -1, func(args [][]byte) resp.Value {
		// Matches real Redis: a repeated key is counted once per
		// occurrence, not deduplicated.
		var n int64
		for _, k := range args {
			if s.Exists(string(k)) {
				n++
			}
		}
		return resp.NewInteger(n)
	})
}
