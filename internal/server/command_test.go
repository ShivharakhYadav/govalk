package server

import (
	"reflect"
	"testing"

	"github.com/ShivharakhYadav/govalk/internal/resp"
)

func TestCommandFromValue_Valid(t *testing.T) {
	v := resp.NewArray([]resp.Value{
		resp.NewBulkString([]byte("get")),
		resp.NewBulkString([]byte("foo")),
	})
	cmd, err := commandFromValue(v)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd.Name != "GET" {
		t.Fatalf("Name = %q, want %q (uppercased)", cmd.Name, "GET")
	}
	want := [][]byte{[]byte("foo")}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Fatalf("Args = %v, want %v", cmd.Args, want)
	}
}

func TestCommandFromValue_NameOnlyNoArgs(t *testing.T) {
	v := resp.NewArray([]resp.Value{resp.NewBulkString([]byte("PING"))})
	cmd, err := commandFromValue(v)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd.Name != "PING" || len(cmd.Args) != 0 {
		t.Fatalf("got %+v, want Name=PING Args=[]", cmd)
	}
}

func TestCommandFromValue_Invalid(t *testing.T) {
	cases := []struct {
		name string
		v    resp.Value
	}{
		{"not an array (integer)", resp.NewInteger(5)},
		{"not an array (simple string)", resp.NewSimpleString("PING")},
		{"null array", resp.NewNullArray()},
		{"empty array", resp.NewArray([]resp.Value{})},
		{"nested array element", resp.NewArray([]resp.Value{
			resp.NewArray([]resp.Value{resp.NewBulkString([]byte("x"))}),
		})},
		{"integer element", resp.NewArray([]resp.Value{resp.NewInteger(1)})},
		{"null bulk string element", resp.NewArray([]resp.Value{resp.NewNullBulkString()})},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := commandFromValue(c.v); err == nil {
				t.Fatalf("got nil error, want an error for %+v", c.v)
			}
		})
	}
}
