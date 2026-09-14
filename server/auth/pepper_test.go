package auth_test

import (
	"bytes"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/oioio-space/encre/server/auth"
)

func TestPepperEncryptDecryptRoundTrip(t *testing.T) {
	pep := testPepper(t)
	plaintext := []byte("a totp secret, base32 encoded")

	sealed, err := pep.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if bytes.Contains(sealed, plaintext) {
		t.Error("Encrypt() output contains the plaintext")
	}

	got, err := pep.Decrypt(sealed)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Errorf("Decrypt() = %q, want %q", got, plaintext)
	}
}

func TestPepperEncryptIsNondeterministic(t *testing.T) {
	pep := testPepper(t)
	a, err := pep.Encrypt([]byte("same secret"))
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	b, err := pep.Encrypt([]byte("same secret"))
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if bytes.Equal(a, b) {
		t.Error("Encrypt() of the same plaintext twice produced identical ciphertext, want a fresh nonce each time")
	}
}

// TestPepperDecryptWrongKeyFails is a smaller-scoped sibling of
// TestOfflineAttackWithoutPepperKeyFails, isolating the AES-GCM path alone.
func TestPepperDecryptWrongKeyFails(t *testing.T) {
	realPep := testPepper(t)
	sealed, err := realPep.Encrypt([]byte("totp secret"))
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	wrong, err := auth.NewPepper("test", bytes.Repeat([]byte("z"), 32))
	if err != nil {
		t.Fatalf("NewPepper() error = %v", err)
	}
	if _, err := wrong.Decrypt(sealed); err == nil {
		t.Error("Decrypt() with the wrong key succeeded, want an error")
	}
}

// TestPepperRotationVerifiesOldAndNewKeys checks the rotation window
// LoadPepper's doc comment describes: a Pepper built with a new current key
// and the old one retired still verifies a hash written under the old key,
// while a Pepper that never knew the old key (rotation completed, entry
// dropped) does not.
func TestPepperRotationVerifiesOldAndNewKeys(t *testing.T) {
	oldKey := bytes.Repeat([]byte("o"), 32)
	newKey := bytes.Repeat([]byte("n"), 32)

	oldPep, err := auth.NewPepper("v1", oldKey)
	if err != nil {
		t.Fatalf("NewPepper(old) error = %v", err)
	}
	hash, err := auth.HashPassword("s3cret", oldPep)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	type retired = struct {
		Version string
		Key     []byte
	}
	rotating, err := auth.NewPepper("v2", newKey, retired{Version: "v1", Key: oldKey})
	if err != nil {
		t.Fatalf("NewPepper(rotating) error = %v", err)
	}
	ok, err := auth.VerifyPassword("s3cret", hash, rotating)
	if err != nil {
		t.Fatalf("VerifyPassword() during rotation window: error = %v", err)
	}
	if !ok {
		t.Error("VerifyPassword() during rotation window (old key retired, not dropped) = false, want true")
	}

	// A hash rehashed under the new key stops needing the old one.
	rehash, err := auth.HashPassword("s3cret", rotating)
	if err != nil {
		t.Fatalf("HashPassword(rotating) error = %v", err)
	}
	afterRotation, err := auth.NewPepper("v2", newKey) // v1 fully dropped
	if err != nil {
		t.Fatalf("NewPepper(afterRotation) error = %v", err)
	}
	ok, err = auth.VerifyPassword("s3cret", rehash, afterRotation)
	if err != nil {
		t.Fatalf("VerifyPassword() after rotation: error = %v", err)
	}
	if !ok {
		t.Error("VerifyPassword() of a v2-hashed password after rotation = false, want true")
	}

	// The old hash no longer verifies once v1 is fully dropped.
	if _, err := auth.VerifyPassword("s3cret", hash, afterRotation); !errors.Is(err, auth.ErrUnknownPepperVersion) {
		t.Errorf("VerifyPassword() of a v1-hashed password after v1 is dropped: error = %v, want ErrUnknownPepperVersion", err)
	}
}

func TestNewPepperRejectsWrongKeyLength(t *testing.T) {
	if _, err := auth.NewPepper("v1", []byte("too short")); err == nil {
		t.Error("NewPepper() with a short key: want error, got nil")
	}
}

func TestLoadPepperParsesEnv(t *testing.T) {
	key := bytes.Repeat([]byte("k"), 32)
	t.Setenv(auth.PepperKeyEnv, "v1:"+base64.StdEncoding.EncodeToString(key))
	t.Setenv(auth.PepperKeyRetiredEnv, "")

	pep, err := auth.LoadPepper()
	if err != nil {
		t.Fatalf("LoadPepper() error = %v", err)
	}
	hash, err := auth.HashPassword("s3cret", pep)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	ok, err := auth.VerifyPassword("s3cret", hash, pep)
	if err != nil || !ok {
		t.Errorf("round trip through LoadPepper(): ok = %v, err = %v", ok, err)
	}
}

func TestLoadPepperMissingKeyFails(t *testing.T) {
	t.Setenv(auth.PepperKeyEnv, "")
	if _, err := auth.LoadPepper(); err == nil {
		t.Error("LoadPepper() with no ENCRE_PEPPER_KEY set: want error, got nil")
	}
}
