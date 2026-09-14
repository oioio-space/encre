package auth_test

import (
	"testing"
	"time"

	"github.com/oioio-space/encre/server/store"
)

// openTestDB opens a fresh in-memory store, closed automatically when t ends.
func openTestDB(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// seedParent creates a parent row with id as both its ID and email local
// part, returning the ID.
func seedParent(t *testing.T, db *store.Store, id string) string {
	t.Helper()
	p := &store.Parent{
		ID:        id,
		Email:     id + "@example.com",
		PassHash:  []byte("placeholder"),
		CreatedAt: time.Now(),
	}
	if err := db.CreateParent(t.Context(), p); err != nil {
		t.Fatalf("CreateParent() error = %v", err)
	}
	return id
}

// seedChild creates a parent and a child under it, returning the child's ID.
func seedChild(t *testing.T, db *store.Store, id string) string {
	t.Helper()
	seedParent(t, db, id+"-parent")
	c := &store.Child{ID: id, ParentID: id + "-parent", Pseudo: id, CreatedAt: time.Now()}
	if err := db.CreateChild(t.Context(), c); err != nil {
		t.Fatalf("CreateChild() error = %v", err)
	}
	return id
}
