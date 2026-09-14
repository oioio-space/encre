package store_test

import (
	"errors"
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

func TestParentByIDFailsOnClosedDB(t *testing.T) {
	db := openTestStore(t)
	if err := db.DB().Close(); err != nil {
		t.Fatalf("closing underlying db: %v", err)
	}

	if _, err := db.ParentByID(t.Context(), "p1"); err == nil {
		t.Error("ParentByID() on a closed database: want error, got nil")
	}
}
