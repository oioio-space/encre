package parent

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/oioio-space/encre/server/auth"
	"github.com/oioio-space/encre/server/store"
)

func TestServerNowUsesClockWhenSet(t *testing.T) {
	fixed := time.Date(2020, time.January, 2, 3, 4, 5, 0, time.UTC)
	s := &Server{clock: func() time.Time { return fixed }}
	if got := s.now(); !got.Equal(fixed) {
		t.Errorf("now() with a clock set: got %v, want %v", got, fixed)
	}

	s2 := &Server{}
	if got := s2.now(); got.IsZero() {
		t.Errorf("now() with no clock set: got zero time, want time.Now()")
	}
}

// TestRequireParentSessionRejectsLapsedTOTPFreshness is encre-qpx.6's
// end-to-end regression test (M5), exercised through the exact middleware
// every parent route runs: a parent session is valid for
// [auth.ParentSessionTTL] (7 days), but requireParentSession must reject it
// once TOTP freshness ([auth.TOTPFreshDuration], 1 hour) has lapsed — long
// before the session itself expires — because it now goes through
// [auth.RequireParent] rather than [auth.LookupSession] alone.
func TestRequireParentSessionRejectsLapsedTOTPFreshness(t *testing.T) {
	db, err := store.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	pep, err := auth.NewPepper("test", bytes.Repeat([]byte("k"), 32))
	if err != nil {
		t.Fatalf("NewPepper() error = %v", err)
	}
	if err := db.CreateParent(t.Context(), &store.Parent{ID: "parent1", Email: "p@example.test", PassHash: []byte("x")}); err != nil {
		t.Fatalf("CreateParent() error = %v", err)
	}

	now := time.Unix(1_700_000_000, 0).UTC()
	token, err := auth.CreateSession(t.Context(), db, store.SessionParent, "parent1", now)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	sess, err := auth.LookupSession(t.Context(), db, token, store.SessionParent, now)
	if err != nil {
		t.Fatalf("LookupSession() error = %v", err)
	}
	if err := auth.VerifyParentTOTPForSession(t.Context(), db, sess, "", now, pep); err == nil {
		t.Fatal("VerifyParentTOTPForSession() with an empty code unexpectedly succeeded")
	}
	// Stamp freshness by hand (this server has no enrolled TOTP secret to
	// verify a real code against, and that is not what this test is about).
	if _, err := db.DB().ExecContext(t.Context(),
		`UPDATE sessions SET totp_ok_until = ? WHERE token = ?`, now.Add(auth.TOTPFreshDuration).Unix(), sess.Token); err != nil {
		t.Fatalf("stamping totp freshness: %v", err)
	}

	tmpl, err := parseTemplates()
	if err != nil {
		t.Fatalf("parseTemplates() error = %v", err)
	}
	csrf, err := newCSRFSigner()
	if err != nil {
		t.Fatalf("newCSRFSigner() error = %v", err)
	}
	later := now.Add(auth.TOTPFreshDuration + time.Second)
	s := &Server{db: db, pep: pep, tmpl: tmpl, csrf: csrf, clock: func() time.Time { return later }}

	var reached bool
	handler := s.requireParentSession(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true }))

	req := httptest.NewRequest(http.MethodGet, "/parent/children", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieParent, Value: token})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if reached {
		t.Error("requireParentSession let the request through with lapsed TOTP freshness, want it rejected")
	}
	if rec.Code != http.StatusSeeOther {
		t.Errorf("requireParentSession with lapsed TOTP freshness: status = %d, want %d (redirect to login)", rec.Code, http.StatusSeeOther)
	}
}
