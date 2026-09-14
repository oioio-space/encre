package store_test

import (
	"errors"
	"testing"
	"time"

	"github.com/oioio-space/encre/server/store"
)

func TestCreateAndFetchSession(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")

	sess := &store.Session{
		Token:       "tok1",
		Kind:        store.SessionChild,
		SubjectID:   childID,
		ExpiresAt:   time.Now().Add(24 * time.Hour).UTC(),
		TOTPOKUntil: time.Time{},
	}
	if err := db.CreateSession(ctx, sess); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	got, err := db.Session(ctx, "tok1")
	if err != nil {
		t.Fatalf("Session() error = %v", err)
	}
	if got.SubjectID != childID || got.Kind != store.SessionChild {
		t.Errorf("Session() = %+v", got)
	}
}

func TestSessionNotFound(t *testing.T) {
	db := openTestStore(t)
	if _, err := db.Session(t.Context(), "nope"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Session() error = %v, want ErrNotFound", err)
	}
}

func TestDeleteSession(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")

	sess := &store.Session{Token: "tok1", Kind: store.SessionChild, SubjectID: childID, ExpiresAt: time.Now().Add(time.Hour)}
	if err := db.CreateSession(ctx, sess); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if err := db.DeleteSession(ctx, "tok1"); err != nil {
		t.Fatalf("DeleteSession() error = %v", err)
	}
	if _, err := db.Session(ctx, "tok1"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Session() after delete: error = %v, want ErrNotFound", err)
	}
}

func TestDeleteSessionIsIdempotent(t *testing.T) {
	db := openTestStore(t)
	if err := db.DeleteSession(t.Context(), "nope"); err != nil {
		t.Errorf("DeleteSession() on unknown token: error = %v, want nil", err)
	}
}

func TestCreateSessionDuplicateTokenFails(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")

	sess := &store.Session{Token: "tok1", Kind: store.SessionChild, SubjectID: childID, ExpiresAt: time.Now().Add(time.Hour)}
	if err := db.CreateSession(ctx, sess); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if err := db.CreateSession(ctx, sess); err == nil {
		t.Error("CreateSession() with duplicate token: want error, got nil")
	}
}

func TestSessionWithTOTPOKUntil(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")

	totpOK := time.Unix(1_700_003_600, 0).UTC()
	sess := &store.Session{
		Token: "tok1", Kind: store.SessionParent, SubjectID: childID,
		ExpiresAt: time.Now().Add(time.Hour), TOTPOKUntil: totpOK,
	}
	if err := db.CreateSession(ctx, sess); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	got, err := db.Session(ctx, "tok1")
	if err != nil {
		t.Fatalf("Session() error = %v", err)
	}
	if !got.TOTPOKUntil.Equal(totpOK) {
		t.Errorf("TOTPOKUntil = %v, want %v", got.TOTPOKUntil, totpOK)
	}
	if got.Kind != store.SessionParent {
		t.Errorf("Kind = %v, want SessionParent", got.Kind)
	}
}

func TestPurgeExpiredSessions(t *testing.T) {
	db := openTestStore(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")

	expired := &store.Session{Token: "expired", Kind: store.SessionChild, SubjectID: childID, ExpiresAt: time.Now().Add(-time.Hour)}
	fresh := &store.Session{Token: "fresh", Kind: store.SessionChild, SubjectID: childID, ExpiresAt: time.Now().Add(time.Hour)}
	if err := db.CreateSession(ctx, expired); err != nil {
		t.Fatalf("CreateSession(expired) error = %v", err)
	}
	if err := db.CreateSession(ctx, fresh); err != nil {
		t.Fatalf("CreateSession(fresh) error = %v", err)
	}

	if err := db.PurgeExpiredSessions(ctx); err != nil {
		t.Fatalf("PurgeExpiredSessions() error = %v", err)
	}

	if _, err := db.Session(ctx, "expired"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Session(expired) after purge: error = %v, want ErrNotFound", err)
	}
	if _, err := db.Session(ctx, "fresh"); err != nil {
		t.Errorf("Session(fresh) after purge: error = %v, want nil", err)
	}
}

func TestCreateSessionFailsOnClosedDB(t *testing.T) {
	db := openTestStore(t)
	if err := db.DB().Close(); err != nil {
		t.Fatalf("closing underlying db: %v", err)
	}

	sess := &store.Session{Token: "tok1", Kind: store.SessionChild, SubjectID: "child1", ExpiresAt: time.Now()}
	if err := db.CreateSession(t.Context(), sess); err == nil {
		t.Error("CreateSession() on a closed database: want error, got nil")
	}
}

func TestSessionFailsOnClosedDB(t *testing.T) {
	db := openTestStore(t)
	if err := db.DB().Close(); err != nil {
		t.Fatalf("closing underlying db: %v", err)
	}

	if _, err := db.Session(t.Context(), "tok1"); err == nil {
		t.Error("Session() on a closed database: want error, got nil")
	}
}

func TestDeleteSessionFailsOnClosedDB(t *testing.T) {
	db := openTestStore(t)
	if err := db.DB().Close(); err != nil {
		t.Fatalf("closing underlying db: %v", err)
	}

	if err := db.DeleteSession(t.Context(), "tok1"); err == nil {
		t.Error("DeleteSession() on a closed database: want error, got nil")
	}
}

func TestPurgeExpiredSessionsFailsOnClosedDB(t *testing.T) {
	db := openTestStore(t)
	if err := db.DB().Close(); err != nil {
		t.Fatalf("closing underlying db: %v", err)
	}

	if err := db.PurgeExpiredSessions(t.Context()); err == nil {
		t.Error("PurgeExpiredSessions() on a closed database: want error, got nil")
	}
}
