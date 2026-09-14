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

// parentSession resolves the parent session cookie on r and requires it to
// be TOTP-fresh ([auth.RequireParent], ENCRE_04 §7's "session + TOTP frais"
// on every parent route). On any failure — no cookie, an expired session, a
// parent session past its TOTP freshness window — it writes a 401 and
// returns ok=false; the caller must return immediately without reading r
// further.
func (s *Server) parentSession(w http.ResponseWriter, r *http.Request) (sess *store.Session, ok bool) {
	sess, err := auth.SessionFromCookie(r, s.db, auth.CookieParent, s.now())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "session invalide")
		return nil, false
	}
	if err := auth.RequireParent(sess, s.now()); err != nil {
		writeError(w, http.StatusUnauthorized, "session invalide ou vérification en deux étapes expirée")
		return nil, false
	}
	return sess, true
}

// ownChild loads the child named by the request's {id} path value and
// checks that it belongs to sess's parent, so one parent can never read or
// change another's child. On any failure it writes the appropriate response
// and returns ok=false.
func (s *Server) ownChild(w http.ResponseWriter, r *http.Request, sess *store.Session) (child *store.Child, ok bool) {
	child, err := s.db.ChildByID(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeStoreError(w, err, "enfant introuvable")
		return nil, false
	}
	if child.ParentID != sess.SubjectID {
		writeError(w, http.StatusForbidden, "cet enfant appartient à un autre parent")
		return nil, false
	}
	return child, true
}

// ownList loads the word list named by the request's {id} path value and
// checks that it belongs to a child of sess's parent, so one parent can
// never read or change another's list. On any failure it writes the
// appropriate response and returns ok=false.
func (s *Server) ownList(w http.ResponseWriter, r *http.Request, sess *store.Session) (list *store.WordList, ok bool) {
	ctx := r.Context()
	list, err := s.db.ListByID(ctx, r.PathValue("id"))
	if err != nil {
		s.writeStoreError(w, err, "liste introuvable")
		return nil, false
	}
	child, err := s.db.ChildByID(ctx, list.ChildID)
	if err != nil {
		s.writeStoreError(w, err, "liste introuvable")
		return nil, false
	}
	if child.ParentID != sess.SubjectID {
		writeError(w, http.StatusForbidden, "cette liste appartient à un autre parent")
		return nil, false
	}
	return list, true
}

// ownItem loads the item named by the request's {itemID} path value and
// checks that it belongs to list, so a parent can never generate or attach
// a manual sentence to an item outside the list named in the URL. On any
// failure it writes the appropriate response and returns ok=false.
func (s *Server) ownItem(w http.ResponseWriter, r *http.Request, list *store.WordList) (item *store.Item, ok bool) {
	item, err := s.db.ItemByID(r.Context(), r.PathValue("itemID"))
	if err != nil {
		s.writeStoreError(w, err, "mot introuvable")
		return nil, false
	}
	if item.ListID != list.ID {
		writeError(w, http.StatusForbidden, "ce mot n'appartient pas à cette liste")
		return nil, false
	}
	return item, true
}

// ownSentence loads the sentence named by the request's {sid} path value
// and checks that it belongs to an item of list, the same ownership chain
// [ownItem] checks one level up. On any failure it writes the appropriate
// response and returns ok=false.
func (s *Server) ownSentence(w http.ResponseWriter, r *http.Request, list *store.WordList) (sentence *store.Sentence, ok bool) {
	ctx := r.Context()
	sentence, err := s.db.SentenceByID(ctx, r.PathValue("sid"))
	if err != nil {
		s.writeStoreError(w, err, "phrase introuvable")
		return nil, false
	}
	item, err := s.db.ItemByID(ctx, sentence.ItemID)
	if err != nil {
		s.writeStoreError(w, err, "phrase introuvable")
		return nil, false
	}
	if item.ListID != list.ID {
		writeError(w, http.StatusForbidden, "cette phrase n'appartient pas à cette liste")
		return nil, false
	}
	return sentence, true
}

// ownRun loads the run named by the request's {id} path value and checks
// that it belongs to sess's child, so one child can never read or finish
// another's run. On any failure it writes the appropriate response and
// returns ok=false.
func (s *Server) ownRun(w http.ResponseWriter, r *http.Request, sess *store.Session) (run *store.Run, ok bool) {
	return s.ownRunByID(w, r, sess, r.PathValue("id"))
}

// ownRunByID is [Server.ownRun] with the run ID passed explicitly, for a
// route whose path value carries more than the bare ID — [handleChildResultCard]'s
// "{runID}.png" — and so cannot hand r.PathValue("id") straight to
// [store.Store.RunByID].
func (s *Server) ownRunByID(w http.ResponseWriter, r *http.Request, sess *store.Session, id string) (run *store.Run, ok bool) {
	run, err := s.db.RunByID(r.Context(), id)
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
