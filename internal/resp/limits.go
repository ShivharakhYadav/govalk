package resp

// Protocol-level size limits enforced by the reader (see reader.go, P2)
// before allocating memory for a declared length prefix. Without this, a
// client declaring e.g. "$999999999999\r\n" could force an allocation of
// that size before a single payload byte arrives — an easy memory-
// exhaustion DoS against a network-facing server. See PLAN.md §3
// "Security" / "Declared-length-before-allocation".
const (
	// MaxBulkLen is the largest allowed declared length of a single bulk
	// string payload, in bytes. 512 MiB matches Redis's own default
	// proto-max-bulk-len, a reasonable, familiar ceiling.
	MaxBulkLen = 512 * 1024 * 1024

	// MaxArrayLen is the largest allowed declared element count of a
	// single request array.
	MaxArrayLen = 1024 * 1024

	// MaxInlineLen is the largest allowed length of a single inline
	// command line — a plain-text command not using the RESP array
	// protocol (e.g. typed manually over a raw TCP connection).
	MaxInlineLen = 64 * 1024
)
