package auth_test

import (
	"strings"
	"testing"

	"github.com/oioio-space/encre/server/auth"
	"pgregory.net/rapid"
)

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := auth.HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	ok, err := auth.VerifyPassword("correct horse battery staple", hash)
	if err != nil {
		t.Fatalf("VerifyPassword() error = %v", err)
	}
	if !ok {
		t.Error("VerifyPassword() with the right password = false, want true")
	}

	ok, err = auth.VerifyPassword("wrong password", hash)
	if err != nil {
		t.Fatalf("VerifyPassword() error = %v", err)
	}
	if ok {
		t.Error("VerifyPassword() with the wrong password = true, want false")
	}
}

func TestHashPasswordSaltsEveryCall(t *testing.T) {
	first, err := auth.HashPassword("same password")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	second, err := auth.HashPassword("same password")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if first == second {
		t.Error("HashPassword() of the same password twice produced identical hashes, want distinct salts")
	}
}

func TestHashPasswordUsesENCRE04Params(t *testing.T) {
	hash, err := auth.HashPassword("p")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	// ENCRE_04 §7: t=3, m=64 MiB. The PHC string encodes both.
	if !strings.Contains(hash, "m=65536,t=3,") {
		t.Errorf("hash = %q, want it to encode m=65536,t=3", hash)
	}
}

func TestHashAndVerifyPatternSaltsEveryCall(t *testing.T) {
	first, err := auth.HashPattern("0123")
	if err != nil {
		t.Fatalf("HashPattern() error = %v", err)
	}
	second, err := auth.HashPattern("0123")
	if err != nil {
		t.Fatalf("HashPattern() error = %v", err)
	}
	if first == second {
		t.Error("HashPattern() of the same pattern twice produced identical hashes, want distinct salts")
	}

	ok, err := auth.VerifyPattern("0123", first)
	if err != nil {
		t.Fatalf("VerifyPattern() error = %v", err)
	}
	if !ok {
		t.Error("VerifyPattern() with the right pattern = false, want true")
	}

	ok, err = auth.VerifyPattern("9999", first)
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
	rapid.Check(t, func(t *rapid.T) {
		a := rapid.StringN(1, 40, -1).Draw(t, "a")
		b := rapid.StringN(1, 40, -1).Draw(t, "b")
		if a == b {
			t.Skip("rapid drew equal passwords")
		}

		hash, err := auth.HashPassword(a)
		if err != nil {
			t.Fatalf("HashPassword() error = %v", err)
		}

		ok, err := auth.VerifyPassword(a, hash)
		if err != nil {
			t.Fatalf("VerifyPassword(a) error = %v", err)
		}
		if !ok {
			t.Fatalf("VerifyPassword(%q) against its own hash = false, want true", a)
		}

		ok, err = auth.VerifyPassword(b, hash)
		if err != nil {
			t.Fatalf("VerifyPassword(b) error = %v", err)
		}
		if ok {
			t.Fatalf("VerifyPassword(%q) against %q's hash = true, want false", b, a)
		}
	})
}
