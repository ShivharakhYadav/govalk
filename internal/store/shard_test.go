package store

import "testing"

func TestShard_LookupLive_MissingKey(t *testing.T) {
	sh := newShard()
	if e := sh.lookupLive("nope", 1000); e != nil {
		t.Fatalf("lookupLive on missing key = %+v, want nil", e)
	}
}

func TestShard_LookupLive_NotExpired(t *testing.T) {
	sh := newShard()
	want := &entry{kind: kindString, str: []byte("v")}
	sh.data["k"] = want

	got := sh.lookupLive("k", 1000) // no expiresAt set - never expires
	if got != want {
		t.Fatalf("lookupLive = %+v, want the stored entry unchanged", got)
	}
	if _, ok := sh.data["k"]; !ok {
		t.Fatalf("non-expired entry must not be removed")
	}
}

func TestShard_LookupLive_ExpiredIsReaped(t *testing.T) {
	sh := newShard()
	sh.data["k"] = &entry{kind: kindString, str: []byte("v"), expiresAt: 500}

	got := sh.lookupLive("k", 1000) // now > expiresAt
	if got != nil {
		t.Fatalf("lookupLive on expired entry = %+v, want nil", got)
	}
	if _, ok := sh.data["k"]; ok {
		t.Fatalf("expired entry must be reaped (deleted) from the shard's map")
	}
}

// Exercises the double-checked-locking upgrade path (the one genuinely
// risky piece of code in lookupLive) under `go test -race`: one
// goroutine repeatedly reads via lookupLive while another concurrently
// overwrites the same key, alternately with already-expired and live
// entries, so lookupLive actually takes the write-lock reap path while
// racing against real concurrent mutation - not just the uncontended
// happy path.
func TestShard_LookupLive_ConcurrentExpiryAndWrites(t *testing.T) {
	sh := newShard()
	const iterations = 2000
	const now = 1000

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < iterations; i++ {
			var exp int64
			if i%2 == 0 {
				exp = 1 // already expired relative to `now` below
			}
			sh.mu.Lock()
			sh.data["k"] = &entry{kind: kindString, str: []byte("v"), expiresAt: exp}
			sh.mu.Unlock()
		}
	}()

	for i := 0; i < iterations; i++ {
		sh.lookupLive("k", now)
	}
	<-done
}
