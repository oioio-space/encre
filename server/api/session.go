package api

import (
	"net/http"

	"github.com/oioio-space/encre/server/auth"
	"github.com/oioio-space/encre/server/store"
)

// childSession resolves the child session cookie on r via
// [auth.SessionFromCookie] — which, per that package's contract, only ever
// returns a [store.SessionChild] session or an error, never a parent's
// session under the wrong name. On any failure it writes a 401 and returns
// ok=false; the caller must return immediately without reading r further.
func (s *Server) childSession(w http.ResponseWriter, r *http.Request) (sess *store.Session, ok bool) {
	sess, err := auth.SessionFromCookie(r, s.db, auth.CookieChild, s.now())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "session invalide")
		return nil, false
	}
	return sess, true
}

// ownRun loads the run named by the request's {id} path value and checks
// that it belongs to sess's child, so one child can never read or finish
// another's run. On any failure it writes the appropriate response and
// returns ok=false.
func (s *Server) ownRun(w http.ResponseWriter, r *http.Request, sess *store.Session) (run *store.Run, ok bool) {
	run, err := s.db.RunByID(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeStoreError(w, err, "run introuvable")
		return nil, false
	}
	if run.ChildID != sess.SubjectID {
		writeError(w, http.StatusForbidden, "cette run appartient à un autre enfant")
		return nil, false
	}
	return run, true
}
