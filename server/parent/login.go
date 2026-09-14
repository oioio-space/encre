package parent

import (
	"errors"
	"net/http"

	"github.com/oioio-space/encre/server/auth"
	"github.com/oioio-space/encre/server/store"
)

// loginPageData is what the connexion template renders.
type loginPageData struct {
	Error string
}

// handleLoginGet renders the login form. A parent already holding a valid
// session is sent straight to the enfants page instead of being shown the
// form again.
func (s *Server) handleLoginGet(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(auth.CookieParent); err == nil {
		if _, err := auth.LookupSession(r.Context(), s.db, cookie.Value, store.SessionParent, s.now()); err == nil {
			http.Redirect(w, r, "/parent/children", http.StatusSeeOther)
			return
		}
	}
	s.renderLogin(w, r, "")
}

// handleLoginPost verifies email, password and a TOTP code together —
// ENCRE_01 §17: "TOTP obligatoire pour le panneau" — and, on success, opens
// a fresh parent session with its TOTP freshness window already set, so the
// enfants and réglages mutations that follow immediately do not have to ask
// again within [auth.TOTPFreshDuration].
//
// Every rejection reason (unknown email, wrong password, wrong or missing
// TOTP code, rate limit) renders the same generic message: ENCRE_04 §7's
// no-enumeration-oracle requirement applies to the whole login step, not
// just the password check inside it.
func (s *Server) handleLoginPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.renderLogin(w, r, "formulaire invalide")
		return
	}

	if !s.loginLimiter.Allow(auth.ClientIP(r, s.trustedProxies)) {
		s.renderLogin(w, r, "trop de tentatives, réessayez dans une minute")
		return
	}

	email := r.PostFormValue("email")
	password := r.PostFormValue("password")
	totpCode := r.PostFormValue("totp")

	now := s.now()
	parentID, err := auth.LoginParent(r.Context(), s.db, email, password, s.pep)
	if err != nil {
		s.rejectLogin(w, r, err)
		return
	}

	// The session is created before the TOTP code is checked, then deleted
	// again on failure: [auth.VerifyParentTOTPForSession] is what both
	// validates the code and stamps TOTPOKUntil in one atomic step (it is
	// also what every later sensitive action re-verifies against), and it
	// needs an existing session to stamp. Calling [auth.VerifyParentTOTP]
	// separately first would mark the code's step used, making this
	// call — the one that actually sets freshness — fail as a replay.
	token, err := auth.CreateSession(r.Context(), s.db, store.SessionParent, parentID, now)
	if err != nil {
		s.logInternalError(w, r, "creating parent session", err)
		return
	}
	sess, err := auth.LookupSession(r.Context(), s.db, token, store.SessionParent, now)
	if err != nil {
		s.logInternalError(w, r, "looking up freshly created session", err)
		return
	}
	if err := auth.VerifyParentTOTPForSession(r.Context(), s.db, sess, totpCode, now, s.pep); err != nil {
		if delErr := auth.DeleteSession(r.Context(), s.db, token); delErr != nil {
			slogRenderError(r, "cleaning up rejected login session", delErr)
		}
		s.rejectLogin(w, r, err)
		return
	}

	auth.SetSessionCookie(w, auth.CookieParent, token, auth.ParentSessionTTL)
	http.Redirect(w, r, "/parent/children", http.StatusSeeOther)
}

// rejectLogin renders the login page's single, non-enumerating failure
// message for any [auth.LoginParent] or [auth.VerifyParentTOTP] error,
// logging anything that is not one of their expected sentinels.
func (s *Server) rejectLogin(w http.ResponseWriter, r *http.Request, err error) {
	if !errors.Is(err, auth.ErrInvalidCredentials) && !errors.Is(err, auth.ErrInvalidTOTPCode) {
		slogRenderError(r, "login", err)
	}
	s.renderLogin(w, r, "identifiants ou code invalides")
}

// handleLogout ends the session server-side and clears the cookie (encre-qpx.8, L7).
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if err := auth.Logout(r.Context(), s.db, w, r, auth.CookieParent); err != nil {
		slogRenderError(r, "logout", err)
	}
	http.Redirect(w, r, "/parent/login", http.StatusSeeOther)
}

// renderLogin writes the login page, with errMessage shown above the form
// if non-empty.
func (s *Server) renderLogin(w http.ResponseWriter, r *http.Request, errMessage string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, "login.html", loginPageData{Error: errMessage}); err != nil {
		slogRenderError(r, "login.html", err)
	}
}
