package auth_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/oioio-space/encre/server/auth"
	"pgregory.net/rapid"
)

func TestHashAndVerifyPassword(t *testing.T) {
	pep := testPepper(t)
	hash, err := auth.HashPassword("correct horse battery staple", pep)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	ok, err := auth.VerifyPassword("correct horse battery staple", hash, pep)
	if err != nil {
		t.Fatalf("VerifyPassword() error = %v", err)
	}
	if !ok {
		t.Error("VerifyPassword() with the right password = false, want true")
	}

	ok, err = auth.VerifyPassword("wrong password", hash, pep)
	if err != nil {
		t.Fatalf("VerifyPassword() error = %v", err)
	}
	if ok {
		t.Error("VerifyPassword() with the wrong password = true, want false")
	}
}

func TestHashPasswordSaltsEveryCall(t *testing.T) {
	pep := testPepper(t)
	first, err := auth.HashPassword("same password", pep)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	second, err := auth.HashPassword("same password", pep)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if first == second {
		t.Error("HashPassword() of the same password twice produced identical hashes, want distinct salts")
	}
}

func TestHashPasswordUsesENCRE04Params(t *testing.T) {
	pep := testPepper(t)
	hash, err := auth.HashPassword("p", pep)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	// ENCRE_04 §7: t=3, m=64 MiB. The PHC string encodes both.
	if !strings.Contains(hash, "m=65536,t=3,") {
		t.Errorf("hash = %q, want it to encode m=65536,t=3", hash)
	}
}

func TestHashAndVerifyPatternSaltsEveryCall(t *testing.T) {
	pep := testPepper(t)
	first, err := auth.HashPattern("01234", pep)
	if err != nil {
		t.Fatalf("HashPattern() error = %v", err)
	}
	second, err := auth.HashPattern("01234", pep)
	if err != nil {
		t.Fatalf("HashPattern() error = %v", err)
	}
	if first == second {
		t.Error("HashPattern() of the same pattern twice produced identical hashes, want distinct salts")
	}

	ok, err := auth.VerifyPattern("01234", first, pep)
	if err != nil {
		t.Fatalf("VerifyPattern() error = %v", err)
	}
	if !ok {
		t.Error("VerifyPattern() with the right pattern = false, want true")
	}

	ok, err = auth.VerifyPattern("99999", first, pep)
	if err != nil {
		t.Fatalf("VerifyPattern() error = %v", err)
	}
	if ok {
		t.Error("VerifyPattern() with the wrong pattern = true, want false")
	}
}

// TestPasswordVerifiesOnlyItsOwnHash is the property ENCRE-buo asks for: any
// password verifies against its own hash and against no other password's
// hash. Passing this on a fixed input is easy; rapid tries thousands of
// random passwords to catch a hash function that, say, truncates and starts
// colliding.
func TestPasswordVerifiesOnlyItsOwnHash(t *testing.T) {
	pep := testPepper(t)
	rapid.Check(t, func(t *rapid.T) {
		a := rapid.StringN(1, 40, -1).Draw(t, "a")
		b := rapid.StringN(1, 40, -1).Draw(t, "b")
		if a == b {
			t.Skip("rapid drew equal passwords")
		}

		hash, err := auth.HashPassword(a, pep)
		if err != nil {
			t.Fatalf("HashPassword() error = %v", err)
		}

		ok, err := auth.VerifyPassword(a, hash, pep)
		if err != nil {
			t.Fatalf("VerifyPassword(a) error = %v", err)
		}
		if !ok {
			t.Fatalf("VerifyPassword(%q) against its own hash = false, want true", a)
		}

		ok, err = auth.VerifyPassword(b, hash, pep)
		if err != nil {
			t.Fatalf("VerifyPassword(b) error = %v", err)
		}
		if ok {
			t.Fatalf("VerifyPassword(%q) against %q's hash = true, want false", b, a)
		}
	})
}

// TestVerifyPasswordRequiresPepper is the fail-closed check encre-qpx.4
// asks for: a nil Pepper must never silently fall back to comparing an
// unpeppered hash.
func TestVerifyPasswordRequiresPepper(t *testing.T) {
	pep := testPepper(t)
	hash, err := auth.HashPassword("s3cret", pep)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if _, err := auth.VerifyPassword("s3cret", hash, nil); !errors.Is(err, auth.ErrPepperRequired) {
		t.Errorf("VerifyPassword() with a nil Pepper: error = %v, want ErrPepperRequired", err)
	}
	if _, err := auth.HashPassword("s3cret", nil); !errors.Is(err, auth.ErrPepperRequired) {
		t.Errorf("HashPassword() with a nil Pepper: error = %v, want ErrPepperRequired", err)
	}
}

// TestOfflineAttackWithoutPepperKeyFails is encre-qpx.4's acceptance
// criterion made literal: given every byte SQLite would ever persist for a
// child's pattern (the peppered hash) but not the pepper key — exactly what
// a leaked Litestream replica of the .db file alone hands an attacker,
// ENCRE_04 §12 — an offline guesser who tries the real pattern (proving the
// attack would otherwise have worked) cannot verify it: every [auth.Pepper]
// built with a different key rejects it, because [auth.Pepper.VerifyHash]
// re-derives the peppered digest from the candidate and the key it was
// built with, never from anything stored alongside the hash.
func TestOfflineAttackWithoutPepperKeyFails(t *testing.T) {
	realPepper := testPepper(t)
	const pattern = "137913"

	stolenHash, err := auth.HashPattern(pattern, realPepper)
	if err != nil {
		t.Fatalf("HashPattern() error = %v", err)
	}

	attackerPepper, err := auth.NewPepper("attacker-guess", bytes.Repeat([]byte("0"), 32))
	if err != nil {
		t.Fatalf("auth.NewPepper() error = %v", err)
	}

	// The attacker's guessed key has a version the stolen hash does not
	// carry: this is the shape a real attack takes (the hash names "test",
	// their forged Pepper knows nothing about "test" at all), and it must
	// fail exactly like any other verification failure — never a panic,
	// never treated as a match.
	ok, err := attackerPepper.VerifyHash(pattern, stolenHash)
	if err == nil && ok {
		t.Fatal("VerifyHash() with a forged pepper key matched the real pattern, want it to fail")
	}
	if err == nil {
		t.Fatal("VerifyHash() with an unknown key version returned no error and ok=false; want an explicit error (ErrUnknownPepperVersion)")
	}

	// Even a nil pepper — no key material presented at all — must not be
	// treated as "skip peppering."
	if _, err := auth.VerifyPattern(pattern, stolenHash, nil); !errors.Is(err, auth.ErrPepperRequired) {
		t.Errorf("VerifyPattern() with a nil Pepper: error = %v, want ErrPepperRequired", err)
	}
}
