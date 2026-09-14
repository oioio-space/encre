package store_test

import (
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/oioio-space/encre/server/store"
)

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestCreateAndFetchParent(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()

	p := &store.Parent{
		ID:         "p1",
		Email:      "mathieu@example.com",
		PassHash:   []byte("hash"),
		TOTPSecret: []byte("secret"),
		CreatedAt:  time.Unix(1_700_000_000, 0).UTC(),
	}
	if err := db.CreateParent(ctx, p); err != nil {
		t.Fatalf("CreateParent() error = %v", err)
	}

	byID, err := db.ParentByID(ctx, "p1")
	if err != nil {
		t.Fatalf("ParentByID() error = %v", err)
	}
	if byID.Email != p.Email {
		t.Errorf("ParentByID().Email = %q, want %q", byID.Email, p.Email)
	}

	byEmail, err := db.ParentByEmail(ctx, "mathieu@example.com")
	if err != nil {
		t.Fatalf("ParentByEmail() error = %v", err)
	}
	if byEmail.ID != p.ID {
		t.Errorf("ParentByEmail().ID = %q, want %q", byEmail.ID, p.ID)
	}
}

func TestParentByIDNotFound(t *testing.T) {
	db := openTestStore(t)

	_, err := db.ParentByID(t.Context(), "nope")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("ParentByID() error = %v, want ErrNotFound", err)
	}
}

func TestParentByEmailNotFound(t *testing.T) {
	db := openTestStore(t)

	_, err := db.ParentByEmail(t.Context(), "nope@example.com")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("ParentByEmail() error = %v, want ErrNotFound", err)
	}
}

func TestCreateParentDuplicateEmailFails(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()

	p := &store.Parent{ID: "p1", Email: "dup@example.com", PassHash: []byte("h"), CreatedAt: time.Now()}
	if err := db.CreateParent(ctx, p); err != nil {
		t.Fatalf("CreateParent() error = %v", err)
	}

	dup := &store.Parent{ID: "p2", Email: "dup@example.com", PassHash: []byte("h"), CreatedAt: time.Now()}
	if err := db.CreateParent(ctx, dup); err == nil {
		t.Error("CreateParent() with duplicate email: want error, got nil")
	}
}

func TestCreateParentFailsOnClosedDB(t *testing.T) {
	db := openTestStore(t)
	if err := db.DB().Close(); err != nil {
		t.Fatalf("closing underlying db: %v", err)
	}

	p := &store.Parent{ID: "p1", Email: "p1@example.com", PassHash: []byte("h"), CreatedAt: time.Now()}
	if err := db.CreateParent(t.Context(), p); err == nil {
		t.Error("CreateParent() on a closed database: want error, got nil")
	}
}

func TestParentByFamilyCode(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()

	p := &store.Parent{
		ID: "p1", Email: "p1@example.com", PassHash: []byte("h"),
		FamilyCode: "family-abc", CreatedAt: time.Now(),
	}
	if err := db.CreateParent(ctx, p); err != nil {
		t.Fatalf("CreateParent() error = %v", err)
	}

	got, err := db.ParentByFamilyCode(ctx, "family-abc")
	if err != nil {
		t.Fatalf("ParentByFamilyCode() error = %v", err)
	}
	if got.ID != "p1" {
		t.Errorf("ParentByFamilyCode().ID = %q, want %q", got.ID, "p1")
	}

	if _, err := db.ParentByFamilyCode(ctx, "nope"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("ParentByFamilyCode(unknown) error = %v, want ErrNotFound", err)
	}
}

// TestParentFamilyCodeIsUnique is the regression test for encre-qpx.5's
// scoping guarantee: two parents cannot share a family code, or a
// family-scoped child login page reached through one code could resolve
// against the wrong parent's [store.Store.ChildrenOfParent] list.
func TestParentFamilyCodeIsUnique(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()

	a := &store.Parent{ID: "p1", Email: "a@example.com", PassHash: []byte("h"), FamilyCode: "dup", CreatedAt: time.Now()}
	if err := db.CreateParent(ctx, a); err != nil {
		t.Fatalf("CreateParent(a) error = %v", err)
	}
	b := &store.Parent{ID: "p2", Email: "b@example.com", PassHash: []byte("h"), FamilyCode: "dup", CreatedAt: time.Now()}
	if err := db.CreateParent(ctx, b); err == nil {
		t.Error("CreateParent() with a duplicate family code: want error, got nil")
	}
}

func TestSetParentTOTPSecretAndPassHash(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()

	p := &store.Parent{ID: "p1", Email: "p1@example.com", PassHash: []byte("h"), CreatedAt: time.Now()}
	if err := db.CreateParent(ctx, p); err != nil {
		t.Fatalf("CreateParent() error = %v", err)
	}

	if err := db.SetParentTOTPSecret(ctx, "p1", []byte("sealed-secret")); err != nil {
		t.Fatalf("SetParentTOTPSecret() error = %v", err)
	}
	if err := db.SetParentPassHash(ctx, "p1", []byte("new-hash")); err != nil {
		t.Fatalf("SetParentPassHash() error = %v", err)
	}

	got, err := db.ParentByID(ctx, "p1")
	if err != nil {
		t.Fatalf("ParentByID() error = %v", err)
	}
	if string(got.TOTPSecret) != "sealed-secret" {
		t.Errorf("TOTPSecret = %q, want %q", got.TOTPSecret, "sealed-secret")
	}
	if string(got.PassHash) != "new-hash" {
		t.Errorf("PassHash = %q, want %q", got.PassHash, "new-hash")
	}

	if err := db.SetParentTOTPSecret(ctx, "nope", []byte("x")); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("SetParentTOTPSecret(unknown parent) error = %v, want ErrNotFound", err)
	}
	if err := db.SetParentPassHash(ctx, "nope", []byte("x")); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("SetParentPassHash(unknown parent) error = %v, want ErrNotFound", err)
	}
}

// TestParentLogValueRedactsSecrets is the test encre-qpx.8 (M7) asks for:
// logging a Parent must never write its password hash or TOTP secret to the
// log output, structured or not.
func TestParentLogValueRedactsSecrets(t *testing.T) {
	p := &store.Parent{
		ID: "p1", Email: "p1@example.com",
		PassHash:   []byte("super-secret-argon2-hash"),
		TOTPSecret: []byte("super-secret-totp-seed"),
		CreatedAt:  time.Now(),
	}

	var buf strings.Builder
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	logger.Info("parent", "parent", p)
	logger.Info("parent-plain", "parent", p.String())

	out := buf.String()
	if strings.Contains(out, "super-secret-argon2-hash") {
		t.Errorf("log output contains the raw password hash: %s", out)
	}
	if strings.Contains(out, "super-secret-totp-seed") {
		t.Errorf("log output contains the raw totp secret: %s", out)
	}
	if !strings.Contains(out, "p1@example.com") {
		t.Errorf("log output does not contain the email (not a secret this rule protects): %s", out)
	}
}

func TestParentByIDFailsOnClosedDB(t *testing.T) {
	db := openTestStore(t)
	if err := db.DB().Close(); err != nil {
		t.Fatalf("closing underlying db: %v", err)
	}

	if _, err := db.ParentByID(t.Context(), "p1"); err == nil {
		t.Error("ParentByID() on a closed database: want error, got nil")
	}
}
