package resp

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
)

// ErrProtocol wraps every RESP structural violation the Reader detects:
// an unrecognized type prefix in a strict (nested) position, a
// non-numeric or out-of-range length header, a declared length beyond
// the configured limits, or a header line missing its required CRLF
// terminator. There is no way to safely resynchronize a corrupted RESP
// stream, so callers should treat ErrProtocol as fatal for the
// connection: close it.
var ErrProtocol = errors.New("resp: protocol error")

func protoErr(format string, a ...any) error {
	return fmt.Errorf("resp: %s: %w", fmt.Sprintf(format, a...), ErrProtocol)
}

// Reader decodes RESP2 values from a byte stream. It is used for parsing
// client requests, server/test-client replies, and (per PLAN.md §0
// decision 1) AOF records, which are RESP arrays reusing this same
// decoder.
//
// Error contract: ReadValue returns exactly io.EOF only when the stream
// ends cleanly at a value boundary - no byte of a new value has been
// read yet (e.g. end of an AOF file, or a client disconnecting between
// commands). Any other end-of-stream, encountered after a value has
// started, is reported as io.ErrUnexpectedEOF, so callers (see P14
// replay) can distinguish "nothing more to read" from "the last record
// is truncated." Any structural violation is reported as an error
// wrapping ErrProtocol.
type Reader struct {
	br *bufio.Reader
}

// NewReader returns a Reader wrapping r. The internal buffer is sized
// above MaxInlineLen so a single header/inline line essentially never
// forces the multi-chunk bufio.ErrBufferFull path in ordinary operation.
func NewReader(r io.Reader) *Reader {
	return &Reader{br: bufio.NewReaderSize(r, MaxInlineLen+4096)}
}

// ReadValue reads and returns exactly one RESP value: either a typed
// value (+, -, :, $, *) or, if the stream doesn't begin with a
// recognized type prefix, an inline command - returned as an Array of
// BulkStrings, identical in shape to a real multi-bulk request, so
// callers never need to special-case inline input.
func (r *Reader) ReadValue() (Value, error) {
	b, err := r.br.Peek(1)
	if err != nil {
		if err == io.EOF {
			return Value{}, io.EOF
		}
		return Value{}, err
	}
	switch b[0] {
	case '+', '-', ':', '$', '*':
		return r.readTyped()
	default:
		return r.readInline()
	}
}

// readTyped parses exactly one typed RESP value. It never falls back to
// inline parsing: an unrecognized prefix here (e.g. inside an array
// element) is always a protocol error, since inline commands are only
// valid as an entire top-level request, never nested.
func (r *Reader) readTyped() (Value, error) {
	prefixByte, err := r.br.ReadByte()
	if err != nil {
		return Value{}, unexpectedEOF(err)
	}
	switch prefixByte {
	case '+':
		line, err := r.readLine()
		if err != nil {
			return Value{}, err
		}
		return NewSimpleString(string(line)), nil
	case '-':
		line, err := r.readLine()
		if err != nil {
			return Value{}, err
		}
		return NewError(string(line)), nil
	case ':':
		line, err := r.readLine()
		if err != nil {
			return Value{}, err
		}
		n, perr := strconv.ParseInt(string(line), 10, 64)
		if perr != nil {
			return Value{}, protoErr("invalid integer %q", line)
		}
		return NewInteger(n), nil
	case '$':
		return r.readBulkString()
	case '*':
		return r.readArray()
	default:
		return Value{}, protoErr("unrecognized type prefix %q", prefixByte)
	}
}

func (r *Reader) readBulkString() (Value, error) {
	line, err := r.readLine()
	if err != nil {
		return Value{}, err
	}
	n, perr := strconv.ParseInt(string(line), 10, 64)
	if perr != nil {
		return Value{}, protoErr("invalid bulk length %q", line)
	}
	if n == -1 {
		return NewNullBulkString(), nil
	}
	if n < 0 {
		return Value{}, protoErr("invalid negative bulk length %d", n)
	}
	if n > MaxBulkLen {
		return Value{}, protoErr("bulk length %d exceeds MaxBulkLen %d", n, MaxBulkLen)
	}

	buf := make([]byte, n)
	if _, err := io.ReadFull(r.br, buf); err != nil {
		return Value{}, unexpectedEOF(err)
	}
	var crlf [2]byte
	if _, err := io.ReadFull(r.br, crlf[:]); err != nil {
		return Value{}, unexpectedEOF(err)
	}
	if crlf != [2]byte{'\r', '\n'} {
		return Value{}, protoErr("bulk string missing terminating CRLF")
	}
	return NewBulkString(buf), nil
}

func (r *Reader) readArray() (Value, error) {
	line, err := r.readLine()
	if err != nil {
		return Value{}, err
	}
	n, perr := strconv.ParseInt(string(line), 10, 64)
	if perr != nil {
		return Value{}, protoErr("invalid array length %q", line)
	}
	if n == -1 {
		return NewNullArray(), nil
	}
	if n < 0 {
		return Value{}, protoErr("invalid negative array length %d", n)
	}
	if n > MaxArrayLen {
		return Value{}, protoErr("array length %d exceeds MaxArrayLen %d", n, MaxArrayLen)
	}

	elems := make([]Value, 0, n)
	for i := int64(0); i < n; i++ {
		v, err := r.readTyped()
		if err != nil {
			return Value{}, err
		}
		elems = append(elems, v)
	}
	return NewArray(elems), nil
}

// readInline parses a plain-text inline command line: terminated by '\n'
// with an optional preceding '\r' stripped (looser than the strict CRLF
// required for typed headers - this matches RESP's own inline-command
// exception, meant for telnet-style manual use), whitespace-split into
// arguments, and returned as an Array of BulkStrings so callers treat it
// identically to a real multi-bulk request.
//
// Quoted arguments (e.g. SET k "a b") are not supported: out of scope for
// this project. An inline command needing a literal space in an argument
// should use the real RESP array protocol instead.
func (r *Reader) readInline() (Value, error) {
	raw, err := r.readRawLine(MaxInlineLen)
	if err != nil {
		return Value{}, err
	}
	raw = bytes.TrimSuffix(raw, []byte{'\r'})
	fields := bytes.Fields(raw)
	elems := make([]Value, 0, len(fields))
	for _, f := range fields {
		elems = append(elems, NewBulkString(f))
	}
	return NewArray(elems), nil
}

// readLine reads a header line up to and including a mandatory CRLF
// terminator, bounded by MaxInlineLen, returning the content without the
// terminator. Used for every typed-value header line (+, -, :, $, *
// content/length lines), all of which require strict CRLF per the RESP
// spec - unlike readInline's looser bare-\n allowance.
func (r *Reader) readLine() ([]byte, error) {
	raw, err := r.readRawLine(MaxInlineLen)
	if err != nil {
		return nil, err
	}
	if len(raw) < 1 || raw[len(raw)-1] != '\r' {
		return nil, protoErr("header line missing CRLF terminator")
	}
	return raw[:len(raw)-1], nil
}

// readRawLine reads until '\n' (exclusive), bounded by maxLen total
// bytes. Any error is reported via unexpectedEOF: readRawLine is only
// ever called after ReadValue's initial Peek has already observed at
// least one byte of the value being read, so an EOF from here on always
// means the stream ended mid-value, never at a clean boundary.
func (r *Reader) readRawLine(maxLen int) ([]byte, error) {
	var line []byte
	for {
		chunk, err := r.br.ReadSlice('\n')
		line = append(line, chunk...)
		if err == nil {
			break
		}
		if err != bufio.ErrBufferFull {
			return nil, unexpectedEOF(err)
		}
		if len(line) > maxLen {
			return nil, protoErr("line exceeds maximum length %d", maxLen)
		}
	}
	if len(line) > maxLen {
		return nil, protoErr("line exceeds maximum length %d", maxLen)
	}
	return line[:len(line)-1], nil // strip trailing '\n'
}

// unexpectedEOF translates a plain io.EOF into io.ErrUnexpectedEOF. Every
// call site in this file runs strictly after ReadValue's initial Peek has
// already observed the first byte of the current value, so an EOF at
// that point always means the stream ended mid-value.
func unexpectedEOF(err error) error {
	if err == io.EOF {
		return io.ErrUnexpectedEOF
	}
	return err
}
