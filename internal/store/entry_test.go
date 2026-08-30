package store

import (
	"math"
	"testing"
	"time"
)

func TestEntry_Expired(t *testing.T) {
	cases := []struct {
		name      string
		expiresAt int64
		now       int64
		want      bool
	}{
		{"no ttl", 0, 1000, false},
		{"future deadline", 2000, 1000, false},
		{"exact deadline", 1000, 1000, true},
		{"past deadline", 500, 1000, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := &entry{expiresAt: c.expiresAt}
			if got := e.expired(c.now); got != c.want {
				t.Errorf("expired(%d) with expiresAt=%d = %v, want %v", c.now, c.expiresAt, got, c.want)
			}
		})
	}
}

func TestEntry_WithTTL(t *testing.T) {
	e := &entry{kind: kindString, str: []byte("v"), expiresAt: 100}
	e2 := e.withTTL(200)

	if e2.kind != e.kind || string(e2.str) != string(e.str) {
		t.Fatalf("withTTL changed kind/str: got %+v, want same kind/str as %+v", e2, e)
	}
	if e2.expiresAt != 200 {
		t.Fatalf("withTTL(200).expiresAt = %d, want 200", e2.expiresAt)
	}
	if e.expiresAt != 100 {
		t.Fatalf("withTTL must not mutate the original entry: e.expiresAt = %d, want unchanged 100", e.expiresAt)
	}
}

func TestSafeExpireDeadline_NonPositiveSecondsIsImmediate(t *testing.T) {
	now := time.Unix(1000, 0)
	for _, seconds := range []int64{0, -1, -1000} {
		deadline, immediate, err := SafeExpireDeadline(now, seconds)
		if err != nil {
			t.Fatalf("seconds=%d: unexpected error %v", seconds, err)
		}
		if !immediate {
			t.Fatalf("seconds=%d: immediate = false, want true", seconds)
		}
		if deadline != 0 {
			t.Fatalf("seconds=%d: deadline = %d, want 0", seconds, deadline)
		}
	}
}

func TestSafeExpireDeadline_PositiveSecondsComputesDeadline(t *testing.T) {
	now := time.Unix(1000, 0)
	deadline, immediate, err := SafeExpireDeadline(now, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if immediate {
		t.Fatalf("immediate = true, want false")
	}
	want := now.Add(10 * time.Second).UnixNano()
	if deadline != want {
		t.Fatalf("deadline = %d, want %d", deadline, want)
	}
}

// The plan's explicit P10 verification bar: math.MaxInt64 seconds must
// be rejected outright, not silently wrapped around into a bogus
// (possibly already-past) deadline.
func TestSafeExpireDeadline_OverflowRejected(t *testing.T) {
	now := time.Now()
	cases := []int64{
		math.MaxInt64,
		math.MaxInt64 / 2,
		math.MaxInt64/int64(time.Second) + 1, // just past the multiplication boundary
	}
	for _, seconds := range cases {
		deadline, immediate, err := SafeExpireDeadline(now, seconds)
		if err == nil {
			t.Fatalf("seconds=%d: err = nil, want ErrExpireOutOfRange (got deadline=%d immediate=%v)",
				seconds, deadline, immediate)
		}
	}
}

func TestSafeExpireDeadline_LargeButValidSecondsAccepted(t *testing.T) {
	now := time.Unix(0, 0)
	const hundredYearsInSeconds = 100 * 365 * 24 * 3600
	deadline, immediate, err := SafeExpireDeadline(now, hundredYearsInSeconds)
	if err != nil || immediate {
		t.Fatalf("100 years: err=%v immediate=%v, want a valid (non-immediate, no error) deadline", err, immediate)
	}
	if deadline <= 0 {
		t.Fatalf("deadline = %d, want a large positive value", deadline)
	}
}
