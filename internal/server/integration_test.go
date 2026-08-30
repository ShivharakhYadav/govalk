package server

import (
	"net"
	"testing"
	"time"

	"github.com/ShivharakhYadav/govalk/internal/resp"
	"github.com/ShivharakhYadav/govalk/internal/store"
)

// TestPingEcho_EndToEnd is the P7 checkpoint: a real TCP listener, the
// real Accept loop (P4), the real connection handler (P5), and the real
// dispatcher (P6) wired together and driven over an actual OS socket -
// no fakes. redis-cli isn't available in this environment (Redis has no
// official Windows build), so this automated round trip is the stand-in
// manual-verification step the plan calls for.
func TestPingEcho_EndToEnd(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer ln.Close()

	d := NewDispatcher()
	RegisterAdminCommands(d)
	handler := NewConnHandler(d.Dispatch, ConnConfig{IdleTimeout: 5 * time.Second})
	srv := New(ln, handler, Config{MaxConnections: 8, Logger: testLogger()})

	serveDone := make(chan error, 1)
	go func() { serveDone <- srv.Serve() }()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("net.Dial: %v", err)
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("SetDeadline: %v", err)
	}

	r := resp.NewReader(conn)
	w := resp.NewWriter(conn)

	send := func(t *testing.T, args ...string) resp.Value {
		t.Helper()
		elems := make([]resp.Value, len(args))
		for i, a := range args {
			elems[i] = resp.NewBulkString([]byte(a))
		}
		if err := w.WriteValue(resp.NewArray(elems)); err != nil {
			t.Fatalf("write %v: %v", args, err)
		}
		if err := w.Flush(); err != nil {
			t.Fatalf("flush %v: %v", args, err)
		}
		reply, err := r.ReadValue()
		if err != nil {
			t.Fatalf("read reply to %v: %v", args, err)
		}
		return reply
	}

	if reply := send(t, "PING"); reply.Kind != resp.SimpleString || reply.Str != "PONG" {
		t.Fatalf("PING reply = %+v, want +PONG", reply)
	}

	if reply := send(t, "PING", "hello"); reply.Kind != resp.BulkString || string(reply.Bulk) != "hello" {
		t.Fatalf("PING hello reply = %+v, want bulk \"hello\"", reply)
	}

	if reply := send(t, "ECHO", "round trip"); reply.Kind != resp.BulkString || string(reply.Bulk) != "round trip" {
		t.Fatalf("ECHO reply = %+v, want bulk \"round trip\"", reply)
	}

	// An ordinary command-level error (unknown command) must reply with
	// an error but NOT close the connection - only a wire-level
	// protocol violation does that (see conn.go). Prove the connection
	// is still usable afterward.
	if reply := send(t, "FROB"); reply.Kind != resp.Error {
		t.Fatalf("FROB reply = %+v, want an Error", reply)
	}
	if reply := send(t, "PING"); reply.Kind != resp.SimpleString || reply.Str != "PONG" {
		t.Fatalf("PING after unknown command = %+v, want +PONG (connection must stay open)", reply)
	}

	conn.Close()
	ln.Close()
	select {
	case err := <-serveDone:
		if err != nil {
			t.Fatalf("Serve returned %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not return after listener was closed")
	}
}

// Exercises the inline-command path (P2's readInline) over the same
// real end-to-end stack, since PING/ECHO are also the first commands
// that can be driven that way, telnet/nc-style.
func TestPingEcho_InlineCommand(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer ln.Close()

	d := NewDispatcher()
	RegisterAdminCommands(d)
	handler := NewConnHandler(d.Dispatch, ConnConfig{IdleTimeout: 5 * time.Second})
	srv := New(ln, handler, Config{MaxConnections: 8, Logger: testLogger()})
	go srv.Serve()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("net.Dial: %v", err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("SetDeadline: %v", err)
	}

	if _, err := conn.Write([]byte("PING\r\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got, want := string(buf[:n]), "+PONG\r\n"; got != want {
		t.Fatalf("reply = %q, want %q", got, want)
	}
}

// TestStringCommands_EndToEnd is the P9 checkpoint: GET/SET/DEL/EXISTS
// driven over a real socket against a real Store, the same full stack
// (listener -> Accept loop -> connection handler -> dispatcher) as
// TestPingEcho_EndToEnd.
func TestStringCommands_EndToEnd(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer ln.Close()

	d := NewDispatcher()
	RegisterAdminCommands(d)
	RegisterStringCommands(d, store.New())
	handler := NewConnHandler(d.Dispatch, ConnConfig{IdleTimeout: 5 * time.Second})
	srv := New(ln, handler, Config{MaxConnections: 8, Logger: testLogger()})
	go srv.Serve()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("net.Dial: %v", err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("SetDeadline: %v", err)
	}

	r := resp.NewReader(conn)
	w := resp.NewWriter(conn)

	send := func(t *testing.T, args ...string) resp.Value {
		t.Helper()
		elems := make([]resp.Value, len(args))
		for i, a := range args {
			elems[i] = resp.NewBulkString([]byte(a))
		}
		if err := w.WriteValue(resp.NewArray(elems)); err != nil {
			t.Fatalf("write %v: %v", args, err)
		}
		if err := w.Flush(); err != nil {
			t.Fatalf("flush %v: %v", args, err)
		}
		reply, err := r.ReadValue()
		if err != nil {
			t.Fatalf("read reply to %v: %v", args, err)
		}
		return reply
	}

	if reply := send(t, "GET", "foo"); reply.Kind != resp.BulkString || !reply.Null {
		t.Fatalf("GET on missing key = %+v, want Null Bulk String", reply)
	}

	if reply := send(t, "SET", "foo", "bar"); reply.Kind != resp.SimpleString || reply.Str != "OK" {
		t.Fatalf("SET reply = %+v, want +OK", reply)
	}

	if reply := send(t, "GET", "foo"); reply.Kind != resp.BulkString || string(reply.Bulk) != "bar" {
		t.Fatalf("GET reply = %+v, want bulk \"bar\"", reply)
	}

	if reply := send(t, "EXISTS", "foo", "missing"); reply.Kind != resp.Integer || reply.Int != 1 {
		t.Fatalf("EXISTS reply = %+v, want :1", reply)
	}

	if reply := send(t, "DEL", "foo"); reply.Kind != resp.Integer || reply.Int != 1 {
		t.Fatalf("DEL reply = %+v, want :1", reply)
	}

	if reply := send(t, "GET", "foo"); !reply.Null {
		t.Fatalf("GET after DEL = %+v, want Null Bulk String", reply)
	}
}

// TestTTLCommands_EndToEnd is the P10 checkpoint: EXPIRE/TTL/PERSIST
// driven over a real socket against a real Store, same full stack as
// the other end-to-end tests in this file.
func TestTTLCommands_EndToEnd(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer ln.Close()

	d := NewDispatcher()
	RegisterAdminCommands(d)
	s := store.New()
	RegisterStringCommands(d, s)
	RegisterTTLCommands(d, s)
	handler := NewConnHandler(d.Dispatch, ConnConfig{IdleTimeout: 5 * time.Second})
	srv := New(ln, handler, Config{MaxConnections: 8, Logger: testLogger()})
	go srv.Serve()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("net.Dial: %v", err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("SetDeadline: %v", err)
	}

	r := resp.NewReader(conn)
	w := resp.NewWriter(conn)

	send := func(t *testing.T, args ...string) resp.Value {
		t.Helper()
		elems := make([]resp.Value, len(args))
		for i, a := range args {
			elems[i] = resp.NewBulkString([]byte(a))
		}
		if err := w.WriteValue(resp.NewArray(elems)); err != nil {
			t.Fatalf("write %v: %v", args, err)
		}
		if err := w.Flush(); err != nil {
			t.Fatalf("flush %v: %v", args, err)
		}
		reply, err := r.ReadValue()
		if err != nil {
			t.Fatalf("read reply to %v: %v", args, err)
		}
		return reply
	}

	send(t, "SET", "k", "v")

	if reply := send(t, "TTL", "k"); reply.Int != -1 {
		t.Fatalf("TTL before EXPIRE = %+v, want :-1", reply)
	}

	if reply := send(t, "EXPIRE", "k", "100"); reply.Int != 1 {
		t.Fatalf("EXPIRE reply = %+v, want :1", reply)
	}

	if reply := send(t, "TTL", "k"); reply.Int <= 0 || reply.Int > 100 {
		t.Fatalf("TTL after EXPIRE = %+v, want a positive integer <= 100", reply)
	}

	if reply := send(t, "PERSIST", "k"); reply.Int != 1 {
		t.Fatalf("PERSIST reply = %+v, want :1", reply)
	}

	if reply := send(t, "TTL", "k"); reply.Int != -1 {
		t.Fatalf("TTL after PERSIST = %+v, want :-1", reply)
	}

	if reply := send(t, "GET", "k"); reply.Null || string(reply.Bulk) != "v" {
		t.Fatalf("GET at the end = %+v, want bulk \"v\" (value survived the whole TTL dance)", reply)
	}
}
