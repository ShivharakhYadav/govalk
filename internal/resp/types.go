// Package resp implements the RESP2 wire protocol (Redis Serialization
// Protocol) used both for client/server request-reply traffic and, per
// PLAN.md §0 decision 1, for AOF persistence records.
package resp

// Kind identifies which of the five RESP2 types a Value holds.
type Kind byte

const (
	SimpleString Kind = iota
	Error
	Integer
	BulkString
	Array
)

func (k Kind) String() string {
	switch k {
	case SimpleString:
		return "SimpleString"
	case Error:
		return "Error"
	case Integer:
		return "Integer"
	case BulkString:
		return "BulkString"
	case Array:
		return "Array"
	default:
		return "Unknown"
	}
}

// Value is a single RESP2 protocol value. Only the field(s) matching Kind
// are meaningful:
//
//   - SimpleString, Error: Str
//   - Integer:             Int
//   - BulkString:          Bulk. A nil Bulk with Null=true is the RESP
//     Null Bulk String ("$-1\r\n"), distinct from a present-but-empty
//     bulk string ("$0\r\n\r\n", Bulk = []byte{}, Null=false).
//   - Array:                Elems. A nil Elems with Null=true is the RESP
//     Null Array ("*-1\r\n"), distinct from a present-but-empty array
//     ("*0\r\n", Elems = []Value{}, Null=false).
//
// Bulk is a byte slice, not a string, because RESP bulk strings are
// binary-safe: a value may contain arbitrary bytes including embedded
// "\r\n" or NUL, which a Go string would represent fine but which must
// never be mistaken for text to be scanned line-by-line (see reader.go).
type Value struct {
	Kind  Kind
	Str   string
	Int   int64
	Bulk  []byte
	Elems []Value
	Null  bool
}

// NewSimpleString returns a RESP Simple String value ("+OK\r\n" style).
// s must not contain '\r' or '\n' — simple strings are not binary-safe;
// use NewBulkString for arbitrary payloads.
func NewSimpleString(s string) Value {
	return Value{Kind: SimpleString, Str: s}
}

// NewError returns a RESP Error value ("-ERR ...\r\n" style). msg must
// not contain '\r' or '\n', same constraint as NewSimpleString.
func NewError(msg string) Value {
	return Value{Kind: Error, Str: msg}
}

// NewInteger returns a RESP Integer value.
func NewInteger(n int64) Value {
	return Value{Kind: Integer, Int: n}
}

// NewBulkString returns a RESP Bulk String value wrapping b directly (not
// copied) — b may be nil to represent a present, zero-length bulk string;
// use NewNullBulkString for the protocol's distinct "no value" case.
func NewBulkString(b []byte) Value {
	return Value{Kind: BulkString, Bulk: b}
}

// NewNullBulkString returns the RESP Null Bulk String ("$-1\r\n").
func NewNullBulkString() Value {
	return Value{Kind: BulkString, Null: true}
}

// NewArray returns a RESP Array value wrapping elems directly (not
// copied) — elems may be nil to represent a present, zero-length array;
// use NewNullArray for the protocol's distinct "no value" case.
func NewArray(elems []Value) Value {
	return Value{Kind: Array, Elems: elems}
}

// NewNullArray returns the RESP Null Array ("*-1\r\n").
func NewNullArray() Value {
	return Value{Kind: Array, Null: true}
}
