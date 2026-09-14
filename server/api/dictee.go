package api

import (
	"net/http"

	"github.com/oioio-space/encre/server/store"
)

// maxDicteeResults bounds a dictée-result request well past what a real
// dictée ever holds (a week's list is a handful of words), so a client
// cannot make the handler write an unbounded number of rows.
const maxDicteeResults = 200

// dicteeCompletionBonusSeconds and dicteeCorrectWordBonusSeconds are
// ENCRE_01 §14's "bonus fixe pour l'avoir faite, bonus par mot du deck
// juste": five minutes for reporting the dictée at all, plus thirty
// seconds per word the child got right on paper. Both numbers are a
// starting point pending balancing (brief/ENCRE_04 §4 asks for a
// simulation run before any number here is treated as final) — what is not
// negotiable, and what every test in this file pins down, is the sign:
// there is no path through this handler that ever subtracts.
const (
	dicteeCompletionBonusSeconds  = 5 * 60
	dicteeCorrectWordBonusSeconds = 30
)

// dicteeResultInput is one word's outcome, as the parent reports it.
type dicteeResultInput struct {
	ItemID  string `json:"itemID"`
	Correct bool   `json:"correct"`
}

// dicteeResultRequest is the body of POST /lists/{id}/dictee-result.
type dicteeResultRequest struct {
	Results []dicteeResultInput `json:"results"`
}

// dicteeResultResponse is the body handleListDicteeResult answers: what was
// recorded, and the bonus it earned — echoed back rather than left for the
// client to recompute, since the server is the only place ENCRE_01 §14's
// rule is enforced.
type dicteeResultResponse struct {
	Correct      int `json:"correct"`
	Total        int `json:"total"`
	BonusSeconds int `json:"bonusSeconds"`
}

// handleListDicteeResult records a dictée the child took on paper, word by
// word, and credits the bonus play time it earns (ENCRE_01 §14). It never
// lowers anything: a wrong word contributes nothing beyond the fixed
// completion bonus every report earns, and there is no field anywhere in
// [dicteeResultRequest] a parent could use to subtract play time, mastery,
// or anything else — "jamais de perte" is a property of what this handler
// cannot do, not a check it runs.
func (s *Server) handleListDicteeResult(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.parentSession(w, r)
	if !ok {
		return
	}
	list, ok := s.ownList(w, r, sess)
	if !ok {
		return
	}
	req, ok := decodeJSON[dicteeResultRequest](w, r)
	if !ok {
		return
	}
	if len(req.Results) == 0 {
		writeError(w, http.StatusBadRequest, "aucun résultat")
		return
	}
	if len(req.Results) > maxDicteeResults {
		writeError(w, http.StatusBadRequest, "trop de résultats")
		return
	}

	ctx := r.Context()
	items, err := s.db.ItemsOfList(ctx, list.ID)
	if err != nil {
		s.writeStoreError(w, err, "")
		return
	}
	inList := make(map[string]bool, len(items))
	for _, item := range items {
		inList[item.ID] = true
	}
	for _, res := range req.Results {
		if !inList[res.ItemID] {
			writeError(w, http.StatusBadRequest, "un mot ne fait pas partie de cette liste")
			return
		}
	}

	now := s.now()
	correct := 0
	for _, res := range req.Results {
		id, err := newDicteeResultID()
		if err != nil {
			s.log.Error("generating dictee result id", "error", err)
			writeError(w, http.StatusInternalServerError, "erreur serveur")
			return
		}
		dr := &store.DicteeResult{ID: id, ListID: list.ID, ItemID: res.ItemID, Correct: res.Correct, EnteredAt: now}
		if err := s.db.SaveDicteeResult(ctx, dr); err != nil {
			s.log.Error("saving dictee result", "error", err)
			writeError(w, http.StatusInternalServerError, "erreur serveur")
			return
		}
		if res.Correct {
			correct++
		}
	}

	bonus := dicteeCompletionBonusSeconds + correct*dicteeCorrectWordBonusSeconds
	if err := s.db.AddBonusSeconds(ctx, list.ChildID, dayOf(now), bonus); err != nil {
		s.log.Error("crediting dictee bonus", "error", err)
		writeError(w, http.StatusInternalServerError, "erreur serveur")
		return
	}

	writeJSON(w, http.StatusOK, dicteeResultResponse{
		Correct: correct, Total: len(req.Results), BonusSeconds: bonus,
	})
}
