package auth

import (
	"bytes"
	"errors"
	"testing"
)

// testPepperInternal builds a fixed-key [Pepper] for tests in this
// (internal, package auth) file — the auth_test package's own testPepper
// helper is not visible here.
func testPepperInternal(t *testing.T) *Pepper {
	t.Helper()
	pep, err := NewPepper("test", bytes.Repeat([]byte("k"), pepperKeyBytes))
	if err != nil {
		t.Fatalf("NewPepper() error = %v", err)
	}
	return pep
}

// TestArgon2SemSaturationRejects fills argon2Sem to capacity by hand and
// checks that HashPassword fails fast with ErrTooManyPasswordChecks instead
// of blocking: the whole point of TryAcquire over Acquire is no unbounded
// queue in front of a memory-hard hash. It lives in the internal test file
// because argon2Sem is unexported — the property under test (a hard cap, not
// a queue) is not observable from outside the package.
func TestArgon2SemSaturationRejects(t *testing.T) {
	slots := 0
	for argon2Sem.TryAcquire(1) {
		slots++
	}
	t.Cleanup(func() { argon2Sem.Release(int64(slots)) })
	if slots == 0 {
		t.Fatal("argon2Sem had no capacity to acquire even once")
	}

	pep := testPepperInternal(t)
	if _, err := HashPassword("anything", pep); !errors.Is(err, ErrTooManyPasswordChecks) {
		t.Errorf("HashPassword() under a saturated argon2Sem: error = %v, want ErrTooManyPasswordChecks", err)
	}
	if _, err := VerifyPassword("anything", "test:$argon2id$v=19$m=65536,t=3,p=4$c2FsdA$aGFzaA", pep); !errors.Is(err, ErrTooManyPasswordChecks) {
		t.Errorf("VerifyPassword() under a saturated argon2Sem: error = %v, want ErrTooManyPasswordChecks", err)
	}

	// Release one slot and confirm normal operation resumes.
	argon2Sem.Release(1)
	slots--
	if _, err := HashPassword("anything", pep); err != nil {
		t.Errorf("HashPassword() after releasing a slot: error = %v, want nil", err)
	}
}
