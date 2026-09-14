package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/oioio-space/encre/engine"
	"github.com/oioio-space/encre/lexique"
	"github.com/oioio-space/encre/server/store"
)

// maxQuickWordRunes bounds a quick-word request: this button adds one word
// (occasionally a short phrase) to "mes mots", not an arbitrary paste — that
// is what the list-collage screen is for.
const maxQuickWordRunes = 60

// sourceQuickWord is this package's [store.Item.Source] value for an item
// [handleQuickWord] created directly from the "ajouter ce mot" button
// (ENCRE_04 §7). store.Item.Source has no type of its own — "its values are
// defined by the caller" — and server/parent's list analysis already claims
// 1 for its own lexique-analysed items (see that package's sourceLexique);
// this package's items are also lexique-analysed, one word at a time
// instead of a pasted list, so they get a value of their own rather than
// silently reusing server/parent's.
const sourceQuickWord = 2

// mesMotsListID is the deterministic ID of the one "mes mots" list a
// child's quick words accumulate into: a fixed, child-scoped ID rather than
// a fresh one per word, so that ten taps of "ajouter ce mot" build one deck
// instead of ten single-item lists.
func mesMotsListID(childID string) string { return "mesmots-" + childID }

// quickWordRequest is the body of POST /children/{id}/quick-word.
type quickWordRequest struct {
	Text string `json:"text"`
}

// quickWordResponse is the body handleQuickWord answers.
type quickWordResponse struct {
	ItemID     string               `json:"itemID"`
	Traps      map[engine.Color]int `json:"traps"`
	Confidence float64              `json:"confidence"`
}

// handleQuickWord adds one word to the child's "mes mots" deck (ENCRE_04
// §7, the "ajouter ce mot" button): analysed the same way server/parent's
// list-collage screen analyses a pasted word ([lexique.Lexicon.Analyze]),
// enabled immediately — there is no parent review step for a single word
// the parent just typed themselves.
func (s *Server) handleQuickWord(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.parentSession(w, r)
	if !ok {
		return
	}
	child, ok := s.ownChild(w, r, sess)
	if !ok {
		return
	}
	req, ok := decodeJSON[quickWordRequest](w, r)
	if !ok {
		return
	}
	text := strings.TrimSpace(req.Text)
	switch {
	case text == "":
		writeError(w, http.StatusBadRequest, "le mot est vide")
		return
	case utf8.RuneCountInString(text) > maxQuickWordRunes:
		writeError(w, http.StatusBadRequest, "le mot est trop long")
		return
	}

	ctx := r.Context()
	listID, err := s.mesMotsList(ctx, child.ID, s.now())
	if err != nil {
		s.log.Error("preparing mes mots list", "error", err)
		writeError(w, http.StatusInternalServerError, "erreur serveur")
		return
	}

	itemID, err := newItemID()
	if err != nil {
		s.log.Error("generating item id", "error", err)
		writeError(w, http.StatusInternalServerError, "erreur serveur")
		return
	}
	analysis := lexique.Embedded().Analyze(text)
	item := &store.Item{
		ID: itemID, ListID: listID, Kind: engine.KindWord, Text: text,
		Colors: analysis.Traps, Rules: analysis.Rules(), Family: analysis.Family,
		Source: sourceQuickWord, Confidence: analysis.Confidence,
		Enabled: true, Confirmed: true,
	}
	if err := s.db.SaveItem(ctx, item); err != nil {
		s.log.Error("saving quick word item", "error", err)
		writeError(w, http.StatusInternalServerError, "erreur serveur")
		return
	}

	writeJSON(w, http.StatusOK, quickWordResponse{
		ItemID: itemID, Traps: analysis.Traps, Confidence: analysis.Confidence,
	})
}

// mesMotsList returns the ID of childID's "mes mots" list, creating it —
// already validated, so a quick word is playable the moment it is added —
// the first time this child ever adds one. now stamps a freshly created
// list's CreatedAt.
func (s *Server) mesMotsList(ctx context.Context, childID string, now time.Time) (string, error) {
	id := mesMotsListID(childID)
	_, err := s.db.ListByID(ctx, id)
	switch {
	case err == nil:
		return id, nil
	case errors.Is(err, store.ErrNotFound):
		// fall through to creation below
	default:
		return "", err
	}

	l := &store.WordList{ID: id, ChildID: childID, Label: "Mes mots", Validated: true, CreatedAt: now}
	if err := s.db.CreateList(ctx, l); err != nil {
		return "", err
	}
	return id, nil
}
