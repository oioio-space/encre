package auth_test

import (
	"testing"
	"time"

	"github.com/oioio-space/encre/server/auth"
)

func TestLimiterAllowsUpToBurstThenBites(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	clock := func() time.Time { return now }
	lim := auth.NewLimiter(5, clock) // ENCRE_04 §7: 5 logins/min/IP

	for i := range 5 {
		if !lim.Allow("1.2.3.4") {
			t.Fatalf("Allow() call %d = false, want true (within burst of 5)", i+1)
		}
	}
	if lim.Allow("1.2.3.4") {
		t.Error("Allow() call 6 = true, want false (limit is 5/min)")
	}
}

func TestLimiterRelaxesAfterTheWindow(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	clock := func() time.Time { return now }
	lim := auth.NewLimiter(5, clock)

	for range 5 {
		lim.Allow("1.2.3.4")
	}
	if lim.Allow("1.2.3.4") {
		t.Fatal("Allow() at the limit = true, want false")
	}

	now = now.Add(time.Minute)
	if !lim.Allow("1.2.3.4") {
		t.Error("Allow() a minute later = false, want true (window has passed)")
	}
}

func TestLimiterKeysAreIndependent(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	clock := func() time.Time { return now }
	lim := auth.NewLimiter(5, clock)

	for range 5 {
		lim.Allow("1.2.3.4")
	}
	if lim.Allow("1.2.3.4") {
		t.Fatal("Allow() at the limit for 1.2.3.4 = true, want false")
	}
	if !lim.Allow("5.6.7.8") {
		t.Error("Allow() for a different key = false, want true (limits are per-key)")
	}
}

func TestLimiterChildPatternThreshold(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	clock := func() time.Time { return now }
	lim := auth.NewLimiter(10, clock) // ENCRE_04 §7: 10 patterns/min/child

	for i := range 10 {
		if !lim.Allow("child1") {
			t.Fatalf("Allow() call %d = false, want true (within burst of 10)", i+1)
		}
	}
	if lim.Allow("child1") {
		t.Error("Allow() call 11 = true, want false (limit is 10/min)")
	}
}

// TestLimiterEvictsLeastRecentlyUsedWhenFull is the regression test for the
// unbounded map an independent audit found: a limiter keyed on an
// attacker-controlled value (see [auth.ClientIP]'s doc comment) with no cap
// grows one bucket per forged key forever. With a cap of 2: key1 is touched,
// then key2, then key1 again (making key2 the least recently used); a third
// distinct key, key3, must evict key2 — not key1, whose second Allow call
// makes it more recent — and key2's later reappearance must find a fresh
// bucket, not its old one.
func TestLimiterEvictsLeastRecentlyUsedWhenFull(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	clock := func() time.Time { return now }
	lim := auth.NewLimiterSize(5, 2, clock)

	lim.Allow("key1") // key1: 1 of 5 tokens spent
	lim.Allow("key2") // key2: 1 of 5 tokens spent
	lim.Allow("key1") // key1 touched again: key2 is now the least recently used

	lim.Allow("key3") // a third key: the map is full, key2 must be evicted

	// key1 survived eviction with 2 of its 5 tokens already spent: only 3
	// more calls should succeed.
	for i := range 3 {
		if !lim.Allow("key1") {
			t.Fatalf("Allow(key1) call %d = false, want true (3 tokens should remain)", i+1)
		}
	}
	if lim.Allow("key1") {
		t.Error("Allow(key1) after its remaining tokens = true, want false (key1 must not have been evicted)")
	}

	// key2 is gone: a fresh bucket has its full burst back, even though it
	// had already spent a token before eviction.
	for i := range 5 {
		if !lim.Allow("key2") {
			t.Fatalf("Allow(key2) call %d after eviction = false, want true (fresh bucket)", i+1)
		}
	}
}
