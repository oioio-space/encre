// Package parent implements ENCRE's parent panel (ENCRE_04 §11, ENCRE_01
// §17): the connexion, enfants and réglages pages a parent uses from their
// phone in the evening.
//
// It renders server-side with [html/template] and [htmx], never
// [text/template] — every value a parent typed (a child's pseudo, in
// particular) is escaped before it reaches the page. htmx itself is
// vendored under server/parent/static and served with [embed], so the
// built server stays a single binary with no external JavaScript fetch.
//
// [Server] wraps a [github.com/oioio-space/encre/server/store.Store] and
// implements [net/http.Handler]:
//
//	db, err := store.Open("file:/var/lib/encre/encre.db")
//	if err != nil {
//		log.Fatal(err)
//	}
//	pep, err := auth.LoadPepper()
//	if err != nil {
//		log.Fatal(err)
//	}
//	srv, err := parent.NewServer(db, pep)
//	if err != nil {
//		log.Fatal(err)
//	}
//	http.ListenAndServe(":8443", srv)
//
// Every route under /parent/ except the login page itself requires a
// session [github.com/oioio-space/encre/server/auth.RequireParent] accepts:
// a [github.com/oioio-space/encre/server/store.SessionParent] session — never
// a child session, even one replayed under the parent cookie name — with a
// TOTP check still fresh right now, re-checked on every request rather than
// only at login, per ENCRE_04 §7's "session + TOTP frais" covering the whole
// route block. Every mutating route additionally requires a valid CSRF
// token bound to that session (see csrf.go) and an Origin or Sec-Fetch-Site
// header that names this server, rejecting the request by default when both
// are missing. [Server] never writes an internal error's text to a
// response; see renderError.
package parent
