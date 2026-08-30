package server

import (
	"errors"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// fakeAcceptResult is one pre-programmed return value for fakeListener's
// Accept method.
type fakeAcceptResult struct {
	conn net.Conn
	err  error
}

// fakeListener implements net.Listener by replaying a fixed, pre-
// programmed sequence of Accept results. Once the sequence is exhausted
// (the channel is closed), it returns net.ErrClosed, so Serve's normal
// shutdown path exits the test cleanly without needing a separate
// goroutine to call Close.
type fakeListener struct {
	results chan fakeAcceptResult
}

func (f *fakeListener) Accept() (net.Conn, error) {
	r, ok := <-f.results
	if !ok {
		return nil, net.ErrClosed
	}
	return r.conn, r.err
}

func (f *fakeListener) Close() error   { return nil }
func (f *fakeListener) Addr() net.Addr { return fakeAddr{} }

type fakeAddr struct{}

func (fakeAddr) Network() string { return "fake" }
func (fakeAddr) String() string  { return "fake" }

func TestServe_TransientErrorThenSuccess(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()

	results := make(chan fakeAcceptResult, 2)
	results <- fakeAcceptResult{err: errors.New("transient accept error")}
	results <- fakeAcceptResult{conn: serverConn}
	close(results) // next Accept() call returns net.ErrClosed, ending Serve

	ln := &fakeListener{results: results}

	handled := make(chan net.Conn, 1)
	handler := func(c net.Conn) { handled <- c }

	srv := New(ln, handler, Config{MaxConnections: 1, Logger: testLogger()})

	done := make(chan error, 1)
	go func() { done <- srv.Serve() }()

	select {
	case c := <-handled:
		if c != serverConn {
			t.Fatalf("handler got the wrong conn")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("handler was never called after transient error + successful accept")
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve returned %v, want nil (clean stop on net.ErrClosed)", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not return after the listener's result stream closed")
	}
}

func TestServe_ConnectionCapRejectsExcess(t *testing.T) {
	clientConn1, serverConn1 := net.Pipe()
	clientConn2, serverConn2 := net.Pipe()
	defer clientConn1.Close()
	defer clientConn2.Close()

	results := make(chan fakeAcceptResult, 2)
	results <- fakeAcceptResult{conn: serverConn1}
	results <- fakeAcceptResult{conn: serverConn2}
	close(results)

	ln := &fakeListener{results: results}

	handlerCalled := make(chan net.Conn, 2)
	release := make(chan struct{})
	handler := func(c net.Conn) {
		handlerCalled <- c
		<-release // hold the only slot open until the test says so
	}

	srv := New(ln, handler, Config{MaxConnections: 1, Logger: testLogger()})

	done := make(chan error, 1)
	go func() { done <- srv.Serve() }()
	defer close(release)

	// The first connection should occupy the single available slot.
	select {
	case c := <-handlerCalled:
		if c != serverConn1 {
			t.Fatalf("handler got the wrong conn for slot 1")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("handler was never called for the first connection")
	}

	// The second connection must be rejected: the client end reads a
	// RESP error reply. This read also unblocks Server.reject's
	// synchronous write on the net.Pipe, letting Serve proceed to its
	// next Accept call.
	buf := make([]byte, 256)
	if err := clientConn2.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	n, err := clientConn2.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("reading rejection reply: %v", err)
	}
	got := string(buf[:n])
	want := "-ERR max connections reached\r\n"
	if got != want {
		t.Fatalf("rejection reply = %q, want %q", got, want)
	}

	select {
	case c := <-handlerCalled:
		t.Fatalf("handler must not be called for a rejected connection, got %v", c)
	default:
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve returned %v, want nil (clean stop on net.ErrClosed)", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not return after the listener's result stream closed")
	}
}
