package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/oioio-space/encre/content"
	"github.com/oioio-space/encre/engine"
)

// rulesResponse is the body of GET /child/rules: the fioles d'encre
// (ENCRE_01, "affiché en fioles d'encre") — the child's level in every
// Couleur, keyed the same way [runStartResponse.Levels] already is.
type rulesResponse struct {
	Levels map[engine.Color]int `json:"levels"`
}

// handleChildRules answers the child's level in each Couleur, straight from
// the engine standing: there is no second copy of this to drift from the one
// [handleRunStart] already reports.
func (s *Server) handleChildRules(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.childSession(w, r)
	if !ok {
		return
	}
	child, err := s.db.ChildByID(r.Context(), sess.SubjectID)
	if err != nil {
		s.writeStoreError(w, err, "enfant introuvable")
		return
	}
	writeJSON(w, http.StatusOK, rulesResponse{Levels: child.Engine().Level})
}

// childExploits is this package's reading of [server/store.Child.Exploits],
// which server/store keeps as opaque JSON because it has no engine type of
// its own yet — the same gap [dailyLimit] fills for the daily-limit column.
// Achieved holds the IDs of every [content.Exploit] the child has earned.
type childExploits struct {
	Achieved []string `json:"achieved"`
}

// achievedExploits reads raw as a [childExploits] and returns the set of
// achieved IDs, or an empty set if raw is empty or unparsable — a child who
// has achieved nothing yet, not a broken response.
func achievedExploits(raw []byte) map[string]bool {
	out := map[string]bool{}
	if len(raw) == 0 {
		return out
	}
	var ce childExploits
	if err := json.Unmarshal(raw, &ce); err != nil {
		return out
	}
	for _, id := range ce.Achieved {
		out[id] = true
	}
	return out
}

// exploitView is one feat in the gallery GET /child/exploits answers.
type exploitView struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Reward string `json:"reward"`
	Hidden bool   `json:"hidden"`
	// Achieved says whether this child has done it.
	Achieved bool `json:"achieved"`
}

// hiddenPlaceholder is what an unachieved hidden Exploit shows in the
// gallery (ENCRE_03 §8, "Cachés (???)"): its name is never written until the
// child has actually done it.
const hiddenPlaceholder = "???"

// handleChildExploits answers the gallery of feats: every [content.Exploit],
// visible ones first (the order [content.Pack.Exploits] already carries), an
// unachieved hidden one masked as "???" with no reward shown, and an
// achieved one — hidden or not — shown in full.
func (s *Server) handleChildExploits(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.childSession(w, r)
	if !ok {
		return
	}
	child, err := s.db.ChildByID(r.Context(), sess.SubjectID)
	if err != nil {
		s.writeStoreError(w, err, "enfant introuvable")
		return
	}
	achieved := achievedExploits(child.Exploits)

	pack := content.Embedded()
	out := make([]exploitView, len(pack.Exploits))
	for i, e := range pack.Exploits {
		got := achieved[e.ID]
		v := exploitView{ID: e.ID, Name: e.Name, Reward: e.Reward, Hidden: e.Hidden, Achieved: got}
		if e.Hidden && !got {
			v.Name, v.Reward = hiddenPlaceholder, ""
		}
		out[i] = v
	}
	writeJSON(w, http.StatusOK, struct {
		Exploits []exploitView `json:"exploits"`
	}{out})
}

// bestiaryWordState names where one word stands in the gallery (ENCRE_02
// §3's "grille 3 × 4" panel): the child has never met it, has met and spelled
// it, or has kept it gold.
type bestiaryWordState string

const (
	bestiaryMissing bestiaryWordState = "missing"
	bestiaryKnown   bestiaryWordState = "known"
	bestiaryGold    bestiaryWordState = "gold"
	bestiaryCursed  bestiaryWordState = "cursed"
)

// bestiaryWord is one card GET /child/bestiary answers.
type bestiaryWord struct {
	ItemID    string            `json:"itemID"`
	Text      string            `json:"text"`
	Family    string            `json:"family"`
	State     bestiaryWordState `json:"state"`
	Tarnished bool              `json:"tarnished"`
}

// handleChildBestiary answers every validated, enabled word the child owns
// (ENCRE_02 §3's gallery), each carrying the state its [engine.WordState]
// implies: gold ones in gold, met-but-not-gold ones known, never-met ones
// missing — the "manquantes en silhouette" the charte graphique asks for,
// left to the client to draw.
//
// It shares [Server.wordCatalog] with [Server.handleRunStart]: the pool a
// deck can draw from and the pool the gallery shows are the same catalog,
// so the two can never quietly disagree about what the child owns.
func (s *Server) handleChildBestiary(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.childSession(w, r)
	if !ok {
		return
	}
	ctx := r.Context()

	states, err := s.db.WordStates(ctx, sess.SubjectID)
	if err != nil {
		s.writeStoreError(w, err, "")
		return
	}
	catalog, err := s.wordCatalog(ctx, sess.SubjectID, states)
	if err != nil {
		s.log.Error("loading word catalog", "error", err)
		writeError(w, http.StatusInternalServerError, "erreur serveur")
		return
	}

	words := make([]bestiaryWord, len(catalog.Known))
	for i, item := range catalog.Known {
		st := states[item.ID]
		words[i] = bestiaryWord{
			ItemID: item.ID, Text: item.Text, Family: item.Family,
			State:     wordBestiaryState(st),
			Tarnished: st != nil && st.Tarnished,
		}
	}
	writeJSON(w, http.StatusOK, struct {
		Words []bestiaryWord `json:"words"`
	}{words})
}

// wordBestiaryState maps a [engine.WordState] (nil for a word never met) to
// its gallery state.
func wordBestiaryState(st *engine.WordState) bestiaryWordState {
	switch {
	case st == nil || !st.Seen:
		return bestiaryMissing
	case st.Cursed:
		return bestiaryCursed
	case st.Gold:
		return bestiaryGold
	default:
		return bestiaryKnown
	}
}

// resultCardRunID strips the ".png" suffix ENCRE_04 §7's
// GET /child/result-card/{runID}.png asks for. net/http's [http.ServeMux]
// wildcard segments cannot mix a literal suffix into the same segment as
// {runID} (a pattern only matches "{runID}" wholesale), so the route this
// package registers captures the whole "{runID}.png" segment and this
// trims it back down to the run ID [Server.ownRun] actually looks up.
func resultCardRunID(r *http.Request) string {
	return strings.TrimSuffix(r.PathValue("runID"), ".png")
}
