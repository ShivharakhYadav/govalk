package server

import (
	"testing"

	"github.com/ShivharakhYadav/govalk/internal/resp"
)

func TestExpire_PositiveSecondsSetsTTLAndSurvivesInValue(t *testing.T) {
	d := newTestDispatcher()
	d.Dispatch(arrayCmd("SET", "k", "v"))

	reply := d.Dispatch(arrayCmd("EXPIRE", "k", "100"))
	if reply.Kind != resp.Integer || reply.Int != 1 {
		t.Fatalf("EXPIRE reply = %+v, want :1", reply)
	}

	// The value itself must survive - EXPIRE only touches metadata.
	getReply := d.Dispatch(arrayCmd("GET", "k"))
	if getReply.Kind != resp.BulkString || string(getReply.Bulk) != "v" {
		t.Fatalf("GET after EXPIRE = %+v, want bulk \"v\"", getReply)
	}

	ttlReply := d.Dispatch(arrayCmd("TTL", "k"))
	if ttlReply.Kind != resp.Integer || ttlReply.Int <= 0 || ttlReply.Int > 100 {
		t.Fatalf("TTL reply = %+v, want a positive integer <= 100", ttlReply)
	}
}

func TestExpire_MissingKey(t *testing.T) {
	d := newTestDispatcher()
	reply := d.Dispatch(arrayCmd("EXPIRE", "nope", "100"))
	if reply.Kind != resp.Integer || reply.Int != 0 {
		t.Fatalf("EXPIRE on missing key reply = %+v, want :0", reply)
	}
}

// Non-positive seconds is the plan's explicit P10 edge case: EXPIRE
// with a zero or negative TTL deletes the key immediately rather than
// setting a (nonsensical) past deadline.
func TestExpire_ZeroOrNegativeSecondsDeletesImmediately(t *testing.T) {
	for _, seconds := range []string{"0", "-1", "-1000"} {
		t.Run(seconds, func(t *testing.T) {
			d := newTestDispatcher()
			d.Dispatch(arrayCmd("SET", "k", "v"))

			reply := d.Dispatch(arrayCmd("EXPIRE", "k", seconds))
			if reply.Kind != resp.Integer || reply.Int != 1 {
				t.Fatalf("EXPIRE k %s reply = %+v, want :1 (key existed and was deleted)", seconds, reply)
			}

			getReply := d.Dispatch(arrayCmd("GET", "k"))
			if !getReply.Null {
				t.Fatalf("GET after EXPIRE k %s = %+v, want Null Bulk String (key deleted)", seconds, getReply)
			}
		})
	}
}

func TestExpire_OverflowSecondsRejected(t *testing.T) {
	d := newTestDispatcher()
	d.Dispatch(arrayCmd("SET", "k", "v"))

	reply := d.Dispatch(arrayCmd("EXPIRE", "k", "9223372036854775807")) // math.MaxInt64
	if reply.Kind != resp.Error {
		t.Fatalf("EXPIRE with math.MaxInt64 seconds = %+v, want an Error", reply)
	}

	// Must not have touched the key at all.
	getReply := d.Dispatch(arrayCmd("GET", "k"))
	if getReply.Null || string(getReply.Bulk) != "v" {
		t.Fatalf("GET after rejected EXPIRE = %+v, want the value untouched", getReply)
	}
}

func TestExpire_NonIntegerSeconds(t *testing.T) {
	d := newTestDispatcher()
	d.Dispatch(arrayCmd("SET", "k", "v"))

	reply := d.Dispatch(arrayCmd("EXPIRE", "k", "notanumber"))
	if reply.Kind != resp.Error {
		t.Fatalf("EXPIRE with non-integer seconds = %+v, want an Error", reply)
	}
}

func TestTTL_States(t *testing.T) {
	d := newTestDispatcher()

	if reply := d.Dispatch(arrayCmd("TTL", "nope")); reply.Kind != resp.Integer || reply.Int != -2 {
		t.Fatalf("TTL on missing key = %+v, want :-2", reply)
	}

	d.Dispatch(arrayCmd("SET", "k", "v"))
	if reply := d.Dispatch(arrayCmd("TTL", "k")); reply.Kind != resp.Integer || reply.Int != -1 {
		t.Fatalf("TTL on key with no expiry = %+v, want :-1", reply)
	}

	d.Dispatch(arrayCmd("EXPIRE", "k", "100"))
	if reply := d.Dispatch(arrayCmd("TTL", "k")); reply.Kind != resp.Integer || reply.Int <= 0 {
		t.Fatalf("TTL on key with an expiry = %+v, want a positive integer", reply)
	}
}

func TestPersist_ClearsTTL(t *testing.T) {
	d := newTestDispatcher()
	d.Dispatch(arrayCmd("SET", "k", "v"))
	d.Dispatch(arrayCmd("EXPIRE", "k", "100"))

	reply := d.Dispatch(arrayCmd("PERSIST", "k"))
	if reply.Kind != resp.Integer || reply.Int != 1 {
		t.Fatalf("PERSIST reply = %+v, want :1", reply)
	}

	ttlReply := d.Dispatch(arrayCmd("TTL", "k"))
	if ttlReply.Kind != resp.Integer || ttlReply.Int != -1 {
		t.Fatalf("TTL after PERSIST = %+v, want :-1 (no TTL)", ttlReply)
	}

	// The value itself must survive PERSIST.
	getReply := d.Dispatch(arrayCmd("GET", "k"))
	if getReply.Null || string(getReply.Bulk) != "v" {
		t.Fatalf("GET after PERSIST = %+v, want bulk \"v\"", getReply)
	}
}

func TestPersist_NoTTLOrMissingKey(t *testing.T) {
	d := newTestDispatcher()

	if reply := d.Dispatch(arrayCmd("PERSIST", "nope")); reply.Kind != resp.Integer || reply.Int != 0 {
		t.Fatalf("PERSIST on missing key = %+v, want :0", reply)
	}

	d.Dispatch(arrayCmd("SET", "k", "v"))
	if reply := d.Dispatch(arrayCmd("PERSIST", "k")); reply.Kind != resp.Integer || reply.Int != 0 {
		t.Fatalf("PERSIST on key with no TTL = %+v, want :0", reply)
	}
}

func TestTTLCommands_WrongArity(t *testing.T) {
	d := newTestDispatcher()
	cases := []resp.Value{
		arrayCmd("EXPIRE", "k"),
		arrayCmd("EXPIRE", "k", "1", "extra"),
		arrayCmd("TTL"),
		arrayCmd("TTL", "k", "extra"),
		arrayCmd("PERSIST"),
		arrayCmd("PERSIST", "k", "extra"),
	}
	for _, req := range cases {
		if reply := d.Dispatch(req); reply.Kind != resp.Error {
			t.Fatalf("%+v reply = %+v, want an arity Error", req, reply)
		}
	}
}
