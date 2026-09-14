package parent

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"slices"
)

// csrfFormField is the hidden input name every state-changing form on the
// panel must carry.
const csrfFormField = "csrf_token"

// csrfKeyBytes is the size of [csrfSigner.key]: 256 bits, an HMAC key never
// needs more.
const csrfKeyBytes = 32

// csrfSigner derives a synchronizer CSRF token from a parent session's raw
// cookie token, per ENCRE_04 §7. It never persists a token of its own: the
// token a page embeds and the token a submitted form is checked against are
// both recomputed from key and the session token on the fly, so there is
// nothing to store, expire or garbage-collect.
//
// Deriving from the raw session token — not the session's row ID or subject
// ID — is what ties the CSRF token to the exact cookie the browser holds: an
// attacker who cannot read that cookie (it is HttpOnly, and same-origin
// policy keeps a foreign page from fetching this one) cannot compute the
// token either, even knowing key would not be enough without it.
//
// A csrfSigner's zero value is not usable; build one with [newCSRFSigner].
type csrfSigner struct {
	key [csrfKeyBytes]byte
}

// newCSRFSigner returns a csrfSigner keyed from [crypto/rand]. Every
// [Server] gets its own key, generated once at startup: a token computed
// against one process's key never validates against another's, which is
// fine — a token only ever needs to outlive the request that issued the
// page containing it, well within one process's lifetime.
func newCSRFSigner() (*csrfSigner, error) {
	var c csrfSigner
	if _, err := rand.Read(c.key[:]); err != nil {
		return nil, fmt.Errorf("generating csrf key: %w", err)
	}
	return &c, nil
}

// Token returns the CSRF token bound to sessionToken.
func (c *csrfSigner) Token(sessionToken string) string {
	mac := hmac.New(sha256.New, c.key[:])
	mac.Write([]byte(sessionToken))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// Valid reports whether submitted is the CSRF token bound to sessionToken,
// comparing in constant time so a byte-by-byte timing side channel cannot
// help an attacker guess it.
func (c *csrfSigner) Valid(sessionToken, submitted string) bool {
	if submitted == "" {
		return false
	}
	want := c.Token(sessionToken)
	return subtle.ConstantTimeCompare([]byte(want), []byte(submitted)) == 1
}

// mutatingMethods are the HTTP methods [requireSameOrigin] and the CSRF
// check apply to: anything that is not a safe, read-only request per RFC
// 9110 §9.2.1.
var mutatingMethods = []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}

// requireSameOrigin rejects a mutating request unless Sec-Fetch-Site or
// Origin says it came from this server, per ENCRE_04 §7. Sec-Fetch-Site is
// checked first because it cannot be spoofed by anything short of a browser
// bug; Origin is the fallback for the older browsers that do not send it.
// If a request carries neither header at all, it is rejected — not allowed
// through as same-origin by default — because the alternative is trusting a
// client that could simply omit both to bypass the check entirely.
func requireSameOrigin(r *http.Request) bool {
	if !slices.Contains(mutatingMethods, r.Method) {
		return true
	}

	switch site := r.Header.Get("Sec-Fetch-Site"); site {
	case "same-origin", "none":
		return true
	case "cross-site", "same-site":
		return false
	}

	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	return origin == requestOrigin(r)
}

// requestOrigin reconstructs the scheme://host origin r was received on, to
// compare against a request's Origin header.
func requestOrigin(r *http.Request) string {
	scheme := "https"
	if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") != "https" {
		scheme = "http"
	}
	return scheme + "://" + r.Host
}
