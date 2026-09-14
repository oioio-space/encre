// Package auth implements ENCRE's authentication: parent passwords, TOTP
// second factor, sessions, child login and the login rate limits ENCRE_04 §7
// specifies.
//
// It builds on [github.com/oioio-space/encre/server/store] rather than
// replacing any of it: this package hashes, verifies and rate-limits, and
// [server/store] persists the results. Every hash and every stored TOTP
// secret goes through a [Pepper] — an out-of-database key (ENCRE_04 §12) —
// before [server/store] ever sees it; see [Pepper]'s doc comment. A typical
// parent login, including the TOTP check every parent-panel route requires,
// looks like:
//
//	pep, err := auth.LoadPepper()
//	if err != nil {
//		log.Fatal(err)
//	}
//	id, err := auth.LoginParent(ctx, db, email, password, pep)
//	if err != nil {
//		http.Error(w, "identifiants invalides", http.StatusUnauthorized)
//		return
//	}
//	token, err := auth.CreateSession(ctx, db, store.SessionParent, id, time.Now())
//	if err != nil {
//		http.Error(w, "erreur serveur", http.StatusInternalServerError)
//		return
//	}
//	if err := auth.VerifyParentTOTPForSession(ctx, db, sess, totpCode, time.Now(), pep); err != nil {
//		http.Error(w, "code invalide", http.StatusUnauthorized)
//		return
//	}
//	auth.SetSessionCookie(w, auth.CookieParent, token, auth.ParentSessionTTL)
//
// Reading a session back always goes through [SessionFromCookie] or
// [LookupSession] with an explicit session kind — never through
// [server/store.Store.Session] directly, which does not know or care which
// cookie a token arrived under — and every route this session gates, not
// only the ones that mutate something, must pass it to [RequireParent]
// before proceeding — this is the example a caller actually copies:
//
//	sess, err := auth.SessionFromCookie(r, db, auth.CookieParent, time.Now())
//	if err != nil {
//		http.Error(w, "session invalide", http.StatusUnauthorized)
//		return
//	}
//	if err := auth.RequireParent(sess, time.Now()); err != nil {
//		// Also reached by a session whose TOTP freshness lapsed: ENCRE_04
//		// §7 marks the whole parent route block "session + TOTP frais",
//		// not just the actions once treated as individually sensitive.
//		http.Error(w, "session invalide", http.StatusUnauthorized)
//		return
//	}
//	// sess.Kind is guaranteed to be store.SessionParent here: a child's
//	// token, even copied verbatim into the encre_parent cookie, resolves to
//	// store.ErrNotFound instead.
//
// [HashPassword], [HashPattern], [VerifyPassword] and [VerifyPattern]
// compare in constant time (via [Pepper.VerifyHash]'s HMAC equality check),
// and no function in this package ever returns or logs the plaintext secret
// it compared against. Two things that claim is deliberately narrower than
// it sounds: a session token is looked up by an equality match in SQL on
// its SHA-256 hash, not compared in application code at all, constant-time
// or otherwise — harmless, since the hash itself denies an attacker
// anything to time against, but not the same property as the hash
// comparisons above; and errors returned to a caller outside this package
// are not scrubbed of a database driver's own error text (see
// [server/store]'s errors, which this package wraps with %w rather than
// discarding) — [ErrInvalidCredentials], [ErrSessionExpired] and this
// package's other sentinels exist so a caller CAN show a generic message
// without inspecting the text, but nothing here strips that text from an
// error a caller chooses to log or wrap further; see
// [github.com/oioio-space/encre/server/parent]'s renderError and
// logInternalError, or [github.com/oioio-space/encre/server/api]'s
// writeStoreError, for the actual pattern that keeps it out of a response.
package auth
