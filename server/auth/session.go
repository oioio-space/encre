package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/oioio-space/encre/server/store"
)

const (
	// ChildSessionTTL is how long a child session lasts (ENCRE_04 §7).
	ChildSessionTTL = 24 * time.Hour
	// ParentSessionTTL is how long a parent session lasts (ENCRE_04 §7).
	ParentSessionTTL = 7 * 24 * time.Hour

	// CookieChild is the cookie name a child session is set and read under.
	CookieChild = "encre_child"
	// CookieParent is the cookie name a parent session is set and read under.
	CookieParent = "encre_parent"

	// sessionTokenBytes is how much entropy [NewSessionToken] reads from
	// crypto/rand: 256 bits, well past any brute-force concern.
	sessionTokenBytes = 32
)

// NewSessionToken returns a fresh session token: sessionTokenBytes of
// [crypto/rand], base64url-encoded. It is never math/rand — a predictable
// session token defeats the point of one.
func NewSessionToken() (string, error) {
	b := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating session token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// hashSessionToken returns the SHA-256 digest of token, hex-encoded. It is
// what [server/store.Session.Token] holds: a database backup leaks these
// hashes, never a value an attacker can present as a cookie.
func hashSessionToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// CreateSession creates a session for subjectID, valid from now for
// [ChildSessionTTL] or [ParentSessionTTL] depending on kind, and returns the
// raw token to set as a cookie. The store never sees that raw token — only
// its hash — so callers must not log it either.
func CreateSession(ctx context.Context, db *store.Store, kind store.SessionKind, subjectID string, now time.Time) (string, error) {
	token, err := NewSessionToken()
	if err != nil {
		return "", err
	}

	ttl := ChildSessionTTL
	if kind == store.SessionParent {
		ttl = ParentSessionTTL
	}
	sess := &store.Session{
		Token:     hashSessionToken(token),
		Kind:      kind,
		SubjectID: subjectID,
		ExpiresAt: now.Add(ttl),
	}
	if err := db.CreateSession(ctx, sess); err != nil {
		return "", err
	}
	return token, nil
}

// LookupSession resolves a raw cookie token to its session, requiring it to
// be a want-kind session. It returns [store.ErrNotFound] if no session
// hashes to token, or [ErrSessionExpired] if it did once but now has passed.
// now is a parameter rather than [time.Now] so tests can move it without
// sleeping.
//
// A session of the wrong kind is reported the same way as an unknown token —
// [store.ErrNotFound], not a distinct error — deliberately: without this, a
// child session token replayed under the parent cookie name would resolve to
// a real (child) session and let the caller decide whether that matters,
// which is exactly the privilege escalation this check exists to close.
// Callers must never skip want by using a lower-level store query directly.
func LookupSession(ctx context.Context, db *store.Store, token string, want store.SessionKind, now time.Time) (*store.Session, error) {
	sess, err := db.Session(ctx, hashSessionToken(token))
	if err != nil {
		return nil, err
	}
	if sess.Kind != want {
		return nil, store.ErrNotFound
	}
	if now.After(sess.ExpiresAt) {
		return nil, ErrSessionExpired
	}
	return sess, nil
}

// DeleteSession removes the session behind a raw cookie token. Deleting an
// unknown or already-removed token is not an error.
func DeleteSession(ctx context.Context, db *store.Store, token string) error {
	return db.DeleteSession(ctx, hashSessionToken(token))
}

// TOTPFresh reports whether sess's TOTP freshness window
// ([server/store.Session.TOTPOKUntil], set by a recent [VerifyParentTOTP])
// still covers now. ENCRE_04 §7 requires this check before a parent session
// is allowed to perform a sensitive action. A non-parent session is never
// fresh, regardless of TOTPOKUntil: freshness is a parent-only concept, and
// [server/store.Session.Kind] is what makes that true even if a caller
// reaches this function with a session [LookupSession] should have already
// rejected.
func TOTPFresh(sess *store.Session, now time.Time) bool {
	return sess.Kind == store.SessionParent && sess.TOTPOKUntil.After(now)
}

// RequireTOTPFresh returns [ErrTOTPRequired] unless [TOTPFresh] holds.
func RequireTOTPFresh(sess *store.Session, now time.Time) error {
	if !TOTPFresh(sess, now) {
		return ErrTOTPRequired
	}
	return nil
}

// SetSessionCookie writes name=token as ENCRE_04 §7's session cookie:
// HttpOnly, Secure, SameSite=Lax, path "/", expiring after ttl.
func SetSessionCookie(w http.ResponseWriter, name, token string, ttl time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    token,
		Path:     "/",
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearSessionCookie tells the browser to drop name immediately, for logout.
func ClearSessionCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

// SessionFromCookie reads name's cookie from r and resolves it with
// [LookupSession], deriving the required session kind from name itself
// ([CookieChild] or [CookieParent]) so a session token replayed under the
// wrong cookie name never resolves. It returns [http.ErrNoCookie] if the
// cookie is absent, or an error if name is neither [CookieChild] nor
// [CookieParent].
func SessionFromCookie(r *http.Request, db *store.Store, name string, now time.Time) (*store.Session, error) {
	want, err := sessionKindForCookie(name)
	if err != nil {
		return nil, err
	}
	c, err := r.Cookie(name)
	if err != nil {
		return nil, err
	}
	sess, err := LookupSession(r.Context(), db, c.Value, want, now)
	if err != nil {
		return nil, err
	}
	return sess, nil
}

// sessionKindForCookie maps a cookie name to the session kind it must carry.
func sessionKindForCookie(name string) (store.SessionKind, error) {
	switch name {
	case CookieChild:
		return store.SessionChild, nil
	case CookieParent:
		return store.SessionParent, nil
	default:
		return 0, fmt.Errorf("auth: unknown session cookie name %q", name)
	}
}
