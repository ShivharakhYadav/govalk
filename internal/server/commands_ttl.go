package server

import (
	"strconv"
	"time"

	"github.com/ShivharakhYadav/govalk/internal/resp"
	"github.com/ShivharakhYadav/govalk/internal/store"
)

// RegisterTTLCommands adds EXPIRE, TTL, and PERSIST to d, backed by s.
// SET's own EX option is not wired here - see commands_string.go - this
// covers only the three commands the plan scopes to P10.
func RegisterTTLCommands(d *Dispatcher, s *store.Store) {
	d.Register("EXPIRE", 2, 2, func(args [][]byte) resp.Value {
		key := string(args[0])
		seconds, perr := strconv.ParseInt(string(args[1]), 10, 64)
		if perr != nil {
			return resp.NewError("ERR value is not an integer or out of range")
		}

		deadline, immediate, err := store.SafeExpireDeadline(time.Now(), seconds)
		if err != nil {
			return resp.NewError("ERR invalid expire time in 'expire' command")
		}

		// Non-positive seconds means "delete now", not "set a TTL" -
		// see SafeExpireDeadline's doc comment.
		if immediate {
			return boolToInteger(s.Delete(key))
		}
		return boolToInteger(s.SetTTL(key, deadline))
	})

	d.Register("TTL", 1, 1, func(args [][]byte) resp.Value {
		remaining, hasTTL, found := s.TTL(string(args[0]))
		if !found {
			return resp.NewInteger(-2)
		}
		if !hasTTL {
			return resp.NewInteger(-1)
		}
		// Round to the nearest second, matching real Redis's TTL
		// command (which rounds its internal millisecond value rather
		// than truncating).
		seconds := int64((remaining + 500*time.Millisecond) / time.Second)
		return resp.NewInteger(seconds)
	})

	d.Register("PERSIST", 1, 1, func(args [][]byte) resp.Value {
		return boolToInteger(s.Persist(string(args[0])))
	})
}

func boolToInteger(b bool) resp.Value {
	if b {
		return resp.NewInteger(1)
	}
	return resp.NewInteger(0)
}
