package resp

import (
	"reflect"
	"testing"
)

func TestKindString(t *testing.T) {
	cases := []struct {
		k    Kind
		want string
	}{
		{SimpleString, "SimpleString"},
		{Error, "Error"},
		{Integer, "Integer"},
		{BulkString, "BulkString"},
		{Array, "Array"},
		{Kind(99), "Unknown"},
	}
	for _, c := range cases {
		if got := c.k.String(); got != c.want {
			t.Errorf("Kind(%d).String() = %q, want %q", c.k, got, c.want)
		}
	}
}

func TestNewSimpleString(t *testing.T) {
	v := NewSimpleString("OK")
	if v.Kind != SimpleString || v.Str != "OK" {
		t.Fatalf("NewSimpleString(\"OK\") = %+v, want Kind=SimpleString Str=OK", v)
	}
}

func TestNewError(t *testing.T) {
	v := NewError("ERR wrong number of arguments")
	if v.Kind != Error || v.Str != "ERR wrong number of arguments" {
		t.Fatalf("NewError(...) = %+v, want Kind=Error with matching Str", v)
	}
}

func TestNewInteger(t *testing.T) {
	cases := []int64{0, 1, -1, 9223372036854775807, -9223372036854775808}
	for _, n := range cases {
		v := NewInteger(n)
		if v.Kind != Integer || v.Int != n {
			t.Errorf("NewInteger(%d) = %+v, want Kind=Integer Int=%d", n, v, n)
		}
	}
}

func TestNewBulkString(t *testing.T) {
	cases := [][]byte{
		nil,
		{},
		[]byte("hello"),
		[]byte("with\r\nembedded\x00newline-and-nul"),
	}
	for _, b := range cases {
		v := NewBulkString(b)
		if v.Kind != BulkString {
			t.Fatalf("NewBulkString(%q).Kind = %v, want BulkString", b, v.Kind)
		}
		if v.Null {
			t.Fatalf("NewBulkString(%q).Null = true, want false (present, possibly empty)", b)
		}
		if !reflect.DeepEqual(v.Bulk, b) {
			t.Fatalf("NewBulkString(%q).Bulk = %q, want unchanged", b, v.Bulk)
		}
	}
}

func TestNewNullBulkStringDistinctFromEmpty(t *testing.T) {
	null := NewNullBulkString()
	empty := NewBulkString([]byte{})

	if !null.Null {
		t.Fatalf("NewNullBulkString().Null = false, want true")
	}
	if empty.Null {
		t.Fatalf("NewBulkString([]byte{}).Null = true, want false — empty bulk string is not null")
	}
	if null.Kind != BulkString || empty.Kind != BulkString {
		t.Fatalf("both null and empty bulk strings must have Kind=BulkString")
	}
}

func TestNewArray(t *testing.T) {
	elems := []Value{NewInteger(1), NewSimpleString("two")}
	v := NewArray(elems)
	if v.Kind != Array || v.Null {
		t.Fatalf("NewArray(...) = %+v, want Kind=Array Null=false", v)
	}
	if !reflect.DeepEqual(v.Elems, elems) {
		t.Fatalf("NewArray(...).Elems = %+v, want %+v", v.Elems, elems)
	}

	empty := NewArray(nil)
	if empty.Kind != Array || empty.Null {
		t.Fatalf("NewArray(nil) = %+v, want Kind=Array Null=false (present, empty)", empty)
	}
	if empty.Elems != nil {
		t.Fatalf("NewArray(nil).Elems = %+v, want nil", empty.Elems)
	}
}

func TestNewNullArrayDistinctFromEmpty(t *testing.T) {
	null := NewNullArray()
	empty := NewArray([]Value{})

	if !null.Null {
		t.Fatalf("NewNullArray().Null = false, want true")
	}
	if empty.Null {
		t.Fatalf("NewArray([]Value{}).Null = true, want false — empty array is not null")
	}
	if null.Kind != Array || empty.Kind != Array {
		t.Fatalf("both null and empty arrays must have Kind=Array")
	}
}

// Nested arrays are legal RESP (e.g. a reply containing sub-arrays); make
// sure the Value type can represent them with no special-casing.
func TestNewArrayNested(t *testing.T) {
	inner := NewArray([]Value{NewInteger(1), NewInteger(2)})
	outer := NewArray([]Value{inner, NewBulkString([]byte("x"))})

	if outer.Kind != Array || len(outer.Elems) != 2 {
		t.Fatalf("outer array malformed: %+v", outer)
	}
	if outer.Elems[0].Kind != Array || len(outer.Elems[0].Elems) != 2 {
		t.Fatalf("nested array malformed: %+v", outer.Elems[0])
	}
}
