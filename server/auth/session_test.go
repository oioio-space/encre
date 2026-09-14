package auth_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/oioio-space/encre/server/auth"
	"github.com/oioio-space/encre/server/store"
	"pgregory.net/rapid"
)

func TestNewSessionTokenIsUnique(t *testing.T) {
	seen := make(map[string]bool)
	for range 10_000 {
		tok, err := auth.NewSessionToken()
		if err != nil {
			t.Fatalf("NewSessionToken() error = %v", err)
		}
		if seen[tok] {
			t.Fatalf("NewSessionToken() produced a duplicate: %q", tok)
		}
		seen[tok] = true
	}
}

// TestNewSessionTokenIsUniqueProperty is the rapid-flavored version of the
// same property, run through rapid's own iteration count and shrinker.
func TestNewSessionTokenIsUniqueProperty(t *testing.T) {
	seen := make(map[string]bool)
	rapid.Check(t, func(t *rapid.T) {
		tok, err := auth.NewSessionToken()
		if err != nil {
			t.Fatalf("NewSessionToken() error = %v", err)
		}
		if seen[tok] {
			t.Fatalf("NewSessionToken() produced a duplicate: %q", tok)
		}
		seen[tok] = true
	})
}

func TestCreateAndLookupSession(t *testing.T) {
	db := openTestDB(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")
	now := time.Unix(1_700_000_000, 0).UTC()

	token, err := auth.CreateSession(ctx, db, store.SessionChild, childID, now)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	sess, err := auth.LookupSession(ctx, db, token, store.SessionChild, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("LookupSession() error = %v", err)
	}
	if sess.SubjectID != childID {
		t.Errorf("LookupSession().SubjectID = %q, want %q", sess.SubjectID, childID)
	}
	if !sess.ExpiresAt.Equal(now.Add(auth.ChildSessionTTL)) {
		t.Errorf("ExpiresAt = %v, want %v", sess.ExpiresAt, now.Add(auth.ChildSessionTTL))
	}
}

func TestCreateSessionUsesParentTTL(t *testing.T) {
	db := openTestDB(t)
	ctx := t.Context()
	parentID := seedParent(t, db, "parent1")
	now := time.Unix(1_700_000_000, 0).UTC()

	token, err := auth.CreateSession(ctx, db, store.SessionParent, parentID, now)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	sess, err := auth.LookupSession(ctx, db, token, store.SessionParent, now)
	if err != nil {
		t.Fatalf("LookupSession() error = %v", err)
	}
	if !sess.ExpiresAt.Equal(now.Add(auth.ParentSessionTTL)) {
		t.Errorf("ExpiresAt = %v, want %v", sess.ExpiresAt, now.Add(auth.ParentSessionTTL))
	}
	if auth.ParentSessionTTL != 7*24*time.Hour {
		t.Errorf("ParentSessionTTL = %v, want 7 days (ENCRE_04 §7)", auth.ParentSessionTTL)
	}
	if auth.ChildSessionTTL != 24*time.Hour {
		t.Errorf("ChildSessionTTL = %v, want 24h (ENCRE_04 §7)", auth.ChildSessionTTL)
	}
}

func TestLookupSessionExpired(t *testing.T) {
	db := openTestDB(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")
	now := time.Unix(1_700_000_000, 0).UTC()

	token, err := auth.CreateSession(ctx, db, store.SessionChild, childID, now)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	_, err = auth.LookupSession(ctx, db, token, store.SessionChild, now.Add(auth.ChildSessionTTL+time.Second))
	if !errors.Is(err, auth.ErrSessionExpired) {
		t.Errorf("LookupSession() after expiry: error = %v, want ErrSessionExpired", err)
	}
}

func TestLookupSessionUnknownToken(t *testing.T) {
	db := openTestDB(t)
	_, err := auth.LookupSession(t.Context(), db, "does-not-exist", store.SessionChild, time.Now())
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("LookupSession() error = %v, want ErrNotFound", err)
	}
}

// TestLookupSessionRejectsWrongKind is the regression test for the privilege
// escalation an independent audit found: a child session token, presented as
// if it were a parent session (the attack is trivial — copy the encre_child
// cookie's value into encre_parent), must not resolve to anything. Before
// this fix LookupSession trusted whichever kind the row on disk actually
// carried and never compared it to what the caller asked for.
func TestLookupSessionRejectsWrongKind(t *testing.T) {
	db := openTestDB(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")
	now := time.Unix(1_700_000_000, 0).UTC()

	childToken, err := auth.CreateSession(ctx, db, store.SessionChild, childID, now)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	// The child's own token, asked for as a parent session: this is the
	// escalation — it must fail exactly like an unknown token, not reveal
	// that the token is valid for a different kind.
	_, err = auth.LookupSession(ctx, db, childToken, store.SessionParent, now)
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("LookupSession(childToken, want=SessionParent) error = %v, want ErrNotFound", err)
	}

	// The same token, asked for as what it actually is, still works: this
	// rules out a fix that broke correct lookups instead of just rejecting
	// mismatched ones.
	sess, err := auth.LookupSession(ctx, db, childToken, store.SessionChild, now)
	if err != nil {
		t.Fatalf("LookupSession(childToken, want=SessionChild) error = %v", err)
	}
	if sess.SubjectID != childID {
		t.Errorf("SubjectID = %q, want %q", sess.SubjectID, childID)
	}
}

func TestSessionFromCookieDerivesKindFromCookieName(t *testing.T) {
	db := openTestDB(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")
	now := time.Unix(1_700_000_000, 0).UTC()

	childToken, err := auth.CreateSession(ctx, db, store.SessionChild, childID, now)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	// A child token copied into the parent cookie must not resolve, even
	// through the cookie-reading entry point real handlers use.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieParent, Value: childToken})
	if _, err := auth.SessionFromCookie(req, db, auth.CookieParent, now); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("SessionFromCookie(child token under CookieParent) error = %v, want ErrNotFound", err)
	}

	// The matching cookie name resolves normally.
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieChild, Value: childToken})
	sess, err := auth.SessionFromCookie(req, db, auth.CookieChild, now)
	if err != nil {
		t.Fatalf("SessionFromCookie(child token under CookieChild) error = %v", err)
	}
	if sess.SubjectID != childID {
		t.Errorf("SubjectID = %q, want %q", sess.SubjectID, childID)
	}
}

// TestSessionTokenNeverStoredInClear reads the sessions table directly:
// production code that ever persisted the raw cookie value instead of its
// hash would still pass every LookupSession-based test, because LookupSession
// hashes before it queries either way. Only a direct read of the row catches
// it.
func TestSessionTokenNeverStoredInClear(t *testing.T) {
	db := openTestDB(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")
	now := time.Unix(1_700_000_000, 0).UTC()

	token, err := auth.CreateSession(ctx, db, store.SessionChild, childID, now)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	var stored string
	err = db.DB().QueryRowContext(ctx, `SELECT token FROM sessions WHERE subject_id = ?`, childID).Scan(&stored)
	if err != nil {
		t.Fatalf("querying sessions table: %v", err)
	}
	if stored == token {
		t.Error("sessions.token equals the raw cookie value, want a hash")
	}
	if strings.Contains(stored, token) {
		t.Error("sessions.token contains the raw cookie value, want a hash")
	}
}

func TestSetSessionCookie(t *testing.T) {
	rec := httptest.NewRecorder()
	auth.SetSessionCookie(rec, auth.CookieChild, "tok-value", auth.ChildSessionTTL)

	res := rec.Result()
	cookies := res.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("len(cookies) = %d, want 1", len(cookies))
	}
	c := cookies[0]
	if c.Name != auth.CookieChild {
		t.Errorf("cookie name = %q, want %q", c.Name, auth.CookieChild)
	}
	if c.Value != "tok-value" {
		t.Errorf("cookie value = %q, want %q", c.Value, "tok-value")
	}
	if !c.HttpOnly {
		t.Error("cookie HttpOnly = false, want true (ENCRE_04 §7)")
	}
	if !c.Secure {
		t.Error("cookie Secure = false, want true (ENCRE_04 §7)")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie SameSite = %v, want Lax (ENCRE_04 §7)", c.SameSite)
	}
}

func TestClearSessionCookie(t *testing.T) {
	rec := httptest.NewRecorder()
	auth.ClearSessionCookie(rec, auth.CookieParent)

	c := rec.Result().Cookies()[0]
	if c.MaxAge >= 0 {
		t.Errorf("cookie MaxAge = %d, want negative (expire immediately)", c.MaxAge)
	}
}
