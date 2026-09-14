// Package auth implements ENCRE's authentication: parent passwords, TOTP
// second factor, sessions, child login and the login rate limits ENCRE_04 §7
// specifies.
//
// It builds on [github.com/oioio-space/encre/server/store] rather than
// replacing any of it: this package hashes, verifies and rate-limits, and
// [server/store] persists the results. A typical parent login looks like:
//
//	id, err := auth.LoginParent(ctx, db, email, password)
//	if err != nil {
//		http.Error(w, "identifiants invalides", http.StatusUnauthorized)
//		return
//	}
//	token, err := auth.CreateSession(ctx, db, store.SessionParent, id, time.Now())
//	if err != nil {
//		http.Error(w, "erreur serveur", http.StatusInternalServerError)
//		return
//	}
//	auth.SetSessionCookie(w, auth.CookieParent, token, auth.ParentSessionTTL)
//
// Reading a session back always goes through [SessionFromCookie] or
// [LookupSession] with an explicit session kind — never through
// [server/store.Store.Session] directly, which does not know or care which
// cookie a token arrived under:
//
//	sess, err := auth.SessionFromCookie(r, db, auth.CookieParent, time.Now())
//	if err != nil {
//		http.Error(w, "session invalide", http.StatusUnauthorized)
//		return
//	}
//	// sess.Kind is guaranteed to be store.SessionParent here: a child's
//	// token, even copied verbatim into the encre_parent cookie, resolves to
//	// store.ErrNotFound instead.
//
// Every function in this package that compares a secret — a password hash, a
// pattern hash, a session token, a TOTP code — does so in constant time, and
// no function ever returns or logs the secret it compared against. Errors
// returned to a caller outside this package never carry a database error's
// text; see [ErrInvalidCredentials] and [ErrSessionExpired].
package auth
