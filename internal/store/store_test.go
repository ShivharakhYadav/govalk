package store

import (
	"sync"
	"testing"
	"time"
)

func TestStore_SetGetRoundTrip(t *testing.T) {
	s := New()
	s.Set("k", []byte("v"), 0)

	got, ok, err := s.Get("k")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok || string(got) != "v" {
		t.Fatalf("Get = (%q, %v), want (\"v\", true)", got, ok)
	}
}

func TestStore_GetMissingKey(t *testing.T) {
	s := New()
	got, ok, err := s.Get("nope")
	if err != nil || ok || got != nil {
		t.Fatalf("Get on missing key = (%q, %v, %v), want (nil, false, nil)", got, ok, err)
	}
}

func TestStore_SetOverwritesPreviousValue(t *testing.T) {
	s := New()
	s.Set("k", []byte("v1"), 0)
	s.Set("k", []byte("v2"), 0)

	got, ok, _ := s.Get("k")
	if !ok || string(got) != "v2" {
		t.Fatalf("Get = (%q, %v), want (\"v2\", true)", got, ok)
	}
}

func TestStore_Delete(t *testing.T) {
	s := New()
	s.Set("k", []byte("v"), 0)

	if !s.Delete("k") {
		t.Fatalf("Delete on existing key = false, want true")
	}
	if s.Delete("k") {
		t.Fatalf("Delete on already-deleted key = true, want false")
	}
	if _, ok, _ := s.Get("k"); ok {
		t.Fatalf("Get after Delete should not find the key")
	}
}

func TestStore_Exists(t *testing.T) {
	s := New()
	if s.Exists("k") {
		t.Fatalf("Exists on missing key = true, want false")
	}
	s.Set("k", []byte("v"), 0)
	if !s.Exists("k") {
		t.Fatalf("Exists on set key = false, want true")
	}
	s.Delete("k")
	if s.Exists("k") {
		t.Fatalf("Exists after Delete = true, want false")
	}
}

func TestStore_TTL_ExpiredKeyIsHiddenAndReaped(t *testing.T) {
	s := New()
	past := time.Now().Add(-time.Hour).UnixNano()
	s.Set("k", []byte("v"), past)

	if _, ok, _ := s.Get("k"); ok {
		t.Fatalf("Get on an already-expired key found it, want not found")
	}
	if s.Exists("k") {
		t.Fatalf("Exists on an already-expired key = true, want false")
	}

	sh := s.shardFor("k")
	sh.mu.RLock()
	_, stillThere := sh.data["k"]
	sh.mu.RUnlock()
	if stillThere {
		t.Fatalf("expired key was not reaped from the shard's map")
	}
}

func TestStore_TTL_FutureDeadlineStillLive(t *testing.T) {
	s := New()
	future := time.Now().Add(time.Hour).UnixNano()
	s.Set("k", []byte("v"), future)

	got, ok, _ := s.Get("k")
	if !ok || string(got) != "v" {
		t.Fatalf("Get on a not-yet-expired key = (%q, %v), want (\"v\", true)", got, ok)
	}
}

func TestStore_Get_WrongType(t *testing.T) {
	s := New()
	// P8 has no public API to create a non-string entry yet (list
	// commands are P16) - construct one directly to prove Get's type
	// check is genuinely kind-generic, not a string-only special case
	// that would need reworking once lists exist.
	sh := s.shardFor("k")
	sh.mu.Lock()
	sh.data["k"] = &entry{kind: kindList}
	sh.mu.Unlock()

	_, _, err := s.Get("k")
	if err != ErrWrongType {
		t.Fatalf("Get on a list-kind key: err = %v, want ErrWrongType", err)
	}
}

func TestStore_Exists_IgnoresType(t *testing.T) {
	s := New()
	sh := s.shardFor("k")
	sh.mu.Lock()
	sh.data["k"] = &entry{kind: kindList}
	sh.mu.Unlock()

	if !s.Exists("k") {
		t.Fatalf("Exists should be true regardless of value type")
	}
}

func TestStore_ShardForIsDeterministic(t *testing.T) {
	s := New()
	a := s.shardFor("some-key")
	b := s.shardFor("some-key")
	if a != b {
		t.Fatalf("shardFor(\"some-key\") returned different shards across calls")
	}
}

// TestStore_ConcurrentAccess hammers overlapping keys from many
// goroutines simultaneously - the test `go test -race` is specifically
// meant to validate (P8's verification bar). It doesn't assert much
// about final values: concurrent writers racing for the same key make
// the winner nondeterministic by design (last-writer-wins, same as
// Redis's single-threaded model). It does assert that nothing panics or
// deadlocks, and that any value observed was genuinely written by some
// goroutine, never a torn/corrupted read.
func TestStore_ConcurrentAccess(t *testing.T) {
	s := New()
	const goroutines = 50
	const opsPerGoroutine = 200

	valid := map[string]bool{"v0": true, "v1": true, "v2": true, "v3": true}

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				key := "key" + string(rune('A'+(i+g)%8)) // 8 overlapping keys
				switch i % 4 {
				case 0:
					s.Set(key, []byte("v"+string(rune('0'+(g%4)))), 0)
				case 1:
					if v, ok, err := s.Get(key); ok && err == nil && !valid[string(v)] {
						t.Errorf("Get returned an impossible/torn value %q", v)
					}
				case 2:
					s.Exists(key)
				case 3:
					s.Delete(key)
				}
			}
		}(g)
	}
	wg.Wait()
}
