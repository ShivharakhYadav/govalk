package resp

import (
	"errors"
	"io"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func readAll(t *testing.T, input string) []Value {
	t.Helper()
	r := NewReader(strings.NewReader(input))
	var out []Value
	for {
		v, err := r.ReadValue()
		if err == io.EOF {
			return out
		}
		if err != nil {
			t.Fatalf("unexpected error reading %q: %v", input, err)
		}
		out = append(out, v)
	}
}

func TestReadValue_ValidTypes(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  Value
	}{
		{"simple string", "+OK\r\n", NewSimpleString("OK")},
		{"error", "-ERR bad thing\r\n", NewError("ERR bad thing")},
		{"integer", ":1000\r\n", NewInteger(1000)},
		{"negative integer", ":-1\r\n", NewInteger(-1)},
		{"bulk string", "$5\r\nhello\r\n", NewBulkString([]byte("hello"))},
		{"empty bulk string", "$0\r\n\r\n", NewBulkString([]byte{})},
		{"null bulk string", "$-1\r\n", NewNullBulkString()},
		{"empty array", "*0\r\n", NewArray([]Value{})},
		{"null array", "*-1\r\n", NewNullArray()},
		{
			"array of bulk strings",
			"*2\r\n$3\r\nGET\r\n$3\r\nfoo\r\n",
			NewArray([]Value{NewBulkString([]byte("GET")), NewBulkString([]byte("foo"))}),
		},
		{
			"nested array",
			"*2\r\n*1\r\n:1\r\n$1\r\nx\r\n",
			NewArray([]Value{
				NewArray([]Value{NewInteger(1)}),
				NewBulkString([]byte("x")),
			}),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := readAll(t, c.input)
			if len(got) != 1 {
				t.Fatalf("got %d values, want 1: %+v", len(got), got)
			}
			if !reflect.DeepEqual(got[0], c.want) {
				t.Fatalf("got %+v, want %+v", got[0], c.want)
			}
		})
	}
}

// Bulk strings are binary-safe: embedded CRLF and NUL bytes must survive
// intact, never be mistaken for the record's own terminator.
func TestReadValue_BulkStringBinarySafe(t *testing.T) {
	payload := []byte("foo\r\nbar\x00baz")
	input := "$" + strconv.Itoa(len(payload)) + "\r\n" + string(payload) + "\r\n"

	got := readAll(t, input)
	if len(got) != 1 {
		t.Fatalf("got %d values, want 1", len(got))
	}
	if got[0].Kind != BulkString {
		t.Fatalf("Kind = %v, want BulkString", got[0].Kind)
	}
	if !reflect.DeepEqual(got[0].Bulk, payload) {
		t.Fatalf("Bulk = %q, want %q", got[0].Bulk, payload)
	}
}

func TestReadValue_Inline(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string // expected bulk-string field values
	}{
		{"simple", "PING\r\n", []string{"PING"}},
		{"multi-word", "SET foo bar\r\n", []string{"SET", "foo", "bar"}},
		{"bare LF, no CR", "PING\n", []string{"PING"}},
		{"extra whitespace", "SET   foo   bar\r\n", []string{"SET", "foo", "bar"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := readAll(t, c.input)
			if len(got) != 1 {
				t.Fatalf("got %d values, want 1", len(got))
			}
			v := got[0]
			if v.Kind != Array {
				t.Fatalf("Kind = %v, want Array", v.Kind)
			}
			if len(v.Elems) != len(c.want) {
				t.Fatalf("got %d fields, want %d: %+v", len(v.Elems), len(c.want), v.Elems)
			}
			for i, want := range c.want {
				if v.Elems[i].Kind != BulkString || string(v.Elems[i].Bulk) != want {
					t.Fatalf("field %d = %+v, want BulkString %q", i, v.Elems[i], want)
				}
			}
		})
	}
}

func TestReadValue_MultipleValuesSequentially(t *testing.T) {
	// Two inline commands back to back, as a real connection would send.
	got := readAll(t, "PING\r\nPING\r\n")
	if len(got) != 2 {
		t.Fatalf("got %d values, want 2", len(got))
	}
}

func TestReadValue_CleanEOFAtBoundary(t *testing.T) {
	r := NewReader(strings.NewReader(""))
	_, err := r.ReadValue()
	if err != io.EOF {
		t.Fatalf("err = %v, want exactly io.EOF", err)
	}
}

func TestReadValue_ProtocolErrors(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"malformed nested prefix", "*1\r\nXhello\r\n"},
		{"typed header missing CR", "+OK\n"},
		{"invalid integer", ":notanumber\r\n"},
		{"invalid bulk length", "$notanumber\r\nhello\r\n"},
		{"negative bulk length other than -1", "$-2\r\n"},
		{"negative array length other than -1", "*-2\r\n"},
		{"oversized bulk length", "$99999999999999\r\n"},
		{"oversized array length", "*99999999999999\r\n"},
		{"bulk string missing terminating CRLF", "$5\r\nhelloXX"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := NewReader(strings.NewReader(c.input))
			_, err := r.ReadValue()
			if err == nil {
				t.Fatalf("got nil error, want an error wrapping ErrProtocol")
			}
			if !errors.Is(err, ErrProtocol) {
				t.Fatalf("err = %v, want it to wrap ErrProtocol", err)
			}
		})
	}
}

func TestReadValue_TruncatedStreamMidValue(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"truncated bulk payload", "$5\r\nhel"},
		{"truncated array (missing element)", "*2\r\n$3\r\nGET\r\n"},
		{"truncated header line", "$5"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := NewReader(strings.NewReader(c.input))
			_, err := r.ReadValue()
			if !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Fatalf("err = %v, want io.ErrUnexpectedEOF", err)
			}
		})
	}
}
