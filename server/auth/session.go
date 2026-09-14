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
	// CookieParent is the cookie name a parent session is set and read
	// under, carrying the "__Host-" prefix (encre-qpx.6, RFC 6265bis §4.1.3):
	// a browser refuses to set or honor a "__Host-"-prefixed cookie unless it
	// was sent with Secure, Path=/ and no Domain attribute, which rules out
	// exactly the fixation this prefix exists to close — a compromised
	// sibling subdomain can no longer plant a same-named cookie with
	// Domain=example.com that [net/http.Request.Cookie] would then read in
	// place of (or ahead of) the real one.
	CookieParent = "__Host-encre_parent"

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

// RequireParent is the single gate every parent-panel route must call
// (encre-qpx.6, M5): it returns nil only for a session that is both a
// [server/store.SessionParent] session and TOTP-fresh right now
// ([TOTPFresh]). ENCRE_04 §7 marks the entire parent route block "session +
// TOTP frais" — not just the actions this codebase used to treat as
// individually sensitive — because a parent session's TOTP freshness window
// ([TOTPFreshDuration]) is far shorter than the session itself
// ([ParentSessionTTL]): a password alone, reused from some other leak, must
// never carry a read past that window on its own, all the way out to
// /export.
//
// It returns [store.ErrNotFound] for a non-[server/store.SessionParent]
// session — the same sentinel [LookupSession] already uses for a wrong-kind
// session, so a caller that checks this and nothing else still gets the
// privilege-escalation protection [LookupSession]'s doc comment describes —
// and [ErrTOTPRequired] for a parent session whose freshness has lapsed or
// was never set. Callers must treat both identically in what they show the
// caller (redirect to the login page, in server/parent's case): the whole
// point of a single gate is that no route can accidentally special-case one
// of the two checks away.
func RequireParent(sess *store.Session, now time.Time) error {
	if sess.Kind != store.SessionParent {
		return store.ErrNotFound
	}
	return RequireTOTPFresh(sess, now)
}

// SetSessionCookie writes name=token as ENCRE_04 §7's session cookie:
// HttpOnly, Secure, path "/", expiring after ttl. The parent cookie
// ([CookieParent]) is SameSite=Strict; every other cookie this package
// issues ([CookieChild]) is SameSite=Lax (encre-qpx.6, M3): SameSite alone
// is scoped to the whole site, not this origin, so a compromised sibling
// subdomain could still ride a Lax cookie into a cross-site-adjacent
// request; Strict on the parent cookie closes that for the side that can
// reach /export and a child's settings, while Lax stays on the child cookie
// because a top-level navigation into the game (SameSite=Strict's one real
// behavioral difference from Lax) is an expected way to reach it.
// #nosec G124 -- SameSite is always set, to sameSiteForCookie's result,
// which is always http.SameSiteStrictMode or http.SameSiteLaxMode, never
// the zero value; gosec's static check does not follow the helper call.
func SetSessionCookie(w http.ResponseWriter, name, token string, ttl time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    token,
		Path:     "/",
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: sameSiteForCookie(name),
	})
}

// ClearSessionCookie tells the browser to drop name immediately.
//
// This is a low-level primitive: it has no effect on the session
// server-side. A logout handler that calls only this leaves the session
// token — which an attacker may already have captured — valid until it
// expires on its own, up to [ParentSessionTTL] later. [Logout] is what
// actually ends a session; call this directly only when there is deliberately
// no server-side session left to delete (for example, after
// [DeleteSession] has already been called, or when clearing a cookie for a
// token that never resolved to one).
// #nosec G124 -- see SetSessionCookie's #nosec comment; the same
// sameSiteForCookie call is used here.
func ClearSessionCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: sameSiteForCookie(name),
	})
}

// Logout deletes the session behind name's cookie on r (if any) and clears
// the cookie, in that order: the two effects — server-side revocation and
// telling the browser to forget the cookie — belong together (encre-qpx.8,
// L7), because a handler that calls only [ClearSessionCookie] leaves the
// captured-or-not token valid server-side for as long as it would otherwise
// have lived. A missing or already-invalid cookie is not an error: logout is
// idempotent, the same as [DeleteSession] it calls.
func Logout(ctx context.Context, db *store.Store, w http.ResponseWriter, r *http.Request, name string) error {
	if cookie, err := r.Cookie(name); err == nil {
		if err := DeleteSession(ctx, db, cookie.Value); err != nil {
			return err
		}
	}
	ClearSessionCookie(w, name)
	return nil
}

// sameSiteForCookie returns the [http.SameSite] mode [SetSessionCookie] and
// [ClearSessionCookie] use for name — see [SetSessionCookie]'s doc comment
// for why the parent cookie differs from every other one.
func sameSiteForCookie(name string) http.SameSite {
	if name == CookieParent {
		return http.SameSiteStrictMode
	}
	return http.SameSiteLaxMode
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
