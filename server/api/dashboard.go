package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/oioio-space/encre/engine"
)

// dashboardMessage is the one line ENCRE_01 §14 asks the dashboard to
// always show: "la fréquence prédit l'abandon bien plus que le niveau, en
// simulation" is the finding it is built on, quoted from that same
// paragraph rather than paraphrased.
const dashboardMessage = "Trois sessions courtes par semaine valent mieux qu'une longue."

// dicteeSummary is one list's dictée record, as the dashboard reports it.
type dicteeSummary struct {
	ListID  string `json:"listID"`
	Label   string `json:"label"`
	Correct int    `json:"correct"`
	Total   int    `json:"total"`
}

// dashboardResponse is the body of GET /children/{id}/dashboard
// (brief/ENCRE_01 §14: "dorées, rangs, fioles des Couleurs, temps joué,
// résultats de dictée").
type dashboardResponse struct {
	Pseudo             string               `json:"pseudo"`
	Rank               int                  `json:"rank"`
	BestRank           int                  `json:"bestRank"`
	Levels             map[engine.Color]int `json:"levels"`
	GoldWords          int                  `json:"goldWords"`
	PlayedTodaySeconds int                  `json:"playedTodaySeconds"`
	RemainingSeconds   int                  `json:"remainingSeconds"`
	Dictees            []dicteeSummary      `json:"dictees"`
	Message            string               `json:"message"`
}

// handleChildDashboard answers the parent-panel dashboard for one child
// (ENCRE_01 §14).
func (s *Server) handleChildDashboard(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.parentSession(w, r)
	if !ok {
		return
	}
	child, ok := s.ownChild(w, r, sess)
	if !ok {
		return
	}
	ctx := r.Context()

	states, err := s.db.WordStates(ctx, child.ID)
	if err != nil {
		s.writeStoreError(w, err, "")
		return
	}
	gold := 0
	for _, st := range states {
		if st.Gold {
			gold++
		}
	}

	// bonus (play_time.bonus_seconds) is not read here directly: it is
	// already folded into remaining by [Server.remainingSeconds] below.
	played, _, err := s.db.PlayTime(ctx, child.ID, dayOf(s.now()))
	if err != nil {
		s.writeStoreError(w, err, "")
		return
	}
	remaining, err := s.remainingSeconds(ctx, child)
	if err != nil {
		s.writeStoreError(w, err, "")
		return
	}

	dictees, err := s.dicteeSummaries(ctx, child.ID)
	if err != nil {
		s.log.Error("loading dictee summaries", "error", err)
		writeError(w, http.StatusInternalServerError, "erreur serveur")
		return
	}

	ec := child.Engine()
	writeJSON(w, http.StatusOK, dashboardResponse{
		Pseudo: child.Pseudo, Rank: ec.Rank, BestRank: ec.BestRank, Levels: ec.Level,
		GoldWords: gold, PlayedTodaySeconds: played, RemainingSeconds: remaining,
		Dictees: dictees, Message: dashboardMessage,
	})
}

// dicteeSummaries returns one [dicteeSummary] per word list belonging to
// childID, correct and total counted from [store.Store.DicteeResultsOfList].
// A list with no dictée result recorded yet is omitted rather than shown
// with Total 0, so the dashboard reads as "what has been reported" and not
// as a checklist of what has not.
func (s *Server) dicteeSummaries(ctx context.Context, childID string) ([]dicteeSummary, error) {
	rows, err := s.db.DB().QueryContext(ctx,
		`SELECT id, label FROM word_lists WHERE child_id = ?`, childID)
	if err != nil {
		return nil, fmt.Errorf("querying lists: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type listRow struct{ id, label string }
	var lists []listRow
	for rows.Next() {
		var l listRow
		if err := rows.Scan(&l.id, &l.label); err != nil {
			return nil, fmt.Errorf("scanning list: %w", err)
		}
		lists = append(lists, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating lists: %w", err)
	}

	var out []dicteeSummary
	for _, l := range lists {
		results, err := s.db.DicteeResultsOfList(ctx, l.id)
		if err != nil {
			return nil, fmt.Errorf("loading dictee results of list %s: %w", l.id, err)
		}
		if len(results) == 0 {
			continue
		}
		sum := dicteeSummary{ListID: l.id, Label: l.label, Total: len(results)}
		for _, r := range results {
			if r.Correct {
				sum.Correct++
			}
		}
		out = append(out, sum)
	}
	return out, nil
}
