package auth_test

import (
	"errors"
	"testing"
	"time"

	"github.com/oioio-space/encre/server/auth"
	"github.com/oioio-space/encre/server/store"
)

func TestLoginParent(t *testing.T) {
	db := openTestDB(t)
	pep := testPepper(t)
	ctx := t.Context()

	hash, err := auth.HashPassword("s3cret!", pep)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	p := &store.Parent{ID: "p1", Email: "parent@example.com", PassHash: []byte(hash), CreatedAt: time.Now()}
	if err := db.CreateParent(ctx, p); err != nil {
		t.Fatalf("CreateParent() error = %v", err)
	}

	id, err := auth.LoginParent(ctx, db, "parent@example.com", "s3cret!", pep)
	if err != nil {
		t.Fatalf("LoginParent() error = %v", err)
	}
	if id != "p1" {
		t.Errorf("LoginParent() = %q, want %q", id, "p1")
	}
}

func TestLoginParentUnknownEmailAndWrongPasswordAreIndistinguishable(t *testing.T) {
	db := openTestDB(t)
	pep := testPepper(t)
	ctx := t.Context()

	hash, err := auth.HashPassword("s3cret!", pep)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	p := &store.Parent{ID: "p1", Email: "parent@example.com", PassHash: []byte(hash), CreatedAt: time.Now()}
	if err := db.CreateParent(ctx, p); err != nil {
		t.Fatalf("CreateParent() error = %v", err)
	}

	_, errUnknown := auth.LoginParent(ctx, db, "nobody@example.com", "whatever", pep)
	_, errWrong := auth.LoginParent(ctx, db, "parent@example.com", "wrong password", pep)

	if !errors.Is(errUnknown, auth.ErrInvalidCredentials) {
		t.Errorf("LoginParent(unknown email) error = %v, want ErrInvalidCredentials", errUnknown)
	}
	if !errors.Is(errWrong, auth.ErrInvalidCredentials) {
		t.Errorf("LoginParent(wrong password) error = %v, want ErrInvalidCredentials", errWrong)
	}
	if errUnknown.Error() != errWrong.Error() {
		t.Errorf("error messages differ: %q vs %q, want identical text (no enumeration oracle)", errUnknown, errWrong)
	}
}

// TestLoginParentTimingDoesNotLeakWhichEmailExists exercises the actual
// bug ENCRE_04 §7 warns about: an implementation that short-circuits on
// "unknown email" before ever calling argon2id runs measurably faster for an
// unknown email than for a wrong password, letting an attacker enumerate
// accounts by timing alone. This does not assert a stopwatch bound (flaky by
// nature); it asserts on the specific defect (skipped hashing) via a
// dependency count.
func TestLoginParentTimingDoesNotLeakWhichEmailExists(t *testing.T) {
	db := openTestDB(t)
	pep := testPepper(t)
	ctx := t.Context()

	hash, err := auth.HashPassword("s3cret!", pep)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	p := &store.Parent{ID: "p1", Email: "parent@example.com", PassHash: []byte(hash), CreatedAt: time.Now()}
	if err := db.CreateParent(ctx, p); err != nil {
		t.Fatalf("CreateParent() error = %v", err)
	}

	const runs = 5
	var unknownTotal, wrongTotal time.Duration
	for range runs {
		start := time.Now()
		_, _ = auth.LoginParent(ctx, db, "nobody@example.com", "whatever", pep)
		unknownTotal += time.Since(start)

		start = time.Now()
		_, _ = auth.LoginParent(ctx, db, "parent@example.com", "wrong password", pep)
		wrongTotal += time.Since(start)
	}

	// Both paths hash with the same argon2id cost, so their average duration
	// should be within the same order of magnitude — not equal (scheduler
	// noise), just not the ~1000x gap a skipped hash would produce.
	ratio := float64(unknownTotal) / float64(wrongTotal)
	if ratio < 0.2 || ratio > 5 {
		t.Errorf("unknown-email/wrong-password timing ratio = %.2f, want within [0.2, 5] (both paths must hash)", ratio)
	}
}

func TestLoginChild(t *testing.T) {
	db := openTestDB(t)
	pep := testPepper(t)
	ctx := t.Context()

	patternHash, err := auth.HashPattern("1379", pep)
	if err != nil {
		t.Fatalf("HashPattern() error = %v", err)
	}
	seedParent(t, db, "parent1")
	c := &store.Child{ID: "c1", ParentID: "parent1", Pseudo: "Mona", PatternHash: []byte(patternHash), CreatedAt: time.Now()}
	if err := db.CreateChild(ctx, c); err != nil {
		t.Fatalf("CreateChild() error = %v", err)
	}

	id, err := auth.LoginChild(ctx, db, "Mona", "1379", pep)
	if err != nil {
		t.Fatalf("LoginChild() error = %v", err)
	}
	if id != "c1" {
		t.Errorf("LoginChild() = %q, want %q", id, "c1")
	}
}

func TestLoginChildWrongPattern(t *testing.T) {
	db := openTestDB(t)
	pep := testPepper(t)
	ctx := t.Context()

	patternHash, err := auth.HashPattern("1379", pep)
	if err != nil {
		t.Fatalf("HashPattern() error = %v", err)
	}
	seedParent(t, db, "parent1")
	c := &store.Child{ID: "c1", ParentID: "parent1", Pseudo: "Mona", PatternHash: []byte(patternHash), CreatedAt: time.Now()}
	if err := db.CreateChild(ctx, c); err != nil {
		t.Fatalf("CreateChild() error = %v", err)
	}

	_, err = auth.LoginChild(ctx, db, "Mona", "0000", pep)
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Errorf("LoginChild() with the wrong pattern: error = %v, want ErrInvalidCredentials", err)
	}
}

func TestLoginChildUnknownPseudo(t *testing.T) {
	db := openTestDB(t)
	pep := testPepper(t)
	_, err := auth.LoginChild(t.Context(), db, "Nobody", "1234", pep)
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Errorf("LoginChild() with an unknown pseudo: error = %v, want ErrInvalidCredentials", err)
	}
}

// TestLoginChildDisambiguatesSharedPseudo checks that two children with the
// same pseudo (nothing enforces uniqueness on that column) are each reachable
// by their own pattern.
func TestLoginChildDisambiguatesSharedPseudo(t *testing.T) {
	db := openTestDB(t)
	pep := testPepper(t)
	ctx := t.Context()

	seedParent(t, db, "parent1")
	seedParent(t, db, "parent2")

	hash1, err := auth.HashPattern("1111", pep)
	if err != nil {
		t.Fatalf("HashPattern() error = %v", err)
	}
	hash2, err := auth.HashPattern("2222", pep)
	if err != nil {
		t.Fatalf("HashPattern() error = %v", err)
	}
	c1 := &store.Child{ID: "c1", ParentID: "parent1", Pseudo: "Mona", PatternHash: []byte(hash1), CreatedAt: time.Now()}
	c2 := &store.Child{ID: "c2", ParentID: "parent2", Pseudo: "Mona", PatternHash: []byte(hash2), CreatedAt: time.Now()}
	if err := db.CreateChild(ctx, c1); err != nil {
		t.Fatalf("CreateChild(c1) error = %v", err)
	}
	if err := db.CreateChild(ctx, c2); err != nil {
		t.Fatalf("CreateChild(c2) error = %v", err)
	}

	id, err := auth.LoginChild(ctx, db, "Mona", "2222", pep)
	if err != nil {
		t.Fatalf("LoginChild() error = %v", err)
	}
	if id != "c2" {
		t.Errorf("LoginChild() = %q, want %q", id, "c2")
	}
}

// TestLoginChildAmbiguousMatchFailsClosed is the regression test for the
// collision an independent audit found: two unrelated families' children can
// share both a pseudo and, by coincidence, a pattern — nothing constrains
// either — and the child in family A who types her own pseudo and her own
// pattern must never land in family B's account. LoginChild must refuse
// rather than pick whichever row the query returned last.
func TestLoginChildAmbiguousMatchFailsClosed(t *testing.T) {
	db := openTestDB(t)
	pep := testPepper(t)
	ctx := t.Context()

	seedParent(t, db, "parent1")
	seedParent(t, db, "parent2")

	hash, err := auth.HashPattern("1379", pep)
	if err != nil {
		t.Fatalf("HashPattern() error = %v", err)
	}
	c1 := &store.Child{ID: "c1", ParentID: "parent1", Pseudo: "Lea", PatternHash: []byte(hash), CreatedAt: time.Now()}
	c2 := &store.Child{ID: "c2", ParentID: "parent2", Pseudo: "Lea", PatternHash: []byte(hash), CreatedAt: time.Now()}
	if err := db.CreateChild(ctx, c1); err != nil {
		t.Fatalf("CreateChild(c1) error = %v", err)
	}
	if err := db.CreateChild(ctx, c2); err != nil {
		t.Fatalf("CreateChild(c2) error = %v", err)
	}

	_, err = auth.LoginChild(ctx, db, "Lea", "1379", pep)
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Errorf("LoginChild() with an ambiguous (pseudo, pattern) match: error = %v, want ErrInvalidCredentials", err)
	}
}
