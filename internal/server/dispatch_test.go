package server

import (
	"reflect"
	"strings"
	"testing"

	"github.com/ShivharakhYadav/govalk/internal/resp"
)

func arrayCmd(fields ...string) resp.Value {
	elems := make([]resp.Value, len(fields))
	for i, f := range fields {
		elems[i] = resp.NewBulkString([]byte(f))
	}
	return resp.NewArray(elems)
}

func TestDispatch_UnknownCommand(t *testing.T) {
	d := NewDispatcher()
	reply := d.Dispatch(arrayCmd("FROB", "x"))

	if reply.Kind != resp.Error {
		t.Fatalf("Kind = %v, want Error", reply.Kind)
	}
	if !strings.Contains(reply.Str, "unknown command") || !strings.Contains(reply.Str, "FROB") {
		t.Fatalf("reply = %q, want it to mention unknown command FROB", reply.Str)
	}
}

func TestDispatch_WrongArity(t *testing.T) {
	d := NewDispatcher()
	called := false
	d.Register("SET", 2, 2, func(args [][]byte) resp.Value {
		called = true
		return resp.NewSimpleString("OK")
	})

	cases := []struct {
		name string
		req  resp.Value
	}{
		{"too few", arrayCmd("SET", "key")},
		{"too many", arrayCmd("SET", "key", "val", "extra")},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			called = false
			reply := d.Dispatch(c.req)
			if reply.Kind != resp.Error {
				t.Fatalf("Kind = %v, want Error", reply.Kind)
			}
			want := "ERR wrong number of arguments for 'set' command"
			if reply.Str != want {
				t.Fatalf("reply = %q, want %q", reply.Str, want)
			}
			if called {
				t.Fatalf("command handler must not run when arity check fails")
			}
		})
	}
}

func TestDispatch_UnboundedMaxArgs(t *testing.T) {
	d := NewDispatcher()
	var gotArgs [][]byte
	d.Register("MSET", 2, -1, func(args [][]byte) resp.Value {
		gotArgs = args
		return resp.NewSimpleString("OK")
	})

	reply := d.Dispatch(arrayCmd("MSET", "k1", "v1", "k2", "v2", "k3", "v3"))
	if reply.Kind != resp.SimpleString || reply.Str != "OK" {
		t.Fatalf("reply = %+v, want +OK", reply)
	}
	want := [][]byte{[]byte("k1"), []byte("v1"), []byte("k2"), []byte("v2"), []byte("k3"), []byte("v3")}
	if !reflect.DeepEqual(gotArgs, want) {
		t.Fatalf("args = %v, want %v", gotArgs, want)
	}
}

func TestDispatch_CorrectInvocationAndCaseInsensitiveName(t *testing.T) {
	d := NewDispatcher()
	var gotArgs [][]byte
	d.Register("get", 1, 1, func(args [][]byte) resp.Value {
		gotArgs = args
		return resp.NewBulkString([]byte("value-for-" + string(args[0])))
	})

	// Registered lowercase, dispatched via a mixed-case wire command
	// name - both must resolve to the same handler.
	reply := d.Dispatch(arrayCmd("GeT", "foo"))

	if reply.Kind != resp.BulkString || string(reply.Bulk) != "value-for-foo" {
		t.Fatalf("reply = %+v, want bulk \"value-for-foo\"", reply)
	}
	if !reflect.DeepEqual(gotArgs, [][]byte{[]byte("foo")}) {
		t.Fatalf("args = %v, want [foo]", gotArgs)
	}
}

func TestDispatch_MalformedRequestNeverReachesHandlers(t *testing.T) {
	d := NewDispatcher()
	called := false
	d.Register("PING", 0, 0, func(args [][]byte) resp.Value {
		called = true
		return resp.NewSimpleString("PONG")
	})

	reply := d.Dispatch(resp.NewInteger(5)) // not even shaped like a command

	if reply.Kind != resp.Error {
		t.Fatalf("Kind = %v, want Error", reply.Kind)
	}
	if called {
		t.Fatalf("no command handler should run for a malformed request")
	}
}
