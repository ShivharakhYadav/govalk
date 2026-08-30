package server

import (
	"testing"

	"github.com/ShivharakhYadav/govalk/internal/resp"
	"github.com/ShivharakhYadav/govalk/internal/store"
)

// WRONGTYPE propagation through GET is exercised end-to-end once list
// commands exist (P16) - there's no public way to put a non-string
// value in a Store yet, so there's nothing genuine to trigger it with
// here. store_test.go already covers Store.Get's WRONGTYPE behavior
// directly; this file only needs to prove GET forwards whatever error
// Store.Get returns, which the store-level test plus this package's
// error-propagation pattern in dispatch.go together already establish.

func newTestDispatcher() *Dispatcher {
	d := NewDispatcher()
	RegisterStringCommands(d, store.New())
	return d
}

func TestGet_MissingKeyReturnsNullBulk(t *testing.T) {
	d := newTestDispatcher()
	reply := d.Dispatch(arrayCmd("GET", "nope"))
	if reply.Kind != resp.BulkString || !reply.Null {
		t.Fatalf("reply = %+v, want a Null Bulk String", reply)
	}
}

func TestSetGet_RoundTrip(t *testing.T) {
	d := newTestDispatcher()

	setReply := d.Dispatch(arrayCmd("SET", "k", "v"))
	if setReply.Kind != resp.SimpleString || setReply.Str != "OK" {
		t.Fatalf("SET reply = %+v, want +OK", setReply)
	}

	getReply := d.Dispatch(arrayCmd("GET", "k"))
	if getReply.Kind != resp.BulkString || getReply.Null || string(getReply.Bulk) != "v" {
		t.Fatalf("GET reply = %+v, want bulk \"v\"", getReply)
	}
}

func TestSet_Overwrites(t *testing.T) {
	d := newTestDispatcher()
	d.Dispatch(arrayCmd("SET", "k", "v1"))
	d.Dispatch(arrayCmd("SET", "k", "v2"))

	reply := d.Dispatch(arrayCmd("GET", "k"))
	if string(reply.Bulk) != "v2" {
		t.Fatalf("GET after overwrite = %+v, want bulk \"v2\"", reply)
	}
}

func TestDel_SingleKey(t *testing.T) {
	d := newTestDispatcher()
	d.Dispatch(arrayCmd("SET", "k", "v"))

	reply := d.Dispatch(arrayCmd("DEL", "k"))
	if reply.Kind != resp.Integer || reply.Int != 1 {
		t.Fatalf("DEL existing key reply = %+v, want :1", reply)
	}

	reply = d.Dispatch(arrayCmd("DEL", "k"))
	if reply.Kind != resp.Integer || reply.Int != 0 {
		t.Fatalf("DEL already-deleted key reply = %+v, want :0", reply)
	}

	getReply := d.Dispatch(arrayCmd("GET", "k"))
	if !getReply.Null {
		t.Fatalf("GET after DEL = %+v, want Null Bulk String", getReply)
	}
}

func TestDel_MultipleKeysMixed(t *testing.T) {
	d := newTestDispatcher()
	d.Dispatch(arrayCmd("SET", "a", "1"))
	d.Dispatch(arrayCmd("SET", "b", "2"))
	// "c" deliberately never set.

	reply := d.Dispatch(arrayCmd("DEL", "a", "b", "c"))
	if reply.Kind != resp.Integer || reply.Int != 2 {
		t.Fatalf("DEL a b c reply = %+v, want :2 (a and b existed, c did not)", reply)
	}
}

func TestExists_Variadic(t *testing.T) {
	d := newTestDispatcher()
	d.Dispatch(arrayCmd("SET", "a", "1"))

	reply := d.Dispatch(arrayCmd("EXISTS", "a", "a", "missing"))
	if reply.Kind != resp.Integer || reply.Int != 2 {
		t.Fatalf("EXISTS a a missing reply = %+v, want :2 (a counted twice, missing not at all)", reply)
	}
}

func TestGet_WrongArity(t *testing.T) {
	d := newTestDispatcher()
	for _, req := range []resp.Value{arrayCmd("GET"), arrayCmd("GET", "a", "b")} {
		if reply := d.Dispatch(req); reply.Kind != resp.Error {
			t.Fatalf("GET wrong arity reply = %+v, want Error", reply)
		}
	}
}

func TestSet_WrongArity(t *testing.T) {
	d := newTestDispatcher()
	for _, req := range []resp.Value{arrayCmd("SET"), arrayCmd("SET", "k"), arrayCmd("SET", "k", "v", "extra")} {
		if reply := d.Dispatch(req); reply.Kind != resp.Error {
			t.Fatalf("SET wrong arity reply = %+v, want Error", reply)
		}
	}
}

func TestDelExists_RequireAtLeastOneKey(t *testing.T) {
	d := newTestDispatcher()
	if reply := d.Dispatch(arrayCmd("DEL")); reply.Kind != resp.Error {
		t.Fatalf("DEL with no keys reply = %+v, want Error", reply)
	}
	if reply := d.Dispatch(arrayCmd("EXISTS")); reply.Kind != resp.Error {
		t.Fatalf("EXISTS with no keys reply = %+v, want Error", reply)
	}
}
