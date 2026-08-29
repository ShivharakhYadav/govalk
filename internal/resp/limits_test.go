package resp

import "testing"

// Guards against a future accidental edit turning one of these into zero
// or negative, which would make the reader (P2) reject every request.
func TestLimitsArePositive(t *testing.T) {
	for name, v := range map[string]int{
		"MaxBulkLen":   MaxBulkLen,
		"MaxArrayLen":  MaxArrayLen,
		"MaxInlineLen": MaxInlineLen,
	} {
		if v <= 0 {
			t.Errorf("%s = %d, want a positive limit", name, v)
		}
	}
}
