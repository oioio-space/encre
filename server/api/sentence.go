package api

import (
	"cmp"
	"context"
	"net/http"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/oioio-space/encre/engine"
	"github.com/oioio-space/encre/server/gen"
	"github.com/oioio-space/encre/server/store"
)

// generatedSentenceView is one sentence as [handleGenerateSentences] and
// [handleRegenerateSentence] answer it.
type generatedSentenceView struct {
	ID         string `json:"id"`
	Text       string `json:"text"`
	TargetForm string `json:"targetForm"`
}

// generateSentencesResponse is the body of
// POST /lists/{id}/items/{itemID}/sentences/generate.
type generateSentencesResponse struct {
	Sentences []generatedSentenceView `json:"sentences"`
	// Rejected is how many of the model's own phrases [gen.Filter] turned
	// down — 0 to 3, since ENCRE_04 §9 asks for three per word. The parent
	// panel is expected to show "complétez à la main" once
	// len(Sentences) < 3.
	Rejected int `json:"rejected"`
}

// errGenUnavailable is what [handleGenerateSentences] and
// [handleRegenerateSentence] answer with when there is no way to reach the
// model at all: no [Server.genClient] configured (no API key in the
// environment) or the call itself failed. Either way the response is the
// same shape and status, so the parent panel has exactly one code path for
// "fall back to typing the sentence" (ENCRE_03 §9).
const errGenUnavailable = "génération indisponible pour le moment ; saisissez la phrase vous-même"

// handleGenerateSentences asks [server/gen] for three sentences for one
// item's word (ENCRE_04 §9) and stores the ones [gen.Filter] accepts, each
// unapproved — the parent still taps each one before it can be played.
//
// It answers 503 with [errGenUnavailable], never a 500, when generation
// itself is what failed (no client configured, the API unreachable, a
// malformed reply): none of that is this server's fault to apologize for,
// and the parent has an immediate way forward — [handleManualSentence].
func (s *Server) handleGenerateSentences(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.parentSession(w, r)
	if !ok {
		return
	}
	list, ok := s.ownList(w, r, sess)
	if !ok {
		return
	}
	item, ok := s.ownItem(w, r, list)
	if !ok {
		return
	}
	if s.genClient == nil {
		writeError(w, http.StatusServiceUnavailable, errGenUnavailable)
		return
	}

	ctx := r.Context()
	result, err := gen.GenerateSentences(ctx, s.genClient, wordRequestFor(item), s.whitelist)
	if err != nil {
		s.log.Error("generating sentences", "item", item.ID, "error", err)
		writeError(w, http.StatusServiceUnavailable, errGenUnavailable)
		return
	}

	out, err := s.saveGenerated(ctx, item.ID, result.Accepted)
	if err != nil {
		s.log.Error("saving generated sentences", "item", item.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "erreur serveur")
		return
	}
	writeJSON(w, http.StatusOK, generateSentencesResponse{Sentences: out, Rejected: result.Rejected})
}

// handleRegenerateSentence replaces one sentence's text with a fresh one
// for the same item, drawn the same way [handleGenerateSentences] draws its
// first three (ENCRE_04 §7, "le parent valide chaque phrase d'un tap ; une
// phrase refusée est régénérée"). The sentence keeps its ID, so nothing
// referencing it elsewhere has to change, but loses its audio and approval
// — both were recorded against the old text.
func (s *Server) handleRegenerateSentence(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.parentSession(w, r)
	if !ok {
		return
	}
	list, ok := s.ownList(w, r, sess)
	if !ok {
		return
	}
	sentence, ok := s.ownSentence(w, r, list)
	if !ok {
		return
	}
	if s.genClient == nil {
		writeError(w, http.StatusServiceUnavailable, errGenUnavailable)
		return
	}

	ctx := r.Context()
	item, err := s.db.ItemByID(ctx, sentence.ItemID)
	if err != nil {
		s.writeStoreError(w, err, "mot introuvable")
		return
	}
	result, err := gen.GenerateSentences(ctx, s.genClient, wordRequestFor(item), s.whitelist)
	if err != nil || len(result.Accepted) == 0 {
		if err != nil {
			s.log.Error("regenerating sentence", "sentence", sentence.ID, "error", err)
		}
		writeError(w, http.StatusServiceUnavailable, errGenUnavailable)
		return
	}

	fresh := result.Accepted[0]
	sentence.Text, sentence.TargetForm, sentence.AudioPath, sentence.Approved = fresh.Texte, fresh.Forme, "", false
	if err := s.db.SaveSentence(ctx, sentence); err != nil {
		s.log.Error("saving regenerated sentence", "sentence", sentence.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "erreur serveur")
		return
	}
	writeJSON(w, http.StatusOK, generatedSentenceView{ID: sentence.ID, Text: sentence.Text, TargetForm: sentence.TargetForm})
}

// handleApproveSentence marks one sentence approved (ENCRE_04 §7, "le
// parent valide chaque phrase d'un tap").
func (s *Server) handleApproveSentence(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.parentSession(w, r)
	if !ok {
		return
	}
	list, ok := s.ownList(w, r, sess)
	if !ok {
		return
	}
	sentence, ok := s.ownSentence(w, r, list)
	if !ok {
		return
	}
	if err := s.db.ApproveSentence(r.Context(), sentence.ID); err != nil {
		s.writeStoreError(w, err, "phrase introuvable")
		return
	}
	writeJSON(w, http.StatusOK, struct{}{})
}

// maxManualSentenceRunes bounds a manual sentence: generous for eight CE1
// words with punctuation, not for an arbitrary paste.
const maxManualSentenceRunes = 200

// manualSentenceRequest is the body of
// POST /lists/{id}/items/{itemID}/sentences/manual.
type manualSentenceRequest struct {
	Texte string `json:"texte"`
	Forme string `json:"forme"`
}

// handleManualSentence is ENCRE_03 §9's fallback: "le parent tape la
// phrase" — for when the API is unavailable, or simply because the parent
// prefers to write their own. A sentence entered this way is stored
// approved immediately and skips [gen.Filter]: the trust model is the same
// one [handleQuickWord] already applies to a word the parent typed
// themselves — nobody is protecting a CE1 reader from their own parent's
// sentence, only from an AI's.
func (s *Server) handleManualSentence(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.parentSession(w, r)
	if !ok {
		return
	}
	list, ok := s.ownList(w, r, sess)
	if !ok {
		return
	}
	item, ok := s.ownItem(w, r, list)
	if !ok {
		return
	}
	req, ok := decodeJSON[manualSentenceRequest](w, r)
	if !ok {
		return
	}
	texte := strings.TrimSpace(req.Texte)
	forme := strings.TrimSpace(req.Forme)
	switch {
	case texte == "":
		writeError(w, http.StatusBadRequest, "la phrase est vide")
		return
	case utf8.RuneCountInString(texte) > maxManualSentenceRunes:
		writeError(w, http.StatusBadRequest, "la phrase est trop longue")
		return
	case forme == "":
		forme = item.Text
	}

	id, err := newOpaqueID("sentence id")
	if err != nil {
		s.log.Error("generating sentence id", "error", err)
		writeError(w, http.StatusInternalServerError, "erreur serveur")
		return
	}
	sentence := &store.Sentence{ID: id, ItemID: item.ID, Text: texte, TargetForm: forme, Approved: true}
	if err := s.db.SaveSentence(r.Context(), sentence); err != nil {
		s.log.Error("saving manual sentence", "error", err)
		writeError(w, http.StatusInternalServerError, "erreur serveur")
		return
	}
	writeJSON(w, http.StatusOK, generatedSentenceView{ID: id, Text: texte, TargetForm: forme})
}

// wordRequestFor builds the [gen.WordRequest] item's own word and Couleurs
// describe, Couleurs sorted for a deterministic prompt.
func wordRequestFor(item *store.Item) gen.WordRequest {
	var couleurs []engine.Color
	for c, n := range item.Colors {
		if n > 0 {
			couleurs = append(couleurs, c)
		}
	}
	slices.SortFunc(couleurs, cmp.Compare)
	return gen.WordRequest{Mot: item.Text, Couleurs: couleurs}
}

// saveGenerated persists every accepted sentence for itemID, unapproved,
// and returns the views [handleGenerateSentences] answers with.
func (s *Server) saveGenerated(ctx context.Context, itemID string, accepted []gen.Sentence) ([]generatedSentenceView, error) {
	out := make([]generatedSentenceView, 0, len(accepted))
	for _, a := range accepted {
		id, err := newOpaqueID("sentence id")
		if err != nil {
			return nil, err
		}
		sentence := &store.Sentence{ID: id, ItemID: itemID, Text: a.Texte, TargetForm: a.Forme}
		if err := s.db.SaveSentence(ctx, sentence); err != nil {
			return nil, err
		}
		out = append(out, generatedSentenceView{ID: id, Text: a.Texte, TargetForm: a.Forme})
	}
	return out, nil
}
