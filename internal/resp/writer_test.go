package resp

import (
	"bytes"
	"testing"
)

func encode(t *testing.T, v Value) string {
	t.Helper()
	var buf bytes.Buffer
	w := NewWriter(&buf)
	if err := w.WriteValue(v); err != nil {
		t.Fatalf("WriteValue(%+v) error: %v", v, err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush error: %v", err)
	}
	return buf.String()
}

func TestWriteValue_ByteExact(t *testing.T) {
	cases := []struct {
		name string
		v    Value
		want string
	}{
		{"simple string", NewSimpleString("OK"), "+OK\r\n"},
		{"error", NewError("ERR bad thing"), "-ERR bad thing\r\n"},
		{"integer", NewInteger(1000), ":1000\r\n"},
		{"negative integer", NewInteger(-1), ":-1\r\n"},
		{"bulk string", NewBulkString([]byte("hello")), "$5\r\nhello\r\n"},
		{"empty bulk string", NewBulkString([]byte{}), "$0\r\n\r\n"},
		{"null bulk string", NewNullBulkString(), "$-1\r\n"},
		{"empty array", NewArray([]Value{}), "*0\r\n"},
		{"null array", NewNullArray(), "*-1\r\n"},
		{
			"array of bulk strings",
			NewArray([]Value{NewBulkString([]byte("GET")), NewBulkString([]byte("foo"))}),
			"*2\r\n$3\r\nGET\r\n$3\r\nfoo\r\n",
		},
		{
			"nested array",
			NewArray([]Value{
				NewArray([]Value{NewInteger(1)}),
				NewBulkString([]byte("x")),
			}),
			"*2\r\n*1\r\n:1\r\n$1\r\nx\r\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := encode(t, c.v)
			if got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

// Bulk strings are binary-safe: embedded CRLF/NUL must be written
// verbatim, with the length prefix reflecting the raw byte count.
func TestWriteValue_BulkStringBinarySafe(t *testing.T) {
	payload := []byte("foo\r\nbar\x00baz")
	got := encode(t, NewBulkString(payload))
	want := "$12\r\n" + string(payload) + "\r\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestConvenienceMethods_MatchWriteValue(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	must(w.WriteSimpleString("OK"))
	must(w.WriteError("ERR x"))
	must(w.WriteInteger(42))
	must(w.WriteBulkString([]byte("hi")))
	must(w.WriteNullBulkString())
	must(w.WriteArray([]Value{NewInteger(1)}))
	must(w.WriteNullArray())
	must(w.Flush())

	want := "+OK\r\n" + "-ERR x\r\n" + ":42\r\n" + "$2\r\nhi\r\n" + "$-1\r\n" + "*1\r\n:1\r\n" + "*-1\r\n"
	if buf.String() != want {
		t.Fatalf("got %q, want %q", buf.String(), want)
	}
}

// Cross-validates the encoder against the P2 reader: whatever the Writer
// produces, the Reader must decode back to an identical Value. Catches
// asymmetries between the two implementations that isolated byte-exact
// tests on either side, alone, might miss.
func TestWriteThenRead_RoundTrip(t *testing.T) {
	values := []Value{
		NewSimpleString("OK"),
		NewError("ERR something"),
		NewInteger(0),
		NewInteger(-9223372036854775808),
		NewInteger(9223372036854775807),
		NewBulkString([]byte("hello")),
		NewBulkString([]byte{}),
		NewBulkString([]byte("foo\r\nbar\x00baz")),
		NewNullBulkString(),
		NewArray([]Value{}),
		NewNullArray(),
		NewArray([]Value{
			NewBulkString([]byte("SET")),
			NewBulkString([]byte("key")),
			NewBulkString([]byte("val with spaces and \x00 nul")),
		}),
		NewArray([]Value{
			NewArray([]Value{NewInteger(1), NewInteger(2)}),
			NewNullBulkString(),
			NewSimpleString("tail"),
		}),
	}

	for i, v := range values {
		var buf bytes.Buffer
		w := NewWriter(&buf)
		if err := w.WriteValue(v); err != nil {
			t.Fatalf("case %d: WriteValue error: %v", i, err)
		}
		if err := w.Flush(); err != nil {
			t.Fatalf("case %d: Flush error: %v", i, err)
		}

		r := NewReader(&buf)
		got, err := r.ReadValue()
		if err != nil {
			t.Fatalf("case %d: ReadValue error: %v", i, err)
		}
		if !valuesEqual(got, v) {
			t.Fatalf("case %d: round-trip mismatch: got %+v, want %+v", i, got, v)
		}
	}
}

// valuesEqual compares two Values the way the round-trip test needs:
// treating a nil and an empty-but-present Bulk/Elems as equal, since
// that distinction is a Go-slice implementation detail, not part of the
// RESP value itself (both encode identically, e.g. "$0\r\n\r\n").
func valuesEqual(a, b Value) bool {
	if a.Kind != b.Kind || a.Null != b.Null {
		return false
	}
	switch a.Kind {
	case SimpleString, Error:
		return a.Str == b.Str
	case Integer:
		return a.Int == b.Int
	case BulkString:
		return bytes.Equal(a.Bulk, b.Bulk)
	case Array:
		if len(a.Elems) != len(b.Elems) {
			return false
		}
		for i := range a.Elems {
			if !valuesEqual(a.Elems[i], b.Elems[i]) {
				return false
			}
		}
		return true
	default:
		return false
	}
}
