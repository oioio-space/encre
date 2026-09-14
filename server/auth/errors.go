package auth

import "errors"

// ErrInvalidCredentials is returned by [LoginParent] and [LoginChild] for any
// reason a login can fail: unknown email, unknown pseudo, wrong password,
// wrong pattern. Callers must show the same message for all of them — see
// ENCRE_04 §7's "no enumeration oracle" requirement — so this package
// collapses every such reason into one sentinel before it reaches them.
var ErrInvalidCredentials = errors.New("auth: invalid credentials")

// ErrSessionExpired is returned by [LookupSession] for a session whose
// expiry has passed. The row is left for
// [server/store.Store.PurgeExpiredSessions] rather than deleted inline, so a
// read-only lookup never needs a write. Nothing in this codebase calls that
// method on a schedule yet (encre-qpx.8, L6) — a deployment's startup or a
// periodic job must, or expired rows accumulate forever; they are otherwise
// harmless, since [LookupSession] already rejects them by expiry alone.
var ErrSessionExpired = errors.New("auth: session expired")

// ErrTOTPRequired is returned by actions ENCRE_04 §7 marks sensitive when the
// parent session's TOTP freshness ([server/store.Session.TOTPOKUntil]) has
// lapsed or was never set.
var ErrTOTPRequired = errors.New("auth: fresh totp code required")
