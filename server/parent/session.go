package parent

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/oioio-space/encre/server/auth"
	"github.com/oioio-space/encre/server/store"
)

// ctxKey namespaces this package's context values so they cannot collide
// with a key some other package sets on the same [context.Context].
type ctxKey int

const (
	ctxSession ctxKey = iota
	ctxCSRFToken
)

// requireParentSession wraps next so it only ever runs for a request
// carrying a session [auth.RequireParent] accepts — a valid
// [github.com/oioio-space/encre/server/store.SessionParent] session with a
// fresh TOTP check (encre-qpx.6, M5). A missing cookie, an expired session,
// a child session presented under the parent cookie name, and a parent
// session whose TOTP freshness has lapsed are all rejected the same way —
// redirected to the login page — never distinguished in the response: see
// [auth.LookupSession]'s doc comment for why a child token must never
// resolve a parent page, and [auth.RequireParent]'s for why freshness is
// checked on every route, not only the ones this package used to single out
// as sensitive.
//
// On success it stores the session and this request's CSRF token
// (csrfSigner.Token of the session's raw cookie value) in the request
// context, retrievable with [sessionFromContext] and [csrfTokenFromContext].
func (s *Server) requireParentSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(auth.CookieParent)
		if err != nil {
			s.redirectToLogin(w, r)
			return
		}
		sess, err := auth.LookupSession(r.Context(), s.db, cookie.Value, store.SessionParent, s.now())
		if err != nil {
			s.redirectToLogin(w, r)
			return
		}
		if err := auth.RequireParent(sess, s.now()); err != nil {
			s.redirectToLogin(w, r)
			return
		}

		ctx := context.WithValue(r.Context(), ctxSession, sess)
		ctx = context.WithValue(ctx, ctxCSRFToken, s.csrf.Token(cookie.Value))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// maxRequestBytes bounds any mutating request's whole body, [parseAnyForm]'s
// one enforcement point for every route in this package: generous for a
// spoken word or short sentence's recording (the largest body this package
// accepts, from [handleItemAudioPost]) at any reasonable bitrate, small
// enough that one upload cannot exhaust the server's disk on its own.
const maxRequestBytes = 8 << 20 // 8 MiB

// maxMultipartMemory bounds how much of a multipart request
// [parseAnyForm] buffers in memory before spilling the rest to a temp file;
// it does not bound the request's total size, [maxRequestBytes] does that.
const maxMultipartMemory = 1 << 20 // 1 MiB

// parseAnyForm parses r's body whether it is
// application/x-www-form-urlencoded ([http.Request.ParseForm]) or
// multipart/form-data ([http.Request.ParseMultipartForm], which
// [handleItemAudioPost]'s upload sends) — [requireCSRF] needs
// [http.Request.PostFormValue] populated either way to find
// [csrfFormField], and ParseForm alone never reads a multipart body. It
// wraps r.Body in [http.MaxBytesReader] first, capping every mutating
// route in this package at [maxRequestBytes] in one place rather than each
// handler enforcing its own limit — or, as a plain [http.Request.ParseForm]
// call with no limit at all would, none.
func parseAnyForm(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		// #nosec G120 -- bounded twice over: maxMultipartMemory itself caps
		// what ParseMultipartForm buffers in memory, and the MaxBytesReader
		// wrap just above already caps the whole body at maxRequestBytes
		// before a single byte of it reaches this call.
		return r.ParseMultipartForm(maxMultipartMemory)
	}
	return r.ParseForm()
}

// requireCSRF wraps next so a mutating request is rejected unless
// [requireSameOrigin] holds and its csrfFormField field matches the token
// [requireParentSession] bound to the caller's own session. It must run
// after requireParentSession: it reads the session requireParentSession
// stored in the context.
func (s *Server) requireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !requireSameOrigin(r) {
			s.renderError(w, r, http.StatusForbidden, "origine non reconnue")
			return
		}

		if _, ok := sessionFromContext(r.Context()); !ok {
			s.renderError(w, r, http.StatusForbidden, "session invalide")
			return
		}
		cookie, err := r.Cookie(auth.CookieParent)
		if err != nil {
			s.renderError(w, r, http.StatusForbidden, "session invalide")
			return
		}

		if err := parseAnyForm(w, r); err != nil {
			s.renderError(w, r, http.StatusBadRequest, "formulaire invalide")
			return
		}
		if !s.csrf.Valid(cookie.Value, r.PostFormValue(csrfFormField)) {
			s.renderError(w, r, http.StatusForbidden, "jeton de sécurité invalide ou expiré")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// sessionFromContext returns the session [Server.requireParentSession]
// stored on ctx.
func sessionFromContext(ctx context.Context) (*store.Session, bool) {
	sess, ok := ctx.Value(ctxSession).(*store.Session)
	return sess, ok
}

// csrfTokenFromContext returns the CSRF token [Server.requireParentSession]
// bound to the current request's session, for a template to embed as
// csrfFormField.
func csrfTokenFromContext(ctx context.Context) string {
	token, _ := ctx.Value(ctxCSRFToken).(string)
	return token
}

// redirectToLogin sends the browser to the login page. It is used both when
// no session is present and when one is present but invalid or expired, so
// the two cases are indistinguishable from the response alone.
func (s *Server) redirectToLogin(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/parent/login", http.StatusSeeOther)
}

// renderError writes a generic, French, user-facing message at the given
// status and logs the real cause (if any) server-side — never the reverse.
// No handler in this package writes an internal error's text into a
// response.
func (s *Server) renderError(w http.ResponseWriter, r *http.Request, status int, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := s.tmpl.ExecuteTemplate(w, "error.html", map[string]any{"Message": message}); err != nil {
		slog.ErrorContext(r.Context(), "rendering error page", "error", err)
	}
}

// logInternalError logs err server-side and renders a generic 500: no
// database error, no wrapped detail, ever reaches the parent's browser.
func (s *Server) logInternalError(w http.ResponseWriter, r *http.Request, context string, err error) {
	slog.ErrorContext(r.Context(), context, "error", err)
	if errors.Is(err, store.ErrNotFound) {
		s.renderError(w, r, http.StatusNotFound, "introuvable")
		return
	}
	s.renderError(w, r, http.StatusInternalServerError, "une erreur interne est survenue")
}

// slogRenderError logs a template-execution failure server-side. Templates
// only fail to execute for a programming error (a bad field reference, a
// type mismatch) — never for anything a parent's input could trigger, since
// every value is escaped, not rejected — so there is nothing more specific
// to tell the client; whatever the partial response already wrote stands.
func slogRenderError(r *http.Request, template string, err error) {
	slog.ErrorContext(r.Context(), "rendering template", "template", template, "error", err)
}

// now returns the current time, through [Server.clock] so tests can move it.
func (s *Server) now() time.Time {
	if s.clock != nil {
		return s.clock()
	}
	return time.Now()
}
