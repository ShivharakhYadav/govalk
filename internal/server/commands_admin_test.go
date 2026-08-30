package server

import (
	"testing"

	"github.com/ShivharakhYadav/govalk/internal/resp"
)

func TestPing_NoArgs(t *testing.T) {
	d := NewDispatcher()
	RegisterAdminCommands(d)

	reply := d.Dispatch(arrayCmd("PING"))
	if reply.Kind != resp.SimpleString || reply.Str != "PONG" {
		t.Fatalf("reply = %+v, want +PONG", reply)
	}
}

func TestPing_WithMessage(t *testing.T) {
	d := NewDispatcher()
	RegisterAdminCommands(d)

	reply := d.Dispatch(arrayCmd("PING", "hello"))
	if reply.Kind != resp.BulkString || string(reply.Bulk) != "hello" {
		t.Fatalf("reply = %+v, want bulk \"hello\"", reply)
	}
}

func TestPing_TooManyArgs(t *testing.T) {
	d := NewDispatcher()
	RegisterAdminCommands(d)

	reply := d.Dispatch(arrayCmd("PING", "a", "b"))
	if reply.Kind != resp.Error {
		t.Fatalf("reply = %+v, want an arity Error", reply)
	}
}

func TestEcho_Basic(t *testing.T) {
	d := NewDispatcher()
	RegisterAdminCommands(d)

	reply := d.Dispatch(arrayCmd("ECHO", "hello world"))
	if reply.Kind != resp.BulkString || string(reply.Bulk) != "hello world" {
		t.Fatalf("reply = %+v, want bulk \"hello world\"", reply)
	}
}

func TestEcho_WrongArity(t *testing.T) {
	d := NewDispatcher()
	RegisterAdminCommands(d)

	cases := []resp.Value{
		arrayCmd("ECHO"),
		arrayCmd("ECHO", "a", "b"),
	}
	for _, req := range cases {
		reply := d.Dispatch(req)
		if reply.Kind != resp.Error {
			t.Fatalf("reply = %+v, want an arity Error", reply)
		}
	}
}
