package store_test

import (
	"errors"
	"testing"
	"time"

	"github.com/oioio-space/encre/server/store"
)

func TestPutAndGetTOTPPending(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	seedParent(t, db, "p1")

	now := time.Unix(1_700_000_000, 0).UTC()
	p := &store.TOTPPending{ParentID: "p1", Secret: []byte("sealed"), ExpiresAt: now.Add(10 * time.Minute)}
	if err := db.PutTOTPPending(ctx, p); err != nil {
		t.Fatalf("PutTOTPPending() error = %v", err)
	}

	got, err := db.TOTPPendingByParentID(ctx, "p1")
	if err != nil {
		t.Fatalf("TOTPPendingByParentID() error = %v", err)
	}
	if string(got.Secret) != "sealed" {
		t.Errorf("Secret = %q, want %q", got.Secret, "sealed")
	}
	if !got.ExpiresAt.Equal(p.ExpiresAt) {
		t.Errorf("ExpiresAt = %v, want %v", got.ExpiresAt, p.ExpiresAt)
	}
}

// TestPutTOTPPendingReplacesEarlierEnrollment checks that reloading the
// enrollment page (a fresh [store.TOTPPending] for a parent that already had
// one) overwrites rather than duplicates the row.
func TestPutTOTPPendingReplacesEarlierEnrollment(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	seedParent(t, db, "p1")

	now := time.Unix(1_700_000_000, 0).UTC()
	first := &store.TOTPPending{ParentID: "p1", Secret: []byte("first"), ExpiresAt: now}
	if err := db.PutTOTPPending(ctx, first); err != nil {
		t.Fatalf("PutTOTPPending(first) error = %v", err)
	}
	second := &store.TOTPPending{ParentID: "p1", Secret: []byte("second"), ExpiresAt: now.Add(time.Minute)}
	if err := db.PutTOTPPending(ctx, second); err != nil {
		t.Fatalf("PutTOTPPending(second) error = %v", err)
	}

	got, err := db.TOTPPendingByParentID(ctx, "p1")
	if err != nil {
		t.Fatalf("TOTPPendingByParentID() error = %v", err)
	}
	if string(got.Secret) != "second" {
		t.Errorf("Secret = %q, want %q (replaced, not duplicated)", got.Secret, "second")
	}
}

func TestTOTPPendingByParentIDNotFound(t *testing.T) {
	db := openTestStore(t)
	if _, err := db.TOTPPendingByParentID(t.Context(), "nope"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("TOTPPendingByParentID(unknown) error = %v, want ErrNotFound", err)
	}
}

func TestDeleteTOTPPending(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	seedParent(t, db, "p1")

	if err := db.PutTOTPPending(ctx, &store.TOTPPending{ParentID: "p1", Secret: []byte("s"), ExpiresAt: time.Now()}); err != nil {
		t.Fatalf("PutTOTPPending() error = %v", err)
	}
	if err := db.DeleteTOTPPending(ctx, "p1"); err != nil {
		t.Fatalf("DeleteTOTPPending() error = %v", err)
	}
	if _, err := db.TOTPPendingByParentID(ctx, "p1"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("TOTPPendingByParentID() after delete: error = %v, want ErrNotFound", err)
	}

	// Deleting a row that does not exist is not an error.
	if err := db.DeleteTOTPPending(ctx, "p1"); err != nil {
		t.Errorf("DeleteTOTPPending() on an already-deleted row: error = %v, want nil", err)
	}
}

func TestPurgeExpiredTOTPPending(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	seedParent(t, db, "p1")
	seedParent(t, db, "p2")

	now := time.Unix(1_700_000_000, 0).UTC()
	if err := db.PutTOTPPending(ctx, &store.TOTPPending{ParentID: "p1", Secret: []byte("expired"), ExpiresAt: now.Add(-time.Minute)}); err != nil {
		t.Fatalf("PutTOTPPending(p1) error = %v", err)
	}
	if err := db.PutTOTPPending(ctx, &store.TOTPPending{ParentID: "p2", Secret: []byte("fresh"), ExpiresAt: now.Add(time.Minute)}); err != nil {
		t.Fatalf("PutTOTPPending(p2) error = %v", err)
	}

	if err := db.PurgeExpiredTOTPPending(ctx, now); err != nil {
		t.Fatalf("PurgeExpiredTOTPPending() error = %v", err)
	}

	if _, err := db.TOTPPendingByParentID(ctx, "p1"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("TOTPPendingByParentID(p1) after purge: error = %v, want ErrNotFound", err)
	}
	if _, err := db.TOTPPendingByParentID(ctx, "p2"); err != nil {
		t.Errorf("TOTPPendingByParentID(p2) after purge: error = %v, want nil", err)
	}
}
