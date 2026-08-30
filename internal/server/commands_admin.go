package server

import "github.com/ShivharakhYadav/govalk/internal/resp"

// RegisterAdminCommands adds PING and ECHO to d - the smallest possible
// vertical slice proving listener -> connection handler -> dispatch all
// work together end-to-end (P7).
//
// Matches real Redis semantics: PING with no argument replies +PONG (a
// Simple String); PING with one argument, and ECHO's single required
// argument, both reply with that argument echoed back as a Bulk String,
// not a Simple String - an echoed value is arbitrary client-supplied
// bytes, not necessarily printable/CRLF-free text, so it must go out
// through the binary-safe encoding.
func RegisterAdminCommands(d *Dispatcher) {
	d.Register("PING", 0, 1, func(args [][]byte) resp.Value {
		if len(args) == 0 {
			return resp.NewSimpleString("PONG")
		}
		return resp.NewBulkString(args[0])
	})

	d.Register("ECHO", 1, 1, func(args [][]byte) resp.Value {
		return resp.NewBulkString(args[0])
	})
}
