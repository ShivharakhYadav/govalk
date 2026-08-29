package resp

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
)

// Writer encodes RESP2 values to a byte stream. It is used for server
// replies, the hand-rolled test client's requests (P22), and (per
// PLAN.md §0 decision 1) AOF records, which are RESP arrays reusing this
// same encoder — WriteValue is the single encoding path for all three.
type Writer struct {
	bw *bufio.Writer
}

// NewWriter returns a Writer wrapping w. Writes are buffered; call Flush
// to ensure they reach the underlying stream.
func NewWriter(w io.Writer) *Writer {
	return &Writer{bw: bufio.NewWriter(w)}
}

// Flush writes any buffered data to the underlying io.Writer.
func (w *Writer) Flush() error {
	return w.bw.Flush()
}

// WriteValue encodes v in RESP2 wire format. It is the single source of
// truth for encoding; every convenience method below (WriteSimpleString,
// WriteBulkString, ...) is a thin wrapper that builds a Value and calls
// this.
func (w *Writer) WriteValue(v Value) error {
	switch v.Kind {
	case SimpleString:
		return w.writeLine('+', v.Str)
	case Error:
		return w.writeLine('-', v.Str)
	case Integer:
		return w.writeLine(':', strconv.FormatInt(v.Int, 10))
	case BulkString:
		return w.writeBulkString(v)
	case Array:
		return w.writeArray(v)
	default:
		return fmt.Errorf("resp: cannot encode unknown Kind %d", v.Kind)
	}
}

func (w *Writer) writeLine(prefix byte, content string) error {
	if err := w.bw.WriteByte(prefix); err != nil {
		return err
	}
	if _, err := w.bw.WriteString(content); err != nil {
		return err
	}
	_, err := w.bw.WriteString("\r\n")
	return err
}

func (w *Writer) writeBulkString(v Value) error {
	if v.Null {
		_, err := w.bw.WriteString("$-1\r\n")
		return err
	}
	if err := w.bw.WriteByte('$'); err != nil {
		return err
	}
	if _, err := w.bw.WriteString(strconv.Itoa(len(v.Bulk))); err != nil {
		return err
	}
	if _, err := w.bw.WriteString("\r\n"); err != nil {
		return err
	}
	if _, err := w.bw.Write(v.Bulk); err != nil {
		return err
	}
	_, err := w.bw.WriteString("\r\n")
	return err
}

func (w *Writer) writeArray(v Value) error {
	if v.Null {
		_, err := w.bw.WriteString("*-1\r\n")
		return err
	}
	if err := w.bw.WriteByte('*'); err != nil {
		return err
	}
	if _, err := w.bw.WriteString(strconv.Itoa(len(v.Elems))); err != nil {
		return err
	}
	if _, err := w.bw.WriteString("\r\n"); err != nil {
		return err
	}
	for _, elem := range v.Elems {
		if err := w.WriteValue(elem); err != nil {
			return err
		}
	}
	return nil
}

// The methods below are thin conveniences over WriteValue, saving every
// call site in command dispatch (P6+) from spelling out the
// New*/WriteValue pair for the common cases.

func (w *Writer) WriteSimpleString(s string) error { return w.WriteValue(NewSimpleString(s)) }
func (w *Writer) WriteError(msg string) error      { return w.WriteValue(NewError(msg)) }
func (w *Writer) WriteInteger(n int64) error       { return w.WriteValue(NewInteger(n)) }
func (w *Writer) WriteBulkString(b []byte) error   { return w.WriteValue(NewBulkString(b)) }
func (w *Writer) WriteNullBulkString() error       { return w.WriteValue(NewNullBulkString()) }
func (w *Writer) WriteArray(elems []Value) error   { return w.WriteValue(NewArray(elems)) }
func (w *Writer) WriteNullArray() error            { return w.WriteValue(NewNullArray()) }
