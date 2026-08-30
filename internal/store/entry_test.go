package store

import "testing"

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
