package api

import (
	"cmp"
	"net/http"
	"slices"

	"github.com/oioio-space/encre/engine"
	"github.com/oioio-space/encre/server/auth"
	"github.com/oioio-space/encre/server/store"
)

// childLoginRequest is the body of POST /child/login.
type childLoginRequest struct {
	Pseudo  string `json:"pseudo"`
	Pattern string `json:"pattern"`
}

// handleChildLogin verifies pseudo and pattern with [auth.LoginChild] and,
// on success, opens a child session cookie (brief/ENCRE_04 §7).
func (s *Server) handleChildLogin(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[childLoginRequest](w, r)
	if !ok {
		return
	}

	ctx := r.Context()
	childID, err := auth.LoginChild(ctx, s.db, req.Pseudo, req.Pattern)
	if err != nil {
		// auth.ErrInvalidCredentials covers every reason a child login can
		// fail; ENCRE_04 §7 asks for no enumeration oracle, so every one of
		// them answers identically.
		writeError(w, http.StatusUnauthorized, "identifiants invalides")
		return
	}

	token, err := auth.CreateSession(ctx, s.db, store.SessionChild, childID, s.now())
	if err != nil {
		s.log.Error("creating child session", "error", err)
		writeError(w, http.StatusInternalServerError, "erreur serveur")
		return
	}
	auth.SetSessionCookie(w, auth.CookieChild, token, auth.ChildSessionTTL)
	writeJSON(w, http.StatusOK, map[string]string{"childID": childID})
}

// gardeCandidate is one gold word available to hold in the Garde for the
// next run.
type gardeCandidate struct {
	ItemID    string `json:"itemID"`
	Tarnished bool   `json:"tarnished"`
}

// childMeResponse is the body of GET /child/me.
type childMeResponse struct {
	ChildID          string           `json:"childID"`
	Pseudo           string           `json:"pseudo"`
	Rank             int              `json:"rank"`
	RemainingSeconds int              `json:"remainingSeconds"`
	Garde            []gardeCandidate `json:"garde"`
}

// handleChildMe answers the profile screen: rank, time left today, and the
// gold words available to the Garde, tarnished ones first (brief/ENCRE_04
// §7).
func (s *Server) handleChildMe(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.childSession(w, r)
	if !ok {
		return
	}
	ctx := r.Context()

	child, err := s.db.ChildByID(ctx, sess.SubjectID)
	if err != nil {
		s.writeStoreError(w, err, "enfant introuvable")
		return
	}
	states, err := s.db.WordStates(ctx, sess.SubjectID)
	if err != nil {
		s.writeStoreError(w, err, "")
		return
	}
	remaining, err := s.remainingSeconds(ctx, child)
	if err != nil {
		s.writeStoreError(w, err, "")
		return
	}

	writeJSON(w, http.StatusOK, childMeResponse{
		ChildID:          child.ID,
		Pseudo:           child.Pseudo,
		Rank:             child.Engine().Rank,
		RemainingSeconds: remaining,
		Garde:            goldGardeCandidates(states),
	})
}

// goldGardeCandidates lists every gold, untamed-curse word in states,
// tarnished ones first and ties broken by ID — the same "tarnished to the
// head" rule engine.BuildDeck applies when it actually builds the Garde, so
// the list a child chooses from matches the order they would be offered in,
// deterministically rather than in map iteration's random order.
func goldGardeCandidates(states map[string]*engine.WordState) []gardeCandidate {
	var out []gardeCandidate
	for id, st := range states {
		if st.Gold && !st.Cursed {
			out = append(out, gardeCandidate{ItemID: id, Tarnished: st.Tarnished})
		}
	}
	slices.SortFunc(out, func(a, b gardeCandidate) int {
		switch {
		case a.Tarnished && !b.Tarnished:
			return -1
		case b.Tarnished && !a.Tarnished:
			return 1
		default:
			return cmp.Compare(a.ItemID, b.ItemID)
		}
	})
	return out
}
