package server

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/ShivharakhYadav/govalk/internal/resp"
)

func TestConnHandler_IdleTimeoutClosesConnection(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()

	handler := NewConnHandler(func(req resp.Value) resp.Value {
		t.Fatalf("handler must not be called: client never sends anything")
		return resp.Value{}
	}, ConnConfig{IdleTimeout: 50 * time.Millisecond})

	done := make(chan struct{})
	go func() {
		handler(serverConn)
		close(done)
	}()

	select {
	case <-done:
		// Handler returning is exactly what lets Server.handle's
		// deferred conn.Close() run in production.
	case <-time.After(2 * time.Second):
		t.Fatal("connection handler did not return after the idle timeout elapsed")
	}
}

func TestConnHandler_RequestReplyRoundTrip(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()

	handler := NewConnHandler(func(req resp.Value) resp.Value {
		return resp.NewSimpleString("PONG")
	}, ConnConfig{IdleTimeout: 2 * time.Second})

	go handler(serverConn)

	if _, err := clientConn.Write([]byte("PING\r\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := clientConn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	buf := make([]byte, 64)
	n, err := clientConn.Read(buf)
	if err != nil {
		t.Fatalf("read reply: %v", err)
	}
	got, want := string(buf[:n]), "+PONG\r\n"
	if got != want {
		t.Fatalf("reply = %q, want %q", got, want)
	}
}

func TestConnHandler_ProtocolErrorRepliesThenCloses(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()

	called := false
	handler := NewConnHandler(func(req resp.Value) resp.Value {
		called = true
		return resp.NewSimpleString("unused")
	}, ConnConfig{IdleTimeout: 2 * time.Second})

	done := make(chan struct{})
	go func() {
		handler(serverConn)
		close(done)
	}()

	// A nested array element with an unrecognized prefix - a genuine
	// protocol violation the reader cannot safely resynchronize from.
	if _, err := clientConn.Write([]byte("*1\r\nXhello\r\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := clientConn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	buf := make([]byte, 128)
	n, err := clientConn.Read(buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	got := string(buf[:n])
	if !strings.HasPrefix(got, "-ERR") {
		t.Fatalf("reply = %q, want a RESP error reply", got)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not return (allowing close) after the protocol error")
	}

	if called {
		t.Fatalf("request handler must not be invoked for a malformed request")
	}
}

func TestConnHandler_ClientDisconnectReturnsPromptly(t *testing.T) {
	clientConn, serverConn := net.Pipe()

	handler := NewConnHandler(func(req resp.Value) resp.Value {
		t.Fatalf("handler must not be called")
		return resp.Value{}
	}, ConnConfig{IdleTimeout: 5 * time.Second}) // deliberately long

	done := make(chan struct{})
	go func() {
		handler(serverConn)
		close(done)
	}()

	if err := clientConn.Close(); err != nil {
		t.Fatalf("client Close: %v", err)
	}

	select {
	case <-done:
		// Must return via the io.EOF path, well before the 5s idle
		// timeout - proves disconnect detection doesn't wait on it.
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not return promptly after client disconnect")
	}
}
