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

// TestRequireParentRejectsSessionWithNoTOTPCheckYet checks that a brand-new
// parent session — valid, but never through a TOTP check — does not satisfy
// RequireParent: TOTPOKUntil is the zero Time until [VerifyParentTOTPForSession]
// sets it, and the zero Time is never "after now".
func TestRequireParentRejectsSessionWithNoTOTPCheckYet(t *testing.T) {
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

	if err := auth.RequireParent(sess, now); !errors.Is(err, auth.ErrTOTPRequired) {
		t.Errorf("RequireParent() before any TOTP check: error = %v, want ErrTOTPRequired", err)
	}
}

// TestRequireParentAcceptsFreshParentSession checks the happy path: a
// parent session right after a successful TOTP check satisfies RequireParent.
func TestRequireParentAcceptsFreshParentSession(t *testing.T) {
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
	sess.TOTPOKUntil = now.Add(auth.TOTPFreshDuration)

	if err := auth.RequireParent(sess, now); err != nil {
		t.Errorf("RequireParent() right after a TOTP check: error = %v, want nil", err)
	}
}

// TestRequireParentRejectsLapsedFreshness is encre-qpx.6's core regression
// test (M5): a parent session stays valid for ParentSessionTTL (7 days) but
// TOTP freshness only lasts TOTPFreshDuration (1 hour) — RequireParent must
// refuse the session once freshness has lapsed, well before the session
// itself expires.
func TestRequireParentRejectsLapsedFreshness(t *testing.T) {
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
	sess.TOTPOKUntil = now.Add(auth.TOTPFreshDuration)

	if err := auth.RequireParent(sess, now); err != nil {
		t.Errorf("RequireParent() right after a TOTP check: error = %v, want nil", err)
	}

	later := now.Add(auth.TOTPFreshDuration + time.Second)
	if err := auth.RequireParent(sess, later); !errors.Is(err, auth.ErrTOTPRequired) {
		t.Errorf("RequireParent() past TOTPFreshDuration (session itself still valid for days): error = %v, want ErrTOTPRequired", err)
	}
}

func TestRequireParentRejectsChildSession(t *testing.T) {
	db := openTestDB(t)
	ctx := t.Context()
	childID := seedChild(t, db, "child1")
	now := time.Unix(1_700_000_000, 0).UTC()

	token, err := auth.CreateSession(ctx, db, store.SessionChild, childID, now)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	sess, err := auth.LookupSession(ctx, db, token, store.SessionChild, now)
	if err != nil {
		t.Fatalf("LookupSession() error = %v", err)
	}

	if err := auth.RequireParent(sess, now); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("RequireParent() on a child session: error = %v, want ErrNotFound", err)
	}
}

// TestParentCookieCarriesHostPrefixAndStrictSameSite is encre-qpx.6's M3
// regression test: the parent cookie must carry the __Host- prefix and
// SameSite=Strict; the child cookie is unaffected (still Lax, no prefix).
func TestParentCookieCarriesHostPrefixAndStrictSameSite(t *testing.T) {
	if !strings.HasPrefix(auth.CookieParent, "__Host-") {
		t.Errorf("CookieParent = %q, want a __Host- prefix", auth.CookieParent)
	}

	rec := httptest.NewRecorder()
	auth.SetSessionCookie(rec, auth.CookieParent, "tok-value", auth.ParentSessionTTL)
	c := rec.Result().Cookies()[0]
	if c.SameSite != http.SameSiteStrictMode {
		t.Errorf("parent cookie SameSite = %v, want Strict", c.SameSite)
	}
	if !c.Secure {
		t.Error("parent cookie Secure = false, want true (required for __Host-)")
	}
	if c.Path != "/" {
		t.Errorf("parent cookie Path = %q, want \"/\" (required for __Host-)", c.Path)
	}

	rec2 := httptest.NewRecorder()
	auth.SetSessionCookie(rec2, auth.CookieChild, "tok-value", auth.ChildSessionTTL)
	c2 := rec2.Result().Cookies()[0]
	if c2.SameSite != http.SameSiteLaxMode {
		t.Errorf("child cookie SameSite = %v, want Lax (unchanged)", c2.SameSite)
	}
}

// TestLogoutDeletesSessionAndClearsCookie is encre-qpx.8's regression test
// (L7): Logout must both remove the session server-side and clear the
// cookie — a handler that only cleared the cookie would leave a captured
// token valid for days.
func TestLogoutDeletesSessionAndClearsCookie(t *testing.T) {
	db := openTestDB(t)
	ctx := t.Context()
	parentID := seedParent(t, db, "parent1")
	now := time.Unix(1_700_000_000, 0).UTC()

	token, err := auth.CreateSession(ctx, db, store.SessionParent, parentID, now)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/parent/logout", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieParent, Value: token})
	rec := httptest.NewRecorder()

	if err := auth.Logout(ctx, db, rec, req, auth.CookieParent); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}

	if _, err := auth.LookupSession(ctx, db, token, store.SessionParent, now); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("LookupSession() after Logout(): error = %v, want ErrNotFound (session deleted)", err)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].MaxAge >= 0 {
		t.Errorf("Logout() cookies = %+v, want exactly one cookie with a negative MaxAge", cookies)
	}
}

// TestLogoutWithNoCookieIsIdempotent checks that Logout does not error when
// there is no cookie to read at all — mirroring DeleteSession's own
// idempotence.
func TestLogoutWithNoCookieIsIdempotent(t *testing.T) {
	db := openTestDB(t)
	req := httptest.NewRequest(http.MethodPost, "/parent/logout", nil)
	rec := httptest.NewRecorder()

	if err := auth.Logout(t.Context(), db, rec, req, auth.CookieParent); err != nil {
		t.Errorf("Logout() with no cookie: error = %v, want nil", err)
	}
}
