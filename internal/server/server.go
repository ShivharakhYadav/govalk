// Package server implements the TCP accept loop and connection lifecycle
// for the govalk RESP server.
package server

import (
	"errors"
	"log/slog"
	"net"
	"time"

	"github.com/ShivharakhYadav/govalk/internal/resp"
)

// Handler processes one accepted connection. It is called in its own
// goroutine per connection and is responsible for closing conn before
// returning (Server also closes it defensively afterward, so a Handler
// that forgets is not a leak, just redundant).
type Handler func(conn net.Conn)

// Config controls Server behavior.
type Config struct {
	// MaxConnections bounds the number of connections handled
	// concurrently. Connections beyond the cap are rejected immediately
	// with a RESP error reply rather than queued or blocked - graceful
	// degradation over collapse under heavy load (PLAN.md §3 "Heavy
	// load handling"). Values <= 0 are treated as 1.
	MaxConnections int

	// Logger receives accept-loop diagnostics (transient errors,
	// backoff). Defaults to slog.Default() if nil.
	Logger *slog.Logger
}

// Server accepts TCP connections on a net.Listener and dispatches each
// to a Handler in its own goroutine, subject to a bounded concurrent-
// connection cap.
type Server struct {
	ln      net.Listener
	handler Handler
	sem     chan struct{}
	logger  *slog.Logger
}

// New wraps an already-listening net.Listener. The caller owns creating
// (net.Listen) and, on shutdown (P19), closing ln.
func New(ln net.Listener, handler Handler, cfg Config) *Server {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	maxConn := cfg.MaxConnections
	if maxConn <= 0 {
		maxConn = 1
	}
	return &Server{
		ln:      ln,
		handler: handler,
		sem:     make(chan struct{}, maxConn),
		logger:  logger,
	}
}

// Serve runs the Accept loop until it hits a fatal error. A clean
// listener close (net.ErrClosed) is not an error from Serve's
// perspective: it returns nil, matching net.Listener.Accept's own
// documented shutdown contract.
//
// Any other Accept error is treated as transient - the same strategy
// net/http.Server.Serve has used for years, not the now-deprecated
// net.Error.Temporary() check - and retried with exponential backoff,
// starting at 5ms and capping at 1s, reset to 5ms after every successful
// Accept.
func (s *Server) Serve() error {
	const (
		minBackoff = 5 * time.Millisecond
		maxBackoff = 1 * time.Second
	)
	backoff := minBackoff

	for {
		conn, err := s.ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			s.logger.Warn("accept error, retrying", "error", err, "backoff", backoff)
			time.Sleep(backoff)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}
		backoff = minBackoff

		select {
		case s.sem <- struct{}{}:
			go s.handle(conn)
		default:
			s.reject(conn)
		}
	}
}

func (s *Server) handle(conn net.Conn) {
	defer func() { <-s.sem }()
	defer conn.Close()
	s.handler(conn)
}

// reject sends a RESP error reply and closes conn without ever spawning
// a handler goroutine for it, so a saturated connection cap degrades
// with a distinguishable error instead of an unbounded accept queue or a
// silent hang. See PLAN.md §3 "Heavy load handling".
func (s *Server) reject(conn net.Conn) {
	defer conn.Close()
	w := resp.NewWriter(conn)
	if err := w.WriteError("ERR max connections reached"); err != nil {
		return
	}
	_ = w.Flush()
}
