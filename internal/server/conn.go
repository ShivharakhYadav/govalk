package server

import (
	"errors"
	"io"
	"net"
	"time"

	"github.com/ShivharakhYadav/govalk/internal/resp"
)

// RequestHandler processes one decoded RESP request - typically an
// Array of BulkStrings, a command and its arguments, whether it arrived
// as a real multi-bulk request or an inline command - and returns the
// RESP reply to send back. Implemented by command dispatch (P6+); this
// package only needs the shape of the seam, not what's behind it yet.
type RequestHandler func(req resp.Value) resp.Value

// ConnConfig controls per-connection timeout behavior.
type ConnConfig struct {
	// IdleTimeout bounds how long a connection may go without
	// completing a read or a write before being dropped - the
	// slowloris mitigation from PLAN.md §3 "Security". Reset before
	// every read and every write, so a client actively exchanging
	// commands is never cut off mid-conversation, only one that goes
	// silent. Zero disables the timeout (not recommended outside
	// tests).
	IdleTimeout time.Duration
}

// NewConnHandler returns a Handler (see server.go) that decodes RESP
// requests from a connection in a loop, dispatches each to handle, and
// writes back the reply. It returns - letting the caller (Server.handle)
// close the connection - on client disconnect, idle timeout, or protocol
// error. It never closes conn itself, matching Server's contract that it
// owns the connection lifecycle.
func NewConnHandler(handle RequestHandler, cfg ConnConfig) Handler {
	return func(conn net.Conn) {
		r := resp.NewReader(conn)
		w := resp.NewWriter(conn)

		for {
			if cfg.IdleTimeout > 0 {
				if err := conn.SetReadDeadline(time.Now().Add(cfg.IdleTimeout)); err != nil {
					return
				}
			}

			req, err := r.ReadValue()
			if err != nil {
				switch {
				case err == io.EOF:
					return // clean disconnect between commands
				case isTimeout(err):
					return // idle timeout: slowloris mitigation
				case errors.Is(err, resp.ErrProtocol):
					// Can't safely resynchronize a corrupted RESP
					// stream (PLAN.md §3 "Correctness"): tell the
					// client why, then close.
					writeBestEffort(conn, w, cfg.IdleTimeout,
						resp.NewError("ERR Protocol error: "+err.Error()))
					return
				default:
					// io.ErrUnexpectedEOF or any other I/O error:
					// nothing more to usefully do with this
					// connection.
					return
				}
			}

			reply := handle(req)

			if cfg.IdleTimeout > 0 {
				if err := conn.SetWriteDeadline(time.Now().Add(cfg.IdleTimeout)); err != nil {
					return
				}
			}
			if err := w.WriteValue(reply); err != nil {
				return
			}
			if err := w.Flush(); err != nil {
				return
			}
		}
	}
}

func isTimeout(err error) bool {
	ne, ok := err.(net.Error)
	return ok && ne.Timeout()
}

// writeBestEffort attempts one final reply before the connection closes.
// Failures are deliberately ignored: the caller is closing the
// connection regardless, so there is nothing useful left to do with a
// write error here.
func writeBestEffort(conn net.Conn, w *resp.Writer, timeout time.Duration, v resp.Value) {
	if timeout > 0 {
		_ = conn.SetWriteDeadline(time.Now().Add(timeout))
	}
	_ = w.WriteValue(v)
	_ = w.Flush()
}
